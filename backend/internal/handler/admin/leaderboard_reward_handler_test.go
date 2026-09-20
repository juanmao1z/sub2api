package admin

import (
	"bytes"
	"context"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
)

// @brief Capture service invocations while verifying authorization at the HTTP boundary.
type rewardHandlerStub struct {
	calls          int
	actor          int64
	day, key, rate string
}

func (s *rewardHandlerStub) Preview(_ context.Context, rate string) (*service.LeaderboardRewardPreview, error) {
	s.calls++
	s.rate = rate
	return &service.LeaderboardRewardPreview{Date: "2026-09-18"}, nil
}
func (s *rewardHandlerStub) Pay(_ context.Context, day, key, rate string, actor int64) (*service.LeaderboardRewardPreview, error) {
	s.calls++
	s.actor = actor
	s.day = day
	s.key = key
	s.rate = rate
	return &service.LeaderboardRewardPreview{Paid: true}, nil
}

// @brief Anonymous and non-admin identities cannot read recipient finances or issue credits.
func TestLeaderboardRewardHandlerAuthorization(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		for _, role := range []string{"", "user", "admin"} {
			t.Run(method+role, func(t *testing.T) {
				svc := &rewardHandlerStub{}
				h := &LeaderboardRewardHandler{svc: svc}
				router := gin.New()
				router.Use(func(c *gin.Context) {
					if role != "" {
						c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 7})
						c.Set(string(middleware.ContextKeyUserRole), role)
					}
				})
				router.GET("/rewards", h.Preview)
				router.POST("/rewards", h.Pay)
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(method, "/rewards?rate_percent=12.34", bytes.NewBufferString(`{"date":"2026-09-18","preview_id":"digest","rate_percent":"12.34","amount":9999,"user_id":99}`))
				req.Header.Set("Content-Type", "application/json")
				router.ServeHTTP(rec, req)
				switch role {
				case "":
					require.Equal(t, 401, rec.Code)
					require.Zero(t, svc.calls)
				case "user":
					require.Equal(t, 403, rec.Code)
					require.Zero(t, svc.calls)
				default:
					require.Equal(t, 200, rec.Code)
					require.Equal(t, 1, svc.calls)
					require.Equal(t, "private, no-store", rec.Header().Get("Cache-Control"))
					require.Equal(t, "12.34", svc.rate)
					if method == http.MethodPost {
						require.Equal(t, int64(7), svc.actor)
						require.Equal(t, "2026-09-18", svc.day)
						require.Equal(t, "digest", svc.key)
					}
				}
			})
		}
	}
}
