import type { GroupPlatform } from '@/types'

export const OPENAI_CC_SWITCH_CODEX_MODEL = 'gpt-5.6-sol'
export const OPENAI_TO_CLAUDE_DEFAULT_MODEL = 'claude-opus-4-8'
export const GROK_CC_SWITCH_MODEL = 'grok-4.5'

export type CcSwitchClientType = 'claude' | 'gemini' | 'codex'

export interface CcSwitchImportConfig {
  app: string
  endpoint: string
  model?: string
  /** Base64-encoded JSON config content for the provider (used via deeplink `config` param) */
  configJson?: Record<string, unknown>
  /** Role-based model mapping for Claude (deeplink native params) */
  haikuModel?: string
  sonnetModel?: string
  opusModel?: string
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
 * Balance query CC Switch runs against the imported provider. CC Switch fills
 * `{{baseUrl}}` with the provider's base URL as stored — Codex and Grok imports
 * carry a trailing `/v1` (see `withV1Endpoint`), Claude ones do not, and users
 * may edit it either way afterwards — then evaluates the script, so the URL
 * strips an existing `/v1` instead of blindly appending one (`/v1/v1/usage`
 * is a 404 and CC Switch shows "query failed").
 */
export const CC_SWITCH_USAGE_SCRIPT = `({
    request: {
      url: "{{baseUrl}}".replace(/\\/+$/, "").replace(/\\/v1$/, "") + "/v1/usage",
      method: "GET",
      headers: { "Authorization": "Bearer {{apiKey}}" }
    },
    extractor: function(response) {
      const remaining = response?.remaining ?? response?.quota?.remaining ?? response?.balance;
      const unit = response?.unit ?? response?.quota?.unit ?? "USD";
      return {
        isValid: response?.is_active ?? response?.isValid ?? true,
        remaining,
        unit
      };
    }
  })`

function withV1Endpoint(baseUrl: string): string {
  const normalizedBaseUrl = baseUrl.replace(/\/+$/, '')
  return normalizedBaseUrl.endsWith('/v1') ? normalizedBaseUrl : `${normalizedBaseUrl}/v1`
}

function withoutTrailingSlashes(baseUrl: string): string {
  return baseUrl.replace(/\/+$/, '')
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
      model: OPENAI_TO_CLAUDE_DEFAULT_MODEL,
      haikuModel: OPENAI_TO_CLAUDE_DEFAULT_MODEL,
      sonnetModel: OPENAI_TO_CLAUDE_DEFAULT_MODEL,
      opusModel: OPENAI_TO_CLAUDE_DEFAULT_MODEL,
      configJson: {
        env: {
          ANTHROPIC_AUTH_TOKEN: '__API_KEY__',
          ANTHROPIC_BASE_URL: baseUrl
        },
        api_format: 'openai_chat_completions'
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
        endpoint: `${baseUrl.replace(/\/+$/, '')}/antigravity`,
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
        // CC Switch's Codex provider appends the OpenAI-compatible path itself.
        // Passing /v1 here can make the client request /v1/v1/....
        endpoint: withoutTrailingSlashes(baseUrl),
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
    case 'grok':
      return {
        app: 'grokbuild',
        endpoint: withV1Endpoint(baseUrl),
        model: GROK_CC_SWITCH_MODEL
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

  if (config.haikuModel) {
    entries.push(['haikuModel', config.haikuModel])
  }

  if (config.sonnetModel) {
    entries.push(['sonnetModel', config.sonnetModel])
  }

  if (config.opusModel) {
    entries.push(['opusModel', config.opusModel])
  }

  if (config.configJson) {
    // Replace placeholder with actual API key in config JSON
    const configStr = JSON.stringify(config.configJson).replace('__API_KEY__', input.apiKey)
    entries.push(['config', btoa(configStr)])
  }

  return `ccswitch://v1/import?${new URLSearchParams(entries).toString()}`
}
