package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestOpsRepository_GetPublicHomepageStatusAggregatesConfirmedMetrics(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	now := time.Date(2026, time.July, 14, 16, 30, 0, 0, time.UTC)
	beijingDayStart := time.Date(2026, time.July, 14, 16, 0, 0, 0, time.UTC)
	beijingDayEnd := time.Date(2026, time.July, 15, 16, 0, 0, 0, time.UTC)
	startedAt := time.Date(2026, time.May, 14, 19, 50, 12, 557602000, time.UTC)

	mock.ExpectQuery(`SELECT created_at\s+FROM usage_logs\s+ORDER BY created_at ASC\s+LIMIT 1`).
		WillReturnRows(sqlmock.NewRows([]string{"created_at"}).AddRow(startedAt))
	mock.ExpectQuery(`SELECT COALESCE\(COUNT\(DISTINCT user_id\), 0\)\s+FROM usage_logs`).
		WithArgs(now.Add(-time.Hour), now).
		WillReturnRows(sqlmock.NewRows([]string{"active_users"}).AddRow(int64(10)))
	mock.ExpectQuery(`SELECT\s+COALESCE\(COUNT\(\*\), 0\) AS success_count`).
		WithArgs(beijingDayStart, beijingDayEnd).
		WillReturnRows(sqlmock.NewRows([]string{"success_count", "token_consumed"}).AddRow(int64(11429), int64(123)))
	mock.ExpectQuery(`SELECT\s+COALESCE\(COUNT\(\*\) FILTER`).
		WithArgs(beijingDayStart, beijingDayEnd).
		WillReturnRows(sqlmock.NewRows([]string{
			"error_total", "business_limited", "error_sla", "upstream_excl", "upstream_429", "upstream_529",
		}).AddRow(int64(5), int64(2), int64(3), int64(1), int64(1), int64(0)))
	mock.ExpectQuery(`SELECT COALESCE\(SUM\(input_tokens \+ output_tokens \+ cache_creation_tokens \+ cache_read_tokens\), 0\)\s+FROM usage_dashboard_daily`).
		WillReturnRows(sqlmock.NewRows([]string{"total_tokens"}).AddRow(int64(18043746148)))

	repo := &opsRepository{db: db}
	status, err := repo.GetPublicHomepageStatus(context.Background(), now)
	require.NoError(t, err)
	require.NotNil(t, status.StartedAt)
	require.Equal(t, startedAt, *status.StartedAt)
	require.Equal(t, int64(10), status.ActiveUsers1h)
	require.Equal(t, int64(11429), status.SuccessCountToday)
	require.Equal(t, int64(3), status.ErrorCountSLAToday)
	require.Equal(t, int64(18043746148), status.TotalTokens)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestOpsRepository_GetPublicHomepageStatusHandlesNoUsageRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	now := time.Date(2026, time.July, 14, 16, 30, 0, 0, time.UTC)
	mock.ExpectQuery(`SELECT created_at`).WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(`SELECT COALESCE\(COUNT\(DISTINCT user_id\), 0\)`).
		WillReturnRows(sqlmock.NewRows([]string{"active_users"}).AddRow(int64(0)))
	mock.ExpectQuery(`SELECT\s+COALESCE\(COUNT\(\*\), 0\) AS success_count`).
		WillReturnRows(sqlmock.NewRows([]string{"success_count", "token_consumed"}).AddRow(int64(0), int64(0)))
	mock.ExpectQuery(`SELECT\s+COALESCE\(COUNT\(\*\) FILTER`).
		WillReturnRows(sqlmock.NewRows([]string{
			"error_total", "business_limited", "error_sla", "upstream_excl", "upstream_429", "upstream_529",
		}).AddRow(int64(0), int64(0), int64(0), int64(0), int64(0), int64(0)))
	mock.ExpectQuery(`SELECT COALESCE\(SUM\(input_tokens`).
		WillReturnRows(sqlmock.NewRows([]string{"total_tokens"}).AddRow(int64(0)))

	status, err := (&opsRepository{db: db}).GetPublicHomepageStatus(context.Background(), now)
	require.NoError(t, err)
	require.Nil(t, status.StartedAt)
	require.Zero(t, status.ActiveUsers1h)
	require.Zero(t, status.SuccessCountToday)
	require.Zero(t, status.ErrorCountSLAToday)
	require.Zero(t, status.TotalTokens)
	require.NoError(t, mock.ExpectationsWereMet())
}
