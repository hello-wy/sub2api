import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import type { UserUsageTrendPoint } from '@/types'
import UserUsageTrend from '../UserUsageTrend.vue'

const lineProps = vi.hoisted(() => vi.fn())

vi.mock('vue-chartjs', () => ({
  Line: {
    props: ['data', 'options'],
    setup(props: { data: unknown; options: unknown }) {
      lineProps(props)
      return () => null
    }
  }
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key === 'admin.dashboard.noDataAvailable' ? '暂无数据' : key })
}))

const trend: UserUsageTrendPoint[] = [
  { date: '2026-07-20', user_id: 1, email: 'alice@example.com', username: 'Alice', requests: 2, tokens: 1200, cost: 1, actual_cost: 1 },
  { date: '2026-07-21', user_id: 1, email: 'alice@example.com', username: 'Alice', requests: 3, tokens: 2400, cost: 2, actual_cost: 2 },
  { date: '2026-07-21', user_id: 2, email: 'bob@example.com', username: 'Bob', requests: 1, tokens: 800, cost: 0.5, actual_cost: 0.5 }
]

const mountTrend = (props: { trendData: UserUsageTrendPoint[]; loading?: boolean }) => mount(UserUsageTrend, {
  props,
  global: {
    stubs: { LoadingSpinner: { template: '<span data-testid="loading" />' } }
  }
})

describe('UserUsageTrend', () => {
  beforeEach(() => lineProps.mockClear())

  it('按用户聚合每日数据，并优先在图例显示邮箱', () => {
    mountTrend({ trendData: trend })

    const data = lineProps.mock.calls[0][0].data as {
      labels: string[]
      datasets: Array<{ label: string; data: number[] }>
    }

    expect(data.labels).toEqual(['2026-07-20', '2026-07-21'])
    expect(data.datasets).toHaveLength(2)
    expect(data.datasets[0]).toMatchObject({ label: 'alice@example.com', data: [1200, 2400] })
    expect(data.datasets[1]).toMatchObject({ label: 'bob@example.com', data: [0, 800] })
  })

  it('提供加载状态和空数据状态', () => {
    const loading = mountTrend({ trendData: [], loading: true })
    expect(loading.find('[data-testid="loading"]').exists()).toBe(true)

    const empty = mountTrend({ trendData: [] })
    expect(empty.text()).toContain('暂无数据')
  })
})
