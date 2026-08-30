import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, post } = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: {
    get,
    post,
  },
}))

import { checkinAPI } from '@/api/checkin'

describe('checkin api', () => {
  beforeEach(() => {
    get.mockReset()
    post.mockReset()
  })

  it('falls back to the daily base reward range when status omits reward bounds', async () => {
    get.mockResolvedValue({
      data: {
        summary: {
          qq_bound: true,
          can_check_in: true,
          checked_in_today: false,
          today: '2026-06-28',
          streak_days: 1,
          this_month_count: 0,
          base_reward: 3,
          bonus_reward: 0,
          today_reward: 3,
          total_reward: 0,
          recent_records: [],
        },
        balance: 0,
      },
    })

    const status = await checkinAPI.getCheckinStatus()

    expect(status.base_reward_min).toBe(1)
    expect(status.base_reward_max).toBe(3)
    expect(status.today_reward_min).toBe(1)
    expect(status.today_reward_max).toBe(3)
    expect(status.reward_cycle_days).toBe(30)
    expect(status.reward_cycle_day).toBe(1)
    expect(status.reward_cycle_number).toBe(1)
  })
})
