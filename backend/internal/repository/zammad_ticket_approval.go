package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type ZammadTicketApprovalRepository struct {
	db *sql.DB
}

func NewZammadTicketApprovalRepository(db *sql.DB) *ZammadTicketApprovalRepository {
	return &ZammadTicketApprovalRepository{db: db}
}

func (r *ZammadTicketApprovalRepository) GetOrCreate(ctx context.Context, ticketLinkID int64) (*service.ZammadTicketApproval, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("nil zammad approval database")
	}
	if _, err := r.db.ExecContext(ctx, `
INSERT INTO support_ticket_approvals (ticket_link_id)
VALUES ($1)
ON CONFLICT (ticket_link_id) DO NOTHING`, ticketLinkID); err != nil {
		return nil, fmt.Errorf("create zammad approval: %w", err)
	}
	return r.scanApproval(r.db.QueryRowContext(ctx, approvalSelectSQL+" WHERE ticket_link_id = $1", ticketLinkID))
}

func (r *ZammadTicketApprovalRepository) Decide(ctx context.Context, ticketLinkID, adminID int64, state, reason string) (*service.ZammadTicketApproval, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("nil zammad approval database")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin zammad approval decision: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `
INSERT INTO support_ticket_approvals (ticket_link_id)
VALUES ($1)
ON CONFLICT (ticket_link_id) DO NOTHING`, ticketLinkID); err != nil {
		return nil, fmt.Errorf("create zammad approval: %w", err)
	}
	var approval service.ZammadTicketApproval
	var approvedBy sql.NullInt64
	var approvedAt, createdAt, updatedAt sql.NullTime
	var reasonValue sql.NullString
	var executionResult []byte
	if err := tx.QueryRowContext(ctx, approvalSelectSQL+" WHERE ticket_link_id = $1 FOR UPDATE", ticketLinkID).Scan(
		&approval.ID, &approval.TicketLinkID, &approval.ApprovalState, &approvedBy,
		&approvedAt, &reasonValue, &executionResult, &createdAt, &updatedAt,
	); err != nil {
		return nil, fmt.Errorf("lock zammad approval: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE support_ticket_approvals
SET approval_state = $2, approved_by = $3, approved_at = CURRENT_TIMESTAMP,
    decision_reason = $4, updated_at = CURRENT_TIMESTAMP
WHERE ticket_link_id = $1`, ticketLinkID, state, adminID, reason); err != nil {
		return nil, fmt.Errorf("update zammad approval: %w", err)
	}
	if err := tx.QueryRowContext(ctx, approvalSelectSQL+" WHERE ticket_link_id = $1", ticketLinkID).Scan(
		&approval.ID, &approval.TicketLinkID, &approval.ApprovalState, &approvedBy,
		&approvedAt, &reasonValue, &executionResult, &createdAt, &updatedAt,
	); err != nil {
		return nil, fmt.Errorf("read zammad approval decision: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit zammad approval decision: %w", err)
	}
	setApprovalNullableFields(&approval, approvedBy, approvedAt, reasonValue, executionResult, createdAt, updatedAt)
	return &approval, nil
}

const approvalSelectSQL = `
SELECT id, ticket_link_id, approval_state, approved_by, approved_at,
       decision_reason, execution_result, created_at, updated_at
FROM support_ticket_approvals`

func (r *ZammadTicketApprovalRepository) scanApproval(row *sql.Row) (*service.ZammadTicketApproval, error) {
	var approval service.ZammadTicketApproval
	var approvedBy sql.NullInt64
	var approvedAt, createdAt, updatedAt sql.NullTime
	var reasonValue sql.NullString
	var executionResult []byte
	if err := row.Scan(
		&approval.ID, &approval.TicketLinkID, &approval.ApprovalState, &approvedBy,
		&approvedAt, &reasonValue, &executionResult, &createdAt, &updatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("load zammad approval: %w", err)
	}
	setApprovalNullableFields(&approval, approvedBy, approvedAt, reasonValue, executionResult, createdAt, updatedAt)
	return &approval, nil
}

func setApprovalNullableFields(approval *service.ZammadTicketApproval, approvedBy sql.NullInt64, approvedAt sql.NullTime, reasonValue sql.NullString, executionResult []byte, createdAt, updatedAt sql.NullTime) {
	if approvedBy.Valid {
		approval.ApprovedBy = &approvedBy.Int64
	}
	if approvedAt.Valid {
		approval.ApprovedAt = &approvedAt.Time
	}
	if reasonValue.Valid {
		approval.DecisionReason = reasonValue.String
	}
	if len(executionResult) > 0 {
		_ = json.Unmarshal(executionResult, &approval.ExecutionResult)
	}
	if createdAt.Valid {
		approval.CreatedAt = createdAt.Time
	}
	if updatedAt.Valid {
		approval.UpdatedAt = updatedAt.Time
	}
}
