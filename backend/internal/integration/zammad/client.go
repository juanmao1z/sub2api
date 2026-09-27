package zammad

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

const (
	defaultTimeout       = 10 * time.Second
	maxResponseBodyBytes = 1 << 20
)

var ErrNotConfigured = errors.New("zammad integration is not configured")

// APIError preserves the HTTP status while keeping upstream response bodies bounded.
type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("zammad API returned HTTP %d", e.StatusCode)
	}
	return fmt.Sprintf("zammad API returned HTTP %d: %s", e.StatusCode, e.Body)
}

type Client struct {
	baseURL    *url.URL
	token      string
	httpClient *http.Client
	enabled    bool
}

// NewClient creates a server-side Zammad client. The token is never exposed by this package.
func NewClient(cfg config.ZammadConfig) (*Client, error) {
	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	client := &Client{enabled: cfg.Enabled, token: strings.TrimSpace(cfg.APIToken)}
	if !cfg.Enabled {
		return client, nil
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return nil, fmt.Errorf("zammad base_url must be an https URL")
	}
	if cfg.Enabled && client.token == "" {
		return nil, ErrNotConfigured
	}
	timeout := defaultTimeout
	if cfg.TimeoutSeconds > 0 {
		timeout = time.Duration(cfg.TimeoutSeconds) * time.Second
	}
	client.baseURL = parsed
	client.httpClient = &http.Client{Timeout: timeout}
	return client, nil
}

func (c *Client) Enabled() bool { return c != nil && c.enabled }

func (c *Client) Configured() bool {
	return c != nil && c.enabled && c.baseURL != nil && c.token != "" && c.httpClient != nil
}

type Ticket struct {
	ID         int64  `json:"id"`
	Number     string `json:"number"`
	Title      string `json:"title"`
	State      string `json:"state"`
	StateID    int64  `json:"state_id"`
	CustomerID int64  `json:"customer_id"`
	Group      string `json:"group"`
}

type Article struct {
	ID          int64  `json:"id,omitempty"`
	TicketID    int64  `json:"ticket_id,omitempty"`
	Subject     string `json:"subject,omitempty"`
	Body        string `json:"body"`
	ContentType string `json:"content_type,omitempty"`
	Type        string `json:"type,omitempty"`
	Internal    bool   `json:"internal,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
	CreatedBy   int64  `json:"created_by,omitempty"`
}

type TicketState struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

type CreateTicketRequest struct {
	Title      string  `json:"title"`
	Group      string  `json:"group"`
	CustomerID int64   `json:"customer_id,omitempty"`
	Article    Article `json:"article"`
	Tags       string  `json:"tags,omitempty"`
}

type UpdateTicketRequest struct {
	StateID int64 `json:"state_id,omitempty"`
}

type TicketStatus string

const (
	TicketStatusOpen     TicketStatus = "OPEN"
	TicketStatusResolved TicketStatus = "RESOLVED"
	TicketStatusClosed   TicketStatus = "CLOSED"
)

func (c *Client) Health(ctx context.Context) error {
	if !c.Configured() {
		return ErrNotConfigured
	}
	var user map[string]json.RawMessage
	return c.doJSON(ctx, http.MethodGet, "/api/v1/users/me", nil, &user)
}

// SearchUsersByEmail preserves all matches so ambiguous addresses can be
// blocked by the identity policy instead of silently selecting one user.
func (c *Client) SearchUsersByEmail(ctx context.Context, email string) ([]User, error) {
	if !c.Configured() {
		return nil, ErrNotConfigured
	}
	email = normalizeEmail(email)
	if email == "" {
		return nil, fmt.Errorf("zammad user email is required")
	}
	query := url.Values{}
	query.Set("query", "email:"+email)
	var users []User
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/users/search?"+query.Encode(), nil, &users); err != nil {
		return nil, err
	}
	exact := make([]User, 0, len(users))
	for _, user := range users {
		if normalizeEmail(user.Email) == email {
			exact = append(exact, user)
		}
	}
	return exact, nil
}

func (c *Client) GetUser(ctx context.Context, id int64) (*User, error) {
	if id <= 0 {
		return nil, fmt.Errorf("zammad user id must be positive")
	}
	if !c.Configured() {
		return nil, ErrNotConfigured
	}
	var user User
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/users/"+strconv.FormatInt(id, 10), nil, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

func (c *Client) CreateUser(ctx context.Context, payload CreateUserRequest) (*User, error) {
	if !c.Configured() {
		return nil, ErrNotConfigured
	}
	payload.Email = normalizeEmail(payload.Email)
	payload.Login = strings.TrimSpace(payload.Login)
	if payload.Email == "" || payload.Login == "" {
		return nil, fmt.Errorf("zammad user email and login are required")
	}
	if payload.Firstname == "" {
		payload.Firstname = "Sub2API"
	}
	if payload.Lastname == "" {
		payload.Lastname = "Customer"
	}
	var user User
	if err := c.doJSON(ctx, http.MethodPost, "/api/v1/users", payload, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

func (c *Client) UpdateUserEmail(ctx context.Context, id int64, email string) (*User, error) {
	if id <= 0 {
		return nil, fmt.Errorf("zammad user id must be positive")
	}
	if !c.Configured() {
		return nil, ErrNotConfigured
	}
	email = normalizeEmail(email)
	if email == "" {
		return nil, fmt.Errorf("zammad user email is required")
	}
	var user User
	if err := c.doJSON(ctx, http.MethodPut, "/api/v1/users/"+strconv.FormatInt(id, 10), map[string]string{"email": email}, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

func (c *Client) CreateTicket(ctx context.Context, payload CreateTicketRequest) (*Ticket, error) {
	if !c.Configured() {
		return nil, ErrNotConfigured
	}
	var ticket Ticket
	if err := c.doJSON(ctx, http.MethodPost, "/api/v1/tickets", payload, &ticket); err != nil {
		return nil, err
	}
	return &ticket, nil
}

func (c *Client) GetTicket(ctx context.Context, id int64) (*Ticket, error) {
	if id <= 0 {
		return nil, fmt.Errorf("zammad ticket id must be positive")
	}
	if !c.Configured() {
		return nil, ErrNotConfigured
	}
	var ticket Ticket
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/tickets/"+strconv.FormatInt(id, 10), nil, &ticket); err != nil {
		return nil, err
	}
	return &ticket, nil
}

func (c *Client) AddArticle(ctx context.Context, ticketID int64, article Article) (*Article, error) {
	if ticketID <= 0 {
		return nil, fmt.Errorf("zammad ticket id must be positive")
	}
	if !c.Configured() {
		return nil, ErrNotConfigured
	}
	article.TicketID = ticketID
	var result Article
	if err := c.doJSON(ctx, http.MethodPost, "/api/v1/ticket_articles", article, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) ListArticles(ctx context.Context, ticketID int64) ([]Article, error) {
	if ticketID <= 0 {
		return nil, fmt.Errorf("zammad ticket id must be positive")
	}
	if !c.Configured() {
		return nil, ErrNotConfigured
	}
	var articles []Article
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/ticket_articles/by_ticket/"+strconv.FormatInt(ticketID, 10), nil, &articles); err != nil {
		return nil, err
	}
	return articles, nil
}

func (c *Client) UpdateTicketStatus(ctx context.Context, ticketID int64, status TicketStatus) (*Ticket, error) {
	if ticketID <= 0 {
		return nil, fmt.Errorf("zammad ticket id must be positive")
	}
	stateID, err := c.findStateID(ctx, status)
	if err != nil {
		return nil, err
	}
	return c.UpdateTicket(ctx, ticketID, UpdateTicketRequest{StateID: stateID})
}

func (c *Client) findStateID(ctx context.Context, status TicketStatus) (int64, error) {
	if !c.Configured() {
		return 0, ErrNotConfigured
	}
	var states []TicketState
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/ticket_states", nil, &states); err != nil {
		return 0, err
	}
	aliases := map[TicketStatus][]string{
		TicketStatusOpen:     {"open", "new", "pending"},
		TicketStatusResolved: {"resolved", "solved", "closed successful"},
		TicketStatusClosed:   {"closed", "closed unsuccessful"},
	}
	for _, state := range states {
		if !state.Active {
			continue
		}
		name := normalizeStateName(state.Name)
		for _, alias := range aliases[status] {
			if name == alias {
				return state.ID, nil
			}
		}
	}
	return 0, fmt.Errorf("zammad ticket state is not configured for %s", status)
}

func normalizeStateName(name string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(name)), " "))
}

func NormalizeTicketStatus(state string) (TicketStatus, error) {
	switch normalizeStateName(state) {
	case "new", "open", "pending":
		return TicketStatusOpen, nil
	case "resolved", "solved", "closed successful":
		return TicketStatusResolved, nil
	case "closed", "closed unsuccessful":
		return TicketStatusClosed, nil
	default:
		return "", fmt.Errorf("unsupported zammad ticket state %q", state)
	}
}

func (c *Client) UpdateTicket(ctx context.Context, ticketID int64, update UpdateTicketRequest) (*Ticket, error) {
	if ticketID <= 0 {
		return nil, fmt.Errorf("zammad ticket id must be positive")
	}
	if !c.Configured() {
		return nil, ErrNotConfigured
	}
	var ticket Ticket
	if err := c.doJSON(ctx, http.MethodPut, "/api/v1/tickets/"+strconv.FormatInt(ticketID, 10), update, &ticket); err != nil {
		return nil, err
	}
	return &ticket, nil
}

func (c *Client) doJSON(ctx context.Context, method, path string, payload any, result any) error {
	if !c.Configured() {
		return ErrNotConfigured
	}
	relative, err := url.Parse(path)
	if err != nil || relative.IsAbs() || relative.Host != "" {
		return fmt.Errorf("invalid zammad API path")
	}
	endpoint := *c.baseURL
	endpoint.Path = strings.TrimRight(endpoint.Path, "/") + relative.Path
	endpoint.RawQuery = relative.RawQuery
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("encode zammad request: %w", err)
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint.String(), body)
	if err != nil {
		return fmt.Errorf("create zammad request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Token "+c.token)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request zammad API: %w", err)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodyBytes))
	if err != nil {
		return fmt.Errorf("read zammad response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return &APIError{StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(responseBody))}
	}
	if result == nil || len(bytes.TrimSpace(responseBody)) == 0 {
		return nil
	}
	if err := json.Unmarshal(responseBody, result); err != nil {
		return fmt.Errorf("decode zammad response: %w", err)
	}
	return nil
}
