package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type ZammadIdentityMappingRepository struct {
	db *sql.DB
}

func NewZammadIdentityMappingRepository(db *sql.DB) *ZammadIdentityMappingRepository {
	return &ZammadIdentityMappingRepository{db: db}
}

func (r *ZammadIdentityMappingRepository) GetBySub2APIUserID(ctx context.Context, userID int64) (*service.ZammadIdentityMapping, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("nil zammad identity mapping database")
	}
	mapping := &service.ZammadIdentityMapping{}
	var zammadUserID sql.NullInt64
	err := r.db.QueryRowContext(ctx, `
SELECT id, sub2api_user_id, zammad_user_id, email, sync_state,
       conflict_reason, last_synced_at, created_at, updated_at
FROM support_ticket_zammad_users
WHERE sub2api_user_id = $1`, userID).Scan(
		&mapping.ID,
		&mapping.Sub2APIUserID,
		&zammadUserID,
		&mapping.Email,
		&mapping.SyncState,
		&nullableString{target: &mapping.ConflictReason},
		&nullableTime{target: &mapping.LastSyncedAt},
		&mapping.CreatedAt,
		&mapping.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load zammad identity mapping: %w", err)
	}
	if zammadUserID.Valid {
		mapping.ZammadUserID = &zammadUserID.Int64
	}
	return mapping, nil
}

func (r *ZammadIdentityMappingRepository) Upsert(ctx context.Context, mapping *service.ZammadIdentityMapping) (*service.ZammadIdentityMapping, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("nil zammad identity mapping database")
	}
	if mapping == nil || mapping.Sub2APIUserID <= 0 || mapping.ZammadUserID == nil || *mapping.ZammadUserID <= 0 {
		return nil, errors.New("invalid zammad identity mapping")
	}
	var stored service.ZammadIdentityMapping
	var zammadUserID sql.NullInt64
	err := r.db.QueryRowContext(ctx, `
INSERT INTO support_ticket_zammad_users (
    sub2api_user_id, zammad_user_id, email, sync_state, conflict_reason,
    last_synced_at, updated_at
) VALUES ($1, $2, $3, $4, NULL, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (sub2api_user_id) DO UPDATE SET
    zammad_user_id = EXCLUDED.zammad_user_id,
    email = EXCLUDED.email,
    sync_state = EXCLUDED.sync_state,
    conflict_reason = NULL,
    last_synced_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
RETURNING id, sub2api_user_id, zammad_user_id, email, sync_state,
          conflict_reason, last_synced_at, created_at, updated_at`,
		mapping.Sub2APIUserID, *mapping.ZammadUserID, mapping.Email, mapping.SyncState,
	).Scan(
		&stored.ID,
		&stored.Sub2APIUserID,
		&zammadUserID,
		&stored.Email,
		&stored.SyncState,
		&nullableString{target: &stored.ConflictReason},
		&nullableTime{target: &stored.LastSyncedAt},
		&stored.CreatedAt,
		&stored.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("save zammad identity mapping: %w", err)
	}
	if zammadUserID.Valid {
		stored.ZammadUserID = &zammadUserID.Int64
	}
	return &stored, nil
}

func (r *ZammadIdentityMappingRepository) MarkConflict(ctx context.Context, userID int64, email, reason string) error {
	if r == nil || r.db == nil {
		return errors.New("nil zammad identity mapping database")
	}
	_, err := r.db.ExecContext(ctx, `
INSERT INTO support_ticket_zammad_users (
    sub2api_user_id, zammad_user_id, email, sync_state, conflict_reason, updated_at
) VALUES ($1, NULL, $2, $3, $4, CURRENT_TIMESTAMP)
ON CONFLICT (sub2api_user_id) DO UPDATE SET
    email = EXCLUDED.email,
    sync_state = EXCLUDED.sync_state,
    conflict_reason = EXCLUDED.conflict_reason,
    updated_at = CURRENT_TIMESTAMP`,
		userID, email, service.ZammadIdentityStateConflict, reason)
	if err != nil {
		return fmt.Errorf("save zammad identity conflict: %w", err)
	}
	return nil
}

type nullableString struct {
	target *string
}

func (n *nullableString) Scan(value any) error {
	if value == nil {
		*n.target = ""
		return nil
	}
	v, ok := value.(string)
	if !ok {
		return fmt.Errorf("expected string, got %T", value)
	}
	*n.target = v
	return nil
}

type nullableTime struct {
	target **time.Time
}

func (n *nullableTime) Scan(value any) error {
	if value == nil {
		*n.target = nil
		return nil
	}
	v, ok := value.(time.Time)
	if !ok {
		return fmt.Errorf("expected time.Time, got %T", value)
	}
	*n.target = &v
	return nil
}
