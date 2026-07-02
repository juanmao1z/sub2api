package poolmaintainer

import "time"

const (
	CollectionStatusOK        = "ok"
	CollectionStatusFailed    = "failed"
	CollectionStatusNeedLogin = "need_login"

	PlanChangeStatusReady   = "ready"
	PlanChangeStatusBlocked = "blocked"
	PlanChangeStatusNoop    = "noop"

	RiskLevelInfo    = "info"
	RiskLevelWarning = "warning"
	RiskLevelError   = "error"
)

type AccountSnapshot struct {
	ID             int64     `json:"id"`
	Name           string    `json:"name"`
	Platform       string    `json:"platform,omitempty"`
	Type           string    `json:"type,omitempty"`
	RateMultiplier float64   `json:"rate_multiplier"`
	GroupIDs       []int64   `json:"group_ids,omitempty"`
	Priority       int       `json:"priority"`
	Schedulable    bool      `json:"schedulable"`
	UpdatedAt      time.Time `json:"updated_at,omitempty"`
}

type CollectedRate struct {
	UpstreamID    string    `json:"upstream_id"`
	UpstreamGroup string    `json:"upstream_group"`
	Rate          float64   `json:"rate"`
	Source        string    `json:"source,omitempty"`
	CollectedAt   time.Time `json:"collected_at"`
}

type CollectionResult struct {
	UpstreamID  string          `json:"upstream_id"`
	Status      string          `json:"status"`
	Rates       []CollectedRate `json:"rates,omitempty"`
	Warnings    []string        `json:"warnings,omitempty"`
	Error       string          `json:"error,omitempty"`
	Snapshot    string          `json:"snapshot,omitempty"`
	CollectedAt time.Time       `json:"collected_at"`
}

type Plan struct {
	GeneratedAt time.Time           `json:"generated_at"`
	Config      PlanConfigSnapshot  `json:"config"`
	Collections []CollectionResult  `json:"collections"`
	Accounts    []PlanAccountChange `json:"accounts"`
	Summary     PlanSummary         `json:"summary"`
}

type PlanConfigSnapshot struct {
	LocalBaseURL string            `json:"local_base_url"`
	SalesGroups  []SaleGroupConfig `json:"sales_groups"`
	SafetyMargin float64           `json:"safety_margin"`
}

type PlanSummary struct {
	TotalAccounts   int `json:"total_accounts"`
	ReadyChanges    int `json:"ready_changes"`
	BlockedAccounts int `json:"blocked_accounts"`
	NoopAccounts    int `json:"noop_accounts"`
}

type PlanAccountChange struct {
	AccountID     int64           `json:"account_id"`
	AccountName   string          `json:"account_name"`
	Kind          string          `json:"kind"`
	UpstreamID    string          `json:"upstream_id,omitempty"`
	UpstreamGroup string          `json:"upstream_group,omitempty"`
	AllowedGroups []string        `json:"allowed_sales_groups,omitempty"`
	Status        string          `json:"status"`
	RiskLevel     string          `json:"risk_level"`
	Reasons       []string        `json:"reasons,omitempty"`
	Warnings      []string        `json:"warnings,omitempty"`
	Current       AccountSnapshot `json:"current"`
	Target        *AccountTarget  `json:"target,omitempty"`
}

type AccountTarget struct {
	Name           string  `json:"name"`
	RateMultiplier float64 `json:"rate_multiplier"`
	GroupIDs       []int64 `json:"group_ids"`
	Priority       int     `json:"priority"`
	Schedulable    bool    `json:"schedulable"`
}

type ApplyResult struct {
	AppliedAt time.Time            `json:"applied_at"`
	DryRun    bool                 `json:"dry_run"`
	Results   []ApplyAccountResult `json:"results"`
	Summary   ApplySummary         `json:"summary"`
}

type ApplyAccountResult struct {
	AccountID int64  `json:"account_id"`
	Status    string `json:"status"`
	Message   string `json:"message,omitempty"`
}

type ApplySummary struct {
	Success   int `json:"success"`
	Skipped   int `json:"skipped"`
	Conflicts int `json:"conflicts"`
	Failed    int `json:"failed"`
}
