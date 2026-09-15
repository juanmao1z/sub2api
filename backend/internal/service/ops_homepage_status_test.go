//go:build unit

package service

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type publicHomepageStatusRepositoryStub struct {
	snapshot  PublicHomepageStatusSnapshot
	err       error
	returnNil bool
	calls     atomic.Int64
	started   chan struct{}
	release   chan struct{}
	once      sync.Once
}

func (r *publicHomepageStatusRepositoryStub) GetPublicHomepageStatus(ctx context.Context, _ time.Time) (*PublicHomepageStatusSnapshot, error) {
	r.calls.Add(1)
	if r.started != nil {
		r.once.Do(func() { close(r.started) })
	}
	if r.release != nil {
		select {
		case <-r.release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if r.err != nil {
		return nil, r.err
	}
	if r.returnNil {
		return nil, nil
	}
	snapshot := r.snapshot
	return &snapshot, nil
}

func TestOpsService_GetPublicHomepageStatusCachesSuccess(t *testing.T) {
	startedAt := time.Date(2026, time.May, 14, 19, 50, 12, 0, time.UTC)
	repo := &publicHomepageStatusRepositoryStub{snapshot: PublicHomepageStatusSnapshot{
		StartedAt:          &startedAt,
		ActiveUsers1h:      10,
		SuccessCountToday:  999,
		ErrorCountSLAToday: 1,
		TotalTokens:        18043746148,
	}}
	svc := &OpsService{publicHomepageRepo: repo, publicHomepageCacheTTL: time.Minute}

	first, err := svc.GetPublicHomepageStatus(context.Background())
	require.NoError(t, err)
	second, err := svc.GetPublicHomepageStatus(context.Background())
	require.NoError(t, err)

	require.Equal(t, int64(10), first.ActiveUsers1h)
	require.NotNil(t, first.SuccessRateToday)
	require.InDelta(t, 0.999, *first.SuccessRateToday, 0.0000001)
	require.Equal(t, int64(18043746148), first.TotalTokens)
	require.Equal(t, first.UpdatedAt, second.UpdatedAt)
	require.Equal(t, int64(1), repo.calls.Load())
}

func TestOpsService_GetPublicHomepageStatusDoesNotCacheErrors(t *testing.T) {
	repo := &publicHomepageStatusRepositoryStub{err: errors.New("database unavailable")}
	svc := &OpsService{publicHomepageRepo: repo, publicHomepageCacheTTL: time.Minute}

	_, err := svc.GetPublicHomepageStatus(context.Background())
	require.Error(t, err)
	require.Equal(t, "HOMEPAGE_STATUS_UNAVAILABLE", infraerrors.Reason(err))

	repo.err = nil
	repo.snapshot = PublicHomepageStatusSnapshot{ActiveUsers1h: 3, TotalTokens: 7}
	status, err := svc.GetPublicHomepageStatus(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(3), status.ActiveUsers1h)
	require.Equal(t, int64(2), repo.calls.Load())
}

func TestOpsService_GetPublicHomepageStatusRefreshesAfterExpiry(t *testing.T) {
	repo := &publicHomepageStatusRepositoryStub{snapshot: PublicHomepageStatusSnapshot{ActiveUsers1h: 2}}
	svc := &OpsService{publicHomepageRepo: repo, publicHomepageCacheTTL: time.Minute}

	first, err := svc.GetPublicHomepageStatus(context.Background())
	require.NoError(t, err)
	svc.publicHomepageMu.Lock()
	svc.publicHomepageCache.expiresAt = time.Now().Add(-time.Second)
	svc.publicHomepageMu.Unlock()
	repo.snapshot.ActiveUsers1h = 5

	second, err := svc.GetPublicHomepageStatus(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(2), first.ActiveUsers1h)
	require.Equal(t, int64(5), second.ActiveUsers1h)
	require.Equal(t, int64(2), repo.calls.Load())
}

func TestOpsService_GetPublicHomepageStatusHandlesEmptyData(t *testing.T) {
	repo := &publicHomepageStatusRepositoryStub{}
	svc := &OpsService{publicHomepageRepo: repo}

	status, err := svc.GetPublicHomepageStatus(context.Background())
	require.NoError(t, err)
	require.Nil(t, status.StartedAt)
	require.Nil(t, status.SuccessRateToday)
	require.Zero(t, status.ActiveUsers1h)
	require.Zero(t, status.TotalTokens)
}

func TestOpsService_GetPublicHomepageStatusReturnsRawSuccessRatio(t *testing.T) {
	repo := &publicHomepageStatusRepositoryStub{snapshot: PublicHomepageStatusSnapshot{
		SuccessCountToday:  1,
		ErrorCountSLAToday: 2,
	}}
	svc := &OpsService{publicHomepageRepo: repo}

	status, err := svc.GetPublicHomepageStatus(context.Background())
	require.NoError(t, err)
	require.NotNil(t, status.SuccessRateToday)
	require.Equal(t, float64(1)/float64(3), *status.SuccessRateToday)
	require.NotEqual(t, 0.3333, *status.SuccessRateToday)
}

func TestOpsService_GetPublicHomepageStatusReturnsZeroRateWhenOnlySLAErrorsExist(t *testing.T) {
	repo := &publicHomepageStatusRepositoryStub{snapshot: PublicHomepageStatusSnapshot{ErrorCountSLAToday: 4}}
	svc := &OpsService{publicHomepageRepo: repo}

	status, err := svc.GetPublicHomepageStatus(context.Background())
	require.NoError(t, err)
	require.NotNil(t, status.SuccessRateToday)
	require.Zero(t, *status.SuccessRateToday)
}

func TestOpsService_GetPublicHomepageStatusRejectsNilSnapshot(t *testing.T) {
	svc := &OpsService{publicHomepageRepo: &publicHomepageStatusRepositoryStub{returnNil: true}}

	_, err := svc.GetPublicHomepageStatus(context.Background())
	require.Error(t, err)
	require.Equal(t, "HOMEPAGE_STATUS_UNAVAILABLE", infraerrors.Reason(err))
}

func TestOpsService_GetPublicHomepageStatusCanceledCallerDoesNotCancelRefresh(t *testing.T) {
	repo := &publicHomepageStatusRepositoryStub{
		snapshot: PublicHomepageStatusSnapshot{ActiveUsers1h: 8},
		started:  make(chan struct{}),
		release:  make(chan struct{}),
	}
	svc := &OpsService{publicHomepageRepo: repo, publicHomepageCacheTTL: time.Minute}

	leaderCtx, cancelLeader := context.WithCancel(context.Background())
	leaderErr := make(chan error, 1)
	go func() {
		_, err := svc.GetPublicHomepageStatus(leaderCtx)
		leaderErr <- err
	}()
	<-repo.started
	cancelLeader()
	require.ErrorIs(t, <-leaderErr, context.Canceled)

	type result struct {
		status *PublicHomepageStatus
		err    error
	}
	followerResult := make(chan result, 1)
	go func() {
		status, err := svc.GetPublicHomepageStatus(context.Background())
		followerResult <- result{status: status, err: err}
	}()
	close(repo.release)

	got := <-followerResult
	require.NoError(t, got.err)
	require.Equal(t, int64(8), got.status.ActiveUsers1h)
	require.Equal(t, int64(1), repo.calls.Load())
}

func TestOpsService_GetPublicHomepageStatusCoalescesConcurrentColdCache(t *testing.T) {
	repo := &publicHomepageStatusRepositoryStub{
		snapshot: PublicHomepageStatusSnapshot{ActiveUsers1h: 12},
		started:  make(chan struct{}),
		release:  make(chan struct{}),
	}
	svc := &OpsService{publicHomepageRepo: repo, publicHomepageCacheTTL: time.Minute}

	const callers = 12
	start := make(chan struct{})
	results := make(chan *PublicHomepageStatus, callers)
	errs := make(chan error, callers)
	var ready sync.WaitGroup
	ready.Add(callers)
	for range callers {
		go func() {
			ready.Done()
			<-start
			status, err := svc.GetPublicHomepageStatus(context.Background())
			results <- status
			errs <- err
		}()
	}
	ready.Wait()
	close(start)
	<-repo.started
	close(repo.release)

	for range callers {
		require.NoError(t, <-errs)
		status := <-results
		require.NotNil(t, status)
		require.Equal(t, int64(12), status.ActiveUsers1h)
	}
	require.Equal(t, int64(1), repo.calls.Load())
}
