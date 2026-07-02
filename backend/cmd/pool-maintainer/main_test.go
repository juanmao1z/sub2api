package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCollectWritesPlanAndReport(t *testing.T) {
	var sawAuth bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/admin/accounts", r.URL.Path)
		require.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		sawAuth = true
		writeJSON(t, w, map[string]any{
			"code":    0,
			"message": "success",
			"data": map[string]any{
				"items": []map[string]any{
					{
						"id":              7,
						"name":            "https://api.mdkj.lol-pro-0.2",
						"rate_multiplier": 0.2,
						"group_ids":       []int64{18},
						"priority":        50,
						"schedulable":     true,
					},
				},
				"total":     1,
				"page":      1,
				"page_size": 1000,
				"pages":     1,
			},
		})
	}))
	defer server.Close()

	root := t.TempDir()
	configPath := filepath.Join(root, "pool-maintainer.yaml")
	htmlDir := filepath.Join(root, "snapshots")
	outDir := filepath.Join(root, "run")
	require.NoError(t, os.MkdirAll(htmlDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(htmlDir, "mdkj.html"), []byte(`<table><tr><td>pro</td><td>0.10</td></tr></table>`), 0o600))
	require.NoError(t, os.WriteFile(configPath, []byte(`
local_sub2api:
  base_url: "`+server.URL+`"
  admin_token_env: "POOL_MAINTAINER_TEST_TOKEN"
policy:
  safety_margin: 0.02
  self_built_rate: 0
  sales_groups:
    - name: "0.12"
      group_id: 12
      rate: 0.12
    - name: "0.18"
      group_id: 18
      rate: 0.18
  priority:
    self_built: 1
    upstream_start: 5
    upstream_step: 5
upstreams:
  - id: "mdkj"
    base_url: "https://api.mdkj.lol"
    pricing_page_url: "https://api.mdkj.lol/"
    browser_profile: "mdkj"
    group_name_aliases:
      pro: ["pro"]
accounts:
  - match_name: "https://api.mdkj.lol-pro-*"
    upstream_id: "mdkj"
    upstream_group: "pro"
    allowed_sales_groups: ["0.12", "0.18"]
self_built_accounts:
  - match_name: "self-*"
    allowed_sales_groups: ["0.12", "0.18"]
`), 0o600))
	t.Setenv("POOL_MAINTAINER_TEST_TOKEN", "test-token")

	err := run([]string{"collect", "--config", configPath, "--html-dir", htmlDir, "--out", outDir})

	require.NoError(t, err)
	require.True(t, sawAuth)
	require.FileExists(t, filepath.Join(outDir, "apply-plan.json"))
	require.FileExists(t, filepath.Join(outDir, "report.html"))

	rawPlan, err := os.ReadFile(filepath.Join(outDir, "apply-plan.json"))
	require.NoError(t, err)
	var plan map[string]any
	require.NoError(t, json.Unmarshal(rawPlan, &plan))
	require.Equal(t, float64(1), plan["summary"].(map[string]any)["ready_changes"])

	report, err := os.ReadFile(filepath.Join(outDir, "report.html"))
	require.NoError(t, err)
	require.Contains(t, string(report), "https://api.mdkj.lol-pro-0.1")
}

func TestApplyRejectsPlanFromDifferentConfig(t *testing.T) {
	var called bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		t.Fatalf("apply should reject mismatched plan before calling Admin API")
	}))
	defer server.Close()

	root := t.TempDir()
	configPath := filepath.Join(root, "pool-maintainer.yaml")
	planPath := filepath.Join(root, "apply-plan.json")
	require.NoError(t, os.WriteFile(configPath, []byte(`
local_sub2api:
  base_url: "`+server.URL+`"
  admin_token_env: "POOL_MAINTAINER_TEST_TOKEN"
policy:
  safety_margin: 0.02
  sales_groups:
    - name: "0.12"
      group_id: 12
      rate: 0.12
  priority:
    self_built: 1
    upstream_start: 5
    upstream_step: 5
upstreams: []
accounts: []
self_built_accounts: []
`), 0o600))
	require.NoError(t, os.WriteFile(planPath, []byte(`{
  "generated_at": "2026-07-03T00:00:00Z",
  "config": {
    "local_base_url": "https://other.example",
    "sales_groups": [{"name":"0.12","group_id":12,"rate":0.12}],
    "safety_margin": 0.02
  },
  "accounts": []
}`), 0o600))
	t.Setenv("POOL_MAINTAINER_TEST_TOKEN", "test-token")

	err := run([]string{"apply", "--config", configPath, "--plan", planPath})

	require.Error(t, err)
	require.Contains(t, strings.ToLower(err.Error()), "plan config")
	require.False(t, called)
}

func TestApplyRejectsPlanWhenPlanningRulesChanged(t *testing.T) {
	var called bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		t.Fatalf("apply should reject stale plan config before calling Admin API")
	}))
	defer server.Close()

	root := t.TempDir()
	configPath := filepath.Join(root, "pool-maintainer.yaml")
	planPath := filepath.Join(root, "apply-plan.json")
	require.NoError(t, os.WriteFile(configPath, []byte(`
local_sub2api:
  base_url: "`+server.URL+`"
  admin_token_env: "POOL_MAINTAINER_TEST_TOKEN"
policy:
  safety_margin: 0.02
  self_built_rate: 0.08
  sales_groups:
    - name: "0.12"
      group_id: 12
      rate: 0.12
  priority:
    self_built: 1
    upstream_start: 5
    upstream_step: 5
upstreams:
  - id: "mdkj"
    base_url: "https://api.mdkj.lol"
    pricing_page_url: "https://api.mdkj.lol/"
accounts:
  - match_name: "https://api.mdkj.lol-pro-*"
    upstream_id: "mdkj"
    upstream_group: "pro"
    allowed_sales_groups: ["0.12"]
self_built_accounts:
  - match_name: "self-*"
    allowed_sales_groups: ["0.12"]
`), 0o600))
	require.NoError(t, os.WriteFile(planPath, []byte(`{
  "generated_at": "2026-07-03T00:00:00Z",
  "config": {
    "local_base_url": "`+server.URL+`",
    "sales_groups": [{"name":"0.12","group_id":12,"rate":0.12}],
    "safety_margin": 0.02,
    "self_built_rate": 0,
    "priority": {"self_built":1,"upstream_start":5,"upstream_step":5},
    "upstreams": [{"id":"mdkj","base_url":"https://api.mdkj.lol","pricing_page_url":"https://api.mdkj.lol/"}],
    "accounts": [{"match_name":"https://api.mdkj.lol-pro-*","upstream_id":"mdkj","upstream_group":"pro","allowed_sales_groups":["0.12"]}],
    "self_built_accounts": [{"match_name":"self-*","allowed_sales_groups":["0.12"]}]
  },
  "accounts": []
}`), 0o600))
	t.Setenv("POOL_MAINTAINER_TEST_TOKEN", "test-token")

	err := run([]string{"apply", "--config", configPath, "--plan", planPath})

	require.Error(t, err)
	require.Contains(t, strings.ToLower(err.Error()), "plan config")
	require.False(t, called)
}

func writeJSON(t *testing.T, w http.ResponseWriter, body any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	require.NoError(t, json.NewEncoder(w).Encode(body))
}
