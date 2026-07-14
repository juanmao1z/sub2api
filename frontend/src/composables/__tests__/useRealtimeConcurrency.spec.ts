import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent, h, ref, type Ref } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { statusAPI, type RealtimeConcurrencyStatus } from '@/api/status'
import { useRealtimeConcurrency } from '@/composables/useRealtimeConcurrency'

vi.mock('@/api/status', () => ({
  statusAPI: { getRealtimeConcurrency: vi.fn() },
}))

let hidden = false

function deferred<T>() {
  let resolve!: (value: T | PromiseLike<T>) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise
    reject = rejectPromise
  })
  return { promise, resolve, reject }
}

function mountHarness(enabled: Ref<boolean>, intervalMs = 5000) {
  let state!: ReturnType<typeof useRealtimeConcurrency>
  const wrapper = mount(defineComponent({
    setup() {
      state = useRealtimeConcurrency(enabled, intervalMs)
      return () => h('div')
    },
  }))

  return { wrapper, state }
}

describe('useRealtimeConcurrency', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.clearAllMocks()
    hidden = false
    vi.spyOn(document, 'hidden', 'get').mockImplementation(() => hidden)
  })

  afterEach(() => {
    vi.restoreAllMocks()
    vi.useRealTimers()
  })

  it('fetches immediately and refreshes on the configured interval', async () => {
    vi.mocked(statusAPI.getRealtimeConcurrency).mockResolvedValue({
      current: 3,
      updated_at: '2026-07-14T12:00:00Z',
    })

    const { wrapper, state } = mountHarness(ref(true))
    await flushPromises()

    expect(statusAPI.getRealtimeConcurrency).toHaveBeenCalledTimes(1)
    expect(state.current.value).toBe(3)
    expect(state.available.value).toBe(true)

    await vi.advanceTimersByTimeAsync(5000)

    expect(statusAPI.getRealtimeConcurrency).toHaveBeenCalledTimes(2)
    wrapper.unmount()
  })

  it('pauses while hidden and refreshes immediately when visible again', async () => {
    vi.mocked(statusAPI.getRealtimeConcurrency).mockResolvedValue({
      current: 4,
      updated_at: '2026-07-14T12:00:00Z',
    })

    const { wrapper } = mountHarness(ref(true))
    await flushPromises()

    hidden = true
    document.dispatchEvent(new Event('visibilitychange'))
    await vi.advanceTimersByTimeAsync(10000)
    expect(statusAPI.getRealtimeConcurrency).toHaveBeenCalledTimes(1)

    hidden = false
    document.dispatchEvent(new Event('visibilitychange'))
    await flushPromises()
    expect(statusAPI.getRealtimeConcurrency).toHaveBeenCalledTimes(2)
    wrapper.unmount()
  })

  it('does not fetch while disabled and starts immediately when enabled', async () => {
    vi.mocked(statusAPI.getRealtimeConcurrency).mockResolvedValue({
      current: 5,
      updated_at: '2026-07-14T12:00:00Z',
    })
    const enabled = ref(false)
    const { wrapper, state } = mountHarness(enabled)
    await flushPromises()

    expect(statusAPI.getRealtimeConcurrency).not.toHaveBeenCalled()
    expect(state.current.value).toBeNull()
    expect(state.available.value).toBe(false)

    enabled.value = true
    await flushPromises()
    expect(statusAPI.getRealtimeConcurrency).toHaveBeenCalledTimes(1)
    expect(state.current.value).toBe(5)
    expect(state.available.value).toBe(true)

    enabled.value = false
    await flushPromises()
    expect(state.current.value).toBeNull()
    expect(state.available.value).toBe(false)

    await vi.advanceTimersByTimeAsync(10000)
    expect(statusAPI.getRealtimeConcurrency).toHaveBeenCalledTimes(1)
    wrapper.unmount()
  })

  it('clears unavailable state after a failure and retries on the next cycle', async () => {
    vi.mocked(statusAPI.getRealtimeConcurrency)
      .mockRejectedValueOnce(new Error('network failure'))
      .mockResolvedValueOnce({
        current: 6,
        updated_at: '2026-07-14T12:00:05Z',
      })

    const { wrapper, state } = mountHarness(ref(true))
    await flushPromises()
    expect(state.current.value).toBeNull()
    expect(state.available.value).toBe(false)

    await vi.advanceTimersByTimeAsync(5000)
    expect(state.current.value).toBe(6)
    expect(state.available.value).toBe(true)
    wrapper.unmount()
  })

  it.each([
    { current: -1, updated_at: '2026-07-14T12:00:00Z' },
    { current: 1.5, updated_at: '2026-07-14T12:00:00Z' },
  ])('rejects invalid concurrency status $current', async (status) => {
    vi.mocked(statusAPI.getRealtimeConcurrency).mockResolvedValue(
      status as RealtimeConcurrencyStatus,
    )

    const { wrapper, state } = mountHarness(ref(true))
    await flushPromises()

    expect(state.current.value).toBeNull()
    expect(state.available.value).toBe(false)
    wrapper.unmount()
  })

  it('prevents overlapping refresh requests', async () => {
    const request = deferred<RealtimeConcurrencyStatus>()
    vi.mocked(statusAPI.getRealtimeConcurrency).mockReturnValue(request.promise)

    const { wrapper } = mountHarness(ref(true))
    await flushPromises()
    await vi.advanceTimersByTimeAsync(15000)

    expect(statusAPI.getRealtimeConcurrency).toHaveBeenCalledTimes(1)

    request.resolve({ current: 7, updated_at: '2026-07-14T12:00:15Z' })
    await flushPromises()
    await vi.advanceTimersByTimeAsync(5000)
    expect(statusAPI.getRealtimeConcurrency).toHaveBeenCalledTimes(2)
    wrapper.unmount()
  })

  it('discards a response that resolves after disabling', async () => {
    const request = deferred<RealtimeConcurrencyStatus>()
    vi.mocked(statusAPI.getRealtimeConcurrency).mockReturnValue(request.promise)
    const enabled = ref(true)
    const { wrapper, state } = mountHarness(enabled)
    await flushPromises()

    enabled.value = false
    await flushPromises()
    request.resolve({ current: 8, updated_at: '2026-07-14T12:00:00Z' })
    await flushPromises()

    expect(state.current.value).toBeNull()
    expect(state.available.value).toBe(false)
    wrapper.unmount()
  })

  it('discards a response that resolves after the page becomes hidden', async () => {
    const firstRequest = deferred<RealtimeConcurrencyStatus>()
    vi.mocked(statusAPI.getRealtimeConcurrency).mockReturnValue(firstRequest.promise)
    const { wrapper, state } = mountHarness(ref(true))
    await flushPromises()

    hidden = true
    document.dispatchEvent(new Event('visibilitychange'))
    firstRequest.resolve({ current: 9, updated_at: '2026-07-14T12:00:00Z' })
    await flushPromises()

    expect(state.current.value).toBeNull()
    expect(state.available.value).toBe(false)
    wrapper.unmount()
  })

  it('aborts a hidden request and refreshes immediately when visible again', async () => {
    const firstRequest = deferred<RealtimeConcurrencyStatus>()
    vi.mocked(statusAPI.getRealtimeConcurrency)
      .mockReturnValueOnce(firstRequest.promise)
      .mockResolvedValueOnce({
        current: 11,
        updated_at: '2026-07-14T12:00:01Z',
      })
    const { wrapper, state } = mountHarness(ref(true))
    await flushPromises()

    const firstSignal = vi.mocked(statusAPI.getRealtimeConcurrency).mock.calls[0][0] as AbortSignal
    expect(firstSignal).toBeInstanceOf(AbortSignal)

    hidden = true
    document.dispatchEvent(new Event('visibilitychange'))
    expect(firstSignal.aborted).toBe(true)

    hidden = false
    document.dispatchEvent(new Event('visibilitychange'))
    await flushPromises()

    expect(statusAPI.getRealtimeConcurrency).toHaveBeenCalledTimes(2)
    expect(state.current.value).toBe(11)
    expect(state.available.value).toBe(true)

    firstRequest.resolve({ current: 99, updated_at: '2026-07-14T12:00:02Z' })
    await flushPromises()
    expect(state.current.value).toBe(11)
    wrapper.unmount()
  })

  it('aborts a disabled request and refreshes immediately when re-enabled', async () => {
    const firstRequest = deferred<RealtimeConcurrencyStatus>()
    vi.mocked(statusAPI.getRealtimeConcurrency)
      .mockReturnValueOnce(firstRequest.promise)
      .mockResolvedValueOnce({
        current: 12,
        updated_at: '2026-07-14T12:00:01Z',
      })
    const enabled = ref(true)
    const { wrapper, state } = mountHarness(enabled)
    await flushPromises()

    const firstSignal = vi.mocked(statusAPI.getRealtimeConcurrency).mock.calls[0][0] as AbortSignal
    expect(firstSignal).toBeInstanceOf(AbortSignal)

    enabled.value = false
    await flushPromises()
    expect(firstSignal.aborted).toBe(true)

    enabled.value = true
    await flushPromises()

    expect(statusAPI.getRealtimeConcurrency).toHaveBeenCalledTimes(2)
    expect(state.current.value).toBe(12)
    expect(state.available.value).toBe(true)

    firstRequest.resolve({ current: 99, updated_at: '2026-07-14T12:00:02Z' })
    await flushPromises()
    expect(state.current.value).toBe(12)
    wrapper.unmount()
  })

  it('discards a response and cleans up polling after unmount', async () => {
    const request = deferred<RealtimeConcurrencyStatus>()
    vi.mocked(statusAPI.getRealtimeConcurrency).mockReturnValue(request.promise)
    const { wrapper, state } = mountHarness(ref(true))
    await flushPromises()

    const signal = vi.mocked(statusAPI.getRealtimeConcurrency).mock.calls[0][0] as AbortSignal

    wrapper.unmount()
    expect(signal).toBeInstanceOf(AbortSignal)
    expect(signal.aborted).toBe(true)
    request.resolve({ current: 10, updated_at: '2026-07-14T12:00:00Z' })
    await flushPromises()
    await vi.advanceTimersByTimeAsync(10000)

    expect(state.current.value).toBeNull()
    expect(state.available.value).toBe(false)
    expect(statusAPI.getRealtimeConcurrency).toHaveBeenCalledTimes(1)
  })

  it('does not restart polling when visibility changes after unmount', async () => {
    vi.mocked(statusAPI.getRealtimeConcurrency).mockResolvedValue({
      current: 13,
      updated_at: '2026-07-14T12:00:00Z',
    })
    const { wrapper } = mountHarness(ref(true))
    await flushPromises()
    wrapper.unmount()
    vi.mocked(statusAPI.getRealtimeConcurrency).mockClear()

    hidden = true
    document.dispatchEvent(new Event('visibilitychange'))
    hidden = false
    document.dispatchEvent(new Event('visibilitychange'))
    await vi.advanceTimersByTimeAsync(10000)

    expect(statusAPI.getRealtimeConcurrency).not.toHaveBeenCalled()
  })
})
