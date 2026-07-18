<template>
  <div
    class="group relative flex flex-col overflow-hidden rounded-lg border border-gray-200 bg-white transition-colors duration-200 hover:border-primary-300 dark:border-dark-600 dark:bg-dark-800 dark:hover:border-primary-500/60"
  >
    <div class="h-1 bg-primary-500" />

    <div class="flex flex-1 flex-col p-4">
      <!-- Header: name + badge + price -->
      <div class="mb-3">
        <div class="flex min-w-0 items-center justify-between gap-2">
          <h3 class="truncate text-base font-bold text-gray-900 dark:text-white">{{ plan.name }}</h3>
          <span class="shrink-0 rounded-md border border-primary-200 bg-primary-50 px-2 py-0.5 text-[11px] font-medium text-primary-700 dark:border-primary-500/30 dark:bg-primary-500/10 dark:text-primary-300">
            {{ pLabel }}
          </span>
        </div>
        <p v-if="plan.description" class="mt-0.5 text-xs leading-relaxed text-gray-500 dark:text-dark-400 line-clamp-2">
          {{ plan.description }}
        </p>
        <div class="mt-3 flex items-end justify-between gap-3">
          <div class="flex items-baseline gap-1">
            <span class="text-sm font-semibold text-primary-600 dark:text-primary-400">¥</span>
            <span class="text-2xl font-extrabold tracking-tight text-gray-950 dark:text-white">{{ displayPrice }}</span>
          </div>
          <div class="shrink-0 text-right">
            <span class="text-[11px] text-gray-400 dark:text-dark-500">/ {{ validitySuffix }}</span>
            <div v-if="plan.original_price" class="mt-0.5 flex items-center justify-end gap-1.5">
              <span class="text-xs text-gray-400 line-through dark:text-dark-500">¥{{ displayOriginalPrice }}</span>
              <span class="rounded bg-primary-50 px-1.5 py-0.5 text-[10px] font-semibold text-primary-700 dark:bg-primary-500/10 dark:text-primary-300">{{ discountText }}</span>
            </div>
          </div>
        </div>
      </div>

      <!-- Group quota info (compact) -->
      <div class="mb-3 grid grid-cols-2 gap-x-3 gap-y-1 rounded-lg bg-gray-50 px-3 py-2 text-xs dark:bg-dark-700/50">
        <div class="flex items-center justify-between gap-2">
          <span class="text-gray-400 dark:text-dark-500">{{ t('payment.planCard.rate') }}</span>
          <span class="shrink-0 font-medium text-gray-700 dark:text-gray-300">{{ rateDisplay }}</span>
        </div>
        <div v-if="hasPeakRate" class="col-span-2 flex items-center justify-between gap-2">
          <span class="text-gray-400 dark:text-dark-500">{{ t('payment.planCard.peakRate') }}</span>
          <span class="text-right font-medium text-amber-700 dark:text-amber-300">{{ peakRateDisplay }}</span>
        </div>
        <div v-if="plan.daily_limit_usd != null" class="flex items-center justify-between gap-2">
          <span class="text-gray-400 dark:text-dark-500">{{ t('payment.planCard.dailyLimit') }}</span>
          <span class="shrink-0 font-medium text-gray-700 dark:text-gray-300">${{ plan.daily_limit_usd }}</span>
        </div>
        <div v-if="plan.weekly_limit_usd != null" class="flex items-center justify-between gap-2">
          <span class="text-gray-400 dark:text-dark-500">{{ t('payment.planCard.weeklyLimit') }}</span>
          <span class="shrink-0 font-medium text-gray-700 dark:text-gray-300">${{ plan.weekly_limit_usd }}</span>
        </div>
        <div v-if="plan.monthly_limit_usd != null" class="flex items-center justify-between gap-2">
          <span class="text-gray-400 dark:text-dark-500">{{ t('payment.planCard.monthlyLimit') }}</span>
          <span class="shrink-0 font-medium text-gray-700 dark:text-gray-300">${{ plan.monthly_limit_usd }}</span>
        </div>
        <div v-if="plan.daily_limit_usd == null && plan.weekly_limit_usd == null && plan.monthly_limit_usd == null" class="flex items-center justify-between">
          <span class="text-gray-400 dark:text-dark-500">{{ t('payment.planCard.quota') }}</span>
          <span class="font-medium text-gray-700 dark:text-gray-300">{{ t('payment.planCard.unlimited') }}</span>
        </div>
        <div v-if="modelScopeLabels.length > 0" class="col-span-2 flex items-center justify-between">
          <span class="text-gray-400 dark:text-dark-500">{{ t('payment.planCard.models') }}</span>
          <div class="flex flex-wrap justify-end gap-1">
            <span v-for="scope in modelScopeLabels" :key="scope"
              class="rounded bg-gray-200/80 px-1.5 py-0.5 text-[10px] font-medium text-gray-600 dark:bg-dark-600 dark:text-gray-300">
              {{ scope }}
            </span>
          </div>
        </div>
      </div>

      <!-- Features list (compact) -->
      <div v-if="plan.features.length > 0" class="mb-3 space-y-1">
        <div v-for="feature in plan.features" :key="feature" class="flex items-start gap-1.5">
          <svg class="mt-0.5 h-3.5 w-3.5 flex-shrink-0 text-primary-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
            <path stroke-linecap="round" stroke-linejoin="round" d="M4.5 12.75l6 6 9-13.5" />
          </svg>
          <span class="text-xs text-gray-600 dark:text-gray-300">{{ feature }}</span>
        </div>
      </div>

      <div class="flex-1" />

      <div class="mb-3 grid grid-cols-2 rounded-lg bg-gray-100 p-1 dark:bg-dark-700/70">
        <button
          type="button"
          :class="paymentSource === 'recharge' ? sourceActiveClass : sourceIdleClass"
          :aria-pressed="paymentSource === 'recharge'"
          :disabled="disabled || !rechargeAvailable"
          @click="paymentSource = 'recharge'"
        >
          <Icon name="creditCard" size="sm" />
          {{ t('wallet.subscriptionPaymentRecharge') }}
        </button>
        <button
          type="button"
          :class="paymentSource === 'balance' ? sourceActiveClass : sourceIdleClass"
          :aria-pressed="paymentSource === 'balance'"
          :disabled="disabled"
          @click="paymentSource = 'balance'"
        >
          <Icon name="dollar" size="sm" />
          {{ t('wallet.subscriptionPaymentBalance') }}
        </button>
      </div>

      <div class="mb-3 grid min-h-10 grid-cols-2 gap-3 text-xs">
        <template v-if="paymentSource === 'recharge'">
          <div>
            <span class="block text-gray-500 dark:text-gray-400">{{ t('wallet.subscriptionPlanDiscount') }}</span>
            <strong class="mt-0.5 block text-primary-600 dark:text-primary-400">{{ discountText || t('wallet.subscriptionNoDiscount') }}</strong>
          </div>
          <div class="text-right">
            <span class="block text-gray-500 dark:text-gray-400">{{ t('wallet.subscriptionSettlementAmount') }}</span>
            <strong :class="['mt-0.5 block', rechargeAvailable ? 'text-gray-900 dark:text-white' : 'text-amber-600 dark:text-amber-300']">
              {{ rechargeAvailable ? rechargeAmountLabel : t('payment.notAvailable') }}
            </strong>
          </div>
        </template>
        <template v-else-if="hasEnoughBalance">
          <div>
            <span class="block text-gray-500 dark:text-gray-400">{{ t('wallet.subscriptionBalanceNoDiscount') }}</span>
            <strong class="mt-0.5 block text-gray-900 dark:text-white">-${{ effectiveBalancePrice.toFixed(2) }}</strong>
          </div>
          <div class="text-right">
            <span class="block text-gray-500 dark:text-gray-400">{{ t('wallet.subscriptionBalanceSettlement') }}</span>
            <strong class="mt-0.5 block text-primary-600 dark:text-primary-400">${{ balanceAfterPayment.toFixed(2) }}</strong>
          </div>
        </template>
        <template v-else>
          <span class="col-span-2 self-center text-red-600 dark:text-red-400">
            {{ t('wallet.subscriptionBalanceInsufficient') }} · {{ t('wallet.subscriptionBalanceRequired') }} ${{ effectiveBalancePrice.toFixed(2) }}
          </span>
        </template>
      </div>

      <button
        type="button"
        class="btn btn-primary w-full py-2.5 text-sm font-semibold"
        :disabled="submitDisabled"
        @click="emit('subscribe', plan, paymentSource)"
      >
        {{ submitting ? t('common.processing') : t('wallet.subscribeAction') }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { SubscriptionPlan } from '@/types/payment'
import { useAppStore } from '@/stores/app'
import Icon from '@/components/icons/Icon.vue'
import { hasPeakRate as groupHasPeakRate, formatPeakRateWindow, serverTimezoneLabel } from '@/utils/peak-rate'
import { platformLabel } from '@/utils/platformColors'

const props = withDefaults(defineProps<{
  plan: SubscriptionPlan
  subscriptionUsdToCnyRate?: number
  availableBalance?: number
  balancePrice?: number
  rechargeAvailable?: boolean
  rechargeAmountLabel?: string
  disabled?: boolean
  submitting?: boolean
}>(), {
  subscriptionUsdToCnyRate: 0,
  availableBalance: 0,
  balancePrice: 0,
  rechargeAvailable: true,
  rechargeAmountLabel: '',
  disabled: false,
  submitting: false,
})
const emit = defineEmits<{
  subscribe: [plan: SubscriptionPlan, source: 'recharge' | 'balance']
}>()
const { t } = useI18n()
const paymentSource = ref<'recharge' | 'balance'>(props.rechargeAvailable ? 'recharge' : 'balance')

const platform = computed(() => props.plan.group_platform || '')
const pLabel = computed(() => platformLabel(platform.value))

function cnyPrice(value: number): string {
  const rate = Number.isFinite(props.subscriptionUsdToCnyRate) && props.subscriptionUsdToCnyRate > 0
    ? props.subscriptionUsdToCnyRate
    : 1
  return new Intl.NumberFormat('zh-CN', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(value * rate)
}

const displayPrice = computed(() => cnyPrice(props.plan.price))
const displayOriginalPrice = computed(() => cnyPrice(props.plan.original_price ?? 0))
const effectiveBalancePrice = computed(() => props.balancePrice > 0 ? props.balancePrice : props.plan.price)
const hasEnoughBalance = computed(() => effectiveBalancePrice.value > 0 && props.availableBalance + 1e-9 >= effectiveBalancePrice.value)
const balanceAfterPayment = computed(() => Math.max(0, props.availableBalance - effectiveBalancePrice.value))
const submitDisabled = computed(() => props.disabled
  || props.submitting
  || (paymentSource.value === 'recharge' ? !props.rechargeAvailable : !hasEnoughBalance.value))

const sourceActiveClass = 'flex min-h-9 items-center justify-center gap-1.5 rounded-md bg-white px-2 text-xs font-semibold text-primary-700 shadow-sm outline-none transition-colors focus-visible:ring-2 focus-visible:ring-primary-500/40 dark:bg-dark-800 dark:text-primary-300'
const sourceIdleClass = 'flex min-h-9 items-center justify-center gap-1.5 rounded-md px-2 text-xs font-medium text-gray-500 outline-none transition-colors hover:text-gray-800 focus-visible:ring-2 focus-visible:ring-primary-500/40 disabled:cursor-not-allowed disabled:opacity-40 dark:text-gray-400 dark:hover:text-gray-200'

const discountText = computed(() => {
  if (!props.plan.original_price || props.plan.original_price <= 0) return ''
  const pct = Math.round((1 - props.plan.price / props.plan.original_price) * 100)
  return pct > 0 ? `-${pct}%` : ''
})

const rateDisplay = computed(() => {
  const rate = props.plan.rate_multiplier ?? 1
  return `×${Number(rate.toPrecision(10))}`
})

const appStore = useAppStore()

const hasPeakRate = computed(() => groupHasPeakRate(props.plan))

const peakRateDisplay = computed(() => {
  return formatPeakRateWindow(props.plan, serverTimezoneLabel(appStore.cachedPublicSettings?.server_utc_offset))
})

const MODEL_SCOPE_LABELS: Record<string, string> = {
  claude: 'Claude',
  gemini_text: 'Gemini',
  gemini_image: 'Imagen',
}

const modelScopeLabels = computed(() => {
  if (platform.value !== 'antigravity') return []
  const scopes = props.plan.supported_model_scopes
  if (!scopes || scopes.length === 0) return []
  return scopes.map(s => MODEL_SCOPE_LABELS[s] || s)
})

const validitySuffix = computed(() => {
  const u = props.plan.validity_unit || 'day'
  if (u === 'month') return t('payment.perMonth')
  if (u === 'year') return t('payment.perYear')
  return `${props.plan.validity_days}${t('payment.days')}`
})
</script>
