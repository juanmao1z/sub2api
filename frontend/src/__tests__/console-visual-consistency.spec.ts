import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const sourceRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const theme = readFileSync(resolve(sourceRoot, 'styles/console-theme.css'), 'utf8')
const pages = readFileSync(resolve(sourceRoot, 'styles/console-pages.css'), 'utf8')
const payment = readFileSync(resolve(sourceRoot, 'views/user/PaymentView.vue'), 'utf8')

describe('console visual consistency', () => {
  it('aligns the sidebar brand row with the 56px top bar', () => {
    expect(theme).toMatch(/\.console-signal \.sidebar-header \{[\s\S]*?height: 56px;/)
    expect(theme).toMatch(/\.console-signal \.header-inner \{[\s\S]*?height: 56px;/)
  })

  it('uses the same surface and border for text and select filters', () => {
    expect(theme).toContain(':is(.input,.select-trigger) {')
    expect(theme).toContain('background: var(--signal-inset);')
    expect(theme).toContain('border-color: var(--signal-control-line);')
  })

  it('uses the usage-cost amber accent for both payment confirmation buttons', () => {
    expect(payment.match(/signal-payment-submit/g)).toHaveLength(2)
    expect(pages).toMatch(/\.signal-payment-submit \{[\s\S]*?background: var\(--signal-amber\);/)
  })
})
