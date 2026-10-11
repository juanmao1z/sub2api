package handler

import (
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type TicketHandler struct {
	ticketService *service.TicketService
}

func NewTicketHandler(ticketService *service.TicketService) *TicketHandler {
	return &TicketHandler{ticketService: ticketService}
}

type ticketCreateRequest struct {
	Subject  string `json:"subject"`
	Content  string `json:"content"`
	Contact  string `json:"contact"`
	Category string `json:"category"`
	Priority string `json:"priority"`
	OrderID  *int64 `json:"order_id"`
	ClientID string `json:"client_id"`
}

type ticketReplyRequest struct {
	Content  string `json:"content"`
	Status   string `json:"status"`
	ClientID string `json:"client_id"`
}

type ticketUpdateRequest struct {
	Status     *string `json:"status"`
	Category   *string `json:"category"`
	Priority   *string `json:"priority"`
	AssignedTo *int64  `json:"assigned_to"`
	ClearOwner bool    `json:"clear_owner"`
}

type ticketReadRequest struct {
	LastMessageID int64 `json:"last_message_id"`
}

func (h *TicketHandler) List(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	page, pageSize := response.ParsePagination(c)
	result, err := h.ticketService.List(c.Request.Context(), subject.UserID, false, service.TicketListFilter{
		Page: page, PageSize: pageSize,
		Search: c.Query("search"), Status: c.Query("status"), Category: c.Query("category"), Priority: c.Query("priority"),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *TicketHandler) Stats(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	result, err := h.ticketService.Stats(c.Request.Context(), subject.UserID, false)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *TicketHandler) Create(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req ticketCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid ticket payload")
		return
	}
	result, err := h.ticketService.Create(c.Request.Context(), subject.UserID, service.TicketCreateInput{
		Subject: req.Subject, Content: req.Content, Contact: req.Contact, Category: req.Category,
		Priority: req.Priority, OrderID: req.OrderID, ClientID: req.ClientID,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, result)
}

func (h *TicketHandler) GetByID(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, beforeID, ok := parseTicketIDs(c)
	if !ok {
		return
	}
	result, err := h.ticketService.Detail(c.Request.Context(), id, subject.UserID, false, beforeID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *TicketHandler) Reply(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, _, ok := parseTicketIDs(c)
	if !ok {
		return
	}
	var req ticketReplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid reply payload")
		return
	}
	result, err := h.ticketService.Reply(c.Request.Context(), id, subject.UserID, false, req.Content, req.ClientID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if status := strings.TrimSpace(req.Status); status != "" && status != result.Ticket.Status {
		result, err = h.ticketService.Update(c.Request.Context(), id, subject.UserID, false, service.TicketUpdateInput{Status: &status})
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
	}
	response.Success(c, result)
}

func (h *TicketHandler) Update(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, _, ok := parseTicketIDs(c)
	if !ok {
		return
	}
	var req ticketUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid ticket update payload")
		return
	}
	result, err := h.ticketService.Update(c.Request.Context(), id, subject.UserID, false, service.TicketUpdateInput{Status: req.Status})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *TicketHandler) MarkRead(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, _, ok := parseTicketIDs(c)
	if !ok {
		return
	}
	var req ticketReadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid read cursor payload")
		return
	}
	if err := h.ticketService.MarkRead(c.Request.Context(), id, subject.UserID, false, req.LastMessageID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"ticket_id": id, "last_message_id": req.LastMessageID})
}

func parseTicketIDs(c *gin.Context) (int64, int64, bool) {
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid ticket id")
		return 0, 0, false
	}
	beforeID := int64(0)
	if raw := strings.TrimSpace(c.Query("before_id")); raw != "" {
		beforeID, err = strconv.ParseInt(raw, 10, 64)
		if err != nil || beforeID < 0 {
			response.BadRequest(c, "Invalid before_id")
			return 0, 0, false
		}
	}
	return id, beforeID, true
}
