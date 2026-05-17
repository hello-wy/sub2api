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

function mountModal(platform = 'openai') {
  return mount(UseKeyModal, {
    props: {
      show: true,
      apiKey: 'sk-test',
      baseUrl: 'https://example.com/v1',
      platform: platform as any
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
  beforeEach(() => {
    copyToClipboard.mockClear()
    copyToClipboard.mockResolvedValue(true)
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
})
