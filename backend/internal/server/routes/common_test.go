package routes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type publicConcurrencyProviderStub struct {
	status         *service.PublicConcurrencyStatus
	err            error
	homepageStatus *service.PublicHomepageStatus
	homepageErr    error
}

func (s *publicConcurrencyProviderStub) GetPublicConcurrency(context.Context) (*service.PublicConcurrencyStatus, error) {
	return s.status, s.err
}

func (s *publicConcurrencyProviderStub) GetPublicHomepageStatus(context.Context) (*service.PublicHomepageStatus, error) {
	return s.homepageStatus, s.homepageErr
}

func TestPublicConcurrencyReturnsDirectStatusObject(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterCommonRoutes(router, &publicConcurrencyProviderStub{
		status: &service.PublicConcurrencyStatus{
			Current:   9,
			UpdatedAt: time.Date(2026, time.July, 14, 8, 9, 10, 0, time.UTC),
		},
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/status/concurrency", nil)
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var body map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Len(t, body, 2)
	require.Contains(t, body, "current")
	require.Contains(t, body, "updated_at")
	for _, forbidden := range []string{"account", "user", "platform", "capacity", "waiting"} {
		require.NotContains(t, body, forbidden)
	}

	var current int
	require.NoError(t, json.Unmarshal(body["current"], &current))
	require.GreaterOrEqual(t, current, 0)
	require.Equal(t, 9, current)
	var updatedAt string
	require.NoError(t, json.Unmarshal(body["updated_at"], &updatedAt))
	_, err := time.Parse(time.RFC3339, updatedAt)
	require.NoError(t, err)
}

func TestPublicConcurrencyMapsApplicationError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterCommonRoutes(router, &publicConcurrencyProviderStub{
		err: infraerrors.ServiceUnavailable(
			"CONCURRENCY_STATUS_UNAVAILABLE",
			"realtime concurrency unavailable",
		),
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/status/concurrency", nil)
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	assertPublicConcurrencyUnavailableReason(t, recorder)
}

func TestPublicConcurrencyMapsNilProvider(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterCommonRoutes(router, nil)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/status/concurrency", nil)
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	assertPublicConcurrencyUnavailableReason(t, recorder)
}

func assertPublicConcurrencyUnavailableReason(t *testing.T, recorder *httptest.ResponseRecorder) {
	t.Helper()
	var body struct {
		Reason string `json:"reason"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, "CONCURRENCY_STATUS_UNAVAILABLE", body.Reason)
}

func TestPublicHomepageStatusReturnsExactPublicContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	startedAt := time.Date(2026, time.May, 14, 19, 50, 12, 0, time.UTC)
	successRate := 0.9987
	updatedAt := time.Date(2026, time.July, 15, 8, 9, 10, 0, time.UTC)
	router := gin.New()
	RegisterCommonRoutes(router, &publicConcurrencyProviderStub{homepageStatus: &service.PublicHomepageStatus{
		StartedAt:        &startedAt,
		ActiveUsers1h:    10,
		SuccessRateToday: &successRate,
		TotalTokens:      18043746148,
		UpdatedAt:        updatedAt,
	}})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/status/homepage", nil)
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var body map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Len(t, body, 5)
	for _, field := range []string{"started_at", "active_users_1h", "success_rate_today", "total_tokens", "updated_at"} {
		require.Contains(t, body, field)
	}
	for _, forbidden := range []string{"account", "user_id", "email", "group", "platform", "concurrency"} {
		require.NotContains(t, body, forbidden)
	}
}

func TestPublicHomepageStatusMapsUnavailableError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterCommonRoutes(router, &publicConcurrencyProviderStub{homepageErr: infraerrors.ServiceUnavailable(
		"HOMEPAGE_STATUS_UNAVAILABLE",
		"homepage status unavailable",
	)})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/status/homepage", nil)
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	var body struct {
		Reason string `json:"reason"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, "HOMEPAGE_STATUS_UNAVAILABLE", body.Reason)
}

func TestPublicHomepageStatusSerializesEmptySuccessRateAsNull(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterCommonRoutes(router, &publicConcurrencyProviderStub{homepageStatus: &service.PublicHomepageStatus{
		UpdatedAt: time.Date(2026, time.July, 15, 8, 9, 10, 0, time.UTC),
	}})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/status/homepage", nil)
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var body map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Len(t, body, 5)
	require.JSONEq(t, "null", string(body["success_rate_today"]))
	require.JSONEq(t, "null", string(body["started_at"]))
}
