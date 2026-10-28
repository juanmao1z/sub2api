//go:build unit

package repository

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

// This test requires an explicitly selected disposable database, never production.
func TestV0219UpgradePreservesCustomSchemaAndData(t *testing.T) {
	dsn := os.Getenv("UPGRADE_TEST_DSN")
	if dsn == "" {
		t.Skip("set UPGRADE_TEST_DSN to an isolated PostgreSQL database")
	}
	base, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = base.Close() })
	database := fmt.Sprintf("upgrade_v0219_%d", time.Now().UnixNano())
	_, err = base.Exec(`CREATE DATABASE ` + database)
	require.NoError(t, err)
	db, err := sql.Open("postgres", dsn+" dbname="+database)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = db.Close()
		_, _ = base.Exec(`DROP DATABASE ` + database + ` WITH (FORCE)`)
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Preserve every historical custom migration; only 246–248 are new.
	baseline := fstest.MapFS{}
	entries, err := fs.ReadDir(migrations.FS, ".")
	require.NoError(t, err)
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".sql") || strings.HasPrefix(name, "246_") || strings.HasPrefix(name, "247_") || strings.HasPrefix(name, "248_") {
			continue
		}
		data, readErr := migrations.FS.ReadFile(name)
		require.NoError(t, readErr)
		baseline[name] = &fstest.MapFile{Data: data}
	}
	require.NoError(t, applyMigrationsFS(ctx, db, baseline))
	var beforeCount int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM schema_migrations`).Scan(&beforeCount))
	_, err = db.Exec(`CREATE TABLE upgrade_ledger_snapshot AS SELECT filename, checksum FROM schema_migrations;
		INSERT INTO users (email, password_hash) VALUES ('upgrade@example.invalid', 'test-only');
		INSERT INTO support_tickets (user_id, subject, description)
		 SELECT id, 'preserved ticket', 'before upgrade' FROM users WHERE email='upgrade@example.invalid';
		INSERT INTO groups (name, platform, allow_image_generation) VALUES
		 ('upgrade-gemini', 'gemini', false),
		 ('upgrade-antigravity', 'antigravity', false),
		 ('upgrade-openai', 'openai', false),
		 ('upgrade-composite', 'composite', false),
		 ('upgrade-enabled', 'gemini', true);
		INSERT INTO groups (name, platform, allow_image_generation, deleted_at)
		 VALUES ('upgrade-deleted', 'gemini', false, now());`)
	require.NoError(t, err)
	require.NoError(t, ApplyMigrations(ctx, db))

	for name, enabled := range map[string]bool{
		"upgrade-gemini": true, "upgrade-antigravity": true,
		"upgrade-openai": false, "upgrade-composite": false,
		"upgrade-enabled": true, "upgrade-deleted": true,
	} {
		var actual, countBilling bool
		require.NoError(t, db.QueryRow(`SELECT allow_image_generation, image_count_billing FROM groups WHERE name=$1`, name).Scan(&actual, &countBilling))
		require.Equal(t, enabled, actual, name)
		require.False(t, countBilling, name)
	}
	var ticketBody string
	require.NoError(t, db.QueryRow(`SELECT description FROM support_tickets WHERE subject='preserved ticket'`).Scan(&ticketBody))
	require.Equal(t, "before upgrade", ticketBody)
	var changedChecksums, afterCount int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM upgrade_ledger_snapshot old LEFT JOIN schema_migrations current USING (filename)
		WHERE current.checksum IS DISTINCT FROM old.checksum`).Scan(&changedChecksums))
	require.Zero(t, changedChecksums)
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM schema_migrations`).Scan(&afterCount))
	require.Equal(t, beforeCount+3, afterCount)

	// Exercise the new table and FK rather than just checking its existence.
	_, err = db.Exec(`INSERT INTO scoped_admin_api_keys (id, name, key_prefix, key_hash, created_by)
		SELECT 'upgrade-test-key', 'test only', 'test', repeat('0',64), id FROM users WHERE email='upgrade@example.invalid'`)
	require.NoError(t, err)
	var deletedAt sql.NullTime
	require.NoError(t, db.QueryRow(`SELECT deleted_at FROM scoped_admin_api_keys WHERE id='upgrade-test-key'`).Scan(&deletedAt))
	require.False(t, deletedAt.Valid)

	// Both rerunning the new migration set and checking the old set must accept
	// the ledger. Image rollback leaves new schema and data in place.
	require.NoError(t, ApplyMigrations(ctx, db))
	require.NoError(t, applyMigrationsFS(ctx, db, baseline))
	var rerunCount int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM schema_migrations`).Scan(&rerunCount))
	require.Equal(t, afterCount, rerunCount)
	t.Logf("migrations=%d -> %d; checksums unchanged; ticket preserved; image switches and scoped-key schema verified", beforeCount, afterCount)
}
