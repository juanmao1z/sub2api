//go:build unit

package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestOpsService_GetPublicConcurrencyCachesSuccess(t *testing.T) {
	cache := &stubConcurrencyCacheForTest{totalAccountConcurrency: 4}
	svc := &OpsService{
		concurrencyService:        NewConcurrencyService(cache),
		publicConcurrencyCacheTTL: 2 * time.Second,
	}

	first, err := svc.GetPublicConcurrency(context.Background())
	require.NoError(t, err)
	second, err := svc.GetPublicConcurrency(context.Background())
	require.NoError(t, err)
	require.Equal(t, 4, first.Current)
	require.Equal(t, first.UpdatedAt, second.UpdatedAt)
	require.Equal(t, int64(1), cache.totalAccountConcurrencyCalls.Load())
}

func TestOpsService_GetPublicConcurrencyDoesNotCacheErrors(t *testing.T) {
	cache := &stubConcurrencyCacheForTest{
		totalAccountConcurrencyErr: errors.New("redis unavailable"),
	}
	svc := &OpsService{
		concurrencyService:        NewConcurrencyService(cache),
		publicConcurrencyCacheTTL: 2 * time.Second,
	}

	_, firstErr := svc.GetPublicConcurrency(context.Background())
	require.Error(t, firstErr)
	require.Equal(t, "CONCURRENCY_STATUS_UNAVAILABLE", infraerrors.Reason(firstErr))

	cache.totalAccountConcurrencyErr = nil
	cache.totalAccountConcurrency = 6
	second, secondErr := svc.GetPublicConcurrency(context.Background())
	require.NoError(t, secondErr)
	require.Equal(t, 6, second.Current)
	require.Equal(t, int64(2), cache.totalAccountConcurrencyCalls.Load())
}

func TestOpsService_GetPublicConcurrencyRefreshesAfterExpiry(t *testing.T) {
	cache := &stubConcurrencyCacheForTest{totalAccountConcurrency: 2}
	svc := &OpsService{
		concurrencyService:        NewConcurrencyService(cache),
		publicConcurrencyCacheTTL: 2 * time.Second,
	}

	first, err := svc.GetPublicConcurrency(context.Background())
	require.NoError(t, err)

	svc.publicConcurrencyMu.Lock()
	svc.publicConcurrencyCache.expiresAt = time.Now().Add(-time.Second)
	svc.publicConcurrencyMu.Unlock()
	cache.totalAccountConcurrency = 5

	second, err := svc.GetPublicConcurrency(context.Background())
	require.NoError(t, err)
	require.Equal(t, 2, first.Current)
	require.Equal(t, 5, second.Current)
	require.Equal(t, int64(2), cache.totalAccountConcurrencyCalls.Load())
}

func TestOpsService_GetPublicConcurrencyMapsUnavailableErrors(t *testing.T) {
	tests := []struct {
		name string
		svc  *OpsService
	}{
		{name: "nil service", svc: nil},
		{name: "nil concurrency service", svc: &OpsService{}},
		{
			name: "aggregation failure",
			svc: &OpsService{
				concurrencyService: NewConcurrencyService(&stubConcurrencyCacheForTest{
					totalAccountConcurrencyErr: errors.New("redis unavailable"),
				}),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.svc.GetPublicConcurrency(context.Background())
			require.Error(t, err)
			require.Equal(t, 503, infraerrors.Code(err))
			require.Equal(t, "CONCURRENCY_STATUS_UNAVAILABLE", infraerrors.Reason(err))
		})
	}
}

type blockingTotalConcurrencyCacheForTest struct {
	stubConcurrencyCacheForTest
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

type observedDoneContextForTest struct {
	context.Context
	observed chan struct{}
	once     sync.Once
}

func (c *observedDoneContextForTest) Done() <-chan struct{} {
	c.once.Do(func() { close(c.observed) })
	return c.Context.Done()
}

func (c *blockingTotalConcurrencyCacheForTest) GetTotalAccountConcurrency(ctx context.Context) (int, error) {
	c.totalAccountConcurrencyCalls.Add(1)
	c.once.Do(func() { close(c.started) })
	select {
	case <-c.release:
		return c.totalAccountConcurrency, c.totalAccountConcurrencyErr
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

func TestOpsService_GetPublicConcurrencyCoalescesConcurrentColdCache(t *testing.T) {
	cache := &blockingTotalConcurrencyCacheForTest{
		stubConcurrencyCacheForTest: stubConcurrencyCacheForTest{totalAccountConcurrency: 9},
		started:                     make(chan struct{}),
		release:                     make(chan struct{}),
	}
	svc := &OpsService{
		concurrencyService:        NewConcurrencyService(cache),
		publicConcurrencyCacheTTL: 2 * time.Second,
	}

	const callers = 16
	start := make(chan struct{})
	results := make(chan *PublicConcurrencyStatus, callers)
	errs := make(chan error, callers)
	var ready sync.WaitGroup
	ready.Add(callers)
	for range callers {
		go func() {
			ready.Done()
			<-start
			status, err := svc.GetPublicConcurrency(context.Background())
			results <- status
			errs <- err
		}()
	}
	ready.Wait()
	close(start)
	<-cache.started
	aggregationCompletedAfter := time.Now().UTC()
	close(cache.release)

	for range callers {
		require.NoError(t, <-errs)
		status := <-results
		require.NotNil(t, status)
		require.Equal(t, 9, status.Current)
		require.False(t, status.UpdatedAt.Before(aggregationCompletedAfter))
	}
	require.Equal(t, int64(1), cache.totalAccountConcurrencyCalls.Load())
}

func TestOpsService_GetPublicConcurrencyCanceledLeaderDoesNotCancelSharedRefresh(t *testing.T) {
	cache := &blockingTotalConcurrencyCacheForTest{
		stubConcurrencyCacheForTest: stubConcurrencyCacheForTest{totalAccountConcurrency: 11},
		started:                     make(chan struct{}),
		release:                     make(chan struct{}),
	}
	svc := &OpsService{
		concurrencyService:        NewConcurrencyService(cache),
		publicConcurrencyCacheTTL: 2 * time.Second,
	}

	leaderCtx, cancelLeader := context.WithCancel(context.Background())
	leaderErr := make(chan error, 1)
	go func() {
		_, err := svc.GetPublicConcurrency(leaderCtx)
		leaderErr <- err
	}()

	<-cache.started
	cancelLeader()
	select {
	case err := <-leaderErr:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("canceled leader did not return promptly")
	}

	type result struct {
		status *PublicConcurrencyStatus
		err    error
	}
	followerBaseCtx, cancelFollower := context.WithCancel(context.Background())
	defer cancelFollower()
	followerCtx := &observedDoneContextForTest{
		Context:  followerBaseCtx,
		observed: make(chan struct{}),
	}
	followerResult := make(chan result, 1)
	go func() {
		status, err := svc.GetPublicConcurrency(followerCtx)
		followerResult <- result{status: status, err: err}
	}()

	select {
	case <-followerCtx.observed:
	case <-time.After(time.Second):
		t.Fatal("follower did not join the shared refresh")
	}
	close(cache.release)
	select {
	case got := <-followerResult:
		require.NoError(t, got.err)
		require.NotNil(t, got.status)
		require.Equal(t, 11, got.status.Current)
	case <-time.After(time.Second):
		t.Fatal("follower did not receive the shared refresh result")
	}
	require.Equal(t, int64(1), cache.totalAccountConcurrencyCalls.Load())
}
