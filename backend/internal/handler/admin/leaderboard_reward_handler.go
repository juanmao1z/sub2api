package admin

import (
	"context"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"time"
)

// @brief Narrow contract for admin-only preview and atomic daily settlement.
type leaderboardRewardService interface {
	Preview(context.Context, string) (*service.LeaderboardRewardPreview, error)
	Pay(context.Context, string, string, string, int64) (*service.LeaderboardRewardPreview, error)
}

// @brief Expose reward actions under the existing admin authentication and audit middleware.
type LeaderboardRewardHandler struct{ svc leaderboardRewardService }

// @brief Bind the reward transaction service to its HTTP endpoints.
func NewLeaderboardRewardHandler(svc *service.LeaderboardRewardService) *LeaderboardRewardHandler {
	return &LeaderboardRewardHandler{svc: svc}
}

// @brief Defense in depth: require a verified admin identity even if a route is registered incorrectly.
func rewardAdmin(c *gin.Context) (int64, bool) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	role, hasRole := middleware.GetUserRoleFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "请先登录")
		return 0, false
	}
	if !hasRole || role != "admin" {
		response.Forbidden(c, "仅管理员可发放奖励")
		return 0, false
	}
	c.Header("Cache-Control", "private, no-store")
	return subject.UserID, true
}

// @brief Return yesterday's recipients, exact amounts, and durable payment status.
func (h *LeaderboardRewardHandler) Preview(c *gin.Context) {
	if _, ok := rewardAdmin(c); !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	result, err := h.svc.Preview(ctx, c.DefaultQuery("rate_percent", "10"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

// @brief Pay the preview once; amounts and recipients are always recomputed on the server.
func (h *LeaderboardRewardHandler) Pay(c *gin.Context) {
	actor, ok := rewardAdmin(c)
	if !ok {
		return
	}
	var input struct {
		Date        string `json:"date" binding:"required"`
		PreviewID   string `json:"preview_id" binding:"required"`
		RatePercent string `json:"rate_percent" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "请刷新奖励信息后重试")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()
	result, err := h.svc.Pay(ctx, input.Date, input.PreviewID, input.RatePercent, actor)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}
