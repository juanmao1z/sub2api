import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { reactive, nextTick } from 'vue'
import CustomPageView from '../CustomPageView.vue'

const route = reactive({ params: { id: 'old' } })
const { getPage } = vi.hoisted(() => ({ getPage: vi.fn() }))
vi.mock('vue-router', () => ({ useRoute: () => route }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key, locale: { value: 'en' } }) }))
vi.mock('@/stores', () => ({ useAppStore: () => ({
  publicSettingsLoaded: true,
  cachedPublicSettings: { custom_menu_items: [
    { id: 'old', url: 'md:old' }, { id: 'new', url: 'md:new' },
    { id: 'external', url: 'https://example.com/docs' },
    { id: 'guide', url: 'md:cc-switch-codex' }
  ] }
}) }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ isAdmin: false, user: { id: 1 }, token: 'test' }) }))
vi.mock('@/stores/adminSettings', () => ({ useAdminSettingsStore: () => ({ customMenuItems: [] }) }))
vi.mock('@/api/client', () => ({ apiClient: { get: getPage }, buildApiUrl: (path: string) => `/api/v1${path}` }))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<div><slot /></div>' } }))
enableAutoUnmount(afterEach)

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (error: Error) => void
  const promise = new Promise<T>((yes, no) => { resolve = yes; reject = no })
  return { promise, resolve, reject }
}
const response = (markdown: string) => ({ data: markdown })
const mountPage = () => mount(CustomPageView, { global: { stubs: { Icon: true } } })

beforeEach(() => {
  route.params.id = 'old'
  getPage.mockReset()
  vi.stubGlobal('ResizeObserver', class { observe() {} unobserve() {} disconnect() {} })
})
afterEach(() => vi.unstubAllGlobals())

describe('custom Markdown page request ordering', () => {
  it.each(['success', 'error'])('ignores an old %s after switching pages', async (outcome) => {
    const old = deferred<ReturnType<typeof response>>()
    getPage.mockReturnValueOnce(old.promise).mockResolvedValueOnce(response('## New page'))
    const wrapper = mountPage()
    route.params.id = 'new'
    await flushPromises()
    expect(wrapper.get('.markdown-page-content h2').text()).toBe('New page')
    if (outcome === 'error') old.reject(new Error('offline'))
    else old.resolve(response('## Old page'))
    await flushPromises()
    expect(wrapper.get('.markdown-page-content h2').text()).toBe('New page')
    expect(wrapper.get('.toc-item').text()).toBe('New page')
  })

  it('keeps the current loading state when the old request completes', async () => {
    const old = deferred<ReturnType<typeof response>>()
    const current = deferred<ReturnType<typeof response>>()
    getPage.mockReturnValueOnce(old.promise).mockReturnValueOnce(current.promise)
    const wrapper = mountPage()
    route.params.id = 'new'
    await nextTick()
    old.resolve(response('## Old page'))
    await flushPromises()
    expect(wrapper.find('.custom-page-loading-spinner').exists()).toBe(true)
    expect(wrapper.find('.markdown-page-content').exists()).toBe(false)
    current.resolve(response('## New page'))
    await flushPromises()
    expect(wrapper.get('.markdown-page-content h2').text()).toBe('New page')
  })

  it('shows an embedded page immediately after leaving a pending Markdown page', async () => {
    const old = deferred<ReturnType<typeof response>>()
    getPage.mockReturnValueOnce(old.promise)
    const wrapper = mountPage()
    route.params.id = 'external'
    await nextTick()
    expect(wrapper.get('iframe').attributes('src')).toContain('https://example.com/docs')
    old.resolve(response('## Old page'))
    await flushPromises()
    expect(wrapper.find('.markdown-page-content').exists()).toBe(false)
    expect(wrapper.get('iframe').attributes('src')).toContain('https://example.com/docs')
  })

  it('preserves the built-in guide after a stale remote request completes', async () => {
    const old = deferred<ReturnType<typeof response>>()
    getPage.mockReturnValueOnce(old.promise)
    const wrapper = mountPage()
    route.params.id = 'guide'
    await flushPromises()
    const guide = wrapper.get('.markdown-page-content').text()
    expect(guide.length).toBeGreaterThan(100)
    expect(wrapper.findAll('.guide-tab')).toHaveLength(4)
    old.resolve(response('## Old page'))
    await flushPromises()
    expect(wrapper.get('.markdown-page-content').text()).toBe(guide)
    expect(getPage).toHaveBeenCalledTimes(1)
  })

  it('ignores requests completed after unmounting', async () => {
    const old = deferred<ReturnType<typeof response>>()
    getPage.mockReturnValueOnce(old.promise)
    const wrapper = mountPage()
    const copyButtons = vi.spyOn(document, 'createElement')
    wrapper.unmount()
    const calls = copyButtons.mock.calls.length
    old.resolve(response('## Old page\n```sh\necho old\n```'))
    await flushPromises()
    expect(copyButtons.mock.calls.length).toBe(calls)
    copyButtons.mockRestore()
  })
})
