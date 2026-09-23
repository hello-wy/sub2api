<!-- Adapted from wangyunjeff/sub2api-state-kit (LGPL-3.0); see NOTICE.state-kit.md. -->
<template>
  <section class="space-y-3 rounded-lg border border-gray-200 p-4 dark:border-dark-600" data-testid="codex-account-ticket-settings">
    <div class="flex items-start justify-between gap-4">
      <div>
        <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.accounts.stateTicket.title') }}</h3>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accounts.stateTicket.description') }}</p>
      </div>
      <Toggle v-model="enabled" :disabled="!status || busy || disabled" :aria-label="t('admin.accounts.stateTicket.enable')" data-testid="codex-account-ticket-enabled" />
    </div>
    <p v-if="loading" class="text-xs text-gray-500">{{ t('common.loading') }}</p>
    <template v-if="status">
      <p class="rounded-lg bg-amber-50 p-3 text-xs text-amber-800 dark:bg-amber-900/20 dark:text-amber-200" data-testid="codex-account-ticket-experiment">{{ t('admin.accounts.stateTicket.experiment') }}</p>
      <p class="text-xs text-gray-500 dark:text-gray-400" data-testid="codex-account-ticket-quarantine-hint">{{ t('admin.accounts.stateTicket.quarantineHint') }}</p>
      <div>
        <label :for="`codex-account-ticket-plan-${accountId}`" class="input-label">{{ t('admin.accounts.stateTicket.plan') }}</label>
        <select :id="`codex-account-ticket-plan-${accountId}`" v-model="ticketPlan" class="input w-full text-sm" :disabled="busy || disabled" data-testid="codex-account-ticket-plan">
          <option value="pro">{{ t('admin.accounts.stateTicket.planPro') }}</option>
          <option value="team">{{ t('admin.accounts.stateTicket.planTeam') }}</option>
        </select>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accounts.stateTicket.planHint') }}</p>
      </div>
      <p v-if="!status.global_enabled" class="rounded bg-amber-50 p-2 text-xs text-amber-800 dark:bg-amber-900/20 dark:text-amber-200" data-testid="codex-account-ticket-global-off">
        {{ t('admin.accounts.stateTicket.globalOff') }}
        <a href="/admin/settings?tab=gateway" target="_blank" rel="noopener noreferrer" class="font-medium underline">{{ t('admin.accounts.stateTicket.gatewaySettings') }}</a>
      </p>
      <div class="text-xs text-gray-500 dark:text-gray-400" data-testid="codex-account-ticket-global-pool">
        <p v-if="status.proxy_configured">{{ t('admin.accounts.stateTicket.globalPoolConfigured', { address: status.proxy_display }) }}</p>
        <p v-else class="text-amber-700 dark:text-amber-300">{{ t('admin.accounts.stateTicket.globalPoolMissing') }}</p>
        <p>{{ t('admin.accounts.stateTicket.globalPoolHint') }} <a href="/admin/settings?tab=gateway" target="_blank" rel="noopener noreferrer" class="font-medium underline">{{ t('admin.accounts.stateTicket.gatewaySettings') }}</a></p>
      </div>
      <label class="block">
        <span class="input-label">{{ t('admin.accounts.stateTicket.modelLabel') }}</span>
        <select v-model="model" class="input text-sm" :disabled="busy || disabled" data-testid="codex-account-ticket-model">
          <option value="gpt-6-astra">gpt-6-astra</option>
          <option value="gpt-5.6-sol">gpt-5.6-sol</option>
        </select>
      </label>
      <p v-if="!status.fixed_proxy_configured" class="text-xs text-amber-700 dark:text-amber-300">{{ t('admin.accounts.stateTicket.fixedProxyMissing') }}</p>
      <div class="flex flex-wrap items-center gap-2 text-sm" aria-live="polite" data-testid="codex-account-ticket-status">
        <span :data-testid="refreshingUsable ? 'codex-account-ticket-refreshing-usable' : undefined" :class="hasUsableTicket ? 'text-emerald-600 dark:text-emerald-400' : status.state === 'error' ? 'text-amber-700 dark:text-amber-300' : 'text-gray-600 dark:text-gray-300'">{{ stateLabel }}</span>
        <span v-if="status.state === 'harvesting' && status.attempts" class="text-xs text-gray-500">{{ t('admin.accounts.stateTicket.attempts', { count: status.attempts }) }}</span>
      </div>
      <div v-if="savedTicket" class="space-y-1 rounded bg-emerald-50 p-2 text-xs text-emerald-800 dark:bg-emerald-900/20 dark:text-emerald-200" data-testid="codex-account-ticket-saved-ticket">
        <p class="font-medium">{{ t('admin.accounts.stateTicket.savedTicket') }}</p>
        <p v-if="status.verified_at" data-testid="ticket-business-verification">{{ t('admin.accounts.stateTicket.verifiedAt', { time: formatLocalDate(status.verified_at) }) }} · {{ t('admin.accounts.stateTicket.verifiedModel', { model: status.verified_model || status.model }) }}</p>
        <p>{{ t('admin.accounts.stateTicket.verificationHint') }}</p>
        <p v-if="capturedAt || expiresAt" class="flex flex-wrap gap-x-3 gap-y-1">
          <span v-if="capturedAt">{{ t('admin.accounts.stateTicket.capturedAt', { time: capturedAt }) }}</span>
          <span v-if="expiresAt">{{ t('admin.accounts.stateTicket.expiresAt', { time: expiresAt }) }}</span>
        </p>
        <p data-testid="codex-account-ticket-saved-ticket-hint">{{ t('admin.accounts.stateTicket.savedTicketHint') }}</p>
      </div>
      <p v-if="usableAfterRefreshFailure" class="text-xs text-amber-700 dark:text-amber-300" data-testid="codex-account-ticket-usable-error">
        {{ t('admin.accounts.stateTicket.refreshFailedUsable') }}
      </p>
      <p v-if="retryAfter" class="text-xs text-gray-600 dark:text-gray-300" data-testid="codex-account-ticket-retry-after">
        {{ t('admin.accounts.stateTicket.retryAfter', { time: retryAfter }) }}
      </p>
      <div class="space-y-1 rounded bg-gray-50 p-2 text-xs dark:bg-dark-700" aria-live="polite" data-testid="codex-account-ticket-watchdog">
        <p class="flex flex-wrap items-center gap-2">
          <span class="font-medium text-gray-700 dark:text-gray-200">{{ t('admin.accounts.stateTicket.watchdog') }}</span>
          <span :class="status.watchdog.enabled ? 'text-emerald-600 dark:text-emerald-400' : 'text-gray-500 dark:text-gray-400'" data-testid="codex-account-ticket-watchdog-status">{{ status.watchdog.enabled ? t('admin.accounts.stateTicket.watchdogEnabled') : t('admin.accounts.stateTicket.watchdogDisabled') }}</span>
        </p>
        <p class="text-gray-500 dark:text-gray-400">{{ t('admin.accounts.stateTicket.watchdogHint') }}</p>
        <p v-if="status.watchdog.trigger_count > 0" class="text-gray-600 dark:text-gray-300" data-testid="codex-account-ticket-watchdog-event">
          {{ t('admin.accounts.stateTicket.watchdogTriggerCount', { count: status.watchdog.trigger_count }) }}
          <span v-if="watchdogLastReason"> · {{ t('admin.accounts.stateTicket.watchdogLastReason', { reason: watchdogLastReason }) }}</span>
          <span v-if="watchdogLastTriggeredAt"> · {{ watchdogLastTriggeredAt }}</span>
        </p>
      </div>
      <p v-if="status.last_error" class="break-words text-xs text-amber-700 dark:text-amber-300" data-testid="codex-account-ticket-error">{{ status.last_error }}</p>
      <p v-if="proxyChanged" class="text-xs text-amber-700 dark:text-amber-300">{{ t('admin.accounts.stateTicket.fixedProxyUnsaved') }}</p>
      <p v-else-if="dirty" class="text-xs text-gray-500">{{ t('admin.accounts.stateTicket.unsaved') }}</p>
      <div class="flex flex-wrap gap-2">
        <button type="button" class="btn btn-primary btn-sm" :disabled="!canQuickStart" data-testid="codex-account-ticket-quick-start" @click="quickStart">
          {{ t('admin.accounts.stateTicket.quickStart') }}
        </button>
        <button type="button" class="btn btn-primary btn-sm" :disabled="busy || disabled || !dirty || proxyChanged || (enabled && (!status.proxy_configured || !status.fixed_proxy_configured))" data-testid="codex-account-ticket-save" @click="save">
          {{ t('admin.accounts.stateTicket.save') }}
        </button>
        <button type="button" class="btn btn-secondary btn-sm" :disabled="!canHarvest" data-testid="codex-account-ticket-harvest" @click="harvest">
          {{ status.state === 'ready' ? t('admin.accounts.stateTicket.reacquire') : t('admin.accounts.stateTicket.acquire') }}
        </button>
      </div>
      <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accounts.stateTicket.failureHint') }}</p>
    </template>
    <p v-if="error" role="alert" class="text-xs text-red-600 dark:text-red-400">{{ error }}</p>
    <p v-if="saved" role="status" class="text-xs text-emerald-600 dark:text-emerald-400">{{ t('admin.accounts.stateTicket.saved') }}</p>
    <button v-if="!status && !loading" type="button" class="btn btn-secondary btn-sm" @click="load(true)">{{ t('admin.accounts.stateTicket.retry') }}</button>
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Toggle from '@/components/common/Toggle.vue'
import { extractApiErrorMessage } from '@/utils/apiError'
import { ticketVerificationBlocked, ticketVerificationStateKey } from '@/utils/codexTicketStatus'
import { getCodexAccountTicket, saveCodexAccountTicket, harvestCodexAccountTicket, type CodexAccountTicketStatus, type CodexTicketPlan } from '@/api/admin/codexTickets'

const props = defineProps<{ accountId: number; visible: boolean; proxyChanged?: boolean; disabled?: boolean }>()
const emit = defineEmits<{ 'busy-change': [busy: boolean]; 'dirty-change': [dirty: boolean] }>()
const { t, locale } = useI18n()
const status = ref<CodexAccountTicketStatus | null>(null)
const enabled = ref(false)
const ticketPlan = ref<CodexTicketPlan>('pro')
const model = ref('gpt-6-astra')
const loading = ref(false)
const busy = ref(false)
const error = ref('')
const saved = ref(false)
let generation = 0
let revision = 0
let visibilityRevision = 0
let controller: AbortController | undefined
let timer: ReturnType<typeof setTimeout> | undefined
const dirty = computed(() => !!status.value && (enabled.value !== status.value.enabled || ticketPlan.value !== status.value.ticket_plan || model.value !== status.value.model))
const canHarvest = computed(() => !busy.value && !props.disabled && !dirty.value && !props.proxyChanged && !status.value?.authentication_blocked && status.value?.global_enabled && status.value.enabled && status.value.proxy_configured && status.value.fixed_proxy_configured && status.value.state !== 'harvesting')
const canQuickStart = computed(() => props.visible && !busy.value && !props.disabled && !props.proxyChanged && !status.value?.authentication_blocked && status.value?.global_enabled && status.value.proxy_configured && status.value.fixed_proxy_configured && status.value.state !== 'harvesting')
const hasUsableTicket = computed(() => status.value?.ticket_usable === true && !ticketVerificationBlocked(status.value))
const refreshingUsable = computed(() => status.value?.state === 'harvesting' && hasUsableTicket.value)
const savedTicket = computed(() => hasUsableTicket.value && !!(capturedAt.value || expiresAt.value))
const usableAfterRefreshFailure = computed(() => hasUsableTicket.value && !!status.value?.last_error)
const remainingTime = computed(() => formatRemaining(status.value?.remaining_seconds ?? 0))
const capturedAt = computed(() => formatLocalDate(status.value?.captured_at))
const expiresAt = computed(() => formatLocalDate(status.value?.expires_at))
const retryAfter = computed(() => formatLocalDate(status.value?.retry_after))
const stateLabel = computed(() => {
  if (!status.value) return ''
  const verificationState = ticketVerificationStateKey(status.value)
  if (verificationState) return t(`admin.accounts.stateTicket.${verificationState}`)
  if (refreshingUsable.value) return t('admin.accounts.stateTicket.refreshing', { time: remainingTime.value })
  if (status.value.state === 'ready') {
    return t('admin.accounts.stateTicket.ready', { time: remainingTime.value })
  }
  return t(`admin.accounts.stateTicket.states.${status.value.state}`)
})
const watchdogLastReason = computed(() => {
  const reason = status.value?.watchdog.last_reason
  if (reason === 'model_mismatch') return t('admin.accounts.stateTicket.watchdogModelMismatch')
  if (reason === 'state_312') return t('admin.accounts.stateTicket.watchdogState312')
  return ''
})
const watchdogLastTriggeredAt = computed(() => {
  return formatLocalDate(status.value?.watchdog.last_triggered_at)
})

function formatRemaining(seconds: number) {
  const total = Math.max(0, Math.floor(seconds))
  return `${Math.floor(total / 60)}m ${String(total % 60).padStart(2, '0')}s`
}

function formatLocalDate(value?: string) {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  return date.toLocaleString(locale.value, { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false })
}
watch(busy, value => emit('busy-change', value), { flush: 'sync' })
watch(dirty, value => emit('dirty-change', value), { flush: 'sync' })
watch([enabled, ticketPlan, model], () => { saved.value = false }, { flush: 'sync' })

async function load(initial = false) {
  const currentGeneration = generation
  const currentRevision = revision
  const current = new AbortController()
  controller?.abort()
  controller = current
  if (initial) loading.value = true
  try {
    const next = await getCodexAccountTicket(props.accountId, current.signal)
    if (current.signal.aborted || generation !== currentGeneration || revision !== currentRevision || !props.visible) return
    // Polling updates status only; it must never overwrite an in-progress edit.
    const synchronizeDraft = initial || !dirty.value
    status.value = next
    if (synchronizeDraft) {
      enabled.value = next.enabled
      ticketPlan.value = next.ticket_plan
      model.value = next.model
    }
    error.value = ''
  } catch {
    if (!current.signal.aborted && generation === currentGeneration && revision === currentRevision) error.value = t('admin.accounts.stateTicket.loadFailed')
  } finally {
    if (generation === currentGeneration && controller === current) loading.value = false
  }
}

function schedulePoll() {
  timer = setTimeout(async () => {
    const currentGeneration = generation
    const currentVisibilityRevision = visibilityRevision
    if (!props.visible) return
    if (!busy.value) await load()
    if (generation === currentGeneration && visibilityRevision === currentVisibilityRevision && props.visible) schedulePoll()
  }, 3000)
}

async function save() {
  if (!status.value || busy.value || props.disabled || props.proxyChanged || !dirty.value || (enabled.value && (!status.value.proxy_configured || !status.value.fixed_proxy_configured))) return
  busy.value = true
  saved.value = false
  error.value = ''
  revision++
  const currentGeneration = generation
  try {
    const next = await saveCodexAccountTicket(props.accountId, {
      enabled: enabled.value,
      ticket_plan: ticketPlan.value,
      model: model.value
    })
    if (generation !== currentGeneration) return
    status.value = next
    enabled.value = next.enabled
    ticketPlan.value = next.ticket_plan
    model.value = next.model
    // Keep success feedback after the draft watchers have cleared the old message.
    saved.value = true
  } catch (cause) {
    if (generation === currentGeneration) error.value = extractApiErrorMessage(cause instanceof Error ? null : cause, t('admin.accounts.stateTicket.saveFailed'))
  } finally {
    if (generation === currentGeneration) busy.value = false
  }
}

async function quickStart() {
  if (!canQuickStart.value) return
  const accountId = props.accountId
  const currentGeneration = generation
  revision++
  busy.value = true
  saved.value = false
  error.value = ''
  let configured = false
  try {
    let next = await saveCodexAccountTicket(accountId, { enabled: true, ticket_plan: ticketPlan.value, model: model.value })
    if (generation !== currentGeneration) return
    configured = true
    status.value = next
    enabled.value = next.enabled
    ticketPlan.value = next.ticket_plan
    model.value = next.model
    saved.value = true
    // Saving a changed configuration can already start acquisition. Reuse that
    // job or a valid ticket; never revoke one merely because this button repeats.
    if (next.state !== 'ready' && next.state !== 'harvesting' && (!next.retry_after || Date.parse(next.retry_after) <= Date.now())) {
      next = await harvestCodexAccountTicket(accountId)
      if (generation === currentGeneration) status.value = next
    }
  } catch (cause) {
    if (generation === currentGeneration) {
      const fallback = t(configured ? 'admin.accounts.stateTicket.harvestFailed' : 'admin.accounts.stateTicket.saveFailed')
      error.value = `${configured ? t('admin.accounts.stateTicket.quickStartSaved') + ' ' : ''}${extractApiErrorMessage(cause instanceof Error ? null : cause, fallback)}`
    }
  } finally {
    if (generation === currentGeneration) busy.value = false
  }
}

async function harvest() {
  if (!canHarvest.value) return
  busy.value = true
  saved.value = false
  error.value = ''
  revision++
  const currentGeneration = generation
  try {
    const next = await harvestCodexAccountTicket(props.accountId)
    if (generation === currentGeneration) status.value = next
  } catch (cause) {
    if (generation === currentGeneration) error.value = extractApiErrorMessage(cause instanceof Error ? null : cause, t('admin.accounts.stateTicket.harvestFailed'))
  } finally {
    if (generation === currentGeneration) busy.value = false
  }
}

watch(() => [props.accountId, props.visible] as const, async ([accountId, visible], previous) => {
  const currentVisibilityRevision = ++visibilityRevision
  clearTimeout(timer)
  controller?.abort()
  loading.value = false
  const accountChanged = !previous || previous[0] !== accountId
  if (accountChanged) {
    generation++
    status.value = null
    enabled.value = false
    ticketPlan.value = 'pro'
    model.value = 'gpt-6-astra'
    error.value = ''
    saved.value = false
    busy.value = false
  }
  const currentGeneration = generation
  if (!visible) return
  if (!busy.value) await load(!status.value)
  if (generation === currentGeneration && visibilityRevision === currentVisibilityRevision && props.visible) schedulePoll()
}, { immediate: true })

onBeforeUnmount(() => { generation++; clearTimeout(timer); controller?.abort(); emit('busy-change', false); emit('dirty-change', false) })
</script>
