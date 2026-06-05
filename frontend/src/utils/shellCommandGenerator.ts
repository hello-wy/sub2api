import type { GroupPlatform } from '@/types'

export type ShellType = 'bash' | 'cmd' | 'powershell'
export type ClientType = 'claude' | 'codex' | 'codex-ws' | 'opencode' | 'gemini'
export type OpenCodeVariant = 'claude' | 'gemini'

export interface ShellCommandInput {
  clientType: ClientType
  shellType: ShellType
  baseUrl: string
  apiKey: string
  platform: GroupPlatform
  openCodeVariant?: OpenCodeVariant
}

export interface ShellCommandOutput {
  command: string
  label: string
}

interface FileToWrite {
  path: string
  content: string
}

const CODEX_API_KEY_PLACEHOLDER = '<YOUR_OPENAI_API_KEY>'

export function escapeForBash(value: string): string {
  return value.replace(/[\\$`!\"]/g, '\\$&')
}

export function escapeForCmd(value: string): string {
  return value
    .replace(/\^/g, '^^')
    .replace(/%/g, '%%')
    .replace(/"/g, '^"')
    .replace(/&/g, '^&')
    .replace(/\|/g, '^|')
    .replace(/</g, '^<')
    .replace(/>/g, '^>')
}

export function escapeForPowerShell(value: string): string {
  return value.replace(/'/g, "''")
}

export function normalizeBaseUrl(baseUrl: string): string {
  return baseUrl || window.location.origin
}

export function withoutV1(baseUrl: string): string {
  return normalizeBaseUrl(baseUrl).replace(/\/v1\/?$/, '').replace(/\/+$/, '')
}

export function ensureV1(value: string): string {
  const trimmed = value.replace(/\/+$/, '')
  return trimmed.endsWith('/v1') ? trimmed : `${trimmed}/v1`
}

export function geminiV1Beta(baseUrl: string): string {
  const trimmed = withoutV1(baseUrl).replace(/\/+$/, '')
  return trimmed.endsWith('/v1beta') ? trimmed : `${trimmed}/v1beta`
}

export function antigravityV1(baseUrl: string): string {
  return ensureV1(`${withoutV1(baseUrl)}/antigravity`)
}

export function antigravityGeminiV1Beta(baseUrl: string): string {
  const trimmed = `${withoutV1(baseUrl)}/antigravity`.replace(/\/+$/, '')
  return trimmed.endsWith('/v1beta') ? trimmed : `${trimmed}/v1beta`
}

export function generateClaudeCodeSettings(baseUrl: string, apiKey: string, platform: GroupPlatform): string {
  return JSON.stringify(
    {
      env: {
        ANTHROPIC_BASE_URL: baseUrl,
        ANTHROPIC_AUTH_TOKEN: apiKey,
        CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC: '1',
        ...(platform === 'openai' ? { CLAUDE_CODE_ATTRIBUTION_HEADER: '0' } : {})
      }
    },
    null,
    2
  )
}

export function generateCodexConfig(baseUrl: string, isWebSocket = false): string {
  return `model_provider = "OpenAI"
model = "gpt-5.5"
review_model = "gpt-5.5"
model_reasoning_effort = "xhigh"
disable_response_storage = true
network_access = "enabled"
windows_wsl_setup_acknowledged = true
model_context_window = 1000000
model_auto_compact_token_limit = 900000

[model_providers.OpenAI]
name = "OpenAI"
base_url = "${baseUrl}"
wire_api = "responses"${isWebSocket ? '\nsupports_websockets = true' : ''}
requires_openai_auth = true${isWebSocket ? '\n\n[features]\nresponses_websockets_v2 = true' : ''}`
}

export function generateCodexAuth(apiKey: string): string {
  return JSON.stringify({ OPENAI_API_KEY: apiKey }, null, 2)
}

export function generateOpenCodeConfig(platform: GroupPlatform | 'antigravity-claude' | 'antigravity-gemini', baseUrl: string, apiKey: string): string {
  const provider: Record<string, any> = {
    [platform]: {
      options: {
        baseURL: baseUrl,
        apiKey
      }
    }
  }
  const openaiModels = {
    'gpt-5.2': {
      name: 'GPT-5.2',
      limit: { context: 400000, output: 128000 },
      options: { store: false },
      variants: { low: {}, medium: {}, high: {}, xhigh: {} }
    },
    'gpt-5.5': {
      name: 'GPT-5.5',
      limit: { context: 1050000, output: 128000 },
      options: { store: false },
      variants: { low: {}, medium: {}, high: {}, xhigh: {} }
    },
    'gpt-5.4': {
      name: 'GPT-5.4',
      limit: { context: 1050000, output: 128000 },
      options: { store: false },
      variants: { low: {}, medium: {}, high: {}, xhigh: {} }
    },
    'gpt-5.4-mini': {
      name: 'GPT-5.4 Mini',
      limit: { context: 400000, output: 128000 },
      options: { store: false },
      variants: { low: {}, medium: {}, high: {}, xhigh: {} }
    },
    'gpt-5.3-codex-spark': {
      name: 'GPT-5.3 Codex Spark',
      limit: { context: 128000, output: 32000 },
      options: { store: false },
      variants: { low: {}, medium: {}, high: {}, xhigh: {} }
    },
    'gpt-5.3-codex': {
      name: 'GPT-5.3 Codex',
      limit: { context: 400000, output: 128000 },
      options: { store: false },
      variants: { low: {}, medium: {}, high: {}, xhigh: {} }
    },
    'codex-mini-latest': {
      name: 'Codex Mini',
      limit: { context: 200000, output: 100000 },
      options: { store: false },
      variants: { low: {}, medium: {}, high: {} }
    }
  }
  const geminiModels = {
    'gemini-2.0-flash': {
      name: 'Gemini 2.0 Flash',
      limit: { context: 1048576, output: 65536 },
      modalities: { input: ['text', 'image', 'pdf'], output: ['text'] }
    },
    'gemini-2.5-flash': {
      name: 'Gemini 2.5 Flash',
      limit: { context: 1048576, output: 65536 },
      modalities: { input: ['text', 'image', 'pdf'], output: ['text'] }
    },
    'gemini-2.5-pro': {
      name: 'Gemini 2.5 Pro',
      limit: { context: 2097152, output: 65536 },
      modalities: { input: ['text', 'image', 'pdf'], output: ['text'] },
      options: { thinking: { budgetTokens: 24576, type: 'enabled' } }
    },
    'gemini-3-flash-preview': {
      name: 'Gemini 3 Flash Preview',
      limit: { context: 1048576, output: 65536 },
      modalities: { input: ['text', 'image', 'pdf'], output: ['text'] }
    },
    'gemini-3-pro-preview': {
      name: 'Gemini 3 Pro Preview',
      limit: { context: 1048576, output: 65536 },
      modalities: { input: ['text', 'image', 'pdf'], output: ['text'] },
      options: { thinking: { budgetTokens: 24576, type: 'enabled' } }
    },
    'gemini-3.1-pro-preview': {
      name: 'Gemini 3.1 Pro Preview',
      limit: { context: 1048576, output: 65536 },
      modalities: { input: ['text', 'image', 'pdf'], output: ['text'] },
      options: { thinking: { budgetTokens: 24576, type: 'enabled' } }
    }
  }
  const antigravityGeminiModels = {
    'gemini-2.5-flash': {
      name: 'Gemini 2.5 Flash',
      limit: { context: 1048576, output: 65536 },
      modalities: { input: ['text', 'image', 'pdf'], output: ['text'] },
      options: { thinking: { budgetTokens: 24576, type: 'disable' } }
    },
    'gemini-2.5-flash-lite': {
      name: 'Gemini 2.5 Flash Lite',
      limit: { context: 1048576, output: 65536 },
      modalities: { input: ['text', 'image', 'pdf'], output: ['text'] },
      options: { thinking: { budgetTokens: 24576, type: 'enabled' } }
    },
    'gemini-2.5-flash-thinking': {
      name: 'Gemini 2.5 Flash (Thinking)',
      limit: { context: 1048576, output: 65536 },
      modalities: { input: ['text', 'image', 'pdf'], output: ['text'] },
      options: { thinking: { budgetTokens: 24576, type: 'enabled' } }
    },
    'gemini-3-flash': {
      name: 'Gemini 3 Flash',
      limit: { context: 1048576, output: 65536 },
      modalities: { input: ['text', 'image', 'pdf'], output: ['text'] },
      options: { thinking: { budgetTokens: 24576, type: 'enabled' } }
    },
    'gemini-3.1-pro-low': {
      name: 'Gemini 3.1 Pro Low',
      limit: { context: 1048576, output: 65536 },
      modalities: { input: ['text', 'image', 'pdf'], output: ['text'] },
      options: { thinking: { budgetTokens: 24576, type: 'enabled' } }
    },
    'gemini-3.1-pro-high': {
      name: 'Gemini 3.1 Pro High',
      limit: { context: 1048576, output: 65536 },
      modalities: { input: ['text', 'image', 'pdf'], output: ['text'] },
      options: { thinking: { budgetTokens: 24576, type: 'enabled' } }
    },
    'gemini-2.5-flash-image': {
      name: 'Gemini 2.5 Flash Image',
      limit: { context: 1048576, output: 65536 },
      modalities: { input: ['text', 'image'], output: ['image'] },
      options: { thinking: { budgetTokens: 24576, type: 'enabled' } }
    },
    'gemini-3.1-flash-image': {
      name: 'Gemini 3.1 Flash Image',
      limit: { context: 1048576, output: 65536 },
      modalities: { input: ['text', 'image'], output: ['image'] },
      options: { thinking: { budgetTokens: 24576, type: 'enabled' } }
    }
  }
  const claudeModels = {
    'claude-opus-4-6-thinking': {
      name: 'Claude 4.6 Opus (Thinking)',
      limit: { context: 200000, output: 128000 },
      modalities: { input: ['text', 'image', 'pdf'], output: ['text'] },
      options: { thinking: { budgetTokens: 24576, type: 'enabled' } }
    },
    'claude-sonnet-4-6': {
      name: 'Claude 4.6 Sonnet',
      limit: { context: 200000, output: 64000 },
      modalities: { input: ['text', 'image', 'pdf'], output: ['text'] },
      options: { thinking: { budgetTokens: 24576, type: 'enabled' } }
    }
  }

  if (platform === 'gemini') {
    provider[platform].npm = '@ai-sdk/google'
    provider[platform].models = geminiModels
  } else if (platform === 'anthropic') {
    provider[platform].npm = '@ai-sdk/anthropic'
  } else if (platform === 'antigravity-claude') {
    provider[platform].npm = '@ai-sdk/anthropic'
    provider[platform].name = 'Antigravity (Claude)'
    provider[platform].models = claudeModels
  } else if (platform === 'antigravity-gemini') {
    provider[platform].npm = '@ai-sdk/google'
    provider[platform].name = 'Antigravity (Gemini)'
    provider[platform].models = antigravityGeminiModels
  } else if (platform === 'openai') {
    provider[platform].models = openaiModels
  }

  const agent = platform === 'openai'
    ? {
        build: { options: { store: false } },
        plan: { options: { store: false } }
      }
    : undefined

  return JSON.stringify(
    {
      provider,
      ...(agent ? { agent } : {}),
      $schema: 'https://opencode.ai/config.json'
    },
    null,
    2
  )
}

function labelForShell(shellType: ShellType): string {
  switch (shellType) {
    case 'cmd':
      return 'Windows CMD'
    case 'powershell':
      return 'PowerShell'
    default:
      return 'Terminal'
  }
}

function heredocDelimiter(content: string): string {
  let delimiter = 'EOF'
  while (content.includes(`\n${delimiter}\n`) || content.startsWith(`${delimiter}\n`) || content.endsWith(`\n${delimiter}`)) {
    delimiter = `${delimiter}_CONFIG`
  }
  return delimiter
}

function bashWrite(files: FileToWrite[], dir: string): string {
  return [
    `mkdir -p "${dir}"`,
    ...files.map((file) => {
      const delimiter = heredocDelimiter(file.content)
      return `cat > "${file.path}" << '${delimiter}'\n${file.content}\n${delimiter}`
    })
  ].join(' && ')
}

function bashCodexCliWrite(config: string, auth: string): ShellCommandOutput {
  const configDelimiter = heredocDelimiter(config)
  const authDelimiter = heredocDelimiter(auth)
  const command = [
    'mkdir -p ~/.codex && \\',
    `cat > ~/.codex/config.toml << '${configDelimiter}'`,
    config,
    configDelimiter,
    '',
    `cat > ~/.codex/auth.json << '${authDelimiter}'`,
    auth,
    authDelimiter,
    '',
    'chmod 600 ~/.codex/auth.json && \\',
    'ls -la ~/.codex && \\',
    'cat ~/.codex/config.toml'
  ].join('\n')

  return { command, label: labelForShell('bash') }
}

function cmdEchoLine(line: string): string {
  return line.length ? `echo ${escapeForCmd(line)}` : 'echo.'
}

function cmdWrite(files: FileToWrite[], dir: string): string {
  return [
    `if not exist "${dir}" mkdir "${dir}"`,
    ...files.map((file) => `(${file.content.split('\n').map(cmdEchoLine).join(' & ')})> "${file.path}"`)
  ].join(' && ')
}

function powershellWrite(files: FileToWrite[], dir: string): string {
  return [
    `New-Item -ItemType Directory -Force -Path '${escapeForPowerShell(dir)}' | Out-Null`,
    ...files.map((file) => `Set-Content -Path '${escapeForPowerShell(file.path)}' -Value '${escapeForPowerShell(file.content)}'`)
  ].join('; ')
}

function writeCommand(shellType: ShellType, dir: string, files: FileToWrite[]): ShellCommandOutput {
  switch (shellType) {
    case 'cmd':
      return { command: cmdWrite(files, dir), label: labelForShell(shellType) }
    case 'powershell':
      return { command: powershellWrite(files, dir), label: labelForShell(shellType) }
    default:
      return { command: bashWrite(files, dir), label: labelForShell(shellType) }
  }
}

export function generateClaudeCodeCommand(shellType: ShellType, baseUrl: string, apiKey: string, platform: GroupPlatform): ShellCommandOutput {
  const dir = shellType === 'cmd' ? '%userprofile%\\.claude' : '~/.claude'
  const filePath = shellType === 'cmd' ? '%userprofile%\\.claude\\settings.json' : '~/.claude/settings.json'
  return writeCommand(shellType, dir, [
    { path: filePath, content: generateClaudeCodeSettings(baseUrl, apiKey, platform) }
  ])
}

export function generateCodexCliCommand(shellType: ShellType, baseUrl: string, apiKey: string, isWebSocket = false): ShellCommandOutput {
  const dir = shellType === 'cmd' ? '%userprofile%\\.codex' : '~/.codex'
  const configPath = shellType === 'cmd' ? '%userprofile%\\.codex\\config.toml' : '~/.codex/config.toml'
  const authPath = shellType === 'cmd' ? '%userprofile%\\.codex\\auth.json' : '~/.codex/auth.json'
  const config = generateCodexConfig(baseUrl, isWebSocket)
  const auth = generateCodexAuth(shellType === 'bash' ? CODEX_API_KEY_PLACEHOLDER : apiKey)

  if (shellType === 'bash') {
    return bashCodexCliWrite(config, auth)
  }

  return writeCommand(shellType, dir, [
    { path: configPath, content: config },
    { path: authPath, content: auth }
  ])
}

export function generateOpenCodeCommand(shellType: ShellType, baseUrl: string, apiKey: string, platform: GroupPlatform, variant?: OpenCodeVariant): ShellCommandOutput {
  const dir = shellType === 'cmd' ? '%userprofile%\\.config\\opencode' : '~/.config/opencode'
  const path = shellType === 'cmd' ? '%userprofile%\\.config\\opencode\\opencode.json' : '~/.config/opencode/opencode.json'
  let configPlatform: GroupPlatform | 'antigravity-claude' | 'antigravity-gemini' = platform
  if (platform === 'antigravity') {
    configPlatform = variant === 'gemini' ? 'antigravity-gemini' : 'antigravity-claude'
  }
  return writeCommand(shellType, dir, [
    { path, content: generateOpenCodeConfig(configPlatform, baseUrl, apiKey) }
  ])
}

export function generateShellCommand(input: ShellCommandInput): ShellCommandOutput {
  switch (input.clientType) {
    case 'claude':
      return generateClaudeCodeCommand(input.shellType, input.baseUrl, input.apiKey, input.platform)
    case 'codex-ws':
      return generateCodexCliCommand(input.shellType, input.baseUrl, input.apiKey, true)
    case 'codex':
      return generateCodexCliCommand(input.shellType, input.baseUrl, input.apiKey, false)
    case 'opencode':
      return generateOpenCodeCommand(input.shellType, input.baseUrl, input.apiKey, input.platform, input.openCodeVariant)
    default:
      return { command: '', label: labelForShell(input.shellType) }
  }
}
