import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const dir = dirname(fileURLToPath(import.meta.url))
const headerSource = readFileSync(resolve(dir, '../AppHeader.vue'), 'utf8')

describe('AppHeader customization', () => {
  it('does not show the upstream GitHub link in the account menu', () => {
    expect(headerSource).not.toContain('https://github.com/Wei-Shaw/sub2api')
    expect(headerSource).not.toContain("t('nav.github')")
  })
})
