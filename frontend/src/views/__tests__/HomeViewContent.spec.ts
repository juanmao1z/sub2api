import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import enLanding from '@/i18n/locales/en/landing'
import zhLanding from '@/i18n/locales/zh/landing'

const source = readFileSync(resolve(process.cwd(), 'src/views/HomeView.vue'), 'utf8')

describe('HomeView default content', () => {
  it('replaces the provider section and footer with the realtime status after the feature grid', () => {
    expect(source).not.toContain('home.providers.')
    expect(source).not.toContain('<footer')
    expect(source).not.toContain('githubUrl')
    expect(source).not.toContain('currentYear')
    expect(source).toContain('<RealtimeConcurrencyStatus')
    expect(source).toContain('useRealtimeConcurrency')
    expect(source).toMatch(
      /t\('home\.features\.balanceQuotaDesc'\)[\s\S]*?<\/p>\s*<\/div>\s*<\/div>\s*<RealtimeConcurrencyStatus/,
    )
  })

  it('keeps polling disabled until settings resolve to the default home', () => {
    expect(source).toContain(
      'computed(() => appStore.publicSettingsLoaded && !homeContent.value)',
    )
    expect(source).toContain('useRealtimeConcurrency(showDefaultHome, 5000)')
  })

  it('defines the exact realtime status copy in both locales', () => {
    expect(zhLanding.home.realtimeConcurrency).toEqual({
      title: '实时 API 并发',
      refreshHint: '每 5 秒更新',
    })
    expect(enLanding.home.realtimeConcurrency).toEqual({
      title: 'Live API concurrency',
      refreshHint: 'Updates every 5 seconds',
    })
  })
})
