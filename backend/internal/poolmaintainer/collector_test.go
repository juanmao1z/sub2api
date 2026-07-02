package poolmaintainer

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestExtractRatesFromHTMLReadsTableRows(t *testing.T) {
	upstream := UpstreamConfig{
		ID: "mdkj",
		GroupNameAliases: map[string][]string{
			"pro": {"pro"},
			"max": {"max"},
		},
	}

	raw := `
<table>
  <tr><th>Group</th><th>Rate</th></tr>
  <tr><td>pro</td><td>0.20</td></tr>
  <tr><td>max</td><td>0.35</td></tr>
</table>`

	result := ExtractRatesFromHTML(upstream, raw)

	require.Equal(t, "mdkj", result.UpstreamID)
	require.Equal(t, CollectionStatusOK, result.Status)
	require.Len(t, result.Rates, 2)
	require.Equal(t, "pro", result.Rates[0].UpstreamGroup)
	require.InDelta(t, 0.20, result.Rates[0].Rate, 1e-12)
	require.Equal(t, "max", result.Rates[1].UpstreamGroup)
	require.InDelta(t, 0.35, result.Rates[1].Rate, 1e-12)
	require.Empty(t, result.Warnings)
	require.Empty(t, result.Error)
}

func TestExtractRatesFromHTMLUsesAliases(t *testing.T) {
	upstream := UpstreamConfig{
		ID: "mdkj",
		GroupNameAliases: map[string][]string{
			"pro": {"专业版", "Pro"},
		},
	}

	raw := `<div><span>专业版</span><span>倍率 0.28</span></div>`

	result := ExtractRatesFromHTML(upstream, raw)

	require.Equal(t, CollectionStatusOK, result.Status)
	require.Len(t, result.Rates, 1)
	require.Equal(t, "pro", result.Rates[0].UpstreamGroup)
	require.InDelta(t, 0.28, result.Rates[0].Rate, 1e-12)
}

func TestExtractRatesFromHTMLMarksNeedLogin(t *testing.T) {
	upstream := UpstreamConfig{
		ID: "mdkj",
		GroupNameAliases: map[string][]string{
			"pro": {"pro"},
		},
	}

	raw := `<html><body><h1>登录</h1><p>请输入验证码后 sign in</p></body></html>`

	result := ExtractRatesFromHTML(upstream, raw)

	require.Equal(t, CollectionStatusNeedLogin, result.Status)
	require.Empty(t, result.Rates)
	require.Empty(t, result.Error)
}

func TestReadCollectionSnapshotsKeepsMissingFileAsFailed(t *testing.T) {
	now := time.Date(2026, 7, 3, 5, 0, 0, 0, time.UTC)
	htmlDir := t.TempDir()
	existingPath := filepath.Join(htmlDir, "mdkj.html")
	require.NoError(t, os.WriteFile(existingPath, []byte(`<div>pro: 0.22</div>`), 0o600))

	cfg := &Config{
		Upstreams: []UpstreamConfig{
			{
				ID: "mdkj",
				GroupNameAliases: map[string][]string{
					"pro": {"pro"},
				},
			},
			{
				ID: "missing",
				GroupNameAliases: map[string][]string{
					"basic": {"basic"},
				},
			},
		},
	}

	results := ReadCollectionSnapshots(cfg, htmlDir, now)

	require.Len(t, results, 2)
	require.Equal(t, "mdkj", results[0].UpstreamID)
	require.Equal(t, CollectionStatusOK, results[0].Status)
	require.Equal(t, existingPath, results[0].Snapshot)
	require.Equal(t, now, results[0].CollectedAt)
	require.Len(t, results[0].Rates, 1)
	require.Equal(t, "missing", results[1].UpstreamID)
	require.Equal(t, CollectionStatusFailed, results[1].Status)
	require.Equal(t, filepath.Join(htmlDir, "missing.html"), results[1].Snapshot)
	require.Equal(t, now, results[1].CollectedAt)
	require.Contains(t, results[1].Error, "missing.html")
}
