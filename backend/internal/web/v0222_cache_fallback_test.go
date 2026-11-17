//go:build unit && embed

package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestExpiredSettingsFallbackUsesCurrentCSPNonce(t *testing.T) {
	provider := &mockSettingsProvider{settings: map[string]string{"site_name": "preserved brand"}}
	server, err := NewFrontendServer(provider)
	require.NoError(t, err)
	first := httptest.NewRecorder()
	c1, _ := gin.CreateTestContext(first)
	c1.Request = httptest.NewRequest(http.MethodGet, "/home", nil)
	c1.Set(middleware.CSPNonceKey, "first-nonce")
	server.serveIndexHTML(c1)
	require.Equal(t, http.StatusOK, first.Code)
	server.cacheTTL = time.Nanosecond
	provider.err = context.DeadlineExceeded
	second := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(second)
	c2.Request = httptest.NewRequest(http.MethodGet, "/home", nil)
	c2.Request.Header.Set("If-None-Match", first.Header().Get("ETag"))
	c2.Set(middleware.CSPNonceKey, "second-nonce")
	server.serveIndexHTML(c2)
	require.Equal(t, 2, provider.called, "expired cache must attempt settings refresh")
	require.Equal(t, http.StatusOK, second.Code, "nonce-bearing fallback must not return 304")
	require.Contains(t, second.Body.String(), `nonce="second-nonce"`)
	require.NotContains(t, second.Body.String(), `nonce="first-nonce"`)
	require.Contains(t, second.Body.String(), "preserved brand")
	assertNoStoreHTMLHeaders(t, second.Header())
}
