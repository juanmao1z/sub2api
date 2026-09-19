import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import type { HealthState, MonitorMatrixBucket, MonitorMatrixRow } from '@/api/channelMonitorV2'
import ChannelStatusCards from '../ChannelStatusCards.vue'

vi.mock('vue-i18n', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue-i18n')>()),
  useI18n: () => ({ t: (key: string) => key }),
}))

/** @brief Build a user-facing bucket with redacted volume and a retained health state. */
function bucket(state: HealthState, errorRate: number, index: number): MonitorMatrixBucket {
  return {
    bucket_start: `2026-09-19T00:${String(index).padStart(2, '0')}:00Z`,
    metrics: { request_count: 0, error_rate: errorRate },
    health: { overall: state, error_rate: state, ttft: state, minimum_sample: 1 },
  } as MonitorMatrixBucket
}

describe('ChannelStatusCards', () => {
  it('colors redacted history by health and reserves the short gray bar for unknown intervals', () => {
    const row = {
      platform: 'openai',
      group_name: 'stable',
      metrics: { request_count: 0, cache_rate: 0.8, error_rate: 0.1, ttft: { p50_ms: 2000 } },
      health: { overall: 'healthy' },
      buckets: [bucket('healthy', 0, 0), bucket('warning', 0.5, 1), bucket('critical', 0.9, 2), bucket('unknown', 0, 3)],
    } as MonitorMatrixRow
    const wrapper = mount(ChannelStatusCards, {
      props: { rows: [row], healthMode: 'overall', showThroughput: false },
    })

    const bars = wrapper.findAll('.channel-history > div')
    expect(bars.map(bar => bar.classes().find(name => name.startsWith('health-')))).toEqual([
      'health-healthy', 'health-warning', 'health-critical', 'health-unknown',
    ])
    expect(bars.map(bar => bar.attributes('style'))).toEqual([
      'height: 100%;', 'height: 50%;', 'height: 25%;', 'height: 18%;',
    ])
    expect(wrapper.find('.channel-throughput').exists()).toBe(false)
  })
})
