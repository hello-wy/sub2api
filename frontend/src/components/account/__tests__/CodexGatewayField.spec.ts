import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createI18n } from 'vue-i18n'
import CodexGatewayField from '../CodexGatewayField.vue'

const { listGateways } = vi.hoisted(() => ({ listGateways: vi.fn() }))
vi.mock('@/api/admin/accounts', () => ({ listCodexGateways: listGateways }))

const i18n = createI18n({ legacy: false, locale: 'en', missingWarn: false, fallbackWarn: false })
const wrappers: ReturnType<typeof mount>[] = []
function render(props: { active?: boolean; inherited?: boolean; modelValue?: string } = {}) {
  const wrapper = mount(CodexGatewayField, {
    props: { active: true, modelValue: '', ...props },
    attachTo: document.body,
    global: { plugins: [i18n] }
  })
  wrappers.push(wrapper)
  return wrapper
}

beforeEach(() => {
  listGateways.mockReset().mockResolvedValue(['https://relay.example/backend-api/codex'])
})
afterEach(() => {
  wrappers.splice(0).forEach(wrapper => wrapper.unmount())
  document.body.innerHTML = ''
})

describe('CodexGatewayField', () => {
  it('selects server history, accepts a new URL, and restores the official endpoint', async () => {
    const wrapper = render()
    await flushPromises()
    await wrapper.get('button[aria-haspopup]').trigger('click')
    await flushPromises()
    const option = Array.from(document.querySelectorAll('[role="option"]'))
      .find(el => el.textContent?.includes('https://relay.example')) as HTMLElement
    option.click()
    await flushPromises()
    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual(['https://relay.example/backend-api/codex'])

    await wrapper.setProps({ modelValue: 'https://relay.example/backend-api/codex' })
    await wrapper.get('button[aria-haspopup]').trigger('click')
    await flushPromises()
    const input = document.querySelector('.select-search-input') as HTMLInputElement
    input.value = 'https://new.example/codex'
    input.dispatchEvent(new Event('input', { bubbles: true }))
    await flushPromises()
    ;(document.querySelector('[role="option"]') as HTMLElement).click()
    await flushPromises()
    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual(['https://new.example/codex'])

    await wrapper.setProps({ modelValue: 'https://new.example/codex' })
    await wrapper.get('.select-clear').trigger('click')
    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual([''])
  })

  it('reloads history on reopening without changing the selected account value', async () => {
    const wrapper = render({ modelValue: 'https://current.example/codex' })
    await flushPromises()
    expect(wrapper.text()).toContain('https://current.example/codex')
    await wrapper.setProps({ active: false })
    await wrapper.setProps({ active: true })
    await flushPromises()
    expect(listGateways).toHaveBeenCalledTimes(2)
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
  })

  it('keeps the field usable when history fails and permits retry', async () => {
    listGateways.mockRejectedValueOnce(new Error('offline'))
    const wrapper = render()
    await flushPromises()
    expect(wrapper.text()).toContain('admin.accounts.codexGateway.retry')
    await wrapper.get('button:not([aria-haspopup])').trigger('click')
    await flushPromises()
    expect(listGateways).toHaveBeenCalledTimes(2)
    expect(wrapper.text()).not.toContain('admin.accounts.codexGateway.retry')
  })

  it('shows inheritance instead of editing a shadow account gateway', async () => {
    const wrapper = render({ inherited: true })
    await flushPromises()
    expect(listGateways).not.toHaveBeenCalled()
    expect(wrapper.find('button').exists()).toBe(false)
    expect(wrapper.text()).toContain('admin.accounts.codexGateway.inherited')
  })
})
