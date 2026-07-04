import { beforeEach, describe, expect, it, vi } from 'vitest'

const { put } = vi.hoisted(() => ({
  put: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: {
    put,
  },
}))

import { userAttributesAPI } from '@/api/admin/userAttributes'

describe('admin user attributes api', () => {
  beforeEach(() => {
    put.mockReset()
  })

  it('serializes numeric user attribute values as strings before updating', async () => {
    put.mockResolvedValue({ data: [] })

    await userAttributesAPI.updateUserAttributeValues(1, {
      10: 25,
      11: 'active',
      12: 12.5,
    })

    expect(put).toHaveBeenCalledWith('/admin/users/1/attributes', {
      values: {
        10: '25',
        11: 'active',
        12: '12.5',
      },
    })
  })
})
