package poolmaintainer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestWritePlanJSONRoundTrips(t *testing.T) {
	plan := sampleReportPlan()
	path := filepath.Join(t.TempDir(), "plan.json")

	require.NoError(t, WritePlanJSON(plan, path))
	read, err := ReadPlanJSON(path)

	require.NoError(t, err)
	require.Equal(t, plan, *read)
}

func TestWriteHTMLReportMarksFailedCollectionRed(t *testing.T) {
	plan := sampleReportPlan()
	path := filepath.Join(t.TempDir(), "report.html")

	require.NoError(t, WriteHTMLReport(plan, path))
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	html := string(raw)

	require.Contains(t, html, `class="collection status-failed"`)
	require.Contains(t, html, `class="collection status-need-login"`)
	require.Contains(t, html, "采集失败")
	require.Contains(t, html, "需要重新登录")
	require.Contains(t, html, "color: #b42318")
}

func TestWriteHTMLReportIncludesFiveChangeFields(t *testing.T) {
	plan := sampleReportPlan()
	path := filepath.Join(t.TempDir(), "report.html")

	require.NoError(t, WriteHTMLReport(plan, path))
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	html := string(raw)

	for _, field := range []string{"name", "rate_multiplier", "group_ids", "priority", "schedulable"} {
		require.Contains(t, html, field)
	}
	for _, value := range []string{"current-name", "target-name", "1.2", "0.98", "[10 20]", "[30 40]", "7", "3", "true", "false"} {
		require.Contains(t, html, value)
	}
	for _, secret := range []string{"token", "credential", "password", "secret"} {
		require.NotContains(t, strings.ToLower(html), secret)
	}
}

func sampleReportPlan() Plan {
	collectedAt := time.Date(2026, 7, 3, 4, 30, 0, 0, time.UTC)
	return Plan{
		GeneratedAt: collectedAt,
		Config: PlanConfigSnapshot{
			LocalBaseURL: "https://api.example.test",
			SalesGroups: []SaleGroupConfig{
				{Name: "basic", GroupID: 10, Rate: 0.12},
			},
			SafetyMargin: 0.02,
		},
		Collections: []CollectionResult{
			{UpstreamID: "ok-site", Status: CollectionStatusOK, CollectedAt: collectedAt},
			{UpstreamID: "failed-site", Status: CollectionStatusFailed, Error: "timeout", CollectedAt: collectedAt},
			{UpstreamID: "login-site", Status: CollectionStatusNeedLogin, Error: "session expired", CollectedAt: collectedAt},
		},
		Accounts: []PlanAccountChange{
			{
				AccountID:   42,
				AccountName: "current-name",
				Kind:        "upstream",
				Status:      PlanChangeStatusReady,
				RiskLevel:   RiskLevelInfo,
				Current: AccountSnapshot{
					ID:             42,
					Name:           "current-name",
					RateMultiplier: 1.2,
					GroupIDs:       []int64{10, 20},
					Priority:       7,
					Schedulable:    true,
					UpdatedAt:      collectedAt,
				},
				Target: &AccountTarget{
					Name:           "target-name",
					RateMultiplier: 0.98,
					GroupIDs:       []int64{30, 40},
					Priority:       3,
					Schedulable:    false,
				},
			},
		},
		Summary: PlanSummary{
			TotalAccounts:   1,
			ReadyChanges:    1,
			BlockedAccounts: 0,
			NoopAccounts:    0,
		},
	}
}
