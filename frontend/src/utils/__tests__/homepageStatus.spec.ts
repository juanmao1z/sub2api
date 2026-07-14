import { describe, expect, it } from 'vitest'
import {
  calculateBeijingUptime,
  formatBeijingStartTime,
  formatCompactTokens,
  formatSuccessRate,
} from '@/utils/homepageStatus'

describe('homepage status formatting', () => {
  it('calculates calendar uptime in Beijing time', () => {
    expect(calculateBeijingUptime(
      '2026-05-14T19:50:12Z',
      new Date('2026-07-15T20:51:13Z'),
    )).toEqual({
      years: 0,
      months: 2,
      days: 1,
      hours: 1,
      minutes: 1,
      seconds: 1,
    })
  })

  it('clamps future start times to zero', () => {
    expect(calculateBeijingUptime(
      '2026-07-16T00:00:00Z',
      new Date('2026-07-15T00:00:00Z'),
    )).toEqual({
      years: 0,
      months: 0,
      days: 0,
      hours: 0,
      minutes: 0,
      seconds: 0,
    })
  })

  it('formats the source timestamp in Beijing time', () => {
    expect(formatBeijingStartTime('2026-05-14T19:50:12Z', 'zh-CN'))
      .toBe('2026-05-15 03:50:12')
  })

  it('formats tokens compactly and success as a one-decimal percentage', () => {
    expect(formatCompactTokens(18_043_746_148, 'en-US')).toBe('18.0B')
    expect(formatCompactTokens(850_400_000, 'zh-CN')).toBe('850.4M')
    expect(formatSuccessRate(0.9987, 'zh-CN')).toBe('99.9%')
  })

  it('returns placeholders when aggregate values are absent', () => {
    expect(formatCompactTokens(null, 'zh-CN')).toBe('--')
    expect(formatSuccessRate(null, 'zh-CN')).toBe('--')
  })
})
