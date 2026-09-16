package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"strconv"
)

// SupportTicketHandler exposes administrative ticket operations.
type SupportTicketHandler struct{ svc *service.SupportTicketService }

// NewSupportTicketHandler creates an administrative ticket handler.
func NewSupportTicketHandler(svc *service.SupportTicketService) *SupportTicketHandler {
	return &SupportTicketHandler{svc: svc}
}
func adminTicketID(c *gin.Context) (int64, bool) {
	id, e := strconv.ParseInt(c.Param("id"), 10, 64)
	if e != nil || id <= 0 {
		response.BadRequest(c, "invalid ticket id")
		return 0, false
	}
	return id, true
}

type statusRequest struct {
	Status string `json:"status" binding:"required"`
}
type adminMessageRequest struct {
	Body string `json:"body" binding:"required"`
}

// List lists all tickets.
func (h *SupportTicketHandler) List(c *gin.Context) {
	v, e := h.svc.List(c, 0, true)
	if e != nil {
		response.ErrorFrom(c, e)
		return
	}
	response.Success(c, v)
}

// Get returns a ticket and messages.
func (h *SupportTicketHandler) Get(c *gin.Context) {
	id, ok := adminTicketID(c)
	if !ok {
		return
	}
	t, e := h.svc.Get(c, id, 0, true)
	if e != nil {
		response.ErrorFrom(c, e)
		return
	}
	m, _ := h.svc.Messages(c, id)
	response.Success(c, gin.H{"ticket": t, "messages": m})
}

// AddMessage appends an administrator reply.
func (h *SupportTicketHandler) AddMessage(c *gin.Context) {
	id, ok := adminTicketID(c)
	if !ok {
		return
	}
	if _, e := h.svc.Get(c, id, 0, true); e != nil {
		response.ErrorFrom(c, e)
		return
	}
	var req adminMessageRequest
	if e := c.ShouldBindJSON(&req); e != nil {
		response.BadRequest(c, e.Error())
		return
	}
	m, e := h.svc.AddMessage(c, id, 0, "ADMIN", req.Body)
	if e != nil {
		response.ErrorFrom(c, e)
		return
	}
	response.Created(c, m)
}

// SetStatus updates ticket status.
func (h *SupportTicketHandler) SetStatus(c *gin.Context) {
	id, ok := adminTicketID(c)
	if !ok {
		return
	}
	var req statusRequest
	if e := c.ShouldBindJSON(&req); e != nil {
		response.BadRequest(c, e.Error())
		return
	}
	t, e := h.svc.SetStatus(c, id, req.Status)
	if e != nil {
		response.ErrorFrom(c, e)
		return
	}
	response.Success(c, t)
}
