package poolmaintainer

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAdminClientUnwrapsResponseEnvelope(t *testing.T) {
	var sawAuth bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/admin/accounts", r.URL.Path)
		require.Equal(t, "1", r.URL.Query().Get("page"))
		require.Equal(t, "1000", r.URL.Query().Get("page_size"))
		require.Equal(t, "name", r.URL.Query().Get("sort_by"))
		require.Equal(t, "asc", r.URL.Query().Get("sort_order"))
		sawAuth = r.Header.Get("Authorization") == "Bearer secret-token"

		writeAdminJSON(t, w, http.StatusOK, map[string]any{
			"code":    0,
			"message": "ok",
			"data": map[string]any{
				"items": []map[string]any{
					{
						"id":              42,
						"name":            "upstream-a-1.2",
						"rate_multiplier": 1.2,
						"group_ids":       []int64{20, 10},
						"priority":        5,
						"schedulable":     true,
						"updated_at":      "2026-07-03T01:02:03Z",
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

	client := NewAdminClient(server.URL+"/api/v1", "secret-token", server.Client())
	accounts, err := client.ListAccounts(context.Background())

	require.NoError(t, err)
	require.True(t, sawAuth)
	require.Len(t, accounts, 1)
	require.Equal(t, AccountSnapshot{
		ID:             42,
		Name:           "upstream-a-1.2",
		RateMultiplier: 1.2,
		GroupIDs:       []int64{20, 10},
		Priority:       5,
		Schedulable:    true,
		UpdatedAt:      accounts[0].UpdatedAt,
	}, accounts[0])
	require.Equal(t, "2026-07-03T01:02:03Z", accounts[0].UpdatedAt.Format("2006-01-02T15:04:05Z"))
}

func TestAdminClientListAccountsReadsAllPages(t *testing.T) {
	var pages []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/admin/accounts", r.URL.Path)
		page := r.URL.Query().Get("page")
		pages = append(pages, page)
		var items []map[string]any
		switch page {
		case "1":
			items = []map[string]any{
				{
					"id":              1,
					"name":            "account-1",
					"rate_multiplier": 0.1,
					"group_ids":       []int64{10},
					"priority":        5,
					"schedulable":     true,
				},
			}
		case "2":
			items = []map[string]any{
				{
					"id":              2,
					"name":            "account-2",
					"rate_multiplier": 0.2,
					"group_ids":       []int64{20},
					"priority":        10,
					"schedulable":     true,
				},
			}
		default:
			t.Fatalf("unexpected page %s", page)
		}
		writeAdminJSON(t, w, http.StatusOK, map[string]any{
			"code":    0,
			"message": "ok",
			"data": map[string]any{
				"items":     items,
				"total":     2,
				"page":      mustAtoi(t, page),
				"page_size": 1000,
				"pages":     2,
			},
		})
	}))
	defer server.Close()

	client := NewAdminClient(server.URL, "secret-token", server.Client())
	accounts, err := client.ListAccounts(context.Background())

	require.NoError(t, err)
	require.Equal(t, []string{"1", "2"}, pages)
	require.Len(t, accounts, 2)
	require.Equal(t, int64(1), accounts[0].ID)
	require.Equal(t, int64(2), accounts[1].ID)
}

func TestAdminClientApplyPlanDetectsDrift(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/api/v1/admin/accounts/42", r.URL.Path)
		writeAdminJSON(t, w, http.StatusOK, map[string]any{
			"code":    0,
			"message": "ok",
			"data": map[string]any{
				"id":              42,
				"name":            "account-1.1",
				"rate_multiplier": 1.2,
				"group_ids":       []int64{10},
				"priority":        5,
				"schedulable":     true,
			},
		})
	}))
	defer server.Close()

	client := NewAdminClient(server.URL, "secret-token", server.Client())
	result, err := client.ApplyPlan(context.Background(), &Plan{
		Accounts: []PlanAccountChange{
			{
				AccountID: 42,
				Status:    PlanChangeStatusReady,
				Current: AccountSnapshot{
					ID:             42,
					Name:           "account-1.1",
					RateMultiplier: 1.1,
					GroupIDs:       []int64{10},
					Priority:       5,
					Schedulable:    true,
				},
				Target: &AccountTarget{
					Name:           "account-1.2",
					RateMultiplier: 1.2,
					GroupIDs:       []int64{10},
					Priority:       5,
					Schedulable:    true,
				},
			},
		},
	}, false)

	require.NoError(t, err)
	require.Len(t, result.Results, 1)
	require.Equal(t, "conflict", result.Results[0].Status)
	require.Equal(t, 1, result.Summary.Conflicts)
	require.Equal(t, 0, result.Summary.Success)
}

func TestAdminClientApplyPlanDetectsPlatformTypeStatusDrift(t *testing.T) {
	var calls []string
	liveByID := map[string]map[string]any{
		"42": {
			"id":              42,
			"name":            "account-1.1",
			"platform":        "anthropic",
			"type":            "apikey",
			"status":          "active",
			"rate_multiplier": 1.1,
			"group_ids":       []int64{10},
			"priority":        5,
			"schedulable":     true,
		},
		"43": {
			"id":              43,
			"name":            "account-1.1",
			"platform":        "openai",
			"type":            "oauth",
			"status":          "active",
			"rate_multiplier": 1.1,
			"group_ids":       []int64{10},
			"priority":        5,
			"schedulable":     true,
		},
		"44": {
			"id":              44,
			"name":            "account-1.1",
			"platform":        "openai",
			"type":            "apikey",
			"status":          "error",
			"rate_multiplier": 1.1,
			"group_ids":       []int64{10},
			"priority":        5,
			"schedulable":     true,
		},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		require.Equal(t, http.MethodGet, r.Method)
		id := pathBase(r.URL.Path)
		writeAdminJSON(t, w, http.StatusOK, map[string]any{
			"code":    0,
			"message": "ok",
			"data":    liveByID[id],
		})
	}))
	defer server.Close()

	var plan Plan
	require.NoError(t, json.Unmarshal([]byte(`{
	  "accounts": [
	    {
	      "account_id": 42,
	      "status": "ready",
	      "current": {"id":42,"name":"account-1.1","platform":"openai","type":"apikey","status":"active","rate_multiplier":1.1,"group_ids":[10],"priority":5,"schedulable":true},
	      "target": {"name":"account-1.2","rate_multiplier":1.2,"group_ids":[10],"priority":5,"schedulable":true}
	    },
	    {
	      "account_id": 43,
	      "status": "ready",
	      "current": {"id":43,"name":"account-1.1","platform":"openai","type":"apikey","status":"active","rate_multiplier":1.1,"group_ids":[10],"priority":5,"schedulable":true},
	      "target": {"name":"account-1.2","rate_multiplier":1.2,"group_ids":[10],"priority":5,"schedulable":true}
	    },
	    {
	      "account_id": 44,
	      "status": "ready",
	      "current": {"id":44,"name":"account-1.1","platform":"openai","type":"apikey","status":"active","rate_multiplier":1.1,"group_ids":[10],"priority":5,"schedulable":true},
	      "target": {"name":"account-1.2","rate_multiplier":1.2,"group_ids":[10],"priority":5,"schedulable":true}
	    }
	  ]
	}`), &plan))

	client := NewAdminClient(server.URL, "secret-token", server.Client())
	result, err := client.ApplyPlan(context.Background(), &plan, false)

	require.NoError(t, err)
	require.Equal(t, []string{
		"GET /api/v1/admin/accounts/42",
		"GET /api/v1/admin/accounts/43",
		"GET /api/v1/admin/accounts/44",
	}, calls)
	require.Equal(t, 3, result.Summary.Conflicts)
	require.Equal(t, 0, result.Summary.Success)
	for _, item := range result.Results {
		require.Equal(t, "conflict", item.Status)
	}
}

func mustAtoi(t *testing.T, value string) int {
	t.Helper()
	n, err := strconv.Atoi(value)
	require.NoError(t, err)
	return n
}

func TestAdminClientApplyPlanDisablesThenUpdatesAccount(t *testing.T) {
	var calls []string
	var updatePayload map[string]any
	var schedulablePayload map[string]any
	current := AccountSnapshot{
		ID:             42,
		Name:           "account-1.1",
		RateMultiplier: 1.1,
		GroupIDs:       []int64{10},
		Priority:       5,
		Schedulable:    true,
	}
	final := AccountSnapshot{
		ID:             42,
		Name:           "account-1.2",
		RateMultiplier: 1.2,
		GroupIDs:       []int64{30, 10},
		Priority:       15,
		Schedulable:    false,
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer secret-token", r.Header.Get("Authorization"))
		calls = append(calls, r.Method+" "+r.URL.Path)

		switch len(calls) {
		case 1:
			require.Equal(t, "GET /api/v1/admin/accounts/42", calls[0])
			writeEnvelopeData(t, w, current)
		case 2:
			require.Equal(t, "POST /api/v1/admin/accounts/42/schedulable", calls[1])
			require.NoError(t, json.NewDecoder(r.Body).Decode(&schedulablePayload))
			writeEnvelopeData(t, w, final)
		case 3:
			require.Equal(t, "PUT /api/v1/admin/accounts/42", calls[2])
			require.NoError(t, json.NewDecoder(r.Body).Decode(&updatePayload))
			writeEnvelopeData(t, w, final)
		case 4:
			require.Equal(t, "GET /api/v1/admin/accounts/42", calls[3])
			writeEnvelopeData(t, w, final)
		default:
			t.Fatalf("unexpected call %d: %s", len(calls), calls[len(calls)-1])
		}
	}))
	defer server.Close()

	client := NewAdminClient(server.URL, "secret-token", server.Client())
	result, err := client.ApplyPlan(context.Background(), &Plan{
		Accounts: []PlanAccountChange{
			{
				AccountID: 42,
				Status:    PlanChangeStatusReady,
				Current:   current,
				Target: &AccountTarget{
					Name:           final.Name,
					RateMultiplier: final.RateMultiplier,
					GroupIDs:       final.GroupIDs,
					Priority:       final.Priority,
					Schedulable:    final.Schedulable,
				},
			},
		},
	}, false)

	require.NoError(t, err)
	require.Equal(t, []string{
		"GET /api/v1/admin/accounts/42",
		"POST /api/v1/admin/accounts/42/schedulable",
		"PUT /api/v1/admin/accounts/42",
		"GET /api/v1/admin/accounts/42",
	}, calls)
	require.Equal(t, map[string]any{
		"name":                       "account-1.2",
		"rate_multiplier":            1.2,
		"group_ids":                  []any{float64(30), float64(10)},
		"priority":                   float64(15),
		"confirm_mixed_channel_risk": true,
	}, updatePayload)
	require.Equal(t, map[string]any{"schedulable": false}, schedulablePayload)
	require.Equal(t, 1, result.Summary.Success)
	require.Equal(t, "applied", result.Results[0].Status)
}

func TestAdminClientApplyPlanDoesNotUpdateFieldsWhenDisableFails(t *testing.T) {
	var calls []string
	current := AccountSnapshot{
		ID:             42,
		Name:           "account-1.1",
		RateMultiplier: 1.1,
		GroupIDs:       []int64{10},
		Priority:       5,
		Schedulable:    true,
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		switch len(calls) {
		case 1:
			require.Equal(t, "GET /api/v1/admin/accounts/42", calls[0])
			writeEnvelopeData(t, w, current)
		case 2:
			require.Equal(t, "POST /api/v1/admin/accounts/42/schedulable", calls[1])
			writeAdminJSON(t, w, http.StatusInternalServerError, map[string]any{
				"code":    1,
				"message": "schedulable update failed",
			})
		default:
			t.Fatalf("unexpected call after failed disable: %s", calls[len(calls)-1])
		}
	}))
	defer server.Close()

	client := NewAdminClient(server.URL, "secret-token", server.Client())
	result, err := client.ApplyPlan(context.Background(), &Plan{
		Accounts: []PlanAccountChange{
			{
				AccountID: 42,
				Status:    PlanChangeStatusReady,
				Current:   current,
				Target: &AccountTarget{
					Name:           "account-1.2",
					RateMultiplier: 1.2,
					GroupIDs:       []int64{},
					Priority:       15,
					Schedulable:    false,
				},
			},
		},
	}, false)

	require.NoError(t, err)
	require.Equal(t, []string{
		"GET /api/v1/admin/accounts/42",
		"POST /api/v1/admin/accounts/42/schedulable",
	}, calls)
	require.Equal(t, 1, result.Summary.Failed)
	require.Equal(t, "failed", result.Results[0].Status)
}

func TestAdminClientDryRunDoesNotMutate(t *testing.T) {
	var calls []string
	current := AccountSnapshot{
		ID:             42,
		Name:           "account-1.1",
		RateMultiplier: 1.1,
		GroupIDs:       []int64{10},
		Priority:       5,
		Schedulable:    true,
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		require.Equal(t, http.MethodGet, r.Method)
		writeEnvelopeData(t, w, current)
	}))
	defer server.Close()

	client := NewAdminClient(server.URL, "secret-token", server.Client())
	result, err := client.ApplyPlan(context.Background(), &Plan{
		Accounts: []PlanAccountChange{
			{
				AccountID: 42,
				Status:    PlanChangeStatusReady,
				Current:   current,
				Target: &AccountTarget{
					Name:           "account-1.2",
					RateMultiplier: 1.2,
					GroupIDs:       []int64{10},
					Priority:       5,
					Schedulable:    true,
				},
			},
			{
				AccountID: 43,
				Status:    PlanChangeStatusNoop,
			},
		},
	}, true)

	require.NoError(t, err)
	require.Equal(t, []string{"GET /api/v1/admin/accounts/42"}, calls)
	require.True(t, result.DryRun)
	require.Equal(t, 1, result.Summary.Success)
	require.Equal(t, 1, result.Summary.Skipped)
	require.Equal(t, "dry_run", result.Results[0].Status)
	require.Equal(t, "skipped", result.Results[1].Status)
}

func pathBase(value string) string {
	parts := strings.Split(strings.Trim(value, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

func writeEnvelopeData(t *testing.T, w http.ResponseWriter, data any) {
	t.Helper()
	writeAdminJSON(t, w, http.StatusOK, map[string]any{
		"code":    0,
		"message": "ok",
		"data":    data,
	})
}

func writeAdminJSON(t *testing.T, w http.ResponseWriter, status int, body any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	require.NoError(t, json.NewEncoder(w).Encode(body))
}
