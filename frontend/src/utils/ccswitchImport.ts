import type { GroupPlatform } from '@/types'

export const OPENAI_CC_SWITCH_CODEX_MODEL = 'gpt-5.4'
export const OPENAI_TO_CLAUDE_DEFAULT_MODEL = 'claude-opus-4-6'

export type CcSwitchClientType = 'claude' | 'gemini' | 'codex'

export interface CcSwitchImportConfig {
  app: string
  endpoint: string
  model?: string
  /** Extra parameters for cross-platform imports (e.g. openai → claude) */
  apiFormat?: string
  authField?: string
  modelMapping?: Record<string, string>
}

export interface CcSwitchImportDeeplinkInput {
  baseUrl: string
  platform?: GroupPlatform | null
  clientType: CcSwitchClientType
  providerName: string
  apiKey: string
  usageScript: string
}

export function resolveCcSwitchImportConfig(
  platform: GroupPlatform | undefined | null,
  clientType: CcSwitchClientType,
  baseUrl: string
): CcSwitchImportConfig {
  const effectivePlatform = platform || 'anthropic'

  // Cross-platform: openai key → claude client
  if (effectivePlatform === 'openai' && clientType === 'claude') {
    return {
      app: 'claude',
      endpoint: baseUrl,
      apiFormat: 'openai_chat_completions',
      authField: 'ANTHROPIC_AUTH_TOKEN',
      model: OPENAI_TO_CLAUDE_DEFAULT_MODEL,
      modelMapping: {
        default: OPENAI_TO_CLAUDE_DEFAULT_MODEL,
        thinking: OPENAI_TO_CLAUDE_DEFAULT_MODEL,
        haiku: OPENAI_TO_CLAUDE_DEFAULT_MODEL,
        sonnet: OPENAI_TO_CLAUDE_DEFAULT_MODEL,
        opus: OPENAI_TO_CLAUDE_DEFAULT_MODEL
      }
    }
  }

  // Cross-platform: anthropic/gemini key → codex client
  if (effectivePlatform !== 'openai' && clientType === 'codex') {
    return {
      app: 'codex',
      endpoint: baseUrl,
      model: OPENAI_CC_SWITCH_CODEX_MODEL
    }
  }

  switch (effectivePlatform) {
    case 'antigravity':
      return {
        app: clientType === 'gemini' ? 'gemini' : clientType === 'codex' ? 'codex' : 'claude',
        endpoint: `${baseUrl}/antigravity`,
        model: clientType === 'codex' ? OPENAI_CC_SWITCH_CODEX_MODEL : undefined
      }
    case 'openai':
      // openai → codex (default) or openai → gemini
      if (clientType === 'gemini') {
        return {
          app: 'gemini',
          endpoint: baseUrl
        }
      }
      return {
        app: 'codex',
        endpoint: baseUrl,
        model: OPENAI_CC_SWITCH_CODEX_MODEL
      }
    case 'gemini':
      if (clientType === 'claude') {
        return {
          app: 'claude',
          endpoint: baseUrl
        }
      }
      return {
        app: 'gemini',
        endpoint: baseUrl
      }
    default:
      // anthropic
      if (clientType === 'gemini') {
        return {
          app: 'gemini',
          endpoint: baseUrl
        }
      }
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
    ['name', input.providerName],
    ['homepage', input.baseUrl],
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

  if (config.apiFormat) {
    entries.push(['apiFormat', config.apiFormat])
  }

  if (config.authField) {
    entries.push(['authField', config.authField])
  }

  if (config.modelMapping) {
    entries.push(['modelMapping', JSON.stringify(config.modelMapping)])
  }

  return `ccswitch://v1/import?${new URLSearchParams(entries).toString()}`
}
