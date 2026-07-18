<template>
  <div class="docs-shell">
    <header class="docs-header">
      <div class="docs-header-inner">
        <router-link to="/home" class="docs-header-brand" aria-label="返回 SolidAPI 首页">
          <img :src="siteLogo || '/logo.png'" alt="" />
          <span>
            <strong>SolidAPI</strong>
            <small>使用文档</small>
          </span>
        </router-link>

        <div class="docs-header-context" aria-live="polite">
          <span>指南</span>
          <Icon name="chevronRight" size="xs" />
          <strong>{{ activeSection.title }}</strong>
        </div>

        <div class="docs-header-actions">
          <span class="docs-reading-label">已阅读 {{ readingProgress }}%</span>
          <button
            type="button"
            class="docs-icon-button"
            :aria-label="isDark ? '切换为亮色模式' : '切换为暗色模式'"
            :title="isDark ? '切换为亮色模式' : '切换为暗色模式'"
            @click="toggleTheme"
          >
            <Icon :name="isDark ? 'sun' : 'moon'" size="sm" />
          </button>
          <router-link to="/login" class="docs-login">进入控制台 <Icon name="arrowRight" size="xs" /></router-link>
        </div>
      </div>
      <div class="docs-reading-track" aria-hidden="true">
        <span :style="{ transform: `scaleX(${readingProgress / 100})` }"></span>
      </div>
    </header>

    <div class="docs-layout">
      <button type="button" class="docs-mobile-toggle" @click="mobileNavOpen = !mobileNavOpen">
        <Icon name="menu" size="sm" />
        <span>{{ activeSection.title }}</span>
        <Icon :name="mobileNavOpen ? 'chevronUp' : 'chevronDown'" size="sm" class="ml-auto" />
      </button>

      <aside class="docs-sidebar" :class="{ 'is-open': mobileNavOpen }" aria-label="文档章节">
        <div class="docs-sidebar-shell relative flex h-full flex-col">
          <div class="docs-sidebar-header">
            <router-link to="/home" class="docs-sidebar-logo" aria-label="SolidAPI 首页">
              <img :src="siteLogo || '/logo.png'" alt="SolidAPI Logo" class="h-full w-full object-contain" />
            </router-link>
            <div class="docs-sidebar-brand">
              <router-link to="/home" class="docs-sidebar-brand-title">SolidAPI</router-link>
              <span>使用文档</span>
            </div>
          </div>

          <label class="docs-search">
            <Icon name="search" size="sm" />
            <input v-model="searchQuery" type="search" placeholder="搜索教程" aria-label="搜索教程" />
          </label>

          <nav class="docs-sidebar-nav">
            <div v-for="group in filteredGroups" :key="group.title" class="docs-nav-group">
              <h2 class="docs-nav-group-title">{{ group.title }}</h2>
              <button
                v-for="item in group.items"
                :key="item.id"
                type="button"
                class="docs-nav-item"
                :class="{ 'is-active': activeId === item.id }"
                @click="selectSection(item.id)"
              >
                <Icon :name="item.icon" size="sm" />
                <span>{{ item.title }}</span>
              </button>
            </div>
            <div v-if="filteredGroups.length === 0" class="docs-search-empty">没有找到相关教程</div>
          </nav>
          <div class="docs-sidebar-footer">
            <Icon name="infoCircle" size="sm" />
            <span>先检查端点、密钥和模型名称是否与分组一致。</span>
          </div>
        </div>
      </aside>

      <main class="docs-main">
        <div class="docs-breadcrumb"><span>文档</span><Icon name="chevronRight" size="xs" /><strong>{{ activeSection.title }}</strong></div>
        <article :key="activeSection.id" class="docs-article">
          <div class="docs-article-meta">
            <span class="docs-kicker">{{ activeSection.kicker }}</span>
            <span>{{ currentSectionIndex + 1 }} / {{ documentationSections.length }}</span>
            <span>约 {{ estimatedReadMinutes }} 分钟</span>
          </div>
          <h1>{{ activeSection.title }}</h1>
          <p class="docs-lead">{{ activeSection.intro }}</p>

          <section v-if="activeSection.steps?.length" class="docs-section">
            <h2>操作步骤</h2>
            <ol class="docs-steps">
              <li v-for="(step, index) in activeSection.steps" :key="step.title">
                <span class="docs-step-number">{{ String(index + 1).padStart(2, '0') }}</span>
                <div>
                  <h3>{{ step.title }}</h3>
                  <p>{{ step.body }}</p>
                </div>
              </li>
            </ol>
          </section>

          <section v-if="activeSection.media?.length" class="docs-section">
            <h2>页面指引</h2>
            <div class="docs-media-list">
              <figure v-for="media in activeSection.media" :key="media.src" class="docs-media">
                <img :src="media.src" :alt="media.alt" loading="lazy" />
                <figcaption>{{ media.caption }}</figcaption>
              </figure>
            </div>
          </section>

          <section v-if="activeSection.codes?.length" class="docs-section">
            <h2>配置示例</h2>
            <div v-for="block in activeSection.codes" :key="block.id" class="docs-code-card">
              <div class="docs-code-head">
                <span><Icon name="terminal" size="xs" /> {{ block.label }}</span>
                <button type="button" class="docs-copy" @click="copyCode(block.id, block.content)">
                  <Icon :name="copiedId === block.id ? 'check' : 'copy'" size="xs" />
                  {{ copiedId === block.id ? '已复制' : '复制代码' }}
                </button>
              </div>
              <pre><code>{{ block.content }}</code></pre>
            </div>
          </section>

          <section v-if="activeSection.notes?.length" class="docs-section">
            <h2>注意事项</h2>
            <div class="docs-note-list">
              <div v-for="note in activeSection.notes" :key="note" class="docs-note">
                <Icon name="lightbulb" size="sm" />
                <p>{{ note }}</p>
              </div>
            </div>
          </section>

          <section v-if="activeSection.links?.length" class="docs-section docs-links-section">
            <h2>相关入口</h2>
            <div class="docs-links">
              <a v-for="link in activeSection.links" :key="link.label" :href="link.href" target="_blank" rel="noopener noreferrer">
                <span>{{ link.label }}</span><Icon name="externalLink" size="xs" />
              </a>
            </div>
          </section>
        </article>
        <div class="docs-article-footer">
          <button type="button" class="docs-back-top" @click="scrollTop"><Icon name="arrowUp" size="xs" /> 返回文档顶部</button>
          <button v-if="nextSection" type="button" class="docs-next" @click="selectSection(nextSection.id)">
            <span>下一篇</span>
            <strong>{{ nextSection.title }}</strong>
            <Icon name="arrowRight" size="xs" />
          </button>
        </div>
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import { useClipboard } from '@/composables/useClipboard'
import { useAppStore } from '@/stores'
import { sanitizeUrl } from '@/utils/url'
import affiliateOverviewImage from '@/assets/docs/affiliate-overview.png'
import apiKeyListImage from '@/assets/docs/api-key-list.png'
import ccSwitchBalanceImage from '@/assets/docs/cc-switch-balance.png'
import ccSwitchUsageEntryImage from '@/assets/docs/cc-switch-usage-entry.png'

type IconName = InstanceType<typeof Icon>['$props']['name']
type CodeBlock = { id: string; label: string; content: string }
type Step = { title: string; body: string }
type SectionMedia = { src: string; alt: string; caption: string }
type Section = {
  id: string
  title: string
  kicker: string
  icon: IconName
  intro: string
  steps?: Step[]
  codes?: CodeBlock[]
  media?: SectionMedia[]
  notes?: string[]
  links?: { label: string; href: string }[]
}

const sections: Section[] = [
  {
    id: 'quick-start', title: '快速开始', kicker: 'START HERE', icon: 'bolt',
    intro: '三分钟完成第一次请求。创建一个 API Key，确认可用分组，然后把对应端点填入你常用的客户端。',
    steps: [
      { title: '创建 API Key', body: '登录后打开“API 密钥”，创建一个新密钥并妥善保存。密钥只会在创建时完整显示。' },
      { title: '选择可用分组', body: '在密钥详情中确认默认分组与平台类型。不同平台对应不同的端点格式，客户端教程会给出完整示例。' },
      { title: '发起一次测试请求', body: '先使用低成本模型验证连通性，再切换到日常使用的模型。遇到 401 或 404 时，优先核对端点末尾的路径。' }
    ],
    media: [{ src: apiKeyListImage, alt: 'SolidAPI API 密钥列表与使用指引入口', caption: '在 API 密钥列表中复制密钥，或直接选择“使用密钥”“导入到 CC Switch”。' }],
    codes: [{ id: 'curl-test', label: 'curl · OpenAI Responses', content: `curl "$BASE_URL/v1/responses" \\\n+  -H "Authorization: Bearer $API_KEY" \\\n+  -H "Content-Type: application/json" \\\n+  -d '{"model":"gpt-5.5","input":"你好，介绍一下你自己。"}'` }],
    notes: ['BASE_URL 使用控制台“端点”中显示的地址；不要重复拼接 /v1。', '将 API Key 放入环境变量或系统密钥环，不要提交到 Git 仓库。']
  },
  {
    id: 'api-key', title: 'API 密钥', kicker: 'ACCOUNT', icon: 'key',
    intro: 'API Key 是客户端访问网关的唯一凭证。每个密钥都可以单独设置名称、分组和额度，便于审计与轮换。',
    steps: [
      { title: '创建与命名', body: '进入“API 密钥”页面，使用可识别的名称，例如 personal-mac 或 ci-staging。' },
      { title: '限制使用范围', body: '按需绑定分组、设置过期时间和额度。生产环境建议为每个应用单独创建密钥。' },
      { title: '轮换与撤销', body: '怀疑泄露时立即撤销旧密钥，再更新客户端环境变量。撤销不会影响其他密钥。' }
    ],
    notes: ['不要在前端公开代码、截图或日志中暴露完整密钥。', '如果客户端支持多个密钥，建议保留一个备用密钥用于平滑轮换。']
  },
  {
    id: 'endpoints', title: '端点与模型', kicker: 'REFERENCE', icon: 'server',
    intro: '端点由分组平台决定。保持协议、路径和模型名称一致，是排查请求失败最快的方法。',
    codes: [
      { id: 'openai-endpoint', label: 'OpenAI 兼容', content: `Base URL: https://your-domain.example/v1\n协议: Responses 或 Chat Completions\n认证: Authorization: Bearer <API_KEY>` },
      { id: 'anthropic-endpoint', label: 'Anthropic Messages', content: `Base URL: https://your-domain.example\n协议: Messages API\n认证: x-api-key: <API_KEY>\n版本: anthropic-version: 2023-06-01` }
    ],
    notes: ['模型名必须使用分组中已启用的模型；模型列表以控制台实时显示为准。', '不同客户端对 Base URL 的字段命名不同，按教程示例填写，不要同时填两套端点。']
  },
  {
    id: 'affiliate-rewards', title: '邀请返利', kicker: 'ACCOUNT', icon: 'gift',
    intro: '在邀请返利页面复制专属邀请链接。受邀用户完成符合条件的充值后，返利额度会进入可转余额，具体比例以页面实时显示为准。',
    steps: [
      { title: '复制邀请链接', body: '登录后打开“邀请返利”，复制专属链接并发送给新用户。不要手动修改链接中的邀请码。' },
      { title: '等待有效充值', body: '新用户通过邀请链接注册，并完成符合返利规则的充值后，系统会生成返利记录。' },
      { title: '转入账户余额', body: '返利额度达到可转条件后，在页面中将其转入账户余额；到账记录可在余额明细中查看。' }
    ],
    media: [{ src: affiliateOverviewImage, alt: 'SolidAPI 邀请返利页面', caption: '页面会同时展示返利比例、邀请人数、可转额度以及专属邀请链接。' }],
    notes: ['返利比例、冻结时间和有效范围以站点当前规则为准。', '只有通过专属链接或邀请码完成注册的用户才会建立邀请关系。'],
    links: [{ label: '打开邀请返利', href: '/affiliate' }]
  },
  {
    id: 'cc-switch', title: 'CC switch（推荐）', kicker: 'CORE TUTORIAL', icon: 'swap',
    intro: 'CC switch 可以在多个 Claude Code 配置之间快速切换。适合需要同时使用个人、团队和临时网关的场景。',
    steps: [
      { title: '安装并打开 CC switch', body: '安装桌面客户端后，在配置列表中新增一个 Claude Code 配置。' },
      { title: '填写网关信息', body: '将 Base URL、API Key 和默认模型填入对应字段，保存后点击“启用”。' },
      { title: '从终端确认', body: '打开新的终端窗口运行 claude，发送一条简单消息确认当前配置已经生效。' }
    ],
    codes: [{ id: 'cc-switch-config', label: 'Claude Code 环境变量', content: `ANTHROPIC_BASE_URL=https://your-domain.example\nANTHROPIC_AUTH_TOKEN=<API_KEY>\nCLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1` }],
    notes: ['切换配置后请重启终端会话，避免旧环境变量继续生效。', '建议为每个环境使用不同名称和颜色，降低误用生产密钥的风险。']
  },
  {
    id: 'claude-code', title: 'Claude Code', kicker: 'CORE TUTORIAL', icon: 'terminal',
    intro: 'Claude Code 通过 Anthropic Messages API 工作。将网关地址写入环境变量后，CLI 与 VS Code 插件都可以复用同一份配置。',
    steps: [
      { title: '设置环境变量', body: '在 shell 配置文件或项目级 .env 中设置 Base URL 与令牌。' },
      { title: '启动 Claude Code', body: '重新打开终端，运行 claude。首次使用时选择跳过官方登录，直接使用环境变量。' },
      { title: '验证工具调用', body: '让 Claude Code 执行一次只读目录操作，确认文本回复和工具调用都能正常返回。' }
    ],
    codes: [{ id: 'claude-settings', label: '~/.claude/settings.json', content: `{
  "env": {
    "ANTHROPIC_BASE_URL": "https://your-domain.example",
    "ANTHROPIC_AUTH_TOKEN": "<API_KEY>",
    "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1"
  }
}` }],
    notes: ['ANTHROPIC_BASE_URL 不要附加 /v1；网关会根据平台自动处理 Messages 路径。', '若出现模型不存在，请使用分组中显示的 Claude 模型 ID。']
  },
  {
    id: 'codex', title: 'Codex CLI / App', kicker: 'CORE TUTORIAL', icon: 'cpu',
    intro: 'Codex CLI 使用 OpenAI Responses 协议。推荐通过 config.toml 指定模型提供商，并用 auth.json 保存 API Key。',
    steps: [
      { title: '创建配置目录', body: '在用户目录创建 ~/.codex，并准备 config.toml 与 auth.json 两个文件。' },
      { title: '写入提供商配置', body: '将 Base URL 指向 OpenAI 兼容端点，wire_api 使用 responses。' },
      { title: '启动并检查', body: '运行 codex，输入 /status 查看当前模型与提供商；若需切换配置，重启 CLI。' }
    ],
    codes: [
      { id: 'codex-config', label: '~/.codex/config.toml', content: `model_provider = "OpenAI"
model = "gpt-5.5"
model_reasoning_effort = "xhigh"
disable_response_storage = true

[model_providers.OpenAI]
name = "OpenAI"
base_url = "https://your-domain.example/v1"
wire_api = "responses"
requires_openai_auth = true` },
      { id: 'codex-auth', label: '~/.codex/auth.json', content: `{
  "OPENAI_API_KEY": "<API_KEY>"
}` }
    ],
    notes: ['Codex App 与 CLI 共用配置目录；修改后需要完全退出并重新打开应用。', '需要 WebSocket 时，使用控制台为该分组生成的专用端点与配置。']
  },
  {
    id: 'gemini-cli', title: 'Gemini CLI', kicker: 'CORE TUTORIAL', icon: 'sparkles',
    intro: 'Gemini CLI 使用 Google Generative Language 兼容接口。配置 API Key 和端点后即可在终端中运行。',
    steps: [
      { title: '安装 Gemini CLI', body: '按照官方方式安装 CLI，并确认 node 与 npm 已加入 PATH。' },
      { title: '设置密钥和端点', body: '将密钥写入环境变量，将 API 端点设置为控制台对应的 Gemini 分组地址。' },
      { title: '发送测试提示词', body: '运行 gemini 并发送短提示词，确认返回内容和多模态模型选择正常。' }
    ],
    codes: [{ id: 'gemini-env', label: 'Shell 环境变量', content: `export GEMINI_API_KEY="<API_KEY>"
export GOOGLE_GEMINI_BASE_URL="https://your-domain.example/v1beta"` }],
    notes: ['Gemini CLI 的变量名会随版本变化，若 CLI 不识别，请以当前版本帮助信息为准。', '使用图像或 PDF 时，确认当前模型的输入模态已在分组中启用。']
  },
  {
    id: 'cursor', title: 'Cursor', kicker: 'CLIENTS', icon: 'edit',
    intro: 'Cursor 支持在设置中添加 OpenAI 兼容模型。适合将代码补全、Agent 和聊天统一到同一网关。',
    steps: [
      { title: '打开模型设置', body: '进入 Settings → Models，开启 OpenAI API，并填写 API Key 与 Base URL。' },
      { title: '添加自定义模型', body: '输入分组中可用的模型名称，保存后在 Chat 或 Composer 中选择该模型。' },
      { title: '检查请求来源', body: '发送一次短消息，再从用量记录确认请求已经进入预期分组。' }
    ],
    notes: ['Cursor 的 Base URL 通常需要包含 /v1；如遇 404，检查是否重复拼接路径。', '建议关闭不需要的官方模型，避免误用其他额度。']
  },
  {
    id: 'cline', title: 'Cline', kicker: 'CLIENTS', icon: 'chatBubble',
    intro: 'Cline 是 VS Code 内的 Agent 扩展。选择 OpenAI Compatible 或 Anthropic Provider，并将网关配置保存到工作区。',
    steps: [
      { title: '安装扩展', body: '在 VS Code 扩展市场安装 Cline，打开侧边栏设置页。' },
      { title: '选择 Provider', body: '按分组平台选择 OpenAI Compatible 或 Anthropic，填写端点、密钥和模型。' },
      { title: '限制自动执行', body: '第一次使用建议保持审批模式，确认请求和工具调用符合预期后再提高自动化级别。' }
    ],
    notes: ['Cline 会在工作区保存部分配置，公共仓库不要提交包含密钥的设置文件。']
  },
  {
    id: 'continue', title: 'Continue', kicker: 'CLIENTS', icon: 'document',
    intro: 'Continue 支持通过 config.yaml 定义自定义模型和 Provider。配置一次后，补全与聊天可以共用。',
    steps: [
      { title: '打开配置文件', body: '在 Continue 的配置面板中打开 config.yaml，新增一个 models 条目。' },
      { title: '填写 Provider', body: 'OpenAI 兼容接口使用 provider: openai；Anthropic 接口使用 provider: anthropic。' },
      { title: '选择模型', body: '保存配置后，在 Continue 面板顶部切换到刚添加的模型。' }
    ],
    codes: [{ id: 'continue-config', label: 'config.yaml', content: `models:
  - name: SolidAPI
    provider: openai
    model: gpt-5.5
    apiBase: https://your-domain.example/v1
    apiKey: <API_KEY>` }]
  },
  {
    id: 'opencode', title: 'OpenCode', kicker: 'CLIENTS', icon: 'terminal',
    intro: 'OpenCode 使用 opencode.json 管理多个 Provider。一个文件可以同时声明 Claude、OpenAI 与 Gemini 网关。',
    steps: [
      { title: '创建配置文件', body: '在项目目录或用户配置目录创建 opencode.json。' },
      { title: '声明 Provider', body: '为当前分组添加 options.baseURL 与 options.apiKey，并配置可用模型。' },
      { title: '启动并切换', body: '启动 OpenCode 后使用模型选择器切换到 SolidAPI 配置。' }
    ],
    codes: [{ id: 'opencode-config', label: 'opencode.json', content: `{
  "$schema": "https://opencode.ai/config.json",
  "provider": {
    "router-team": {
      "options": {
        "baseURL": "https://your-domain.example/v1",
        "apiKey": "<API_KEY>"
      }
    }
  }
}` }]
  },
  {
    id: 'cherry-studio', title: 'Cherry Studio', kicker: 'CLIENTS', icon: 'chat',
    intro: 'Cherry Studio 适合管理多个对话模型。新增自定义渠道时，选择与分组一致的协议类型。',
    steps: [
      { title: '新增渠道', body: '打开设置 → 模型服务，点击新增服务商并选择 OpenAI 或 Anthropic。' },
      { title: '填写连接参数', body: '填写 API 地址、API Key 与模型名称，保存后点击测试连接。' },
      { title: '创建助手', body: '在助手设置中选择刚刚创建的模型，即可开始对话。' }
    ],
    notes: ['Cherry Studio 的 API 地址字段可能会自动补全 /v1，请保存后检查最终地址。']
  },
  {
    id: 'chatbox', title: 'ChatBox', kicker: 'CLIENTS', icon: 'chatBubble',
    intro: 'ChatBox 通过自定义 API 服务连接网关，适合个人桌面对话和提示词管理。',
    steps: [
      { title: '进入 API 设置', body: '打开设置 → 模型提供方，新增一个 OpenAI Compatible 服务。' },
      { title: '保存并设为默认', body: '填入 API Host、Key 和模型后保存，并将该服务设为默认。' },
      { title: '发送测试消息', body: '用短消息确认回复速度与模型名称显示正确。' }
    ]
  },
  {
    id: 'open-webui', title: 'Open WebUI', kicker: 'CLIENTS', icon: 'globe',
    intro: 'Open WebUI 可以把 SolidAPI 作为 OpenAI 兼容连接使用，适合团队共享工作台。',
    steps: [
      { title: '打开管理员设置', body: '进入 Connections → OpenAI API，新增一个连接。' },
      { title: '填写连接信息', body: '填写 API Base URL 与 API Key，保存后刷新模型列表。' },
      { title: '限制可见模型', body: '按团队需要隐藏不使用的模型，避免用户选到未配置的模型。' }
    ],
    notes: ['如果 Open WebUI 部署在容器中，请确保容器网络可以访问网关域名。']
  },
  {
    id: 'lobe-chat', title: 'LobeChat', kicker: 'CLIENTS', icon: 'chat',
    intro: 'LobeChat 通过自定义 OpenAI Provider 接入。建议为不同模型创建独立的助手预设。',
    steps: [
      { title: '打开设置', body: '进入设置 → 语言模型 → OpenAI，启用自定义接口。' },
      { title: '填写端点和密钥', body: '将 API Key、Base URL 和模型列表填写完整，保存后刷新。' },
      { title: '选择助手', body: '为不同任务创建代码、写作或分析助手，并分别绑定对应模型。' }
    ]
  },
  {
    id: 'other-clients', title: '其他客户端', kicker: 'CLIENTS', icon: 'grid',
    intro: 'Cursor、Zed、TRAE、NextChat、AnythingLLM、LibreChat、TypingMind、BoltAI、Droid CLI 和 OpenClaw 都可以按平台类型使用同一套端点信息。',
    steps: [
      { title: '先确认协议', body: 'OpenAI 兼容客户端使用 /v1 端点；Anthropic 客户端使用 Messages 端点；Gemini 客户端使用 v1beta 端点。' },
      { title: '按照字段映射填写', body: '将 API Key 映射到客户端的 API Key 或 Token 字段，将模型 ID 原样复制。' },
      { title: '保留最小权限', body: '客户端只启用实际使用的模型和工具，并为自动化任务单独创建密钥。' }
    ],
    notes: ['不同客户端的字段名称可能是 Endpoint、Base URL、API Host 或 Server URL，本质上都是同一个端点配置。', '配置完成后优先查看控制台用量记录，而不是只根据客户端的“连接成功”提示判断。']
  },
  {
    id: 'troubleshooting', title: '故障排查', kicker: 'HELP', icon: 'exclamationTriangle',
    intro: '按错误类型定位问题。大多数连接失败都可以通过端点、认证头和模型三项检查解决。',
    steps: [
      { title: '401 / 403', body: '确认密钥没有多余空格、没有被撤销，并检查客户端使用的是正确的认证字段。' },
      { title: '404 / model not found', body: '检查 Base URL 是否重复包含 /v1，模型名称是否存在于当前分组。' },
      { title: '429 / timeout', body: '查看额度与并发限制，降低请求频率后重试；长上下文任务建议减少并发。' },
      { title: '工具调用异常', body: '先使用最新版本客户端，关闭自定义 system prompt，再用最小提示词复现。' }
    ],
    notes: ['反馈问题时请提供时间、客户端、模型和错误码，不要发送 API Key 或完整请求内容。']
  }
]

const referenceCode = (...items: string[]) => items.join('\n')

const referenceSections: Section[] = [
  {
    id: 'cc-switch-setup', title: 'CC Switch 配置 Claude、Codex、Gemini', kicker: '核心教程', icon: 'swap',
    intro: '适合不想手动改配置文件的同学，按步骤填入即可。',
    steps: [
      { title: '添加自定义配置', body: '安装后，在应用顶部 Tab 切换到你的目标配置对象（Claude、Codex、Gemini），然后添加自定义配置。' },
      { title: '填写基础参数', body: '按下方参数填入并保存：供应商名称随意输入；API Key 粘贴你的 key；API 请求地址填写 https://ai.router.team。' },
      { title: '开启余额 / 订阅额度显示（可选）', body: '在当前供应商卡片中打开“用量查询”开关，查询方式选择“自定义”，粘贴下方配置，保存后点击刷新。' }
    ],
    codes: [{ id: 'cc-switch-basic-reference', label: '供应商基础参数', content: referenceCode('供应商名称: RouterTeam', 'API Key: sk-********（粘贴你的 key）', 'API 请求地址: https://ai.router.team') }, { id: 'cc-switch-balance-reference', label: '自定义用量查询配置', content: '({ request: { url: "https://ai.router.team/api/public/cc-switch/balance", method: "GET", headers: {"Authorization": "Bearer {{apiKey}}"} }, extractor: function(response) { return { remaining: response.balance, unit: "USD" }; } })' }],
    media: [
      { src: ccSwitchUsageEntryImage, alt: 'CC Switch 供应商卡片中的用量查询入口', caption: '在供应商卡片右侧打开用量查询入口，再选择自定义查询。' },
      { src: ccSwitchBalanceImage, alt: 'SolidAPI CC Switch 自定义余额查询配置', caption: '请求地址使用站点根地址，查询脚本返回 balance 与 USD 单位。' }
    ],
    notes: ['订阅分组的 balance 是当前还能实际调用的每日剩余额度；按量分组显示当前账户余额。', 'resetAt / resetRule 对应每日额度恢复时间；未用完的每日额度会在下一次重置或套餐到期时作废。', '想分别查看不同分组的额度时，建议按分组单独创建 API Key。']
  },
  {
    id: 'claude-code-setup', title: 'Claude Code 快速开始', kicker: '核心教程', icon: 'terminal',
    intro: 'Anthropic 官方 CLI 工具，面向高效开发流程。',
    steps: [
      { title: '安装 Node.js', body: '访问 https://nodejs.org，下载 LTS 版本的 Windows Installer (.msi)，按默认设置完成安装。' },
      { title: '安装 Claude Code CLI', body: '在 CMD/PowerShell（管理员模式）执行 npm install -g @anthropic-ai/claude-code。' },
      { title: '配置 RouterTeam', body: '访问 https://ai.router.team/api-keys 创建密钥，分组请选择 Claude Code 相关分组，额度建议设置为无限额度。Windows 配置位置为 %USERPROFILE%\\.claude\\settings.json。' },
      { title: '启动 Claude Code', body: '配置完成后进入工程目录，运行 claude。修改 settings.json 后需要重启 Claude Code 才生效。' }
    ],
    codes: [
      { id: 'claude-node-reference', label: 'CMD/PowerShell（管理员模式）', content: referenceCode('node --version', 'npm --version') },
      { id: 'claude-install-reference', label: '安装 Claude Code CLI', content: 'npm install -g @anthropic-ai/claude-code' },
      { id: 'claude-settings-reference', label: 'settings.json 配置（推荐，永久生效）', content: referenceCode('{', '  "env": {', '    "ANTHROPIC_AUTH_TOKEN": "粘贴为Claude Code分组密钥key",', '    "ANTHROPIC_BASE_URL": "https://ai.router.team"', '  }', '}') },
      { id: 'claude-run-reference', label: '启动 Claude Code', content: referenceCode('cd your-project-folder', 'claude') }
    ],
    notes: ['请将 ANTHROPIC_AUTH_TOKEN 替换为在 https://ai.router.team/api-keys 生成的 Claude Code API 密钥。']
  },
  {
    id: 'codex-cli-app', title: 'Codex 快速开始', kicker: '核心教程', icon: 'cpu',
    intro: '强大的 OpenAI 代码助手，支持工程级任务。',
    steps: [
      { title: '安装 Node.js', body: '访问 https://nodejs.org，下载 LTS 版本的 Windows Installer (.msi)，按默认设置完成安装。' },
      { title: '全局安装 Codex CLI', body: '在 CMD/PowerShell（管理员模式）执行 npm install -g @openai/codex@latest。' },
      { title: '创建 Codex 专用 API Token', body: '访问 https://ai.router.team/api-keys 创建密钥，分组请选择 Codex 相关分组，额度建议设置为无限额度。Codex 需要使用专门的分组令牌，与 Claude Code 的令牌不同。' },
      { title: '创建配置并启动', body: '创建 %USERPROFILE%\\.codex 目录，在其中创建 config.toml 与 auth.json，进入工程目录运行 codex。' }
    ],
    codes: [
      { id: 'codex-install-reference', label: '安装 Codex CLI', content: 'npm install -g @openai/codex@latest' },
      { id: 'codex-folder-reference', label: 'CMD/PowerShell（管理员模式）', content: referenceCode('mkdir %USERPROFILE%\\.codex', 'cd %USERPROFILE%\\.codex') },
      { id: 'codex-config-reference', label: 'config.toml', content: referenceCode('approval_policy = "never"', 'sandbox_mode = "danger-full-access"', 'model_provider = "routerteam"', 'model = "gpt-5.5"', 'model_reasoning_effort = "xhigh"', 'plan_mode_reasoning_effort = "xhigh"', 'model_reasoning_summary = "detailed"', 'network_access = "enabled"', 'disable_response_storage = true', 'windows_wsl_setup_acknowledged = true', 'model_verbosity = "high"', '', '[model_providers.routerteam]', 'name = "routerteam"', 'base_url = "https://ai.router.team"', 'wire_api = "responses"', 'requires_openai_auth = true') },
      { id: 'codex-auth-reference', label: 'auth.json', content: referenceCode('{', '  "OPENAI_API_KEY": "粘贴为Codex专用分组密钥key"', '}') },
      { id: 'codex-run-reference', label: '启动 Codex', content: referenceCode('mkdir my-codex-project', 'cd my-codex-project', 'codex') }
    ]
  },
  {
    id: 'gemini-cli-setup', title: 'Gemini CLI 快速开始', kicker: '核心教程', icon: 'sparkles',
    intro: 'Google 命令行 AI 工具，快速接入即可使用。',
    steps: [
      { title: '安装 Node.js', body: '访问 https://nodejs.org，下载 LTS 版本的 Windows Installer (.msi)，按默认设置完成安装。' },
      { title: '全局安装 Gemini CLI', body: '在 CMD/PowerShell（管理员模式）执行 npm install -g @google/gemini-cli。' },
      { title: '配置 Gemini CLI', body: '访问 https://ai.router.team/api-keys 创建 Gemini CLI 专用 API 密钥，在 %USERPROFILE%\\.gemini\\ 创建 .env 与 settings.json。' },
      { title: '启动 Gemini CLI', body: '配置完成后重启 Gemini CLI，运行 gemini。配置文件更加安全且便于管理。' }
    ],
    codes: [
      { id: 'gemini-install-reference', label: '安装 Gemini CLI', content: 'npm install -g @google/gemini-cli' },
      { id: 'gemini-env-reference', label: '%USERPROFILE%\\.gemini\\.env', content: referenceCode('GOOGLE_GEMINI_BASE_URL=https://ai.router.team', 'GEMINI_API_KEY=粘贴为Gemini CLI相关分组密钥key', 'GEMINI_MODEL=gemini-3-pro-preview') },
      { id: 'gemini-settings-reference', label: '%USERPROFILE%\\.gemini\\settings.json', content: referenceCode('{', '  "ide": { "enabled": true },', '  "security": { "auth": { "selectedType": "gemini-api-key" } }', '}') },
      { id: 'gemini-run-reference', label: '启动 Gemini CLI', content: 'gemini' }
    ],
    notes: ['请将 GEMINI_API_KEY 替换为在 https://ai.router.team/api-keys 生成的 Gemini CLI 专用 API 密钥。']
  }
]

referenceSections.push(
  {
    id: 'cursor-reference', title: 'Cursor 配置教程', kicker: '更多客户端', icon: 'edit',
    intro: 'Cursor 自定义 API 路由使用 OpenAI /v1/chat/completions 兼容格式。你可以直接在 Cursor 的 Other 配置里接入 RouterTeam。',
    steps: [
      { title: '开启 OpenAI 配置', body: '进入 Cursor Settings → Models，确保你是 Cursor Pro 账号，打开 OpenAI API Key 开关并粘贴 RouterTeam 后台创建的 key。' },
      { title: '填写 Base URL', body: '打开 Override OpenAI Base URL，填写 https://ai.router.team/v1。' },
      { title: '选择模型', body: 'Claude 系列直接在 Cursor 模型列表中选择；GPT 系列需要在 Add model 输入自定义模型名称。' }
    ],
    codes: [
      { id: 'cursor-base-url-reference', label: 'Cursor Base URL', content: 'https://ai.router.team/v1' },
      { id: 'cursor-models-reference', label: 'GPT 自定义模型名', content: referenceCode('routerteam-g-5', 'routerteam-g-5-low', 'routerteam-g-5-medium', 'routerteam-g-5-high', 'routerteam-g-5-xhigh', 'routerteam-g-5-codex', 'routerteam-g-5-codex-low', 'routerteam-g-5-codex-medium', 'routerteam-g-5-codex-high', 'routerteam-g-5-codex-xhigh', 'routerteam-g-5.1', 'routerteam-g-5.1-low', 'routerteam-g-5.1-medium', 'routerteam-g-5.1-high', 'routerteam-g-5.1-xhigh', 'routerteam-g-5.1-codex', 'routerteam-g-5.1-codex-low', 'routerteam-g-5.1-codex-medium', 'routerteam-g-5.1-codex-high', 'routerteam-g-5.1-codex-xhigh', 'routerteam-g-5.2', 'routerteam-g-5.2-low', 'routerteam-g-5.2-medium', 'routerteam-g-5.2-high', 'routerteam-g-5.2-xhigh', 'routerteam-g-5.2-codex', 'routerteam-g-5.2-codex-low', 'routerteam-g-5.2-codex-medium', 'routerteam-g-5.2-codex-high', 'routerteam-g-5.2-codex-xhigh', 'routerteam-g-5.3-codex', 'routerteam-g-5.3-codex-low', 'routerteam-g-5.3-codex-medium', 'routerteam-g-5.3-codex-high', 'routerteam-g-5.3-codex-xhigh', 'routerteam-g-5.5', 'routerteam-g-5.5-low', 'routerteam-g-5.5-medium', 'routerteam-g-5.5-high', 'routerteam-g-5.5-xhigh', 'routerteam-g-5.5-codex', 'routerteam-g-5.5-codex-low', 'routerteam-g-5.5-codex-medium', 'routerteam-g-5.5-codex-high', 'routerteam-g-5.5-codex-xhigh') }
    ],
    notes: ['Claude 模型直接选择 Opus 4.6、Opus 4.5、Sonnet 或 Haiku，不需要输入自定义模型名。', '基础模型不带强度后缀；强度模型在末尾追加 -low/-medium/-high/-xhigh。']
  },
  {
    id: 'codex-plugin', title: 'VSCode Codex 插件安装教程', kicker: '更多客户端', icon: 'document',
    intro: '适合使用 VSCode 图形界面安装 Codex 插件，并通过 API Key 登录的用户。',
    steps: [
      { title: '安装插件', body: 'VSCode 打开后选择任意项目文件夹进入工作区，打开左侧插件扩展界面，搜索 Codex 并安装。' },
      { title: '配置全局文件', body: 'Windows 路径为 %USERPROFILE%\\.codex\\config.toml 与 %USERPROFILE%\\.codex\\auth.json；macOS/Linux 为 ~/.codex。' },
      { title: '使用 API Key 登录', body: '退出已有登录后选择 Use API Key，输入在 https://ai.router.team/api-keys 创建的 Codex 分组 key，点击 OK，再点击 Continue。' }
    ],
    codes: [
      { id: 'codex-plugin-config-reference', label: 'config.toml', content: referenceCode('approval_policy = "never"', 'sandbox_mode = "danger-full-access"', 'model_provider = "routerteam"', 'model = "gpt-5.5"', 'model_reasoning_effort = "xhigh"', 'plan_mode_reasoning_effort = "xhigh"', 'model_reasoning_summary = "detailed"', 'network_access = "enabled"', 'disable_response_storage = true', 'windows_wsl_setup_acknowledged = true', 'model_verbosity = "high"', '', '[model_providers.routerteam]', 'name = "routerteam"', 'base_url = "https://ai.router.team"', 'wire_api = "responses"', 'requires_openai_auth = true') },
      { id: 'codex-plugin-auth-reference', label: 'auth.json', content: referenceCode('{', '  "OPENAI_API_KEY": "粘贴为Codex专用分组密钥key"', '}') }
    ],
    notes: ['登录成功后即可开始对话，底部可以切换模型与推理强度。']
  },
  {
    id: 'cline-reference', title: 'Cline 接入教程', kicker: '更多客户端', icon: 'chatBubble',
    intro: 'Cline 官方支持 OpenAI Compatible 提供商。你只需要填入 Base URL、API Key 和模型 ID，即可直接接入 RouterTeam。',
    steps: [{ title: '选择 Provider', body: '打开 Cline 设置页，在 API Provider 中选择 OpenAI Compatible。' }, { title: '填写参数', body: 'Base URL 填写 https://ai.router.team/v1；API Key 粘贴后台创建的 API 密钥；Model ID 输入 gpt-5.5，或替换为其他 RouterTeam 模型名。' }, { title: '开始使用', body: '保存后新建会话即可开始使用；若模型列表未自动出现，可手动输入模型 ID。' }],
    codes: [{ id: 'cline-reference', label: '关键参数', content: referenceCode('Base URL: https://ai.router.team/v1', '模型 ID: gpt-5.5') }],
    notes: ['Cline 的 OpenAI Compatible 配置按 Base URL 工作，不要填完整的 /chat/completions 路径。']
  },
  {
    id: 'continue-reference', title: 'Continue 接入教程', kicker: '更多客户端', icon: 'document',
    intro: 'Continue 官方支持 OpenAI 兼容提供商。最常见的接入方式是在 config.yaml 中为 OpenAI provider 增加 apiBase。',
    steps: [{ title: '打开配置文件', body: '打开 Continue 配置文件，在 models 数组内新增一个 OpenAI provider 模型。' }, { title: '填写参数', body: '将 apiBase 指向 https://ai.router.team/v1，填入 API Key，模型名建议先用 gpt-5.5，保存后重载 Continue。' }],
    codes: [{ id: 'continue-reference', label: 'Continue config.yaml 示例', content: referenceCode('name: RouterTeam', 'version: 1.0.0', 'schema: v1', 'models:', '  - name: RouterTeam GPT-5.5', '    provider: openai', '    model: gpt-5.5', '    apiBase: https://ai.router.team/v1', '    apiKey: sk-替换为你的key') }],
    notes: ['Continue 在某些模型上会优先走 /responses 流程；希望严格使用传统 OpenAI 兼容聊天接口时，优先选择 gpt-5.5。']
  },
  {
    id: 'zed-reference', title: 'Zed 接入教程', kicker: '更多客户端', icon: 'terminal',
    intro: 'Zed 官方支持通过 openai_compatible provider 添加自定义模型。你可以通过 UI 添加，也可以直接修改设置文件。',
    steps: [{ title: '添加 Provider', body: '在 Zed 中执行 agent: open settings，在 LLM Providers 区域点击 Add Provider；或者直接编辑设置文件，新增 openai_compatible provider。' }],
    codes: [{ id: 'zed-reference', label: 'settings.json 示例', content: referenceCode('{', '  "language_models": {', '    "openai_compatible": {', '      "RouterTeam": {', '        "api_url": "https://ai.router.team/v1",', '        "api_key": "sk-替换为你的key",', '        "available_models": [{', '          "name": "gpt-5.5",', '          "display_name": "RouterTeam GPT-5.5",', '          "max_tokens": 128000,', '          "max_completion_tokens": 16384', '        }]', '      }', '    }', '  }', '}') }],
    notes: ['如果接入的是更偏 Responses API 的模型，可以继续补充 capabilities 字段；首次配置建议先从最小示例开始。']
  },
  {
    id: 'trae-cn-reference', title: 'TRAE CN 接入教程', kicker: '更多客户端', icon: 'grid',
    intro: '如果当前使用的 TRAE CN 版本提供自定义模型或 OpenAI Compatible provider 入口，可以按下面的方式接入 RouterTeam。',
    steps: [{ title: '找到自定义入口', body: '打开 TRAE CN 设置，找到模型、Provider 或自定义模型相关页面，选择 OpenAI 兼容或自定义 provider 类型。' }, { title: '填写参数', body: 'Base URL 填写 https://ai.router.team/v1；API Key 填入 RouterTeam 密钥；模型 ID 先填写 gpt-5.5，保存后重新打开模型选择器。' }],
    codes: [{ id: 'trae-cn-reference', label: '建议先用这组参数试跑', content: referenceCode('Base URL: https://ai.router.team/v1', '模型 ID: gpt-5.5') }],
    notes: ['TRAE CN 的自定义提供商入口可能会随版本调整。若看不到 Custom / OpenAI Compatible / 自定义模型入口，建议先升级客户端。']
  },
  {
    id: 'codex2claude-reference', title: 'Codex2Claude', kicker: '更多客户端', icon: 'swap',
    intro: '适用于继续使用 Claude Code 客户端，但后端实际走 Codex / GPT-5.5 模型的场景。这套配置使用 Anthropic 兼容入口，因此 Base URL 填根地址即可。',
    steps: [{ title: '创建 Codex 专用密钥', body: '访问 https://ai.router.team，在 API 密钥管理创建一个 Codex 专用密钥，不要使用 Claude Code 分组密钥。' }, { title: '修改 settings.json', body: 'Windows 为 %USERPROFILE%\\.claude\\settings.json，macOS/Linux 为 ~/.claude/settings.json，将下面 JSON 合并到配置中。' }, { title: '重启 Claude Code', body: '保存后完全退出并重启 Claude Code。' }],
    codes: [{ id: 'codex2claude-reference', label: 'Codex2Claude 配置示例', content: referenceCode('{', '  "env": {', '    "ANTHROPIC_API_KEY": "替换为你创建的 Codex key",', '    "ANTHROPIC_BASE_URL": "https://ai.router.team",', '    "ANTHROPIC_DEFAULT_HAIKU_MODEL": "gpt-5.5-low",', '    "ANTHROPIC_DEFAULT_OPUS_MODEL": "gpt-5.5-xhigh-fast",', '    "ANTHROPIC_DEFAULT_SONNET_MODEL": "gpt-5.5-high",', '    "ANTHROPIC_MODEL": "gpt-5.5-xhigh",', '    "ANTHROPIC_REASONING_MODEL": "gpt-5.5-xhigh",', '    "CLAUDE_CODE_ATTRIBUTION_HEADER": "0",', '    "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1",', '    "DISABLE_ERROR_REPORTING": "1",', '    "DISABLE_TELEMETRY": "1",', '    "MCP_TIMEOUT": "60000"', '  },', '  "model": "gpt-5.5-xhigh",', '  "skipDangerousModePermissionPrompt": true', '}') }],
    notes: ['Base URL 使用站点根地址，不要追加 /v1。']
  },
  {
    id: 'opencode-reference', title: 'OpenCode 配置教程', kicker: '更多客户端', icon: 'terminal',
    intro: 'OpenCode 与 Codex 的配置是两套独立步骤，请按下方内容单独配置。',
    steps: [{ title: '安装并登录', body: '安装 OpenCode CLI 并登录，登录时选择 Other，并设置 routerteam 供应商。' }, { title: '编辑配置文件', body: '配置文件路径为 ~/.config/opencode/opencode.json，替换为你的 API Key 与 baseURL。' }, { title: '切换思考强度', body: '使用 Ctrl + T 按 low → medium → high → xhigh → low 循环切换。' }],
    codes: [{ id: 'opencode-install-reference', label: 'Terminal', content: referenceCode('npm install -g opencode-ai', 'opencode auth login') }, { id: 'opencode-reference', label: 'opencode.json', content: referenceCode('{', '  "$schema": "https://opencode.ai/config.json",', '  "provider": {', '    "routerteam": {', '      "npm": "@ai-sdk/openai",', '      "name": "RouterTeam",', '      "options": { "baseURL": "https://ai.router.team", "apiKey": "替换为你的key" },', '      "models": { "gpt-5.5": { "attachment": true, "reasoning": true, "limit": { "context": 1000000, "output": 128000 } } }', '    }', '  }', '}') }, { id: 'opencode-shortcut-reference', label: '快捷键', content: 'Ctrl + T' }],
    notes: ['low：思考最浅；medium：平衡速度与质量；high：更深入分析与规划；xhigh：思考最深，速度更慢、消耗更高。', '建议先用 Tab 切到 Plan 模式梳理需求，再切回 Build 模式写代码。']
  }
)

referenceSections.push(
  {
    id: 'openclaw-reference', title: 'OpenClaw 配置教程', kicker: '更多客户端', icon: 'cloud',
    intro: 'OpenClaw 可通过 OpenAI Responses 兼容方式接入 RouterTeam。',
    steps: [{ title: '安装并初始化', body: '执行 npm i -g openclaw，然后运行 clawdbot onboard。模型供应商配置请选择跳过（Model/auth provider - Skip for now），其他配置可按需填写。' }, { title: '打开 Web 面板', body: '运行 openclaw dashboard，在面板的 Config 中修改 Raw JSON 配置。' }, { title: '保存并更新', body: '修改 models 和 agents.defaults.model 后，点击 Save 和 Update，等待完成后回到首页 chat 对话。' }],
    codes: [{ id: 'openclaw-install-reference', label: 'Terminal', content: referenceCode('npm i -g openclaw', 'clawdbot onboard', 'openclaw dashboard') }, { id: 'openclaw-reference', label: 'clawdbot.json（Codex）', content: referenceCode('{', '  "models": { "mode": "merge", "providers": {', '    "routerteam": {', '      "baseUrl": "https://ai.router.team/v1",', '      "apiKey": "替换为你的key",', '      "auth": "api-key", "api": "openai-responses",', '      "models": [{ "id": "gpt-5.5", "name": "routerteam (gpt-5.5)", "reasoning": true, "input": ["text", "image"], "contextWindow": 200000, "maxTokens": 8192 }]', '    }', '  } },', '  "agents": { "defaults": { "thinkingDefault": "high", "model": { "primary": "routerteam/gpt-5.5" } } }', '}') }, { id: 'openclaw-claude-reference', label: 'clawdbot.json（Claude）', content: referenceCode('{', '  "models": { "providers": {', '    "routerteam-cc": {', '      "baseUrl": "https://ai.router.team/v1", "apiKey": "你的key", "api": "anthropic-messages",', '      "models": [{ "id": "claude-sonnet-4-5-20250929", "name": "Claude 4.5 Sonnet", "contextWindow": 200000 }]', '    }', '  } },', '  "agents": { "defaults": { "model": { "primary": "routerteam-cc/claude-sonnet-4-5-20250929" } } }', '}') }],
    notes: ['thinkingDefault 表示默认思考强度，可改成 medium、high 或 xhigh。']
  },
  {
    id: 'droid-cli-reference', title: 'Droid CLI 配置教程', kicker: '更多客户端', icon: 'terminal',
    intro: 'Droid CLI 读取 ~/.factory/config.json，可在其中添加 custom_models 指向本服务端点。',
    steps: [{ title: '清理旧配置', body: '如果 ~/.factory/settings.json 中存在旧的 routerteam 配置，请先清除后再按本教程配置并重启 Droid CLI。' }, { title: '编辑配置文件', body: '打开 ~/.factory/config.json，添加 custom_models 配置并替换 API Key。' }],
    codes: [{ id: 'droid-cli-reference', label: 'config.json', content: referenceCode('{', '  "custom_models": [{', '    "model_display_name": "GPT-5.5 [routerteam]",', '    "model": "gpt-5.5",', '    "base_url": "https://ai.router.team",', '    "api_key": "替换为你的key",', '    "provider": "openai", "max_tokens": 16384,', '    "extra_args": { "reasoning": { "effort": "high" } }', '  }]', '}') }]
  },
  {
    id: 'cherry-studio-reference', title: 'Cherry Studio 接入教程', kicker: '更多客户端', icon: 'chat',
    intro: '这里只提供 Codex（Openai-Response）接入方式。',
    steps: [{ title: '新增供应商', body: '供应商类型选择 Openai-Response；API 地址填写 https://ai.router.team；API Key 使用后台创建的 sk- 开头密钥。' }, { title: '填写模型', body: '模型 ID 固定填写 gpt-5。推荐地址不加结尾 /，单独以 / 结尾可能导致 v1 被忽略。' }],
    codes: [{ id: 'cherry-studio-reference', label: '关键参数', content: referenceCode('供应商类型: Openai-Response', 'API 地址: https://ai.router.team', 'API Key: sk-替换为你的key', '模型 ID: gpt-5') }]
  },
  {
    id: 'chatbox-reference', title: 'ChatBox 接入教程', kicker: '更多客户端', icon: 'chatBubble',
    intro: '按照以下步骤配置即可完成 ChatBox 的 RouterTeam 接入。',
    steps: [{ title: '添加服务', body: '打开【设置】-【添加】，名称填写 RouterTeam（可自定义），API 模式选择 OpenAI Responses API 兼容。' }, { title: '填写连接', body: 'API 密钥粘贴在 https://ai.router.team/api-keys 创建的 key；API 主机填写 https://ai.router.team；API 路径留空。' }, { title: '创建模型', body: '开启【改善网络兼容性】；新建模型，模型 ID 填写 gpt-5.5，其余保持默认，点击保存模型和检查按钮。' }],
    codes: [{ id: 'chatbox-reference', label: '关键参数', content: referenceCode('API 主机: https://ai.router.team', '模型 ID: gpt-5.5') }]
  },
  {
    id: 'typingmind-reference', title: 'TypingMind 接入教程', kicker: '更多客户端', icon: 'chat',
    intro: 'TypingMind 支持添加自定义模型，也支持给 OpenAI Chat Endpoint 配置自定义代理地址。最稳妥的方式是新建自定义 OpenAI 模型并指向完整聊天端点。',
    steps: [{ title: '添加自定义模型', body: '打开左侧 Models 页面，点击 Add Custom Models；模型名填 RouterTeam GPT-5.5，实际 model ID 填 gpt-5.5。' }, { title: '填写完整端点', body: '在 OpenAI provider 的 Proxy / Custom Endpoint 填写 https://ai.router.team/v1/chat/completions，API Key 填入 RouterTeam 密钥并保存。' }],
    codes: [{ id: 'typingmind-reference', label: 'TypingMind 推荐填写', content: referenceCode('Custom Endpoint: https://ai.router.team/v1/chat/completions', '模型 ID: gpt-5.5') }],
    notes: ['Proxy 字段对应完整聊天端点，而不是仅主机根地址；需要填到 /v1/chat/completions。']
  },
  {
    id: 'boltai-reference', title: 'BoltAI 接入教程', kicker: '更多客户端', icon: 'bolt',
    intro: 'BoltAI 官方支持 OpenAI-compatible Server，更适合填写完整聊天接口 endpoint。',
    steps: [{ title: '新增服务器', body: '打开 BoltAI 设置，进入 AI Providers 或自定义服务器设置，新建 OpenAI-compatible Server。' }, { title: '填写参数', body: 'Endpoint 填写 https://ai.router.team/v1/chat/completions；API Key 填入 RouterTeam key；模型名填写 gpt-5.5，保存后开始对话。' }],
    codes: [{ id: 'boltai-reference', label: 'BoltAI 推荐填写', content: referenceCode('Endpoint: https://ai.router.team/v1/chat/completions', '模型 ID: gpt-5.5') }],
    notes: ['某些版本只填到 /v1 不会自动补全聊天路径。']
  },
  {
    id: 'open-webui-reference', title: 'Open WebUI 接入教程', kicker: '更多客户端', icon: 'globe',
    intro: 'Open WebUI 官方支持连接任何 OpenAI-compatible provider，可以在管理员后台添加新的 OpenAI 连接。',
    steps: [{ title: '添加连接', body: '进入 Admin Settings → Connections → OpenAI，点击 Add Connection。' }, { title: '填写 URL', body: 'URL 填写 https://ai.router.team/v1，API Key 填写你的密钥。' }, { title: '补充模型', body: '如果自动校验没有拉到模型列表，在 Model IDs (Filter) 手动加入 gpt-5.5，点击 Save 后回到聊天页选择模型。' }],
    codes: [{ id: 'open-webui-reference', label: '关键参数', content: referenceCode('URL: https://ai.router.team/v1', 'Model IDs: gpt-5.5') }],
    notes: ['校验连接时会调用 /models。无法返回列表不代表聊天不可用，可手动添加模型 ID。']
  },
  {
    id: 'librechat-reference', title: 'LibreChat 接入教程', kicker: '更多客户端', icon: 'chat',
    intro: 'LibreChat 通过 librechat.yaml 添加 OpenAI API-compatible custom endpoint，在 endpoints.custom 下新增 RouterTeam provider。',
    steps: [{ title: '挂载配置文件', body: '确认 LibreChat 实例已经启用 librechat.yaml 挂载。' }, { title: '添加自定义端点', body: '在 endpoints.custom 下新增 RouterTeam 配置，将 API Key 放进 .env，再重启 LibreChat。' }],
    codes: [{ id: 'librechat-reference', label: 'librechat.yaml 示例', content: referenceCode('version: 1.3.5', 'cache: true', 'endpoints:', '  custom:', '    - name: "RouterTeam"', '      apiKey: "${ROUTERTEAM_KEY}"', '      baseURL: "https://ai.router.team/v1"', '      models:', '        default: ["gpt-5.5"]', '        fetch: true', '      titleConvo: true', '      titleModel: "gpt-5.5"', '      modelDisplayLabel: "RouterTeam"') }, { id: 'librechat-env-reference', label: '.env 示例', content: 'ROUTERTEAM_KEY=sk-替换为你的key' }],
    notes: ['Docker 部署时别忘了把 librechat.yaml 正确挂载进容器。']
  },
  {
    id: 'lobe-chat-reference', title: 'LobeChat 接入教程', kicker: '更多客户端', icon: 'chat',
    intro: 'LobeChat 自托管场景支持通过环境变量覆盖 OpenAI provider 的代理地址和模型列表，常用 OPENAI_PROXY_URL 与 OPENAI_MODEL_LIST。',
    steps: [{ title: '准备环境变量', body: '找到 LobeChat 的部署环境变量文件，设置 OpenAI API Key 为 RouterTeam Key。' }, { title: '设置代理和模型', body: '代理地址设置为 https://ai.router.team/v1，模型列表加入 gpt-5.5，重启 LobeChat。' }],
    codes: [{ id: 'lobe-chat-reference', label: '环境变量示例', content: referenceCode('OPENAI_API_KEY=sk-替换为你的key', 'OPENAI_PROXY_URL=https://ai.router.team/v1', 'OPENAI_MODEL_LIST=gpt-5.5') }],
    notes: ['如果启用了其他 provider，建议先用 RouterTeam 专属 key 只接 OpenAI provider。']
  },
  {
    id: 'nextchat-reference', title: 'NextChat 接入教程', kicker: '更多客户端', icon: 'chat',
    intro: 'NextChat 自托管场景支持通过环境变量覆盖 OpenAI 请求地址和模型列表，常用 BASE_URL 与 CUSTOM_MODELS。',
    steps: [{ title: '打开环境变量', body: '打开 NextChat 部署环境变量，或你自己的 .env.local。' }, { title: '填写地址和模型', body: '将 OpenAI Key 改成 RouterTeam API Key，把 Base URL 指向 https://ai.router.team/v1，在自定义模型列表加入 gpt-5.5，重启应用。' }],
    codes: [{ id: 'nextchat-reference', label: '.env.local 示例', content: referenceCode('OPENAI_API_KEY=sk-替换为你的key', 'BASE_URL=https://ai.router.team/v1', 'CUSTOM_MODELS=gpt-5.5') }],
    notes: ['较新的 NextChat 版本也可以维护自定义模型显示名，第一次接入建议先填最小必要变量。']
  },
  {
    id: 'anythingllm-reference', title: 'AnythingLLM 接入教程', kicker: '更多客户端', icon: 'database',
    intro: 'AnythingLLM 提供 OpenAI (generic) provider，适合接入任意 OpenAI-compatible 服务。',
    steps: [{ title: '选择 Provider', body: '打开管理后台，进入 LLM Provider 设置，Provider 类型选择 OpenAI (generic)。' }, { title: '填写参数', body: 'Base URL 填写 https://ai.router.team/v1，API Key 填入 key，模型 ID 输入 gpt-5.5，保存即可。' }],
    codes: [{ id: 'anythingllm-reference', label: '推荐参数', content: referenceCode('Base URL: https://ai.router.team/v1', '模型 ID: gpt-5.5') }],
    notes: ['如果 AnythingLLM 还负责嵌入模型或 reranker，不要把所有 provider 都改成同一个地址；这里只改主聊天 LLM provider。']
  }
)

const documentationSections = [
  ...sections.slice(0, 4),
  ...referenceSections,
  sections.find((section) => section.id === 'troubleshooting')!
]

const groups = [
  { title: '开始使用', items: documentationSections.slice(0, 4) },
  { title: '核心教程', items: referenceSections.slice(0, 4) },
  { title: '更多客户端', items: referenceSections.slice(4) },
  { title: '帮助', items: [documentationSections[documentationSections.length - 1]] }
]

const activeId = ref('quick-start')
const mobileNavOpen = ref(false)
const copiedId = ref('')
const searchQuery = ref('')
const readingProgress = ref(0)
const isDark = ref(document.documentElement.classList.contains('dark'))
const appStore = useAppStore()
const { copyToClipboard } = useClipboard()
const activeSection = computed(() => documentationSections.find((section) => section.id === activeId.value) ?? documentationSections[0])
const currentSectionIndex = computed(() => Math.max(0, documentationSections.findIndex((section) => section.id === activeSection.value.id)))
const nextSection = computed(() => documentationSections[currentSectionIndex.value + 1] ?? null)
const estimatedReadMinutes = computed(() => {
  const section = activeSection.value
  const contentLength = [
    section.intro,
    ...(section.steps?.flatMap((step) => [step.title, step.body]) ?? []),
    ...(section.codes?.map((code) => code.content) ?? []),
    ...(section.notes ?? [])
  ].join('').length
  return Math.max(2, Math.ceil(contentLength / 450))
})
const filteredGroups = computed(() => {
  const keyword = searchQuery.value.trim().toLocaleLowerCase()
  if (!keyword) return groups
  return groups
    .map((group) => ({
      ...group,
      items: group.items.filter((item) => `${item.title} ${item.intro} ${item.kicker}`.toLocaleLowerCase().includes(keyword))
    }))
    .filter((group) => group.items.length > 0)
})
const siteLogo = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '', {
  allowRelative: true,
  allowDataUrl: true
}))

function selectSection(id: string) {
  activeId.value = id
  mobileNavOpen.value = false
  window.history.replaceState(null, '', `/docs#${id}`)
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

async function copyCode(id: string, content: string) {
  const copied = await copyToClipboard(content, '代码已复制')
  if (copied) {
    copiedId.value = id
    window.setTimeout(() => { if (copiedId.value === id) copiedId.value = '' }, 1800)
  }
}

function toggleTheme() {
  const nextIsDark = !document.documentElement.classList.contains('dark')
  document.documentElement.classList.toggle('dark', nextIsDark)
  localStorage.setItem('theme', nextIsDark ? 'dark' : 'light')
  isDark.value = nextIsDark
}

function scrollTop() {
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

function updateReadingProgress() {
  const maxScroll = document.documentElement.scrollHeight - window.innerHeight
  readingProgress.value = maxScroll > 0
    ? Math.min(100, Math.max(0, Math.round((window.scrollY / maxScroll) * 100)))
    : 100
}

onMounted(() => {
  const hash = window.location.hash.slice(1)
  if (documentationSections.some((section) => section.id === hash)) activeId.value = hash
  updateReadingProgress()
  window.addEventListener('scroll', updateReadingProgress, { passive: true })
  window.addEventListener('resize', updateReadingProgress)
})

onBeforeUnmount(() => {
  window.removeEventListener('scroll', updateReadingProgress)
  window.removeEventListener('resize', updateReadingProgress)
})

watch(activeId, () => {
  copiedId.value = ''
  window.requestAnimationFrame(updateReadingProgress)
})
</script>

<style scoped>
.docs-shell {
  --docs-bg: #f4f7fb;
  --docs-surface: #ffffff;
  --docs-surface-muted: #f7f9fc;
  --docs-border: #dce4ee;
  --docs-border-strong: #c8d4e2;
  --docs-text: #111827;
  --docs-text-soft: #405169;
  --docs-muted: #718096;
  --docs-accent: #1677ff;
  --docs-accent-soft: #eaf3ff;
  --docs-code: #0d1622;
  min-height: 100vh;
  color: var(--docs-text);
  background: var(--docs-bg);
}

.dark .docs-shell {
  --docs-bg: #080d14;
  --docs-surface: #0e1621;
  --docs-surface-muted: #121c29;
  --docs-border: #263548;
  --docs-border-strong: #354960;
  --docs-text: #f3f7fc;
  --docs-text-soft: #b6c3d3;
  --docs-muted: #8191a5;
  --docs-accent: #69b1ff;
  --docs-accent-soft: #102b50;
  --docs-code: #080f18;
}

.docs-header {
  position: sticky;
  top: 0;
  z-index: 30;
  border-bottom: 1px solid var(--docs-border);
  background: var(--docs-surface);
}

.docs-header-inner {
  width: min(1360px, calc(100% - 40px));
  height: 68px;
  margin: 0 auto;
  display: grid;
  grid-template-columns: 240px minmax(0, 1fr) auto;
  align-items: center;
  gap: 28px;
}

.docs-header-brand {
  min-width: 0;
  display: inline-flex;
  align-items: center;
  gap: 10px;
  color: var(--docs-text);
  text-decoration: none;
}

.docs-header-brand img {
  width: 34px;
  height: 34px;
  object-fit: contain;
}

.docs-header-brand > span {
  min-width: 0;
  display: flex;
  flex-direction: column;
  line-height: 1.15;
}

.docs-header-brand strong {
  font-size: 15px;
  font-weight: 800;
}

.docs-header-brand small {
  margin-top: 3px;
  color: var(--docs-muted);
  font-size: 11px;
  font-weight: 600;
}

.docs-header-context {
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--docs-muted);
  font-size: 12px;
}

.docs-header-context strong {
  min-width: 0;
  overflow: hidden;
  color: var(--docs-text-soft);
  font-weight: 700;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.docs-header-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  color: var(--docs-muted);
  font-size: 12px;
}

.docs-reading-label {
  white-space: nowrap;
  font-variant-numeric: tabular-nums;
}

.docs-icon-button {
  width: 36px;
  height: 36px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--docs-border);
  border-radius: 7px;
  background: var(--docs-surface-muted);
  color: var(--docs-text-soft);
  cursor: pointer;
  transition: border-color 160ms ease, color 160ms ease, background 160ms ease;
}

.docs-icon-button:hover {
  border-color: var(--docs-border-strong);
  background: var(--docs-accent-soft);
  color: var(--docs-accent);
}

.docs-login {
  min-height: 36px;
  display: inline-flex;
  align-items: center;
  gap: 7px;
  border-radius: 7px;
  background: #1677ff;
  padding: 0 14px;
  color: #ffffff;
  font-weight: 750;
  text-decoration: none;
}

.dark .docs-login {
  background: #4096ff;
  color: #07101b;
}

.docs-reading-track {
  position: absolute;
  right: 0;
  bottom: -1px;
  left: 0;
  height: 3px;
  overflow: hidden;
  background: transparent;
}

.docs-reading-track span {
  display: block;
  width: 100%;
  height: 100%;
  transform-origin: left center;
  background: #1677ff;
  transition: transform 100ms linear;
}

.dark .docs-reading-track span {
  background: #69b1ff;
}

.docs-layout {
  width: min(1360px, calc(100% - 40px));
  margin: 0 auto;
  display: grid;
  grid-template-columns: 256px minmax(0, 1fr);
  gap: 48px;
}

.docs-sidebar {
  position: sticky;
  top: 84px;
  align-self: start;
  height: calc(100vh - 100px);
  min-height: 520px;
}

.docs-sidebar-shell {
  min-height: 100%;
  border-right: 1px solid var(--docs-border);
  padding: 18px 22px 16px 0;
}

.docs-sidebar-header {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 0 8px 17px;
}

.docs-sidebar-logo {
  width: 36px;
  height: 36px;
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  border-radius: 7px;
}

.docs-sidebar-brand {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.docs-sidebar-brand-title {
  color: var(--docs-text);
  font-size: 15px;
  font-weight: 800;
  text-decoration: none;
}

.docs-sidebar-brand > span {
  color: var(--docs-muted);
  font-size: 11px;
}

.docs-search {
  min-height: 38px;
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0 0 18px;
  border: 1px solid var(--docs-border);
  border-radius: 7px;
  background: var(--docs-surface);
  padding: 0 11px;
  color: var(--docs-muted);
}

.docs-search:focus-within {
  border-color: var(--docs-accent);
  color: var(--docs-accent);
}

.docs-search input {
  min-width: 0;
  width: 100%;
  border: 0;
  outline: 0;
  background: transparent;
  color: var(--docs-text);
  font: inherit;
  font-size: 12px;
}

.docs-search input::placeholder {
  color: var(--docs-muted);
}

.docs-sidebar-nav {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  overscroll-behavior: contain;
  scrollbar-width: thin;
}

.docs-nav-group + .docs-nav-group {
  margin-top: 20px;
}

.docs-nav-group-title {
  margin: 0 0 7px;
  padding: 0 10px;
  color: var(--docs-muted);
  font-size: 10px;
  font-weight: 800;
  letter-spacing: 0;
}

.docs-nav-item {
  position: relative;
  width: 100%;
  min-height: 38px;
  display: flex;
  align-items: center;
  gap: 10px;
  border: 0;
  border-radius: 6px;
  background: transparent;
  padding: 8px 10px;
  color: var(--docs-text-soft);
  font: inherit;
  font-size: 12px;
  line-height: 1.4;
  text-align: left;
  cursor: pointer;
  transition: background 140ms ease, color 140ms ease;
}

.docs-nav-item svg {
  flex-shrink: 0;
}

.docs-nav-item:hover {
  background: var(--docs-surface);
  color: var(--docs-accent);
}

.docs-nav-item.is-active {
  background: var(--docs-accent-soft);
  color: var(--docs-accent);
  font-weight: 750;
}

.docs-nav-item.is-active::before {
  position: absolute;
  top: 8px;
  bottom: 8px;
  left: 0;
  width: 2px;
  border-radius: 1px;
  background: var(--docs-accent);
  content: "";
}

.docs-search-empty {
  padding: 20px 10px;
  color: var(--docs-muted);
  font-size: 12px;
  text-align: center;
}

.docs-sidebar-footer {
  flex-shrink: 0;
  display: flex;
  align-items: flex-start;
  gap: 8px;
  margin-top: 14px;
  border-top: 1px solid var(--docs-border);
  padding: 14px 8px 0;
  color: var(--docs-muted);
  font-size: 10px;
  line-height: 1.6;
}

.docs-sidebar-footer svg {
  flex-shrink: 0;
  margin-top: 2px;
}

.docs-main {
  min-width: 0;
  padding: 30px 0 64px;
}

.docs-mobile-toggle {
  display: none;
}

.docs-breadcrumb {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 34px;
  color: var(--docs-muted);
  font-size: 11px;
}

.docs-breadcrumb strong {
  color: var(--docs-text-soft);
  font-weight: 700;
}

.docs-article {
  max-width: 900px;
  animation: docs-in 220ms ease-out both;
}

@keyframes docs-in {
  from { opacity: 0; transform: translateY(6px); }
  to { opacity: 1; transform: translateY(0); }
}

.docs-article-meta {
  display: flex;
  align-items: center;
  gap: 12px;
  color: var(--docs-muted);
  font-size: 11px;
  font-variant-numeric: tabular-nums;
}

.docs-article-meta span + span {
  padding-left: 12px;
  border-left: 1px solid var(--docs-border);
}

.docs-kicker {
  color: var(--docs-accent);
  font-weight: 850;
  letter-spacing: 0;
}

.docs-article h1 {
  margin: 13px 0 14px;
  color: var(--docs-text);
  font-size: 42px;
  font-weight: 850;
  letter-spacing: 0;
  line-height: 1.15;
}

.docs-lead {
  max-width: 760px;
  margin: 0;
  color: var(--docs-text-soft);
  font-size: 16px;
  line-height: 1.8;
}

.docs-section {
  margin-top: 46px;
}

.docs-section h2 {
  margin: 0 0 18px;
  color: var(--docs-text);
  font-size: 17px;
  font-weight: 800;
}

.docs-steps {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 28px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.docs-steps li {
  min-width: 0;
  display: grid;
  grid-template-columns: 34px minmax(0, 1fr);
  gap: 12px;
  border-top: 1px solid var(--docs-border);
  padding: 18px 0;
}

.docs-step-number {
  width: 28px;
  height: 28px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 6px;
  background: var(--docs-accent-soft);
  color: var(--docs-accent);
  font-size: 10px;
  font-weight: 850;
}

.docs-steps h3 {
  margin: 2px 0 6px;
  color: var(--docs-text);
  font-size: 13px;
  font-weight: 800;
}

.docs-steps p {
  margin: 0;
  color: var(--docs-muted);
  font-size: 13px;
  line-height: 1.7;
}

.docs-media-list {
  display: grid;
  gap: 18px;
}

.docs-media {
  margin: 0;
  overflow: hidden;
  border: 1px solid var(--docs-border);
  border-radius: 8px;
  background: var(--docs-surface);
}

.docs-media img {
  display: block;
  width: 100%;
  max-height: 620px;
  object-fit: contain;
  background: #ffffff;
}

.docs-media figcaption {
  border-top: 1px solid var(--docs-border);
  padding: 11px 14px;
  color: var(--docs-muted);
  font-size: 11px;
  line-height: 1.6;
}

.docs-code-card {
  margin-top: 14px;
  overflow: hidden;
  border: 1px solid var(--docs-border-strong);
  border-radius: 8px;
  background: var(--docs-code);
}

.docs-code-head {
  min-height: 42px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  border-bottom: 1px solid #263747;
  background: #152230;
  padding: 8px 12px 8px 15px;
  color: #a9b8ca;
  font-size: 11px;
}

.docs-code-head > span {
  min-width: 0;
  display: inline-flex;
  align-items: center;
  gap: 7px;
}

.docs-copy {
  min-height: 28px;
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  border: 1px solid #3b5268;
  border-radius: 6px;
  background: #1c3042;
  padding: 0 9px;
  color: #d5e0ec;
  font: inherit;
  font-size: 10px;
  cursor: pointer;
  transition: border-color 140ms ease, background 140ms ease;
}

.docs-copy:hover {
  border-color: #6faeff;
  background: #21446d;
  color: #ffffff;
}

.docs-code-card pre {
  margin: 0;
  overflow-x: auto;
  padding: 18px;
  color: #dce8f5;
  font: 12px/1.75 ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  white-space: pre;
}

.docs-note-list {
  display: grid;
  gap: 9px;
}

.docs-note {
  display: flex;
  gap: 10px;
  border-left: 2px solid var(--docs-accent);
  background: var(--docs-accent-soft);
  padding: 12px 14px;
  color: var(--docs-text-soft);
  font-size: 12px;
  line-height: 1.7;
}

.docs-note p {
  margin: 0;
}

.docs-note svg {
  flex-shrink: 0;
  margin-top: 2px;
  color: var(--docs-accent);
}

.docs-links {
  display: flex;
  flex-wrap: wrap;
  gap: 9px;
}

.docs-links a {
  min-height: 36px;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  border: 1px solid var(--docs-border);
  border-radius: 7px;
  background: var(--docs-surface);
  padding: 0 12px;
  color: var(--docs-text-soft);
  font-size: 12px;
  font-weight: 700;
  text-decoration: none;
  transition: border-color 140ms ease, color 140ms ease;
}

.docs-links a:hover {
  border-color: var(--docs-accent);
  color: var(--docs-accent);
}

.docs-article-footer {
  max-width: 900px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 18px;
  margin-top: 52px;
  border-top: 1px solid var(--docs-border);
  padding-top: 20px;
}

.docs-back-top,
.docs-next {
  border: 0;
  background: transparent;
  font: inherit;
  cursor: pointer;
}

.docs-back-top {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  padding: 8px 0;
  color: var(--docs-muted);
  font-size: 11px;
}

.docs-back-top:hover {
  color: var(--docs-accent);
}

.docs-next {
  display: grid;
  grid-template-columns: auto auto 12px;
  align-items: center;
  gap: 7px;
  padding: 8px 0;
  color: var(--docs-muted);
  font-size: 10px;
  text-align: right;
}

.docs-next strong {
  color: var(--docs-text-soft);
  font-size: 12px;
}

.docs-next:hover strong,
.docs-next:hover svg {
  color: var(--docs-accent);
}

@media (max-width: 980px) {
  .docs-header-inner {
    grid-template-columns: minmax(0, 1fr) auto;
  }

  .docs-header-context {
    display: none;
  }

  .docs-layout {
    display: block;
    width: min(900px, calc(100% - 40px));
  }

  .docs-mobile-toggle {
    width: 100%;
    min-height: 44px;
    display: flex;
    align-items: center;
    gap: 9px;
    margin-top: 16px;
    border: 1px solid var(--docs-border);
    border-radius: 7px;
    background: var(--docs-surface);
    padding: 0 12px;
    color: var(--docs-text-soft);
    font: inherit;
    font-size: 12px;
    font-weight: 700;
    text-align: left;
  }

  .docs-sidebar {
    position: static;
    display: none;
    height: auto;
    min-height: 0;
    max-height: 62vh;
    padding-top: 10px;
  }

  .docs-sidebar.is-open {
    display: block;
  }

  .docs-sidebar-shell {
    border: 1px solid var(--docs-border);
    border-radius: 7px;
    background: var(--docs-surface);
    padding: 14px;
  }

  .docs-sidebar-header,
  .docs-sidebar-footer {
    display: none;
  }

  .docs-search {
    flex-shrink: 0;
  }

  .docs-sidebar-nav {
    max-height: 44vh;
  }

  .docs-main {
    padding-top: 26px;
  }
}

@media (max-width: 640px) {
  .docs-header-inner,
  .docs-layout {
    width: calc(100% - 28px);
  }

  .docs-header-inner {
    height: 60px;
    gap: 12px;
  }

  .docs-header-brand img {
    width: 30px;
    height: 30px;
  }

  .docs-header-brand small,
  .docs-reading-label {
    display: none;
  }

  .docs-header-actions {
    gap: 7px;
  }

  .docs-icon-button {
    width: 34px;
    height: 34px;
  }

  .docs-login {
    min-height: 34px;
    padding: 0 10px;
    font-size: 11px;
  }

  .docs-main {
    padding-bottom: 38px;
  }

  .docs-breadcrumb {
    margin-bottom: 26px;
  }

  .docs-article h1 {
    font-size: 34px;
  }

  .docs-lead {
    font-size: 15px;
  }

  .docs-section {
    margin-top: 36px;
  }

  .docs-steps {
    grid-template-columns: minmax(0, 1fr);
  }

  .docs-code-head {
    align-items: flex-start;
  }

  .docs-code-card pre {
    padding: 14px;
    font-size: 11px;
  }

  .docs-article-footer {
    align-items: flex-start;
    flex-direction: column;
  }

  .docs-next {
    align-self: flex-end;
  }
}

@media (prefers-reduced-motion: reduce) {
  .docs-article {
    animation: none;
  }

  .docs-reading-track span,
  .docs-nav-item,
  .docs-copy {
    transition: none;
  }
}
</style>
