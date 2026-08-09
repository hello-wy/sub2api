import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import LotteryView from '../LotteryView.vue'

const { lotteryAPI, appStore, authStore, push } = vi.hoisted(() => ({
  lotteryAPI: {
    getStatus: vi.fn(),
    listDraws: vi.fn(),
    getPrizePool: vi.fn(),
    purchaseTicket: vi.fn(),
    draw: vi.fn(),
  },
  appStore: {
    showError: vi.fn(),
    showInfo: vi.fn(),
    showSuccess: vi.fn(),
  },
  authStore: {
    user: { balance: 100 },
    refreshUser: vi.fn(),
  },
  push: vi.fn(),
}))

vi.mock('@/api/lottery', () => ({ lotteryAPI }))
vi.mock('@/stores', () => ({
  useAppStore: () => appStore,
  useAuthStore: () => authStore,
}))
vi.mock('vue-router', async (importOriginal) => {
  const original = await importOriginal<typeof import('vue-router')>()
  return { ...original, useRouter: () => ({ push }) }
})

const status = {
  enabled: true,
  available_tickets: 1,
  pity_misses: 0,
  pity_remaining: 4,
  remaining_purchases: 5,
  recharge_tickets_today: 0,
  invitation_tickets_today: 0,
  purchased_tickets_today: 0,
  ticket_debt: 0,
}

const currentPrizePool = {
  prizes: [
    { id: 'none', label: '谢谢参与', type: 'none', probability: 0.5, eligible_for_pity: false },
    { id: 'quota-10', label: '$100', type: 'balance', amount: 100, probability: 0.5, eligible_for_pity: true },
  ],
  invitation_first_payment_amount: 20,
  invitation_consumption_amount: 100,
}

const mountLottery = () => mount(LotteryView, {
  global: {
    stubs: {
      AppLayout: { template: '<div><slot /></div>' },
      ScrollablePageLayout: { template: '<div><slot /></div>' },
      BaseDialog: {
        props: ['show'],
        emits: ['close'],
        template: '<div v-if="show"><slot /><slot name="footer" /></div>',
      },
      Icon: true,
      RouterLink: true,
    },
  },
})

describe('LotteryView', () => {
  beforeEach(() => {
    sessionStorage.clear()
    vi.clearAllMocks()
    authStore.user.balance = 100
    lotteryAPI.getStatus.mockResolvedValue({ data: status })
    lotteryAPI.listDraws.mockResolvedValue({ data: [] })
    lotteryAPI.getPrizePool.mockResolvedValue({ data: currentPrizePool })
    authStore.refreshUser.mockResolvedValue(undefined)
  })

  it('购买失败后重试复用同一请求 ID', async () => {
    lotteryAPI.purchaseTicket
      .mockRejectedValueOnce({ status: 0, message: 'Network error' })
      .mockResolvedValueOnce({ data: { ...status, available_tickets: 2, remaining_purchases: 4 } })
    const wrapper = mountLottery()
    await flushPromises()

    const openButton = wrapper.findAll('button').find((button) => button.text().includes('$30 购买次数'))
    expect(openButton).toBeDefined()
    await openButton!.trigger('click')
    await wrapper.findAll('button').find((button) => button.text() === '确认付款')!.trigger('click')
    await flushPromises()

    const firstRequestID = lotteryAPI.purchaseTicket.mock.calls[0][0]
    await wrapper.findAll('button').find((button) => button.text() === '确认付款')!.trigger('click')
    await flushPromises()

    expect(lotteryAPI.purchaseTicket).toHaveBeenCalledTimes(2)
    expect(lotteryAPI.purchaseTicket.mock.calls[1][0]).toBe(firstRequestID)
    expect(sessionStorage.getItem('lottery:purchase:pending-request-id')).toBeNull()
    wrapper.unmount()
  })

  it('抽奖失败后重试复用同一请求 ID', async () => {
    lotteryAPI.draw.mockRejectedValue({ status: 0, message: 'Network error' })
    const wrapper = mountLottery()
    await flushPromises()

    const drawButton = wrapper.find('.lottery-wheel-action')
    await drawButton.trigger('click')
    await flushPromises()
    const firstRequestID = lotteryAPI.draw.mock.calls[0][0]
    await drawButton.trigger('click')
    await flushPromises()

    expect(lotteryAPI.draw).toHaveBeenCalledTimes(2)
    expect(lotteryAPI.draw.mock.calls[1][0]).toBe(firstRequestID)
    expect(sessionStorage.getItem('lottery:draw:pending-request-id')).toBe(firstRequestID)
    wrapper.unmount()
  })

  it('历史记录使用开奖快照而不是当前奖池金额', async () => {
    lotteryAPI.listDraws.mockResolvedValueOnce({
      data: [{
        id: 1,
        request_id: 'lottery-history-1',
        prize_id: 'quota-10',
        prize_label: '$10',
        prize_type: 'balance',
        amount: 10,
        guaranteed: false,
        created_at: '2026-08-09T00:00:00Z',
      }],
    })
    const wrapper = mountLottery()
    await flushPromises()

    expect(wrapper.find('.lottery-recent-item').text()).toContain('$10')
    expect(wrapper.find('.lottery-recent-item').text()).not.toContain('$100')
    wrapper.unmount()
  })
})
