package admin

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

type ticketAdminReplyRequest struct {
	Content string `json:"content"`
	Status  string `json:"status"`
}

type ticketAdminUpdateRequest struct {
	Status     *string `json:"status"`
	Category   *string `json:"category"`
	Priority   *string `json:"priority"`
	AssignedTo *int64  `json:"assigned_to"`
	ClearOwner bool    `json:"clear_owner"`
}

type ticketInternalNoteRequest struct {
	Content string `json:"content"`
}

type ticketTagsRequest struct {
	TagIDs []int64 `json:"tag_ids"`
}

func (h *TicketHandler) List(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "Admin not authenticated")
		return
	}
	page, pageSize := response.ParsePagination(c)
	result, err := h.ticketService.List(c.Request.Context(), subject.UserID, true, service.TicketListFilter{
		Page: page, PageSize: pageSize,
		Search: c.Query("search"), Status: c.Query("status"), Category: c.Query("category"), Priority: c.Query("priority"), AssignedTo: c.Query("assigned_to"),
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
		response.Unauthorized(c, "Admin not authenticated")
		return
	}
	result, err := h.ticketService.Stats(c.Request.Context(), subject.UserID, true)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *TicketHandler) GetByID(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "Admin not authenticated")
		return
	}
	id, beforeID, ok := parseAdminTicketIDs(c)
	if !ok {
		return
	}
	result, err := h.ticketService.Detail(c.Request.Context(), id, subject.UserID, true, beforeID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *TicketHandler) Reply(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "Admin not authenticated")
		return
	}
	id, _, ok := parseAdminTicketIDs(c)
	if !ok {
		return
	}
	var req ticketAdminReplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid reply payload")
		return
	}
	result, err := h.ticketService.Reply(c.Request.Context(), id, subject.UserID, true, req.Content, "")
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if status := strings.TrimSpace(req.Status); status != "" && status != result.Ticket.Status {
		result, err = h.ticketService.Update(c.Request.Context(), id, subject.UserID, true, service.TicketUpdateInput{Status: &status})
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
	}
	response.Success(c, result)
}

func (h *TicketHandler) AddInternalNote(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "Admin not authenticated")
		return
	}
	id, _, ok := parseAdminTicketIDs(c)
	if !ok {
		return
	}
	var req ticketInternalNoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid internal note payload")
		return
	}
	result, err := h.ticketService.AddInternalNote(c.Request.Context(), id, subject.UserID, req.Content)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *TicketHandler) Update(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "Admin not authenticated")
		return
	}
	id, _, ok := parseAdminTicketIDs(c)
	if !ok {
		return
	}
	var req ticketAdminUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid ticket update payload")
		return
	}
	result, err := h.ticketService.Update(c.Request.Context(), id, subject.UserID, true, service.TicketUpdateInput{
		Status: req.Status, Category: req.Category, Priority: req.Priority, AssignedTo: req.AssignedTo, ClearOwner: req.ClearOwner,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *TicketHandler) MarkRead(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "Admin not authenticated")
		return
	}
	id, _, ok := parseAdminTicketIDs(c)
	if !ok {
		return
	}
	var req struct {
		LastMessageID int64 `json:"last_message_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid read cursor payload")
		return
	}
	if err := h.ticketService.MarkRead(c.Request.Context(), id, subject.UserID, true, req.LastMessageID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"ticket_id": id, "last_message_id": req.LastMessageID})
}

func (h *TicketHandler) ListTags(c *gin.Context) {
	result, err := h.ticketService.ListTags(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": result})
}

func (h *TicketHandler) ListAssignees(c *gin.Context) {
	result, err := h.ticketService.ListAssignees(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": result})
}

func (h *TicketHandler) ReplaceTags(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "Admin not authenticated")
		return
	}
	id, _, ok := parseAdminTicketIDs(c)
	if !ok {
		return
	}
	var req ticketTagsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid ticket tags payload")
		return
	}
	if err := h.ticketService.ReplaceTags(c.Request.Context(), id, subject.UserID, req.TagIDs); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"ticket_id": id, "tag_ids": req.TagIDs})
}

func parseAdminTicketIDs(c *gin.Context) (int64, int64, bool) {
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
