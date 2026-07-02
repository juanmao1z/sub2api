import { mount } from '@vue/test-utils'
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
  })

  it('keeps the new-window shortcut for other custom iframe pages', () => {
    const wrapper = mountCustomPage('docs', [
      menuItem({ id: 'docs', label: '文档', url: 'https://docs.example.com/' }),
    ])

    expect(wrapper.get('.custom-open-fab').text()).toContain('新窗口打开')
    expect(wrapper.get('.custom-open-fab').attributes('href')).toBe('https://docs.example.com/')
  })
})
