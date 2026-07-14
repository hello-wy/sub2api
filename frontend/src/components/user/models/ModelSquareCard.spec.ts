import { mount } from '@vue/test-utils'
import { defineComponent, h } from 'vue'
import { describe, expect, it, vi } from 'vitest'
import ModelSquareCard from './ModelSquareCard.vue'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string, params?: { count?: number }) => params?.count == null ? key : `${key}:${params.count}` }) }))
vi.mock('@/composables/useChannelMonitorFormat', () => ({
  useChannelMonitorFormat: () => ({
    statusLabel: (value: string) => value,
    statusBadgeClass: () => 'status-class',
    formatLatency: (value: number | null) => value == null ? '—' : String(value),
    formatPercent: (value: number) => `${value.toFixed(2)}%`,
  }),
}))
vi.mock('@/components/common/PlatformIcon.vue', () => ({
  default: defineComponent({ name: 'PlatformIcon', props: ['platform', 'size'], setup: (props) => () => h('i', { 'data-platform': props.platform }) }),
}))
vi.mock('@/components/user/monitor/MonitorTimeline.vue', () => ({
  default: defineComponent({ name: 'MonitorTimeline', props: ['buckets', 'countdownSeconds', 'showCountdown'], setup: () => () => h('div', { 'data-test': 'timeline' }) }),
}))

const group = { id: 1, name: 'Codex', platform: 'openai', subscription_type: 'standard', rate_multiplier: 0.8, peak_rate_enabled: false, peak_start: '', peak_end: '', peak_rate_multiplier: 1, is_exclusive: false }
const card = {
  key: 'channel:openai:gpt', model: 'gpt-5.5', platform: 'openai', channel: 'Main', groups: [group], billingMode: 'token' as const, searchText: '',
  pricing: { billing_mode: 'token' as const, input_price: 0.0000025, output_price: 0.000015, cache_write_price: null, cache_read_price: 0.00000025, image_output_price: null, per_request_price: null, intervals: [] },
}

function render(primaryThroughput: number | null = 49.5, cardOverride = card, includeActiveGroup = true, viewMode: 'grid' | 'list' = 'grid') {
  return mount(ModelSquareCard, {
    props: {
      card: cardOverride, activeGroup: includeActiveGroup ? group : undefined, effectiveRate: 0.8, rechargeMultiplier: 2, viewMode,
      monitor: { id: 1, name: 'Main', provider: 'openai', group_name: 'Codex', primary_model: 'gpt-5.5', primary_status: 'operational', primary_latency_ms: 14720, primary_ping_latency_ms: 20, primary_throughput_tps: primaryThroughput, availability_7d: 99.9, extra_models: [], timeline: [] },
    },
  })
}

describe('ModelSquareCard', () => {
  it('renders a compact grid card with pricing and monitor status', () => {
    const wrapper = render()
    const platformIcon = wrapper.getComponent({ name: 'PlatformIcon' })
    expect(platformIcon.props()).toMatchObject({ platform: 'openai', size: 'lg' })
    expect(platformIcon.classes()).toEqual(expect.arrayContaining(['text-emerald-500', 'dark:text-emerald-400']))
    expect(wrapper.text()).toContain('modelSquare.billing.usageBased')
    expect(wrapper.text()).toContain('Codex')
    expect(wrapper.text()).toContain('$1')
    expect(wrapper.text()).toContain('$6')
    expect(wrapper.get('[data-test="price-grid"]').classes()).toContain('grid-cols-2')
    expect(wrapper.get('[data-test="price-grid"]').classes()).not.toContain('sm:grid-cols-4')
    expect(wrapper.get('[data-test="monitor-summary"]').text()).toContain('49.5TPS')
    expect(wrapper.get('[data-test="monitor-summary"]').text()).toContain('14720ms')
    expect(wrapper.get('[data-test="monitor-summary"]').text()).toContain('20ms')
    expect(wrapper.get('[data-test="monitor-summary"]').text()).toContain('99.90%')
    expect(wrapper.getComponent({ name: 'MonitorTimeline' }).props()).toMatchObject({ buckets: [], showCountdown: false })
    expect(wrapper.find('.numeric').exists()).toBe(true)
    expect(wrapper.find('.metric-number').exists()).toBe(false)
  })

  it('uses responsive list columns while retaining monitor metrics and timeline', () => {
    const wrapper = render(49.5, card, true, 'list')
    expect(wrapper.get('[data-test="card-content"]').classes()).toContain('lg:grid-cols-[minmax(0,11fr)_minmax(0,9fr)]')
    expect(wrapper.get('[data-test="card-identity"]').classes()).toEqual(expect.arrayContaining(['lg:pr-4', 'xl:pr-6']))
    expect(wrapper.get('[data-test="price-grid"]').classes()).toEqual(expect.arrayContaining(['sm:grid-cols-4', 'lg:grid-cols-2', 'xl:grid-cols-4', 'lg:border-l', 'lg:pl-5']))
    expect(wrapper.get('[data-test="monitor-summary"]').text()).toContain('49.5TPS')
    expect(wrapper.get('[data-test="monitor-summary"]').text()).toContain('14720ms')
    expect(wrapper.get('[data-test="monitor-summary"]').text()).toContain('20ms')
    expect(wrapper.get('[data-test="monitor-summary"]').text()).toContain('99.90%')
    expect(wrapper.getComponent({ name: 'MonitorTimeline' }).props('showCountdown')).toBe(false)
  })

  it('renders every formatted price digit without truncating the numeric value', () => {
    const precisePrices = {
      ...card,
      pricing: { ...card.pricing, input_price: 0.00000030864125, output_price: 0 },
    }
    const wrapper = render(49.5, precisePrices, true, 'list')
    const value = wrapper.get('[data-test="price-value"]')

    expect(value.text()).toBe('$0.123457')
    expect(value.classes()).toContain('whitespace-nowrap')
    expect(value.classes()).not.toContain('truncate')
  })

  it('shows unavailable throughput as an em dash instead of zero in list view', () => {
    expect(render(null, card, true, 'list').text()).toContain('—TPS')
  })

  it('does not render monitor status when no monitor was matched', () => {
    const wrapper = mount(ModelSquareCard, {
      props: { card, activeGroup: group, effectiveRate: 0.8, rechargeMultiplier: 2 },
    })

    expect(wrapper.find('[data-test="monitor-summary"]').exists()).toBe(false)
    expect(wrapper.findComponent({ name: 'MonitorTimeline' }).exists()).toBe(false)
  })

  it('hides the current group only when no group was resolved', () => {
    const wrapper = render(49.5, { ...card, groups: [] }, false)
    expect(wrapper.text()).not.toContain('modelSquare.currentGroup')
    expect(wrapper.text()).not.toContain('Codex')
  })

  it('omits zero price dimensions while retaining positive dimensions', () => {
    const zeroPrices = {
      ...card,
      pricing: { ...card.pricing, input_price: 0, output_price: 0.000015, cache_read_price: 0, image_output_price: 0 },
    }
    const wrapper = render(49.5, zeroPrices)

    expect(wrapper.text()).not.toContain('modelSquare.price.input')
    expect(wrapper.text()).toContain('modelSquare.price.output')
    expect(wrapper.text()).not.toContain('modelSquare.price.cacheRead')
    expect(wrapper.text()).not.toContain('modelSquare.price.imageOutput')
  })

  it('uses the responsive no-pricing state when every configured dimension is zero', () => {
    const zeroPrices = {
      ...card,
      pricing: { ...card.pricing, input_price: 0, output_price: 0, cache_write_price: 0, cache_read_price: 0, image_output_price: 0 },
    }
    const wrapper = render(49.5, zeroPrices, true, 'list')

    expect(wrapper.get('[data-test="no-pricing"]').text()).toContain('modelSquare.noPricing')
    expect(wrapper.get('[data-test="no-pricing"]').classes()).toEqual(expect.arrayContaining(['lg:border-l', 'lg:pl-5']))
  })

  it('omits a zero per-request price', () => {
    const zeroRequestPrice = {
      ...card,
      billingMode: 'per_request' as const,
      pricing: { ...card.pricing, billing_mode: 'per_request' as const, per_request_price: 0 },
    }
    expect(render(49.5, zeroRequestPrice).text()).toContain('modelSquare.noPricing')
  })
})
