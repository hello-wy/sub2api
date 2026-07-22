<template>
  <AppLayout>
    <ScrollablePageLayout>
      <div class="mx-auto max-w-7xl space-y-6">
      <div v-if="loading" class="flex items-center justify-center py-20">
        <div class="h-8 w-8 animate-spin rounded-full border-4 border-primary-500 border-t-transparent"></div>
      </div>
      <template v-else>
        <template v-if="paymentPhase === 'paying'">
          <PaymentStatusPanel
            :order-id="paymentState.orderId"
            :qr-code="paymentState.qrCode"
            :expires-at="paymentState.expiresAt"
            :payment-type="paymentState.paymentType"
            :pay-url="paymentState.payUrl"
            :order-type="paymentState.orderType"
            :currency="paymentState.currency || selectedCurrency"
            @done="onPaymentDone"
            @success="onPaymentSuccess"
            @settled="onPaymentSettled"
          />
        </template>
        <template v-else>
          <header class="px-1">
            <div>
              <h1 class="text-2xl font-semibold text-gray-950 dark:text-white">{{ t('wallet.catalogTitle') }}</h1>
              <p class="mt-1.5 max-w-2xl text-sm leading-6 text-gray-500 dark:text-gray-400">{{ t('wallet.catalogHint') }}</p>
            </div>
          </header>

          <div class="grid items-start gap-5 xl:grid-cols-[minmax(0,1fr)_320px]">
            <main class="min-w-0 space-y-5">
              <section id="wallet-recharge" class="scroll-mt-6 space-y-5 rounded-lg border border-gray-200 bg-white p-5 dark:border-dark-600 dark:bg-dark-800 sm:p-6">
                <div>
                  <h2 class="text-lg font-semibold text-gray-950 dark:text-white">{{ t('wallet.rechargeSectionTitle') }}</h2>
                  <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('wallet.rechargeSectionHint') }}</p>
                </div>
                <div v-if="enabledMethods.length === 0" class="rounded-lg bg-gray-50 py-14 text-center dark:bg-dark-700/60">
                  <Icon name="creditCard" size="xl" class="mx-auto mb-3 text-gray-300 dark:text-gray-500" />
                  <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('payment.notAvailable') }}</p>
                </div>
                <template v-else>
                  <div class="rounded-lg border border-gray-200 p-4 dark:border-dark-600">
                    <PaymentMethodSelector
                      :methods="methodOptions"
                      :selected="selectedMethod"
                      @select="selectedMethod = $event"
                    />
                  </div>
                  <RechargePackageSelector
                    v-model="amount"
                    :multiplier="balanceRechargeMultiplier"
                    :min="globalMinAmount"
                    :max="globalMaxAmount"
                    :format-amount="formatSelectedPaymentAmount"
                  />
                  <p v-if="amountError" class="text-xs text-amber-600 dark:text-amber-300">{{ amountError }}</p>
                  <div v-if="validAmount > 0" class="grid gap-4 rounded-lg bg-gray-50 p-4 md:grid-cols-[minmax(0,1fr)_auto] md:items-center dark:bg-dark-700/60">
                    <dl class="grid grid-cols-2 gap-x-5 gap-y-3 text-sm sm:grid-cols-4">
                      <div>
                        <dt class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.paymentAmount') }}</dt>
                        <dd class="mt-1 font-semibold text-gray-900 dark:text-white">{{ formatSelectedPaymentAmount(validAmount) }}</dd>
                      </div>
                      <div v-if="hasLoyaltyDiscount">
                        <dt class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.loyaltyDiscount', { discount: loyaltyDiscountPercent }) }}</dt>
                        <dd class="mt-1 font-semibold text-emerald-600 dark:text-emerald-400">-{{ formatSelectedPaymentAmount(loyaltyDiscountAmount) }}</dd>
                      </div>
                      <div v-if="hasLoyaltyDiscount">
                        <dt class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.discountedPaymentAmount') }}</dt>
                        <dd class="mt-1 font-semibold text-gray-900 dark:text-white">{{ formatSelectedPaymentAmount(paymentBaseAmount) }}</dd>
                      </div>
                      <div v-if="feeRate > 0">
                        <dt class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.fee') }} ({{ feeRate }}%)</dt>
                        <dd class="mt-1 font-semibold text-gray-900 dark:text-white">{{ formatSelectedPaymentAmount(feeAmount) }}</dd>
                      </div>
                      <div :class="{ 'sm:col-start-1': hasLoyaltyDiscount && feeRate === 0 }">
                        <dt class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.creditedBalance') }}</dt>
                        <dd class="mt-1 font-semibold text-primary-600 dark:text-primary-400">${{ creditedAmount.toFixed(2) }}</dd>
                      </div>
                      <div>
                        <dt class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.actualPay') }}</dt>
                        <dd class="mt-1 text-base font-bold text-primary-600 dark:text-primary-400">{{ formatSelectedPaymentAmount(totalAmount) }}</dd>
                      </div>
                    </dl>
                    <button :class="['btn min-w-[190px] px-5 py-3 text-sm font-semibold', paymentButtonClass]" :disabled="!canSubmit || submitting" @click="handleSubmitRecharge">
                      <span v-if="submitting" class="flex items-center justify-center gap-2">
                        <span class="h-4 w-4 animate-spin rounded-full border-2 border-white border-t-transparent"></span>
                        {{ t('common.processing') }}
                      </span>
                      <span v-else>{{ t('payment.createOrder') }} {{ formatSelectedPaymentAmount(totalAmount) }}</span>
                    </button>
                  </div>
                </template>
              </section>

              <section id="wallet-subscription" class="scroll-mt-6 space-y-5 rounded-lg border border-gray-200 bg-white p-5 dark:border-dark-600 dark:bg-dark-800 sm:p-6">
                <div>
                  <h2 class="text-lg font-semibold text-gray-950 dark:text-white">{{ t('wallet.subscriptionSectionTitle') }}</h2>
                  <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('wallet.subscriptionSectionHint') }}</p>
                </div>
                <div v-if="checkout.plans.length === 0" class="rounded-lg bg-gray-50 py-14 text-center dark:bg-dark-700/60">
                  <Icon name="gift" size="xl" class="mx-auto mb-3 text-gray-300 dark:text-gray-500" />
                  <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('payment.noPlans') }}</p>
                </div>
                <div v-else :class="planGridClass">
                  <SubscriptionPlanCard
                    v-for="plan in checkout.plans"
                    :key="plan.id"
                    :plan="plan"
                    :subscription-usd-to-cny-rate="subscriptionUsdToCnyRate"
                    :available-balance="availableBalance"
                    :balance-price="subscriptionBalancePrice(plan)"
                    :recharge-available="canRechargeSubscriptionPlan(plan)"
                    :recharge-before-discount-label="formatSubscriptionBeforeDiscountAmount(plan)"
                    :recharge-after-discount-label="formatSubscriptionAfterDiscountAmount(plan)"
                    :recharge-amount-label="formatSubscriptionRechargeAmount(plan)"
                    :loyalty-discount-label="subscriptionLoyaltyDiscountLabel"
                    :disabled="submitting"
                    :submitting="submitting && submittingPlanId === plan.id"
                    @subscribe="subscribeToPlan"
                  />
                </div>
              </section>

              <section v-if="checkout.help_text || checkout.help_image_url" class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-600 dark:bg-dark-800">
                <div class="flex flex-col items-center gap-3">
                  <img v-if="checkout.help_image_url" :src="checkout.help_image_url" :alt="t('payment.helpImageAlt')" class="h-40 max-w-full cursor-pointer rounded-lg object-contain transition-opacity hover:opacity-80" @click="previewImage = checkout.help_image_url" />
                  <p v-if="checkout.help_text" class="text-center text-sm text-gray-500 dark:text-gray-400">{{ checkout.help_text }}</p>
                </div>
              </section>

            </main>

            <aside class="space-y-5">
              <section class="rounded-lg border border-gray-200 bg-white p-5 dark:border-dark-600 dark:bg-dark-800">
                <div class="flex items-start justify-between gap-3">
                  <div>
                    <p class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('redeem.currentBalance') }}</p>
                    <p class="mt-4 text-4xl font-bold tabular-nums text-gray-950 dark:text-white">${{ user?.balance?.toFixed(2) || '0.00' }}</p>
                  </div>
                  <button type="button" class="flex h-10 w-10 items-center justify-center rounded-lg border border-gray-200 text-gray-500 transition hover:border-primary-300 hover:text-primary-600 active:scale-95 dark:border-dark-600 dark:text-gray-400" :aria-label="t('wallet.refreshBalance')" :title="t('wallet.refreshBalance')" :disabled="refreshingSummary" @click="refreshWalletSummary">
                    <Icon name="refresh" size="md" :class="{ 'animate-spin': refreshingSummary }" />
                  </button>
                </div>
              </section>

              <div id="wallet-redeem" class="scroll-mt-6">
                <WalletRedeemPanel compact />
              </div>

              <section id="wallet-subscription-summary" class="scroll-mt-6 overflow-hidden rounded-lg border border-gray-200 bg-white dark:border-dark-600 dark:bg-dark-800">
                <header class="flex items-center justify-between gap-3 border-b border-gray-100 px-5 py-4 dark:border-dark-700">
                  <div class="flex items-center gap-1.5">
                    <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('wallet.activeSubscriptions') }}</h2>
                    <HelpTooltip :content="t('wallet.singleSubscriptionNotice')" width-class="w-72" />
                  </div>
                </header>
                <div v-if="activeSubscriptions.length" class="divide-y divide-gray-100 dark:divide-dark-700">
                  <div v-for="sub in activeSubscriptions" :key="sub.id" class="px-5 py-4">
                    <div class="flex items-center gap-2">
                      <span :class="['h-2 w-2 shrink-0 rounded-full', platformAccentBarClass(sub.group?.platform || '')]"></span>
                      <p class="min-w-0 flex-1 truncate text-sm font-semibold text-gray-900 dark:text-white">{{ sub.group?.name || t('payment.groupFallback', { id: sub.group_id }) }}</p>
                      <span class="text-xs font-medium text-emerald-600 dark:text-emerald-400">{{ t('userSubscriptions.status.active') }}</span>
                    </div>
                    <p class="mt-2 text-xs text-gray-500 dark:text-gray-400">
                      <span v-if="sub.expires_at">{{ t('userSubscriptions.daysRemaining', { days: getDaysRemaining(sub.expires_at) }) }}</span>
                      <span v-else>{{ t('userSubscriptions.noExpiration') }}</span>
                      <span class="mx-1.5">·</span>
                      <span>{{ t('payment.planCard.rate') }} ×{{ sub.group?.rate_multiplier ?? 1 }}</span>
                    </p>
                    <div v-if="subscriptionUsageRows(sub).length" class="mt-3 space-y-3">
                      <div v-for="usage in subscriptionUsageRows(sub)" :key="usage.period" class="space-y-1.5">
                        <div class="flex items-center justify-between gap-3 text-xs">
                          <span class="font-medium text-gray-600 dark:text-gray-300">{{ usage.label }}</span>
                          <span class="shrink-0 tabular-nums text-gray-500 dark:text-gray-400">${{ usage.used.toFixed(2) }} / ${{ usage.limit.toFixed(2) }}</span>
                        </div>
                        <div class="h-1.5 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700">
                          <div :class="['h-full rounded-full transition-[width] duration-300', subscriptionUsageBarClass(usage.percentage)]" :style="{ width: `${usage.percentage}%` }"></div>
                        </div>
                      </div>
                    </div>
                    <div v-else class="mt-3 flex items-center justify-between rounded-md bg-gray-50 px-3 py-2 text-xs dark:bg-dark-700/60">
                      <span class="text-gray-500 dark:text-gray-400">{{ t('userSubscriptions.usage') }}</span>
                      <span class="font-medium text-emerald-600 dark:text-emerald-400">{{ t('userSubscriptions.unlimited') }}</span>
                    </div>
                  </div>
                </div>
                <div v-else class="px-5 py-8 text-center text-sm text-gray-500 dark:text-gray-400">{{ t('payment.noActiveSubscription') }}</div>
              </section>

              <div id="wallet-history" class="scroll-mt-6">
                <WalletBalanceHistory ref="walletHistoryRef" compact />
              </div>
            </aside>
          </div>
        </template>
      </template>
    </div>
    <ConfirmDialog
      :show="Boolean(pendingSubscriptionPurchase)"
      :title="subscriptionPurchaseDialogTitle"
      :message="subscriptionPurchaseDialogMessage"
      :warning-message="subscriptionPurchaseDialogWarningMessage"
      :confirm-text="subscriptionPurchaseDialogConfirmText"
      @confirm="confirmPendingSubscriptionPurchase"
      @cancel="cancelPendingSubscriptionPurchase"
    >
      <dl
        v-if="pendingSubscriptionPurchase?.stage === 'balance'"
        class="overflow-hidden rounded-lg border border-gray-200 bg-gray-50 dark:border-dark-600 dark:bg-dark-700/60"
      >
        <div class="flex items-center justify-between gap-4 border-b border-gray-200 px-4 py-3 dark:border-dark-600">
          <dt class="text-xs text-gray-500 dark:text-gray-400">{{ t('wallet.subscriptionConfirmPlan') }}</dt>
          <dd class="min-w-0 truncate text-sm font-semibold text-gray-900 dark:text-white">
            {{ pendingSubscriptionPurchase.plan.name }}
          </dd>
        </div>
        <div class="grid grid-cols-3 divide-x divide-gray-200 px-1 py-3 dark:divide-dark-600">
          <div class="px-3">
            <dt class="text-xs text-gray-500 dark:text-gray-400">{{ t('wallet.subscriptionBalanceRequired') }}</dt>
            <dd class="mt-1 text-sm font-semibold tabular-nums text-gray-900 dark:text-white">
              ${{ pendingSubscriptionBalancePrice.toFixed(2) }}
            </dd>
          </div>
          <div class="px-3 text-center">
            <dt class="text-xs text-gray-500 dark:text-gray-400">{{ t('wallet.subscriptionBalanceAvailable') }}</dt>
            <dd class="mt-1 text-sm font-semibold tabular-nums text-gray-700 dark:text-gray-200">
              ${{ availableBalance.toFixed(2) }}
            </dd>
          </div>
          <div class="px-3 text-right">
            <dt class="text-xs text-gray-500 dark:text-gray-400">{{ t('wallet.subscriptionBalanceSettlement') }}</dt>
            <dd class="mt-1 text-sm font-semibold tabular-nums text-primary-600 dark:text-primary-400">
              ${{ pendingSubscriptionBalanceAfterPayment.toFixed(2) }}
            </dd>
          </div>
        </div>
      </dl>
    </ConfirmDialog>
    <!-- Image Preview Overlay -->
    <Teleport to="body">
      <Transition name="modal">
        <div v-if="previewImage" class="fixed inset-0 z-[60] flex items-center justify-center bg-black/70 backdrop-blur-sm" @click="previewImage = ''">
          <img :src="previewImage" alt="" class="max-h-[85vh] max-w-[90vw] rounded-xl object-contain shadow-2xl" />
        </div>
      </Transition>
    </Teleport>
    </ScrollablePageLayout>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, nextTick, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { usePaymentStore } from '@/stores/payment'
import { useSubscriptionStore } from '@/stores/subscriptions'
import { useAppStore } from '@/stores'
import { paymentAPI } from '@/api/payment'
import { extractApiErrorMessage, extractI18nErrorMessage } from '@/utils/apiError'
import { isMobileDevice } from '@/utils/device'
import type { SubscriptionPlan, CheckoutInfoResponse, CreateOrderResult, OrderType } from '@/types/payment'
import type { UserSubscription } from '@/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import ScrollablePageLayout from '@/components/layout/ScrollablePageLayout.vue'
import PaymentMethodSelector from '@/components/payment/PaymentMethodSelector.vue'
import RechargePackageSelector from '@/components/wallet/RechargePackageSelector.vue'
import WalletBalanceHistory from '@/components/wallet/WalletBalanceHistory.vue'
import WalletRedeemPanel from '@/components/wallet/WalletRedeemPanel.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import { METHOD_ORDER, getPaymentPopupFeatures, isBuiltInAlipayMethod, isBuiltInWxpayMethod } from '@/components/payment/providerConfig'
import {
  PAYMENT_RECOVERY_STORAGE_KEY,
  buildCreateOrderPayload,
  clearPaymentRecoverySnapshot,
  decidePaymentLaunch,
  getVisibleMethods,
  normalizeVisibleMethod,
  readPaymentRecoverySnapshot,
  type PaymentRecoverySnapshot,
  writePaymentRecoverySnapshot,
} from '@/components/payment/paymentFlow'
import { platformAccentBarClass } from '@/utils/platformColors'
import SubscriptionPlanCard from '@/components/payment/SubscriptionPlanCard.vue'
import PaymentStatusPanel from '@/components/payment/PaymentStatusPanel.vue'
import Icon from '@/components/icons/Icon.vue'
import { DEFAULT_PAYMENT_CURRENCY, formatPaymentAmount, normalizePaymentCurrency } from '@/components/payment/currency'
import type { PaymentMethodOption } from '@/components/payment/PaymentMethodSelector.vue'
import { buildPaymentErrorToastMessage, describePaymentScenarioError } from './paymentUx'
import { hasWechatResumeQuery, parseWechatResumeRoute, stripWechatResumeQuery } from './paymentWechatResume'

const i18n = useI18n()
const { t } = i18n
const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const paymentStore = usePaymentStore()
const subscriptionStore = useSubscriptionStore()
const appStore = useAppStore()

const user = computed(() => authStore.user)
const activeSubscriptions = computed(() => subscriptionStore.activeSubscriptions)

function getDaysRemaining(expiresAt: string): number {
  const diff = new Date(expiresAt).getTime() - Date.now()
  return Math.max(0, Math.ceil(diff / (1000 * 60 * 60 * 24)))
}

interface SubscriptionUsageRow {
  period: 'daily' | 'weekly' | 'monthly'
  label: string
  used: number
  limit: number
  percentage: number
}

function subscriptionUsageRows(subscription: UserSubscription): SubscriptionUsageRow[] {
  const periods = [
    { period: 'daily' as const, used: subscription.daily_usage_usd, limit: subscription.group?.daily_limit_usd },
    { period: 'weekly' as const, used: subscription.weekly_usage_usd, limit: subscription.group?.weekly_limit_usd },
    { period: 'monthly' as const, used: subscription.monthly_usage_usd, limit: subscription.group?.monthly_limit_usd },
  ]

  return periods.flatMap(({ period, used, limit }) => {
    if (!limit || limit <= 0) return []
    const safeUsed = Number.isFinite(used) ? Math.max(0, used) : 0
    return [{
      period,
      label: t(`userSubscriptions.${period}`),
      used: safeUsed,
      limit,
      percentage: Math.min(100, (safeUsed / limit) * 100),
    }]
  })
}

function subscriptionUsageBarClass(percentage: number): string {
  if (percentage >= 90) return 'bg-red-500'
  if (percentage >= 70) return 'bg-amber-500'
  return 'bg-primary-500'
}

const loading = ref(true)
const submitting = ref(false)
const errorMessage = ref('')
const errorHintMessage = ref('')
const refreshingSummary = ref(false)
const amount = ref<number | null>(null)
const selectedMethod = ref('')
const submittingPlanId = ref<number | null>(null)
const previewImage = ref('')
type SubscriptionPaymentSource = 'recharge' | 'balance'
type SubscriptionPurchaseStage = 'override' | 'balance'

interface PendingSubscriptionPurchase {
  plan: SubscriptionPlan
  source: SubscriptionPaymentSource
  stage: SubscriptionPurchaseStage
}

const pendingSubscriptionPurchase = ref<PendingSubscriptionPurchase | null>(null)
const walletHistoryRef = ref<InstanceType<typeof WalletBalanceHistory> | null>(null)

const paymentPhase = ref<'select' | 'paying'>('select')

interface CreateOrderOptions {
  openid?: string
  wechatResumeToken?: string
  paymentType?: string
  isResume?: boolean
  mobileQrFallbackAttempted?: boolean
}

interface WeixinJSBridgeLike {
  invoke(
    action: string,
    payload: Record<string, unknown>,
    callback: (result: Record<string, unknown>) => void,
  ): void
}

function emptyPaymentState(): PaymentRecoverySnapshot {
  return {
    orderId: 0,
    amount: 0,
    qrCode: '',
    expiresAt: '',
    paymentType: '',
    payUrl: '',
    outTradeNo: '',
    clientSecret: '',
    intentId: '',
    currency: '',
    countryCode: '',
    paymentEnv: '',
    payAmount: 0,
    orderType: '',
    paymentMode: '',
    resumeToken: '',
    createdAt: 0,
  }
}

function getWeixinJSBridge(): WeixinJSBridgeLike | undefined {
  return (window as Window & { WeixinJSBridge?: WeixinJSBridgeLike }).WeixinJSBridge
}

function waitForWeixinJSBridge(timeoutMs = 4000): Promise<WeixinJSBridgeLike | null> {
  const existing = getWeixinJSBridge()
  if (existing) return Promise.resolve(existing)

  return new Promise((resolve) => {
    let settled = false
    const finish = (bridge: WeixinJSBridgeLike | null) => {
      if (settled) return
      settled = true
      document.removeEventListener('WeixinJSBridgeReady', handleReady)
      document.removeEventListener('onWeixinJSBridgeReady', handleReady)
      window.clearTimeout(timer)
      resolve(bridge)
    }
    const handleReady = () => finish(getWeixinJSBridge() ?? null)
    const timer = window.setTimeout(() => finish(getWeixinJSBridge() ?? null), timeoutMs)
    document.addEventListener('WeixinJSBridgeReady', handleReady, false)
    document.addEventListener('onWeixinJSBridgeReady', handleReady, false)
  })
}

async function invokeWechatJsapiPayment(payload: Record<string, unknown>): Promise<Record<string, unknown>> {
  const bridge = await waitForWeixinJSBridge()
  if (!bridge) {
    throw new Error('WECHAT_JSAPI_UNAVAILABLE')
  }
  return new Promise((resolve) => {
    bridge.invoke('getBrandWCPayRequest', payload, (result) => resolve(result || {}))
  })
}

const paymentState = ref<PaymentRecoverySnapshot>(emptyPaymentState())

function persistRecoverySnapshot(snapshot: PaymentRecoverySnapshot) {
  if (typeof window === 'undefined' || !snapshot.orderId) return
  writePaymentRecoverySnapshot(window.localStorage, snapshot, PAYMENT_RECOVERY_STORAGE_KEY)
}

function removeRecoverySnapshot() {
  if (typeof window === 'undefined') return
  clearPaymentRecoverySnapshot(window.localStorage, PAYMENT_RECOVERY_STORAGE_KEY)
}

function resetPayment() {
  paymentPhase.value = 'select'
  paymentState.value = emptyPaymentState()
  removeRecoverySnapshot()
}

async function redirectToPaymentResult(state: PaymentRecoverySnapshot): Promise<void> {
  const query: Record<string, string | undefined> = {}
  if (state.orderId > 0) {
    query.order_id = String(state.orderId)
  }
  if (state.outTradeNo) {
    query.out_trade_no = state.outTradeNo
  }
  if (state.resumeToken) {
    query.resume_token = state.resumeToken
  }
  await router.push({
    path: '/payment/result',
    query,
  })
}

function buildWechatOAuthAuthorizeUrl(
  authorizeUrl: string,
  context: { paymentType: string; orderType: OrderType; planId?: number; orderAmount: number },
): string {
  const normalizedUrl = authorizeUrl.trim()
  if (!normalizedUrl || typeof window === 'undefined') {
    return normalizedUrl
  }

  try {
    const targetUrl = new URL(normalizedUrl, window.location.origin)
    const redirectPath = targetUrl.searchParams.get('redirect') || '/wallet'
    const redirectUrl = new URL(redirectPath, window.location.origin)
    const paymentType = normalizeVisibleMethod(context.paymentType) || context.paymentType.trim() || 'wxpay'

    redirectUrl.searchParams.set('payment_type', paymentType)
    redirectUrl.searchParams.set('order_type', context.orderType)

    if (context.planId) {
      redirectUrl.searchParams.set('plan_id', String(context.planId))
    } else {
      redirectUrl.searchParams.delete('plan_id')
    }

    if (context.orderAmount > 0) {
      redirectUrl.searchParams.set('amount', String(context.orderAmount))
    } else {
      redirectUrl.searchParams.delete('amount')
    }

    targetUrl.searchParams.set('redirect', `${redirectUrl.pathname}${redirectUrl.search}`)
    return targetUrl.toString()
  } catch {
    return normalizedUrl
  }
}

function onPaymentDone() {
  const wasSubscription = paymentState.value.orderType === 'subscription'
  resetPayment()
  if (wasSubscription) {
    subscriptionStore.fetchActiveSubscriptions(true).catch(() => {})
  }
}

function onPaymentSuccess() {
  removeRecoverySnapshot()
  authStore.refreshUser()
  if (paymentState.value.orderType === 'subscription') {
    subscriptionStore.fetchActiveSubscriptions(true).catch(() => {})
  }
}

function onPaymentSettled() {
  removeRecoverySnapshot()
}

// All checkout data from single API call
const checkout = ref<CheckoutInfoResponse>({
  methods: {}, global_min: 0, global_max: 0,
  plans: [], balance_disabled: false, balance_recharge_multiplier: 10, subscription_usd_to_cny_rate: 0, recharge_fee_rate: 0, help_text: '', help_image_url: '', stripe_publishable_key: '',
})

const walletSectionIds: Record<string, string> = {
  recharge: 'wallet-recharge',
  subscription: 'wallet-subscription',
  history: 'wallet-history',
  redeem: 'wallet-redeem',
}

function scrollToWalletSection(tab: unknown) {
  if (typeof tab !== 'string') return
  const sectionId = walletSectionIds[tab]
  if (!sectionId || typeof document === 'undefined') return
  document.getElementById(sectionId)?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

async function refreshWalletSummary() {
  if (refreshingSummary.value) return
  refreshingSummary.value = true
  try {
    await Promise.all([
      authStore.refreshUser(),
      subscriptionStore.fetchActiveSubscriptions(true),
    ])
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('common.error')))
  } finally {
    refreshingSummary.value = false
  }
}

const visibleMethods = computed(() => getVisibleMethods(checkout.value.methods))
const enabledMethods = computed(() => Object.keys(visibleMethods.value))
const validAmount = computed(() => amount.value ?? 0)
const balanceRechargeMultiplier = computed(() => {
  const multiplier = checkout.value.balance_recharge_multiplier
  return Number.isFinite(multiplier) && multiplier > 0 ? multiplier : 10
})
// 订阅 CNY 换算汇率（1 USD = X CNY）。0 = 未配置，订阅保持 price 直付（与后端 opt-in 条件严格镜像）。
const subscriptionUsdToCnyRate = computed(() => {
  const rate = checkout.value.subscription_usd_to_cny_rate
  return Number.isFinite(rate) && rate > 0 ? rate : 0
})
const creditedAmount = computed(() => Math.round((validAmount.value * balanceRechargeMultiplier.value) * 100) / 100)
const loyaltyInfo = computed(() => checkout.value.loyalty)
const loyaltyDiscountPercent = computed(() => {
  const raw = loyaltyInfo.value?.discount_percent ?? 0
  if (!loyaltyInfo.value?.enabled || !Number.isFinite(raw) || raw <= 0) return 0
  return Math.min(100, Math.max(0, raw))
})

// Adaptive grid: center single card, 2-col for 2 plans, 3-col for 3+
const planGridClass = computed(() => {
  const n = checkout.value.plans.length
  if (n <= 2) return 'grid grid-cols-1 gap-5 sm:grid-cols-2'
  return 'grid grid-cols-1 gap-5 sm:grid-cols-2 2xl:grid-cols-3'
})

// Check if an amount fits a method's [min, max]. 0 = no limit.
function amountFitsMethod(amt: number, methodType: string): boolean {
  if (amt <= 0) return true
  const ml = visibleMethods.value[methodType]
  if (!ml) return false
  if (ml.single_min > 0 && amt < ml.single_min) return false
  if (ml.single_max > 0 && amt > ml.single_max) return false
  return true
}

// Visible methods decide the amount range shown to users.
const globalMinAmount = computed(() => {
  const limits = Object.values(visibleMethods.value)
  if (limits.length === 0) return 0
  if (limits.some(limit => limit.single_min <= 0)) return 0
  return Math.min(...limits.map(limit => limit.single_min))
})
const globalMaxAmount = computed(() => {
  const limits = Object.values(visibleMethods.value)
  if (limits.length === 0) return 0
  if (limits.some(limit => limit.single_max <= 0)) return 0
  return Math.max(...limits.map(limit => limit.single_max))
})

// Selected method's limits (for validation and error messages)
const selectedLimit = computed(() => visibleMethods.value[selectedMethod.value])
const selectedCurrency = computed(() => normalizePaymentCurrency(selectedLimit.value?.currency))
const localeCode = computed(() => {
  const raw = i18n.locale as unknown
  if (typeof raw === 'string') return raw
  if (raw && typeof raw === 'object' && 'value' in raw) {
    return String((raw as { value?: string }).value || '')
  }
  return undefined
})

function currencyFractionDigits(currency: string): number {
  try {
    return new Intl.NumberFormat(undefined, {
      style: 'currency',
      currency,
    }).resolvedOptions().maximumFractionDigits ?? 2
  } catch {
    return 2
  }
}

function roundPaymentAmount(value: number, currency: string): number {
  if (!Number.isFinite(value)) return 0
  const factor = 10 ** currencyFractionDigits(currency)
  return Math.round(value * factor) / factor
}

function ceilPaymentAmount(value: number, currency: string): number {
  if (!Number.isFinite(value)) return 0
  const factor = 10 ** currencyFractionDigits(currency)
  return Math.ceil(value * factor) / factor
}

function subscriptionPaymentAmountForCurrency(value: number, currency: string): number {
  const rate = subscriptionUsdToCnyRate.value
  if (rate <= 0 || currency !== DEFAULT_PAYMENT_CURRENCY) return roundPaymentAmount(value, currency)
  return roundPaymentAmount(value * rate, currency)
}

function formatSelectedPaymentAmount(value: number): string {
  return formatPaymentAmount(value, selectedCurrency.value, localeCode.value)
}

const feeRate = computed(() => checkout.value?.recharge_fee_rate ?? 0)

function balancePaymentBaseForCurrency(value: number, currency: string): number {
  if (value <= 0) return 0
  if (loyaltyDiscountPercent.value <= 0) return roundPaymentAmount(value, currency)
  return roundPaymentAmount(value * (100 - loyaltyDiscountPercent.value) / 100, currency)
}

function balanceFeeAmountForCurrency(value: number, currency: string): number {
  if (feeRate.value <= 0 || value <= 0) return 0
  return ceilPaymentAmount((value * feeRate.value) / 100, currency)
}

function balanceTotalAmountForCurrency(value: number, currency: string): number {
  const paymentBase = balancePaymentBaseForCurrency(value, currency)
  if (paymentBase <= 0) return 0
  return roundPaymentAmount(paymentBase + balanceFeeAmountForCurrency(paymentBase, currency), currency)
}

function balanceTotalAmountForMethod(value: number, methodType: string): number {
  const currency = normalizePaymentCurrency(visibleMethods.value[methodType]?.currency)
  return balanceTotalAmountForCurrency(value, currency)
}

const paymentBaseAmount = computed(() => balancePaymentBaseForCurrency(validAmount.value, selectedCurrency.value))
const loyaltyDiscountAmount = computed(() => roundPaymentAmount(Math.max(0, validAmount.value - paymentBaseAmount.value), selectedCurrency.value))
const hasLoyaltyDiscount = computed(() => loyaltyDiscountPercent.value > 0 && loyaltyDiscountAmount.value > 0)
const selectedBalanceTotalAmount = computed(() => balanceTotalAmountForCurrency(validAmount.value, selectedCurrency.value))

const methodOptions = computed<PaymentMethodOption[]>(() =>
  enabledMethods.value.map((type) => {
    const ml = visibleMethods.value[type]
    return {
      type,
      display_name: ml?.display_name,
      fee_rate: ml?.fee_rate ?? 0,
      available: ml?.available !== false && amountFitsMethod(balanceTotalAmountForMethod(validAmount.value, type), type),
    }
  })
)

const feeAmount = computed(() =>
  feeRate.value > 0 && paymentBaseAmount.value > 0
    ? balanceFeeAmountForCurrency(paymentBaseAmount.value, selectedCurrency.value)
    : 0
)
const totalAmount = computed(() =>
  paymentBaseAmount.value > 0
    ? roundPaymentAmount(paymentBaseAmount.value + feeAmount.value, selectedCurrency.value)
    : validAmount.value
)

const amountError = computed(() => {
  if (validAmount.value <= 0) return ''
  // No method can handle this amount
  if (!enabledMethods.value.some((m) => amountFitsMethod(balanceTotalAmountForMethod(validAmount.value, m), m))) {
    return t('payment.amountNoMethod')
  }
  // Selected method can't handle this amount (but others can)
  const ml = selectedLimit.value
  if (ml) {
    if (ml.single_min > 0 && selectedBalanceTotalAmount.value < ml.single_min) return t('payment.amountTooLow', { min: formatSelectedPaymentAmount(ml.single_min) })
    if (ml.single_max > 0 && selectedBalanceTotalAmount.value > ml.single_max) return t('payment.amountTooHigh', { max: formatSelectedPaymentAmount(ml.single_max) })
  }
  return ''
})

const canSubmit = computed(() =>
  validAmount.value > 0
    && amountFitsMethod(selectedBalanceTotalAmount.value, selectedMethod.value)
    && selectedLimit.value?.available !== false
)

const availableBalance = computed(() => Number(user.value?.balance ?? 0))
const balanceSubscriptionOperationKeys = new Map<string, string>()
const balanceSubscriptionOperationStoragePrefix = 'sub2api:balance-subscription-operation'

function balanceSubscriptionOperationStorageKey(planId: number): string {
  const actor = user.value?.id || user.value?.username || 'unknown'
  return `${balanceSubscriptionOperationStoragePrefix}:${actor}:${planId}`
}

function readStoredBalanceSubscriptionOperationKey(storageKey: string): string {
  try {
    const key = window.localStorage.getItem(storageKey) || ''
    return /^[\x21-\x7e]{1,128}$/.test(key) ? key : ''
  } catch {
    return ''
  }
}

function persistBalanceSubscriptionOperationKey(storageKey: string, key: string) {
  try {
    window.localStorage.setItem(storageKey, key)
  } catch {
    // The in-memory map still protects retries while this page remains open.
  }
}

function balanceSubscriptionOperationKey(planId: number): string {
  const storageKey = balanceSubscriptionOperationStorageKey(planId)
  const existing = balanceSubscriptionOperationKeys.get(storageKey)
  if (existing) return existing
  const stored = readStoredBalanceSubscriptionOperationKey(storageKey)
  if (stored) {
    balanceSubscriptionOperationKeys.set(storageKey, stored)
    return stored
  }
  const requestID = globalThis.crypto?.randomUUID?.() ?? `${Date.now()}-${Math.random().toString(36).slice(2)}`
  const key = `balance-subscription-${planId}-${requestID}`
  balanceSubscriptionOperationKeys.set(storageKey, key)
  persistBalanceSubscriptionOperationKey(storageKey, key)
  return key
}

function clearBalanceSubscriptionOperationKey(planId: number) {
  const storageKey = balanceSubscriptionOperationStorageKey(planId)
  balanceSubscriptionOperationKeys.delete(storageKey)
  try {
    window.localStorage.removeItem(storageKey)
  } catch {
    // No cleanup is needed when storage is unavailable.
  }
}

function subscriptionBeforeDiscountAmount(plan: SubscriptionPlan, currency: string): number {
  return subscriptionPaymentAmountForCurrency(plan.price, currency)
}

function subscriptionAfterDiscountAmount(plan: SubscriptionPlan, currency: string): number {
  return balancePaymentBaseForCurrency(subscriptionBeforeDiscountAmount(plan, currency), currency)
}

function subscriptionTotalAmountForPlan(plan: SubscriptionPlan): number {
  const discountedAmount = subscriptionAfterDiscountAmount(plan, selectedCurrency.value)
  if (feeRate.value <= 0 || discountedAmount <= 0) return discountedAmount
  return roundPaymentAmount(
    discountedAmount + balanceFeeAmountForCurrency(discountedAmount, selectedCurrency.value),
    selectedCurrency.value,
  )
}

function canRechargeSubscriptionPlan(plan: SubscriptionPlan): boolean {
  return enabledMethods.value.length > 0
    && amountFitsMethod(subscriptionTotalAmountForPlan(plan), selectedMethod.value)
    && selectedLimit.value?.available !== false
}

function formatSubscriptionRechargeAmount(plan: SubscriptionPlan): string {
  return formatSelectedPaymentAmount(subscriptionTotalAmountForPlan(plan))
}

function formatSubscriptionBeforeDiscountAmount(plan: SubscriptionPlan): string {
  return formatSelectedPaymentAmount(subscriptionBeforeDiscountAmount(plan, selectedCurrency.value))
}

function formatSubscriptionAfterDiscountAmount(plan: SubscriptionPlan): string {
  return formatSelectedPaymentAmount(subscriptionAfterDiscountAmount(plan, selectedCurrency.value))
}

const subscriptionLoyaltyDiscountLabel = computed(() => {
  if (loyaltyDiscountPercent.value <= 0) return t('wallet.subscriptionNoDiscount')
  const key = loyaltyInfo.value?.discount_scope === 'permanent'
    ? 'wallet.subscriptionPermanentDiscount'
    : 'wallet.subscriptionWeeklyDiscount'
  return t(key, {
    level: loyaltyInfo.value?.discount_level || '',
    discount: loyaltyDiscountPercent.value,
  })
})

function subscriptionBalancePrice(plan: SubscriptionPlan): number {
  const cnyPrice = subscriptionBeforeDiscountAmount(plan, DEFAULT_PAYMENT_CURRENCY)
  return Math.round(cnyPrice * balanceRechargeMultiplier.value * 100) / 100
}

function hasEnoughBalanceForPlan(plan: SubscriptionPlan): boolean {
  const balancePrice = subscriptionBalancePrice(plan)
  return balancePrice > 0 && availableBalance.value + 1e-9 >= balancePrice
}

// Auto-switch to first available method when current selection can't handle the amount
watch(() => [validAmount.value, selectedMethod.value] as const, ([amt, method]) => {
  if (amt <= 0 || amountFitsMethod(balanceTotalAmountForMethod(amt, method), method)) return
  const available = enabledMethods.value.find((m) => amountFitsMethod(balanceTotalAmountForMethod(amt, m), m))
  if (available) selectedMethod.value = available
})

// Payment button class: follows selected payment method color
const paymentButtonClass = computed(() => {
  const m = selectedMethod.value
  if (!m) return 'btn-primary'
  if (isBuiltInAlipayMethod(m)) return 'btn-alipay'
  if (isBuiltInWxpayMethod(m)) return 'btn-wxpay'
  if (m === 'stripe') return 'btn-stripe'
  if (m === 'airwallex') return 'btn-airwallex'
  return 'btn-primary'
})

async function handleSubmitRecharge() {
  if (!canSubmit.value || submitting.value) return
  await createOrder(validAmount.value, 'balance')
}

const activeSubscriptionToReplace = computed(() => activeSubscriptions.value.find((subscription) => {
  if (subscription.status !== 'active') return false
  if (!subscription.expires_at) return true
  return new Date(subscription.expires_at).getTime() > Date.now()
}) ?? null)

const subscriptionOverrideMessage = computed(() => {
  const subscription = activeSubscriptionToReplace.value
  if (!subscription) return ''
  const name = subscription.group?.name || t('payment.groupFallback', { id: subscription.group_id })
  if (!subscription.expires_at) {
    return t('wallet.subscriptionOverrideNoExpiryMessage', { name })
  }
  return t('wallet.subscriptionOverrideMessage', {
    name,
    days: getDaysRemaining(subscription.expires_at),
  })
})

const subscriptionPurchaseDialogTitle = computed(() => {
  return pendingSubscriptionPurchase.value?.stage === 'balance'
    ? t('wallet.subscriptionBalanceConfirmTitle')
    : t('wallet.subscriptionOverrideTitle')
})

const subscriptionPurchaseDialogMessage = computed(() => {
  return pendingSubscriptionPurchase.value?.stage === 'balance'
    ? t('wallet.subscriptionBalanceConfirmMessage')
    : ''
})

const subscriptionPurchaseDialogWarningMessage = computed(() => {
  if (!pendingSubscriptionPurchase.value || !activeSubscriptionToReplace.value) return ''
  return subscriptionOverrideMessage.value
})

const subscriptionPurchaseDialogConfirmText = computed(() => {
  return pendingSubscriptionPurchase.value?.stage === 'balance'
    ? t('wallet.subscriptionBalanceConfirmAction')
    : t('wallet.subscriptionOverrideConfirm')
})

const pendingSubscriptionBalancePrice = computed(() => {
  const plan = pendingSubscriptionPurchase.value?.plan
  return plan ? subscriptionBalancePrice(plan) : 0
})

const pendingSubscriptionBalanceAfterPayment = computed(() => {
  return Math.max(0, availableBalance.value - pendingSubscriptionBalancePrice.value)
})

async function subscribeToPlan(plan: SubscriptionPlan, source: SubscriptionPaymentSource) {
  if (submitting.value) return
  if (source === 'balance' && !hasEnoughBalanceForPlan(plan)) return
  if (source === 'recharge' && !canRechargeSubscriptionPlan(plan)) return

  if (source === 'balance') {
    pendingSubscriptionPurchase.value = { plan, source, stage: 'balance' }
    return
  }

  if (activeSubscriptionToReplace.value) {
    pendingSubscriptionPurchase.value = { plan, source, stage: 'override' }
    return
  }

  await executeSubscriptionPurchase(plan, source)
}

function cancelPendingSubscriptionPurchase() {
  pendingSubscriptionPurchase.value = null
}

async function confirmPendingSubscriptionPurchase() {
  const pending = pendingSubscriptionPurchase.value
  if (!pending) return

  pendingSubscriptionPurchase.value = null
  await executeSubscriptionPurchase(pending.plan, pending.source)
}

async function executeSubscriptionPurchase(plan: SubscriptionPlan, source: SubscriptionPaymentSource) {
  submittingPlanId.value = plan.id
  if (source === 'balance') {
    submitting.value = true
    errorMessage.value = ''
    try {
      await paymentAPI.purchaseSubscriptionWithBalance(plan.id, balanceSubscriptionOperationKey(plan.id))
      clearBalanceSubscriptionOperationKey(plan.id)
      await Promise.allSettled([
        authStore.refreshUser(),
        subscriptionStore.fetchActiveSubscriptions(true),
        walletHistoryRef.value?.refresh(),
      ])
      appStore.showInfo(t('wallet.subscriptionBalanceSuccess'))
    } catch (error) {
      appStore.showError(extractApiErrorMessage(error, t('common.error')))
    } finally {
      submitting.value = false
      submittingPlanId.value = null
    }
    return
  }
  await createOrder(plan.price, 'subscription', plan.id)
  submittingPlanId.value = null
}

async function createOrder(orderAmount: number, orderType: OrderType, planId?: number, options: CreateOrderOptions = {}) {
  submitting.value = true
  errorMessage.value = ''
  errorHintMessage.value = ''
  const requestType = normalizeVisibleMethod(options.paymentType || selectedMethod.value) || options.paymentType || selectedMethod.value
  try {
    const payload = buildCreateOrderPayload({
      amount: orderAmount,
      paymentType: requestType,
      orderType,
      planId,
      origin: typeof window !== 'undefined' ? window.location.origin : '',
      isMobile: isMobileDevice(),
      isWechatBrowser: typeof window !== 'undefined' && /MicroMessenger/i.test(window.navigator.userAgent),
      forceQRCode: !!(checkout.value.alipay_force_qrcode && normalizeVisibleMethod(requestType) === 'alipay'),
    })
    if (options.openid) {
      payload.openid = options.openid
    }
    if (options.wechatResumeToken) {
      payload.wechat_resume_token = options.wechatResumeToken
    }

    const result = await paymentStore.createOrder(payload) as CreateOrderResult & { resume_token?: string }
    const openWindow = (url: string) => {
      const win = window.open(url, 'paymentPopup', getPaymentPopupFeatures())
      if (!win || win.closed) {
        window.location.href = url
      }
    }
    const visibleMethod = normalizeVisibleMethod(requestType) || requestType
    // When user clicks the dedicated Stripe button, leave method blank so the
    // landing page renders Stripe's full Payment Element (card/link/alipay/wxpay).
    const stripeMethod = visibleMethod === 'stripe'
      ? ''
      : visibleMethod === 'wxpay' ? 'wechat_pay' : 'alipay'
    const stripeRouteUrl = result.client_secret && visibleMethod !== 'airwallex'
      ? router.resolve({
        path: '/payment/stripe',
        query: {
          order_id: String(result.order_id),
          client_secret: result.client_secret,
          method: stripeMethod || undefined,
          resume_token: result.resume_token || undefined,
        },
      }).href
      : ''
    const airwallexRouteUrl = result.client_secret && result.intent_id
      ? router.resolve({
        path: '/payment/airwallex',
        query: {
          order_id: String(result.order_id),
          out_trade_no: result.out_trade_no || undefined,
          resume_token: result.resume_token || undefined,
        },
      }).href
      : ''
    const decision = decidePaymentLaunch(result, {
      visibleMethod,
      orderType,
      isMobile: isMobileDevice(),
      isWechatBrowser: typeof window !== 'undefined' && /MicroMessenger/i.test(window.navigator.userAgent),
      forceQRCode: !!(checkout.value.alipay_force_qrcode && visibleMethod === 'alipay'),
      stripePopupUrl: stripeRouteUrl,
      stripeRouteUrl,
      airwallexRouteUrl,
    })

    if (decision.kind === 'wechat_oauth' && decision.oauth?.authorize_url) {
      window.location.href = buildWechatOAuthAuthorizeUrl(decision.oauth.authorize_url, {
        paymentType: visibleMethod,
        orderType,
        planId,
        orderAmount,
      })
      return
    }

    if (decision.kind === 'unhandled') {
      applyScenarioError({ reason: 'UNHANDLED_PAYMENT_SCENARIO' }, visibleMethod)
      return
    }

    paymentState.value = decision.paymentState
    paymentPhase.value = 'paying'
    persistRecoverySnapshot(decision.recovery)

    if (decision.kind === 'stripe_popup') {
      openWindow(decision.paymentState.payUrl)
      return
    }
    if (decision.kind === 'stripe_route') {
      window.location.href = decision.paymentState.payUrl
      return
    }
    if (decision.kind === 'airwallex_route') {
      window.location.href = decision.paymentState.payUrl
      return
    }
    if (decision.kind === 'wechat_jsapi' && decision.jsapi) {
      try {
        const jsapiResult = await invokeWechatJsapiPayment(decision.jsapi as Record<string, unknown>)
        const errMsg = String(jsapiResult.err_msg || '').toLowerCase()
        if (errMsg.includes('cancel')) {
          appStore.showInfo(t('payment.qr.cancelled'))
          resetPayment()
        } else if (errMsg && !errMsg.includes('ok')) {
          resetPayment()
          const fallbackApplied = await attemptMobileQrFallback(
            { reason: 'WECHAT_JSAPI_FAILED', message: errMsg },
            {
              orderAmount,
              orderType,
              planId,
              paymentType: visibleMethod,
              attempted: options.mobileQrFallbackAttempted === true,
            },
          )
          if (!fallbackApplied) {
            applyScenarioError({ reason: 'WECHAT_JSAPI_FAILED', message: errMsg }, visibleMethod)
          }
        } else {
          const resultState = { ...decision.paymentState }
          resetPayment()
          await redirectToPaymentResult(resultState)
        }
      } catch (err: unknown) {
        resetPayment()
        const fallbackApplied = await attemptMobileQrFallback(err, {
          orderAmount,
          orderType,
          planId,
          paymentType: visibleMethod,
          attempted: options.mobileQrFallbackAttempted === true,
        })
        if (!fallbackApplied) {
          throw err
        }
      }
      return
    }
    if (decision.kind === 'redirect_waiting' && decision.paymentState.payUrl) {
      if (isMobileDevice()) {
        window.location.href = decision.paymentState.payUrl
        return
      }
      openWindow(decision.paymentState.payUrl)
    }
  } catch (err: unknown) {
    const apiErr = err as Record<string, unknown>
    if (apiErr.reason === 'TOO_MANY_PENDING') {
      const metadata = apiErr.metadata as Record<string, unknown> | undefined
      errorMessage.value = t('payment.errors.tooManyPending', { max: metadata?.max || '' })
      errorHintMessage.value = ''
    } else if (apiErr.reason === 'CANCEL_RATE_LIMITED') {
      errorMessage.value = t('payment.errors.cancelRateLimited')
      errorHintMessage.value = ''
    } else if (await attemptMobileQrFallback(err, {
      orderAmount,
      orderType,
      planId,
      paymentType: requestType,
      attempted: options.mobileQrFallbackAttempted === true,
    })) {
      return
    } else {
      const handled = applyScenarioError(
        err,
        normalizeVisibleMethod(options.paymentType || selectedMethod.value) || selectedMethod.value,
      )
      if (!handled) {
        errorMessage.value = extractI18nErrorMessage(err, t, 'payment.errors', extractApiErrorMessage(err, t('payment.result.failed')))
        errorHintMessage.value = ''
      }
      if (handled) {
        return
      }
    }
    appStore.showError(buildPaymentErrorToastMessage(errorMessage.value, errorHintMessage.value))
  } finally {
    submitting.value = false
  }
}

interface MobileQrFallbackContext {
  orderAmount: number
  orderType: OrderType
  planId?: number
  paymentType: string
  attempted: boolean
}

function shouldFallbackToDesktopQr(err: unknown, paymentMethod: string, attempted: boolean): boolean {
  if (attempted || !isMobileDevice()) {
    return false
  }

  const normalizedMethod = normalizeVisibleMethod(paymentMethod) || paymentMethod
  const reason = typeof err === 'object' && err && 'reason' in err && typeof err.reason === 'string'
    ? err.reason
    : ''
  const message = err instanceof Error
    ? err.message
    : (typeof err === 'object' && err && 'message' in err && typeof err.message === 'string'
      ? err.message
      : '')
  const normalizedMessage = message.toLowerCase()

  if (normalizedMethod === 'wxpay') {
    return reason === 'WECHAT_H5_NOT_AUTHORIZED'
      || reason === 'WECHAT_PAYMENT_MP_NOT_CONFIGURED'
      || reason === 'WECHAT_JSAPI_FAILED'
      || reason === 'PAYMENT_GATEWAY_ERROR'
      || reason === 'UNHANDLED_PAYMENT_SCENARIO'
      || normalizedMessage.includes('weixinjsbridge is unavailable')
      || normalizedMessage.includes('wechat_jsapi_unavailable')
  }

  if (normalizedMethod === 'alipay') {
    return reason === 'PAYMENT_GATEWAY_ERROR' || reason === 'UNHANDLED_PAYMENT_SCENARIO'
  }

  return false
}

async function attemptMobileQrFallback(err: unknown, context: MobileQrFallbackContext): Promise<boolean> {
  if (!shouldFallbackToDesktopQr(err, context.paymentType, context.attempted)) {
    return false
  }

  try {
    const visibleMethod = normalizeVisibleMethod(context.paymentType) || context.paymentType
    const payload = buildCreateOrderPayload({
      amount: context.orderAmount,
      paymentType: visibleMethod,
      orderType: context.orderType,
      planId: context.planId,
      origin: typeof window !== 'undefined' ? window.location.origin : '',
      isMobile: false,
      isWechatBrowser: false,
    })
    const result = await paymentStore.createOrder(payload) as CreateOrderResult & { resume_token?: string }
    const stripeMethod = visibleMethod === 'wxpay' ? 'wechat_pay' : 'alipay'
    const stripeRouteUrl = result.client_secret
      ? router.resolve({
        path: '/payment/stripe',
        query: {
          order_id: String(result.order_id),
          client_secret: result.client_secret,
          method: stripeMethod,
          resume_token: result.resume_token || undefined,
        },
      }).href
      : ''
    const decision = decidePaymentLaunch(result, {
      visibleMethod,
      orderType: context.orderType,
      isMobile: false,
      isWechatBrowser: false,
      stripePopupUrl: stripeRouteUrl,
      stripeRouteUrl,
    })

    if (decision.kind !== 'qr_waiting' || !decision.paymentState.qrCode) {
      return false
    }

    errorMessage.value = ''
    errorHintMessage.value = ''
    paymentState.value = decision.paymentState
    paymentPhase.value = 'paying'
    persistRecoverySnapshot(decision.recovery)
    appStore.showWarning(t('payment.errors.mobilePaymentFallbackToQr'))
    return true
  } catch {
    return false
  }
}

function applyScenarioError(err: unknown, paymentMethod: string): boolean {
  const descriptor = describePaymentScenarioError(err, {
    paymentMethod,
    isMobile: isMobileDevice(),
    isWechatBrowser: typeof window !== 'undefined' && /MicroMessenger/i.test(window.navigator.userAgent),
  })
  if (!descriptor) {
    errorMessage.value = ''
    errorHintMessage.value = ''
    return false
  }
  errorMessage.value = t(descriptor.messageKey)
  errorHintMessage.value = descriptor.hintKey ? t(descriptor.hintKey) : ''
  appStore.showError(buildPaymentErrorToastMessage(errorMessage.value, errorHintMessage.value))
  return true
}

async function resumeWechatPaymentFromQuery() {
  const resume = parseWechatResumeRoute(route.query, checkout.value.plans, validAmount.value)
  if (!resume) {
    return
  }

  selectedMethod.value = resume.paymentType
  if (resume.orderType === 'balance' && resume.orderAmount > 0) {
    amount.value = resume.orderAmount
  }
  await router.replace({ path: route.path, query: stripWechatResumeQuery(route.query) })

  if (resume.wechatResumeToken) {
    await createOrder(0, resume.orderType, resume.planId, {
      wechatResumeToken: resume.wechatResumeToken,
      paymentType: resume.paymentType,
      isResume: true,
    })
    return
  }

  if (resume.orderAmount > 0 && resume.openid) {
    await createOrder(resume.orderAmount, resume.orderType, resume.planId, {
      openid: resume.openid,
      paymentType: resume.paymentType,
      isResume: true,
    })
  }
}

onMounted(async () => {
  try {
    const res = await paymentAPI.getCheckoutInfo()
    checkout.value = res.data
    if (enabledMethods.value.length) {
      const order: readonly string[] = METHOD_ORDER
      const sorted = [...enabledMethods.value].sort((a, b) => {
        const ai = order.indexOf(a)
        const bi = order.indexOf(b)
        return (ai === -1 ? 999 : ai) - (bi === -1 ? 999 : bi)
      })
      selectedMethod.value = sorted[0]
    }
    if (typeof window !== 'undefined') {
      if (hasWechatResumeQuery(route.query)) {
        removeRecoverySnapshot()
      }
      const routeResumeToken = typeof route.query.resume_token === 'string'
        ? route.query.resume_token
        : typeof route.query.wechat_resume_token === 'string'
          ? route.query.wechat_resume_token
          : undefined
      const restored = readPaymentRecoverySnapshot(
        window.localStorage.getItem(PAYMENT_RECOVERY_STORAGE_KEY),
        { resumeToken: routeResumeToken },
      )
      if (restored) {
        paymentState.value = restored
        paymentPhase.value = 'paying'
        const restoredMethod = normalizeVisibleMethod(restored.paymentType)
          || (visibleMethods.value[restored.paymentType] ? restored.paymentType : '')
        if (restoredMethod) {
          selectedMethod.value = restoredMethod
        }
      } else {
        removeRecoverySnapshot()
      }
    }
    await resumeWechatPaymentFromQuery()
  } catch (err: unknown) { appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error'))) }
  finally { loading.value = false }
  // Fetch active subscriptions (uses cache, non-blocking)
  subscriptionStore.fetchActiveSubscriptions().catch(() => {})
  await nextTick()
  if (paymentPhase.value === 'select') scrollToWalletSection(route.query.tab)
})

watch(() => route.query.tab, async (tab) => {
  if (loading.value || paymentPhase.value !== 'select') return
  await nextTick()
  scrollToWalletSection(tab)
})
</script>
