import { describe, expect, it, vi } from 'vitest'
import { shallowMount } from '@vue/test-utils'
import OrderStatsCards from '../OrderStatsCards.vue'
import type { DashboardStats } from '@/types/payment'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

function dashboardStats(currency: string): DashboardStats {
  return {
    today_amount: { [currency]: 12.5 },
    total_amount: { [currency]: 45.75 },
    today_count: 1,
    total_count: 2,
    avg_amount: { [currency]: 22.875 },
    currency,
    available_currencies: [currency],
    subscription_plans: [],
    daily_series: [],
    payment_methods: [],
    top_users: [],
  }
}

describe('OrderStatsCards currency display', () => {
  it('formats revenue using the selected dashboard currency', () => {
    const stats = dashboardStats('USD')
    const wrapper = shallowMount(OrderStatsCards, {
      props: { stats },
      global: { stubs: { Icon: true } },
    })

    expect(wrapper.text()).toContain('$12.50')
    expect(wrapper.text()).toContain('$45.75')
  })
})
