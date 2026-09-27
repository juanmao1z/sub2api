package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type ZammadTicketLink = service.ZammadTicketLink

type ZammadTicketLinkRepository struct {
	db *sql.DB
}

func NewZammadTicketLinkRepository(db *sql.DB) *ZammadTicketLinkRepository {
	return &ZammadTicketLinkRepository{db: db}
}

func (r *ZammadTicketLinkRepository) Create(ctx context.Context, link *ZammadTicketLink) (*ZammadTicketLink, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("nil zammad ticket link database")
	}
	if link == nil || link.UserID <= 0 || link.ZammadTicketID <= 0 || link.TicketType == "" {
		return nil, errors.New("invalid zammad ticket link")
	}
	return r.scanLink(r.db.QueryRowContext(ctx, `
INSERT INTO support_ticket_links (
    local_ticket_id, user_id, zammad_ticket_id, ticket_type, order_id, contact,
    subject_cache, status_cache, sync_state, last_error, last_synced_at, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NULL, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
RETURNING id, local_ticket_id, user_id, zammad_ticket_id, ticket_type, order_id, contact,
          subject_cache, status_cache, sync_state, last_error, created_at,
          updated_at, last_synced_at`,
		link.LocalTicketID, link.UserID, link.ZammadTicketID, link.TicketType, link.OrderID, link.Contact,
		link.SubjectCache, link.StatusCache, link.SyncState))
}

func (r *ZammadTicketLinkRepository) AttachLocalTicket(ctx context.Context, linkID, localTicketID int64) error {
	if r == nil || r.db == nil {
		return errors.New("nil zammad ticket link database")
	}
	result, err := r.db.ExecContext(ctx, `UPDATE support_ticket_links SET local_ticket_id = $2, updated_at = CURRENT_TIMESTAMP WHERE id = $1`, linkID, localTicketID)
	if err != nil {
		return fmt.Errorf("attach local support ticket: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *ZammadTicketLinkRepository) GetByID(ctx context.Context, id int64) (*ZammadTicketLink, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("nil zammad ticket link database")
	}
	return r.scanLink(r.db.QueryRowContext(ctx, linkSelectSQL+" WHERE id = $1", id), id)
}

func (r *ZammadTicketLinkRepository) GetByLocalTicketID(ctx context.Context, id int64) (*ZammadTicketLink, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("nil zammad ticket link database")
	}
	return r.scanLink(r.db.QueryRowContext(ctx, linkSelectSQL+" WHERE local_ticket_id = $1", id), id)
}

func (r *ZammadTicketLinkRepository) GetByZammadTicketID(ctx context.Context, id int64) (*ZammadTicketLink, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("nil zammad ticket link database")
	}
	return r.scanLink(r.db.QueryRowContext(ctx, linkSelectSQL+" WHERE zammad_ticket_id = $1", id), id)
}

func (r *ZammadTicketLinkRepository) ListByUser(ctx context.Context, userID int64) ([]ZammadTicketLink, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("nil zammad ticket link database")
	}
	rows, err := r.db.QueryContext(ctx, linkSelectSQL+" WHERE user_id = $1 ORDER BY created_at DESC", userID)
	if err != nil {
		return nil, fmt.Errorf("list zammad ticket links: %w", err)
	}
	defer rows.Close()
	links := make([]ZammadTicketLink, 0)
	for rows.Next() {
		link, scanErr := scanZammadTicketLink(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		links = append(links, *link)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate zammad ticket links: %w", err)
	}
	return links, nil
}

func (r *ZammadTicketLinkRepository) UpdateCache(ctx context.Context, id int64, subject, status, syncState string, lastError *string) error {
	if r == nil || r.db == nil {
		return errors.New("nil zammad ticket link database")
	}
	result, err := r.db.ExecContext(ctx, `
UPDATE support_ticket_links
SET subject_cache = $2, status_cache = $3, sync_state = $4,
    last_error = $5, last_synced_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
WHERE id = $1`, id, subject, status, syncState, lastError)
	if err != nil {
		return fmt.Errorf("update zammad ticket link cache: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *ZammadTicketLinkRepository) scanLink(row *sql.Row, _ ...int64) (*ZammadTicketLink, error) {
	link, err := scanZammadTicketLink(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load zammad ticket link: %w", err)
	}
	return link, nil
}

const linkSelectSQL = `
SELECT id, local_ticket_id, user_id, zammad_ticket_id, ticket_type, order_id, contact,
       subject_cache, status_cache, sync_state, last_error, created_at,
       updated_at, last_synced_at
FROM support_ticket_links`

func scanZammadTicketLink(row rowScanner) (*ZammadTicketLink, error) {
	link := &ZammadTicketLink{}
	var localTicketID, orderID sql.NullInt64
	var contact, lastError sql.NullString
	var lastSyncedAt sql.NullTime
	if err := row.Scan(
		&link.ID, &localTicketID, &link.UserID, &link.ZammadTicketID, &link.TicketType, &orderID,
		&contact, &link.SubjectCache, &link.StatusCache, &link.SyncState, &lastError,
		&link.CreatedAt, &link.UpdatedAt, &lastSyncedAt,
	); err != nil {
		return nil, err
	}
	if orderID.Valid {
		link.OrderID = &orderID.Int64
	}
	if localTicketID.Valid {
		link.LocalTicketID = &localTicketID.Int64
	}
	if contact.Valid {
		link.Contact = contact.String
	}
	if lastError.Valid {
		link.LastError = lastError.String
	}
	if lastSyncedAt.Valid {
		link.LastSyncedAt = &lastSyncedAt.Time
	}
	return link, nil
}

type ZammadTicketSyncOutboxRepository struct {
	db *sql.DB
}

func NewZammadTicketSyncOutboxRepository(db *sql.DB) *ZammadTicketSyncOutboxRepository {
	return &ZammadTicketSyncOutboxRepository{db: db}
}

func (r *ZammadTicketSyncOutboxRepository) Enqueue(ctx context.Context, ticketLinkID *int64, eventType, idempotencyKey string, payload any) error {
	if r == nil || r.db == nil {
		return errors.New("nil zammad outbox database")
	}
	if eventType == "" || idempotencyKey == "" {
		return errors.New("zammad outbox event type and idempotency key are required")
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode zammad outbox payload: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `
INSERT INTO support_ticket_sync_outbox (ticket_link_id, event_type, idempotency_key, payload)
VALUES ($1, $2, $3, $4)
ON CONFLICT (idempotency_key) DO NOTHING`, ticketLinkID, eventType, idempotencyKey, encoded)
	if err != nil {
		return fmt.Errorf("enqueue zammad ticket sync: %w", err)
	}
	return nil
}
