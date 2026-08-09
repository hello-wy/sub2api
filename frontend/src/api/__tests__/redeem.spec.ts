import { beforeEach, describe, expect, it, vi } from 'vitest'

const post = vi.hoisted(() => vi.fn())

vi.mock('@/api/client', () => ({ apiClient: { post } }))

import { redeem } from '@/api/redeem'

describe('redeem API subscription overwrite confirmation', () => {
  beforeEach(() => {
    post.mockReset()
    post.mockResolvedValue({ data: { type: 'subscription' } })
  })

  it('sends the exact subscription snapshot with overwrite confirmation', async () => {
    await redeem('SUB-CODE', {
      subscriptionId: 11,
      termVersion: 7,
      expiresAt: '2026-08-11T13:00:00.123456Z',
    })

    expect(post).toHaveBeenCalledWith('/redeem', {
      code: 'SUB-CODE',
      confirm_subscription_overwrite: true,
      expected_subscription_id: 11,
      expected_subscription_term_version: 7,
      expected_subscription_expires_at: '2026-08-11T13:00:00.123456Z',
    })
  })

  it('does not send confirmation fields on the first redemption attempt', async () => {
    await redeem('SUB-CODE')

    expect(post).toHaveBeenCalledWith('/redeem', {
      code: 'SUB-CODE',
      confirm_subscription_overwrite: undefined,
      expected_subscription_id: undefined,
      expected_subscription_term_version: undefined,
      expected_subscription_expires_at: undefined,
    })
  })
})
