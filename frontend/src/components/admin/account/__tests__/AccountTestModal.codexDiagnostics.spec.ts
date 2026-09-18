import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import AccountTestModal from '../AccountTestModal.vue'

const { getAvailableModelsMock, copyMock } = vi.hoisted(() => ({
  copyMock: vi.fn(),
  getAvailableModelsMock: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      getAvailableModels: getAvailableModelsMock
    }
  }
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copyToClipboard: copyMock
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

const BaseDialogStub = defineComponent({
  name: 'BaseDialog',
  props: { show: { type: Boolean, default: false } },
  template: '<div v-if="show"><slot /><slot name="footer" /></div>'
})

const SelectStub = defineComponent({
  name: 'SelectStub',
  props: {
    modelValue: { type: [String, Number, Boolean, null], default: '' },
    options: { type: Array, default: () => [] },
    valueKey: { type: String, default: 'value' },
    labelKey: { type: String, default: 'label' }
  },
  emits: ['update:modelValue'],
  template: `
    <select
      v-bind="$attrs"
      :value="modelValue"
      @change="$emit('update:modelValue', $event.target.value)"
    >
      <option
        v-for="option in options"
        :key="option[valueKey]"
        :value="option[valueKey]"
      >
        {{ option[labelKey] }}
      </option>
    </select>
  `
})

const TextAreaStub = defineComponent({
  name: 'TextArea',
  props: {
    modelValue: { type: String, default: '' }
  },
  emits: ['update:modelValue'],
  template: `
    <textarea
      v-bind="$attrs"
      :value="modelValue"
      @input="$emit('update:modelValue', $event.target.value)"
    />
  `
})

function buildAccount() {
  return {
    id: 1,
    name: 'OpenAI OAuth',
    platform: 'openai',
    type: 'oauth',
    status: 'active',
    credentials: {},
    extra: {},
    concurrency: 1,
    priority: 1,
    proxy_id: null,
    auto_pause_on_expired: false
  } as any
}

describe('AccountTestModal', () => {
  const originalFetch = global.fetch

  beforeEach(() => {
    copyMock.mockReset()
    getAvailableModelsMock.mockReset()
    getAvailableModelsMock.mockResolvedValue([
      { id: 'gpt-5.4', display_name: 'GPT-5.4' }
    ])
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      body: {
        getReader: () => ({
          read: vi.fn().mockResolvedValue({ done: true, value: undefined })
        })
      }
    } as any)
    localStorage.setItem('auth_token', 'test-token')
  })

  afterEach(() => {
    global.fetch = originalFetch
    localStorage.clear()
  })

  it('posts compact mode for OpenAI compact probe', async () => {
    const wrapper = mount(AccountTestModal, {
      props: {
        show: true,
        account: buildAccount()
      },
      global: {
        stubs: {
          BaseDialog: BaseDialogStub,
          Select: SelectStub,
          TextArea: TextAreaStub,
          Icon: true
        }
      }
    })

    await flushPromises()
    ;(wrapper.vm as any).selectedModelId = 'gpt-5.4'
    ;(wrapper.vm as any).testMode = 'compact'
    await (wrapper.vm as any).startTest()
    await flushPromises()

    expect(global.fetch).toHaveBeenCalledTimes(1)
    const [, options] = (global.fetch as any).mock.calls[0]
    expect(JSON.parse(options.body)).toMatchObject({
      model_id: 'gpt-5.4',
      mode: 'compact'
    })
  })

  it('renders Chat Completions path status from test SSE', async () => {
    const encoder = new TextEncoder()
    const chunks = [
      encoder.encode('data: {"type":"status","text":"已通过 /v1/chat/completions 验证"}\n\n'),
      encoder.encode('data: {"type":"test_complete","success":true}\n\n')
    ]
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      body: {
        getReader: () => ({
          read: vi.fn().mockImplementation(() => Promise.resolve(
            chunks.length > 0
              ? { done: false, value: chunks.shift() }
              : { done: true, value: undefined }
          ))
        })
      }
    } as any)

    const wrapper = mount(AccountTestModal, {
      props: {
        show: true,
        account: buildAccount()
      },
      global: {
        stubs: {
          BaseDialog: BaseDialogStub,
          Select: SelectStub,
          TextArea: TextAreaStub,
          Icon: true
        }
      }
    })

    await flushPromises()
    ;(wrapper.vm as any).selectedModelId = 'gpt-5.4'
    await (wrapper.vm as any).startTest()
    await flushPromises()

    expect(wrapper.text()).toContain('已通过 /v1/chat/completions 验证')
  })
})

function mountDiagnostics(account = buildAccount()) {
  return mount(AccountTestModal, {
    props: { show: true, account },
    global: { stubs: { BaseDialog: BaseDialogStub, Select: SelectStub, TextArea: TextAreaStub, Icon: true } }
  })
}
function mockDiagnosticStream(events: unknown[], truncate = false) {
  const encoded = new TextEncoder().encode(events.map(event => `data: ${JSON.stringify(event)}\n\n`).join('').trimEnd() + (truncate ? '' : '\n\n'))
  // Deliberately split an SSE event across chunks.
  const chunks = [encoded.slice(0, 23), encoded.slice(23)]
  global.fetch = vi.fn().mockResolvedValue({
    ok: true,
    body: { getReader: () => ({ read: vi.fn().mockImplementation(async () => chunks.length ? { done: false, value: chunks.shift() } : { done: true }) }) }
  }) as any
}
const diagnosticResult = (capability: string, status = 'passed', source = 'parent') => ({
  type: 'capability_result',
  data: { capability, status, source, target_url: `${capability === 'websocket' ? 'wss' : 'https'}://relay.example/backend-api/codex/responses`, http_status: capability === 'websocket' ? 101 : 200, duration_ms: 12 }
})

describe('Codex gateway acceptance results', () => {
  const originalFetch = global.fetch
  afterEach(() => { global.fetch = originalFetch })

  it('defaults OAuth tests to gateway mode and copies actual targets and sources', async () => {
    mockDiagnosticStream([
      diagnosticResult('http'), diagnosticResult('websocket'), diagnosticResult('compact'),
      { type: 'test_complete', success: true }
    ], true)
    const wrapper = mountDiagnostics()
    ;(wrapper.vm as any).selectedModelId = 'gpt-5.4'
    await (wrapper.vm as any).startTest()
    expect(JSON.parse((global.fetch as any).mock.calls[0][1].body).mode).toBe('gateway')
    expect((wrapper.vm as any).status).toBe('success')
    expect(wrapper.findAll('[data-capability]')).toHaveLength(3)
    expect(wrapper.text()).toContain('wss://relay.example/backend-api/codex/responses')
    expect(wrapper.text()).toContain('admin.accounts.openai.diagnostics.sources.parent')
    ;(wrapper.vm as any).copyOutput()
    expect(copyMock).toHaveBeenCalledWith(expect.stringContaining('HTTP 101'), expect.any(String))
    expect(copyMock).toHaveBeenCalledWith(expect.stringContaining('sources.parent'), expect.any(String))
    wrapper.unmount()
  })

  it('retains mixed capability results and only finishes on the suite terminal event', async () => {
    mockDiagnosticStream([
      diagnosticResult('http'), diagnosticResult('websocket', 'failed'), diagnosticResult('compact'),
      { type: 'test_complete', success: false }
    ])
    const wrapper = mountDiagnostics()
    ;(wrapper.vm as any).selectedModelId = 'gpt-5.4'
    await (wrapper.vm as any).startTest()
    expect((wrapper.vm as any).status).toBe('error')
    expect((wrapper.vm as any).capabilityResults.map((result: any) => result.status)).toEqual(['passed', 'failed', 'passed'])
    wrapper.unmount()
  })

  it('marks unfinished probes failed when SSE closes without a terminal event', async () => {
    mockDiagnosticStream([diagnosticResult('http', 'passed', 'official'), diagnosticResult('websocket', 'running')])
    const wrapper = mountDiagnostics()
    ;(wrapper.vm as any).selectedModelId = 'gpt-5.4'
    await (wrapper.vm as any).startTest()
    expect((wrapper.vm as any).status).toBe('error')
    expect((wrapper.vm as any).capabilityResults.map((result: any) => result.status)).toEqual(['passed', 'failed', 'failed'])
    expect(wrapper.text()).toContain('admin.accounts.openai.diagnostics.sources.official')
    wrapper.unmount()
  })

  it('does not turn skipped capabilities green even if the terminal event claims success', async () => {
    mockDiagnosticStream([
      diagnosticResult('http', 'skipped'), diagnosticResult('websocket', 'skipped'), diagnosticResult('compact', 'skipped'),
      { type: 'test_complete', success: true }
    ])
    const wrapper = mountDiagnostics()
    ;(wrapper.vm as any).selectedModelId = 'gpt-5.4'
    await (wrapper.vm as any).startTest()
    expect((wrapper.vm as any).status).toBe('error')
    wrapper.unmount()
  })

  it('cancels pending probes and keeps completed results', async () => {
    global.fetch = vi.fn().mockImplementation((_url, options) => new Promise((_resolve, reject) => {
      options.signal.addEventListener('abort', () => reject(new DOMException('Aborted', 'AbortError')))
    })) as any
    const wrapper = mountDiagnostics()
    ;(wrapper.vm as any).selectedModelId = 'gpt-5.4'
    const pending = (wrapper.vm as any).startTest()
    ;(wrapper.vm as any).handleEvent(diagnosticResult('http'))
    ;(wrapper.vm as any).abortStream()
    await pending
    expect((wrapper.vm as any).capabilityResults.map((result: any) => result.status)).toEqual(['passed', 'cancelled', 'cancelled'])
    expect((wrapper.vm as any).status).toBe('idle')
    wrapper.unmount()
  })

  it.each(['apikey', 'grok'])('preserves normal testing for %s accounts', async (type) => {
    const account = buildAccount()
    if (type === 'apikey') account.type = 'apikey'
    else account.platform = 'grok'
    mockDiagnosticStream([{ type: 'test_complete', success: true }])
    const wrapper = mountDiagnostics(account)
    ;(wrapper.vm as any).selectedModelId = 'test-model'
    await (wrapper.vm as any).startTest()
    expect(JSON.parse((global.fetch as any).mock.calls[0][1].body).mode).toBe(type === 'grok' ? 'text' : 'default')
    expect((wrapper.vm as any).status).toBe('success')
    expect(wrapper.findAll('[data-capability]')).toHaveLength(0)
    wrapper.unmount()
  })
})
