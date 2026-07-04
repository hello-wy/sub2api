import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get } = vi.hoisted(() => ({
  get: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: {
    get,
  },
}))

import { userAPI } from '@/api/user'

describe('user attributes api', () => {
  beforeEach(() => {
    get.mockReset()
  })

  it('loads current user attributes', async () => {
    get.mockResolvedValue({
      data: {
        definitions: [
          { id: 1, key: 'loyalty_weekly_points', type: 'number', enabled: true },
          { id: 2, key: 'loyalty_permanent_points', type: 'number', enabled: true },
        ],
        values: [{ id: 10, user_id: 42, attribute_id: 1, value: '800' }],
      },
    })

    const resp = await userAPI.getMyAttributes()

    expect(get).toHaveBeenCalledWith('/user/attributes')
    expect(resp.definitions).toHaveLength(2)
    expect(resp.values[0].value).toBe('800')
  })

  it('falls back to empty arrays when the response omits lists', async () => {
    get.mockResolvedValue({ data: {} })

    await expect(userAPI.getMyAttributes()).resolves.toEqual({
      definitions: [],
      values: [],
    })
  })
})
