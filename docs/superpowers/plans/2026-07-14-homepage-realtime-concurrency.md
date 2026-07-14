# Homepage Realtime API Concurrency Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove the supported-models section and footer from the public homepage, then show the current number of API requests occupying account concurrency slots.

**Architecture:** The Redis concurrency repository will aggregate only account IDs present in the active index and expose that total through `ConcurrencyService`. `OpsService` will add a two-second success cache and the common routes module will publish only `{current, updated_at}`. The Vue frontend will poll every five seconds through a focused composable and render a single unframed status metric below the feature grid.

**Tech Stack:** Go 1.24, Gin, go-redis, Testify, Vue 3, TypeScript, Axios, Vitest, Vue Test Utils, Tailwind CSS, pnpm.

## Global Constraints

- The metric is the number of current API requests occupying account concurrency slots, not online users, QPS, waiting requests, or configured capacity.
- Remove the supported AI models area and all content below it, including the current footer.
- Keep the header, hero, capability tags, and three feature cards.
- The public API returns only a non-negative total and server timestamp; it must not expose platform, group, account, user, API key, capacity, waiting, or Redis details.
- Cache successful backend aggregation for exactly 2 seconds; do not cache errors or serve stale data after expiry.
- Poll every 5 seconds, pause while the page is hidden, refresh immediately when visible, and clean up timers/listeners on unmount.
- When the status cannot be loaded, render `--` without a global error notification.
- Custom `home_content` mode must not start the default homepage poller.
- Do not rebuild PostgreSQL, Redis, or leaderboard services during deployment.
- All local Windows commands run in `pwsh`, begin with strict error settings, and check every native CLI exit code.
- Unless a step says otherwise, run Task 1 and Task 2 Go commands from `backend`, and run frontend and Git commands from the repository root.

---

### Task 1: Redis Total Account Concurrency

**Files:**
- Modify: `backend/internal/service/concurrency_service.go`
- Modify: `backend/internal/service/concurrency_service_test.go`
- Modify: `backend/internal/repository/concurrency_cache.go`
- Modify: `backend/internal/repository/concurrency_cache_integration_test.go`

**Interfaces:**
- Consumes: Redis sorted sets `concurrency:account:active_index` and `concurrency:account:{accountID}`.
- Produces: `TotalAccountConcurrencyCache.GetTotalAccountConcurrency(context.Context) (int, error)` and `ConcurrencyService.GetTotalAccountConcurrency(context.Context) (int, error)`.

- [ ] **Step 1: Write the failing service tests**

Extend `stubConcurrencyCacheForTest` with `totalAccountConcurrency int`, `totalAccountConcurrencyErr error`, and `totalAccountConcurrencyCalls atomic.Int64`. Add the optional interface method and tests:

```go
func (c *stubConcurrencyCacheForTest) GetTotalAccountConcurrency(_ context.Context) (int, error) {
	c.totalAccountConcurrencyCalls.Add(1)
	return c.totalAccountConcurrency, c.totalAccountConcurrencyErr
}

func TestConcurrencyService_GetTotalAccountConcurrency(t *testing.T) {
	cache := &stubConcurrencyCacheForTest{totalAccountConcurrency: 7}
	svc := NewConcurrencyService(cache)

	got, err := svc.GetTotalAccountConcurrency(context.Background())
	require.NoError(t, err)
	require.Equal(t, 7, got)
	require.Equal(t, int64(1), cache.totalAccountConcurrencyCalls.Load())
}

func TestConcurrencyService_GetTotalAccountConcurrencyRejectsUnavailableCache(t *testing.T) {
	_, err := NewConcurrencyService(nil).GetTotalAccountConcurrency(context.Background())
	require.Error(t, err)
}

func TestConcurrencyService_GetTotalAccountConcurrencyRejectsNegativeValue(t *testing.T) {
	cache := &stubConcurrencyCacheForTest{totalAccountConcurrency: -1}
	_, err := NewConcurrencyService(cache).GetTotalAccountConcurrency(context.Background())
	require.Error(t, err)
}
```

- [ ] **Step 2: Run the service test and verify RED**

```powershell
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
& gofmt -w internal/service/concurrency_service.go internal/service/concurrency_service_test.go internal/repository/concurrency_cache.go internal/repository/concurrency_cache_integration_test.go
if ($LASTEXITCODE -ne 0) { throw "gofmt exited with code $LASTEXITCODE" }
& go test -tags unit ./internal/service -run 'TestConcurrencyService_GetTotalAccountConcurrency' -count=1
if ($LASTEXITCODE -ne 0) { throw "go test exited with code $LASTEXITCODE" }
```

Expected: FAIL because `GetTotalAccountConcurrency` is not defined.

- [ ] **Step 3: Add the optional cache interface and service method**

Add this interface without expanding `ConcurrencyCache`, so existing test stubs remain compatible:

```go
type TotalAccountConcurrencyCache interface {
	GetTotalAccountConcurrency(ctx context.Context) (int, error)
}
```

Add the service method with a bounded request context:

```go
func (s *ConcurrencyService) GetTotalAccountConcurrency(ctx context.Context) (int, error) {
	if s == nil || s.cache == nil {
		return 0, errors.New("concurrency cache is unavailable")
	}
	cache, ok := s.cache.(TotalAccountConcurrencyCache)
	if !ok {
		return 0, errors.New("total account concurrency is unsupported")
	}
	redisCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	current, err := cache.GetTotalAccountConcurrency(redisCtx)
	if err != nil {
		return 0, err
	}
	if current < 0 {
		return 0, errors.New("total account concurrency cannot be negative")
	}
	return current, nil
}
```

- [ ] **Step 4: Write the failing Redis integration test**

Add a test that creates two active accounts, verifies the sum, releases one slot, and inserts an expired member that must not be counted:

```go
func (s *ConcurrencyCacheSuite) TestTotalAccountConcurrency_UsesActiveIndexAndDropsExpiredSlots() {
	require.NoError(s.T(), s.rdb.FlushDB(s.ctx).Err())

	ok, err := s.rawCache.AcquireAccountSlot(s.ctx, 701, 5, "req-701-a")
	require.NoError(s.T(), err)
	require.True(s.T(), ok)
	ok, err = s.rawCache.AcquireAccountSlot(s.ctx, 701, 5, "req-701-b")
	require.NoError(s.T(), err)
	require.True(s.T(), ok)
	ok, err = s.rawCache.AcquireAccountSlot(s.ctx, 702, 5, "req-702-a")
	require.NoError(s.T(), err)
	require.True(s.T(), ok)

	now, err := s.rawCache.redisUnixSeconds(s.ctx)
	require.NoError(s.T(), err)
	require.NoError(s.T(), s.rdb.ZAdd(s.ctx, accountSlotKey(703), redis.Z{
		Score: float64(now - int64(s.rawCache.slotTTLSeconds) - 1), Member: "expired",
	}).Err())
	require.NoError(s.T(), s.rdb.ZAdd(s.ctx, accountActiveIndexKey, redis.Z{
		Score: float64(now + int64(s.rawCache.slotTTLSeconds)), Member: "703",
	}).Err())

	got, err := s.rawCache.GetTotalAccountConcurrency(s.ctx)
	require.NoError(s.T(), err)
	require.Equal(s.T(), 3, got)

	require.NoError(s.T(), s.rawCache.ReleaseAccountSlot(s.ctx, 701, "req-701-a"))
	got, err = s.rawCache.GetTotalAccountConcurrency(s.ctx)
	require.NoError(s.T(), err)
	require.Equal(s.T(), 2, got)
}
```

- [ ] **Step 5: Run the repository test and verify RED**

```powershell
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
& go test -tags integration ./internal/repository -run 'TestConcurrencyCacheSuite/TestTotalAccountConcurrency' -count=1
if ($LASTEXITCODE -ne 0) { throw "go test exited with code $LASTEXITCODE" }
```

Expected: FAIL because `concurrencyCache.GetTotalAccountConcurrency` is not defined. If the integration Redis dependency is unavailable, record that exact environment failure and continue with the unit test; do not misreport it as a product failure.

- [ ] **Step 6: Implement active-index aggregation**

Add a repository method that reads only non-expired active-index members, parses account IDs, reuses the existing batch reader (which prunes expired slots), and sums the result:

```go
func (c *concurrencyCache) GetTotalAccountConcurrency(ctx context.Context) (int, error) {
	now, err := c.redisUnixSeconds(ctx)
	if err != nil {
		return 0, err
	}
	members, err := c.rdb.ZRangeByScore(ctx, accountActiveIndexKey, &redis.ZRangeBy{
		Min: "(" + strconv.FormatInt(now, 10),
		Max: "+inf",
	}).Result()
	if err != nil {
		return 0, fmt.Errorf("read active account index: %w", err)
	}
	accountIDs := make([]int64, 0, len(members))
	for _, member := range members {
		accountID, parseErr := strconv.ParseInt(member, 10, 64)
		if parseErr == nil && accountID > 0 {
			accountIDs = append(accountIDs, accountID)
		}
	}
	counts, err := c.GetAccountConcurrencyBatch(ctx, accountIDs)
	if err != nil {
		return 0, err
	}
	total := 0
	for _, count := range counts {
		total += count
	}
	return total, nil
}
```

- [ ] **Step 7: Verify GREEN and commit**

```powershell
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
& go test -tags unit ./internal/service -run 'TestConcurrencyService_GetTotalAccountConcurrency' -count=1
if ($LASTEXITCODE -ne 0) { throw "service tests exited with code $LASTEXITCODE" }
& go test -tags integration ./internal/repository -run 'TestConcurrencyCacheSuite/TestTotalAccountConcurrency' -count=1
if ($LASTEXITCODE -ne 0) { throw "repository tests exited with code $LASTEXITCODE" }
& git add -- backend/internal/service/concurrency_service.go backend/internal/service/concurrency_service_test.go backend/internal/repository/concurrency_cache.go backend/internal/repository/concurrency_cache_integration_test.go
if ($LASTEXITCODE -ne 0) { throw "git add exited with code $LASTEXITCODE" }
& git commit -m 'feat: aggregate active api concurrency'
if ($LASTEXITCODE -ne 0) { throw "git commit exited with code $LASTEXITCODE" }
```

---

### Task 2: Cached Public Status Endpoint

**Files:**
- Create: `backend/internal/service/ops_public_status.go`
- Create: `backend/internal/service/ops_public_status_test.go`
- Modify: `backend/internal/service/ops_service.go`
- Modify: `backend/internal/server/routes/common.go`
- Create: `backend/internal/server/routes/common_test.go`
- Modify: `backend/internal/server/router.go`

**Interfaces:**
- Consumes: `ConcurrencyService.GetTotalAccountConcurrency(context.Context) (int, error)`.
- Produces: `OpsService.GetPublicConcurrency(context.Context) (*PublicConcurrencyStatus, error)` and anonymous `GET /api/v1/status/concurrency`.

- [ ] **Step 1: Write failing cache and error tests**

Create `ops_public_status_test.go` with tests proving the two-second success cache, retry-after-error behavior, and service-unavailable mapping. Use the Task 1 stub counter:

```go
func TestOpsService_GetPublicConcurrencyCachesSuccess(t *testing.T) {
	cache := &stubConcurrencyCacheForTest{totalAccountConcurrency: 4}
	svc := &OpsService{
		concurrencyService: NewConcurrencyService(cache),
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
	cache := &stubConcurrencyCacheForTest{totalAccountConcurrencyErr: errors.New("redis unavailable")}
	svc := &OpsService{
		concurrencyService: NewConcurrencyService(cache),
		publicConcurrencyCacheTTL: 2 * time.Second,
	}

	_, firstErr := svc.GetPublicConcurrency(context.Background())
	_, secondErr := svc.GetPublicConcurrency(context.Background())
	require.Error(t, firstErr)
	require.Error(t, secondErr)
	require.Equal(t, int64(2), cache.totalAccountConcurrencyCalls.Load())
}

func TestOpsService_GetPublicConcurrencyRefreshesAfterExpiry(t *testing.T) {
	cache := &stubConcurrencyCacheForTest{totalAccountConcurrency: 2}
	svc := &OpsService{
		concurrencyService: NewConcurrencyService(cache),
		publicConcurrencyCacheTTL: 10 * time.Millisecond,
	}

	first, err := svc.GetPublicConcurrency(context.Background())
	require.NoError(t, err)
	time.Sleep(20 * time.Millisecond)
	cache.totalAccountConcurrency = 5
	second, err := svc.GetPublicConcurrency(context.Background())
	require.NoError(t, err)
	require.Equal(t, 2, first.Current)
	require.Equal(t, 5, second.Current)
	require.Equal(t, int64(2), cache.totalAccountConcurrencyCalls.Load())
}
```

Start the new test file with `//go:build unit` so it shares the existing unit-test stub only when the unit test tag is enabled.

- [ ] **Step 2: Run service tests and verify RED**

```powershell
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
& gofmt -w internal/service/ops_public_status.go internal/service/ops_public_status_test.go internal/service/ops_service.go internal/server/routes/common.go internal/server/routes/common_test.go internal/server/router.go
if ($LASTEXITCODE -ne 0) { throw "gofmt exited with code $LASTEXITCODE" }
& go test -tags unit ./internal/service -run 'TestOpsService_GetPublicConcurrency' -count=1
if ($LASTEXITCODE -ne 0) { throw "go test exited with code $LASTEXITCODE" }
```

Expected: FAIL because public status types and methods do not exist.

- [ ] **Step 3: Implement the cached public status service**

Add `sync.Mutex` protected fields to `OpsService`, initialize the TTL to `2 * time.Second` in `NewOpsService`, and create:

```go
const defaultPublicConcurrencyCacheTTL = 2 * time.Second

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
		return nil, infraerrors.ServiceUnavailable("CONCURRENCY_STATUS_UNAVAILABLE", "realtime concurrency unavailable")
	}
	s.publicConcurrencyMu.Lock()
	defer s.publicConcurrencyMu.Unlock()

	now := time.Now().UTC()
	if now.Before(s.publicConcurrencyCache.expiresAt) {
		cached := s.publicConcurrencyCache.status
		return &cached, nil
	}
	current, err := s.concurrencyService.GetTotalAccountConcurrency(ctx)
	if err != nil {
		return nil, infraerrors.ServiceUnavailable("CONCURRENCY_STATUS_UNAVAILABLE", "realtime concurrency unavailable")
	}
	status := PublicConcurrencyStatus{Current: current, UpdatedAt: now}
	ttl := s.publicConcurrencyCacheTTL
	if ttl <= 0 {
		ttl = defaultPublicConcurrencyCacheTTL
	}
	s.publicConcurrencyCache = cachedPublicConcurrency{status: status, expiresAt: now.Add(ttl)}
	return &status, nil
}
```

- [ ] **Step 4: Write failing route contract tests**

Define a small route-local provider interface and a fake returning current `9`. Assert HTTP 200, exact direct JSON keys `current` and `updated_at`, and absence of `account`, `user`, `platform`, `capacity`, and `waiting`. Add an error case asserting HTTP 503.

```go
type publicConcurrencyProviderStub struct {
	status *service.PublicConcurrencyStatus
	err    error
}

func (s *publicConcurrencyProviderStub) GetPublicConcurrency(context.Context) (*service.PublicConcurrencyStatus, error) {
	return s.status, s.err
}
```

- [ ] **Step 5: Run route tests and verify RED**

```powershell
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
& go test ./internal/server/routes -run 'TestPublicConcurrency' -count=1
if ($LASTEXITCODE -ne 0) { throw "go test exited with code $LASTEXITCODE" }
```

Expected: FAIL because `RegisterCommonRoutes` has no provider and the endpoint is absent.

- [ ] **Step 6: Register the anonymous endpoint**

Change `RegisterCommonRoutes` to accept this interface:

```go
type publicConcurrencyProvider interface {
	GetPublicConcurrency(context.Context) (*service.PublicConcurrencyStatus, error)
}
```

Register the exact endpoint with direct success JSON and standardized errors:

```go
r.GET("/api/v1/status/concurrency", func(c *gin.Context) {
	if concurrencyProvider == nil {
		response.Error(c, http.StatusServiceUnavailable, "realtime concurrency unavailable")
		return
	}
	status, err := concurrencyProvider.GetPublicConcurrency(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.JSON(http.StatusOK, status)
})
```

Update `router.go` to call `routes.RegisterCommonRoutes(r, opsService)`.

- [ ] **Step 7: Verify GREEN and commit**

```powershell
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
& go test -tags unit ./internal/service -run 'TestOpsService_GetPublicConcurrency' -count=1
if ($LASTEXITCODE -ne 0) { throw "service tests exited with code $LASTEXITCODE" }
& go test ./internal/server/routes -run 'TestPublicConcurrency' -count=1
if ($LASTEXITCODE -ne 0) { throw "route tests exited with code $LASTEXITCODE" }
& git add -- backend/internal/service/ops_public_status.go backend/internal/service/ops_public_status_test.go backend/internal/service/ops_service.go backend/internal/server/routes/common.go backend/internal/server/routes/common_test.go backend/internal/server/router.go
if ($LASTEXITCODE -ne 0) { throw "git add exited with code $LASTEXITCODE" }
& git commit -m 'feat: expose public concurrency status'
if ($LASTEXITCODE -ne 0) { throw "git commit exited with code $LASTEXITCODE" }
```

---

### Task 3: Frontend Polling API and Lifecycle

**Files:**
- Create: `frontend/src/api/status.ts`
- Modify: `frontend/src/api/index.ts`
- Create: `frontend/src/composables/useRealtimeConcurrency.ts`
- Create: `frontend/src/composables/__tests__/useRealtimeConcurrency.spec.ts`

**Interfaces:**
- Consumes: anonymous `GET /status/concurrency` through the existing Axios base URL `/api/v1`.
- Produces: `statusAPI.getRealtimeConcurrency(): Promise<RealtimeConcurrencyStatus>` and `useRealtimeConcurrency(enabled, 5000)` returning `current` and `available` refs.

- [ ] **Step 1: Write failing composable lifecycle tests**

Mock `statusAPI.getRealtimeConcurrency`, mount a tiny harness component, and cover: immediate fetch, five-second refresh, hidden-page pause, immediate refresh on visibility restoration, disabled custom-home mode, failure to `--` state, and unmount cleanup.

```ts
vi.mock('@/api/status', () => ({
  statusAPI: { getRealtimeConcurrency: vi.fn() }
}))

it('refreshes every five seconds and pauses while hidden', async () => {
  vi.useFakeTimers()
  let hidden = false
  vi.spyOn(document, 'hidden', 'get').mockImplementation(() => hidden)
  vi.mocked(statusAPI.getRealtimeConcurrency).mockResolvedValue({
    current: 3,
    updated_at: '2026-07-14T12:00:00Z'
  })
  const enabled = ref(true)
  const wrapper = mountHarness(enabled)
  await flushPromises()
  expect(statusAPI.getRealtimeConcurrency).toHaveBeenCalledTimes(1)

  await vi.advanceTimersByTimeAsync(5000)
  expect(statusAPI.getRealtimeConcurrency).toHaveBeenCalledTimes(2)

  hidden = true
  document.dispatchEvent(new Event('visibilitychange'))
  await vi.advanceTimersByTimeAsync(10000)
  expect(statusAPI.getRealtimeConcurrency).toHaveBeenCalledTimes(2)

  hidden = false
  document.dispatchEvent(new Event('visibilitychange'))
  await flushPromises()
  expect(statusAPI.getRealtimeConcurrency).toHaveBeenCalledTimes(3)
  wrapper.unmount()
})
```

- [ ] **Step 2: Run composable tests and verify RED**

```powershell
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
& pnpm --dir frontend test:run useRealtimeConcurrency
if ($LASTEXITCODE -ne 0) { throw "pnpm test exited with code $LASTEXITCODE" }
```

Expected: FAIL because the API module and composable do not exist.

- [ ] **Step 3: Implement the typed API module**

```ts
import { apiClient } from './client'

export interface RealtimeConcurrencyStatus {
  current: number
  updated_at: string
}

async function getRealtimeConcurrency(): Promise<RealtimeConcurrencyStatus> {
  const { data } = await apiClient.get<RealtimeConcurrencyStatus>('/status/concurrency')
  return data
}

export const statusAPI = { getRealtimeConcurrency }
```

Export `statusAPI` and `RealtimeConcurrencyStatus` from `frontend/src/api/index.ts`.

- [ ] **Step 4: Implement the polling composable**

Use `onMounted`, `onUnmounted`, and `watch` so disabled custom-home mode never fetches. On any error or invalid negative/non-integer response, set `current` to `null` and `available` to `false`. Keep one interval only, clear it on hide/disable/unmount, and refresh immediately on show/enable.

```ts
export function useRealtimeConcurrency(enabled: Readonly<Ref<boolean>>, intervalMs = 5000) {
  const current = ref<number | null>(null)
  const available = ref(false)
  let timer: ReturnType<typeof setInterval> | null = null
  let mounted = false

  async function refresh() {
    if (!mounted || !enabled.value || document.hidden) return
    try {
      const status = await statusAPI.getRealtimeConcurrency()
      if (!Number.isInteger(status.current) || status.current < 0) throw new Error('invalid concurrency')
      current.value = status.current
      available.value = true
    } catch {
      current.value = null
      available.value = false
    }
  }

  function stopTimer() {
    if (timer !== null) clearInterval(timer)
    timer = null
  }

  function start() {
    stopTimer()
    if (!mounted || !enabled.value || document.hidden) return
    void refresh()
    timer = setInterval(() => void refresh(), intervalMs)
  }

  function onVisibilityChange() {
    if (document.hidden) stopTimer()
    else start()
  }

  watch(enabled, (value) => {
    if (!mounted) return
    if (value) start()
    else {
      stopTimer()
      current.value = null
      available.value = false
    }
  })

  onMounted(() => {
    mounted = true
    document.addEventListener('visibilitychange', onVisibilityChange)
    start()
  })

  onUnmounted(() => {
    mounted = false
    stopTimer()
    document.removeEventListener('visibilitychange', onVisibilityChange)
  })

  return { current, available, refresh }
}
```

- [ ] **Step 5: Verify GREEN and commit**

```powershell
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
& pnpm --dir frontend test:run useRealtimeConcurrency
if ($LASTEXITCODE -ne 0) { throw "pnpm test exited with code $LASTEXITCODE" }
& git add -- frontend/src/api/status.ts frontend/src/api/index.ts frontend/src/composables/useRealtimeConcurrency.ts frontend/src/composables/__tests__/useRealtimeConcurrency.spec.ts
if ($LASTEXITCODE -ne 0) { throw "git add exited with code $LASTEXITCODE" }
& git commit -m 'feat: poll realtime concurrency status'
if ($LASTEXITCODE -ne 0) { throw "git commit exited with code $LASTEXITCODE" }
```

---

### Task 4: Homepage Metric and Content Removal

**Files:**
- Create: `frontend/src/components/home/RealtimeConcurrencyStatus.vue`
- Create: `frontend/src/components/home/__tests__/RealtimeConcurrencyStatus.spec.ts`
- Create: `frontend/src/views/__tests__/HomeViewContent.spec.ts`
- Modify: `frontend/src/views/HomeView.vue`
- Modify: `frontend/src/i18n/locales/zh/landing.ts`
- Modify: `frontend/src/i18n/locales/en/landing.ts`

**Interfaces:**
- Consumes: `useRealtimeConcurrency(computed(() => !homeContent.value), 5000)`.
- Produces: a single unframed metric section below the feature grid.

- [ ] **Step 1: Write failing component and source contract tests**

The component test mounts with `current: 12, available: true` and asserts the title, `12`, update copy, and active status marker. A second case mounts with `current: null, available: false` and asserts `--`.

The HomeView source contract test reads `HomeView.vue` and asserts:

```ts
expect(source).not.toContain("t('home.providers.title')")
expect(source).not.toContain('<footer')
expect(source).toContain('<RealtimeConcurrencyStatus')
expect(source).toContain('useRealtimeConcurrency')
```

- [ ] **Step 2: Run UI tests and verify RED**

```powershell
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
& pnpm --dir frontend test:run RealtimeConcurrencyStatus HomeViewContent
if ($LASTEXITCODE -ne 0) { throw "pnpm test exited with code $LASTEXITCODE" }
```

Expected: FAIL because the component is absent and HomeView still contains provider/footer markup.

- [ ] **Step 3: Implement the unframed status component**

```vue
<template>
  <section class="pb-8 pt-4 text-center" aria-live="polite">
    <div class="flex items-center justify-center gap-2">
      <span
        data-testid="status-dot"
        class="h-2 w-2 rounded-full"
        :class="available ? 'bg-emerald-500 shadow-[0_0_10px_rgba(16,185,129,0.65)]' : 'bg-gray-300 dark:bg-dark-600'"
      />
      <p class="text-sm font-medium text-gray-500 dark:text-dark-400">
        {{ t('home.realtimeConcurrency.title') }}
      </p>
    </div>
    <p class="mt-3 text-5xl font-bold text-gray-900 dark:text-white">
      {{ displayValue }}
    </p>
    <p class="mt-2 text-xs text-gray-400 dark:text-dark-500">
      {{ t('home.realtimeConcurrency.refreshHint') }}
    </p>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

const props = defineProps<{ current: number | null; available: boolean }>()
const { t } = useI18n()
const displayValue = computed(() => props.available && props.current !== null ? String(props.current) : '--')
</script>
```

- [ ] **Step 4: Update HomeView and translations**

Delete the supported-provider heading/list and the complete footer. Insert the component immediately after the feature grid. Import the component and composable, remove the now-unused `githubUrl` and `currentYear`, and initialize:

```ts
const showDefaultHome = computed(() => !homeContent.value)
const {
  current: currentConcurrency,
  available: concurrencyAvailable
} = useRealtimeConcurrency(showDefaultHome, 5000)
```

Add exact translations:

```ts
realtimeConcurrency: {
  title: '实时 API 并发',
  refreshHint: '每 5 秒更新'
}
```

```ts
realtimeConcurrency: {
  title: 'Live API concurrency',
  refreshHint: 'Updates every 5 seconds'
}
```

- [ ] **Step 5: Verify GREEN, typecheck, and commit**

```powershell
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
& pnpm --dir frontend test:run RealtimeConcurrencyStatus HomeViewContent useRealtimeConcurrency
if ($LASTEXITCODE -ne 0) { throw "pnpm tests exited with code $LASTEXITCODE" }
& pnpm --dir frontend typecheck
if ($LASTEXITCODE -ne 0) { throw "pnpm typecheck exited with code $LASTEXITCODE" }
& git add -- frontend/src/components/home/RealtimeConcurrencyStatus.vue frontend/src/components/home/__tests__/RealtimeConcurrencyStatus.spec.ts frontend/src/views/__tests__/HomeViewContent.spec.ts frontend/src/views/HomeView.vue frontend/src/i18n/locales/zh/landing.ts frontend/src/i18n/locales/en/landing.ts
if ($LASTEXITCODE -ne 0) { throw "git add exited with code $LASTEXITCODE" }
& git commit -m 'feat: show homepage api concurrency'
if ($LASTEXITCODE -ne 0) { throw "git commit exited with code $LASTEXITCODE" }
```

---

### Task 5: Full Verification, Visual QA, and Production Rollout

**Files:**
- Modify only if verification exposes a defect in files already listed above.
- Produce: `build-local/sub2api`, `deploy-artifacts/sub2api-custom-0.1.153-ui2-context.tar.gz`.

**Interfaces:**
- Consumes: all prior task deliverables.
- Produces: verified local build and production image `sub2api-custom:0.1.153-ui2` with rollback to `0.1.153-ui1` preserved.

- [ ] **Step 1: Run the complete relevant test and build suite**

```powershell
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
& go -C backend test -tags unit ./internal/service -run 'TestConcurrencyService_GetTotalAccountConcurrency|TestOpsService_GetPublicConcurrency' -count=1
if ($LASTEXITCODE -ne 0) { throw "service tests exited with code $LASTEXITCODE" }
& go -C backend test ./internal/server/routes -run 'TestPublicConcurrency' -count=1
if ($LASTEXITCODE -ne 0) { throw "route tests exited with code $LASTEXITCODE" }
& go -C backend test -tags embed ./internal/web -count=1
if ($LASTEXITCODE -ne 0) { throw "embed tests exited with code $LASTEXITCODE" }
& pnpm --dir frontend test:run RealtimeConcurrencyStatus HomeViewContent useRealtimeConcurrency
if ($LASTEXITCODE -ne 0) { throw "frontend tests exited with code $LASTEXITCODE" }
& pnpm --dir frontend typecheck
if ($LASTEXITCODE -ne 0) { throw "frontend typecheck exited with code $LASTEXITCODE" }
& pnpm --dir frontend build
if ($LASTEXITCODE -ne 0) { throw "frontend build exited with code $LASTEXITCODE" }
& git diff --check
if ($LASTEXITCODE -ne 0) { throw "git diff --check exited with code $LASTEXITCODE" }
```

- [ ] **Step 2: Start a local server and perform visual QA**

Build the embedded server with tag `0.1.153-ui2`, then start the Vite frontend on an unused local port for layout inspection. Verify the page contains no supported-models section or footer, the status metric is centered below the feature grid, text does not overlap, and dark mode remains legible. The local Vite session may show `--` without a backend; the production check in Step 5 must prove the live endpoint displays a non-negative value.

```powershell
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$env:CGO_ENABLED = '0'
& go -C backend build -tags embed -trimpath -ldflags '-s -w -X main.Version=0.1.153-ui2 -X main.BuildType=release' -o ..\build-local\sub2api.exe .\cmd\server
if ($LASTEXITCODE -ne 0) { throw "go build exited with code $LASTEXITCODE" }
$repo = (Resolve-Path -LiteralPath '.').Path
$pwsh = (Get-Command pwsh -ErrorAction Stop).Source
$devScript = "`$ErrorActionPreference = 'Stop'; Set-StrictMode -Version Latest; Set-Location -LiteralPath '$repo'; & pnpm --dir frontend dev --host 127.0.0.1 --port 4173; if (`$LASTEXITCODE -ne 0) { throw 'pnpm dev failed' }"
$devProcess = Start-Process -FilePath $pwsh -ArgumentList @('-NoLogo', '-NoProfile', '-Command', $devScript) -WindowStyle Hidden -PassThru
for ($attempt = 0; $attempt -lt 30; $attempt++) {
  try {
    $response = Invoke-WebRequest -Uri 'http://127.0.0.1:4173/home' -TimeoutSec 2
    if ($response.StatusCode -eq 200) { break }
  } catch {
    if ($attempt -eq 29) { throw }
  }
  Start-Sleep -Milliseconds 500
}
```

Use the in-app browser to capture desktop `1440x900` and mobile `390x844` screenshots. After inspection, stop only `$devProcess` and verify port 4173 is no longer listening.

- [ ] **Step 3: Build the Linux artifact and package the image context**

Run from `backend`, then package from the repository root:

```powershell
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$tag = '0.1.153-ui2'
$env:CGO_ENABLED = '0'
$env:GOOS = 'linux'
$env:GOARCH = 'amd64'
$commit = (& git rev-parse HEAD).Trim()
if ($LASTEXITCODE -ne 0) { throw "git rev-parse exited with code $LASTEXITCODE" }
$date = [DateTimeOffset]::UtcNow.ToString('yyyy-MM-ddTHH:mm:ssZ')
& go build -tags embed -trimpath -ldflags "-s -w -X main.Version=$tag -X main.Commit=$commit -X main.Date=$date -X main.BuildType=release" -o ..\build-local\sub2api .\cmd\server
if ($LASTEXITCODE -ne 0) { throw "go build exited with code $LASTEXITCODE" }
```

Use the existing packaging section in `docs/custom-upgrade-workflow.md` with `$tag = '0.1.153-ui2'`, producing exactly `deploy-artifacts/sub2api-custom-0.1.153-ui2-context.tar.gz`. Immediately check the `tar` exit code and list the archive root to confirm `Dockerfile`, `build-local/`, `backend/`, and `deploy/` are at the root.

- [ ] **Step 4: Upload, build, and switch only the application service**

Upload the archive to `/opt/sub2api-build/sub2api-custom-0.1.153-ui2/context.tar.gz`. On the Linux host, extract into a fresh `context` directory, build `sub2api-custom:0.1.153-ui2`, back up `docker-compose.override.yml`, replace only `sub2api-custom:0.1.153-ui1` with `sub2api-custom:0.1.153-ui2`, and run:

```bash
docker compose -f docker-compose.local.yml -f docker-compose.override.yml -f docker-compose.leaderboard.yml up -d --no-deps sub2api
```

This code block is executed only inside the remote Linux shell; local orchestration remains in `pwsh`.

- [ ] **Step 5: Verify production and preserve rollback evidence**

Verify all of the following from fresh production responses:

- `docker compose -f docker-compose.local.yml -f docker-compose.override.yml -f docker-compose.leaderboard.yml ps` reports the `sub2api` container healthy and PostgreSQL, Redis, and leaderboard were not recreated.
- `curl -fsS http://127.0.0.1:8080/health` succeeds.
- `curl -fsS http://127.0.0.1:8080/api/v1/status/concurrency` returns only `current` and `updated_at`, with `current >= 0`.
- `https://api.zhouz.online/home` no longer contains the supported-models section after the browser loads the embedded frontend.
- Desktop and mobile screenshots show the centered metric without overlap.
- The previous image `sub2api-custom:0.1.153-ui1` and the timestamped compose backup remain available for rollback.

- [ ] **Step 6: Commit any verification-only corrections and record final status**

If verification required corrections, rerun the complete Step 1 suite before committing them. Confirm the final worktree contains only the pre-existing untracked `.superpowers/` directory and intentional generated artifacts before reporting completion.
