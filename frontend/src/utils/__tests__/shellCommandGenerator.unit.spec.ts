import { describe, expect, it } from 'vitest'
import {
  escapeForBash,
  escapeForCmd,
  escapeForPowerShell,
  generateClaudeCodeCommand,
  generateCodexCliCommand,
  generateOpenCodeCommand,
  generateShellCommand
} from '../shellCommandGenerator'

describe('shellCommandGenerator', () => {
  it('generates Claude Code settings command for bash', () => {
    const output = generateClaudeCodeCommand('bash', 'https://example.com/v1', 'sk-test', 'openai')

    expect(output.label).toBe('Terminal')
    expect(output.command).toContain('mkdir -p "~/.claude"')
    expect(output.command).toContain('cat > "~/.claude/settings.json"')
    expect(output.command).toContain('"ANTHROPIC_BASE_URL": "https://example.com/v1"')
    expect(output.command).toContain('"ANTHROPIC_AUTH_TOKEN": "sk-test"')
    expect(output.command).toContain('"CLAUDE_CODE_ATTRIBUTION_HEADER": "0"')
  })

  it('generates Codex CLI command with both files', () => {
    const output = generateCodexCliCommand('cmd', 'https://example.com/v1', 'sk&test', false)

    expect(output.command).toContain('if not exist "%userprofile%\\.codex" mkdir "%userprofile%\\.codex"')
    expect(output.command).toContain(')> "%userprofile%\\.codex\\config.toml"')
    expect(output.command).toContain(')> "%userprofile%\\.codex\\auth.json"')
    expect(output.command).toContain('sk^&test')
  })

  it('generates Codex WebSocket config', () => {
    const output = generateShellCommand({
      clientType: 'codex-ws',
      shellType: 'powershell',
      baseUrl: 'https://example.com/v1',
      apiKey: "sk'test",
      platform: 'openai'
    })

    expect(output.command).toContain('supports_websockets = true')
    expect(output.command).toContain('responses_websockets_v2 = true')
    expect(output.command).toContain("sk''test")
  })

  it('generates OpenCode command for PowerShell', () => {
    const output = generateOpenCodeCommand('powershell', 'https://example.com/v1', 'sk-test', 'openai')

    expect(output.label).toBe('PowerShell')
    expect(output.command).toContain("New-Item -ItemType Directory -Force -Path '~/.config/opencode'")
    expect(output.command).toContain("Set-Content -Path '~/.config/opencode/opencode.json'")
    expect(output.command).toContain('GPT-5.4 Mini')
  })

  it('escapes special characters for each shell', () => {
    expect(escapeForBash('$`\\!"')).toBe('\\$\\`\\\\\\!\\"')
    expect(escapeForCmd('%"&|<>^')).toBe('%%^"^&^|^<^>^^')
    expect(escapeForPowerShell("a'b")).toBe("a''b")
  })
})
