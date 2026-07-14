import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { describe, expect, it } from 'vitest'
import RealtimeConcurrencyStatus from '../RealtimeConcurrencyStatus.vue'

function mountStatus(current: number | null, available: boolean, locale: 'zh' | 'en') {
  const i18n = createI18n({
    legacy: false,
    locale,
    messages: {
      zh: {
        home: {
          realtimeConcurrency: {
            title: () => '实时 API 并发',
            refreshHint: () => '每 5 秒更新',
          },
        },
      },
      en: {
        home: {
          realtimeConcurrency: {
            title: () => 'Live API concurrency',
            refreshHint: () => 'Updates every 5 seconds',
          },
        },
      },
    },
  })

  return mount(RealtimeConcurrencyStatus, {
    props: { current, available },
    global: { plugins: [i18n] },
  })
}

describe('RealtimeConcurrencyStatus', () => {
  it('renders the available value and active status in Chinese', () => {
    const wrapper = mountStatus(12, true, 'zh')

    expect(wrapper.text()).toContain('实时 API 并发')
    expect(wrapper.text()).toContain('12')
    expect(wrapper.text()).toContain('每 5 秒更新')
    expect(wrapper.get('[data-testid="status-dot"]').classes()).toContain('bg-emerald-500')
  })

  it('renders the unavailable fallback and inactive status in English', () => {
    const wrapper = mountStatus(null, false, 'en')

    expect(wrapper.text()).toContain('Live API concurrency')
    expect(wrapper.text()).toContain('--')
    expect(wrapper.text()).toContain('Updates every 5 seconds')
    expect(wrapper.get('[data-testid="status-dot"]').classes()).not.toContain('bg-emerald-500')
  })

  it.each([
    { name: 'negative', current: -1 },
    { name: 'non-integer', current: 1.5 },
    { name: 'null', current: null },
  ])('treats $name current as unavailable even when available is true', ({ current }) => {
    const wrapper = mountStatus(current, true, 'en')

    expect(wrapper.text()).toContain('--')
    expect(wrapper.get('[data-testid="status-dot"]').classes()).not.toContain('bg-emerald-500')
  })
})
