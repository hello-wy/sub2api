import { flushPromises, mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import AdminOrdersView from '../AdminOrdersView.vue'

const { getOrders, getPlans } = vi.hoisted(() => ({
  getOrders: vi.fn(),
  getPlans: vi.fn(),
}))

vi.mock('@/api/admin/payment', () => ({
  adminPaymentAPI: { getOrders, getPlans },
  default: { getOrders, getPlans },
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

const DateRangePickerStub = {
  emits: ['change', 'update:startDate', 'update:endDate'],
  template: `<button class="date-range-picker" @click="$emit('change', { startDate: '2026-08-01', endDate: '2026-08-10', preset: 'custom' })">date</button>`,
}

describe('AdminOrdersView', () => {
  beforeEach(() => {
    getOrders.mockResolvedValue({ data: { items: [], total: 0 } })
    getPlans.mockResolvedValue({ data: [] })
  })

  it('passes the selected inclusive date range to the orders query', async () => {
    const wrapper = mount(AdminOrdersView, {
      global: {
        plugins: [createPinia()],
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          ScrollablePageLayout: { template: '<div><slot /></div>' },
          Select: true,
          DateRangePicker: DateRangePickerStub,
          Icon: true,
          OrderTable: true,
          Pagination: true,
          BaseDialog: true,
          AdminRefundDialog: true,
          OrderStatusBadge: true,
        },
      },
    })

    await flushPromises()
    await wrapper.get('.date-range-picker').trigger('click')
    await flushPromises()

    expect(getOrders).toHaveBeenLastCalledWith(expect.objectContaining({
      start_date: '2026-08-01',
      end_date: '2026-08-10',
    }))
  })
})
