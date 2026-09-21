//go:build embed

package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// TestKedayaEmbeddedModules verifies executable module delivery and SPA settings injection.
// @brief Ensure the assembled frontend works through the production Go static server.
// @param t Test context; requires the normal frontend build to have completed.
func TestKedayaEmbeddedModules(t *testing.T) {
	if _, err := frontendFS.ReadFile("dist/integration/build-manifest.json"); err != nil {
		t.Skip("run the frontend build to verify the assembled Kedaya assets")
	}
	server, err := NewFrontendServer(&mockSettingsProvider{settings: map[string]string{"site_name": "zhouz.online"}})
	require.NoError(t, err)
	router := gin.New()
	router.Use(server.Middleware())
	for _, route := range []string{"/integration/bootstrap.js", "/integration/navigation.js", "/integration/entries.js"} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, route, nil))
		require.Equal(t, http.StatusOK, response.Code, route)
		require.Contains(t, response.Header().Get("Content-Type"), "javascript", route)
		require.False(t, strings.Contains(response.Body.String(), "<!doctype html>"), route)
	}
	for _, route := range []string{"/home", "/dashboard", "/admin/dashboard"} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, route, nil))
		require.Equal(t, http.StatusOK, response.Code, route)
		require.Contains(t, response.Body.String(), "/integration/bootstrap.js", route)
		require.Contains(t, response.Body.String(), `window.__APP_CONFIG__={"site_name":"zhouz.online"}`, route)
		require.NotContains(t, response.Body.String(), "/preview/session.js", route)
	}
}
