import { describe, expect, it, vi, beforeEach } from 'vitest'
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

describe('UseKeyModal', () => {
  it('renders GPT-5.5 and goals feature in OpenAI Codex config', () => {
    const wrapper = mount(UseKeyModal, {
      props: {
        show: true,
        apiKey: 'sk-test',
        baseUrl: 'https://example.com/v1',
        platform: 'openai'
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

    const codeBlocks = wrapper.findAll('pre code').map((code) => code.text())
    const configToml = codeBlocks.find((content) => content.includes('model_provider = "OpenAI"'))

    expect(configToml).toBeDefined()
    expect(configToml).toContain('model = "gpt-5.5"')
    expect(configToml).toContain('review_model = "gpt-5.5"')
    expect(configToml).not.toContain('model = "gpt-5.4"')
    expect(configToml).not.toContain('model_context_window')
    expect(configToml).not.toContain('model_auto_compact_token_limit')
    expect(configToml).toContain('[features]\ngoals = true')
  })

  it('renders GPT-5.5 and goals feature in OpenAI Codex WebSocket config', async () => {
    const wrapper = mount(UseKeyModal, {
      props: {
        show: true,
        apiKey: 'sk-test',
        baseUrl: 'https://example.com/v1',
        platform: 'openai'
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
    expect(configToml).not.toContain('model_context_window')
    expect(configToml).not.toContain('model_auto_compact_token_limit')
    expect(configToml).toContain('[features]\nresponses_websockets_v2 = true\ngoals = true')
  })

  it('renders GPT-5.4 mini entry in OpenCode config', async () => {
    const wrapper = mount(UseKeyModal, {
      props: {
        show: true,
        apiKey: 'sk-test',
        baseUrl: 'https://example.com/v1',
        platform: 'openai'
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
})
