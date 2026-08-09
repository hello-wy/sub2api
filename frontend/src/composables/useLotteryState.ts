import { computed, ref } from 'vue'
import type { LotteryStatus } from '@/api/lottery'

const freeTickets = ref(0)
const lotteryEnabled = ref(true)
const remainingPurchases = ref(5)
const rechargeTicketsToday = ref(0)
const invitationTicketsToday = ref(0)
const purchasedTicketsToday = ref(0)
const misses = ref(0)
const ticketDebt = ref(0)

function applyLotteryStatus(status: LotteryStatus): void {
  lotteryEnabled.value = status.enabled
  freeTickets.value = status.available_tickets
  remainingPurchases.value = status.remaining_purchases
  rechargeTicketsToday.value = status.recharge_tickets_today
  invitationTicketsToday.value = status.invitation_tickets_today
  purchasedTicketsToday.value = status.purchased_tickets_today
  misses.value = status.pity_misses
  ticketDebt.value = status.ticket_debt
}

function setLotteryEnabled(enabled: boolean): void {
  lotteryEnabled.value = enabled
  if (!enabled) freeTickets.value = 0
}

export function useLotteryState() {
  const hasLotteryTickets = computed(() => lotteryEnabled.value && freeTickets.value > 0)

  return {
    freeTickets,
    lotteryEnabled,
    remainingPurchases,
    rechargeTicketsToday,
    invitationTicketsToday,
    purchasedTicketsToday,
    misses,
    ticketDebt,
    hasLotteryTickets,
    applyLotteryStatus,
    setLotteryEnabled,
  }
}
