import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent, h } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ModelSquareView from '../ModelSquareView.vue'

const mocks = vi.hoisted(() => ({
  getAvailable: vi.fn(),
  getUserGroupRates: vi.fn(),
  listMonitors: vi.fn(),
  showError: vi.fn(),
}))

vi.mock('@/api/channels', () => ({ default: { getAvailable: mocks.getAvailable } }))
vi.mock('@/api/groups', () => ({ default: { getUserGroupRates: mocks.getUserGroupRates } }))
vi.mock('@/api/channelMonitor', () => ({ default: { list: mocks.listMonitors } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError: mocks.showError }) }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string, params?: { count?: number }) => params?.count === undefined ? key : `${key}:${params.count}` }) }))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: defineComponent({ setup(_, { slots }) { return () => h('div', { 'data-test': 'layout' }, slots.default?.()) } }) }))
vi.mock('@/components/icons/Icon.vue', () => ({ default: defineComponent({ setup: () => () => h('i') }) }))
vi.mock('@/components/common/Select.vue', () => ({
  default: defineComponent({
    name: 'AppSelect', props: ['modelValue', 'options'], emits: ['update:modelValue'],
    setup(props, { emit }) { return () => h('select', { 'data-test': 'sort-select', value: props.modelValue, onChange: (event: Event) => emit('update:modelValue', (event.target as HTMLSelectElement).value) }, props.options.map((option: { value: string; label: string }) => h('option', { value: option.value }, option.label))) },
  }),
}))
vi.mock('@/components/common/Pagination.vue', () => ({
  default: defineComponent({
    name: 'Pagination', props: ['page', 'total', 'pageSize'], emits: ['update:page'],
    setup(props, { emit }) { return () => h('button', { 'data-test': 'pagination', onClick: () => emit('update:page', Number(props.page) + 1) }, String(props.total)) },
  }),
}))
vi.mock('@/components/user/models/ModelSquareCard.vue', () => ({
  default: defineComponent({
    name: 'ModelSquareCard',
    props: ['card', 'activeGroup', 'effectiveRate', 'monitor', 'viewMode'],
    setup(props) { return () => h('article', { 'data-test': 'model-card', 'data-model': props.card.model, 'data-platform': props.card.platform, 'data-view': props.viewMode }) },
  }),
}))

const groupA = { id: 1, name: 'Standard', platform: 'openai', subscription_type: 'standard', rate_multiplier: 1, peak_rate_enabled: false, peak_start: '', peak_end: '', peak_rate_multiplier: 1, is_exclusive: false }
const groupB = { ...groupA, id: 2, name: 'Premium', rate_multiplier: 1.5 }
const tokenPricing = { billing_mode: 'token', input_price: 1, output_price: 2, cache_write_price: null, cache_read_price: null, image_output_price: null, per_request_price: null, intervals: [] }
const requestPricing = { ...tokenPricing, billing_mode: 'per_request', per_request_price: 0.2 }

function channels() {
  return [{
    name: 'OpenAI Main', description: '', platforms: [
      { platform: 'openai', groups: [groupA], supported_models: [{ name: 'GPT_4o', platform: 'openai', pricing: tokenPricing }] },
      { platform: 'azure', groups: [groupB], supported_models: [{ name: 'Claude', platform: 'azure', pricing: requestPricing }] },
    ],
  }]
}

function monitor(overrides: Record<string, unknown> = {}) {
  return { id: 1, name: ' openai_main ', provider: 'openai', group_name: '', primary_model: 'gpt 4o', primary_status: 'operational', primary_latency_ms: 10, primary_ping_latency_ms: 5, availability_7d: 100, extra_models: [], timeline: [{ status: 'operational', latency_ms: 10, ping_latency_ms: 5, checked_at: '2026-07-13T00:00:00Z' }], ...overrides }
}

async function render(monitors = [monitor()]) {
  mocks.listMonitors.mockResolvedValue({ items: monitors })
  const wrapper = mount(ModelSquareView)
  await flushPromises()
  return wrapper
}

beforeEach(() => {
  vi.clearAllMocks()
  mocks.getAvailable.mockResolvedValue(channels())
  mocks.getUserGroupRates.mockResolvedValue({ 1: 0.8 })
})

describe('ModelSquareView', () => {
  it('builds cards from core channel data and passes enhancement values', async () => {
    const wrapper = await render()
    const cards = wrapper.findAllComponents({ name: 'ModelSquareCard' })

    expect(cards).toHaveLength(2)
    expect(cards[0].props('card')).toMatchObject({ key: 'OpenAI Main:openai:GPT_4o', model: 'GPT_4o', platform: 'openai', channel: 'OpenAI Main', billingMode: 'token', pricing: tokenPricing })
    expect(cards[0].props()).toMatchObject({ activeGroup: groupA, effectiveRate: 0.8 })
    expect(cards[0].props('monitor')).toBeUndefined()
    expect(mocks.listMonitors).toHaveBeenCalledTimes(1)
    expect(cards[1].props('card')).toMatchObject({ model: 'Claude', billingMode: 'per_request' })
  })

  it('still builds core cards when the monitor enhancement fails', async () => {
    mocks.listMonitors.mockRejectedValue(new Error('monitor unavailable'))

    const wrapper = await render([])
    const cards = wrapper.findAllComponents({ name: 'ModelSquareCard' })
    expect(cards).toHaveLength(2)
    expect(cards[0].props('monitor')).toBeUndefined()
    expect(mocks.showError).not.toHaveBeenCalled()
  })

  it.each([
    ['different model suffix', [monitor({ primary_model: 'gpt-4o-mini' })]],
    ['ambiguous duplicate', [monitor(), monitor({ id: 2 })]],
  ])('does not attach monitor for %s', async (_label, monitors) => {
    const wrapper = await render(monitors)
    expect(wrapper.findAllComponents({ name: 'ModelSquareCard' })[0].props('monitor')).toBeUndefined()
  })

  it('attaches a unique monitor even when its timeline is empty', async () => {
    const wrapper = await render([monitor({ group_name: 'Standard', timeline: [] })])
    expect(wrapper.findAllComponents({ name: 'ModelSquareCard' })[0].props('monitor')).toMatchObject({ id: 1, timeline: [] })
  })

  it('attaches only the unique normalized channel, model, and resolved group match', async () => {
    const wrapper = await render([
      monitor({ group_name: 'Standard' }),
      monitor({ id: 2, name: 'Other', primary_model: 'GPT_4o', group_name: 'Standard' }),
    ])
    expect(wrapper.findAllComponents({ name: 'ModelSquareCard' })[0].props('monitor')).toMatchObject({ id: 1 })
  })

  it('shows expanded button filters and filters by search, platform, group, and billing mode', async () => {
    const wrapper = await render([])
    const input = wrapper.get('input')

    expect(wrapper.get('[data-test="sort-select"]').exists()).toBe(true)
    expect(wrapper.get('#model-square-filters').isVisible()).toBe(true)
    expect(wrapper.get('[data-test="platform-filter-openai"]').attributes('aria-pressed')).toBe('false')
    expect(wrapper.get('[data-test="group-filter-2"]').text()).toContain('Premium')

    await input.setValue('premium')
    expect(wrapper.findAll('[data-test="model-card"]').map(node => node.attributes('data-model'))).toEqual(['Claude'])
    await input.setValue('')
    await wrapper.get('[data-test="platform-filter-openai"]').trigger('click')
    expect(wrapper.get('[data-test="platform-filter-openai"]').attributes('aria-pressed')).toBe('true')
    expect(wrapper.findAll('[data-test="model-card"]').map(node => node.attributes('data-model'))).toEqual(['GPT_4o'])

    await wrapper.get('[data-test="group-filter-2"]').trigger('click')
    expect(wrapper.get('[data-test="platform-filter-azure"]').attributes('aria-pressed')).toBe('true')
    expect(wrapper.findAll('[data-test="model-card"]').map(node => node.attributes('data-model'))).toEqual(['Claude'])
    expect(wrapper.findAllComponents({ name: 'ModelSquareCard' })[0].props('activeGroup')).toMatchObject({ id: 2 })

    await wrapper.get('[data-test="billing-filter-usage"]').trigger('click')
    expect(wrapper.findAll('[data-test="model-card"]')).toHaveLength(0)

    await wrapper.get('[data-test="reset-filters"]').trigger('click')
    expect(wrapper.findAll('[data-test="model-card"]')).toHaveLength(2)
    expect(wrapper.get('[data-test="group-filter-all"]').attributes('aria-pressed')).toBe('true')
  })

  it('uses an uncarded filter rail and a dense three-column grid', async () => {
    const wrapper = await render([])
    const aside = wrapper.get('[data-test="filter-sidebar"]')
    const grid = wrapper.get('[data-test="model-card-grid"]')

    expect(aside.classes()).toContain('lg:border-r')
    expect(aside.classes()).not.toContain('rounded-2xl')
    expect(aside.classes()).not.toContain('bg-white')
    expect(aside.classes()).not.toContain('shadow-sm')
    expect(wrapper.get('main').classes()).toContain('min-w-0')
    expect(grid.classes()).toContain('xl:grid-cols-3')
    expect(wrapper.get('[data-test="grid-view"]').attributes('aria-pressed')).toBe('true')
    expect(wrapper.get('[data-test="reset-filters"]').classes()).toContain('filter-reset')
    expect(wrapper.get('[data-test="platform-filter-all"]').classes()).toContain('filter-option-active')
    expect(wrapper.get('[data-test="grid-view"]').classes()).toContain('view-button-active')

    await wrapper.get('[data-test="list-view"]').trigger('click')
    expect(wrapper.get('[data-test="list-view"]').attributes('aria-pressed')).toBe('true')
    expect(wrapper.findAll('[data-test="model-card"]').every(card => card.attributes('data-view') === 'list')).toBe(true)
  })

  it('sorts models locally without changing the default API order', async () => {
    const wrapper = await render([])
    const modelNames = () => wrapper.findAll('[data-test="model-card"]').map(node => node.attributes('data-model'))

    expect(modelNames()).toEqual(['GPT_4o', 'Claude'])
    await wrapper.get('[data-test="sort-select"]').setValue('model-asc')
    expect(modelNames()).toEqual(['Claude', 'GPT_4o'])
    await wrapper.get('[data-test="sort-select"]').setValue('model-desc')
    expect(modelNames()).toEqual(['GPT_4o', 'Claude'])
  })

  it('matches the resolved group monitor exactly and rejects ambiguous group monitors', async () => {
    const matching = monitor({ group_name: 'Standard' })
    const other = monitor({ id: 2, group_name: 'Premium' })
    const wrapper = await render([matching, other])

    expect(wrapper.findAllComponents({ name: 'ModelSquareCard' })[0].props('monitor')).toMatchObject({ id: 1 })

    mocks.listMonitors.mockResolvedValue({ items: [matching, monitor({ id: 3, group_name: 'Standard' })] })
    const ambiguousWrapper = mount(ModelSquareView)
    await flushPromises()
    expect(ambiguousWrapper.findAllComponents({ name: 'ModelSquareCard' })[0].props('monitor')).toBeUndefined()
  })
})
