import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

import type { DashboardStats, ModelStat, TrendDataPoint, UserSpendingRankingItem } from '@/types'
import DashboardView from '../DashboardView.vue'

const { getSnapshotV2, getUserSpendingRanking, push } = vi.hoisted(() => ({
  getSnapshotV2: vi.fn(),
  getUserSpendingRanking: vi.fn(),
  push: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    dashboard: {
      getSnapshotV2,
      getUserSpendingRanking
    }
  }
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    user: { username: 'Kuhne', email: 'kuhne@example.com' }
  })
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({ push })
}))

const formatLocalDate = (date: Date): string => {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

const createDashboardStats = (overrides: Partial<DashboardStats> = {}): DashboardStats => ({
  total_users: 0,
  today_new_users: 0,
  active_users: 0,
  hourly_active_users: 0,
  stats_updated_at: '',
  stats_stale: false,
  total_api_keys: 0,
  active_api_keys: 0,
  total_accounts: 0,
  normal_accounts: 0,
  error_accounts: 0,
  ratelimit_accounts: 0,
  overload_accounts: 0,
  total_requests: 0,
  total_input_tokens: 0,
  total_output_tokens: 0,
  total_cache_creation_tokens: 0,
  total_cache_read_tokens: 0,
  total_tokens: 0,
  total_cost: 0,
  total_actual_cost: 0,
  total_account_cost: 0,
  today_requests: 0,
  today_input_tokens: 0,
  today_output_tokens: 0,
  today_cache_creation_tokens: 0,
  today_cache_read_tokens: 0,
  today_tokens: 0,
  today_cost: 0,
  today_actual_cost: 0,
  today_account_cost: 0,
  average_duration_ms: 0,
  uptime: 0,
  rpm: 0,
  tpm: 0,
  ...overrides
})

const mountDashboard = () => mount(DashboardView, {
  global: {
    stubs: {
      AppLayout: { template: '<div><slot /></div>' },
      LoadingSpinner: true,
      Icon: true,
      'router-link': { props: ['to'], template: '<a><slot /></a>' }
    }
  }
})

describe('admin DashboardView', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date(2026, 6, 15, 20, 0, 0))
    setActivePinia(createPinia())

    getSnapshotV2.mockReset()
    getUserSpendingRanking.mockReset()
    push.mockReset()

    getSnapshotV2.mockResolvedValue({
      stats: createDashboardStats(),
      trend: [],
      models: []
    })
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

  it('默认使用最近 24 小时并在空数据时保留完整仪表盘结构', async () => {
    const wrapper = mountDashboard()
    await flushPromises()

    const now = new Date()
    const yesterday = new Date(now.getTime() - 24 * 60 * 60 * 1000)

    expect(getSnapshotV2).toHaveBeenCalledTimes(1)
    expect(getSnapshotV2).toHaveBeenCalledWith(expect.objectContaining({
      start_date: formatLocalDate(yesterday),
      end_date: formatLocalDate(now),
      granularity: 'hour'
    }))
    expect(wrapper.text()).toContain('晚上好，Kuhne')
    expect(wrapper.findAll('.hero-card')).toHaveLength(3)
    expect(wrapper.findAll('.hero-card')[0].text()).toContain('今日 Token')
    expect(wrapper.findAll('.hero-card')[1].text()).toContain('今日 API 调用')
    expect(wrapper.text()).toContain('该时间范围暂无 Token 使用数据')
    expect(wrapper.text()).toContain('该时间范围暂无模型使用数据')
    expect(wrapper.text()).toContain('该时间范围暂无用户使用数据')
  })

  it('展示真实趋势、模型分布和用户排名，并支持跳转到用户用量', async () => {
    const trend: TrendDataPoint[] = [
      { date: '2026-07-14T20:00:00Z', requests: 20, input_tokens: 80, output_tokens: 20, cache_creation_tokens: 0, cache_read_tokens: 0, total_tokens: 100, cost: 1, actual_cost: 1 },
      { date: '2026-07-15T20:00:00Z', requests: 40, input_tokens: 180, output_tokens: 40, cache_creation_tokens: 0, cache_read_tokens: 0, total_tokens: 220, cost: 2, actual_cost: 2 }
    ]
    const models: ModelStat[] = [
      { model: 'gpt-5', requests: 12, input_tokens: 600, output_tokens: 400, cache_creation_tokens: 0, cache_read_tokens: 0, total_tokens: 1000, cost: 3, actual_cost: 3 },
      { model: 'claude-3-7-sonnet', requests: 8, input_tokens: 300, output_tokens: 200, cache_creation_tokens: 0, cache_read_tokens: 0, total_tokens: 500, cost: 1, actual_cost: 1 }
    ]
    const ranking: UserSpendingRankingItem[] = [{ user_id: 42, email: 'team@example.com', actual_cost: 12.34, requests: 20, tokens: 1000, rank: 1 }]

    getSnapshotV2.mockResolvedValueOnce({
      stats: createDashboardStats({ today_requests: 128420, total_requests: 900000, today_tokens: 2840000, today_actual_cost: 124.82, today_account_cost: 86.4, today_cost: 156.75 }),
      trend,
      models
    })
    getUserSpendingRanking.mockResolvedValueOnce({ ranking, total_actual_cost: 12.34, total_requests: 20, total_tokens: 1000, start_date: '', end_date: '' })

    const wrapper = mountDashboard()
    await flushPromises()

    expect(wrapper.find('.trend-chart').exists()).toBe(true)
    expect(wrapper.text()).toContain('128,420')
    expect(wrapper.findAll('.hero-card')[0].text()).toContain('实际 $124.82')
    expect(wrapper.findAll('.hero-card')[0].text()).toContain('成本 $86.40')
    expect(wrapper.findAll('.hero-card')[0].text()).toContain('标准 $156.75')
    expect(wrapper.text()).toContain('gpt-5')
    expect(wrapper.text()).toContain('team@example.com')

    await wrapper.find('.recent-row').trigger('click')
    expect(push).toHaveBeenCalledWith(expect.objectContaining({
      path: '/admin/usage',
      query: expect.objectContaining({ user_id: '42' })
    }))
  })

  it('切换时间范围后以天粒度重新加载仪表盘', async () => {
    const wrapper = mountDashboard()
    await flushPromises()

    await wrapper.find('select').setValue('7d')
    await flushPromises()

    expect(getSnapshotV2).toHaveBeenLastCalledWith(expect.objectContaining({
      granularity: 'day'
    }))
    expect(getUserSpendingRanking).toHaveBeenCalledTimes(2)
  })
})
