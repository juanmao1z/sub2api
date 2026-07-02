package poolmaintainer

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
)

func BuildPlanConfigSnapshot(cfg *Config) PlanConfigSnapshot {
	if cfg == nil {
		return PlanConfigSnapshot{}
	}
	return PlanConfigSnapshot{
		LocalBaseURL:      cfg.LocalSub2API.BaseURL,
		SalesGroups:       append([]SaleGroupConfig(nil), cfg.Policy.SalesGroups...),
		SafetyMargin:      cfg.Policy.SafetyMargin,
		SelfBuiltRate:     cfg.Policy.SelfBuiltRate,
		Priority:          cfg.Policy.Priority,
		Upstreams:         copyUpstreams(cfg.Upstreams),
		Accounts:          copyAccountRules(cfg.Accounts),
		SelfBuiltAccounts: copySelfBuiltAccountRules(cfg.SelfBuiltAccounts),
	}
}

func ValidatePlanConfigSnapshot(cfg *Config, plan *Plan) error {
	if plan == nil {
		return errors.New("plan config is missing")
	}
	current := canonicalPlanConfigSnapshot(BuildPlanConfigSnapshot(cfg))
	planned := canonicalPlanConfigSnapshot(plan.Config)
	if PlanConfigSnapshotsEqual(current, planned) {
		return nil
	}
	return fmt.Errorf("plan config %s mismatch", firstPlanConfigMismatch(current, planned))
}

func PlanConfigSnapshotsEqual(left, right PlanConfigSnapshot) bool {
	left = canonicalPlanConfigSnapshot(left)
	right = canonicalPlanConfigSnapshot(right)
	return reflect.DeepEqual(left, right)
}

func canonicalPlanConfigSnapshot(snapshot PlanConfigSnapshot) PlanConfigSnapshot {
	snapshot.LocalBaseURL = normalizePlanBaseURL(snapshot.LocalBaseURL)
	return snapshot
}

func firstPlanConfigMismatch(current, planned PlanConfigSnapshot) string {
	if current.LocalBaseURL != planned.LocalBaseURL {
		return fmt.Sprintf("local_base_url: current %q, plan %q", current.LocalBaseURL, planned.LocalBaseURL)
	}
	if !floatEqual(current.SafetyMargin, planned.SafetyMargin) {
		return fmt.Sprintf("safety_margin: current %v, plan %v", current.SafetyMargin, planned.SafetyMargin)
	}
	if !floatEqual(current.SelfBuiltRate, planned.SelfBuiltRate) {
		return fmt.Sprintf("self_built_rate: current %v, plan %v", current.SelfBuiltRate, planned.SelfBuiltRate)
	}
	if !reflect.DeepEqual(current.SalesGroups, planned.SalesGroups) {
		return "sales_groups"
	}
	if !reflect.DeepEqual(current.Priority, planned.Priority) {
		return "priority"
	}
	if !reflect.DeepEqual(current.Upstreams, planned.Upstreams) {
		return "upstreams"
	}
	if !reflect.DeepEqual(current.Accounts, planned.Accounts) {
		return "accounts"
	}
	if !reflect.DeepEqual(current.SelfBuiltAccounts, planned.SelfBuiltAccounts) {
		return "self_built_accounts"
	}
	return "snapshot"
}

func normalizePlanBaseURL(raw string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(raw), "/")
	if strings.HasSuffix(trimmed, "/api/v1") {
		return trimmed
	}
	return trimmed + "/api/v1"
}

func floatEqual(left, right float64) bool {
	return math.Abs(left-right) <= 1e-9
}

func copyUpstreams(values []UpstreamConfig) []UpstreamConfig {
	out := append([]UpstreamConfig(nil), values...)
	for i := range out {
		out[i].GroupNameAliases = copyStringSliceMap(out[i].GroupNameAliases)
	}
	return out
}

func copyAccountRules(values []AccountRuleConfig) []AccountRuleConfig {
	out := append([]AccountRuleConfig(nil), values...)
	for i := range out {
		out[i].AllowedSalesGroups = append([]string(nil), out[i].AllowedSalesGroups...)
	}
	return out
}

func copySelfBuiltAccountRules(values []SelfBuiltAccountConfig) []SelfBuiltAccountConfig {
	out := append([]SelfBuiltAccountConfig(nil), values...)
	for i := range out {
		out[i].AllowedSalesGroups = append([]string(nil), out[i].AllowedSalesGroups...)
	}
	return out
}

func copyStringSliceMap(values map[string][]string) map[string][]string {
	if values == nil {
		return nil
	}
	out := make(map[string][]string, len(values))
	for key, value := range values {
		out[key] = append([]string(nil), value...)
	}
	return out
}
