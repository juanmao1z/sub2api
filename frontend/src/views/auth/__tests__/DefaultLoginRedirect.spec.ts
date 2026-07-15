import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const sourceFiles = [
  'src/views/auth/LoginView.vue',
  'src/components/auth/EmailOAuthButtons.vue',
  'src/components/auth/LinuxDoOAuthSection.vue',
  'src/components/auth/DingTalkOAuthSection.vue',
  'src/components/auth/OidcOAuthSection.vue',
  'src/components/auth/WechatOAuthSection.vue',
  'src/views/auth/OAuthCallbackView.vue',
  'src/views/auth/LinuxDoCallbackView.vue',
  'src/views/auth/DingTalkCallbackView.vue',
  'src/views/auth/DingTalkEmailCompletionView.vue',
  'src/views/auth/OidcCallbackView.vue',
  'src/views/auth/WechatCallbackView.vue',
]

describe('default login redirect', () => {
  it('uses /home for every login entry and callback fallback', () => {
    for (const file of sourceFiles) {
      const source = readFileSync(resolve(process.cwd(), file), 'utf8')
      expect(source, file).not.toContain("return '/dashboard'")
      expect(source, file).not.toContain("|| '/dashboard'")
      expect(source, file).not.toContain("ref('/dashboard')")
      expect(source, file).toContain("'/home'")
    }
  })
})
