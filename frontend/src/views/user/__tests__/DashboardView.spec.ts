import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import DashboardView from '../DashboardView.vue'

const {
  getDashboardStats,
  getDashboardTrend,
  getDashboardModels,
  getByDateRange,
  getMyPlatformQuotas,
  getMyAttributes,
  refreshUser,
} = vi.hoisted(() => ({
  getDashboardStats: vi.fn(),
  getDashboardTrend: vi.fn(),
  getDashboardModels: vi.fn(),
  getByDateRange: vi.fn(),
  getMyPlatformQuotas: vi.fn(),
  getMyAttributes: vi.fn(),
  refreshUser: vi.fn(),
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    user: { username: 'Kuhne', email: 'kuhne@example.com', balance: 42.75 },
    isSimpleMode: false,
    refreshUser,
  }),
}))

vi.mock('@/api/usage', () => ({
  usageAPI: {
    getDashboardStats,
    getDashboardTrend,
    getDashboardModels,
    getByDateRange,
  },
}))

vi.mock('@/api/user', () => ({ getMyAttributes, getMyPlatformQuotas }))

describe('user DashboardView', () => {
  beforeEach(() => {
    refreshUser.mockReset().mockResolvedValue(undefined)
    getDashboardStats.mockReset().mockResolvedValue({
      total_api_keys: 2,
      active_api_keys: 1,
      total_requests: 120,
      total_input_tokens: 1000,
      total_output_tokens: 500,
      total_cache_creation_tokens: 0,
      total_cache_read_tokens: 0,
      total_tokens: 1500,
      total_cost: 12,
      total_actual_cost: 10,
      today_requests: 18,
      today_input_tokens: 100,
      today_output_tokens: 50,
      today_cache_creation_tokens: 0,
      today_cache_read_tokens: 0,
      today_tokens: 150,
      today_cost: 2,
      today_actual_cost: 1.5,
      average_duration_ms: 320,
      rpm: 3,
      tpm: 25,
    })
    getDashboardTrend.mockReset().mockResolvedValue({ trend: [] })
    getDashboardModels.mockReset().mockResolvedValue({ models: [] })
    getByDateRange.mockReset().mockResolvedValue({ items: [] })
    getMyPlatformQuotas.mockReset().mockResolvedValue({ platform_quotas: [] })
    getMyAttributes.mockReset().mockResolvedValue({
      definitions: [
        { id: 1, key: 'loyalty_weekly_points', type: 'number', enabled: true },
        { id: 2, key: 'loyalty_permanent_points', type: 'number', enabled: true },
      ],
      values: [
        { attribute_id: 1, value: '36', updated_at: new Date().toISOString() },
        { attribute_id: 2, value: '920', updated_at: new Date().toISOString() },
      ],
    })
  })

  it("places balance first and shows points plus today's input and output tokens", async () => {
    const wrapper = mount(DashboardView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          LoadingSpinner: true,
          Icon: true,
          'router-link': { template: '<a><slot /></a>' },
        },
      },
    })
    await flushPromises()

    const cards = wrapper.findAll('.hero-card')
    expect(cards).toHaveLength(3)
    expect(cards[0].text()).toContain('账户余额')
    expect(cards[0].text()).toContain('$42.75')
    expect(cards[0].text()).toContain('周积分 36')
    expect(cards[0].text()).toContain('永久积分 920')
    expect(cards[1].text()).toContain('今日 API 调用')
    expect(cards[2].text()).toContain('今日 Token')
    expect(cards[2].text()).toContain('输入100')
    expect(cards[2].text()).toContain('输出50')
    expect(cards[2].text()).toContain('今日消费 $1.50')
  })
})
