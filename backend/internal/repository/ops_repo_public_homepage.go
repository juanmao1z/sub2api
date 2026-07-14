package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

const opsPublicHomepageQueryTimeout = 5 * time.Second

func (r *opsRepository) GetPublicHomepageStatus(ctx context.Context, now time.Time) (*service.PublicHomepageStatusSnapshot, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("nil ops repository")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	queryCtx, cancel := context.WithTimeout(ctx, opsPublicHomepageQueryTimeout)
	defer cancel()

	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return nil, fmt.Errorf("load Asia/Shanghai timezone: %w", err)
	}
	now = now.UTC()
	localNow := now.In(location)
	dayStart := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, location).UTC()
	dayEnd := dayStart.In(location).AddDate(0, 0, 1).UTC()

	snapshot := &service.PublicHomepageStatusSnapshot{}
	var startedAt time.Time
	err = r.db.QueryRowContext(queryCtx, `
SELECT created_at
FROM usage_logs
ORDER BY created_at ASC
LIMIT 1`).Scan(&startedAt)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("query homepage started_at: %w", err)
	}
	if err == nil {
		startedAt = startedAt.UTC()
		snapshot.StartedAt = &startedAt
	}

	if err := r.db.QueryRowContext(queryCtx, `
SELECT COALESCE(COUNT(DISTINCT user_id), 0)
FROM usage_logs
WHERE created_at >= $1 AND created_at < $2`, now.Add(-time.Hour), now).Scan(&snapshot.ActiveUsers1h); err != nil {
		return nil, fmt.Errorf("query homepage active users: %w", err)
	}

	filter := &service.OpsDashboardFilter{}
	successCount, _, err := r.queryUsageCounts(queryCtx, filter, dayStart, dayEnd)
	if err != nil {
		return nil, fmt.Errorf("query homepage success count: %w", err)
	}
	_, _, errorCountSLA, _, _, _, err := r.queryErrorCounts(queryCtx, filter, dayStart, dayEnd)
	if err != nil {
		return nil, fmt.Errorf("query homepage SLA errors: %w", err)
	}
	snapshot.SuccessCountToday = successCount
	snapshot.ErrorCountSLAToday = errorCountSLA

	if err := r.db.QueryRowContext(queryCtx, `
SELECT COALESCE(SUM(input_tokens + output_tokens + cache_creation_tokens + cache_read_tokens), 0)
FROM usage_dashboard_daily`).Scan(&snapshot.TotalTokens); err != nil {
		return nil, fmt.Errorf("query homepage total tokens: %w", err)
	}

	snapshot.ActiveUsers1h = max(snapshot.ActiveUsers1h, 0)
	snapshot.SuccessCountToday = max(snapshot.SuccessCountToday, 0)
	snapshot.ErrorCountSLAToday = max(snapshot.ErrorCountSLAToday, 0)
	snapshot.TotalTokens = max(snapshot.TotalTokens, 0)
	return snapshot, nil
}
