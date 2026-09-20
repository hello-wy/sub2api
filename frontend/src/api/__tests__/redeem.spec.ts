import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, post } = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }))

vi.mock('@/api/client', () => ({ apiClient: { get, post } }))

import { getHistory, redeem } from '@/api/redeem'

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

describe('redemption history pagination', () => {
  beforeEach(() => vi.clearAllMocks())

  it.each([[undefined, undefined, 1, 20], [3, 50, 3, 50], [2, 100, 2, 100]])(
    'sends page %s and size %s to the server', async (page, size, expectedPage, expectedSize) => {
      const response = { items: [], total: 105, page: expectedPage, page_size: expectedSize, pages: 6 }
      get.mockResolvedValue({ data: response })
      expect(await getHistory(page, size)).toEqual(response)
      expect(get).toHaveBeenCalledWith('/redeem/history', {
        params: { page: expectedPage, page_size: expectedSize }
      })
    }
  )
})
