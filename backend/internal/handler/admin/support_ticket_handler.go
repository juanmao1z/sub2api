package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
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
type approvalRequest struct {
	State  string `json:"state" binding:"required"`
	Reason string `json:"reason"`
}

// List lists all tickets.
func (h *SupportTicketHandler) List(c *gin.Context) {
	v, e := h.svc.AdminList(c)
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
	t, e := h.svc.AdminGet(c, id)
	if e != nil {
		response.ErrorFrom(c, e)
		return
	}
	m, err := h.svc.Messages(c, id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"ticket": t.Ticket, "user": t.User, "messages": m})
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
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "Admin user not authenticated")
		return
	}
	var req adminMessageRequest
	if e := c.ShouldBindJSON(&req); e != nil {
		response.BadRequest(c, e.Error())
		return
	}
	m, e := h.svc.AddMessage(c, id, subject.UserID, "ADMIN", req.Body)
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

// GetApproval returns the approval state for a Zammad-linked ticket.
func (h *SupportTicketHandler) GetApproval(c *gin.Context) {
	id, ok := adminTicketID(c)
	if !ok {
		return
	}
	approval, err := h.svc.GetApproval(c, id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, approval)
}

// DecideApproval records an administrative approval decision.
func (h *SupportTicketHandler) DecideApproval(c *gin.Context) {
	id, ok := adminTicketID(c)
	if !ok {
		return
	}
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "Admin user not authenticated")
		return
	}
	var req approvalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	approval, err := h.svc.DecideApproval(c, id, subject.UserID, req.State, req.Reason)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, approval)
}
