package service

import (
	"context"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/supportticket"
	"github.com/Wei-Shaw/sub2api/ent/supportticketmessage"
	"github.com/Wei-Shaw/sub2api/ent/user"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	SupportTicketTypeRefund     = "REFUND"
	SupportTicketTypeSuggestion = "SUGGESTION"
	SupportTicketStatusOpen     = "OPEN"
	SupportTicketStatusClosed   = "CLOSED"
	SupportTicketStatusResolved = "RESOLVED"
)

// SupportTicketService provides persistence and workflow operations for support tickets.
type SupportTicketService struct {
	client *dbent.Client
}

// SupportTicketUser identifies the account that submitted a support ticket.
type SupportTicketUser struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

// SupportTicketAdminView combines a ticket with the submitting user's basic identity.
type SupportTicketAdminView struct {
	Ticket *dbent.SupportTicket `json:"ticket"`
	User   SupportTicketUser    `json:"user"`
}

// NewSupportTicketService creates a support ticket service.
func NewSupportTicketService(client *dbent.Client) *SupportTicketService {
	return &SupportTicketService{client: client}
}

func (s *SupportTicketService) Create(ctx context.Context, userID int64, typ, subject, description, contact string, orderID *int64) (*dbent.SupportTicket, error) {
	typ = strings.ToUpper(strings.TrimSpace(typ))
	subject = strings.TrimSpace(subject)
	description = strings.TrimSpace(description)
	contact = strings.TrimSpace(contact)
	if typ != SupportTicketTypeRefund && typ != SupportTicketTypeSuggestion {
		return nil, infraerrors.BadRequest("INVALID_TYPE", "ticket type must be REFUND or SUGGESTION")
	}
	if subject == "" || description == "" {
		return nil, infraerrors.BadRequest("INVALID_REQUEST", "subject and description are required")
	}
	if typ == SupportTicketTypeRefund {
		if orderID == nil {
			return nil, infraerrors.BadRequest("ORDER_REQUIRED", "refund ticket requires order_id")
		}
		if contact == "" {
			return nil, infraerrors.BadRequest("CONTACT_REQUIRED", "refund ticket requires contact information")
		}
	} else {
		orderID = nil
		contact = ""
	}
	b := s.client.SupportTicket.Create().SetUserID(userID).SetType(typ).SetSubject(subject).SetDescription(description).SetStatus(SupportTicketStatusOpen)
	if contact != "" {
		b.SetContact(contact)
	}
	if orderID != nil {
		b.SetOrderID(*orderID)
	}
	return b.Save(ctx)
}

// AdminList returns all tickets with basic submitting-user identity data.
func (s *SupportTicketService) AdminList(ctx context.Context) ([]*SupportTicketAdminView, error) {
	tickets, err := s.List(ctx, 0, true)
	if err != nil {
		return nil, err
	}
	return s.adminViews(ctx, tickets)
}

// AdminGet returns one ticket with basic submitting-user identity data.
func (s *SupportTicketService) AdminGet(ctx context.Context, id int64) (*SupportTicketAdminView, error) {
	ticket, err := s.Get(ctx, id, 0, true)
	if err != nil {
		return nil, err
	}
	views, err := s.adminViews(ctx, []*dbent.SupportTicket{ticket})
	if err != nil {
		return nil, err
	}
	return views[0], nil
}

func (s *SupportTicketService) adminViews(ctx context.Context, tickets []*dbent.SupportTicket) ([]*SupportTicketAdminView, error) {
	if len(tickets) == 0 {
		return []*SupportTicketAdminView{}, nil
	}
	ids := make([]int64, 0, len(tickets))
	for _, ticket := range tickets {
		ids = append(ids, ticket.UserID)
	}
	users, err := s.client.User.Query().Where(user.IDIn(ids...)).All(ctx)
	if err != nil {
		return nil, err
	}
	byID := make(map[int64]SupportTicketUser, len(users))
	for _, account := range users {
		byID[account.ID] = SupportTicketUser{ID: account.ID, Username: account.Username, Email: account.Email}
	}
	views := make([]*SupportTicketAdminView, 0, len(tickets))
	for _, ticket := range tickets {
		views = append(views, &SupportTicketAdminView{Ticket: ticket, User: byID[ticket.UserID]})
	}
	return views, nil
}

func (s *SupportTicketService) List(ctx context.Context, userID int64, admin bool) ([]*dbent.SupportTicket, error) {
	q := s.client.SupportTicket.Query().Order(dbent.Desc(supportticket.FieldCreatedAt))
	if !admin {
		q = q.Where(supportticket.UserIDEQ(userID))
	}
	return q.All(ctx)
}

func (s *SupportTicketService) Get(ctx context.Context, id, userID int64, admin bool) (*dbent.SupportTicket, error) {
	q := s.client.SupportTicket.Query().Where(supportticket.IDEQ(id))
	if !admin {
		q = q.Where(supportticket.UserIDEQ(userID))
	}
	t, err := q.Only(ctx)
	if dbent.IsNotFound(err) {
		return nil, infraerrors.NotFound("NOT_FOUND", "ticket not found")
	}
	return t, err
}

func (s *SupportTicketService) Messages(ctx context.Context, id int64) ([]*dbent.SupportTicketMessage, error) {
	return s.client.SupportTicketMessage.Query().Where(supportticketmessage.TicketIDEQ(id)).Order(dbent.Asc(supportticketmessage.FieldCreatedAt)).All(ctx)
}

func (s *SupportTicketService) AddMessage(ctx context.Context, ticketID, senderID int64, senderType, body string) (*dbent.SupportTicketMessage, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil, infraerrors.BadRequest("INVALID_REQUEST", "message body is required")
	}
	t, err := s.client.SupportTicket.Get(ctx, ticketID)
	if err != nil {
		return nil, infraerrors.NotFound("NOT_FOUND", "ticket not found")
	}
	if t.Status == SupportTicketStatusClosed {
		return nil, infraerrors.BadRequest("TICKET_CLOSED", "ticket is closed")
	}
	m, err := s.client.SupportTicketMessage.Create().SetTicketID(ticketID).SetSenderID(senderID).SetSenderType(senderType).SetBody(body).Save(ctx)
	if err != nil {
		return nil, err
	}
	_, _ = s.client.SupportTicket.UpdateOneID(ticketID).SetUpdatedAt(time.Now()).SetStatus(SupportTicketStatusOpen).Save(ctx)
	return m, nil
}

func (s *SupportTicketService) SetStatus(ctx context.Context, id int64, status string) (*dbent.SupportTicket, error) {
	status = strings.ToUpper(strings.TrimSpace(status))
	if status != SupportTicketStatusOpen && status != SupportTicketStatusClosed && status != SupportTicketStatusResolved {
		return nil, infraerrors.BadRequest("INVALID_STATUS", "invalid ticket status")
	}
	t, err := s.client.SupportTicket.UpdateOneID(id).SetStatus(status).SetUpdatedAt(time.Now()).Save(ctx)
	if dbent.IsNotFound(err) {
		return nil, infraerrors.NotFound("NOT_FOUND", "ticket not found")
	}
	return t, err
}
