import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import WalletBalanceHistory from '../WalletBalanceHistory.vue'

const getHistory = vi.hoisted(() => vi.fn())
const listBalanceTransactions = vi.hoisted(() => vi.fn())

vi.mock('@/api', () => ({
  redeemAPI: { getHistory },
  lotteryAPI: { listBalanceTransactions },
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
        'redeem.balanceDeductedSubscription': '订阅套餐（余额支付）',
        'redeem.balanceAddedLottery': '抽奖奖励',
        'redeem.balanceDeductedLotteryTicket': '购买抽奖次数',
      }[key] || key),
    }),
  }
})

describe('WalletBalanceHistory', () => {
  beforeEach(() => {
    listBalanceTransactions.mockReset().mockResolvedValue({ data: [] })
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
      {
        id: 3,
        code: 'SUBSCRIPTION-1',
        type: 'subscription_payment',
        value: -200,
        status: 'used',
        used_at: '2026-07-18T16:10:00Z',
        created_at: '2026-07-18T16:10:00Z',
        notes: '轻度包月',
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
    expect(listBalanceTransactions).toHaveBeenCalledWith(200)
    const titles = wrapper.findAll('.wallet-history-title')
    const checkinTitle = titles.find((title) => title.text() === '每日签到')
    expect(checkinTitle).toBeDefined()
    expect(checkinTitle?.classes()).not.toContain('truncate')
    expect(wrapper.text()).not.toContain('每日签到 2026/07/18 15:46:36')
    expect(wrapper.text()).toContain('活动补发')
    expect(wrapper.text()).toContain('订阅套餐（余额支付）')
    expect(wrapper.text()).toContain('-$200.00')
    expect(wrapper.text()).toContain('轻度包月')
  })

  it('uses an internal scrollbar after five records', async () => {
    getHistory.mockResolvedValue(Array.from({ length: 6 }, (_, index) => ({
      id: index + 1,
      code: `ADMIN-${index + 1}`,
      type: 'admin_balance',
      value: 1,
      status: 'used',
      used_at: '2026-07-18T16:04:06Z',
      created_at: '2026-07-18T16:04:06Z',
    })))

    const wrapper = mount(WalletBalanceHistory, {
      props: { compact: true },
      global: {
        stubs: { Icon: true },
      },
    })
    await flushPromises()

    const list = wrapper.get('.wallet-history-list')
    expect(list.classes()).toContain('max-h-[360px]')
    expect(list.classes()).toContain('overflow-y-auto')
  })

  it('exposes a refresh method that reloads the visible history', async () => {
    const wrapper = mount(WalletBalanceHistory, {
      props: { compact: true },
      global: {
        stubs: { Icon: true },
      },
    })
    await flushPromises()

    getHistory.mockResolvedValueOnce([{
      id: 4,
      code: 'SUBSCRIPTION-2',
      type: 'subscription_payment',
      value: -500,
      status: 'used',
      used_at: '2026-07-18T17:10:00Z',
      created_at: '2026-07-18T17:10:00Z',
      notes: '高级包月',
    }])

    await (wrapper.vm as unknown as { refresh: () => Promise<void> }).refresh()
    await flushPromises()

    expect(getHistory).toHaveBeenLastCalledWith(200)
    expect(wrapper.text()).toContain('-$500.00')
    expect(wrapper.text()).toContain('高级包月')
  })

  it('merges lottery wallet movements in chronological order', async () => {
    listBalanceTransactions.mockResolvedValueOnce({ data: [{
      id: 91,
      transaction_type: 'lottery_reward',
      amount: 30,
      description: '抽奖奖励 $30',
      created_at: '2026-07-18T18:00:00Z',
    }, {
      id: 92,
      transaction_type: 'lottery_ticket_purchase',
      amount: -30,
      description: '购买抽奖次数',
      created_at: '2026-07-18T18:01:00Z',
    }] })

    const wrapper = mount(WalletBalanceHistory, {
      props: { compact: true },
      global: { stubs: { Icon: true } },
    })
    await flushPromises()

    const titles = wrapper.findAll('.wallet-history-title')
    expect(titles[0].text()).toBe('购买抽奖次数')
    expect(wrapper.text()).toContain('抽奖奖励')
    expect(wrapper.text()).toContain('+$30.00')
    expect(wrapper.text()).toContain('-$30.00')
  })
})
