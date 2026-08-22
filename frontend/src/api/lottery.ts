import { apiClient } from './client'

export interface LotteryStatus {
  enabled: boolean
  available_tickets: number
  pity_misses: number
  pity_remaining: number
  remaining_purchases: number
  recharge_tickets_today: number
  invitation_tickets_today: number
  purchased_tickets_today: number
  ticket_debt: number
}

export interface LotteryDraw {
  id: number
  request_id: string
  prize_id: string
  prize_label: string
  prize_type: 'none' | 'balance' | 'subscription'
  amount: number
  balance_before?: number
  balance_after?: number
  guaranteed: boolean
  redeem_code?: string
  redeem_status?: 'unused' | 'used' | 'expired'
  redeem_expires_at?: string
  subscription_validity_days?: number
  created_at: string
}

export interface LotteryRecentWinner {
  id: number
  display_name: string
  prize_id: string
  prize_label: string
  prize_type: 'none' | 'balance' | 'subscription'
  amount: number
  probability: number
  guaranteed: boolean
  created_at: string
}

export interface LotteryPrizeConfig {
  id: string
  label: string
  type: 'none' | 'balance' | 'subscription'
  amount?: number
  probability: number
  subscription_group_id?: number
  subscription_plan_id?: number
  eligible_for_pity: boolean
}

export interface LotteryPrizePoolConfig {
  enabled: boolean
  prizes: LotteryPrizeConfig[]
  invitation_first_payment_amount: number
  invitation_consumption_amount: number
  purchase_price: number
}

export interface LotteryBalanceTransaction {
  id: number
  transaction_type: 'lottery_reward' | 'lottery_ticket_purchase'
  amount: number
  description: string
  created_at: string
}

export const lotteryAPI = {
  getPrizePool() {
    return apiClient.get<LotteryPrizePoolConfig>('/lottery/prizes', {
      params: { cache_buster: Date.now() },
      headers: { 'Cache-Control': 'no-cache' },
    })
  },
  getStatus() {
    return apiClient.get<LotteryStatus>('/lottery/status')
  },

  purchaseTicket(idempotencyKey: string) {
    return apiClient.post<LotteryStatus>('/lottery/tickets/purchase', {}, {
      headers: { 'Idempotency-Key': idempotencyKey },
    })
  },

  draw(idempotencyKey: string) {
    return apiClient.post<LotteryDraw>('/lottery/draw', {}, {
      headers: { 'Idempotency-Key': idempotencyKey },
    })
  },

  listDraws(limit = 20) {
    return apiClient.get<LotteryDraw[]>('/lottery/draws', { params: { limit } })
  },

  listRecentWinners(limit = 30) {
    return apiClient.get<LotteryRecentWinner[]>('/lottery/recent-winners', { params: { limit } })
  },

  listBalanceTransactions(limit = 25) {
    return apiClient.get<LotteryBalanceTransaction[]>('/lottery/balance-transactions', { params: { limit } })
  },
}
