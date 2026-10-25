import { mount } from '@vue/test-utils'
import { beforeEach, describe, it, expect, vi } from 'vitest'
import { createPinia } from 'pinia'
import { useAppStore } from '@/stores/app'
import { ref } from 'vue'
import ModelPlazaContent from '../ModelPlazaContent.vue'
import { group, model } from './fixtures'
import { unknownPlazaStatus } from '@/utils/model-plaza'
import type { PlazaModel, ModelPlazaGroup } from '@/api/modelPlaza'

const { statusFor } = vi.hoisted(() => ({ statusFor: vi.fn() }))

vi.mock('vue-i18n', async () => ({
  ...await vi.importActual('vue-i18n'),
  useI18n: () => ({ t: (key: string) => key }),
}))
vi.mock('@/composables/useClipboard', () => ({ useClipboard: () => ({ copied:ref(false), copyToClipboard:vi.fn() }) }))
vi.mock('@/composables/usePlazaStatus', () => ({
  usePlazaStatus: () => ({ statusFor, failed:ref(false), loading:ref(false), enabled:ref(false), mode:ref('v2') }),
}))
const render = (groups = [group()]) => mount(ModelPlazaContent, { props:{ response:{ description:'', groups }, loading:false, embedded:true }, global:{ plugins:[createPinia()], stubs:{ teleport:true } } })
describe('model plaza gallery', () => {
  beforeEach(() => { statusFor.mockReset(); statusFor.mockReturnValue(unknownPlazaStatus()) })
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
  it('uses the same labeled billing multiplier across groups and hides it for official prices', async () => {
    const wrapper = render([group({ user_rate_multiplier:0.1 }), group({ id:2, user_rate_multiplier:1 })])
    expect(wrapper.get('.rate-tag').text()).toBe('modelPlaza.filters.rateLabel 0.1×')
    expect(wrapper.get('.rate-tag').attributes('title')).toBe('modelPlaza.gallery.rateHint')
    expect(wrapper.find('.discount-tag').exists()).toBe(false)
    await wrapper.get('.card-group-picker select').setValue(JSON.stringify([2,'openai','gpt-test']))
    expect(wrapper.get('.rate-tag').text()).toBe('modelPlaza.filters.rateLabel 1×')
    await wrapper.findAll('.price-switch button')[1].trigger('click')
    expect(wrapper.find('.rate-tag').exists()).toBe(false)
    wrapper.unmount()
  })
  it('keeps the billing multiplier label when official price ratios differ', () => {
    const inconsistent = model()
    inconsistent.official_pricing!.output_price = 16e-6
    const wrapper = render([group({ models:[inconsistent] })])
    expect(wrapper.find('.discount-tag').exists()).toBe(false)
    expect(wrapper.get('.rate-tag').text()).toBe('modelPlaza.filters.rateLabel 0.5×')
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
  it('collapses the same platform model across groups and preserves group-specific views', async () => {
    const wrapper = render([
      group({ name:'OpenAI standard', user_rate_multiplier:.5 }),
      group({ id:2, name:'OpenAI premium', user_rate_multiplier:2, models:[model({ name:'gpt-test' })] }),
    ])
    expect(wrapper.findAll('.plaza-model-card')).toHaveLength(1)
    expect(wrapper.get('.group-coverage').exists()).toBe(true)
    expect(wrapper.get('.model-group').text()).toContain('OpenAI standard')
    const groups = wrapper.get('[aria-label="modelPlaza.gallery.groups"]')
    await groups.get('button[title="OpenAI premium"]').trigger('click')
    expect(wrapper.findAll('.plaza-model-card')).toHaveLength(1)
    expect(wrapper.get('.model-group').text()).toContain('OpenAI premium')
    expect(wrapper.find('.group-coverage').exists()).toBe(false)
    wrapper.unmount()
  })
  it('counts unique models and keeps the same model name on different platforms separate', () => {
    const wrapper = render([
      group(), group({ id:2 }),
      group({ id:3, platform:'anthropic', models:[model({ platform:'anthropic' })] }),
    ])
    expect(wrapper.findAll('.plaza-model-card')).toHaveLength(2)
    const providers = wrapper.get('[aria-label="modelPlaza.gallery.providers"]')
    expect(providers.get('button[title="modelPlaza.gallery.allProviders"] .option-count').text()).toBe('2')
    expect(providers.get('button[title="OpenAI"] .option-count').text()).toBe('1')
    wrapper.unmount()
  })
  it('switches price and exact group status together, including insufficient-sample metrics', async () => {
    statusFor.mockImplementation((_model: PlazaModel, g: ModelPlazaGroup) => g.id === 2
      ? { ...unknownPlazaStatus(), health:'critical', latency:2500, availability:80 }
      : { ...unknownPlazaStatus(), throughput:1 })
    const wrapper = render([group({ name:'Standard' }), group({ id:2, name:'Premium', user_rate_multiplier:2 })])
    const card = wrapper.get('.plaza-model-card')
    const picker = card.get('.card-group-picker select')
    expect(picker.element.value).toBe(JSON.stringify([1,'openai','gpt-test']))
    expect(card.get('.price-pair').text()).toContain('$1.00')
    expect(card.get('.model-health').classes()).toContain('unknown')
    expect(wrapper.getComponent({ name:'PlazaModelCard' }).props('variants')).toHaveLength(2)
    await picker.setValue(JSON.stringify([2,'openai','gpt-test']))
    expect(card.get('.price-pair').text()).toContain('$4.00')
    expect(card.get('.model-health').classes()).toContain('critical')
    expect(card.get('.plaza-status-footer strong').text()).toBe('80.0%')
    wrapper.unmount()
  })
  it('selects the lowest displayed base price instead of assuming the lowest multiplier is cheapest', () => {
    const expensive = model()
    expensive.pricing!.input_price = 20e-6
    const wrapper = render([
      group({ name:'Low multiplier', user_rate_multiplier:.1, models:[expensive] }),
      group({ id:2, name:'Lower unit price', user_rate_multiplier:.5 }),
    ])
    expect(wrapper.get('.card-group-picker select').element.value).toBe(JSON.stringify([2,'openai','gpt-test']))
    expect(wrapper.get('.price-pair').text()).toContain('$1.00')
    wrapper.unmount()
  })
  it('opens group comparisons from the selected card variant and resets that selection on reopening', async () => {
    const wrapper = render([group({ name:'Standard' }),group({ id:2, name:'Premium', user_rate_multiplier:2 })])
    await wrapper.get('.card-group-picker select').setValue(JSON.stringify([2,'openai','gpt-test']))
    await wrapper.get('.plaza-model-card').trigger('click')
    expect(wrapper.get('.detail-pricing h3').text()).toBe('Premium')
    expect(wrapper.get('.detail-price-table').text()).toContain('$4.00')
    const buttons = wrapper.findAll('.detail-group-button')
    await buttons[0].trigger('click')
    expect(wrapper.get('.detail-pricing h3').text()).toBe('Standard')
    await wrapper.get('.modal-overlay button[aria-label="Close modal"]').trigger('click')
    expect(wrapper.find('.plaza-model-details').exists()).toBe(false)
    await wrapper.get('.plaza-model-card').trigger('click')
    expect(wrapper.get('.detail-pricing h3').text()).toBe('Premium')
    wrapper.unmount()
  })
  it('keeps copy and group controls independent while the card and model name open details', async () => {
    const wrapper = render([group({ name:'Standard' }),group({ id:2, name:'Premium' })])
    expect(wrapper.find('.model-details-button').exists()).toBe(false)
    await wrapper.get('.copy-model').trigger('click')
    expect(wrapper.find('.plaza-model-details').exists()).toBe(false)
    const picker = wrapper.get('.card-group-picker select')
    await picker.trigger('click')
    await picker.setValue(JSON.stringify([2,'openai','gpt-test']))
    expect(wrapper.find('.plaza-model-details').exists()).toBe(false)
    await wrapper.get('.model-identity h3 button').trigger('click')
    expect(wrapper.get('.detail-pricing h3').text()).toBe('Premium')
    await wrapper.get('.modal-overlay button[aria-label="Close modal"]').trigger('click')
    await wrapper.get('.model-prices').trigger('click')
    expect(wrapper.get('.detail-pricing h3').text()).toBe('Premium')
    wrapper.unmount()
  })
  it('updates the provider banner and unique model count as filters change', async () => {
    const wrapper = render([
      group(), group({ id:2 }),
      group({ id:3, platform:'anthropic', models:[model({ name:'claude-test', platform:'anthropic' })] }),
    ])
    const banner = wrapper.getComponent({ name:'PlazaProviderBanner' })
    expect(banner.props()).toMatchObject({ platform:'all', count:2 })
    const providers = wrapper.get('[aria-label="modelPlaza.gallery.providers"]')
    await providers.get('button[title="OpenAI"]').trigger('click')
    expect(banner.props()).toMatchObject({ platform:'openai', title:'OpenAI', count:1 })
    expect(wrapper.get('.provider-banner-logo').getComponent({ name:'PlatformIcon' }).props('platform')).toBe('openai')
    expect(wrapper.get('.plaza-catalog-header input[type="search"]').exists()).toBe(true)
    await wrapper.get('input[type="search"]').setValue('no matching model')
    expect(banner.props('count')).toBe(0)
    await wrapper.get('.reset-filters').trigger('click')
    expect(banner.props()).toMatchObject({ platform:'all', count:2 })
    wrapper.unmount()
  })
  it('refreshes monitoring data inside an open detail panel without changing the selected group', async () => {
    const availability = ref(90)
    statusFor.mockImplementation(() => ({ ...unknownPlazaStatus(), health:'healthy', availability:availability.value }))
    const wrapper = render([group({ name:'Standard' }),group({ id:2, name:'Premium', user_rate_multiplier:2 })])
    await wrapper.get('.plaza-model-card').trigger('click')
    await wrapper.findAll('.detail-group-button')[1].trigger('click')
    expect(wrapper.get('.detail-monitor strong').text()).toBe('90.0%')
    availability.value = 99
    await wrapper.vm.$nextTick()
    expect(wrapper.get('.detail-monitor strong').text()).toBe('99.0%')
    expect(wrapper.get('.detail-pricing h3').text()).toBe('Premium')
    wrapper.unmount()
  })
  it('sorts displayed prices in the selected price mode and supports list view', async () => {
    const cheap = model({ name:'z-cheap' })
    const expensive = model({ name:'a-expensive' })
    expensive.pricing!.input_price = 10e-6
    cheap.official_pricing!.input_price = 30e-6
    const wrapper = render([group({ models:[expensive,cheap] })])
    await wrapper.get('.sort-control select').setValue('price')
    expect(wrapper.findAll('.model-identity h3')[0].text()).toBe('z-cheap')
    await wrapper.findAll('.price-switch button')[1].trigger('click')
    expect(wrapper.findAll('.model-identity h3')[0].text()).toBe('a-expensive')
    await wrapper.get('button[aria-label="modelPlaza.gallery.list"]').trigger('click')
    expect(wrapper.get('.plaza-model-card').classes()).toContain('model-list-row')
    wrapper.unmount()
  })
  it('limits groups and rates to the selected provider, including matching models in composite groups', async () => {
    const wrapper = render([
      group({ name:'OpenAI only', user_rate_multiplier:.5 }),
      group({ id:2, name:'Claude only', platform:'anthropic', user_rate_multiplier:2, models:[model({ name:'claude-only', platform:'anthropic' })] }),
      group({ id:3, name:'Mixed', platform:'composite', user_rate_multiplier:1, models:[model({ name:'gpt-mixed' }),model({ name:'claude-mixed', platform:'anthropic' })] }),
      group({ id:4, name:'Empty', models:[] }),
    ])
    const providers = wrapper.get('[aria-label="modelPlaza.gallery.providers"]')
    const groups = wrapper.get('[aria-label="modelPlaza.gallery.groups"]')
    const names = () => groups.findAll('.option-label').map(option => option.text())
    await providers.get('button[title="OpenAI"]').trigger('click')
    expect(names()).toEqual(['modelPlaza.gallery.allGroups','OpenAI only','Mixed'])
    expect(groups.get('button[title="modelPlaza.gallery.allGroups"] .option-count').text()).toBe('2')
    expect(groups.findAll('select option').map(option => option.attributes('value'))).toEqual(['all','0.5','1'])
    await groups.get('button[title="Mixed"]').trigger('click')
    await providers.get('button[title="Anthropic"]').trigger('click')
    expect(names()).toEqual(['modelPlaza.gallery.allGroups','Claude only','Mixed'])
    expect(groups.get('button[aria-pressed="true"]').attributes('title')).toBe('Mixed')
    expect(wrapper.findAll('.plaza-model-card')).toHaveLength(1)
    expect(wrapper.get('.model-identity h3').text()).toBe('claude-mixed')
    await providers.get('button[title="modelPlaza.gallery.allProviders"]').trigger('click')
    expect(names()).toEqual(['modelPlaza.gallery.allGroups','OpenAI only','Claude only','Mixed'])
    expect(wrapper.findAll('.plaza-model-card')).toHaveLength(2)
    wrapper.unmount()
  })
  it('clears incompatible group and rate selections when switching providers', async () => {
    const wrapper = render([
      group({ name:'OpenAI only', user_rate_multiplier:.5 }),
      group({ id:2, name:'Claude only', platform:'anthropic', user_rate_multiplier:2, models:[model({ name:'claude-only', platform:'anthropic' })] }),
    ])
    const providers = wrapper.get('[aria-label="modelPlaza.gallery.providers"]')
    const groups = wrapper.get('[aria-label="modelPlaza.gallery.groups"]')
    await groups.get('button[title="OpenAI only"]').trigger('click')
    await groups.get('select').setValue('0.5')
    await providers.get('button[title="Anthropic"]').trigger('click')
    expect(groups.findAll('.option-label').map(option => option.text())).toEqual(['modelPlaza.gallery.allGroups','Claude only'])
    expect(groups.get('button[aria-pressed="true"]').attributes('title')).toBe('modelPlaza.gallery.allGroups')
    expect(groups.find('select').exists()).toBe(false)
    expect(wrapper.findAll('.plaza-model-card')).toHaveLength(1)
    expect(wrapper.get('.model-identity h3').text()).toBe('claude-only')
    await providers.get('button[title="modelPlaza.gallery.allProviders"]').trigger('click')
    expect(groups.get('select').element.value).toBe('all')
    expect(wrapper.findAll('.plaza-model-card')).toHaveLength(2)
    wrapper.unmount()
  })
  it('expands overflowing provider filter chips', async () => {
    const wrapper = render(Array.from({ length:12 },(_,i) => group({ id:i, models:[model({ platform:'vendor-'+i })] })))
    const section = wrapper.findAll('.plaza-filter-section')[0]
    expect(section.findAll('.filter-options button')).toHaveLength(10)
    await section.get('.filter-more').trigger('click')
    expect(section.findAll('.filter-options button')).toHaveLength(13)
    expect(section.get('.filter-more').attributes('aria-expanded')).toBe('true')
    wrapper.unmount()
  })
  it('shows card throughput and latency independently of the monitor page hide-throughput setting', async () => {
    statusFor.mockReturnValue({ ...unknownPlazaStatus(), throughput:60.2, latency:12450 })
    const pinia = createPinia()
    useAppStore(pinia).cachedPublicSettings = { channel_monitor_hide_throughput:true } as NonNullable<ReturnType<typeof useAppStore>['cachedPublicSettings']>
    const wrapper = mount(ModelPlazaContent, { props:{ response:{ description:'', groups:[group()] }, loading:false }, global:{ plugins:[pinia], stubs:{ teleport:true } } })
    expect(wrapper.get('.throughput-metric em').text()).toBe('60.2 t/s')
    expect(wrapper.get('.latency-metric em').text()).toBe('12.45s')
    await wrapper.get('.plaza-model-card').trigger('click')
    expect(wrapper.get('.detail-monitor .throughput-metric em').text()).toBe('60.2 t/s')
    wrapper.unmount()
  })
  it('renders no-data status without invented availability and honors loading/error recovery', async () => {
    const wrapper = render()
    expect(wrapper.get('.plaza-status-footer strong').text()).toBe('—')
    expect(wrapper.get('.throughput-metric em').text()).toBe('—')
    expect(wrapper.get('.latency-metric em').text()).toBe('—')
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
