import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import enLanding from '@/i18n/locales/en/landing'
import zhLanding from '@/i18n/locales/zh/landing'

const source = readFileSync(resolve(process.cwd(), 'src/views/HomeView.vue'), 'utf8')

describe('HomeView default content', () => {
  it('uses the status-first brand layout and removes the old marketing surface', () => {
    expect(source).toContain('home.status.stable')
    expect(source).toContain('{{ siteName }}')
    expect(source).toContain('useHomepageStatus(showDefaultHome, 60_000)')
    expect(source).toContain("const redeemUrl = 'https://pay.ldxp.cn/shop/1WGCPCG0'")
    expect(source).toContain(
      "const leaderboardUrl = 'https://api.zhouz.online/custom/usage-leaderboard'",
    )
    expect(source).toContain("src=\"/community-placeholder.jpg?v=20260731-img3842\"")
    expect(source).toContain('max-h-[calc(100vh-6.5rem)]')
    expect(source).toContain('object-contain')
    expect(source).not.toContain('aspect-square w-full rounded-md object-cover')
    expect(source).toContain("{{ t('home.navigation.community') }}")
    expect(source).toContain('aria-haspopup="dialog"')
    expect(source).toContain('@click.self="closeCommunity"')
    expect(source).toContain('font-[720]')
    expect(source).not.toContain('siteSubtitle')
    expect(source).not.toContain('RealtimeConcurrencyStatus')
    expect(source).not.toContain('useRealtimeConcurrency')
    expect(source).not.toContain('terminal-window')
    expect(source).not.toContain('home.features.')
    expect(source).not.toContain('home.tags.')
    expect(source).not.toContain('home.providers.')
  })

  it('keeps polling disabled until settings resolve to the default home', () => {
    expect(source).toContain(
      'computed(() => appStore.publicSettingsLoaded && !homeContent.value)',
    )
    expect(source).toContain('useHomepageStatus(showDefaultHome, 60_000)')
  })

  it('keeps custom URL and HTML home modes intact', () => {
    expect(source).toContain('v-if="homeContent"')
    expect(source).toContain('v-if="isHomeContentUrl"')
    expect(source).toContain(':src="homeContent.trim()"')
    expect(source).toContain('v-else v-html="homeContent"')
  })

  it('defines the homepage navigation and status copy in both locales', () => {
    expect(zhLanding.home.navigation).toEqual({
      home: '首页',
      dashboard: '控制台',
      redeem: '兑换码',
      leaderboard: '排行榜',
      community: '社群',
      openMenu: '打开导航菜单',
      closeMenu: '关闭导航菜单',
    })
    expect(enLanding.home.navigation).toEqual({
      home: 'Home',
      dashboard: 'Dashboard',
      redeem: 'Redeem',
      leaderboard: 'Leaderboard',
      community: 'Community',
      openMenu: 'Open navigation menu',
      closeMenu: 'Close navigation menu',
    })
    expect(zhLanding.home.status).toMatchObject({
      stable: '本站已稳定运行',
      activeUsers: '近 1 小时活跃用户',
      successRate: '今日成功率',
      totalTokens: '累计处理 Token',
    })
    expect(enLanding.home.status).toMatchObject({
      stable: 'Service uptime',
      activeUsers: 'Active users (1h)',
      successRate: 'Success rate today',
      totalTokens: 'Total tokens processed',
    })
  })
})
