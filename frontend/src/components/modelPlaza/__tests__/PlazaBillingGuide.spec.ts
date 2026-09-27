import { mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { nextTick } from 'vue'
import PlazaBillingGuide from '../PlazaBillingGuide.vue'
import { group, model } from './fixtures'

vi.mock('vue-i18n', async () => {
  const { default: zh } = await import('@/i18n/locales/zh')
  return { useI18n: () => ({ t: (key: string) => key.split('.').reduce<unknown>((value, part) => (value as Record<string, unknown>)?.[part], zh) ?? key }) }
})
beforeEach(() => {
  vi.spyOn(window, 'matchMedia').mockImplementation(query => ({ matches:false, media:query, onchange:null, addListener:vi.fn(), removeListener:vi.fn(), addEventListener:vi.fn(), removeEventListener:vi.fn(), dispatchEvent:vi.fn() }))
})
const entries = (g = group()) => g.models.map((m, i) => ({ key:String(i), model:m, group:g }))
const render = (rows = entries()) => mount(PlazaBillingGuide, {
  props:{ show:true, entries:rows },
  global:{ stubs:{ teleport:true } },
})
afterEach(() => { vi.useRealTimers(); vi.restoreAllMocks() })
describe('billing guide', () => {
  it('uses configured base prices and the personal multiplier without applying it twice', async () => {
    const wrapper = render()
    const values = () => wrapper.findAll('.example-step strong').map(e => e.text())
    expect(values()).toEqual(['$2.00 / 1M tokens','× 0.5','$1.00 / 1M tokens'])
    await wrapper.get('.guide-next').trigger('click')
    expect(values()[1]).toBe('$0.01 + $0.008')
    expect(values()[2]).toBe('$0.018')
    await wrapper.get('.guide-next').trigger('click')
    expect(wrapper.emitted('close')).toHaveLength(1)
    wrapper.unmount()
  })
  it('uses independent image multipliers and count units for image billing', async () => {
    const g = group({ image_rate_independent:true, image_rate_multiplier:.25, models:[model({ pricing:{ ...model().pricing!, billing_mode:'image', per_request_price:.2 } })] })
    const wrapper = render(entries(g))
    expect(wrapper.findAll('.example-step strong')[2].text()).toContain('$0.05')
    expect(wrapper.findAll('.example-step strong')[2].text()).not.toContain('tokens')
    await wrapper.get('.guide-next').trigger('click')
    expect(wrapper.findAll('.example-step strong')[2].text()).toBe('$0.50')
    wrapper.unmount()
  })
  it('never presents missing prices as free usage and handles an empty catalog', async () => {
    const wrapper = render(entries(group({ models:[model({ pricing:null })] })))
    await wrapper.get('.guide-next').trigger('click')
    expect(wrapper.findAll('.example-step strong')[2].text()).toBe('-')
    await wrapper.setProps({ entries:[] })
    expect(wrapper.findAll('.example-step')).toHaveLength(0)
    expect(wrapper.text()).toContain('暂无可用模型')
    wrapper.unmount()
  })
  it('highlights matching formula and example steps, stops after one pass and cleans up on close', async () => {
    vi.useFakeTimers()
    const wrapper = render()
    expect(wrapper.findAll('.formula-step')[0].classes()).toContain('highlighted')
    vi.advanceTimersByTime(2000); await nextTick()
    expect(wrapper.findAll('.formula-step')[1].classes()).toContain('highlighted')
    expect(wrapper.findAll('.example-step')[1].classes()).toContain('highlighted')
    vi.advanceTimersByTime(10000); await nextTick()
    expect(wrapper.findAll('.formula-step')[2].classes()).toContain('highlighted')
    await wrapper.get('.guide-replay').trigger('click')
    expect(wrapper.findAll('.formula-step')[0].classes()).toContain('highlighted')
    await wrapper.setProps({ show:false })
    expect(vi.getTimerCount()).toBe(0)
    wrapper.unmount()
  })
})
