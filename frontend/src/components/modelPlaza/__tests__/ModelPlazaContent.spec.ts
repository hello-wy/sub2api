import { mount } from '@vue/test-utils'
import { describe, it, expect, vi } from 'vitest'
import { createPinia } from 'pinia'
import { ref } from 'vue'
import ModelPlazaContent from '../ModelPlazaContent.vue'
import { group, model } from './fixtures'
import { unknownPlazaStatus } from '@/utils/model-plaza'

vi.mock('vue-i18n', async () => ({
  ...await vi.importActual('vue-i18n'),
  useI18n: () => ({ t: (key: string) => key }),
}))
vi.mock('@/composables/useClipboard', () => ({ useClipboard: () => ({ copied:ref(false), copyToClipboard:vi.fn() }) }))
vi.mock('@/composables/usePlazaStatus', () => ({
  usePlazaStatus: () => ({ statusFor: () => unknownPlazaStatus(), failed:ref(false), loading:ref(false), enabled:ref(false), mode:ref('v2') }),
}))
const render = (groups = [group()]) => mount(ModelPlazaContent, { props:{ response:{ description:'', groups }, loading:false, embedded:true }, global:{ plugins:[createPinia()], stubs:{ teleport:true } } })
describe('model plaza gallery', () => {
  it('switches the whole gallery between paid and official prices without simultaneous amounts', async () => {
    const wrapper = render()
    expect(wrapper.get('.price-pair').text()).toContain('$1.00')
    expect(wrapper.get('.price-pair').text()).not.toContain('$3.00')
    await wrapper.findAll('.price-switch button')[1].trigger('click')
    expect(wrapper.get('.price-pair').text()).toContain('$3.00')
    expect(wrapper.get('.price-pair').text()).not.toContain('$1.00')
    expect(wrapper.get('.model-prices').attributes('data-price-mode')).toBe('official')
    wrapper.unmount()
  })
  it('combines provider and search filters and resets them together', async () => {
    const wrapper = render([group(),group({ id:2, name:'Claude', platform:'anthropic', models:[model({ name:'claude-test', platform:'anthropic' })] })])
    const providers = wrapper.findAll('.plaza-filter-section')[0]
    // Filter by actual option title from platformLabel, independent of localized vendor labels.
    const anthropic = providers.findAll('.filter-options button')[1]
    await anthropic.trigger('click')
    expect(wrapper.findAll('.plaza-model-card')).toHaveLength(1)
    await wrapper.get('input[type="search"]').setValue('missing')
    expect(wrapper.findAll('.plaza-model-card')).toHaveLength(0)
    await wrapper.get('.reset-filters').trigger('click')
    expect(wrapper.findAll('.plaza-model-card')).toHaveLength(2)
    wrapper.unmount()
  })
  it('expands overflowing double-column provider filters', async () => {
    const wrapper = render(Array.from({ length:12 },(_,i) => group({ id:i, models:[model({ platform:'vendor-'+i })] })))
    const section = wrapper.findAll('.plaza-filter-section')[0]
    expect(section.findAll('.filter-options button')).toHaveLength(10)
    await section.get('.filter-more').trigger('click')
    expect(section.findAll('.filter-options button')).toHaveLength(13)
    expect(section.get('.filter-more').attributes('aria-expanded')).toBe('true')
    wrapper.unmount()
  })
  it('renders no-data status without invented availability and honors loading/error recovery', async () => {
    const wrapper = render()
    expect(wrapper.get('.plaza-status-footer strong').text()).toBe('—')
    await wrapper.setProps({ error:true })
    await wrapper.get('.plaza-empty button').trigger('click')
    expect(wrapper.emitted('retry')).toHaveLength(1)
    await wrapper.setProps({ loading:true })
    expect(wrapper.findAll('.model-skeleton')).toHaveLength(6)
    wrapper.unmount()
  })
  it('sanitizes administrator billing notes and keeps them available behind help', async () => {
    const wrapper = render()
    await wrapper.setProps({ response:{ groups:[group()], description:'<img src=x onerror="alert(1)"> **Billing notes**' } })
    await wrapper.get('.billing-help-button').trigger('click')
    expect(wrapper.get('.plaza-description').html()).not.toContain('onerror')
    expect(wrapper.get('.plaza-description').text()).toContain('Billing notes')
    wrapper.unmount()
  })
})
