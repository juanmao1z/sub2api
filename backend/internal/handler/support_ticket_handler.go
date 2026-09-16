package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"strconv"
)

// SupportTicketHandler exposes user support ticket endpoints.
type SupportTicketHandler struct{ svc *service.SupportTicketService }

// NewSupportTicketHandler creates a user ticket handler.
func NewSupportTicketHandler(svc *service.SupportTicketService) *SupportTicketHandler {
	return &SupportTicketHandler{svc: svc}
}

type supportTicketCreateRequest struct {
	Type        string `json:"type"`
	Subject     string `json:"subject"`
	Description string `json:"description"`
	Contact     string `json:"contact"`
	OrderID     *int64 `json:"order_id"`
}
type supportTicketMessageRequest struct {
	Body string `json:"body" binding:"required"`
}

func ticketID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid ticket id")
		return 0, false
	}
	return id, true
}
func currentUser(c *gin.Context) (int64, bool) {
	s, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return 0, false
	}
	return s.UserID, true
}

// List returns tickets belonging to the authenticated user.
func (h *SupportTicketHandler) List(c *gin.Context) {
	uid, ok := currentUser(c)
	if !ok {
		return
	}
	v, err := h.svc.List(c, uid, false)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, v)
}

// Create creates a refund or suggestion ticket.
func (h *SupportTicketHandler) Create(c *gin.Context) {
	uid, ok := currentUser(c)
	if !ok {
		return
	}
	var req supportTicketCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	v, err := h.svc.Create(c, uid, req.Type, req.Subject, req.Description, req.Contact, req.OrderID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, v)
}

// Get returns one owned ticket with its messages.
func (h *SupportTicketHandler) Get(c *gin.Context) {
	id, ok := ticketID(c)
	if !ok {
		return
	}
	uid, ok := currentUser(c)
	if !ok {
		return
	}
	t, err := h.svc.Get(c, id, uid, false)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	m, _ := h.svc.Messages(c, id)
	response.Success(c, gin.H{"ticket": t, "messages": m})
}

// AddMessage appends a user reply.
func (h *SupportTicketHandler) AddMessage(c *gin.Context) {
	id, ok := ticketID(c)
	if !ok {
		return
	}
	uid, ok := currentUser(c)
	if !ok {
		return
	}
	if _, err := h.svc.Get(c, id, uid, false); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	var req supportTicketMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	m, err := h.svc.AddMessage(c, id, uid, "USER", req.Body)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, m)
}

// Close closes an owned ticket.
func (h *SupportTicketHandler) Close(c *gin.Context) {
	id, ok := ticketID(c)
	if !ok {
		return
	}
	uid, ok := currentUser(c)
	if !ok {
		return
	}
	if _, err := h.svc.Get(c, id, uid, false); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	t, err := h.svc.SetStatus(c, id, service.SupportTicketStatusClosed)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, t)
}
