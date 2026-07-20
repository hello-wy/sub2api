import { beforeEach, describe, expect, it, vi } from 'vitest'

const { post } = vi.hoisted(() => ({ post: vi.fn() }))

vi.mock('@/api/client', () => ({ apiClient: { post } }))

import { paymentAPI } from '@/api/payment'

describe('payment api idempotency', () => {
  beforeEach(() => post.mockReset())

  it('binds balance subscription purchases to the supplied idempotency key', async () => {
    post.mockResolvedValue({ data: {} })

    await paymentAPI.purchaseSubscriptionWithBalance(7, 'balance-subscription-7-request')

    expect(post).toHaveBeenCalledWith(
      '/payment/subscriptions/balance',
      { plan_id: 7 },
      { headers: { 'Idempotency-Key': 'balance-subscription-7-request' } },
    )
  })
})
