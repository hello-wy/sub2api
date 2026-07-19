import { describe, expect, it, vi } from 'vitest'
import { shallowMount } from '@vue/test-utils'
import OrderStatsCards from '../OrderStatsCards.vue'
import { formatPaymentAmount } from '@/components/payment/currency'
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
    today_amount: 12.5,
    total_amount: 45.75,
    today_count: 1,
    total_count: 2,
    avg_amount: 22.875,
    pending_orders: 0,
    currency,
    available_currencies: [currency],
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

    expect(wrapper.text()).toContain(formatPaymentAmount(stats.today_amount, 'USD'))
    expect(wrapper.text()).toContain(formatPaymentAmount(stats.total_amount, 'USD'))
    expect(wrapper.text()).not.toContain(formatPaymentAmount(stats.total_amount, 'CNY'))
  })
})
