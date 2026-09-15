package service

import (
	"context"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	defaultPublicHomepageCacheTTL = time.Minute
	publicHomepageRefreshTimeout  = 5 * time.Second
	publicHomepageFlightKey       = "public-homepage"
	publicHomepageUnavailable     = "HOMEPAGE_STATUS_UNAVAILABLE"
)

// PublicHomepageStatusSnapshot is the repository-level aggregate used to
// calculate the anonymous homepage metrics.
type PublicHomepageStatusSnapshot struct {
	StartedAt          *time.Time
	ActiveUsers1h      int64
	SuccessCountToday  int64
	ErrorCountSLAToday int64
	TotalTokens        int64
}

// PublicHomepageStatusRepository is intentionally narrower than OpsRepository
// because the public homepage only needs one aggregate query contract.
type PublicHomepageStatusRepository interface {
	GetPublicHomepageStatus(ctx context.Context, now time.Time) (*PublicHomepageStatusSnapshot, error)
}

type PublicHomepageStatus struct {
	StartedAt        *time.Time `json:"started_at"`
	ActiveUsers1h    int64      `json:"active_users_1h"`
	SuccessRateToday *float64   `json:"success_rate_today"`
	TotalTokens      int64      `json:"total_tokens"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type cachedPublicHomepageStatus struct {
	status    PublicHomepageStatus
	expiresAt time.Time
}

func (s *OpsService) GetPublicHomepageStatus(ctx context.Context) (*PublicHomepageStatus, error) {
	if s == nil || s.publicHomepageRepo == nil {
		return nil, newPublicHomepageUnavailableError()
	}
	if ctx == nil {
		ctx = context.Background()
	}

	now := time.Now().UTC()
	s.publicHomepageMu.Lock()
	if now.Before(s.publicHomepageCache.expiresAt) {
		cached := clonePublicHomepageStatus(s.publicHomepageCache.status)
		s.publicHomepageMu.Unlock()
		return &cached, nil
	}
	s.publicHomepageMu.Unlock()

	resultCh := s.publicHomepageGroup.DoChan(publicHomepageFlightKey, func() (any, error) {
		now := time.Now().UTC()
		s.publicHomepageMu.Lock()
		if now.Before(s.publicHomepageCache.expiresAt) {
			cached := clonePublicHomepageStatus(s.publicHomepageCache.status)
			s.publicHomepageMu.Unlock()
			return cached, nil
		}
		s.publicHomepageMu.Unlock()

		refreshCtx, cancel := context.WithTimeout(context.Background(), publicHomepageRefreshTimeout)
		defer cancel()
		snapshot, err := s.publicHomepageRepo.GetPublicHomepageStatus(refreshCtx, now)
		if err != nil {
			return nil, newPublicHomepageUnavailableError().WithCause(err)
		}
		if snapshot == nil {
			return nil, newPublicHomepageUnavailableError()
		}

		status := publicHomepageStatusFromSnapshot(snapshot, time.Now().UTC())
		ttl := s.publicHomepageCacheTTL
		if ttl <= 0 {
			ttl = defaultPublicHomepageCacheTTL
		}

		s.publicHomepageMu.Lock()
		s.publicHomepageCache = cachedPublicHomepageStatus{
			status:    clonePublicHomepageStatus(status),
			expiresAt: status.UpdatedAt.Add(ttl),
		}
		s.publicHomepageMu.Unlock()

		return status, nil
	})

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-resultCh:
		if result.Err != nil {
			return nil, result.Err
		}
		status, ok := result.Val.(PublicHomepageStatus)
		if !ok {
			return nil, newPublicHomepageUnavailableError()
		}
		return &status, nil
	}
}

func publicHomepageStatusFromSnapshot(snapshot *PublicHomepageStatusSnapshot, updatedAt time.Time) PublicHomepageStatus {
	status := PublicHomepageStatus{UpdatedAt: updatedAt.UTC()}
	if snapshot == nil {
		return status
	}
	if snapshot.StartedAt != nil && !snapshot.StartedAt.IsZero() {
		startedAt := snapshot.StartedAt.UTC()
		status.StartedAt = &startedAt
	}
	status.ActiveUsers1h = nonNegativeInt64(snapshot.ActiveUsers1h)
	status.TotalTokens = nonNegativeInt64(snapshot.TotalTokens)

	success := nonNegativeInt64(snapshot.SuccessCountToday)
	errors := nonNegativeInt64(snapshot.ErrorCountSLAToday)
	denominator := float64(success) + float64(errors)
	if denominator > 0 {
		rate := float64(success) / denominator
		status.SuccessRateToday = &rate
	}
	return status
}

func nonNegativeInt64(value int64) int64 {
	if value < 0 {
		return 0
	}
	return value
}

func clonePublicHomepageStatus(status PublicHomepageStatus) PublicHomepageStatus {
	cloned := status
	if status.StartedAt != nil {
		startedAt := *status.StartedAt
		cloned.StartedAt = &startedAt
	}
	if status.SuccessRateToday != nil {
		rate := *status.SuccessRateToday
		cloned.SuccessRateToday = &rate
	}
	return cloned
}

func newPublicHomepageUnavailableError() *infraerrors.ApplicationError {
	return infraerrors.ServiceUnavailable(publicHomepageUnavailable, "homepage status unavailable")
}
