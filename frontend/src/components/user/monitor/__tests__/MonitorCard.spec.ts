import { defineComponent, h } from 'vue'
import { describe, expect, it, vi } from 'vitest'
import { shallowMount } from '@vue/test-utils'
import MonitorCard from '../MonitorCard.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key, te: () => true }),
  }
})

vi.mock('@/utils/featureFlags', () => ({
  isChannelMonitorQuotaVisible: () => false,
}))

vi.mock('@/composables/useChannelMonitorFormat', () => ({
  providerGradient: () => 'provider-gradient',
  useChannelMonitorFormat: () => ({
    statusLabel: (value: string) => value,
    statusBadgeClass: () => 'status-class',
    providerLabel: (value: string) => value,
    providerBadgeClass: () => 'provider-class',
    formatLatency: (value: number | null) => value == null ? '—' : String(value),
  }),
}))

const MonitorTimelineStub = defineComponent({
  name: 'MonitorTimeline',
  props: ['buckets', 'countdownSeconds', 'showCountdown'],
  setup: () => () => h('div', { 'data-test': 'timeline' }),
})

const timeline = [
  {
    status: 'operational' as const,
    latency_ms: 120,
    ping_latency_ms: 20,
    checked_at: '2026-07-14T00:00:00Z',
  },
]

const item = {
  id: 1,
  name: 'Main',
  provider: 'openai' as const,
  group_name: 'Codex',
  primary_model: 'gpt-5.6-sol',
  primary_status: 'operational' as const,
  primary_latency_ms: 120,
  primary_ping_latency_ms: 20,
  primary_throughput_tps: 50,
  availability_7d: 100,
  extra_models: [],
  timeline,
}

function render(itemOverride = item) {
  return shallowMount(MonitorCard, {
    props: {
      item: itemOverride,
      window: '7d',
      availabilityValue: 100,
      countdownSeconds: 51,
    },
    global: {
      stubs: {
        ProviderIcon: true,
        MonitorMetricPair: true,
        MonitorAvailabilityRow: true,
        MonitorTimeline: MonitorTimelineStub,
      },
    },
  })
}

describe('MonitorCard', () => {
  it('passes recent status records and refresh countdown to the timeline footer', () => {
    const component = render().getComponent(MonitorTimelineStub)

    expect(component.props('buckets')).toEqual(timeline)
    expect(component.props('countdownSeconds')).toBe(51)
    expect(component.props('showCountdown')).not.toBe(false)
  })

  it('keeps the timeline footer when there is no history yet', () => {
    const component = render({ ...item, timeline: [] }).getComponent(MonitorTimelineStub)

    expect(component.props('buckets')).toEqual([])
    expect(component.props('countdownSeconds')).toBe(51)
  })
})
