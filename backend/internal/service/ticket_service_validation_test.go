package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateTicketStatusTransition(t *testing.T) {
	for _, tc := range []struct {
		name    string
		from    string
		to      string
		wantErr bool
	}{
		{name: "open to in progress", from: TicketStatusOpen, to: TicketStatusInProgress},
		{name: "waiting to resolved", from: TicketStatusWaitingUser, to: TicketStatusResolved},
		{name: "closed reopen", from: TicketStatusClosed, to: TicketStatusOpen},
		{name: "closed to resolved", from: TicketStatusClosed, to: TicketStatusResolved, wantErr: true},
		{name: "resolved to waiting", from: TicketStatusResolved, to: TicketStatusWaitingUser, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.wantErr, ValidateTicketStatusTransition(tc.from, tc.to) != nil)
		})
	}
}

func TestTicketReplyClientIDValidation(t *testing.T) {
	repo := &ticketValidationRepository{}
	service := NewTicketService(repo)
	_, err := service.Reply(context.Background(), 1, 1, false, "reply", string(make([]rune, 101)))
	require.Error(t, err)
}

type ticketValidationRepository struct{}

func (*ticketValidationRepository) CreateTicket(context.Context, int64, TicketCreateInput) (Ticket, error) {
	return Ticket{}, nil
}
func (*ticketValidationRepository) ListTickets(context.Context, int64, bool, TicketListFilter) (TicketPage, error) {
	return TicketPage{}, nil
}
func (*ticketValidationRepository) Stats(context.Context, int64, bool) (TicketStats, error) {
	return TicketStats{}, nil
}
func (*ticketValidationRepository) GetTicket(context.Context, int64, int64, bool, int64) (TicketDetail, error) {
	return TicketDetail{Ticket: Ticket{ID: 1}}, nil
}
func (*ticketValidationRepository) AddMessage(context.Context, int64, int64, string, string, string, bool) (TicketDetail, error) {
	return TicketDetail{}, nil
}
func (*ticketValidationRepository) UpdateTicket(context.Context, int64, int64, bool, TicketUpdateInput) (TicketDetail, error) {
	return TicketDetail{}, nil
}
func (*ticketValidationRepository) MarkRead(context.Context, int64, int64, int64) error { return nil }
func (*ticketValidationRepository) ReplaceTags(context.Context, int64, []int64) error   { return nil }
func (*ticketValidationRepository) ListTags(context.Context) ([]TicketTag, error)       { return nil, nil }
