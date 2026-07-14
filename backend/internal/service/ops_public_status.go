package service

import (
	"context"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	defaultPublicConcurrencyCacheTTL = 2 * time.Second
	publicConcurrencyRefreshTimeout  = 3 * time.Second
	publicConcurrencyFlightKey       = "public-concurrency"
	publicConcurrencyUnavailable     = "CONCURRENCY_STATUS_UNAVAILABLE"
)

type PublicConcurrencyStatus struct {
	Current   int       `json:"current"`
	UpdatedAt time.Time `json:"updated_at"`
}

type cachedPublicConcurrency struct {
	status    PublicConcurrencyStatus
	expiresAt time.Time
}

func (s *OpsService) GetPublicConcurrency(ctx context.Context) (*PublicConcurrencyStatus, error) {
	if s == nil || s.concurrencyService == nil {
		return nil, newPublicConcurrencyUnavailableError()
	}
	if ctx == nil {
		ctx = context.Background()
	}

	now := time.Now().UTC()
	s.publicConcurrencyMu.Lock()
	if now.Before(s.publicConcurrencyCache.expiresAt) {
		cached := s.publicConcurrencyCache.status
		s.publicConcurrencyMu.Unlock()
		return &cached, nil
	}
	s.publicConcurrencyMu.Unlock()

	resultCh := s.publicConcurrencyGroup.DoChan(publicConcurrencyFlightKey, func() (any, error) {
		now := time.Now().UTC()
		s.publicConcurrencyMu.Lock()
		if now.Before(s.publicConcurrencyCache.expiresAt) {
			cached := s.publicConcurrencyCache.status
			s.publicConcurrencyMu.Unlock()
			return cached, nil
		}
		s.publicConcurrencyMu.Unlock()

		refreshCtx, cancel := context.WithTimeout(context.Background(), publicConcurrencyRefreshTimeout)
		defer cancel()
		current, err := s.concurrencyService.GetTotalAccountConcurrency(refreshCtx)
		if err != nil {
			return nil, newPublicConcurrencyUnavailableError().WithCause(err)
		}

		updatedAt := time.Now().UTC()
		ttl := s.publicConcurrencyCacheTTL
		if ttl <= 0 {
			ttl = defaultPublicConcurrencyCacheTTL
		}
		status := PublicConcurrencyStatus{Current: current, UpdatedAt: updatedAt}

		s.publicConcurrencyMu.Lock()
		s.publicConcurrencyCache = cachedPublicConcurrency{
			status:    status,
			expiresAt: updatedAt.Add(ttl),
		}
		s.publicConcurrencyMu.Unlock()

		return status, nil
	})

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-resultCh:
		if result.Err != nil {
			return nil, result.Err
		}
		status, ok := result.Val.(PublicConcurrencyStatus)
		if !ok {
			return nil, newPublicConcurrencyUnavailableError()
		}
		return &status, nil
	}
}

func newPublicConcurrencyUnavailableError() *infraerrors.ApplicationError {
	return infraerrors.ServiceUnavailable(publicConcurrencyUnavailable, "realtime concurrency unavailable")
}
