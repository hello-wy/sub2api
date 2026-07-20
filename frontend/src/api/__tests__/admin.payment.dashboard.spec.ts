import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get } = vi.hoisted(() => ({ get: vi.fn() }))

vi.mock('@/api/client', () => ({ apiClient: { get } }))

import { adminPaymentAPI } from '@/api/admin/payment'

describe('admin payment dashboard api', () => {
  beforeEach(() => get.mockReset())

  it('requests revenue for one explicit currency', async () => {
    get.mockResolvedValue({ data: {} })

    await adminPaymentAPI.getDashboard(30, 'USD')

    expect(get).toHaveBeenCalledWith('/admin/payment/dashboard', {
      params: { days: 30, currency: 'USD' },
    })
  })
})
