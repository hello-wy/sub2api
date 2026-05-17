# Implementation Plan: API Key Shell Commands

## Overview

Add shell one-click command generation to the UseKeyModal component. Implementation follows a bottom-up approach: first build the pure-function shell command generator module with tests, then integrate it into the Vue component with UI changes.

## Tasks

- [ ] 1. Create shell command generator module
  - [ ] 1.1 Create `src/utils/shellCommandGenerator.ts` with types and escape functions
    - Define `ShellType`, `ClientType`, `ShellCommandInput`, `ShellCommandOutput` types
    - Implement `escapeForBash(value: string): string` — escape `$`, `` ` ``, `\`, `!`, `"` with backslash inside double-quotes
    - Implement `escapeForCmd(value: string): string` — escape `%`, `"`, `&`, `|`, `<`, `>`, `^` using `^` or doubling
    - Implement `escapeForPowerShell(value: string): string` — use single-quotes with `''` for embedded single-quotes
    - Export all escape functions for testing
    - _Requirements: 7.1, 7.4_

  - [ ] 1.2 Implement client-specific command generators in `shellCommandGenerator.ts`
    - Implement `generateClaudeCodeCommand(shellType, baseUrl, apiKey, platform): ShellCommandOutput`
      - Bash: `mkdir -p ~/.claude && cat > ~/.claude/settings.json << 'EOF'\n{...}\nEOF`
      - CMD: `if not exist "%userprofile%\.claude" mkdir "%userprofile%\.claude" && (echo {...})> "%userprofile%\.claude\settings.json"`
      - PowerShell: `New-Item -ItemType Directory -Force -Path ~/.claude | Out-Null; Set-Content -Path ~/.claude/settings.json -Value '...'`
      - Include `CLAUDE_CODE_ATTRIBUTION_HEADER: "0"` when platform is "openai"
    - Implement `generateCodexCliCommand(shellType, baseUrl, apiKey, isWebSocket): ShellCommandOutput`
      - Write both `config.toml` and `auth.json`
      - Include WebSocket config when `isWebSocket` is true
    - Implement `generateOpenCodeCommand(shellType, baseUrl, apiKey, platform): ShellCommandOutput`
      - Write `opencode.json` to `~/.config/opencode/`
    - Implement main entry `generateShellCommand(input: ShellCommandInput): ShellCommandOutput`
    - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5, 3.6, 4.1, 4.2, 4.3, 4.4, 4.5, 5.2, 5.3, 5.4, 5.5, 7.2, 7.3, 7.5_

- [ ] 2. Install fast-check and write property-based tests
  - [ ] 2.1 Install `fast-check` as a dev dependency
    - Run `npm install --save-dev fast-check` in the frontend directory
    - _Requirements: N/A (test infrastructure)_

  - [ ]* 2.2 Write property test for command structure correctness (Property 1)
    - Create `src/utils/__tests__/shellCommandGenerator.spec.ts`
    - **Property 1: Command structure correctness**
    - For any valid ShellCommandInput, verify the command contains directory creation and file write operations for the correct client paths
    - **Validates: Requirements 3.1, 3.2, 3.3, 4.1, 4.2, 5.2, 5.3, 5.4**

  - [ ]* 2.3 Write property test for embedded content validity (Property 2)
    - **Property 2: Embedded content validity and consistency**
    - For any valid ShellCommandInput, verify embedded file content is parseable (JSON/TOML structure)
    - **Validates: Requirements 3.6, 4.4, 5.5**

  - [ ]* 2.4 Write property test for shell escaping round-trip safety (Property 3)
    - **Property 3: Shell escaping round-trip safety**
    - For any arbitrary string with special characters, verify escape functions produce output that round-trips correctly
    - **Validates: Requirements 7.1, 7.4**

  - [ ]* 2.5 Write property test for command safety invariants (Property 4)
    - **Property 4: Command safety invariants**
    - For any valid input, verify idempotent directory creation, quoted file paths, and truncating write operations
    - **Validates: Requirements 3.5, 4.5, 7.2, 7.3, 7.5**

  - [ ]* 2.6 Write property test for conditional content inclusion (Property 5)
    - **Property 5: Conditional content inclusion**
    - For openai+claude: verify `CLAUDE_CODE_ATTRIBUTION_HEADER` is present; for codex-ws: verify WebSocket config is present
    - **Validates: Requirements 3.4, 4.3**

- [ ] 3. Checkpoint - Verify shell command generator
  - Ensure all tests pass, ask the user if questions arise.

- [ ] 4. Modify UseKeyModal.vue to integrate shell commands
  - [ ] 4.1 Update client tab order and defaults in `UseKeyModal.vue`
    - For `openai` platform: place Claude Code first, remove `allowMessagesDispatch` condition, add Codex CLI, Codex CLI (WS), OpenCode
    - Set default `activeClientTab` to `'claude'` for openai platform
    - Keep existing tab order for other platforms unchanged
    - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 1.6_

  - [ ] 4.2 Enable OS/Shell tabs for OpenCode client tab
    - Remove the `activeClientTab.value !== 'opencode'` exclusion in `showShellTabs` computed
    - Ensure OpenCode defaults to "macOS/Linux" (unix) tab
    - _Requirements: 6.1, 6.2, 6.3_

  - [ ] 4.3 Add shell command section to UseKeyModal template
    - Import `generateShellCommand` from `shellCommandGenerator.ts`
    - Add `shellCommand` computed property that calls `generateShellCommand` with current state
    - Add a new "Shell 一键命令" section above the existing `currentFiles` code blocks
    - Include a code block with the generated command and a copy button
    - Handle clipboard success/failure (show "已复制" for 2 seconds on success, no change on failure)
    - For antigravity+opencode: render one command block per variant (Claude and Gemini)
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 5.6_

- [ ] 5. Checkpoint - Verify UI integration
  - Ensure all tests pass, ask the user if questions arise.

- [ ] 6. Write unit and component tests
  - [ ]* 6.1 Write unit tests for shell command generator
    - Create `src/utils/__tests__/shellCommandGenerator.unit.spec.ts`
    - Test specific known inputs produce expected outputs for each client/shell combination
    - Test edge cases: empty strings, URLs with trailing slashes, special characters in API keys
    - _Requirements: 3.1, 3.2, 3.3, 4.1, 4.2, 5.2, 5.3, 5.4_

  - [ ]* 6.2 Write component tests for UseKeyModal shell command integration
    - Create `src/components/keys/__tests__/UseKeyModal.spec.ts`
    - Test shell command section renders above file displays
    - Test copy button interaction and "已复制" state
    - Test tab switching regenerates the command
    - Test client tab order for each platform
    - Test OpenCode shows OS/Shell tabs
    - _Requirements: 1.1, 1.2, 2.1, 2.3, 6.1_

- [ ] 7. Final checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP
- Each task references specific requirements for traceability
- Checkpoints ensure incremental validation
- Property tests validate universal correctness properties from the design document
- Unit tests validate specific examples and edge cases
- The shell command generator is a pure-function module, making it straightforward to test independently
- `fast-check` needs to be installed as it's not currently in the project's dependencies

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1"] },
    { "id": 1, "tasks": ["1.2", "2.1"] },
    { "id": 2, "tasks": ["2.2", "2.3", "2.4", "2.5", "2.6"] },
    { "id": 3, "tasks": ["4.1", "4.2"] },
    { "id": 4, "tasks": ["4.3"] },
    { "id": 5, "tasks": ["6.1", "6.2"] }
  ]
}
```
