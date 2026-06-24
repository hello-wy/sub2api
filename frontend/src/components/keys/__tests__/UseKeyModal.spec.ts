import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key
  })
}))

const copyToClipboard = vi.fn().mockResolvedValue(true)

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copyToClipboard
  })
}))

import UseKeyModal from '../UseKeyModal.vue'

function mountModal(platform = 'openai') {
  return mount(UseKeyModal, {
    props: {
      show: true,
      apiKey: 'sk-test',
      baseUrl: 'https://example.com/v1',
      platform
    },
    global: {
      stubs: {
        BaseDialog: {
          template: '<div><slot /><slot name="footer" /></div>'
        },
        Icon: {
          template: '<span />'
        }
      }
    }
  })
}

describe('UseKeyModal', () => {
  it('renders safe GPT-5.5 OpenAI Codex setup command', async () => {
    const wrapper = mountModal('openai')
    const codexTab = wrapper.findAll('button').find((button) =>
      button.text().includes('keys.useKeyModal.cliTabs.codexCli')
    )

    expect(codexTab).toBeDefined()
    await codexTab!.trigger('click')
    await nextTick()

    const codeBlocks = wrapper.findAll('pre code').map((code) => code.text())
    const configToml = codeBlocks.find((content) => content.includes('model_provider = "OpenAI"'))

    expect(configToml).toBeDefined()
    expect(configToml).toContain('mkdir -p ~/.codex')
    expect(configToml).toContain('cat > ~/.codex/config.toml')
    expect(configToml).toContain('model = "gpt-5.5"')
    expect(configToml).toContain('review_model = "gpt-5.5"')
    expect(configToml).not.toContain('model = "gpt-5.4"')
    expect(configToml).toContain('"OPENAI_API_KEY": "<YOUR_OPENAI_API_KEY>"')
    expect(configToml).toContain('chmod 600 ~/.codex/auth.json')
    expect(configToml).toContain('ls -la ~/.codex')
    expect(configToml).not.toContain('sk-test')
    expect(configToml).not.toMatch(/^EOF &&/m)
  })

  it('renders GPT-5.5 OpenAI Codex WebSocket setup command', async () => {
    const wrapper = mountModal('openai')

    const wsTab = wrapper.findAll('button').find((button) =>
      button.text().includes('keys.useKeyModal.cliTabs.codexCliWs')
    )

    expect(wsTab).toBeDefined()
    await wsTab!.trigger('click')
    await nextTick()

    const codeBlocks = wrapper.findAll('pre code').map((code) => code.text())
    const configToml = codeBlocks.find((content) => content.includes('supports_websockets = true'))

    expect(configToml).toBeDefined()
    expect(configToml).toContain('model = "gpt-5.5"')
    expect(configToml).toContain('review_model = "gpt-5.5"')
    expect(configToml).not.toContain('model = "gpt-5.4"')
    expect(configToml).toContain('[features]\nresponses_websockets_v2 = true')
    expect(configToml).toContain('"OPENAI_API_KEY": "<YOUR_OPENAI_API_KEY>"')
    expect(configToml).not.toContain('sk-test')
  })

  it('renders GPT-5.4 mini entry in OpenCode config', async () => {
    const wrapper = mountModal('openai')

    const opencodeTab = wrapper.findAll('button').find((button) =>
      button.text().includes('keys.useKeyModal.cliTabs.opencode')
    )

    expect(opencodeTab).toBeDefined()
    await opencodeTab!.trigger('click')
    await nextTick()

    const codeBlocks = wrapper.findAll('pre code')
    const fileBlock = codeBlocks[codeBlocks.length - 1]
    expect(fileBlock.exists()).toBe(true)
    expect(fileBlock.text()).toContain('"name": "GPT-5.4 Mini"')
    expect(fileBlock.text()).not.toContain('"name": "GPT-5.4 Nano"')
  })

  it('renders shell command section above file displays', () => {
    const wrapper = mountModal('openai')
    const text = wrapper.text()

    expect(text).toContain('Shell 一键命令')
    expect(text.indexOf('Shell 一键命令')).toBeLessThan(text.indexOf('~/.claude/settings.json'))
    expect(wrapper.find('pre code').text()).toContain('mkdir -p "~/.claude"')
  })

  it('copies shell command and displays copied state', async () => {
    const wrapper = mountModal('openai')
    const button = wrapper.findAll('button').find((item) => item.text().includes('keys.useKeyModal.copy'))

    expect(button).toBeDefined()
    await button!.trigger('click')
    await nextTick()

    expect(copyToClipboard).toHaveBeenCalledWith(expect.stringContaining('settings.json'), 'keys.copied')
    expect(wrapper.text()).toContain('keys.useKeyModal.copied')
  })

  it('regenerates shell command when shell tab changes', async () => {
    const wrapper = mountModal('openai')
    expect(wrapper.find('pre code').text()).toContain('mkdir -p')

    const cmdTab = wrapper.findAll('button').find((button) => button.text().includes('Windows CMD'))
    await cmdTab!.trigger('click')
    await nextTick()

    expect(wrapper.find('pre code').text()).toContain('if not exist "%userprofile%\\.claude"')
  })

  it('orders openai client tabs with Claude Code first', () => {
    const wrapper = mountModal('openai')
    const clientLabels = wrapper.findAll('button')
      .map((button) => button.text())
      .filter((text) => text.includes('keys.useKeyModal.cliTabs'))

    expect(clientLabels).toEqual([
      'keys.useKeyModal.cliTabs.claudeCode',
      'keys.useKeyModal.cliTabs.codexCli',
      'keys.useKeyModal.cliTabs.codexCliWs',
      'keys.useKeyModal.cliTabs.opencode'
    ])
  })

  it('shows OS/Shell tabs for OpenCode', async () => {
    const wrapper = mountModal('openai')
    const opencodeTab = wrapper.findAll('button').find((button) =>
      button.text().includes('keys.useKeyModal.cliTabs.opencode')
    )

    await opencodeTab!.trigger('click')
    await nextTick()

    expect(wrapper.text()).toContain('macOS / Linux')
    expect(wrapper.text()).toContain('Windows CMD')
    expect(wrapper.text()).toContain('PowerShell')
    expect(wrapper.find('pre code').text()).toContain('~/.config/opencode')
  })

  it('renders Claude Fable 5 OpenCode config with adaptive thinking', async () => {
    const wrapper = mount(UseKeyModal, {
      props: {
        show: true,
        apiKey: 'sk-test',
        baseUrl: 'https://example.com/v1',
        platform: 'antigravity'
      },
      global: {
        stubs: {
          BaseDialog: {
            template: '<div><slot /><slot name="footer" /></div>'
          },
          Icon: {
            template: '<span />'
          }
        }
      }
    })

    const opencodeTab = wrapper.findAll('button').find((button) =>
      button.text().includes('keys.useKeyModal.cliTabs.opencode')
    )

    expect(opencodeTab).toBeDefined()
    await opencodeTab!.trigger('click')
    await nextTick()

    const claudeConfig = wrapper.findAll('pre code')
      .map((code) => code.text())
      .find((content) => content.trimStart().startsWith('{') && content.includes('"antigravity-claude"'))

    expect(claudeConfig).toBeDefined()
    const parsed = JSON.parse(claudeConfig!)
    const fable = parsed.provider['antigravity-claude'].models['claude-fable-5']

    expect(fable.name).toBe('Claude Fable 5')
    expect(fable.limit).toEqual({ context: 1048576, output: 128000 })
    expect(fable.options.thinking).toEqual({ type: 'adaptive' })
    expect(fable.options.thinking).not.toHaveProperty('budgetTokens')
  })
})
