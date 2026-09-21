//go:build embed

package web

import (
	"encoding/json"
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
	manifestBytes, err := frontendFS.ReadFile("dist/integration/build-manifest.json")
	if err != nil {
		t.Skip("run the frontend build to verify the assembled Kedaya assets")
	}
	var manifest struct {
		ReleasePrefix string `json:"releasePrefix"`
	}
	require.NoError(t, json.Unmarshal(manifestBytes, &manifest))
	require.NotEmpty(t, manifest.ReleasePrefix)
	server, err := NewFrontendServer(&mockSettingsProvider{settings: map[string]string{"site_name": "zhouz.online"}})
	require.NoError(t, err)
	router := gin.New()
	router.Use(server.Middleware())
	for _, name := range []string{"bootstrap.js", "navigation.js", "entries.js"} {
		route := manifest.ReleasePrefix + "/" + name
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, route, nil))
		require.Equal(t, http.StatusOK, response.Code, route)
		require.Contains(t, response.Header().Get("Content-Type"), "javascript", route)
		require.False(t, strings.Contains(response.Body.String(), "<!doctype html>"), route)
	}
	for _, route := range []string{"/home", "/dashboard", "/admin/dashboard", "/tickets", "/admin/tickets", "/custom/cc-switch-guide", "/custom/usage-leaderboard"} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, route, nil))
		require.Equal(t, http.StatusOK, response.Code, route)
		require.Contains(t, response.Body.String(), manifest.ReleasePrefix+"/bootstrap.js", route)
		require.Contains(t, response.Body.String(), `window.__APP_CONFIG__={"site_name":"zhouz.online"}`, route)
		require.NotContains(t, response.Body.String(), "/preview/session.js", route)
	}
}
