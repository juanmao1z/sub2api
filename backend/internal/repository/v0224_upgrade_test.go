//go:build unit

package repository

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// v0.2.24 adds no migrations. Run the actual startup migration entry on a restored
// clone and ensure both its ledger and configuration/data remain unchanged.
func TestV0224UpgradeProductionClone(t *testing.T) {
	dsn := os.Getenv("UPGRADE_V0224_CLONE_TEST_DSN")
	if dsn == "" {
		t.Skip("set UPGRADE_V0224_CLONE_TEST_DSN to a disposable restored clone")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	var database string
	require.NoError(t, db.QueryRowContext(ctx, `SELECT current_database()`).Scan(&database))
	require.True(t, strings.HasPrefix(database, "restore_v0224_"), "refuse to migrate a non-clone database")
	var migrations int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM schema_migrations`).Scan(&migrations))
	require.Equal(t, 312, migrations)
	counts := map[string]int64{}
	for _, table := range []string{"users", "accounts", "groups", "support_tickets", "usage_logs", "usage_group_daily_rollups", "channels", "channel_model_pricing", "user_platform_quotas", "settings"} {
		var count int64
		require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM `+table).Scan(&count))
		counts[table] = count
	}
	// Compare digests rather than printing private configuration or user data.
	digests := map[string]string{}
	for _, table := range []string{"schema_migrations", "users", "groups", "support_tickets", "channel_model_pricing", "user_platform_quotas", "settings"} {
		var digest string
		require.NoError(t, db.QueryRowContext(ctx, `SELECT md5(COALESCE(string_agg(to_jsonb(t)::text,',' ORDER BY to_jsonb(t)::text),'')) FROM `+table+` t`).Scan(&digest))
		digests[table] = digest
	}
	started := time.Now()
	for range 2 {
		require.NoError(t, ApplyMigrations(ctx, db))
	}
	for table, before := range counts {
		var after int64
		require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM `+table).Scan(&after))
		require.Equal(t, before, after, table)
	}
	for table, before := range digests {
		var after string
		require.NoError(t, db.QueryRowContext(ctx, `SELECT md5(COALESCE(string_agg(to_jsonb(t)::text,',' ORDER BY to_jsonb(t)::text),'')) FROM `+table+` t`).Scan(&after))
		require.Equal(t, before, after, table)
	}
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM schema_migrations`).Scan(&migrations))
	require.Equal(t, 312, migrations)
	t.Logf("migrations=312 -> 312; duration=%s; repeated startup preserves ledger checksums, critical row counts, user records, tickets, channel pricing, quotas and settings", time.Since(started))
}
