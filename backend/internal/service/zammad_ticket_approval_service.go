package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	ZammadApprovalPending  = "PENDING"
	ZammadApprovalApproved = "APPROVED"
	ZammadApprovalRejected = "REJECTED"
)

type ZammadTicketApproval struct {
	ID              int64          `json:"id"`
	TicketLinkID    int64          `json:"ticket_link_id"`
	ApprovalState   string         `json:"approval_state"`
	ApprovedBy      *int64         `json:"approved_by,omitempty"`
	ApprovedAt      *time.Time     `json:"approved_at,omitempty"`
	DecisionReason  string         `json:"decision_reason,omitempty"`
	ExecutionResult map[string]any `json:"execution_result,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

type ZammadTicketApprovalRepository interface {
	GetOrCreate(context.Context, int64) (*ZammadTicketApproval, error)
	Decide(context.Context, int64, int64, string, string) (*ZammadTicketApproval, error)
}

type ZammadTicketApprovalService struct {
	repo ZammadTicketApprovalRepository
}

func NewZammadTicketApprovalService(repo ZammadTicketApprovalRepository) *ZammadTicketApprovalService {
	return &ZammadTicketApprovalService{repo: repo}
}

func (s *ZammadTicketApprovalService) Get(ctx context.Context, ticketLinkID int64) (*ZammadTicketApproval, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("zammad approval service is not configured")
	}
	if ticketLinkID <= 0 {
		return nil, errors.New("ticket link id must be positive")
	}
	return s.repo.GetOrCreate(ctx, ticketLinkID)
}

// Decide records an administrative decision only. Payment/refund execution is
// intentionally outside this service and remains a later explicit action.
func (s *ZammadTicketApprovalService) Decide(ctx context.Context, ticketLinkID, adminID int64, state, reason string) (*ZammadTicketApproval, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("zammad approval service is not configured")
	}
	if ticketLinkID <= 0 || adminID <= 0 {
		return nil, errors.New("ticket link id and admin id must be positive")
	}
	state = strings.ToUpper(strings.TrimSpace(state))
	if state != ZammadApprovalApproved && state != ZammadApprovalRejected {
		return nil, fmt.Errorf("invalid approval state %s", state)
	}
	return s.repo.Decide(ctx, ticketLinkID, adminID, state, strings.TrimSpace(reason))
}
