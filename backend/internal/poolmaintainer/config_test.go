package poolmaintainer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func writeTempConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pool-maintainer.yaml")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	return path
}

func validConfigYAML() string {
	return `
local_sub2api:
  base_url: "https://api.zhouz.online"
  admin_token_env: "SUB2API_ADMIN_TOKEN"
policy:
  safety_margin: 0.02
  self_built_rate: 0
  sales_groups:
    - name: "0.12"
      group_id: 101
      rate: 0.12
    - name: "0.18"
      group_id: 102
      rate: 0.18
  priority:
    self_built: 1
    upstream_start: 5
    upstream_step: 5
upstreams:
  - id: "mdkj"
    base_url: "https://api.mdkj.lol"
    pricing_page_url: "https://api.mdkj.lol/admin"
    browser_profile: "mdkj"
    group_name_aliases:
      pro: ["pro", "Pro", "PRO"]
accounts:
  - match_name: "https://api.mdkj.lol-pro-*"
    upstream_id: "mdkj"
    upstream_group: "pro"
    allowed_sales_groups: ["0.18"]
self_built_accounts:
  - match_name: "self-*"
    allowed_sales_groups: ["0.12", "0.18"]
`
}

func TestLoadConfigValidatesSalesGroupsAndAccounts(t *testing.T) {
	cfg, err := LoadConfig(writeTempConfig(t, validConfigYAML()))

	require.NoError(t, err)
	require.Equal(t, "SUB2API_ADMIN_TOKEN", cfg.LocalSub2API.AdminTokenEnv)
	group, ok := cfg.SaleGroupByName("0.12")
	require.True(t, ok)
	require.Equal(t, int64(101), group.GroupID)
	upstream, ok := cfg.UpstreamByID("mdkj")
	require.True(t, ok)
	require.Equal(t, "https://api.mdkj.lol", upstream.BaseURL)
	require.InDelta(t, 0.02, cfg.Policy.SafetyMargin, 1e-12)
	require.Equal(t, 1, cfg.Policy.Priority.SelfBuilt)
	require.Equal(t, 5, cfg.Policy.Priority.UpstreamStart)
	require.Equal(t, 5, cfg.Policy.Priority.UpstreamStep)
}

func TestLoadConfigRejectsDuplicateUpstreamIDs(t *testing.T) {
	body := strings.Replace(validConfigYAML(), `accounts:`, `  - id: "mdkj"
    base_url: "https://dup.example"
accounts:`, 1)

	_, err := LoadConfig(writeTempConfig(t, body))

	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate upstream id")
}

func TestLoadConfigRejectsUnknownAllowedSalesGroup(t *testing.T) {
	body := strings.Replace(validConfigYAML(), `allowed_sales_groups: ["0.18"]`, `allowed_sales_groups: ["0.99"]`, 1)

	_, err := LoadConfig(writeTempConfig(t, body))

	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown allowed sales group")
}

func TestLoadConfigRequiresAdminTokenEnv(t *testing.T) {
	body := strings.Replace(validConfigYAML(), `admin_token_env: "SUB2API_ADMIN_TOKEN"`, `admin_token_env: ""`, 1)

	_, err := LoadConfig(writeTempConfig(t, body))

	require.Error(t, err)
	require.Contains(t, err.Error(), "admin_token_env")
}
