package service

import (
	"context"
	"database/sql"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/migrations"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// @brief Use an explicitly supplied disposable database; never run fixture DDL against an arbitrary database.
func rewardTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("LEADERBOARD_REWARD_TEST_DSN")
	if dsn == "" {
		t.Skip("requires disposable LEADERBOARD_REWARD_TEST_DSN")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	var name string
	require.NoError(t, db.QueryRow(`SELECT current_database()`).Scan(&name))
	require.Equal(t, "sub2api_reward_test", name)
	_, err = db.Exec(`CREATE SCHEMA IF NOT EXISTS custom_leaderboard;
 CREATE TABLE IF NOT EXISTS users(id bigint PRIMARY KEY,username text,email text,balance numeric(20,8) NOT NULL DEFAULT 0,deleted_at timestamptz,updated_at timestamptz);
 CREATE TABLE IF NOT EXISTS redeem_codes(code varchar(32) UNIQUE,type varchar(20),value numeric(20,8),status text,used_by bigint REFERENCES users(id),used_at timestamptz,notes text);
 CREATE TABLE IF NOT EXISTS custom_leaderboard.daily_user_usage(bucket_date date,user_id bigint,total_tokens bigint,request_count bigint,PRIMARY KEY(bucket_date,user_id));
 CREATE TABLE IF NOT EXISTS custom_leaderboard.refresh_runs(bucket_date date,status text,message text,finished_at timestamptz);
 CREATE TABLE IF NOT EXISTS usage_logs(user_id bigint,created_at timestamptz,actual_cost numeric(20,10),billing_type smallint,
 input_tokens bigint DEFAULT 0,output_tokens bigint DEFAULT 0,cache_creation_tokens bigint DEFAULT 0,cache_creation_5m_tokens bigint DEFAULT 0,cache_creation_1h_tokens bigint DEFAULT 0,cache_read_tokens bigint DEFAULT 0,image_input_tokens bigint DEFAULT 0,image_output_tokens bigint DEFAULT 0);`)
	require.NoError(t, err)
	migration, err := migrations.FS.ReadFile("239_daily_leaderboard_rewards.sql")
	require.NoError(t, err)
	_, err = db.Exec(string(migration))
	require.NoError(t, err)
	return db
}

// @brief Reset only this disposable fixture and create ties, subscriptions, cache tokens and boundary usage.
func rewardFixture(t *testing.T, db *sql.DB) {
	t.Helper()
	_, err := db.Exec(`TRUNCATE daily_leaderboard_rewards,redeem_codes,users,usage_logs,custom_leaderboard.daily_user_usage,custom_leaderboard.refresh_runs;
 INSERT INTO users(id,username,email) VALUES (1,'one','one@test'),(2,'two','two@test'),(3,'three','three@test'),(4,'four','four@test');
 INSERT INTO usage_logs(user_id,created_at,actual_cost,billing_type,input_tokens,cache_creation_tokens,cache_creation_5m_tokens,cache_creation_1h_tokens,image_input_tokens) VALUES
 (1,'2026-09-17 16:00:00+00',12.345678951,0,90,10,10,10,0),
 (1,'2026-09-18 15:59:59+00',900,1,100,0,0,0,0),
 (2,'2026-09-18 10:00:00+00',20,0,160,0,10,20,10),
 (3,'2026-09-18 10:00:00+00',0,0,100,0,0,0,0),
 (4,'2026-09-18 10:00:00+00',10000,0,100,0,0,0,0),
 (1,'2026-09-18 16:00:00+00',999,0,999,0,0,0,0),
 (1,'2026-09-17 15:59:59+00',999,0,999,0,0,0,0);
 INSERT INTO custom_leaderboard.daily_user_usage VALUES ('2026-09-18',1,200,2),('2026-09-18',2,200,1),('2026-09-18',3,100,1),('2026-09-18',4,100,1);
 INSERT INTO custom_leaderboard.refresh_runs VALUES ('2026-09-18','success','tokens-v2:Asia/Shanghai','2026-09-18 16:01:00+00');`)
	require.NoError(t, err)
}

// @brief Track cache invalidations safely during concurrent replay tests.
type rewardCacheSpy struct {
	BillingCache
	mu  sync.Mutex
	ids []int64
}

func (s *rewardCacheSpy) InvalidateUserBalance(_ context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ids = append(s.ids, id)
	return nil
}

// @brief Verify decimal settlements and transactional guarantees using a real PostgreSQL server.
func TestLeaderboardRewardPostgres(t *testing.T) {
	db := rewardTestDB(t)
	ctx := context.Background()
	clock := func() time.Time { return time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC) }
	makeService := func() *LeaderboardRewardService {
		s := NewLeaderboardRewardService(db, nil, nil)
		s.now = clock
		return s
	}
	t.Run("custom_percentage_changes_preview_amounts", func(t *testing.T) {
		rewardFixture(t, db)
		s := makeService()
		standard, err := s.Preview(ctx, "10")
		require.NoError(t, err)
		custom, err := s.Preview(ctx, "12.34")
		require.NoError(t, err)
		require.Equal(t, "12.34", custom.RatePercent)
		require.Equal(t, "0.1234", custom.Rate)
		require.Equal(t, "1.52345678", custom.Winners[0].Amount)
		require.NotEqual(t, standard.PreviewID, custom.PreviewID)
	})
	t.Run("exact_spend_ranking_and_concurrent_replay", func(t *testing.T) {
		rewardFixture(t, db)
		cache := &rewardCacheSpy{}
		s := makeService()
		s.billing = cache
		p, err := s.Preview(ctx, "10")
		require.NoError(t, err)
		require.Len(t, p.Winners, 3)
		require.Equal(t, []int64{1, 2, 3}, []int64{p.Winners[0].UserID, p.Winners[1].UserID, p.Winners[2].UserID})
		require.Equal(t, "12.3456789510", p.Winners[0].Spend)
		require.Equal(t, "1.23456790", p.Winners[0].Amount)
		require.Equal(t, "0.00000000", p.Winners[2].Amount)
		var wg sync.WaitGroup
		errs := make(chan error, 8)
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() { defer wg.Done(); _, err := s.Pay(ctx, p.Date, p.PreviewID, "10", 99); errs <- err }()
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			require.NoError(t, err)
		}
		var balance string
		require.NoError(t, db.QueryRow(`SELECT balance::text FROM users WHERE id=1`).Scan(&balance))
		require.Equal(t, "1.23456790", balance)
		require.NoError(t, db.QueryRow(`SELECT balance::text FROM users WHERE id=2`).Scan(&balance))
		require.Equal(t, "2.00000000", balance)
		require.NoError(t, db.QueryRow(`SELECT balance::text FROM users WHERE id=4`).Scan(&balance))
		require.Equal(t, "0.00000000", balance)
		var receipts, history int
		require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM daily_leaderboard_rewards`).Scan(&receipts))
		require.Equal(t, 1, receipts)
		require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM redeem_codes`).Scan(&history))
		require.Equal(t, 2, history)
		require.Len(t, cache.ids, 24)
		restarted := makeService()
		paid, err := restarted.Preview(ctx, "100")
		require.NoError(t, err)
		require.True(t, paid.Paid)
		require.Equal(t, int64(99), paid.ActorID)
		restarted.now = func() time.Time { return clock().Add(24 * time.Hour) }
		retry, err := restarted.Pay(ctx, p.Date, p.PreviewID, "99.99", 100)
		require.NoError(t, err)
		require.Equal(t, paid, retry)
	})
	t.Run("failure_rolls_back_all_winners_and_receipt", func(t *testing.T) {
		rewardFixture(t, db)
		s := makeService()
		p, err := s.Preview(ctx, "10")
		require.NoError(t, err)
		_, err = db.Exec(`ALTER TABLE redeem_codes ADD CONSTRAINT reward_test_failure CHECK(used_by<>2)`)
		require.NoError(t, err)
		t.Cleanup(func() { _, _ = db.Exec(`ALTER TABLE redeem_codes DROP CONSTRAINT IF EXISTS reward_test_failure`) })
		_, err = s.Pay(ctx, p.Date, p.PreviewID, "10", 99)
		require.Error(t, err)
		var sum string
		require.NoError(t, db.QueryRow(`SELECT SUM(balance)::text FROM users`).Scan(&sum))
		require.Equal(t, "0.00000000", sum)
		var count int
		require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM daily_leaderboard_rewards`).Scan(&count))
		require.Zero(t, count)
		require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM redeem_codes`).Scan(&count))
		require.Zero(t, count)
		_, err = db.Exec(`ALTER TABLE redeem_codes DROP CONSTRAINT reward_test_failure`)
		require.NoError(t, err)
		_, err = s.Pay(ctx, p.Date, p.PreviewID, "10", 99)
		require.NoError(t, err)
	})
	t.Run("reject_changed_preview_stale_data_and_dates", func(t *testing.T) {
		rewardFixture(t, db)
		s := makeService()
		p, err := s.Preview(ctx, "10")
		require.NoError(t, err)
		_, err = db.Exec(`UPDATE usage_logs SET actual_cost=1 WHERE user_id=2`)
		require.NoError(t, err)
		_, err = s.Pay(ctx, p.Date, p.PreviewID, "10", 99)
		require.ErrorContains(t, err, "奖励金额已变化")
		_, err = db.Exec(`DELETE FROM usage_logs WHERE user_id=3`)
		require.NoError(t, err)
		_, err = s.Preview(ctx, "10")
		require.ErrorContains(t, err, "消费记录已变化")
		rewardFixture(t, db)
		_, err = db.Exec(`DELETE FROM custom_leaderboard.refresh_runs`)
		require.NoError(t, err)
		_, err = s.Preview(ctx, "10")
		require.ErrorContains(t, err, "尚未完成统计")
		rewardFixture(t, db)
		s.now = func() time.Time { return clock().Add(24 * time.Hour) }
		_, err = s.Pay(ctx, p.Date, p.PreviewID, "10", 99)
		require.ErrorContains(t, err, "日期已变化")
	})
	t.Run("empty_and_deleted_winners_fail_closed", func(t *testing.T) {
		rewardFixture(t, db)
		s := makeService()
		_, err := db.Exec(`UPDATE users SET deleted_at=now() WHERE id=1`)
		require.NoError(t, err)
		_, err = s.Preview(ctx, "10")
		require.Error(t, err)
		rewardFixture(t, db)
		_, err = db.Exec(`DELETE FROM custom_leaderboard.daily_user_usage`)
		require.NoError(t, err)
		p, err := s.Preview(ctx, "10")
		require.NoError(t, err)
		require.Empty(t, p.Winners)
		_, err = s.Pay(ctx, p.Date, p.PreviewID, "10", 99)
		require.ErrorContains(t, err, "暂无获奖用户")
	})
}

// @brief The reward date follows Beijing midnight even on a UTC host.
func TestLeaderboardRewardBeijingDate(t *testing.T) {
	s := &LeaderboardRewardService{now: func() time.Time { return time.Date(2026, 9, 18, 16, 0, 0, 0, time.UTC) }}
	require.Equal(t, "2026-09-18", s.yesterday())
	s.now = func() time.Time { return time.Date(2026, 9, 18, 15, 59, 59, 0, time.UTC) }
	require.Equal(t, "2026-09-17", s.yesterday())
}

// @brief Administrators can select any percentage from 0.01 through 100 with at most two decimals.
func TestNormalizeLeaderboardRewardRatePercent(t *testing.T) {
	for input, expected := range map[string]string{
		"0.01": "0.01", "1": "1.00", "10": "10.00", "12.3": "12.30", "100.00": "100.00",
	} {
		actual, err := normalizeLeaderboardRewardRatePercent(input)
		require.NoError(t, err, input)
		require.Equal(t, expected, actual, input)
	}
	for _, input := range []string{"", "0", "0.001", "100.01", "-1", "ten"} {
		_, err := normalizeLeaderboardRewardRatePercent(input)
		require.Error(t, err, input)
	}
}
