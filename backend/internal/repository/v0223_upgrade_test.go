//go:build unit

package repository

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func verifyV0223Migration(t *testing.T, db *sql.DB) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	rows, err := db.QueryContext(ctx, `SELECT filename, checksum FROM schema_migrations`)
	require.NoError(t, err)
	checksums := map[string]string{}
	for rows.Next() {
		var filename, checksum string
		require.NoError(t, rows.Scan(&filename, &checksum))
		checksums[filename] = checksum
	}
	require.NoError(t, rows.Err())
	require.NoError(t, rows.Close())
	require.Len(t, checksums, 311, "clone must be the custom v0.2.22 baseline")
	counts := map[string]int64{}
	for _, table := range []string{"users", "accounts", "groups", "support_tickets", "usage_logs", "usage_group_daily_rollups", "channels", "channel_model_pricing"} {
		var count int64
		require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM `+table).Scan(&count))
		counts[table] = count
	}
	var pricesBefore, groupsBefore string
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COALESCE(jsonb_agg(to_jsonb(p) ORDER BY id),'[]'::jsonb)::text FROM channel_model_pricing p`).Scan(&pricesBefore))
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COALESCE(jsonb_agg(jsonb_build_array(id,force_openai_fast,force_openai_ultrafast) ORDER BY id),'[]'::jsonb)::text FROM groups`).Scan(&groupsBefore))
	started := time.Now()
	require.NoError(t, ApplyMigrations(ctx, db))
	for filename, old := range checksums {
		var current string
		require.NoError(t, db.QueryRowContext(ctx, `SELECT checksum FROM schema_migrations WHERE filename=$1`, filename).Scan(&current))
		require.Equal(t, old, current, filename)
	}
	for table, old := range counts {
		var current int64
		require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM `+table).Scan(&current))
		require.Equal(t, old, current, table)
	}
	var pricesAfter, groupsAfter string
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COALESCE(jsonb_agg(to_jsonb(p)-'ultrafast_multiplier' ORDER BY id),'[]'::jsonb)::text FROM channel_model_pricing p`).Scan(&pricesAfter))
	require.Equal(t, pricesBefore, pricesAfter, "all existing channel price fields must remain unchanged")
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COALESCE(jsonb_agg(jsonb_build_array(id,force_openai_fast,force_openai_ultrafast) ORDER BY id),'[]'::jsonb)::text FROM groups`).Scan(&groupsAfter))
	require.Equal(t, groupsBefore, groupsAfter, "migration must not enable or disable group policies")
	var count int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM schema_migrations`).Scan(&count))
	require.Equal(t, 312, count)
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM channel_model_pricing WHERE ultrafast_multiplier IS NOT NULL`).Scan(&count))
	require.Zero(t, count, "existing prices must use model-default Ultra Fast pricing")
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM pg_constraint WHERE conrelid='channel_model_pricing'::regclass AND conname='channel_model_pricing_ultrafast_multiplier_positive' AND convalidated`).Scan(&count))
	require.Equal(t, 1, count)
	require.NoError(t, ApplyMigrations(ctx, db))
	require.NoError(t, applyMigrationsFS(ctx, db, migrationFilesThrough(t, 251)))
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM schema_migrations`).Scan(&count))
	require.Equal(t, 312, count)
	t.Logf("migrations=311 -> 312; duration=%s; historical checksums, critical row counts, channel prices and group policies preserved; NULL default, positive constraint, repeat and old ledger verified", time.Since(started))
}

func TestV0223UpgradePreservesCustomSchemaAndData(t *testing.T) {
	dsn := os.Getenv("UPGRADE_TEST_DSN")
	if dsn == "" {
		t.Skip("set UPGRADE_TEST_DSN to an isolated PostgreSQL test server")
	}
	base, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = base.Close() })
	database := fmt.Sprintf("upgrade_v0223_%d", time.Now().UnixNano())
	_, err = base.Exec(`CREATE DATABASE ` + database)
	require.NoError(t, err)
	db, err := sql.Open("postgres", dsn+" dbname="+database)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close(); _, _ = base.Exec(`DROP DATABASE ` + database + ` WITH (FORCE)`) })
	require.NoError(t, applyMigrationsFS(context.Background(), db, migrationFilesThrough(t, 251)))
	_, err = db.Exec(`INSERT INTO users (email,password_hash) VALUES ('upgrade-v0223@example.invalid','test-only'); INSERT INTO support_tickets (user_id,subject,description) SELECT id,'preserved v0223 ticket','before upgrade' FROM users WHERE email='upgrade-v0223@example.invalid'; INSERT INTO groups (name,platform,force_openai_fast) VALUES ('upgrade-v0223-fast','openai',true)`)
	require.NoError(t, err)
	var channelID, pricingID int64
	require.NoError(t, db.QueryRow(`INSERT INTO channels (name) VALUES ('upgrade-v0223-channel') RETURNING id`).Scan(&channelID))
	require.NoError(t, db.QueryRow(`INSERT INTO channel_model_pricing (channel_id,models,fast_multiplier) VALUES ($1,'["gpt-6-astra"]',2.5) RETURNING id`, channelID).Scan(&pricingID))
	verifyV0223Migration(t, db)
	var body string
	require.NoError(t, db.QueryRow(`SELECT description FROM support_tickets WHERE subject='preserved v0223 ticket'`).Scan(&body))
	require.Equal(t, "before upgrade", body)
	for _, invalid := range []float64{0, -1} {
		_, err = db.Exec(`UPDATE channel_model_pricing SET ultrafast_multiplier=$1 WHERE id=$2`, invalid, pricingID)
		var pgErr *pq.Error
		require.ErrorAs(t, err, &pgErr)
		require.Equal(t, pq.ErrorCode("23514"), pgErr.Code)
	}
	_, err = db.Exec(`UPDATE channel_model_pricing SET ultrafast_multiplier=1.25 WHERE id=$1`, pricingID)
	require.NoError(t, err)
	require.NoError(t, ApplyMigrations(context.Background(), db))
	var multiplier float64
	require.NoError(t, db.QueryRow(`SELECT ultrafast_multiplier FROM channel_model_pricing WHERE id=$1`, pricingID).Scan(&multiplier))
	require.Equal(t, 1.25, multiplier, "repeat migration must preserve explicit pricing")
}

func TestV0223UpgradeProductionClone(t *testing.T) {
	dsn := os.Getenv("UPGRADE_V0223_CLONE_TEST_DSN")
	if dsn == "" {
		t.Skip("set UPGRADE_V0223_CLONE_TEST_DSN to a disposable restored clone")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	var database string
	require.NoError(t, db.QueryRow(`SELECT current_database()`).Scan(&database))
	require.True(t, strings.HasPrefix(database, "restore_v0223_"), "refuse to migrate a non-clone database")
	verifyV0223Migration(t, db)
}
