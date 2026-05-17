import { describe, expect, it } from 'vitest'
import fc from 'fast-check'
import {
  escapeForBash,
  escapeForCmd,
  escapeForPowerShell,
  generateCodexAuth,
  generateCodexConfig,
  generateOpenCodeConfig,
  generateShellCommand,
  type ClientType,
  type ShellType
} from '../shellCommandGenerator'
import type { GroupPlatform } from '@/types'

fc.configureGlobal({ numRuns: 100 })

const shellTypes: ShellType[] = ['bash', 'cmd', 'powershell']
const clientTypes: ClientType[] = ['claude', 'codex', 'codex-ws', 'opencode']
const platforms: GroupPlatform[] = ['anthropic', 'openai', 'gemini', 'antigravity']

const inputArbitrary = fc.record({
  clientType: fc.constantFrom(...clientTypes),
  shellType: fc.constantFrom(...shellTypes),
  baseUrl: fc.webUrl(),
  apiKey: fc.string({ minLength: 1, maxLength: 40 }),
  platform: fc.constantFrom(...platforms)
})

function expectedDir(clientType: ClientType, shellType: ShellType) {
  const windows = shellType === 'cmd'
  if (clientType === 'claude') return windows ? '%userprofile%\\.claude' : '~/.claude'
  if (clientType === 'opencode') return windows ? '%userprofile%\\.config\\opencode' : '~/.config/opencode'
  return windows ? '%userprofile%\\.codex' : '~/.codex'
}

function expectedFiles(clientType: ClientType, shellType: ShellType) {
  const dir = expectedDir(clientType, shellType)
  if (clientType === 'claude') return [`${dir}${shellType === 'cmd' ? '\\' : '/'}settings.json`]
  if (clientType === 'opencode') return [`${dir}${shellType === 'cmd' ? '\\' : '/'}opencode.json`]
  return [`${dir}${shellType === 'cmd' ? '\\' : '/'}config.toml`, `${dir}${shellType === 'cmd' ? '\\' : '/'}auth.json`]
}

function extractBashContents(command: string) {
  return [...command.matchAll(/<< '([^']+)'\n([\s\S]*?)\n\1/g)].map((match) => match[2])
}

function parseJsonFromGeneratedCommand(command: string) {
  const content = extractBashContents(command)[0]
  return JSON.parse(content)
}

describe('shellCommandGenerator properties', () => {
  it('Feature: apikey-shell-commands, Property 1: Command structure correctness', () => {
    fc.assert(fc.property(inputArbitrary, (input) => {
      const { command } = generateShellCommand(input)
      expect(command).toContain(expectedDir(input.clientType, input.shellType))
      for (const file of expectedFiles(input.clientType, input.shellType)) {
        expect(command).toContain(file)
      }
      if (input.shellType === 'bash') expect(command).toContain('mkdir -p')
      if (input.shellType === 'cmd') expect(command).toContain('if not exist')
      if (input.shellType === 'powershell') expect(command).toContain('New-Item -ItemType Directory -Force')
    }))
  })

  it('Feature: apikey-shell-commands, Property 2: Embedded content validity and consistency', () => {
    fc.assert(fc.property(inputArbitrary.filter((input) => input.shellType === 'bash'), (input) => {
      const { command } = generateShellCommand(input)
      const contents = extractBashContents(command)
      if (input.clientType === 'claude') {
        expect(JSON.parse(contents[0]).env.ANTHROPIC_AUTH_TOKEN).toBe(input.apiKey)
      } else if (input.clientType === 'opencode') {
        expect(contents[0]).toBe(generateOpenCodeConfig(input.platform === 'antigravity' ? 'antigravity-claude' : input.platform, input.baseUrl, input.apiKey))
        expect(JSON.parse(contents[0]).provider).toBeTruthy()
      } else {
        expect(contents[0]).toBe(generateCodexConfig(input.baseUrl, input.clientType === 'codex-ws'))
        expect(contents[1]).toBe(generateCodexAuth(input.apiKey))
        expect(JSON.parse(contents[1]).OPENAI_API_KEY).toBe(input.apiKey)
      }
    }))
  })

  it('Feature: apikey-shell-commands, Property 3: Shell escaping round-trip safety', () => {
    fc.assert(fc.property(fc.string(), (value) => {
      expect(escapeForBash(value).replace(/\\([\\$`!\"])/g, '$1')).toBe(value)
      expect(escapeForCmd(value).replace(/\^\^/g, '^').replace(/%%/g, '%').replace(/\^(["&|<>])/g, '$1')).toBe(value)
      expect(escapeForPowerShell(value).replace(/''/g, "'")).toBe(value)
    }))
  })

  it('Feature: apikey-shell-commands, Property 4: Command safety invariants', () => {
    fc.assert(fc.property(inputArbitrary, (input) => {
      const { command } = generateShellCommand(input)
      if (input.shellType === 'bash') {
        expect(command).toContain('mkdir -p "')
        expect(command).toContain('cat > "')
      } else if (input.shellType === 'cmd') {
        expect(command).toContain('if not exist "')
        expect(command).toContain(')> "')
      } else {
        expect(command).toContain('New-Item -ItemType Directory -Force')
        expect(command).toContain('Set-Content -Path')
      }
    }))
  })

  it('Feature: apikey-shell-commands, Property 5: Conditional content inclusion', () => {
    fc.assert(fc.property(fc.webUrl(), fc.string({ minLength: 1 }), fc.constantFrom(...shellTypes), (baseUrl, apiKey, shellType) => {
      const claude = generateShellCommand({ clientType: 'claude', shellType: 'bash', baseUrl, apiKey, platform: 'openai' })
      expect(parseJsonFromGeneratedCommand(claude.command).env.CLAUDE_CODE_ATTRIBUTION_HEADER).toBe('0')

      const codexWs = generateShellCommand({ clientType: 'codex-ws', shellType, baseUrl, apiKey, platform: 'openai' })
      expect(codexWs.command).toContain('supports_websockets')
      expect(codexWs.command).toContain('responses_websockets_v2')
    }))
  })
})
