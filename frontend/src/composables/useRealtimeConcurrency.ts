import { onMounted, onUnmounted, ref, watch, type Ref } from 'vue'
import { statusAPI } from '@/api/status'

export function useRealtimeConcurrency(
  enabled: Readonly<Ref<boolean>>,
  intervalMs = 5000,
) {
  const current = ref<number | null>(null)
  const available = ref(false)
  let timer: ReturnType<typeof setInterval> | null = null
  let mounted = false
  let generation = 0
  let inFlight = false
  let immediateRefreshQueued = false

  function isActive() {
    return mounted && enabled.value && !document.hidden
  }

  async function refresh() {
    if (!isActive() || inFlight) return

    const requestGeneration = generation
    inFlight = true

    try {
      const status = await statusAPI.getRealtimeConcurrency()
      if (!isActive() || requestGeneration !== generation) return
      if (!Number.isInteger(status.current) || status.current < 0) {
        throw new Error('invalid concurrency')
      }
      current.value = status.current
      available.value = true
    } catch {
      if (!isActive() || requestGeneration !== generation) return
      current.value = null
      available.value = false
    } finally {
      inFlight = false
      if (immediateRefreshQueued) {
        immediateRefreshQueued = false
        if (isActive()) void refresh()
      }
    }
  }

  function stopTimer() {
    if (timer !== null) clearInterval(timer)
    timer = null
  }

  function invalidatePendingResult() {
    generation += 1
    immediateRefreshQueued = false
  }

  function requestImmediateRefresh() {
    if (inFlight) {
      immediateRefreshQueued = true
      return
    }
    void refresh()
  }

  function start() {
    stopTimer()
    if (!isActive()) return
    requestImmediateRefresh()
    timer = setInterval(() => void refresh(), intervalMs)
  }

  function onVisibilityChange() {
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

    stopTimer()
    invalidatePendingResult()
    current.value = null
    available.value = false
  })

  onMounted(() => {
    mounted = true
    document.addEventListener('visibilitychange', onVisibilityChange)
    start()
  })

  onUnmounted(() => {
    mounted = false
    stopTimer()
    invalidatePendingResult()
    document.removeEventListener('visibilitychange', onVisibilityChange)
  })

  return { current, available, refresh }
}
