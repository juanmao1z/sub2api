package service

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// @brief Fixed timezone and percentage for the site's completed daily token leaderboard.
const leaderboardRewardTimezone = "Asia/Shanghai"

// @brief One published winner and their exact decimal balance spend and reward.
type LeaderboardRewardWinner struct {
	Rank     int    `json:"rank"`
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	Tokens   int64  `json:"tokens"`
	Requests int64  `json:"requests"`
	Spend    string `json:"spend"`
	Amount   string `json:"amount"`
}

// @brief Immutable paid receipt or read-only preview; decimal amounts are strings to avoid rounding in transit.
type LeaderboardRewardPreview struct {
	Date        string                    `json:"date"`
	Timezone    string                    `json:"timezone"`
	Rate        string                    `json:"rate"`
	RatePercent string                    `json:"rate_percent,omitempty"`
	PreviewID   string                    `json:"preview_id"`
	Paid        bool                      `json:"paid"`
	PaidAt      *time.Time                `json:"paid_at,omitempty"`
	ActorID     int64                     `json:"actor_id,omitempty"`
	Winners     []LeaderboardRewardWinner `json:"winners"`
}

// @brief Coordinate daily credits with published rankings and the main application's caches.
type LeaderboardRewardService struct {
	db      *sql.DB
	billing BillingCache
	auth    APIKeyAuthCacheInvalidator
	now     func() time.Time
}

// @brief Construct the reward service; all credits go through a single PostgreSQL transaction.
func NewLeaderboardRewardService(db *sql.DB, billing BillingCache, auth APIKeyAuthCacheInvalidator) *LeaderboardRewardService {
	return &LeaderboardRewardService{db: db, billing: billing, auth: auth, now: time.Now}
}

// @brief Resolve yesterday using the agreed Beijing calendar, independently of the server timezone.
func (s *LeaderboardRewardService) yesterday() string {
	return s.now().In(time.FixedZone(leaderboardRewardTimezone, 8*60*60)).AddDate(0, 0, -1).Format("2006-01-02")
}

// @brief Return the recorded payment or a preview of yesterday's three published winners.
// @param ratePercent Administrator-selected percentage in the inclusive 0.01-100 range.
func (s *LeaderboardRewardService) Preview(ctx context.Context, ratePercent string) (*LeaderboardRewardPreview, error) {
	ratePercent, err := normalizeLeaderboardRewardRatePercent(ratePercent)
	if err != nil {
		return nil, err
	}
	tx, err := s.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	day := s.yesterday()
	if receipt, err := rewardReceipt(ctx, tx, day); err != nil || receipt != nil {
		return receipt, err
	}
	return s.preview(ctx, tx, day, ratePercent)
}

// @brief Lock against other payouts and the leaderboard's aggregate refresh transaction.
func (s *LeaderboardRewardService) begin(ctx context.Context) (*sql.Tx, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(193707, 1)`); err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	return tx, nil
}

// @brief Read a durable receipt even on retries after midnight; missing dates return nil.
func rewardReceipt(ctx context.Context, tx *sql.Tx, day string) (*LeaderboardRewardPreview, error) {
	var raw []byte
	err := tx.QueryRowContext(ctx, `SELECT receipt FROM daily_leaderboard_rewards WHERE reward_date=$1::date`, day).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var receipt LeaderboardRewardPreview
	if err = json.Unmarshal(raw, &receipt); err != nil {
		return nil, err
	}
	return &receipt, nil
}

// @brief Reject incomplete aggregation or removed raw usage, and hash exactly the displayed settlement.
func (s *LeaderboardRewardService) preview(ctx context.Context, tx *sql.Tx, day, ratePercent string) (*LeaderboardRewardPreview, error) {
	var available bool
	if err := tx.QueryRowContext(ctx, `SELECT to_regclass('custom_leaderboard.daily_user_usage') IS NOT NULL AND to_regclass('custom_leaderboard.refresh_runs') IS NOT NULL`).Scan(&available); err != nil {
		return nil, err
	}
	if !available {
		return nil, infraerrors.Conflict("REWARD_NOT_READY", "排行榜统计尚未就绪，请稍后重试")
	}
	var fresh bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM custom_leaderboard.refresh_runs
 WHERE bucket_date=$1::date AND status='success' AND message='tokens-v2:Asia/Shanghai'
 AND finished_at >= (($1::date + 1)::timestamp AT TIME ZONE 'Asia/Shanghai'))`, day).Scan(&fresh); err != nil {
		return nil, err
	}
	if !fresh {
		return nil, infraerrors.Conflict("REWARD_NOT_READY", "昨日榜单尚未完成统计，请稍后重试")
	}
	rows, err := tx.QueryContext(ctx, leaderboardRewardWinnersSQL, day, ratePercent)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	preview := &LeaderboardRewardPreview{
		Date: day, Timezone: leaderboardRewardTimezone,
		Rate: leaderboardRewardRateFraction(ratePercent), RatePercent: ratePercent,
		Winners: []LeaderboardRewardWinner{},
	}
	for rows.Next() {
		var winner LeaderboardRewardWinner
		var intact bool
		if err := rows.Scan(&winner.UserID, &winner.Username, &winner.Tokens, &winner.Requests, &winner.Spend, &winner.Amount, &intact); err != nil {
			return nil, err
		}
		if !intact {
			return nil, infraerrors.Conflict("REWARD_DATA_CHANGED", "获奖用户或消费记录已变化，请等待榜单重新统计")
		}
		winner.Rank = len(preview.Winners) + 1
		preview.Winners = append(preview.Winners, winner)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(preview)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(encoded)
	preview.PreviewID = hex.EncodeToString(digest[:])
	return preview, nil
}

// @brief Atomically credit a preview once per date. A retry returns the original durable receipt.
// @param day A YYYY-MM-DD date from Preview; unsettled dates must still be yesterday.
// @param previewID Digest supplied by Preview; a changed settlement requires a fresh preview.
// @param ratePercent Percentage included in the preview digest and permanent receipt.
// @param actorID Authenticated administrator recorded in the permanent receipt.
func (s *LeaderboardRewardService) Pay(ctx context.Context, day, previewID, ratePercent string, actorID int64) (*LeaderboardRewardPreview, error) {
	parsed, err := time.Parse("2006-01-02", day)
	if err != nil || parsed.Format("2006-01-02") != day || actorID <= 0 || len(previewID) != 64 {
		return nil, infraerrors.BadRequest("INVALID_REWARD_REQUEST", "奖励请求无效，请刷新页面")
	}
	ratePercent, err = normalizeLeaderboardRewardRatePercent(ratePercent)
	if err != nil {
		return nil, err
	}
	tx, err := s.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	receipt, err := rewardReceipt(ctx, tx, day)
	if err != nil {
		return nil, err
	}
	if receipt != nil {
		_ = tx.Rollback()
		s.invalidate(receipt)
		return receipt, nil
	}
	if day != s.yesterday() {
		return nil, infraerrors.Conflict("REWARD_DATE_CHANGED", "日期已变化，请刷新昨日奖励")
	}
	preview, err := s.preview(ctx, tx, day, ratePercent)
	if err != nil {
		return nil, err
	}
	if preview.PreviewID != previewID {
		return nil, infraerrors.Conflict("REWARD_PREVIEW_CHANGED", "奖励金额已变化，请刷新后重新核对")
	}
	if len(preview.Winners) == 0 {
		return nil, infraerrors.Conflict("REWARD_EMPTY", "昨日暂无获奖用户")
	}
	for _, winner := range preview.Winners {
		result, err := tx.ExecContext(ctx, `UPDATE users SET balance=balance+$1::numeric, updated_at=now() WHERE id=$2 AND deleted_at IS NULL`, winner.Amount, winner.UserID)
		if err != nil {
			return nil, err
		}
		count, err := result.RowsAffected()
		if err != nil {
			return nil, err
		}
		if count != 1 {
			return nil, infraerrors.Conflict("REWARD_USER_UNAVAILABLE", "获奖用户已不可用")
		}
		// The used adjustment is visible in existing balance history and does not trigger affiliate recharge rebates.
		if _, err = tx.ExecContext(ctx, `INSERT INTO redeem_codes(code,type,value,status,used_by,used_at,notes)
   SELECT $1,'admin_balance',$2::numeric,'used',$3,now(),$4 WHERE $2::numeric>0`,
			fmt.Sprintf("LR%s-%d", parsed.Format("20060102"), winner.UserID), winner.Amount, winner.UserID,
			fmt.Sprintf("%s 日榜第%d名消费返还%s%%（管理员 #%d）", day, winner.Rank, ratePercent, actorID)); err != nil {
			return nil, err
		}
	}
	paidAt := s.now().UTC()
	preview.Paid = true
	preview.PaidAt = &paidAt
	preview.ActorID = actorID
	encoded, err := json.Marshal(preview)
	if err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO daily_leaderboard_rewards(reward_date,actor_id,receipt) VALUES($1::date,$2,$3::jsonb)`, day, actorID, string(encoded)); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	s.invalidate(preview)
	return preview, nil
}

// @brief Invalidate cached balances after commit and again on replay, without rolling back a paid receipt.
func (s *LeaderboardRewardService) invalidate(receipt *LeaderboardRewardPreview) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, winner := range receipt.Winners {
		if s.auth != nil {
			s.auth.InvalidateAuthCacheByUserID(ctx, winner.UserID)
		}
		if s.billing != nil {
			if err := s.billing.InvalidateUserBalance(ctx, winner.UserID); err != nil {
				slog.Error("leaderboard reward balance cache invalidation failed", "user_id", winner.UserID, "error", err)
			}
		}
	}
}

// @brief Match published token ordering and use decimal balance-billed consumption only.
// Validate raw request/token totals so deletion or delayed writes cannot silently underpay a winner.
const leaderboardRewardWinnersSQL = `WITH winners AS (
 SELECT user_id,total_tokens,request_count FROM custom_leaderboard.daily_user_usage
 WHERE bucket_date=$1::date ORDER BY total_tokens DESC,request_count DESC,user_id ASC LIMIT 3
)
SELECT w.user_id,COALESCE(NULLIF(u.username,''),u.email,''),w.total_tokens,w.request_count,
 GREATEST(cost.spend,0)::text,ROUND(GREATEST(cost.spend,0)*$2::numeric/100,8)::text,
 (u.id IS NOT NULL AND u.deleted_at IS NULL AND cost.requests=w.request_count AND cost.tokens=w.total_tokens)
FROM winners w LEFT JOIN users u ON u.id=w.user_id
CROSS JOIN LATERAL (
 SELECT COALESCE(SUM(actual_cost) FILTER (WHERE billing_type=0),0) AS spend,COUNT(*) AS requests,
 COALESCE(SUM(COALESCE(input_tokens,0)::bigint+COALESCE(output_tokens,0)
 +CASE WHEN COALESCE(cache_creation_tokens,0)>0 THEN cache_creation_tokens::bigint
 ELSE GREATEST(COALESCE(cache_creation_5m_tokens,0),0)::bigint+GREATEST(COALESCE(cache_creation_1h_tokens,0),0)::bigint END
 +COALESCE(cache_read_tokens,0)+COALESCE(image_input_tokens,0)+COALESCE(image_output_tokens,0)),0) AS tokens
 FROM usage_logs WHERE user_id=w.user_id
 AND created_at>=($1::date::timestamp AT TIME ZONE 'Asia/Shanghai')
 AND created_at<(($1::date+1)::timestamp AT TIME ZONE 'Asia/Shanghai')
) cost ORDER BY w.total_tokens DESC,w.request_count DESC,w.user_id ASC`

// @brief Normalize an administrator-entered percentage to two decimals in the inclusive 0.01-100 range.
func normalizeLeaderboardRewardRatePercent(value string) (string, error) {
	value = strings.TrimSpace(value)
	parts := strings.Split(value, ".")
	if len(parts) > 2 || parts[0] == "" || len(parts) == 2 && len(parts[1]) > 2 {
		return "", infraerrors.BadRequest("INVALID_REWARD_RATE", "奖励比例必须在 0.01% 到 100% 之间，最多两位小数")
	}
	for _, part := range parts {
		if part == "" && len(parts) == 2 {
			continue
		}
		if _, err := strconv.Atoi(part); err != nil {
			return "", infraerrors.BadRequest("INVALID_REWARD_RATE", "奖励比例必须在 0.01% 到 100% 之间，最多两位小数")
		}
	}
	whole, _ := strconv.Atoi(parts[0])
	fraction := 0
	if len(parts) == 2 && parts[1] != "" {
		fraction, _ = strconv.Atoi(parts[1] + strings.Repeat("0", 2-len(parts[1])))
	}
	cents := whole*100 + fraction
	if cents < 1 || cents > 10000 {
		return "", infraerrors.BadRequest("INVALID_REWARD_RATE", "奖励比例必须在 0.01% 到 100% 之间，最多两位小数")
	}
	return fmt.Sprintf("%d.%02d", cents/100, cents%100), nil
}

// @brief Convert a normalized percentage into the decimal fraction retained for API compatibility.
func leaderboardRewardRateFraction(ratePercent string) string {
	whole, _ := strconv.Atoi(strings.ReplaceAll(ratePercent, ".", ""))
	if whole == 10000 {
		return "1.0000"
	}
	return fmt.Sprintf("0.%04d", whole)
}
