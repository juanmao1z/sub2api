//go:build unit

package repository

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func migrationFilesThrough(t *testing.T, last int) fs.FS {
	t.Helper()
	result := fstest.MapFS{}
	entries, err := fs.ReadDir(migrations.FS, ".")
	require.NoError(t, err)
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".sql") {
			continue
		}
		number, err := strconv.Atoi(name[:3])
		require.NoError(t, err)
		if number > last {
			continue
		}
		data, err := migrations.FS.ReadFile(name)
		require.NoError(t, err)
		result[name] = &fstest.MapFile{Data: data}
	}
	return result
}

func verifyV0221Migration(t *testing.T, db *sql.DB) {
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
	require.Len(t, checksums, 306, "clone must be the custom v0.2.19 migration baseline")
	counts := map[string]int64{}
	for _, table := range []string{"users", "accounts", "groups", "support_tickets", "usage_logs"} {
		var count int64
		require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM `+table).Scan(&count))
		counts[table] = count
	}
	started := time.Now()
	require.NoError(t, applyMigrationsFS(ctx, db, migrationFilesThrough(t, 250)))
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
	require.Equal(t, 309, count)
	var valid, ready bool
	require.NoError(t, db.QueryRowContext(ctx, `SELECT indisvalid, indisready FROM pg_index WHERE indexrelid='idx_usage_logs_api_key_created_cost'::regclass`).Scan(&valid, &ready))
	require.True(t, valid)
	require.True(t, ready)
	var columns, tables int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM information_schema.columns WHERE table_schema='public' AND ((table_name='usage_logs' AND column_name IN ('cache_creation_30m_tokens','cache_creation_12h_tokens')) OR (table_name IN ('channel_model_pricing','channel_pricing_intervals','channel_account_stats_model_pricing','channel_account_stats_pricing_intervals') AND column_name='cache_write_12h_price'))`).Scan(&columns))
	require.Equal(t, 6, columns)
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM information_schema.tables WHERE table_schema='public' AND table_name IN ('mcp_oauth_grants','mcp_oauth_codes','mcp_oauth_credentials')`).Scan(&tables))
	require.Equal(t, 3, tables)
	require.NoError(t, applyMigrationsFS(ctx, db, migrationFilesThrough(t, 250)))
	require.NoError(t, applyMigrationsFS(ctx, db, migrationFilesThrough(t, 248)))
	t.Logf("migrations=306 -> 309; duration=%s; historical checksums and table counts preserved; index valid; TTL/MCP schema verified; repeat/rollback ledger accepted", time.Since(started))
}

func TestV0221UpgradePreservesCustomSchemaAndData(t *testing.T) {
	dsn := os.Getenv("UPGRADE_TEST_DSN")
	if dsn == "" {
		t.Skip("set UPGRADE_TEST_DSN to an isolated PostgreSQL test server")
	}
	base, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = base.Close() })
	database := fmt.Sprintf("upgrade_v0221_%d", time.Now().UnixNano())
	_, err = base.Exec(`CREATE DATABASE ` + database)
	require.NoError(t, err)
	db, err := sql.Open("postgres", dsn+" dbname="+database)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close(); _, _ = base.Exec(`DROP DATABASE ` + database + ` WITH (FORCE)`) })
	require.NoError(t, applyMigrationsFS(context.Background(), db, migrationFilesThrough(t, 248)))
	_, err = db.Exec(`INSERT INTO users (email,password_hash) VALUES ('upgrade-v0221@example.invalid','test-only'); INSERT INTO support_tickets (user_id,subject,description) SELECT id,'preserved v0221 ticket','before upgrade' FROM users WHERE email='upgrade-v0221@example.invalid'`)
	require.NoError(t, err)
	verifyV0221Migration(t, db)
}

func TestV0221UpgradeProductionClone(t *testing.T) {
	dsn := os.Getenv("UPGRADE_CLONE_TEST_DSN")
	if dsn == "" {
		t.Skip("set UPGRADE_CLONE_TEST_DSN to a disposable restored clone")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	var database string
	require.NoError(t, db.QueryRow(`SELECT current_database()`).Scan(&database))
	require.True(t, strings.HasPrefix(database, "restore_v0221_"), "refuse to migrate a non-clone database")
	verifyV0221Migration(t, db)
}
