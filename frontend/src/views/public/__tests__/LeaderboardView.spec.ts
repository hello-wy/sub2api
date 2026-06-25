import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import LeaderboardView from '../LeaderboardView.vue'

const { getUserSpendingRanking } = vi.hoisted(() => ({
  getUserSpendingRanking: vi.fn()
}))

vi.mock('@/api/admin/dashboard', () => ({
  getUserSpendingRanking,
  default: { getUserSpendingRanking },
  dashboardAPI: { getUserSpendingRanking }
}))

vi.mock('@/stores', () => ({
  useAuthStore: () => ({
    user: { id: 7 }
  })
}))

describe('LeaderboardView range switcher', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-06-25T10:00:00+08:00'))
    getUserSpendingRanking.mockReset()
    getUserSpendingRanking.mockResolvedValue({
      ranking: [],
      total_actual_cost: 0,
      total_requests: 0,
      total_tokens: 0,
      start_date: '',
      end_date: ''
    })
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('loads today by default and reloads yesterday when selected', async () => {
    const wrapper = mount(LeaderboardView, {
      global: {
        stubs: {
          AppLayout: { template: '<main><slot /></main>' }
        }
      }
    })

    await flushPromises()

    expect(getUserSpendingRanking).toHaveBeenLastCalledWith({
      start_date: '2026-06-25',
      end_date: '2026-06-25',
      limit: 10
    })
    expect(wrapper.text()).toContain('今日排行榜')

    await wrapper.get('[data-testid="leaderboard-range-yesterday"]').trigger('click')
    await flushPromises()

    expect(getUserSpendingRanking).toHaveBeenLastCalledWith({
      start_date: '2026-06-24',
      end_date: '2026-06-24',
      limit: 10
    })
    expect(wrapper.text()).toContain('昨日排行榜')

    wrapper.unmount()
  })
})
