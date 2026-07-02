package poolmaintainer

import (
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"
)

func WritePlanJSON(plan Plan, path string) error {
	if err := ensureParentDir(path); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(plan)
}

func ReadPlanJSON(path string) (*Plan, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var plan Plan
	if err := json.Unmarshal(raw, &plan); err != nil {
		return nil, err
	}
	return &plan, nil
}

func WriteHTMLReport(plan Plan, path string) error {
	if err := ensureParentDir(path); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	view := reportView{
		Plan:        plan,
		Collections: make([]collectionView, 0, len(plan.Collections)),
		Accounts:    make([]accountChangeView, 0, len(plan.Accounts)),
	}
	for _, collection := range plan.Collections {
		view.Collections = append(view.Collections, collectionView{
			CollectionResult: collection,
			Label:            collectionStatusLabel(collection.Status),
			Class:            collectionStatusClass(collection.Status),
		})
	}
	for _, account := range plan.Accounts {
		target := AccountTarget{}
		if account.Target != nil {
			target = *account.Target
		}
		view.Accounts = append(view.Accounts, accountChangeView{
			PlanAccountChange: account,
			CurrentGroups:     fmt.Sprint(account.Current.GroupIDs),
			TargetGroups:      fmt.Sprint(target.GroupIDs),
			Target:            target,
			HasTarget:         account.Target != nil,
		})
	}

	return htmlReportTemplate.Execute(file, view)
}

func ensureParentDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

func collectionStatusLabel(status string) string {
	switch status {
	case CollectionStatusOK:
		return "采集成功"
	case CollectionStatusFailed:
		return "采集失败"
	case CollectionStatusNeedLogin:
		return "需要重新登录"
	default:
		if strings.TrimSpace(status) == "" {
			return "未知"
		}
		return status
	}
}

func collectionStatusClass(status string) string {
	switch status {
	case CollectionStatusFailed:
		return "collection status-failed"
	case CollectionStatusNeedLogin:
		return "collection status-need-login"
	default:
		return "collection status-ok"
	}
}

type reportView struct {
	Plan        Plan
	Collections []collectionView
	Accounts    []accountChangeView
}

type collectionView struct {
	CollectionResult
	Label string
	Class string
}

type accountChangeView struct {
	PlanAccountChange
	CurrentGroups string
	TargetGroups  string
	Target        AccountTarget
	HasTarget     bool
}

var htmlReportTemplate = template.Must(template.New("pool-maintainer-report").Parse(`<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <title>Sub2API Pool Maintainer Report</title>
  <style>
    body { font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; color: #1f2937; margin: 24px; }
    table { border-collapse: collapse; width: 100%; margin: 16px 0 28px; }
    th, td { border: 1px solid #d0d5dd; padding: 8px; text-align: left; vertical-align: top; }
    th { background: #f2f4f7; }
    .summary { display: flex; gap: 16px; flex-wrap: wrap; }
    .metric { border: 1px solid #d0d5dd; padding: 10px 12px; }
    .collection.status-failed, .collection.status-need-login { color: #b42318; font-weight: 700; }
  </style>
</head>
<body>
  <h1>Sub2API Pool Maintainer Report</h1>
  <p>Generated at: {{ .Plan.GeneratedAt }}</p>

  <h2>Summary</h2>
  <div class="summary">
    <div class="metric">total_accounts: {{ .Plan.Summary.TotalAccounts }}</div>
    <div class="metric">ready_changes: {{ .Plan.Summary.ReadyChanges }}</div>
    <div class="metric">blocked_accounts: {{ .Plan.Summary.BlockedAccounts }}</div>
    <div class="metric">noop_accounts: {{ .Plan.Summary.NoopAccounts }}</div>
  </div>

  <h2>Collection Status</h2>
  <table>
    <thead>
      <tr><th>upstream_id</th><th>status</th><th>error</th><th>warnings</th><th>collected_at</th></tr>
    </thead>
    <tbody>
      {{ range .Collections }}
      <tr class="{{ .Class }}">
        <td>{{ .UpstreamID }}</td>
        <td>{{ .Label }}</td>
        <td>{{ .Error }}</td>
        <td>{{ range .Warnings }}{{ . }} {{ end }}</td>
        <td>{{ .CollectedAt }}</td>
      </tr>
      {{ end }}
    </tbody>
  </table>

  <h2>Account Changes</h2>
  <table>
    <thead>
      <tr>
        <th>account_id</th><th>account_name</th><th>status</th><th>risk_level</th>
        <th>current name</th><th>target name</th>
        <th>current rate_multiplier</th><th>target rate_multiplier</th>
        <th>current group_ids</th><th>target group_ids</th>
        <th>current priority</th><th>target priority</th>
        <th>current schedulable</th><th>target schedulable</th>
        <th>reasons</th><th>warnings</th>
      </tr>
    </thead>
    <tbody>
      {{ range .Accounts }}
      <tr>
        <td>{{ .AccountID }}</td>
        <td>{{ .AccountName }}</td>
        <td>{{ .Status }}</td>
        <td>{{ .RiskLevel }}</td>
        <td>{{ .Current.Name }}</td>
        <td>{{ if .HasTarget }}{{ .Target.Name }}{{ end }}</td>
        <td>{{ .Current.RateMultiplier }}</td>
        <td>{{ if .HasTarget }}{{ .Target.RateMultiplier }}{{ end }}</td>
        <td>{{ .CurrentGroups }}</td>
        <td>{{ if .HasTarget }}{{ .TargetGroups }}{{ end }}</td>
        <td>{{ .Current.Priority }}</td>
        <td>{{ if .HasTarget }}{{ .Target.Priority }}{{ end }}</td>
        <td>{{ .Current.Schedulable }}</td>
        <td>{{ if .HasTarget }}{{ .Target.Schedulable }}{{ end }}</td>
        <td>{{ range .Reasons }}{{ . }} {{ end }}</td>
        <td>{{ range .Warnings }}{{ . }} {{ end }}</td>
      </tr>
      {{ end }}
    </tbody>
  </table>
</body>
</html>
`))
