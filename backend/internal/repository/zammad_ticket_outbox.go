package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// ZammadOutboxItem is a single external synchronization command.
type ZammadOutboxItem struct {
	ID             int64
	TicketLinkID   *int64
	EventType      string
	IdempotencyKey string
	Payload        json.RawMessage
	Attempts       int
	NextAttemptAt  time.Time
	LockedAt       *time.Time
	LockedBy       *string
	CreatedAt      time.Time
}

// ZammadTicketOutboxRepository claims work with PostgreSQL row locks.
// Application-level mutexes are intentionally not used because deployments may
// run multiple Sub2API instances.
type ZammadTicketOutboxRepository struct {
	db        *sql.DB
	lockLease time.Duration
}

func NewZammadTicketOutboxRepository(db *sql.DB) *ZammadTicketOutboxRepository {
	return &ZammadTicketOutboxRepository{db: db, lockLease: 5 * time.Minute}
}

func (r *ZammadTicketOutboxRepository) Claim(ctx context.Context, workerID string, limit int) ([]ZammadOutboxItem, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("nil zammad outbox database")
	}
	if workerID == "" {
		return nil, errors.New("zammad outbox worker id is required")
	}
	if limit <= 0 {
		return []ZammadOutboxItem{}, nil
	}
	leaseSeconds := int64(r.lockLease / time.Second)
	if leaseSeconds <= 0 {
		leaseSeconds = 300
	}
	const query = `
WITH candidates AS (
    SELECT id
    FROM support_ticket_sync_outbox
    WHERE processed_at IS NULL
      AND next_attempt_at <= CURRENT_TIMESTAMP
      AND (locked_at IS NULL OR locked_at < CURRENT_TIMESTAMP - ($3 * INTERVAL '1 second'))
    ORDER BY id
    LIMIT $2
    FOR UPDATE SKIP LOCKED
)
UPDATE support_ticket_sync_outbox AS outbox
SET locked_at = CURRENT_TIMESTAMP,
    locked_by = $1,
    attempts = outbox.attempts + 1,
    updated_at = CURRENT_TIMESTAMP
FROM candidates
WHERE outbox.id = candidates.id
RETURNING outbox.id, outbox.ticket_link_id, outbox.event_type,
          outbox.idempotency_key, outbox.payload, outbox.attempts,
          outbox.next_attempt_at, outbox.locked_at, outbox.locked_by,
          outbox.created_at`
	rows, err := r.db.QueryContext(ctx, query, workerID, limit, leaseSeconds)
	if err != nil {
		return nil, fmt.Errorf("claim zammad outbox: %w", err)
	}
	defer rows.Close()
	items := make([]ZammadOutboxItem, 0, limit)
	for rows.Next() {
		var item ZammadOutboxItem
		if err := rows.Scan(
			&item.ID,
			&item.TicketLinkID,
			&item.EventType,
			&item.IdempotencyKey,
			&item.Payload,
			&item.Attempts,
			&item.NextAttemptAt,
			&item.LockedAt,
			&item.LockedBy,
			&item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan zammad outbox item: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate zammad outbox: %w", err)
	}
	return items, nil
}

func (r *ZammadTicketOutboxRepository) Complete(ctx context.Context, id int64, workerID string) error {
	if r == nil || r.db == nil {
		return errors.New("nil zammad outbox database")
	}
	result, err := r.db.ExecContext(ctx, `
UPDATE support_ticket_sync_outbox
SET processed_at = CURRENT_TIMESTAMP, locked_at = NULL, locked_by = NULL, updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND locked_by = $2 AND processed_at IS NULL`, id, workerID)
	if err != nil {
		return fmt.Errorf("complete zammad outbox item: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *ZammadTicketOutboxRepository) Fail(ctx context.Context, id int64, workerID string, nextAttemptAt time.Time, cause error) error {
	if r == nil || r.db == nil {
		return errors.New("nil zammad outbox database")
	}
	message := "unknown zammad outbox failure"
	if cause != nil {
		message = cause.Error()
	}
	result, err := r.db.ExecContext(ctx, `
UPDATE support_ticket_sync_outbox
SET locked_at = NULL, locked_by = NULL, next_attempt_at = $3, last_error = $4, updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND locked_by = $2 AND processed_at IS NULL`, id, workerID, nextAttemptAt, message)
	if err != nil {
		return fmt.Errorf("fail zammad outbox item: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}
