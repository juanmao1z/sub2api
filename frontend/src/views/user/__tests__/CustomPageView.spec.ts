import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import CustomPageView from '../CustomPageView.vue'
import type { CustomMenuItem, PublicSettings, User } from '@/types'

const {
  routeState,
  appStoreState,
  authStoreState,
  adminSettingsStoreState,
} = vi.hoisted(() => {
  const publicSettings: Partial<PublicSettings> = {
    custom_menu_items: [],
  }
  return {
    routeState: {
      params: {
        id: '',
      },
    },
    appStoreState: {
      publicSettingsLoaded: true,
      cachedPublicSettings: publicSettings as PublicSettings,
      fetchPublicSettings: vi.fn(),
    },
    authStoreState: {
      isAdmin: false,
      user: { id: 42 } as User,
      token: 'auth-token',
    },
    adminSettingsStoreState: {
      customMenuItems: [] as CustomMenuItem[],
    },
  }
})

vi.mock('vue-router', () => ({
  useRoute: () => routeState,
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      locale: { value: 'zh' },
      t: (key: string) => ({
        'customPage.openInNewTab': '新窗口打开',
      })[key] ?? key,
    }),
  }
})

vi.mock('@/stores', () => ({
  useAppStore: () => appStoreState,
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => authStoreState,
}))

vi.mock('@/stores/adminSettings', () => ({
  useAdminSettingsStore: () => adminSettingsStoreState,
}))

vi.mock('@/components/layout/AppLayout.vue', () => ({
  default: {
    name: 'AppLayout',
    template: '<section data-testid="app-layout"><slot /></section>',
  },
}))

vi.mock('@/components/icons/Icon.vue', () => ({
  default: {
    name: 'Icon',
    template: '<span data-testid="icon" />',
  },
}))

vi.mock('@/utils/embedded-url', () => ({
  buildEmbeddedUrl: (url: string) => url,
  detectTheme: () => 'light',
}))

function menuItem(overrides: Partial<CustomMenuItem>): CustomMenuItem {
  return {
    id: 'custom-page',
    label: '自定义页面',
    icon_svg: '',
    url: 'https://api.zhouz.online/leaderboard/',
    visibility: 'user',
    sort_order: 1,
    ...overrides,
  }
}

function mountCustomPage(id: string, items: CustomMenuItem[]) {
  routeState.params.id = id
  appStoreState.cachedPublicSettings = {
    custom_menu_items: items,
  } as PublicSettings
  return mount(CustomPageView)
}

describe('CustomPageView', () => {
  beforeEach(() => {
    appStoreState.fetchPublicSettings.mockClear()
    authStoreState.isAdmin = false
    authStoreState.user = { id: 42 } as User
    authStoreState.token = 'auth-token'
    adminSettingsStoreState.customMenuItems = []
  })

  it('hides the new-window shortcut on the embedded usage leaderboard page', () => {
    const wrapper = mountCustomPage('usage-leaderboard', [
      menuItem({ id: 'usage-leaderboard', label: '使用排行榜' }),
    ])

    expect(wrapper.find('.custom-open-fab').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('新窗口打开')
    expect(wrapper.get('iframe').attributes('src')).toBe('https://api.zhouz.online/leaderboard/')
    expect(wrapper.get('.leaderboard-embed-host').classes()).not.toContain('card')
    expect(wrapper.get('.custom-embed-shell').classes()).toContain('custom-embed-shell-leaderboard')
  })

  it('keeps the new-window shortcut for other custom iframe pages', () => {
    const wrapper = mountCustomPage('docs', [
      menuItem({ id: 'docs', label: '文档', url: 'https://docs.example.com/' }),
    ])

    expect(wrapper.get('.custom-open-fab').text()).toContain('新窗口打开')
    expect(wrapper.get('.custom-open-fab').attributes('href')).toBe('https://docs.example.com/')
    expect(wrapper.get('.custom-page-layout > div').classes()).toContain('card')
    expect(wrapper.get('.custom-embed-shell').classes()).not.toContain('custom-embed-shell-leaderboard')
  })

  it('switches between the built-in guides and rebuilds their contents', async () => {
    const wrapper = mountCustomPage('cc-switch-guide', [
      menuItem({
        id: 'cc-switch-guide',
        label: '使用说明',
        url: 'md:cc-switch-codex',
      }),
    ])

    await flushPromises()

    expect(wrapper.find('iframe').exists()).toBe(false)
    expect(wrapper.get('.guide-reader')).toBeTruthy()
    expect(wrapper.get('.markdown-page-content h1').text()).toBe('CC Switch 配置 Codex')
    expect(wrapper.get('.markdown-page-content').text()).toContain('https://api.zhouz.online/v1')

    const tabs = wrapper.findAll('[role="tab"]')
    expect(tabs.map((tab) => tab.text())).toEqual(['CC Switch 配置 Codex', '网站使用说明', '会话与配置恢复', 'API 错误与网络排查'])
    expect(tabs[0].attributes('aria-selected')).toBe('true')
    expect(tabs[1].attributes('aria-selected')).toBe('false')

    let desktopTocItems = wrapper.findAll('.toc-sidebar .toc-item')
    expect(desktopTocItems.length).toBeGreaterThan(3)
    expect(desktopTocItems.some((item) => item.text() === '开始前的准备')).toBe(true)
    expect(desktopTocItems.some((item) => item.text() === 'CC Switch 配置 Codex')).toBe(false)

    expect(wrapper.get('.guide-mobile-toc summary').text()).toBe('customPage.tableOfContents')

    await tabs[1].trigger('click')
    await flushPromises()

    expect(tabs[0].attributes('aria-selected')).toBe('false')
    expect(tabs[1].attributes('aria-selected')).toBe('true')
    expect(wrapper.get('.markdown-page-content h1').text()).toBe('网站使用说明')
    expect(wrapper.get('.markdown-page-content').text()).toContain('https://api.zhouz.online/v1')
    expect(wrapper.get('.markdown-page-content').text()).toContain('分组名称、平台、倍率和模型权限以当前页面显示为准')
    expect(wrapper.get('.markdown-page-content').text()).toContain('不要继续使用旧教程中的分组名称')
    expect(wrapper.get('.markdown-page-content').text()).not.toContain('当前能力')
    expect(wrapper.get('.markdown-page-content').text()).toContain('实际费用 = 模型基础费用 × 当前生效倍率')

    await tabs[2].trigger('click')
    await flushPromises()

    expect(tabs[1].attributes('aria-selected')).toBe('false')
    expect(tabs[2].attributes('aria-selected')).toBe('true')
    expect(wrapper.get('.markdown-page-content h1').text()).toBe('会话与配置恢复')
    expect(wrapper.get('.markdown-page-content').text()).toContain('不会保存在服务器')
    expect(wrapper.get('.markdown-page-content').text()).toContain('codex++')
    expect(wrapper.get('.markdown-page-content').text()).toContain('~/.codex/sqlite/*.db')

    desktopTocItems = wrapper.findAll('.toc-sidebar .toc-item')
    expect(desktopTocItems.some((item) => item.text() === '如何恢复记录')).toBe(true)
    expect(desktopTocItems.some((item) => item.text() === '配置恢复')).toBe(true)
    expect(desktopTocItems.some((item) => item.text() === '计费说明')).toBe(false)

    await tabs[3].trigger('click')
    await flushPromises()

    expect(tabs[2].attributes('aria-selected')).toBe('false')
    expect(tabs[3].attributes('aria-selected')).toBe('true')
    expect(wrapper.get('.markdown-page-content h1').text()).toBe('API 错误与网络排查')
    expect(wrapper.get('.markdown-page-content').text()).toContain('401 Unauthorized')
    expect(wrapper.get('.markdown-page-content').text()).toContain('429 Too Many Requests')
    expect(wrapper.get('.markdown-page-content').text()).toContain('https://api.zhouz.online/v1')
  })
})
