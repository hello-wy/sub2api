import { afterEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, mount } from '@vue/test-utils'
import AccountProviderFilters from '../AccountProviderFilters.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
enableAutoUnmount(afterEach)

describe('AccountProviderFilters', () => {
  it('only offers platforms with accounts and follows selections and count updates', async () => {
    const wrapper = mount(AccountProviderFilters, { props: { modelValue: '', platformCounts: [] } })
    expect(wrapper.find('[role="group"]').exists()).toBe(false)
    await wrapper.setProps({ platformCounts: [
      { platform: 'openai', count: 35 }, { platform: 'deepseek', count: 2 },
      { platform: 'anthropic', count: 0 }
    ] })
    const buttons = wrapper.findAll('button')
    expect(buttons).toHaveLength(3)
    expect(buttons[0].text()).toContain('37')
    expect(buttons[1].text()).toContain('OpenAI')
    expect(buttons[2].text()).toContain('DeepSeek')
    expect(buttons[1].getComponent(PlatformIcon).props('platform')).toBe('openai')
    expect(buttons[2].get('span[aria-hidden]').attributes('style')).toContain('rgb(77, 107, 254)')
    await buttons[1].trigger('click')
    expect(wrapper.emitted('update:modelValue')).toEqual([['openai']])
    expect(wrapper.emitted('change')).toHaveLength(1)
    await wrapper.setProps({ modelValue: 'openai' })
    expect(buttons[1].attributes('aria-pressed')).toBe('true')
    await buttons[1].trigger('click')
    expect(wrapper.emitted('change')).toHaveLength(1)
    await wrapper.setProps({ modelValue: 'deepseek' })
    expect(buttons[2].attributes('aria-pressed')).toBe('true')
    await buttons[0].trigger('click')
    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual([''])
    expect(wrapper.emitted('change')).toHaveLength(2)
    await wrapper.setProps({ platformCounts: [{ platform: 'openai', count: 35 }] })
    expect(wrapper.findAll('button')).toHaveLength(2)
    expect(wrapper.findAllComponents(PlatformIcon).map(icon => icon.props('platform'))).toEqual(['openai'])
  })
})
