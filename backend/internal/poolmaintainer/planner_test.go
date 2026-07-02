package poolmaintainer

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func plannerTestConfig() *Config {
	return &Config{
		LocalSub2API: LocalSub2APIConfig{
			BaseURL:       "https://api.zhouz.online",
			AdminTokenEnv: "SUB2API_ADMIN_TOKEN",
		},
		Policy: PolicyConfig{
			SafetyMargin:  0.02,
			SelfBuiltRate: 0,
			SalesGroups: []SaleGroupConfig{
				{Name: "0.12", GroupID: 12, Rate: 0.12},
				{Name: "0.18", GroupID: 18, Rate: 0.18},
				{Name: "0.25", GroupID: 25, Rate: 0.25},
			},
			Priority: PriorityConfig{
				SelfBuilt:     1,
				UpstreamStart: 5,
				UpstreamStep:  5,
			},
		},
		Upstreams: []UpstreamConfig{
			{ID: "mdkj", BaseURL: "https://api.mdkj.lol", PricingPageURL: "https://api.mdkj.lol/admin"},
		},
		Accounts: []AccountRuleConfig{
			{
				MatchName:          "https://api.mdkj.lol-pro-*",
				UpstreamID:         "mdkj",
				UpstreamGroup:      "pro",
				AllowedSalesGroups: []string{"0.12", "0.18"},
			},
			{
				MatchName:          "https://api.mdkj.lol-plus-*",
				UpstreamID:         "mdkj",
				UpstreamGroup:      "plus",
				AllowedSalesGroups: []string{"0.18", "0.25"},
			},
			{
				MatchName:          "https://api.mdkj.lol-max-*",
				UpstreamID:         "mdkj",
				UpstreamGroup:      "max",
				AllowedSalesGroups: []string{"0.12", "0.18", "0.25"},
			},
		},
		SelfBuiltAccounts: []SelfBuiltAccountConfig{
			{MatchName: "self-*", AllowedSalesGroups: []string{"0.12", "0.18", "0.25"}},
		},
	}
}

func accountSnapshot(id int64, name string, groups []int64, priority int, rate float64, schedulable bool) AccountSnapshot {
	return AccountSnapshot{
		ID:             id,
		Name:           name,
		RateMultiplier: rate,
		GroupIDs:       groups,
		Priority:       priority,
		Schedulable:    schedulable,
	}
}

func collectionResult(now time.Time, status string, rates ...CollectedRate) CollectionResult {
	for i := range rates {
		rates[i].CollectedAt = now
	}
	return CollectionResult{
		UpstreamID:  "mdkj",
		Status:      status,
		Rates:       rates,
		CollectedAt: now,
	}
}

func collected(group string, rate float64) CollectedRate {
	return CollectedRate{
		UpstreamID:    "mdkj",
		UpstreamGroup: group,
		Rate:          rate,
		Source:        "fixture",
	}
}

func findPlanChange(t *testing.T, plan Plan, accountID int64) PlanAccountChange {
	t.Helper()
	for _, change := range plan.Accounts {
		if change.AccountID == accountID {
			return change
		}
	}
	t.Fatalf("plan change for account %d not found", accountID)
	return PlanAccountChange{}
}

func TestBuildPlanAppliesSafetyMarginAndWhitelist(t *testing.T) {
	now := time.Date(2026, 7, 3, 4, 30, 0, 0, time.UTC)
	cfg := plannerTestConfig()
	accounts := []AccountSnapshot{
		accountSnapshot(1, "https://api.mdkj.lol-pro-0.2", []int64{18, 25}, 50, 0.2, true),
	}
	collections := []CollectionResult{
		collectionResult(now, CollectionStatusOK, collected("pro", 0.10)),
	}

	plan := BuildPlan(cfg, accounts, collections, now)

	change := findPlanChange(t, plan, 1)
	require.Equal(t, PlanChangeStatusReady, change.Status)
	require.Equal(t, RiskLevelInfo, change.RiskLevel)
	require.NotNil(t, change.Target)
	require.Equal(t, "https://api.mdkj.lol-pro-0.1", change.Target.Name)
	require.InDelta(t, 0.10, change.Target.RateMultiplier, 1e-12)
	require.Equal(t, []int64{12, 18}, change.Target.GroupIDs)
	require.True(t, change.Target.Schedulable)
	require.NotContains(t, change.Target.GroupIDs, int64(25), "white list must block cheap-account expansion")
}

func TestBuildPlanAssignsPriorityPerSalesGroupByCost(t *testing.T) {
	now := time.Date(2026, 7, 3, 4, 31, 0, 0, time.UTC)
	cfg := plannerTestConfig()
	accounts := []AccountSnapshot{
		accountSnapshot(1, "self-main", []int64{12, 18, 25}, 50, 0.5, true),
		accountSnapshot(2, "https://api.mdkj.lol-pro-0.2", []int64{18}, 50, 0.2, true),
		accountSnapshot(3, "https://api.mdkj.lol-plus-0.2", []int64{18}, 50, 0.2, true),
	}
	collections := []CollectionResult{
		collectionResult(now, CollectionStatusOK, collected("pro", 0.08), collected("plus", 0.15)),
	}

	plan := BuildPlan(cfg, accounts, collections, now)

	selfBuilt := findPlanChange(t, plan, 1)
	cheap := findPlanChange(t, plan, 2)
	costlier := findPlanChange(t, plan, 3)
	require.NotNil(t, selfBuilt.Target)
	require.NotNil(t, cheap.Target)
	require.NotNil(t, costlier.Target)
	require.Equal(t, 1, selfBuilt.Target.Priority)
	require.Equal(t, 5, cheap.Target.Priority)
	require.Equal(t, 10, costlier.Target.Priority)
	require.Equal(t, []int64{18, 25}, costlier.Target.GroupIDs)
}

func TestBuildPlanKeepsFailedCollectionAccountsUnchanged(t *testing.T) {
	now := time.Date(2026, 7, 3, 4, 32, 0, 0, time.UTC)
	cfg := plannerTestConfig()
	accounts := []AccountSnapshot{
		accountSnapshot(1, "https://api.mdkj.lol-pro-0.2", []int64{18}, 50, 0.2, true),
	}
	collections := []CollectionResult{
		{
			UpstreamID:  "mdkj",
			Status:      CollectionStatusNeedLogin,
			Warnings:    []string{"需要重新登录"},
			Error:       "login required",
			CollectedAt: now,
		},
	}

	plan := BuildPlan(cfg, accounts, collections, now)

	change := findPlanChange(t, plan, 1)
	require.Equal(t, PlanChangeStatusBlocked, change.Status)
	require.Equal(t, RiskLevelError, change.RiskLevel)
	require.Nil(t, change.Target)
	require.Contains(t, change.Warnings, "需要重新登录")
}

func TestBuildPlanSuggestsUnschedulableWhenTooExpensive(t *testing.T) {
	now := time.Date(2026, 7, 3, 4, 33, 0, 0, time.UTC)
	cfg := plannerTestConfig()
	accounts := []AccountSnapshot{
		accountSnapshot(1, "https://api.mdkj.lol-max-0.2", []int64{25}, 50, 0.2, true),
	}
	collections := []CollectionResult{
		collectionResult(now, CollectionStatusOK, collected("max", 0.24)),
	}

	plan := BuildPlan(cfg, accounts, collections, now)

	change := findPlanChange(t, plan, 1)
	require.Equal(t, PlanChangeStatusReady, change.Status)
	require.Equal(t, RiskLevelWarning, change.RiskLevel)
	require.NotNil(t, change.Target)
	require.False(t, change.Target.Schedulable)
	require.Empty(t, change.Target.GroupIDs)
	require.InDelta(t, 0.24, change.Target.RateMultiplier, 1e-12)
}

func TestBuildPlanRenamesOnlyTrailingRateSuffix(t *testing.T) {
	now := time.Date(2026, 7, 3, 4, 34, 0, 0, time.UTC)
	cfg := plannerTestConfig()
	accounts := []AccountSnapshot{
		accountSnapshot(1, "https://api.mdkj.lol-pro-0.2000", []int64{18}, 50, 0.2, true),
	}
	collections := []CollectionResult{
		collectionResult(now, CollectionStatusOK, collected("pro", 0.16)),
	}

	plan := BuildPlan(cfg, accounts, collections, now)

	change := findPlanChange(t, plan, 1)
	require.NotNil(t, change.Target)
	require.Equal(t, "https://api.mdkj.lol-pro-0.16", change.Target.Name)
	require.Equal(t, "0.16", FormatRateSuffix(0.16))
}
