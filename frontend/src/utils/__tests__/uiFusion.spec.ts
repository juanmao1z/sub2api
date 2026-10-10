import { describe, expect, it } from 'vitest'
import { getUiFusionScope, usesLegacyPageStyles } from '../uiFusion'

describe('approved presentation scope', () => {
  it.each(['/dashboard', '/keys', '/usage', '/monitor', '/available-channels', '/admin/settings', '/admin/accounts', '/custom/example'])('enables source UI on %s', path => {
    expect(getUiFusionScope(path)).toBe('console')
    expect(usesLegacyPageStyles(getUiFusionScope(path))).toBe(false)
  })
  it.each(['/home', '/login', '/register', '/payment/qrcode', '/payment/result', '/payment/stripe', '/payment/airwallex', '/payment/stripe-popup', '/setup', '/auth/callback'])('preserves the legacy cascade on %s', path => {
    expect(getUiFusionScope(path)).toBe('preserved')
    expect(usesLegacyPageStyles(getUiFusionScope(path))).toBe(true)
  })
  it('keeps partial and deferred pages independent', () => {
    expect(getUiFusionScope('/purchase')).toBe('partial')
    expect(getUiFusionScope('/recharge')).toBe('deferred')
    expect(getUiFusionScope('/batch-image')).toBe('deferred')
    for (const path of ['/purchase', '/recharge', '/batch-image']) expect(usesLegacyPageStyles(getUiFusionScope(path))).toBe(true)
  })
  it('only permits local controls on security pages', () => {
    for (const path of ['/forgot-password', '/reset-password', '/email-verify']) {
      expect(getUiFusionScope(path)).toBe('controls')
      expect(usesLegacyPageStyles(getUiFusionScope(path))).toBe(true)
    }
  })
  it('does not grant UI scope to deferred or unknown functionality', () => {
    for (const path of ['/tickets', '/admin/tickets', '/infinite-canvas', '/admin/unknown']) expect(getUiFusionScope(path)).toBe('preserved')
  })
})
