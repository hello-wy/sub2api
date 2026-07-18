import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import WalletBalanceHistory from '../WalletBalanceHistory.vue'

const getHistory = vi.hoisted(() => vi.fn())

vi.mock('@/api', () => ({
  redeemAPI: { getHistory },
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => ({
        'wallet.balanceHistory': '余额变动明细',
        'wallet.recordsCount': '2 条记录',
        'redeem.balanceAddedDailyCheckin': '每日签到',
        'redeem.balanceAddedAdmin': '余额充值（管理员）',
      }[key] || key),
    }),
  }
})

describe('WalletBalanceHistory', () => {
  beforeEach(() => {
    getHistory.mockReset().mockResolvedValue([
      {
        id: 1,
        code: 'CHECKIN-1',
        type: 'daily_checkin',
        value: 2.27,
        status: 'used',
        used_at: '2026-07-18T15:46:36Z',
        created_at: '2026-07-18T15:46:36Z',
        notes: '每日签到 2026/07/18 15:46:36',
      },
      {
        id: 2,
        code: 'ADMIN-1',
        type: 'admin_balance',
        value: 20,
        status: 'used',
        used_at: '2026-07-18T16:04:06Z',
        created_at: '2026-07-18T16:04:06Z',
        notes: '活动补发',
      },
    ])
  })

  it('keeps titles readable and removes redundant system notes', async () => {
    const wrapper = mount(WalletBalanceHistory, {
      props: { compact: true },
      global: {
        stubs: { Icon: true },
      },
    })
    await flushPromises()

    expect(getHistory).toHaveBeenCalledWith(200)
    const titles = wrapper.findAll('.wallet-history-title')
    expect(titles[0].text()).toBe('每日签到')
    expect(titles[0].classes()).not.toContain('truncate')
    expect(wrapper.text()).not.toContain('每日签到 2026/07/18 15:46:36')
    expect(wrapper.text()).toContain('活动补发')
  })
})
