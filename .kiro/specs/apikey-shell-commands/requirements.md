# Requirements Document

## Introduction

增强"使用 API 密钥"模态对话框，为每个客户端标签页（Claude Code、Codex CLI、OpenCode）添加 Shell 一键命令功能。用户可以复制粘贴一条命令到终端，自动创建所需目录并写入配置文件，无需手动操作。同时调整客户端标签页顺序，使 Claude Code 始终可见并置于首位。

## Glossary

- **UseKeyModal**: 前端"使用 API 密钥"模态对话框组件，展示各客户端的配置方式
- **Shell_Command_Generator**: 根据客户端类型、操作系统和 API 配置生成一键 Shell 命令的模块
- **Client_Tab**: 模态对话框中的客户端选项卡（Claude Code、Codex CLI、Codex CLI WebSocket、OpenCode、Gemini CLI）
- **OS_Shell_Tab**: 操作系统/Shell 环境选项卡（macOS/Linux、Windows CMD、PowerShell）
- **One_Click_Command**: 一条可复制粘贴到终端的 Shell 命令，自动完成目录创建和配置文件写入
- **Platform**: API 密钥所属的平台类型（openai、anthropic、gemini、antigravity）

## Requirements

### Requirement 1: Claude Code 标签页始终可见并置于首位

**User Story:** As a user, I want Claude Code to always appear as the first client tab regardless of platform, so that I can quickly access the most commonly used client configuration.

#### Acceptance Criteria

1. WHEN the UseKeyModal is opened for the "openai" platform, THE Client_Tab list SHALL display "Claude Code" as the first tab, followed by "Codex CLI", "Codex CLI (WebSocket)", and "OpenCode"
2. WHEN the UseKeyModal is opened for the "openai" platform, THE Client_Tab list SHALL display "Claude Code" without requiring the `allowMessagesDispatch` condition
3. WHEN the UseKeyModal is opened for the "anthropic" platform, THE Client_Tab list SHALL display "Claude Code" as the first tab, followed by "OpenCode"
4. WHEN the UseKeyModal is opened for the "gemini" platform, THE Client_Tab list SHALL display "Gemini CLI" as the first tab, followed by "OpenCode"
5. WHEN the UseKeyModal is opened for the "antigravity" platform, THE Client_Tab list SHALL display "Claude Code" as the first tab, followed by "Gemini CLI" and "OpenCode"
6. WHEN the UseKeyModal is opened for any platform that includes Claude Code, THE UseKeyModal SHALL set "claude" as the default active client tab

### Requirement 2: Shell 一键命令区块显示

**User Story:** As a user, I want to see a shell one-click command section at the top of each client tab's content, so that I can quickly set up the configuration by copying a single command.

#### Acceptance Criteria

1. WHEN a client tab is active, THE UseKeyModal SHALL display a "Shell 一键命令" section above the existing configuration file displays, visually distinguished from the file content blocks below by a distinct section header or background style
2. THE "Shell 一键命令" section SHALL include a code block containing a single compound shell command that creates the required directories and writes all configuration files, formatted according to the currently selected OS/Shell tab (macOS/Linux bash syntax, Windows CMD syntax, or PowerShell syntax)
3. WHEN the user clicks the copy button on the one-click command, THE UseKeyModal SHALL copy the full command text to the system clipboard and display a "已复制" (copied) confirmation for 2 seconds before reverting to the default copy button state
4. WHEN the OS/Shell tab changes, THE UseKeyModal SHALL regenerate the one-click command using the syntax appropriate for the newly selected shell environment (macOS/Linux, Windows CMD, or PowerShell)
5. IF the clipboard write operation fails, THEN THE UseKeyModal SHALL not display the "已复制" confirmation and SHALL leave the copy button in its default state

### Requirement 3: Claude Code 一键命令生成

**User Story:** As a user, I want a one-click shell command for Claude Code that creates the settings.json file, so that I can configure Claude Code without manual file editing.

#### Acceptance Criteria

1. WHEN the Claude Code tab is active and the OS/Shell tab is "macOS/Linux", THE Shell_Command_Generator SHALL produce a single bash command that creates the `~/.claude` directory (using `mkdir -p`) and writes `settings.json` containing a JSON object with an `"env"` key whose value includes `"ANTHROPIC_BASE_URL"` set to the user's configured base URL, `"ANTHROPIC_AUTH_TOKEN"` set to the user's API key, and `"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC"` set to `"1"`
2. WHEN the Claude Code tab is active and the OS/Shell tab is "Windows CMD", THE Shell_Command_Generator SHALL produce a single CMD command that creates the `%userprofile%\.claude` directory (using `mkdir` with existence check) and writes `settings.json` containing a JSON object with an `"env"` key whose value includes `"ANTHROPIC_BASE_URL"` set to the user's configured base URL, `"ANTHROPIC_AUTH_TOKEN"` set to the user's API key, and `"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC"` set to `"1"`
3. WHEN the Claude Code tab is active and the OS/Shell tab is "PowerShell", THE Shell_Command_Generator SHALL produce a single PowerShell command that creates the `~/.claude` directory (using `New-Item -Force` or equivalent) and writes `settings.json` containing a JSON object with an `"env"` key whose value includes `"ANTHROPIC_BASE_URL"` set to the user's configured base URL, `"ANTHROPIC_AUTH_TOKEN"` set to the user's API key, and `"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC"` set to `"1"`
4. IF the platform is "openai" and the Claude Code tab is active, THEN THE Shell_Command_Generator SHALL include `"CLAUDE_CODE_ATTRIBUTION_HEADER": "0"` as an additional field within the `"env"` object in the generated settings.json content
5. THE Shell_Command_Generator SHALL produce a command that is safe to run multiple times: directory creation SHALL be idempotent (using `mkdir -p` or equivalent), and the settings.json file SHALL be overwritten with the newly generated content if it already exists
6. THE Shell_Command_Generator SHALL produce output that is valid JSON conforming to the structure `{"env": {<environment variables>}}`, parseable without errors by standard JSON parsers

### Requirement 4: Codex CLI 一键命令生成

**User Story:** As a user, I want a one-click shell command for Codex CLI that creates both config.toml and auth.json files, so that I can configure Codex CLI without manual file editing.

#### Acceptance Criteria

1. WHEN the Codex CLI tab is active and the OS/Shell tab is "macOS/Linux", THE Shell_Command_Generator SHALL produce a single bash command that creates the `~/.codex` directory using `mkdir -p`, writes `config.toml` with model_provider, model, base_url, and wire_api settings under `[model_providers]`, and writes `auth.json` with the API key in JSON format
2. WHEN the Codex CLI tab is active and the OS/Shell tab is "Windows", THE Shell_Command_Generator SHALL produce a single Windows CMD-compatible command that creates the `%userprofile%\.codex` directory if it does not exist, writes `config.toml` with model_provider, model, base_url, and wire_api settings under `[model_providers]`, and writes `auth.json` with the API key in JSON format
3. WHEN the Codex CLI (WebSocket) tab is active and any OS/Shell tab is selected, THE Shell_Command_Generator SHALL produce a command that includes WebSocket-specific configuration (`supports_websockets = true` in the `[model_providers.OpenAI]` section and a `[features]` section with `responses_websockets_v2 = true`) in the config.toml, in addition to all standard Codex CLI config fields
4. THE Shell_Command_Generator SHALL produce commands whose written config.toml and auth.json file content is character-for-character identical to the content shown in the existing file display blocks for the same tab and OS/Shell selection
5. THE Shell_Command_Generator SHALL use idempotent directory creation in all generated commands (`mkdir -p` for bash, `if not exist ... mkdir` for Windows CMD) so that running the command multiple times does not produce errors

### Requirement 5: OpenCode 一键命令生成

**User Story:** As a user, I want a one-click shell command for OpenCode that creates the opencode.json file, so that I can configure OpenCode without manual file editing.

#### Acceptance Criteria

1. WHEN the OpenCode tab is active, THE Shell_Command_Generator SHALL make shell commands available for all three OS/Shell environments (macOS/Linux, Windows CMD, PowerShell), each selectable via the corresponding OS/Shell tab
2. WHEN the OpenCode tab is active and the OS/Shell tab is "macOS/Linux", THE Shell_Command_Generator SHALL produce a single copyable bash command string that creates the `~/.config/opencode` directory (using `mkdir -p`) and writes the `opencode.json` file to `~/.config/opencode/opencode.json`
3. WHEN the OpenCode tab is active and the OS/Shell tab is "Windows CMD", THE Shell_Command_Generator SHALL produce a single copyable CMD command string that creates the `%userprofile%\.config\opencode` directory (using `if not exist ... mkdir`) and writes the `opencode.json` file to `%userprofile%\.config\opencode\opencode.json`
4. WHEN the OpenCode tab is active and the OS/Shell tab is "PowerShell", THE Shell_Command_Generator SHALL produce a single copyable PowerShell command string that creates the `~/.config/opencode` directory (using `New-Item -Force`) and writes the `opencode.json` file to `~/.config/opencode/opencode.json`
5. THE Shell_Command_Generator SHALL produce commands whose written file content is a character-for-character match with the opencode.json content displayed in the configuration file block below the command
6. WHEN the platform is "antigravity" and the OpenCode tab is active, THE Shell_Command_Generator SHALL produce one shell command per configuration variant (Claude and Gemini), each writing its respective `opencode.json` content to `~/.config/opencode/opencode.json`

### Requirement 6: OpenCode 标签页显示 OS/Shell 选项卡

**User Story:** As a user, I want to see OS/Shell tabs when the OpenCode tab is active, so that I can select the correct shell command for my operating system.

#### Acceptance Criteria

1. WHEN the OpenCode client tab is active, THE UseKeyModal SHALL display the OS/Shell tabs (macOS/Linux, Windows CMD, PowerShell) in the same position and style as they appear for other client tabs
2. WHEN the OpenCode client tab is active and an OS/Shell tab is selected, THE UseKeyModal SHALL display the corresponding one-click command for that shell environment
3. WHEN the OpenCode client tab becomes active, THE UseKeyModal SHALL default the OS/Shell tab to "macOS/Linux" (unix) unless the user has previously selected a different tab in the current session

### Requirement 7: 命令安全性与正确性

**User Story:** As a user, I want the generated shell commands to be safe and correct, so that running them does not corrupt existing configurations or cause errors.

#### Acceptance Criteria

1. THE Shell_Command_Generator SHALL wrap API key and base URL values in shell-appropriate quoting for each environment: double-quotes with `$`, `` ` ``, `\`, `!`, and `"` escaped using backslash in bash; double-quotes with `%`, `"`, `&`, `|`, `<`, `>`, and `^` escaped using `^` or doubling in CMD; single-quotes with embedded single-quotes handled via `''` replacement in PowerShell
2. THE Shell_Command_Generator SHALL use idempotent directory creation commands (`mkdir -p` for bash, `if not exist ... mkdir` for CMD, `New-Item -ItemType Directory -Force` for PowerShell)
3. THE Shell_Command_Generator SHALL overwrite existing configuration files by using truncating write operations (bash `>` redirect, CMD `>` redirect, PowerShell `Set-Content`) so that no previous file content is retained
4. IF the API key or base URL contains characters outside the set `[A-Za-z0-9\-_.:/?#@!$&'()*+,;=%~]`, THEN THE Shell_Command_Generator SHALL escape those characters using the shell-specific quoting rules defined in criterion 1 to prevent command injection or syntax errors
5. THE Shell_Command_Generator SHALL quote file paths in all generated commands so that user profile paths containing spaces (e.g., `C:\Users\John Doe\`) do not cause command parsing errors
