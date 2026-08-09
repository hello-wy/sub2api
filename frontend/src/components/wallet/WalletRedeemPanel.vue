<template>
  <div :class="compact ? '' : 'grid gap-5 lg:grid-cols-[minmax(0,1.35fr)_minmax(260px,0.65fr)]'">
    <section class="rounded-lg border border-gray-200 bg-white p-6 dark:border-dark-600 dark:bg-dark-800">
      <div class="mb-6 flex items-start gap-3">
        <span class="flex h-11 w-11 shrink-0 items-center justify-center rounded-lg bg-primary-50 text-primary-600 dark:bg-primary-950/40 dark:text-primary-400">
          <Icon name="gift" size="lg" />
        </span>
        <div>
          <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('wallet.redeemTitle') }}</h2>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('wallet.redeemHint') }}</p>
        </div>
      </div>

      <form class="space-y-4" @submit.prevent="handleRedeem()">
        <div>
          <label for="wallet-redeem-code" class="input-label">{{ t('redeem.redeemCodeLabel') }}</label>
          <input
            id="wallet-redeem-code"
            v-model="code"
            class="input mt-1 w-full py-3 font-mono text-base uppercase"
            :placeholder="t('redeem.redeemCodePlaceholder')"
            :disabled="submitting"
            autocomplete="off"
          />
          <p class="input-hint">{{ t('redeem.redeemCodeHint') }}</p>
        </div>
        <button type="submit" class="btn btn-primary w-full py-3" :disabled="!code.trim() || submitting">
          <span v-if="submitting" class="mr-2 h-4 w-4 animate-spin rounded-full border-2 border-white border-t-transparent"></span>
          <Icon v-else name="checkCircle" size="md" class="mr-2" />
          {{ submitting ? t('redeem.redeeming') : t('redeem.redeemButton') }}
        </button>
      </form>

      <div v-if="result" class="mt-5 rounded-lg border border-emerald-200 bg-emerald-50 p-4 dark:border-emerald-800 dark:bg-emerald-950/30">
        <p class="text-sm font-semibold text-emerald-800 dark:text-emerald-300">{{ t('redeem.redeemSuccess') }}</p>
        <p class="mt-1 text-sm text-emerald-700 dark:text-emerald-400">{{ result.message }}</p>
      </div>
      <div v-if="errorMessage" class="mt-5 rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:border-red-800 dark:bg-red-950/30 dark:text-red-300">
        {{ errorMessage }}
      </div>
    </section>

    <aside v-if="!compact" class="rounded-lg border border-gray-200 bg-white p-5 dark:border-dark-600 dark:bg-dark-800">
      <p class="text-xs font-medium uppercase text-gray-400 dark:text-gray-500">{{ t('redeem.currentBalance') }}</p>
      <p class="mt-2 text-3xl font-bold text-gray-950 dark:text-white">${{ user?.balance?.toFixed(2) || '0.00' }}</p>
      <div class="my-5 h-px bg-gray-100 dark:bg-dark-700"></div>
      <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('redeem.aboutCodes') }}</h3>
      <ul class="mt-3 space-y-3 text-sm text-gray-500 dark:text-gray-400">
        <li class="flex gap-2"><Icon name="check" size="sm" class="mt-0.5 shrink-0 text-primary-500" />{{ t('redeem.codeRule1') }}</li>
        <li class="flex gap-2"><Icon name="check" size="sm" class="mt-0.5 shrink-0 text-primary-500" />{{ t('redeem.codeRule2') }}</li>
        <li class="flex gap-2"><Icon name="check" size="sm" class="mt-0.5 shrink-0 text-primary-500" />{{ t('redeem.codeRule4') }}</li>
      </ul>
    </aside>

    <ConfirmDialog
      :show="showSubscriptionOverwriteDialog"
      :title="t('redeem.subscriptionOverwriteTitle')"
      :message="t('redeem.subscriptionOverwriteMessage', { expiresAt: subscriptionOverwriteExpiresAt })"
      :warning-message="t('redeem.subscriptionOverwriteWarning')"
      :confirm-text="t('redeem.subscriptionOverwriteConfirm')"
      danger
      @confirm="confirmSubscriptionOverwrite"
      @cancel="cancelSubscriptionOverwrite"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { redeemAPI } from '@/api'
import type { SubscriptionOverwriteConfirmation } from '@/api/redeem'
import { useAuthStore } from '@/stores/auth'
import { useSubscriptionStore } from '@/stores/subscriptions'
import { useAppStore } from '@/stores/app'
import { extractApiErrorCode, extractApiErrorMetadata } from '@/utils/apiError'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()
withDefaults(defineProps<{ compact?: boolean }>(), { compact: false })
const authStore = useAuthStore()
const appStore = useAppStore()
const subscriptionStore = useSubscriptionStore()
const user = computed(() => authStore.user)
const code = ref('')
const submitting = ref(false)
const errorMessage = ref('')
const result = ref<{ message: string; type: string } | null>(null)
const showSubscriptionOverwriteDialog = ref(false)
const subscriptionOverwriteExpiresAt = ref('')
const subscriptionOverwriteConfirmation = ref<SubscriptionOverwriteConfirmation | null>(null)

async function handleRedeem(confirmation?: SubscriptionOverwriteConfirmation) {
  if (!code.value.trim()) return
  submitting.value = true
  result.value = null
  errorMessage.value = ''
  try {
    const response = await redeemAPI.redeem(code.value.trim(), confirmation)
    subscriptionOverwriteConfirmation.value = null
    result.value = response
    code.value = ''
    try {
      await authStore.refreshUser()
    } catch (error) {
      console.error('Failed to refresh user after redeem:', error)
    }
    if (response.type === 'subscription') {
      try {
        await subscriptionStore.fetchActiveSubscriptions(true)
      } catch (error) {
        console.error('Failed to refresh subscriptions after redeem:', error)
        appStore.showWarning(t('redeem.subscriptionRefreshFailed'))
      }
    }
    appStore.showSuccess(t('redeem.codeRedeemSuccess'))
  } catch (error: any) {
    if (extractApiErrorCode(error) === 'SUBSCRIPTION_OVERWRITE_CONFIRMATION_REQUIRED') {
      const metadata = extractApiErrorMetadata(error)
      const nextConfirmation = parseSubscriptionOverwriteConfirmation(metadata)
      if (nextConfirmation) {
        subscriptionOverwriteConfirmation.value = nextConfirmation
        subscriptionOverwriteExpiresAt.value = formatSubscriptionExpiration(nextConfirmation.expiresAt)
        showSubscriptionOverwriteDialog.value = true
        return
      }
    }
    errorMessage.value = error.response?.data?.detail || t('redeem.failedToRedeem')
    appStore.showError(t('redeem.redeemFailed'))
  } finally {
    submitting.value = false
  }
}

function confirmSubscriptionOverwrite() {
  const confirmation = subscriptionOverwriteConfirmation.value
  if (!confirmation) return
  showSubscriptionOverwriteDialog.value = false
  void handleRedeem(confirmation)
}

function cancelSubscriptionOverwrite() {
  showSubscriptionOverwriteDialog.value = false
  subscriptionOverwriteConfirmation.value = null
}

function parseSubscriptionOverwriteConfirmation(metadata?: Record<string, unknown>): SubscriptionOverwriteConfirmation | null {
  const subscriptionId = Number(metadata?.subscription_id)
  const termVersion = Number(metadata?.term_version)
  const expiresAt = metadata?.expires_at
  if (!Number.isSafeInteger(subscriptionId) || subscriptionId <= 0) return null
  if (!Number.isSafeInteger(termVersion) || termVersion <= 0) return null
  if (typeof expiresAt !== 'string' || Number.isNaN(new Date(expiresAt).getTime())) return null
  return { subscriptionId, termVersion, expiresAt }
}

function formatSubscriptionExpiration(value: unknown): string {
  if (typeof value !== 'string') return t('common.unknown')
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat(undefined, {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).format(date)
}
</script>
