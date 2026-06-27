import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import ExternalRechargeView from '../ExternalRechargeView.vue'

const messages: Record<string, string> = {
  'recharge.title': '点击充值-右上角新窗口',
  'recharge.openInNewWindow': '新窗口打开',
  'recharge.iframeTitle': '充值中心'
}

vi.mock('vue-i18n', () => {
  return {
    useI18n: () => ({
      t: (key: string) => messages[key] ?? key
    })
  }
})

vi.mock('@/components/layout/AppLayout.vue', () => {
  return {
    default: {
      name: 'AppLayout',
      template: '<section data-testid="app-layout"><slot /></section>'
    }
  }
})

function mountView() {
  return mount(ExternalRechargeView)
}

describe('ExternalRechargeView', () => {
  it('renders the payment shop as a large embedded site inside the app shell', () => {
    const wrapper = mountView()

    expect(wrapper.get('[data-testid="app-layout"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('点击充值-右上角新窗口')

    const frameShell = wrapper.get('[data-testid="recharge-frame-shell"]')
    expect(frameShell.classes()).toContain('rounded-2xl')
    expect(frameShell.classes()).toContain('overflow-hidden')

    const iframe = wrapper.get('iframe')
    expect(iframe.attributes('src')).toBe('https://pay.ldxp.cn/shop/1WGCPCG0')
    expect(iframe.attributes('title')).toBe('充值中心')
    expect(iframe.classes()).toContain('min-h-[760px]')

    const openButton = wrapper.get('[data-testid="recharge-open-window"]')
    expect(openButton.classes()).toContain('absolute')
    expect(openButton.classes()).toContain('right-3')
    expect(openButton.classes()).toContain('top-3')
  })

  it('opens the payment shop in a new window', async () => {
    const openSpy = vi.spyOn(window, 'open').mockImplementation(() => null)

    const wrapper = mountView()
    await wrapper.get('[data-testid="recharge-open-window"]').trigger('click')

    expect(openSpy).toHaveBeenCalledWith(
      'https://pay.ldxp.cn/shop/1WGCPCG0',
      '_blank',
      'noopener,noreferrer'
    )

    openSpy.mockRestore()
  })
})
