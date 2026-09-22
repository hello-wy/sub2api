import { beforeEach, describe, expect, it, vi } from 'vitest'

const { post } = vi.hoisted(() => ({ post: vi.fn() }))

vi.mock('@/api/client', () => ({
  apiClient: { post }
}))

import { checkOpenAIRiskControl } from '@/api/admin/accounts'

describe('admin account risk control API', () => {
  beforeEach(() => post.mockReset())

  it('calls the dedicated account endpoint', async () => {
    const response = {
      account: { id: 7 },
      result: {
        status: 'suspected',
        suspected: true,
        state_length: 312,
        http_status: 200,
        checked_at: '2026-09-22T08:00:00Z',
        reason: 'turn_state_length_312'
      }
    }
    post.mockResolvedValueOnce({ data: response })

    await expect(checkOpenAIRiskControl(7)).resolves.toEqual(response)
    expect(post).toHaveBeenCalledWith('/admin/accounts/7/risk-control-check')
  })
})
