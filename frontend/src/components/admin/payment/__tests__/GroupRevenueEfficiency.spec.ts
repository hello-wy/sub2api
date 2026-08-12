import { describe, expect, it, vi } from 'vitest'
import { shallowMount } from '@vue/test-utils'
import GroupRevenueEfficiency from '../GroupRevenueEfficiency.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key }),
}))

describe('GroupRevenueEfficiency', () => {
  it('renders the expected quota, actual deducted quota, base usage, and unit recovery value', () => {
    const wrapper = shallowMount(GroupRevenueEfficiency, {
      props: {
        currency: 'CNY',
        groups: [{ group_id: 1, group_name: '专业订阅', rate_multiplier: 2, revenue: 100, expected_quota: 155, user_usage: 1000, base_usage: 500, unit_revenue: 0.2 }],
      },
    })

    expect(wrapper.text()).toContain('专业订阅')
    expect(wrapper.text()).toContain('2x')
    expect(wrapper.text()).toContain('CN¥100.00')
    expect(wrapper.text()).toContain('155')
    expect(wrapper.text()).toContain('1,000')
    expect(wrapper.text()).toContain('500')
    expect(wrapper.text()).toContain('CN¥0.20')
    expect(wrapper.text()).not.toContain('/ payment.admin.groupBaseUsage')
    expect(wrapper.findAll('th')).toHaveLength(7)
  })

  it('marks groups without base usage instead of rendering an invalid ratio', () => {
    const wrapper = shallowMount(GroupRevenueEfficiency, {
      props: {
        currency: 'CNY',
        groups: [{ group_id: 2, group_name: '', rate_multiplier: 1, revenue: 40, expected_quota: null, user_usage: 0, base_usage: 0, unit_revenue: null }],
      },
    })

    expect(wrapper.text()).toContain('payment.admin.unknownGroup #2')
    expect(wrapper.text()).toContain('payment.admin.noUsage')
  })

  it('does not render an invalid multiplier when the API response is missing the field', () => {
    const wrapper = shallowMount(GroupRevenueEfficiency, {
      props: {
        currency: 'CNY',
        groups: [{ group_id: 3, group_name: '旧数据', rate_multiplier: undefined, revenue: 10, expected_quota: null, user_usage: 10, base_usage: 10, unit_revenue: null }],
      },
    })

    expect(wrapper.text()).toContain('-')
    expect(wrapper.text()).not.toContain('NaN')
  })
})
