package poolmaintainer

import (
	"math"
	"path"
	"regexp"
	"sort"
	"strconv"
	"time"
)

const (
	AccountKindUpstream  = "upstream"
	AccountKindSelfBuilt = "self_built"
	AccountKindUnmatched = "unmatched"
)

type AccountMatch struct {
	Kind               string
	UpstreamID         string
	UpstreamGroup      string
	AllowedSalesGroups []string
	MatchName          string
}

type planCandidate struct {
	account      AccountSnapshot
	match        AccountMatch
	cost         float64
	targetGroups []SaleGroupConfig
	blocked      bool
	warnings     []string
	reasons      []string
	risk         string
}

var trailingRateSuffixPattern = regexp.MustCompile(`^(.*-)([0-9]+(?:\.[0-9]+)?)$`)

func BuildPlan(cfg *Config, accounts []AccountSnapshot, collections []CollectionResult, now time.Time) Plan {
	plan := Plan{
		GeneratedAt: now,
		Config:      BuildPlanConfigSnapshot(cfg),
		Collections: append([]CollectionResult(nil), collections...),
		Accounts:    make([]PlanAccountChange, 0, len(accounts)),
	}

	collectionByUpstream := make(map[string]CollectionResult, len(collections))
	for _, collection := range collections {
		collectionByUpstream[collection.UpstreamID] = collection
	}

	candidates := make([]planCandidate, 0, len(accounts))
	for _, account := range accounts {
		candidates = append(candidates, buildPlanCandidate(cfg, account, collectionByUpstream))
	}

	priorityByAccountID := assignCostPriorities(cfg, candidates)
	for _, candidate := range candidates {
		change := candidateToPlanChange(candidate, priorityByAccountID)
		plan.Accounts = append(plan.Accounts, change)
		switch change.Status {
		case PlanChangeStatusReady:
			plan.Summary.ReadyChanges++
		case PlanChangeStatusBlocked:
			plan.Summary.BlockedAccounts++
		default:
			plan.Summary.NoopAccounts++
		}
	}
	plan.Summary.TotalAccounts = len(plan.Accounts)
	return plan
}

func MatchAccount(cfg *Config, account AccountSnapshot) AccountMatch {
	if cfg == nil {
		return AccountMatch{Kind: AccountKindUnmatched}
	}
	for _, rule := range cfg.SelfBuiltAccounts {
		if globMatches(rule.MatchName, account.Name) {
			return AccountMatch{
				Kind:               AccountKindSelfBuilt,
				AllowedSalesGroups: append([]string(nil), rule.AllowedSalesGroups...),
				MatchName:          rule.MatchName,
			}
		}
	}
	for _, rule := range cfg.Accounts {
		if globMatches(rule.MatchName, account.Name) {
			return AccountMatch{
				Kind:               AccountKindUpstream,
				UpstreamID:         rule.UpstreamID,
				UpstreamGroup:      rule.UpstreamGroup,
				AllowedSalesGroups: append([]string(nil), rule.AllowedSalesGroups...),
				MatchName:          rule.MatchName,
			}
		}
	}
	return AccountMatch{Kind: AccountKindUnmatched}
}

func FormatRateSuffix(rate float64) string {
	return strconv.FormatFloat(rate, 'f', -1, 64)
}

func buildPlanCandidate(cfg *Config, account AccountSnapshot, collections map[string]CollectionResult) planCandidate {
	match := MatchAccount(cfg, account)
	candidate := planCandidate{
		account:  account,
		match:    match,
		risk:     RiskLevelInfo,
		warnings: []string{},
		reasons:  []string{},
	}

	switch match.Kind {
	case AccountKindSelfBuilt:
		candidate.cost = cfg.Policy.SelfBuiltRate
	case AccountKindUpstream:
		collection, ok := collections[match.UpstreamID]
		if !ok {
			candidate.blocked = true
			candidate.risk = RiskLevelError
			candidate.warnings = append(candidate.warnings, "采集失败")
			candidate.reasons = append(candidate.reasons, "upstream collection result is missing")
			return candidate
		}
		if collection.Status != CollectionStatusOK {
			candidate.blocked = true
			candidate.risk = RiskLevelError
			candidate.warnings = append(candidate.warnings, collection.Warnings...)
			if collection.Status == CollectionStatusNeedLogin {
				candidate.warnings = append(candidate.warnings, "需要重新登录")
			} else {
				candidate.warnings = append(candidate.warnings, "采集失败")
			}
			if collection.Error != "" {
				candidate.reasons = append(candidate.reasons, collection.Error)
			}
			return candidate
		}
		rate, ok := collectedRateForGroup(collection, match.UpstreamGroup)
		if !ok {
			candidate.blocked = true
			candidate.risk = RiskLevelError
			candidate.warnings = append(candidate.warnings, "采集失败")
			candidate.reasons = append(candidate.reasons, "upstream group rate is missing")
			return candidate
		}
		candidate.cost = rate
	default:
		candidate.blocked = true
		candidate.risk = RiskLevelWarning
		candidate.warnings = append(candidate.warnings, "account is not matched by config")
		return candidate
	}

	candidate.targetGroups = admittedSalesGroups(cfg, match.AllowedSalesGroups, candidate.cost)
	if len(candidate.targetGroups) == 0 {
		candidate.risk = RiskLevelWarning
		candidate.reasons = append(candidate.reasons, "rate exceeds every allowed sales group admission line")
	} else {
		candidate.reasons = append(candidate.reasons, "matched allowed sales groups within safety margin")
	}
	return candidate
}

func assignCostPriorities(cfg *Config, candidates []planCandidate) map[int64]int {
	ranksByGroupID := make(map[int64]map[int64]int)
	for _, group := range cfg.Policy.SalesGroups {
		rates := make([]float64, 0)
		seen := map[string]struct{}{}
		for _, candidate := range candidates {
			if candidate.blocked || !candidateUsesCostPriority(candidate) {
				continue
			}
			if !candidateTargetsGroup(candidate, group.GroupID) {
				continue
			}
			key := FormatRateSuffix(candidate.cost)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			rates = append(rates, candidate.cost)
		}
		sort.Float64s(rates)
		rank := make(map[int64]int, len(rates))
		for index, rate := range rates {
			priority := cfg.Policy.Priority.UpstreamStart + index*cfg.Policy.Priority.UpstreamStep
			rank[rateKey(rate)] = priority
		}
		ranksByGroupID[group.GroupID] = rank
	}

	priorityByAccountID := make(map[int64]int)
	for _, candidate := range candidates {
		if candidate.blocked || !candidateUsesCostPriority(candidate) || len(candidate.targetGroups) == 0 {
			continue
		}
		priority := cfg.Policy.Priority.UpstreamStart
		for index, group := range candidate.targetGroups {
			groupPriority := ranksByGroupID[group.GroupID][rateKey(candidate.cost)]
			if index == 0 || groupPriority > priority {
				priority = groupPriority
			}
		}
		priorityByAccountID[candidate.account.ID] = priority
	}
	return priorityByAccountID
}

func candidateToPlanChange(candidate planCandidate, priorityByAccountID map[int64]int) PlanAccountChange {
	change := PlanAccountChange{
		AccountID:     candidate.account.ID,
		AccountName:   candidate.account.Name,
		Kind:          candidate.match.Kind,
		UpstreamID:    candidate.match.UpstreamID,
		UpstreamGroup: candidate.match.UpstreamGroup,
		AllowedGroups: append([]string(nil), candidate.match.AllowedSalesGroups...),
		Status:        PlanChangeStatusBlocked,
		RiskLevel:     candidate.risk,
		Reasons:       append([]string(nil), candidate.reasons...),
		Warnings:      dedupeStrings(candidate.warnings),
		Current:       candidate.account,
	}
	if change.RiskLevel == "" {
		change.RiskLevel = RiskLevelInfo
	}
	if candidate.blocked {
		return change
	}

	target := AccountTarget{
		Name:           candidate.account.Name,
		RateMultiplier: candidate.cost,
		GroupIDs:       saleGroupIDs(candidate.targetGroups),
		Priority:       candidate.account.Priority,
		Schedulable:    len(candidate.targetGroups) > 0,
	}
	switch candidate.match.Kind {
	case AccountKindSelfBuilt:
		if priority, ok := priorityByAccountID[candidate.account.ID]; ok {
			target.Priority = priority
		}
	case AccountKindUpstream:
		target.Name = renameTrailingRateSuffix(candidate.account.Name, candidate.cost)
		if priority, ok := priorityByAccountID[candidate.account.ID]; ok {
			target.Priority = priority
		}
	}

	if !target.Schedulable {
		change.RiskLevel = RiskLevelWarning
	}
	if accountTargetEqual(candidate.account, target) {
		change.Status = PlanChangeStatusNoop
		change.Target = nil
		return change
	}
	change.Status = PlanChangeStatusReady
	change.Target = &target
	return change
}

func candidateUsesCostPriority(candidate planCandidate) bool {
	return candidate.match.Kind == AccountKindUpstream || candidate.match.Kind == AccountKindSelfBuilt
}

func admittedSalesGroups(cfg *Config, allowed []string, cost float64) []SaleGroupConfig {
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, name := range allowed {
		allowedSet[name] = struct{}{}
	}
	targets := make([]SaleGroupConfig, 0, len(cfg.Policy.SalesGroups))
	for _, group := range cfg.Policy.SalesGroups {
		if _, ok := allowedSet[group.Name]; !ok {
			continue
		}
		if cost <= group.Rate-cfg.Policy.SafetyMargin+1e-9 {
			targets = append(targets, group)
		}
	}
	return targets
}

func collectedRateForGroup(collection CollectionResult, group string) (float64, bool) {
	for _, rate := range collection.Rates {
		if rate.UpstreamGroup == group {
			return rate.Rate, true
		}
	}
	return 0, false
}

func candidateTargetsGroup(candidate planCandidate, groupID int64) bool {
	for _, group := range candidate.targetGroups {
		if group.GroupID == groupID {
			return true
		}
	}
	return false
}

func saleGroupIDs(groups []SaleGroupConfig) []int64 {
	ids := make([]int64, 0, len(groups))
	for _, group := range groups {
		ids = append(ids, group.GroupID)
	}
	return ids
}

func renameTrailingRateSuffix(name string, rate float64) string {
	suffix := FormatRateSuffix(rate)
	matches := trailingRateSuffixPattern.FindStringSubmatch(name)
	if len(matches) == 3 {
		return matches[1] + suffix
	}
	return name + "-" + suffix
}

func accountTargetEqual(current AccountSnapshot, target AccountTarget) bool {
	return current.Name == target.Name &&
		math.Abs(current.RateMultiplier-target.RateMultiplier) <= 1e-9 &&
		int64SlicesEqual(normalizeInt64Slice(current.GroupIDs), normalizeInt64Slice(target.GroupIDs)) &&
		current.Priority == target.Priority &&
		current.Schedulable == target.Schedulable
}

func normalizeInt64Slice(values []int64) []int64 {
	out := append([]int64(nil), values...)
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func int64SlicesEqual(left, right []int64) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func dedupeStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func rateKey(rate float64) int64 {
	return int64(math.Round(rate * 1_000_000))
}

func globMatches(pattern, value string) bool {
	matched, err := path.Match(pattern, value)
	return err == nil && matched
}
