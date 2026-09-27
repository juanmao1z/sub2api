package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/integration/zammad"
)

const zammadDefaultGroup = "Users"

type ZammadTicketService struct {
	provider zammad.TicketProvider
	identity *ZammadIdentityService
	links    ZammadTicketLinkStore
	outbox   ZammadTicketSyncOutbox
}

type ZammadTicketLink struct {
	ID             int64
	LocalTicketID  *int64
	UserID         int64
	ZammadTicketID int64
	TicketType     string
	OrderID        *int64
	Contact        string
	SubjectCache   string
	StatusCache    string
	SyncState      string
	LastError      string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	LastSyncedAt   *time.Time
}

type ZammadTicketLinkStore interface {
	Create(context.Context, *ZammadTicketLink) (*ZammadTicketLink, error)
	AttachLocalTicket(context.Context, int64, int64) error
	GetByLocalTicketID(context.Context, int64) (*ZammadTicketLink, error)
	GetByID(context.Context, int64) (*ZammadTicketLink, error)
	GetByZammadTicketID(context.Context, int64) (*ZammadTicketLink, error)
	ListByUser(context.Context, int64) ([]ZammadTicketLink, error)
	UpdateCache(context.Context, int64, string, string, string, *string) error
}

type ZammadTicketSyncOutbox interface {
	Enqueue(context.Context, *int64, string, string, any) error
}

type ZammadTicketMessage struct {
	ID         int64
	TicketID   int64
	Body       string
	Internal   bool
	CreatedAt  string
	CreatedBy  int64
	SenderType string
}

func NewZammadTicketService(
	provider zammad.TicketProvider,
	identity *ZammadIdentityService,
	links ZammadTicketLinkStore,
	outbox ZammadTicketSyncOutbox,
) *ZammadTicketService {
	return &ZammadTicketService{provider: provider, identity: identity, links: links, outbox: outbox}
}

func (s *ZammadTicketService) Enabled() bool {
	return s != nil && s.provider != nil && s.provider.Configured()
}

// Create creates the Zammad ticket first, then records only its local
// association. Conversation content remains in Zammad.
func (s *ZammadTicketService) Create(ctx context.Context, userID int64, email, typ, subject, description, contact string, orderID *int64) (*ZammadTicketLink, error) {
	if s == nil || s.provider == nil || s.identity == nil || s.links == nil {
		return nil, errors.New("zammad ticket service is not configured")
	}
	if userID <= 0 {
		return nil, errors.New("sub2api user id must be positive")
	}
	typ = strings.ToUpper(strings.TrimSpace(typ))
	subject = strings.TrimSpace(subject)
	description = strings.TrimSpace(description)
	contact = strings.TrimSpace(contact)
	if subject == "" || description == "" {
		return nil, errors.New("ticket subject and description are required")
	}
	customer, err := s.identity.EnsureIdentity(ctx, userID, email)
	if err != nil {
		return nil, err
	}
	remote, err := s.provider.CreateTicket(ctx, zammad.CreateTicketRequest{
		Title:      subject,
		Group:      zammadDefaultGroup,
		CustomerID: customer.ID,
		Tags:       "sub2api," + strings.ToLower(typ),
		Article: zammad.Article{
			Subject:     subject,
			Body:        description,
			ContentType: "text/plain",
			Type:        "note",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create zammad ticket: %w", err)
	}
	link, err := s.links.Create(ctx, &ZammadTicketLink{
		UserID:         userID,
		ZammadTicketID: remote.ID,
		TicketType:     typ,
		OrderID:        orderID,
		Contact:        contact,
		SubjectCache:   subject,
		StatusCache:    remote.State,
		SyncState:      "ACTIVE",
	})
	if err != nil {
		return nil, fmt.Errorf("persist zammad ticket link: %w", err)
	}
	return link, nil
}

func (s *ZammadTicketService) Reply(ctx context.Context, link *ZammadTicketLink, body string, internal bool) error {
	if s == nil || s.provider == nil {
		return errors.New("zammad ticket service is not configured")
	}
	if link == nil || link.ZammadTicketID <= 0 {
		return errors.New("invalid zammad ticket link")
	}
	body = strings.TrimSpace(body)
	if body == "" {
		return errors.New("ticket reply is required")
	}
	_, err := s.provider.AddArticle(ctx, link.ZammadTicketID, zammad.Article{
		Body:        body,
		ContentType: "text/plain",
		Type:        "note",
		Internal:    internal,
	})
	if err != nil {
		return fmt.Errorf("add zammad ticket article: %w", err)
	}
	return nil
}

func (s *ZammadTicketService) AttachLocalTicket(ctx context.Context, link *ZammadTicketLink, localTicketID int64) error {
	if s == nil || s.links == nil || link == nil || localTicketID <= 0 {
		return errors.New("invalid local zammad ticket association")
	}
	return s.links.AttachLocalTicket(ctx, link.ID, localTicketID)
}

func (s *ZammadTicketService) LinkForLocalTicket(ctx context.Context, localTicketID int64) (*ZammadTicketLink, error) {
	if s == nil || s.links == nil {
		return nil, errors.New("zammad ticket service is not configured")
	}
	return s.links.GetByLocalTicketID(ctx, localTicketID)
}

func (s *ZammadTicketService) Messages(ctx context.Context, link *ZammadTicketLink) ([]ZammadTicketMessage, error) {
	if s == nil || s.provider == nil {
		return nil, errors.New("zammad ticket service is not configured")
	}
	if link == nil || link.ZammadTicketID <= 0 {
		return nil, errors.New("invalid zammad ticket link")
	}
	articles, err := s.provider.ListArticles(ctx, link.ZammadTicketID)
	if err != nil {
		return nil, fmt.Errorf("list zammad ticket articles: %w", err)
	}
	messages := make([]ZammadTicketMessage, 0, len(articles))
	for _, article := range articles {
		senderType := "USER"
		if article.Internal {
			senderType = "ADMIN"
		}
		messages = append(messages, ZammadTicketMessage{
			ID:         article.ID,
			TicketID:   link.ZammadTicketID,
			Body:       article.Body,
			Internal:   article.Internal,
			CreatedAt:  article.CreatedAt,
			CreatedBy:  article.CreatedBy,
			SenderType: senderType,
		})
	}
	return messages, nil
}

func (s *ZammadTicketService) SetStatus(ctx context.Context, link *ZammadTicketLink, status string) error {
	if s == nil || s.provider == nil || s.links == nil {
		return errors.New("zammad ticket service is not configured")
	}
	if link == nil || link.ZammadTicketID <= 0 {
		return errors.New("invalid zammad ticket link")
	}
	localStatus := strings.ToUpper(strings.TrimSpace(status))
	if localStatus != SupportTicketStatusOpen && localStatus != SupportTicketStatusResolved && localStatus != SupportTicketStatusClosed {
		return fmt.Errorf("invalid ticket status %s", status)
	}
	remote, err := s.provider.UpdateTicketStatus(ctx, link.ZammadTicketID, zammad.TicketStatus(localStatus))
	if err != nil {
		return fmt.Errorf("update zammad ticket status: %w", err)
	}
	link.StatusCache = localStatus
	link.SubjectCache = remote.Title
	return s.links.UpdateCache(ctx, link.ID, remote.Title, localStatus, "ACTIVE", nil)
}
