export type UiFusionScope = 'console' | 'public' | 'controls' | 'partial' | 'deferred' | 'preserved'

const userPages = new Set([
  '/dashboard', '/keys', '/usage', '/monitor', '/available-channels',
  '/orders', '/redeem', '/affiliate', '/profile', '/subscriptions',
])
const adminPages = new Set([
  '/admin/dashboard', '/admin/ops', '/admin/users', '/admin/groups',
  '/admin/subscriptions', '/admin/accounts', '/admin/channels/pricing',
  '/admin/channels/monitor', '/admin/proxies', '/admin/redeem',
  '/admin/promo-codes', '/admin/announcements', '/admin/usage',
  '/admin/audit-logs', '/admin/risk-control', '/admin/prompt-audit',
  '/admin/plugins', '/admin/settings', '/admin/orders/dashboard',
  '/admin/orders', '/admin/orders/plans', '/admin/affiliates/invites',
  '/admin/affiliates/rebates', '/admin/affiliates/transfers',
])

/** Presentation only: never use this classification for authorization. */
export function getUiFusionScope(path: string): UiFusionScope {
  if (userPages.has(path) || adminPages.has(path) || path.startsWith('/custom/')) return 'console'
  if (path === '/model-plaza' || path === '/key-usage' || path.startsWith('/legal/')) return 'public'
  if (['/forgot-password', '/reset-password', '/email-verify'].includes(path)) return 'controls'
  if (path === '/purchase') return 'partial'
  if (path === '/batch-image' || path === '/recharge') return 'deferred'
  return 'preserved'
}

/** Legacy styles stay enabled for pages explicitly excluded from the fusion. */
export function usesLegacyPageStyles(scope: UiFusionScope): boolean {
  return ['preserved', 'partial', 'deferred', 'controls'].includes(scope)
}
