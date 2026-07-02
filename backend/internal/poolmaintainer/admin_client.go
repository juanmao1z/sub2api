package poolmaintainer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

const (
	applyStatusApplied  = "applied"
	applyStatusConflict = "conflict"
	applyStatusDryRun   = "dry_run"
	applyStatusFailed   = "failed"
	applyStatusSkipped  = "skipped"
)

type AdminClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

func NewAdminClient(baseURL, token string, httpClient *http.Client) *AdminClient {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &AdminClient{
		baseURL:    normalizeAdminBaseURL(baseURL),
		token:      token,
		httpClient: httpClient,
	}
}

func (c *AdminClient) ListAccounts(ctx context.Context) ([]AccountSnapshot, error) {
	const pageSize = 1000
	accounts := make([]AccountSnapshot, 0)
	for page := 1; ; page++ {
		var response adminListAccountsData
		endpoint := fmt.Sprintf("/admin/accounts?page=%d&page_size=%d&sort_by=name&sort_order=asc", page, pageSize)
		if err := c.do(ctx, http.MethodGet, endpoint, nil, &response); err != nil {
			return nil, err
		}
		accounts = append(accounts, response.Items...)

		pages := response.Pages
		if pages <= 0 && response.Total > 0 && response.PageSize > 0 {
			pages = (response.Total + response.PageSize - 1) / response.PageSize
		}
		if pages <= 0 {
			pages = 1
		}
		if page >= pages {
			return accounts, nil
		}
	}
}

func (c *AdminClient) GetAccount(ctx context.Context, id int64) (AccountSnapshot, error) {
	var account AccountSnapshot
	err := c.do(ctx, http.MethodGet, fmt.Sprintf("/admin/accounts/%d", id), nil, &account)
	return account, err
}

func (c *AdminClient) ApplyPlan(ctx context.Context, plan *Plan, dryRun bool) (*ApplyResult, error) {
	result := &ApplyResult{
		AppliedAt: time.Now().UTC(),
		DryRun:    dryRun,
	}
	if plan == nil {
		return result, nil
	}

	for _, change := range plan.Accounts {
		accountResult := ApplyAccountResult{AccountID: change.AccountID}
		if change.Status != PlanChangeStatusReady || change.Target == nil {
			accountResult.Status = applyStatusSkipped
			accountResult.Message = "no target mutation"
			result.Results = append(result.Results, accountResult)
			result.Summary.Skipped++
			continue
		}

		live, err := c.GetAccount(ctx, change.AccountID)
		if err != nil {
			accountResult.Status = applyStatusFailed
			accountResult.Message = err.Error()
			result.Results = append(result.Results, accountResult)
			result.Summary.Failed++
			continue
		}
		if !accountSnapshotMatchesPlannedCurrent(live, change.Current) {
			accountResult.Status = applyStatusConflict
			accountResult.Message = "account changed since plan was generated"
			result.Results = append(result.Results, accountResult)
			result.Summary.Conflicts++
			continue
		}

		if dryRun {
			accountResult.Status = applyStatusDryRun
			accountResult.Message = "mutation skipped by dry run"
			result.Results = append(result.Results, accountResult)
			result.Summary.Success++
			continue
		}

		if live.Schedulable && !change.Target.Schedulable {
			if err := c.updateSchedulable(ctx, change.AccountID, false); err != nil {
				accountResult.Status = applyStatusFailed
				accountResult.Message = err.Error()
				result.Results = append(result.Results, accountResult)
				result.Summary.Failed++
				continue
			}
		}

		if err := c.updateAccount(ctx, change.AccountID, *change.Target); err != nil {
			accountResult.Status = applyStatusFailed
			accountResult.Message = err.Error()
			result.Results = append(result.Results, accountResult)
			result.Summary.Failed++
			continue
		}
		if !live.Schedulable && change.Target.Schedulable {
			if err := c.updateSchedulable(ctx, change.AccountID, true); err != nil {
				accountResult.Status = applyStatusFailed
				accountResult.Message = err.Error()
				result.Results = append(result.Results, accountResult)
				result.Summary.Failed++
				continue
			}
		}
		final, err := c.GetAccount(ctx, change.AccountID)
		if err != nil {
			accountResult.Status = applyStatusFailed
			accountResult.Message = err.Error()
			result.Results = append(result.Results, accountResult)
			result.Summary.Failed++
			continue
		}
		if !accountSnapshotMatchesTarget(final, change.Current, *change.Target) {
			accountResult.Status = applyStatusFailed
			accountResult.Message = "final account state does not match target"
			result.Results = append(result.Results, accountResult)
			result.Summary.Failed++
			continue
		}

		accountResult.Status = applyStatusApplied
		result.Results = append(result.Results, accountResult)
		result.Summary.Success++
	}
	return result, nil
}

func (c *AdminClient) updateAccount(ctx context.Context, id int64, target AccountTarget) error {
	payload := adminUpdateAccountRequest{
		Name:                    target.Name,
		Priority:                target.Priority,
		RateMultiplier:          target.RateMultiplier,
		GroupIDs:                target.GroupIDs,
		ConfirmMixedChannelRisk: true,
	}
	return c.do(ctx, http.MethodPut, fmt.Sprintf("/admin/accounts/%d", id), payload, nil)
}

func (c *AdminClient) updateSchedulable(ctx context.Context, id int64, schedulable bool) error {
	payload := adminSchedulableRequest{Schedulable: schedulable}
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/admin/accounts/%d/schedulable", id), payload, nil)
}

func (c *AdminClient) do(ctx context.Context, method, endpoint string, payload any, out any) error {
	var body *bytes.Reader
	if payload == nil {
		body = bytes.NewReader(nil)
	} else {
		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(raw)
	}

	requestURL, err := c.url(endpoint)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, method, requestURL, body)
	if err != nil {
		return err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if strings.TrimSpace(c.token) != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("admin api %s %s returned status %d", method, endpoint, resp.StatusCode)
	}

	var envelope adminEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return err
	}
	if envelope.Code != 0 {
		if envelope.Message == "" {
			envelope.Message = "admin api error"
		}
		return fmt.Errorf("admin api %s %s failed: %s", method, endpoint, envelope.Message)
	}
	if out == nil {
		return nil
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return nil
	}
	return json.Unmarshal(envelope.Data, out)
}

func (c *AdminClient) url(endpoint string) (string, error) {
	base, err := url.Parse(c.baseURL)
	if err != nil {
		return "", err
	}
	endpointPath, rawQuery, _ := strings.Cut(endpoint, "?")
	base.Path = path.Join(base.Path, endpointPath)
	base.RawQuery = rawQuery
	return base.String(), nil
}

func normalizeAdminBaseURL(raw string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(raw), "/")
	if strings.HasSuffix(trimmed, "/api/v1") {
		return trimmed
	}
	return trimmed + "/api/v1"
}

func accountSnapshotMatchesPlannedCurrent(live, planned AccountSnapshot) bool {
	return live.Name == planned.Name &&
		live.Platform == planned.Platform &&
		live.Type == planned.Type &&
		live.Status == planned.Status &&
		floatEqual(live.RateMultiplier, planned.RateMultiplier) &&
		int64SlicesEqual(normalizeInt64Slice(live.GroupIDs), normalizeInt64Slice(planned.GroupIDs)) &&
		live.Priority == planned.Priority &&
		live.Schedulable == planned.Schedulable
}

func accountSnapshotMatchesTarget(live AccountSnapshot, planned AccountSnapshot, target AccountTarget) bool {
	return live.Name == target.Name &&
		live.Platform == planned.Platform &&
		live.Type == planned.Type &&
		floatEqual(live.RateMultiplier, target.RateMultiplier) &&
		int64SlicesEqual(normalizeInt64Slice(live.GroupIDs), normalizeInt64Slice(target.GroupIDs)) &&
		live.Priority == target.Priority &&
		live.Schedulable == target.Schedulable
}

type adminEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type adminListAccountsData struct {
	Items    []AccountSnapshot `json:"items"`
	Total    int               `json:"total"`
	Page     int               `json:"page"`
	PageSize int               `json:"page_size"`
	Pages    int               `json:"pages"`
}

type adminUpdateAccountRequest struct {
	Name                    string  `json:"name"`
	Priority                int     `json:"priority"`
	RateMultiplier          float64 `json:"rate_multiplier"`
	GroupIDs                []int64 `json:"group_ids"`
	ConfirmMixedChannelRisk bool    `json:"confirm_mixed_channel_risk"`
}

type adminSchedulableRequest struct {
	Schedulable bool `json:"schedulable"`
}
