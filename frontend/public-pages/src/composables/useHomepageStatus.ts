import { onMounted, onUnmounted, ref, watch, type Ref } from 'vue'
import { statusAPI, type HomepageStatus } from '@/api/status'

function isValidDate(value: string): boolean {
  return value.trim().length > 0 && Number.isFinite(Date.parse(value))
}

function isValidStatus(status: HomepageStatus): boolean {
  return (
    (status.started_at === null || isValidDate(status.started_at))
    && Number.isInteger(status.active_users_1h)
    && status.active_users_1h >= 0
    && (
      status.success_rate_today === null
      || (
        Number.isFinite(status.success_rate_today)
        && status.success_rate_today >= 0
        && status.success_rate_today <= 1
      )
    )
    && Number.isInteger(status.total_tokens)
    && status.total_tokens >= 0
    && isValidDate(status.updated_at)
  )
}

export function useHomepageStatus(
  enabled: Readonly<Ref<boolean>>,
  intervalMs = 60_000,
) {
  const status = ref<HomepageStatus | null>(null)
  let timer: ReturnType<typeof setInterval> | null = null
  let mounted = false
  let generation = 0
  let activeRequest: {
    generation: number
    controller: AbortController
  } | null = null

  function isActive(): boolean {
    return mounted && enabled.value && !document.hidden
  }

  async function refresh(): Promise<void> {
    if (!isActive() || activeRequest !== null) return

    const request = {
      generation,
      controller: new AbortController(),
    }
    activeRequest = request

    try {
      const nextStatus = await statusAPI.getHomepageStatus(request.controller.signal)
      if (!isActive() || request.generation !== generation || activeRequest !== request) return
      if (!isValidStatus(nextStatus)) throw new Error('invalid homepage status')
      status.value = nextStatus
    } catch {
      if (!isActive() || request.generation !== generation || activeRequest !== request) return
      status.value = null
    } finally {
      if (activeRequest === request) activeRequest = null
    }
  }

  function stopTimer(): void {
    if (timer !== null) clearInterval(timer)
    timer = null
  }

  function invalidatePendingResult(): void {
    generation += 1
    const request = activeRequest
    activeRequest = null
    request?.controller.abort()
  }

  function start(): void {
    stopTimer()
    if (!isActive()) return
    void refresh()
    timer = setInterval(() => void refresh(), intervalMs)
  }

  function stop(): void {
    stopTimer()
    invalidatePendingResult()
    status.value = null
  }

  function onVisibilityChange(): void {
    if (document.hidden) {
      stopTimer()
      invalidatePendingResult()
      return
    }
    start()
  }

  watch(enabled, (value) => {
    if (!mounted) return
    if (value) {
      start()
      return
    }
    stop()
  })

  onMounted(() => {
    mounted = true
    document.addEventListener('visibilitychange', onVisibilityChange)
    start()
  })

  onUnmounted(() => {
    mounted = false
    stop()
    document.removeEventListener('visibilitychange', onVisibilityChange)
  })

  return { status, refresh }
}
