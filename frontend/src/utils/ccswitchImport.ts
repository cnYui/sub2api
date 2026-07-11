import type { GroupPlatform } from '@/types'

export const OPENAI_CC_SWITCH_CODEX_MODEL = 'gpt-5.5'

export type CcSwitchClientType = 'claude' | 'gemini'

export interface CcSwitchImportConfig {
  app: string
  endpoint: string
  model?: string
  usageBaseUrl?: string
}

export interface CcSwitchImportDeeplinkInput {
  baseUrl: string
  platform?: GroupPlatform | null
  clientType: CcSwitchClientType
  providerName: string
  apiKey: string
  usageScript: string
}

function resolveOpenAiImportUrls(baseUrl: string): {
  endpoint: string
  usageBaseUrl: string
} {
  const normalizedBaseUrl = baseUrl.trim().replace(/\/+$/, '')
  const usageBaseUrl = normalizedBaseUrl.replace(/\/v1$/i, '')

  return {
    endpoint: `${usageBaseUrl}/v1`,
    usageBaseUrl
  }
}

export function resolveCcSwitchImportConfig(
  platform: GroupPlatform | undefined | null,
  clientType: CcSwitchClientType,
  baseUrl: string
): CcSwitchImportConfig {
  switch (platform || 'anthropic') {
    case 'antigravity':
      return {
        app: clientType === 'gemini' ? 'gemini' : 'claude',
        endpoint: `${baseUrl}/antigravity`
      }
    case 'openai': {
      const urls = resolveOpenAiImportUrls(baseUrl)
      return {
        app: 'codex',
        ...urls,
        model: OPENAI_CC_SWITCH_CODEX_MODEL
      }
    }
    case 'gemini':
      return {
        app: 'gemini',
        endpoint: baseUrl
      }
    default:
      return {
        app: 'claude',
        endpoint: baseUrl
      }
  }
}

export function buildCcSwitchImportDeeplink(input: CcSwitchImportDeeplinkInput): string {
  const config = resolveCcSwitchImportConfig(input.platform, input.clientType, input.baseUrl)
  const entries: [string, string][] = [
    ['resource', 'provider'],
    ['app', config.app],
    ...(config.model ? [['model', config.model] as [string, string]] : []),
    ['name', input.providerName],
    ['homepage', input.baseUrl],
    ['endpoint', config.endpoint],
    ...(config.usageBaseUrl
      ? [['usageBaseUrl', config.usageBaseUrl] as [string, string]]
      : []),
    ['apiKey', input.apiKey],
    ['configFormat', 'json'],
    ['usageEnabled', 'true'],
    ['usageScript', btoa(input.usageScript)],
    ['usageAutoInterval', '30']
  ]

  return `ccswitch://v1/import?${new URLSearchParams(entries).toString()}`
}
