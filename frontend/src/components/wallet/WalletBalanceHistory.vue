<template>
  <section class="overflow-hidden rounded-lg border border-gray-200 bg-white dark:border-dark-600 dark:bg-dark-800">
    <header class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-100 px-5 py-4 dark:border-dark-700">
      <div>
        <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('wallet.balanceHistory') }}</h2>
        <p v-if="!compact" class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('wallet.balanceHistoryHint') }}</p>
      </div>
      <span class="rounded-md bg-gray-100 px-2.5 py-1 text-xs font-medium text-gray-600 dark:bg-dark-700 dark:text-gray-300">
        {{ t('wallet.recordsCount', { count: balanceHistory.length }) }}
      </span>
    </header>

    <div v-if="loading" class="flex items-center justify-center py-16">
      <span class="h-7 w-7 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></span>
    </div>
    <div
      v-else-if="displayedHistory.length"
      :class="[
        'wallet-history-list divide-y divide-gray-100 dark:divide-dark-700',
        compact && displayedHistory.length > 5 && 'max-h-[360px] overflow-y-auto overscroll-contain',
      ]"
    >
      <div v-for="item in displayedHistory" :key="item.id" :class="['flex items-start px-5', compact ? 'gap-3 py-3.5' : 'gap-4 py-4']">
        <span :class="[
          'flex shrink-0 items-center justify-center rounded-lg',
          compact ? 'h-8 w-8' : 'h-10 w-10',
          item.value >= 0 ? 'bg-emerald-50 text-emerald-600 dark:bg-emerald-950/40 dark:text-emerald-400' : 'bg-red-50 text-red-600 dark:bg-red-950/40 dark:text-red-400',
        ]">
          <Icon name="dollar" :size="compact ? 'sm' : 'md'" />
        </span>
        <div class="min-w-0 flex-1">
          <div class="flex items-start justify-between gap-3">
            <p class="wallet-history-title min-w-0 text-sm font-medium leading-5 text-gray-900 dark:text-white">{{ itemTitle(item) }}</p>
            <p :class="['shrink-0 text-sm font-semibold tabular-nums', item.value >= 0 ? 'text-emerald-600 dark:text-emerald-400' : 'text-red-600 dark:text-red-400']">
              {{ item.value >= 0 ? '+' : '-' }}${{ Math.abs(item.value).toFixed(2) }}
            </p>
          </div>
          <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">{{ formatDateTime(item.used_at) }}</p>
          <p v-if="historyNote(item)" class="mt-1.5 whitespace-normal break-words text-xs leading-5 text-gray-400">{{ historyNote(item) }}</p>
        </div>
      </div>
    </div>
    <div v-else :class="['flex flex-col items-center text-center', compact ? 'py-10' : 'py-16']">
      <span class="flex h-12 w-12 items-center justify-center rounded-lg bg-gray-100 text-gray-400 dark:bg-dark-700 dark:text-gray-500">
        <Icon name="clock" size="lg" />
      </span>
      <p class="mt-3 text-sm font-medium text-gray-700 dark:text-gray-200">{{ t('wallet.noBalanceHistory') }}</p>
      <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('wallet.noBalanceHistoryHint') }}</p>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { lotteryAPI, redeemAPI, type RedeemHistoryItem } from '@/api'
import Icon from '@/components/icons/Icon.vue'
import { formatDateTime } from '@/utils/format'

const { t } = useI18n()
const props = withDefaults(defineProps<{ compact?: boolean }>(), { compact: false })
type WalletHistoryItem = Omit<RedeemHistoryItem, 'id'> & { id: number | string }

const history = ref<WalletHistoryItem[]>([])
const loading = ref(true)

const balanceHistory = computed(() => history.value.filter((item) =>
  ['balance', 'admin_balance', 'daily_checkin', 'usage_rebate', 'subscription_payment', 'lottery_reward', 'lottery_ticket_purchase', 'qq_bind_welcome_bonus'].includes(item.type)))
const displayedHistory = computed(() => balanceHistory.value)

function itemTitle(item: WalletHistoryItem): string {
  if (item.type === 'lottery_reward') return t('redeem.balanceAddedLottery')
  if (item.type === 'lottery_ticket_purchase') return t('redeem.balanceDeductedLotteryTicket')
  if (item.type === 'qq_bind_welcome_bonus') return t('redeem.balanceAddedQQBindingWelcome')
  if (item.type === 'balance') return t('redeem.balanceAddedRedeem')
  if (item.type === 'daily_checkin') return t('redeem.balanceAddedDailyCheckin')
  if (item.type === 'usage_rebate') return t('redeem.balanceAddedUsageRebate')
  if (item.type === 'subscription_payment') return t('redeem.balanceDeductedSubscription')
  return item.value >= 0 ? t('redeem.balanceAddedAdmin') : t('redeem.balanceDeductedAdmin')
}

function historyNote(item: WalletHistoryItem): string {
  if (!item.notes || ['daily_checkin', 'usage_rebate'].includes(item.type)) return ''
  const note = item.notes.trim()
  const title = itemTitle(item)
  return note === title || note.startsWith(`${title} `) ? '' : note
}

async function refresh() {
  loading.value = true
  try {
    const limit = props.compact ? 200 : 25
    const [redeemHistory, lotteryHistoryResponse] = await Promise.all([
      redeemAPI.getHistory(limit),
      lotteryAPI.listBalanceTransactions(limit),
    ])
    const lotteryHistory = lotteryHistoryResponse.data
    history.value = [
      ...redeemHistory,
      ...lotteryHistory.map((item) => ({
        id: `lottery-${item.id}`,
        code: '',
        type: item.transaction_type,
        value: item.amount,
        status: 'completed',
        used_at: item.created_at,
        created_at: item.created_at,
        notes: item.description,
      })),
    ].sort((left, right) => new Date(right.used_at).getTime() - new Date(left.used_at).getTime())
  } catch (error) {
    console.error('Failed to fetch wallet balance history:', error)
  } finally {
    loading.value = false
  }
}

defineExpose({ refresh })

onMounted(() => {
  void refresh()
})
</script>
