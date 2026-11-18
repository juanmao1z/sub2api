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

	"github.com/stretchr/testify/require"
)

func verifyV0222Migration(t *testing.T, db *sql.DB) {
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
	require.Len(t, checksums, 309, "clone must be the custom v0.2.21 baseline")
	counts := map[string]int64{}
	for _, table := range []string{"users", "accounts", "groups", "support_tickets", "usage_logs", "usage_group_daily_rollups"} {
		var count int64
		require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM `+table).Scan(&count))
		counts[table] = count
	}
	started := time.Now()
	require.NoError(t, applyMigrationsFS(ctx, db, migrationFilesThrough(t, 251)))
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
	var count int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM schema_migrations`).Scan(&count))
	require.Equal(t, 311, count)
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM groups WHERE force_openai_ultrafast IS DISTINCT FROM FALSE`).Scan(&count))
	require.Zero(t, count, "upgrade must not enable Ultra Fast on existing groups")
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM information_schema.columns WHERE table_schema='public' AND table_name='usage_group_rollup_state' AND column_name IN ('expired_before','building_until','build_dirty_from')`).Scan(&count))
	require.Equal(t, 3, count)
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM pg_proc WHERE pronamespace='public'::regnamespace AND proname IN ('apply_group_usage_rollup_invalidation','invalidate_group_usage_rollups_at','invalidate_group_usage_rollup_state','invalidate_group_usage_rollup_state_after_insert')`).Scan(&count))
	require.Equal(t, 4, count)
	require.NoError(t, applyMigrationsFS(ctx, db, migrationFilesThrough(t, 251)))
	require.NoError(t, applyMigrationsFS(ctx, db, migrationFilesThrough(t, 250)))
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM schema_migrations`).Scan(&count))
	require.Equal(t, 311, count)
	t.Logf("migrations=309 -> 311; duration=%s; historical checksums and critical table counts preserved; Ultra Fast default false; rollup columns/functions verified; repeat/old ledger accepted", time.Since(started))
}

func TestV0222UpgradePreservesCustomSchemaAndData(t *testing.T) {
	dsn := os.Getenv("UPGRADE_TEST_DSN")
	if dsn == "" {
		t.Skip("set UPGRADE_TEST_DSN to an isolated PostgreSQL test server")
	}
	base, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = base.Close() })
	database := fmt.Sprintf("upgrade_v0222_%d", time.Now().UnixNano())
	_, err = base.Exec(`CREATE DATABASE ` + database)
	require.NoError(t, err)
	db, err := sql.Open("postgres", dsn+" dbname="+database)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close(); _, _ = base.Exec(`DROP DATABASE ` + database + ` WITH (FORCE)`) })
	require.NoError(t, applyMigrationsFS(context.Background(), db, migrationFilesThrough(t, 250)))
	_, err = db.Exec(`INSERT INTO users (email,password_hash) VALUES ('upgrade-v0222@example.invalid','test-only'); INSERT INTO support_tickets (user_id,subject,description) SELECT id,'preserved v0222 ticket','before upgrade' FROM users WHERE email='upgrade-v0222@example.invalid'; INSERT INTO groups (name,platform,force_openai_fast) VALUES ('upgrade-v0222-fast','openai',true)`)
	require.NoError(t, err)
	verifyV0222Migration(t, db)
	var fast bool
	require.NoError(t, db.QueryRow(`SELECT force_openai_fast FROM groups WHERE name='upgrade-v0222-fast'`).Scan(&fast))
	require.True(t, fast)
	var body string
	require.NoError(t, db.QueryRow(`SELECT description FROM support_tickets WHERE subject='preserved v0222 ticket'`).Scan(&body))
	require.Equal(t, "before upgrade", body)
}

func TestV0222UpgradeProductionClone(t *testing.T) {
	dsn := os.Getenv("UPGRADE_V0222_CLONE_TEST_DSN")
	if dsn == "" {
		t.Skip("set UPGRADE_V0222_CLONE_TEST_DSN to a disposable restored clone")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	var database string
	require.NoError(t, db.QueryRow(`SELECT current_database()`).Scan(&database))
	require.True(t, strings.HasPrefix(database, "restore_v0222_"), "refuse to migrate a non-clone database")
	verifyV0222Migration(t, db)
}
