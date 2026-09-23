import type { GroupPlatform } from '@/types'

export const OPENAI_CC_SWITCH_CODEX_MODEL = 'gpt-5.5'
export const GROK_CC_SWITCH_MODEL = 'grok-4.5'

export type CcSwitchClientType = 'claude' | 'gemini'

export interface CcSwitchImportConfig {
  app: string
  endpoint: string
  model?: string
}

export interface CcSwitchImportDeeplinkInput {
  baseUrl: string
  platform?: GroupPlatform | null
  clientType: CcSwitchClientType
  providerName: string
  apiKey: string
  usageScript: string
}

/**
 * @brief Removes a trailing `/v1` path from a CCS base URL.
 * @param baseUrl The configured API base URL.
 * @return The URL without trailing slashes or a trailing `/v1` segment.
 */
function withoutV1Suffix(baseUrl: string): string {
  return baseUrl.replace(/\/+$/, '').replace(/\/v1$/i, '')
}

/**
 * @brief Ensures an OpenAI-compatible endpoint has exactly one `/v1` suffix.
 * @param baseUrl The configured API base URL.
 * @return The normalized URL ending in `/v1`.
 */
function withV1Endpoint(baseUrl: string): string {
  const normalizedBaseUrl = baseUrl.replace(/\/+$/, '')
  return normalizedBaseUrl.endsWith('/v1') ? normalizedBaseUrl : `${normalizedBaseUrl}/v1`
}

/**
 * @brief Resolves the CCS provider configuration for a platform.
 * @param platform The API group platform.
 * @param clientType The CCS client receiving the import.
 * @param baseUrl The API base URL to expose to CCS.
 * @return The provider app, endpoint, and optional model configuration.
 */
export function resolveCcSwitchImportConfig(
  platform: GroupPlatform | undefined | null,
  clientType: CcSwitchClientType,
  baseUrl: string
): CcSwitchImportConfig {
  switch (platform || 'anthropic') {
    case 'antigravity':
      return {
        app: clientType === 'gemini' ? 'gemini' : 'claude',
        endpoint: `${baseUrl.replace(/\/+$/, '')}/antigravity`
      }
    case 'openai':
      return {
        app: 'codex',
        endpoint: withV1Endpoint(baseUrl),
        model: OPENAI_CC_SWITCH_CODEX_MODEL
      }
    case 'gemini':
      return {
        app: 'gemini',
        endpoint: baseUrl
      }
    case 'grok':
      return {
        app: 'grokbuild',
        endpoint: withoutV1Suffix(baseUrl),
        model: GROK_CC_SWITCH_MODEL
      }
    default:
      return {
        app: 'claude',
        endpoint: baseUrl
      }
  }
}

/**
 * @brief Builds a CCS provider import deeplink.
 * @param input Provider details and the API base URL to import.
 * @return A `ccswitch://` deeplink containing the provider configuration.
 */
export function buildCcSwitchImportDeeplink(input: CcSwitchImportDeeplinkInput): string {
  const platform = input.platform || 'anthropic'
  const baseUrl = platform === 'grok' ? withoutV1Suffix(input.baseUrl) : input.baseUrl
  const config = resolveCcSwitchImportConfig(platform, input.clientType, baseUrl)
  const entries: [string, string][] = [
    ['resource', 'provider'],
    ['app', config.app],
    ['name', input.providerName],
    ['homepage', baseUrl],
    ['endpoint', config.endpoint],
    ['apiKey', input.apiKey],
    ['configFormat', 'json'],
    ['usageEnabled', 'true'],
    ['usageScript', btoa(input.usageScript)],
    ['usageAutoInterval', '30']
  ]

  if (config.model) {
    entries.splice(2, 0, ['model', config.model])
  }

  return `ccswitch://v1/import?${new URLSearchParams(entries).toString()}`
}
