import fs from 'node:fs/promises'
import { transformWithEsbuild } from 'vite'
import { baseCompile } from '@intlify/message-compiler'

// Compile trusted repository locale modules during the build, never in the browser.
function compileMessages(value, filename) {
  if (typeof value === 'string') {
    return baseCompile(value, {
      mode: 'arrow',
      filename,
      onError(error) { throw error },
    }).code
  }
  if (Array.isArray(value)) return `[${value.map(item => compileMessages(item, filename)).join(',')}]`
  if (value && typeof value === 'object') {
    return `{${Object.entries(value).map(([key, item]) => `${JSON.stringify(key)}:${compileMessages(item, filename)}`).join(',')}}`
  }
  throw new TypeError(`Unsupported locale value in ${filename}`)
}

export function precompilePublicMessages(localeRoot) {
  return {
    name: 'precompile-public-messages',
    enforce: 'pre',
    async load(id) {
      if (!id.startsWith(localeRoot) || !id.endsWith('.ts')) return null
      const source = await fs.readFile(id, 'utf8')
      const { code } = await transformWithEsbuild(source, id, { loader: 'ts', format: 'esm' })
      const { default: messages } = await import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}`)
      return { code: `export default ${compileMessages(messages, id)}`, map: null }
    },
  }
}
