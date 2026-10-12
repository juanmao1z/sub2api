package service

import (
	"context"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	TicketCategoryBilling = "billing"
	TicketCategoryAPI     = "api"
	TicketCategoryAccount = "account"
	TicketCategoryOther   = "other"

	TicketPriorityLow    = "low"
	TicketPriorityNormal = "normal"
	TicketPriorityHigh   = "high"
	TicketPriorityUrgent = "urgent"

	TicketStatusOpen        = "open"
	TicketStatusInProgress  = "in_progress"
	TicketStatusWaitingUser = "waiting_user"
	TicketStatusResolved    = "resolved"
	TicketStatusClosed      = "closed"
)

type Ticket struct {
	ID                 int64      `json:"id"`
	UserID             int64      `json:"user_id"`
	Subject            string     `json:"subject"`
	Description        string     `json:"description"`
	Contact            string     `json:"contact"`
	Category           string     `json:"category"`
	Priority           string     `json:"priority"`
	Status             string     `json:"status"`
	OrderID            *int64     `json:"order_id,omitempty"`
	AssignedTo         *int64     `json:"assigned_to,omitempty"`
	FirstResponseDueAt *time.Time `json:"first_response_due_at,omitempty"`
	ResolutionDueAt    *time.Time `json:"resolution_due_at,omitempty"`
	LastMessageID      int64      `json:"last_message_id"`
	LastMessagePreview string     `json:"last_message_preview"`
	LastMessageAt      time.Time  `json:"last_message_at"`
	UnreadCount        int64      `json:"unread_count"`
	UserName           string     `json:"user_name,omitempty"`
	UserEmail          string     `json:"user_email,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

type TicketMessage struct {
	ID         int64          `json:"id"`
	TicketID   int64          `json:"ticket_id"`
	Kind       string         `json:"kind"`
	AuthorRole string         `json:"author_role"`
	AuthorName string         `json:"author_name,omitempty"`
	Content    string         `json:"content"`
	EventType  string         `json:"event_type,omitempty"`
	EventData  map[string]any `json:"event_data,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
}

type TicketRequester struct {
	ID                     int64     `json:"id"`
	Username               string    `json:"username"`
	Email                  string    `json:"email"`
	Balance                float64   `json:"balance"`
	Status                 string    `json:"status"`
	CreatedAt              time.Time `json:"created_at"`
	UsageStatsAvailable    bool      `json:"usage_stats_available"`
	TodayTokens            int64     `json:"today_tokens"`
	TodayCost              float64   `json:"today_cost"`
	RechargeStatsAvailable bool      `json:"recharge_stats_available"`
	Recharged14d           float64   `json:"recharged_14d"`
}

type TicketDetail struct {
	Ticket    Ticket           `json:"ticket"`
	Messages  []TicketMessage  `json:"messages"`
	Tags      []TicketTag      `json:"tags"`
	HasMore   bool             `json:"has_more"`
	Requester *TicketRequester `json:"requester,omitempty"`
}

type TicketPage struct {
	Items    []Ticket `json:"items"`
	Total    int64    `json:"total"`
	Page     int      `json:"page"`
	PageSize int      `json:"page_size"`
	Pages    int      `json:"pages"`
}

type TicketStats struct {
	Total        int64      `json:"total"`
	Open         int64      `json:"open"`
	InProgress   int64      `json:"in_progress"`
	WaitingUser  int64      `json:"waiting_user"`
	Resolved     int64      `json:"resolved"`
	Closed       int64      `json:"closed"`
	Unread       int64      `json:"unread"`
	CanCreate    bool       `json:"can_create"`
	DailyUsed    int64      `json:"daily_used"`
	DailyLimit   int64      `json:"daily_limit"`
	NextCreateAt *time.Time `json:"next_create_at,omitempty"`
}

type TicketListFilter struct {
	Page       int
	PageSize   int
	Search     string
	Status     string
	Category   string
	Priority   string
	AssignedTo string
}

type TicketCreateInput struct {
	Subject  string
	Content  string
	Contact  string
	Category string
	Priority string
	OrderID  *int64
	ClientID string
}

type TicketUpdateInput struct {
	Status     *string
	Category   *string
	Priority   *string
	AssignedTo *int64
	ClearOwner bool
}

type TicketTag struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type TicketAssignee struct {
	ID       int64  `json:"id"`
	Username string `json:"username,omitempty"`
	Email    string `json:"email"`
}
type TicketRepository interface {
	CreateTicket(context.Context, int64, TicketCreateInput) (Ticket, error)
	ListTickets(context.Context, int64, bool, TicketListFilter) (TicketPage, error)
	Stats(context.Context, int64, bool) (TicketStats, error)
	GetTicket(context.Context, int64, int64, bool, int64) (TicketDetail, error)
	AddMessage(context.Context, int64, int64, string, string, string, bool) (TicketDetail, error)
	UpdateTicket(context.Context, int64, int64, bool, TicketUpdateInput) (TicketDetail, error)
	ListAssignees(context.Context) ([]TicketAssignee, error)
	MarkRead(context.Context, int64, int64, int64) error
	ReplaceTags(context.Context, int64, []int64) error
	ListTags(context.Context) ([]TicketTag, error)
}

type TicketService struct {
	repo TicketRepository
}

func NewTicketService(repo TicketRepository) *TicketService {
	return &TicketService{repo: repo}
}

func (s *TicketService) Create(ctx context.Context, userID int64, in TicketCreateInput) (TicketDetail, error) {
	in.Subject = strings.TrimSpace(in.Subject)
	in.Content = strings.TrimSpace(in.Content)
	in.Contact = strings.TrimSpace(in.Contact)
	in.ClientID = strings.TrimSpace(in.ClientID)
	in.Category = normalizeTicketCategory(in.Category)
	in.Priority = normalizeTicketPriority(in.Priority)
	if in.Subject == "" || len([]rune(in.Subject)) > 160 || in.Content == "" || len([]rune(in.Content)) > 10000 || len([]rune(in.Contact)) > 200 || len([]rune(in.ClientID)) > 100 {
		return TicketDetail{}, infraerrors.BadRequest("INVALID_REQUEST", "ticket fields exceed the allowed limits")
	}
	if in.Category == "" {
		return TicketDetail{}, infraerrors.BadRequest("INVALID_CATEGORY", "unsupported ticket category")
	}
	if in.Priority == "" {
		return TicketDetail{}, infraerrors.BadRequest("INVALID_PRIORITY", "unsupported ticket priority")
	}
	if in.OrderID != nil && *in.OrderID <= 0 {
		return TicketDetail{}, infraerrors.BadRequest("INVALID_ORDER", "order_id must be positive")
	}
	if in.Category == TicketCategoryBilling && in.Contact == "" {
		return TicketDetail{}, infraerrors.BadRequest("CONTACT_REQUIRED", "billing tickets require contact information")
	}
	ticket, err := s.repo.CreateTicket(ctx, userID, in)
	if err != nil {
		return TicketDetail{}, err
	}
	return s.repo.GetTicket(ctx, ticket.ID, userID, false, 0)
}

func (s *TicketService) List(ctx context.Context, userID int64, admin bool, filter TicketListFilter) (TicketPage, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 || filter.PageSize > 100 {
		filter.PageSize = 20
	}
	if strings.TrimSpace(filter.Category) != "" {
		filter.Category = normalizeTicketCategory(filter.Category)
	}
	if strings.TrimSpace(filter.Priority) != "" {
		filter.Priority = normalizeTicketPriority(filter.Priority)
	}
	return s.repo.ListTickets(ctx, userID, admin, filter)
}

func (s *TicketService) Stats(ctx context.Context, userID int64, admin bool) (TicketStats, error) {
	return s.repo.Stats(ctx, userID, admin)
}

func (s *TicketService) Detail(ctx context.Context, id, userID int64, admin bool, beforeID int64) (TicketDetail, error) {
	if id <= 0 {
		return TicketDetail{}, infraerrors.BadRequest("INVALID_ID", "invalid ticket id")
	}
	return s.repo.GetTicket(ctx, id, userID, admin, beforeID)
}

func (s *TicketService) Reply(ctx context.Context, id, userID int64, admin bool, content, clientID string) (TicketDetail, error) {
	content = strings.TrimSpace(content)
	clientID = strings.TrimSpace(clientID)
	if content == "" || len([]rune(content)) > 10000 {
		return TicketDetail{}, infraerrors.BadRequest("INVALID_MESSAGE", "reply must contain 1-10000 characters")
	}
	if len([]rune(clientID)) > 100 {
		return TicketDetail{}, infraerrors.BadRequest("INVALID_CLIENT_ID", "client_id must contain at most 100 characters")
	}
	role := "user"
	if admin {
		role = "admin"
	}
	if err := s.ensureVisible(ctx, id, userID, admin); err != nil {
		return TicketDetail{}, err
	}
	return s.repo.AddMessage(ctx, id, userID, role, content, clientID, false)
}

func (s *TicketService) AddInternalNote(ctx context.Context, id, adminID int64, content string) (TicketDetail, error) {
	content = strings.TrimSpace(content)
	if content == "" || len([]rune(content)) > 10000 {
		return TicketDetail{}, infraerrors.BadRequest("INVALID_NOTE", "internal note must contain 1-10000 characters")
	}
	if err := s.ensureVisible(ctx, id, adminID, true); err != nil {
		return TicketDetail{}, err
	}
	return s.repo.AddMessage(ctx, id, adminID, "admin", content, "", true)
}

func (s *TicketService) Update(ctx context.Context, id, userID int64, admin bool, in TicketUpdateInput) (TicketDetail, error) {
	if in.Status != nil {
		status := strings.ToLower(strings.TrimSpace(*in.Status))
		if !validTicketStatus(status) {
			return TicketDetail{}, infraerrors.BadRequest("INVALID_STATUS", "unsupported ticket status")
		}
		in.Status = &status
	}
	if in.Category != nil {
		category := normalizeTicketCategory(*in.Category)
		if category == "" {
			return TicketDetail{}, infraerrors.BadRequest("INVALID_CATEGORY", "unsupported ticket category")
		}
		in.Category = &category
	}
	if in.Priority != nil {
		priority := normalizeTicketPriority(*in.Priority)
		if priority == "" {
			return TicketDetail{}, infraerrors.BadRequest("INVALID_PRIORITY", "unsupported ticket priority")
		}
		in.Priority = &priority
	}
	if !admin {
		if in.Category != nil || in.Priority != nil || in.AssignedTo != nil || in.ClearOwner {
			return TicketDetail{}, infraerrors.Forbidden("TICKET_FIELDS_FORBIDDEN", "users may only change ticket status")
		}
		if in.Status != nil && *in.Status != TicketStatusOpen && *in.Status != TicketStatusClosed {
			return TicketDetail{}, infraerrors.Forbidden("TICKET_STATUS_FORBIDDEN", "users may reopen or close their own ticket")
		}
	}
	if err := s.ensureVisible(ctx, id, userID, admin); err != nil {
		return TicketDetail{}, err
	}
	return s.repo.UpdateTicket(ctx, id, userID, admin, in)
}

func (s *TicketService) MarkRead(ctx context.Context, id, userID int64, admin bool, lastMessageID int64) error {
	if lastMessageID < 0 {
		return infraerrors.BadRequest("INVALID_MESSAGE_ID", "last_message_id must not be negative")
	}
	if err := s.ensureVisible(ctx, id, userID, admin); err != nil {
		return err
	}
	return s.repo.MarkRead(ctx, id, userID, lastMessageID)
}

func (s *TicketService) ReplaceTags(ctx context.Context, id, adminID int64, tagIDs []int64) error {
	if err := s.ensureVisible(ctx, id, adminID, true); err != nil {
		return err
	}
	return s.repo.ReplaceTags(ctx, id, tagIDs)
}

func (s *TicketService) ListTags(ctx context.Context) ([]TicketTag, error) {
	return s.repo.ListTags(ctx)
}

func (s *TicketService) ListAssignees(ctx context.Context) ([]TicketAssignee, error) {
	return s.repo.ListAssignees(ctx)
}
func (s *TicketService) ensureVisible(ctx context.Context, id, userID int64, admin bool) error {
	_, err := s.repo.GetTicket(ctx, id, userID, admin, -1)
	return err
}

func normalizeTicketCategory(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "", TicketCategoryAPI:
		if value == "" {
			return TicketCategoryAPI
		}
		return value
	case TicketCategoryBilling, TicketCategoryAccount, TicketCategoryOther:
		return value
	default:
		return ""
	}
}

func normalizeTicketPriority(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return TicketPriorityNormal
	}
	switch value {
	case TicketPriorityLow, TicketPriorityNormal, TicketPriorityHigh, TicketPriorityUrgent:
		return value
	default:
		return ""
	}
}

func validTicketStatus(value string) bool {
	switch value {
	case TicketStatusOpen, TicketStatusInProgress, TicketStatusWaitingUser, TicketStatusResolved, TicketStatusClosed:
		return true
	default:
		return false
	}
}

func ValidateTicketStatusTransition(current, next string) error {
	if current == next {
		return nil
	}
	allowed := false
	switch current {
	case TicketStatusOpen:
		allowed = next == TicketStatusInProgress || next == TicketStatusResolved || next == TicketStatusClosed
	case TicketStatusInProgress:
		allowed = next == TicketStatusOpen || next == TicketStatusWaitingUser || next == TicketStatusResolved || next == TicketStatusClosed
	case TicketStatusWaitingUser:
		allowed = next == TicketStatusOpen || next == TicketStatusInProgress || next == TicketStatusResolved || next == TicketStatusClosed
	case TicketStatusResolved:
		allowed = next == TicketStatusOpen || next == TicketStatusClosed
	case TicketStatusClosed:
		allowed = next == TicketStatusOpen
	}
	if !allowed {
		return infraerrors.BadRequest("INVALID_STATUS_TRANSITION", "unsupported ticket status transition")
	}
	return nil
}
