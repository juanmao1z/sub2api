import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent, h, ref, type Ref } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { statusAPI, type HomepageStatus } from '@/api/status'
import { useHomepageStatus } from '@/composables/useHomepageStatus'

vi.mock('@/api/status', () => ({
  statusAPI: { getHomepageStatus: vi.fn() },
}))

let hidden = false

const validStatus: HomepageStatus = {
  started_at: '2026-05-14T19:50:12.557602Z',
  active_users_1h: 10,
  success_rate_today: 0.9987,
  total_tokens: 18_043_746_148,
  updated_at: '2026-07-15T12:00:00Z',
}

function deferred<T>() {
  let resolve!: (value: T | PromiseLike<T>) => void
  const promise = new Promise<T>((resolvePromise) => {
    resolve = resolvePromise
  })
  return { promise, resolve }
}

function mountHarness(enabled: Ref<boolean>, intervalMs = 60_000) {
  let state!: ReturnType<typeof useHomepageStatus>
  const wrapper = mount(defineComponent({
    setup() {
      state = useHomepageStatus(enabled, intervalMs)
      return () => h('div')
    },
  }))
  return { wrapper, state }
}

describe('useHomepageStatus', () => {
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

  it('loads immediately and polls every 60 seconds', async () => {
    vi.mocked(statusAPI.getHomepageStatus).mockResolvedValue(validStatus)
    const { wrapper, state } = mountHarness(ref(true))
    await flushPromises()

    expect(statusAPI.getHomepageStatus).toHaveBeenCalledTimes(1)
    expect(state.status.value).toEqual(validStatus)

    await vi.advanceTimersByTimeAsync(60_000)
    expect(statusAPI.getHomepageStatus).toHaveBeenCalledTimes(2)
    wrapper.unmount()
  })

  it('does not poll for custom home content and starts when enabled', async () => {
    vi.mocked(statusAPI.getHomepageStatus).mockResolvedValue(validStatus)
    const enabled = ref(false)
    const { wrapper, state } = mountHarness(enabled)
    await flushPromises()
    expect(statusAPI.getHomepageStatus).not.toHaveBeenCalled()

    enabled.value = true
    await flushPromises()
    expect(statusAPI.getHomepageStatus).toHaveBeenCalledTimes(1)
    expect(state.status.value).toEqual(validStatus)
    wrapper.unmount()
  })

  it('aborts pending work and stops polling when disabled', async () => {
    const pending = deferred<HomepageStatus>()
    vi.mocked(statusAPI.getHomepageStatus).mockReturnValue(pending.promise)
    const enabled = ref(true)
    const { wrapper, state } = mountHarness(enabled)
    await flushPromises()

    const signal = vi.mocked(statusAPI.getHomepageStatus).mock.calls[0][0] as AbortSignal
    enabled.value = false
    await flushPromises()
    expect(signal.aborted).toBe(true)

    pending.resolve(validStatus)
    await flushPromises()
    await vi.advanceTimersByTimeAsync(120_000)
    expect(state.status.value).toBeNull()
    expect(statusAPI.getHomepageStatus).toHaveBeenCalledTimes(1)
    wrapper.unmount()
  })

  it('pauses and aborts while hidden, then refreshes immediately when visible', async () => {
    const pending = deferred<HomepageStatus>()
    vi.mocked(statusAPI.getHomepageStatus)
      .mockReturnValueOnce(pending.promise)
      .mockResolvedValueOnce(validStatus)
    const { wrapper, state } = mountHarness(ref(true))
    await flushPromises()

    const signal = vi.mocked(statusAPI.getHomepageStatus).mock.calls[0][0] as AbortSignal
    hidden = true
    document.dispatchEvent(new Event('visibilitychange'))
    expect(signal.aborted).toBe(true)

    await vi.advanceTimersByTimeAsync(120_000)
    expect(statusAPI.getHomepageStatus).toHaveBeenCalledTimes(1)

    hidden = false
    document.dispatchEvent(new Event('visibilitychange'))
    await flushPromises()
    expect(statusAPI.getHomepageStatus).toHaveBeenCalledTimes(2)
    expect(state.status.value).toEqual(validStatus)

    pending.resolve({ ...validStatus, active_users_1h: 99 })
    await flushPromises()
    expect(state.status.value).toEqual(validStatus)
    wrapper.unmount()
  })

  it('keeps the last successful status while the page is hidden', async () => {
    vi.mocked(statusAPI.getHomepageStatus).mockResolvedValue(validStatus)
    const { wrapper, state } = mountHarness(ref(true))
    await flushPromises()
    expect(state.status.value).toEqual(validStatus)

    hidden = true
    document.dispatchEvent(new Event('visibilitychange'))
    await vi.advanceTimersByTimeAsync(120_000)

    expect(state.status.value).toEqual(validStatus)
    expect(statusAPI.getHomepageStatus).toHaveBeenCalledTimes(1)
    wrapper.unmount()
  })

  it('aborts pending work and stops polling on unmount', async () => {
    const pending = deferred<HomepageStatus>()
    vi.mocked(statusAPI.getHomepageStatus).mockReturnValue(pending.promise)
    const { wrapper, state } = mountHarness(ref(true))
    await flushPromises()

    const signal = vi.mocked(statusAPI.getHomepageStatus).mock.calls[0][0] as AbortSignal
    wrapper.unmount()
    expect(signal.aborted).toBe(true)

    pending.resolve(validStatus)
    await flushPromises()
    await vi.advanceTimersByTimeAsync(120_000)
    expect(state.status.value).toBeNull()
    expect(statusAPI.getHomepageStatus).toHaveBeenCalledTimes(1)
  })

  it('clears values on failures and retries without raising a toast', async () => {
    vi.mocked(statusAPI.getHomepageStatus)
      .mockRejectedValueOnce(new Error('unavailable'))
      .mockResolvedValueOnce(validStatus)
    const { wrapper, state } = mountHarness(ref(true))
    await flushPromises()
    expect(state.status.value).toBeNull()

    await vi.advanceTimersByTimeAsync(60_000)
    expect(state.status.value).toEqual(validStatus)
    wrapper.unmount()
  })

  it.each([
    { ...validStatus, active_users_1h: -1 },
    { ...validStatus, active_users_1h: 1.5 },
    { ...validStatus, success_rate_today: -0.1 },
    { ...validStatus, success_rate_today: 1.1 },
    { ...validStatus, total_tokens: Number.POSITIVE_INFINITY },
    { ...validStatus, total_tokens: -1 },
    { ...validStatus, started_at: 'not-a-date' },
    { ...validStatus, updated_at: 'not-a-date' },
  ])('rejects an invalid homepage payload', async (payload) => {
    vi.mocked(statusAPI.getHomepageStatus).mockResolvedValue(payload)
    const { wrapper, state } = mountHarness(ref(true))
    await flushPromises()
    expect(state.status.value).toBeNull()
    wrapper.unmount()
  })

  it('accepts empty-data nullable fields and zero values', async () => {
    const emptyStatus: HomepageStatus = {
      started_at: null,
      active_users_1h: 0,
      success_rate_today: null,
      total_tokens: 0,
      updated_at: validStatus.updated_at,
    }
    vi.mocked(statusAPI.getHomepageStatus).mockResolvedValue(emptyStatus)
    const { wrapper, state } = mountHarness(ref(true))
    await flushPromises()
    expect(state.status.value).toEqual(emptyStatus)
    wrapper.unmount()
  })
})
