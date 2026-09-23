<template>
  <BaseDialog :show="show" :title="t('admin.accounts.ipChannels.title', { name: account?.name || '' })" width="extra-wide" :close-on-click-outside="!busy && !ticketChannel" :close-on-escape="!busy && !ticketChannel" @close="close">
    <div class="space-y-5">
      <div class="flex items-start justify-between gap-3">
        <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('admin.accounts.ipChannels.description') }}</p>
        <button type="button" class="btn btn-secondary shrink-0" :disabled="busy || loading || ticketBusy" @click="loadChannels()">{{ t('common.refresh') }}</button>
      </div>
      <p class="text-xs text-gray-500 dark:text-gray-400" data-testid="ip-channel-order-hint">{{ t('admin.accounts.ipChannels.orderHint') }}</p>
      <p v-if="isAccountModelMismatchQuarantined(account) || channels.some(channel => isModelDegradationMarker(channel.model_mismatch) && channel.model_mismatch?.quarantined !== false)" role="status" class="rounded-lg bg-red-50 p-3 text-sm text-red-700 dark:bg-red-900/20 dark:text-red-300">{{ t('accountModelMismatch.channelsPaused') }}</p>
      <p v-else-if="hasAccountModelMismatch(account) || channels.some(channel => isModelDegradationMarker(channel.model_mismatch))" role="status" class="rounded-lg bg-amber-50 p-3 text-sm text-amber-800 dark:bg-amber-900/20 dark:text-amber-200">{{ t('accountModelMismatch.observedExplanation') }}</p>
      <p v-if="error" role="alert" class="rounded-lg bg-red-50 p-3 text-sm text-red-700 dark:bg-red-900/20 dark:text-red-300">{{ error }}</p>
      <p v-if="loading && !channels.length" class="py-6 text-center text-gray-500">{{ t('common.loading') }}</p>
      <div v-else class="space-y-3">
        <section v-for="channel in channels" :key="channel.id" class="rounded-xl border border-gray-200 p-4 dark:border-dark-600" :data-testid="`ip-channel-${channel.id}`">
          <div class="mb-3 flex flex-wrap items-center justify-between gap-2">
            <div class="flex items-center gap-2">
              <span class="font-medium text-gray-900 dark:text-gray-100">{{ channel.proxy?.name || t('admin.accounts.ipChannels.direct') }}</span>
              <span v-if="isModelDegradationMarker(channel.model_mismatch)" class="badge badge-danger text-xs">{{ t('accountModelMismatch.label') }}</span>
              <span v-if="!isModelDegradationMarker(channel.model_mismatch) || channel.model_mismatch?.quarantined === false" class="rounded-full px-2 py-0.5 text-xs" :class="accountIPChannelStateClass(accountIPChannelState(channel, now))" :data-testid="`ip-channel-state-${channel.id}`">
                {{ t(accountIPChannelStateKey(accountIPChannelState(channel, now))) }}
              </span>
            </div>
            <span class="font-mono text-sm tabular-nums text-gray-700 dark:text-gray-300">{{ t('admin.accounts.ipChannels.load', { current: channel.current_concurrency ?? '—', max: channel.concurrency > 0 ? channel.concurrency : '∞' }) }}</span>
          </div>
          <p v-if="channel.error_message" class="mb-3 break-words text-sm text-red-600 dark:text-red-300">{{ channel.error_message }}</p>
          <p v-if="hasUpstream429Observation(channel) && !upstream429IsCoolingDown(channel)" class="mb-3 text-xs text-amber-700 dark:text-amber-300">{{ t('admin.accounts.status.upstream429ObservationHint') }}</p>
          <p v-if="cooldownUntil(channel)" class="mb-3 text-xs text-amber-700 dark:text-amber-300">{{ t('admin.accounts.ipChannels.cooldown', { until: formatDateTime(cooldownUntil(channel)!) }) }}</p>
          <form v-if="drafts[channel.id]" class="grid grid-cols-1 items-end gap-3 sm:grid-cols-2 lg:grid-cols-[minmax(10rem,1fr)_6rem_6rem_6rem_auto]" @submit.prevent="saveChannel(channel)">
            <label class="block text-sm text-gray-700 dark:text-gray-300">
              <span class="mb-1 block">{{ t('admin.accounts.proxy') }}</span>
              <select v-model.number="drafts[channel.id].proxy_id" class="input" :disabled="locked || !canChangeProxy(channel)" :aria-label="t('admin.accounts.proxy')">
                <option v-if="channel.proxy_id == null" :value="0">{{ t('admin.accounts.ipChannels.direct') }}</option>
                <option v-for="proxy in proxyChoices(channel)" :key="proxy.id" :value="proxy.id">{{ proxy.name }} · {{ proxy.host }}:{{ proxy.port }}</option>
              </select>
            </label>
            <label class="block text-sm text-gray-700 dark:text-gray-300">
              <span class="mb-1 block">{{ t('admin.accounts.concurrency') }}</span>
              <input v-model.number="drafts[channel.id].concurrency" type="number" min="0" max="10000" step="1" required class="input" :disabled="locked" :aria-label="t('admin.accounts.concurrency')" />
              <span class="text-xs text-gray-500">{{ t('admin.accounts.ipChannels.zeroUnlimited') }}</span>
            </label>
            <label class="block text-sm text-gray-700 dark:text-gray-300">
              <span class="mb-1 block">{{ t('admin.accounts.ipChannels.order') }}</span>
              <input v-model.number="drafts[channel.id].priority" type="number" min="0" step="1" required class="input" :disabled="locked" :aria-label="t('admin.accounts.ipChannels.order')" />
            </label>
            <label class="block text-sm text-gray-700 dark:text-gray-300">
              <span class="mb-1 block">{{ t('admin.accounts.ipChannels.loadFactor') }}</span>
              <input v-model.number="drafts[channel.id].load_factor" type="number" min="0" max="10000" step="1" class="input" :placeholder="String(drafts[channel.id].concurrency)" :disabled="locked" />
            </label>
            <div class="flex flex-wrap items-center gap-2">
              <button type="submit" class="btn btn-primary" :disabled="locked">{{ t('common.save') }}</button>
              <button type="button" class="btn btn-secondary" :disabled="locked" :data-testid="`toggle-channel-${channel.id}`" @click="toggleChannel(channel)">{{ t(`admin.accounts.ipChannels.${channel.enabled ? 'pause' : 'resume'}`) }}</button>
              <button type="button" class="rounded-lg px-2 py-2 text-sm text-red-600 disabled:cursor-not-allowed disabled:opacity-40" :disabled="locked || !canRemove(channel)" :title="t('admin.accounts.ipChannels.removeHint')" :data-testid="`remove-channel-${channel.id}`" @click="removeTarget = channel">{{ t('common.delete') }}</button>
            </div>
          </form>
          <div class="mt-3 flex flex-wrap items-center gap-3">
            <button v-if="hasRecoverableIPChannelState(channel)" type="button" class="text-sm text-emerald-600 hover:underline" :disabled="locked" :data-testid="`recover-channel-${channel.id}`" :title="t('admin.accounts.ipChannels.recoverHint')" @click="recoverChannel(channel)">{{ t('admin.accounts.ipChannels.recover') }}</button>
            <button v-if="account?.platform === 'openai' && account?.type === 'oauth' && !account?.parent_account_id" type="button" class="text-sm text-primary-600 hover:underline" :disabled="locked" :data-testid="`state-channel-${channel.id}`" @click="ticketChannel = channel">{{ t('admin.accounts.stateTicket.channelEntry') }}</button>
            <button type="button" class="text-sm text-primary-600 hover:underline" :disabled="busy" @click="statsChannel = channel">{{ t('admin.accounts.ipChannels.stats') }}</button>
            <button type="button" class="text-sm text-primary-600 hover:underline" :disabled="locked || !channel.schedulable" @click="emit('test', channel.id)">{{ t('admin.accounts.ipChannels.test') }}</button>
            <button type="button" class="text-sm text-primary-600 hover:underline" :disabled="locked" @click="emit('schedule', channel.id)">{{ t('admin.accounts.ipChannels.schedule') }}</button>
          </div>
          <AccountTrafficControls :account-id="channel.id" :platform="account?.platform || 'openai'" :hard-limit="channel.concurrency" :disabled="locked" />
        </section>
      </div>
      <p v-if="channels.length" class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accounts.ipChannels.removeHint') }}</p>
      <form class="space-y-3 rounded-xl border border-dashed border-gray-300 p-4 dark:border-dark-600" @submit.prevent="addChannels">
        <h4 class="font-medium text-gray-900 dark:text-white">{{ t('admin.accounts.ipChannels.add') }}</h4>
        <ProxyMultiSelector v-model="selectedProxyIds" :proxies="availableProxies" :max="Math.max(0, 50 - channels.length)" :disabled="locked || channels.length >= 50" />
        <div class="flex flex-wrap items-end gap-3">
          <label class="block w-32 text-sm text-gray-700 dark:text-gray-300">
            <span class="mb-1 block">{{ t('admin.accounts.ipChannels.perIPConcurrency') }}</span>
            <input v-model.number="newConcurrency" class="input" type="number" min="1" max="10000" step="1" required :disabled="locked" />
          </label>
          <label class="block w-32 text-sm text-gray-700 dark:text-gray-300">
            <span class="mb-1 block">{{ t('admin.accounts.ipChannels.order') }}</span>
            <input v-model.number="newPriority" class="input" type="number" min="0" step="1" required :disabled="locked" />
          </label>
          <button type="submit" class="btn btn-primary" data-testid="add-ip-channels" :disabled="locked || !selectedProxyIds.length">{{ t('admin.accounts.ipChannels.addSelected', { count: selectedProxyIds.length }) }}</button>
        </div>
      </form>
    </div>
    <template #footer><button type="button" class="btn btn-secondary" :disabled="busy || ticketBusy" @click="close">{{ t('common.close') }}</button></template>
  </BaseDialog>
  <BaseDialog :show="!!ticketChannel && show" :title="t('admin.accounts.stateTicket.channelTitle', { name: ticketChannel?.proxy?.name || t('admin.accounts.ipChannels.direct') })" :close-on-click-outside="!ticketBusy && !discardTicketConfirm" :close-on-escape="!ticketBusy && !discardTicketConfirm" @close="closeTicket">
    <CodexAccountTicketSettings v-if="ticketChannel && show" :account-id="ticketChannel.id" :visible="show" :proxy-changed="!!drafts[ticketChannel.id] && drafts[ticketChannel.id].proxy_id !== (ticketChannel.proxy_id ?? 0)" @busy-change="ticketBusy = $event" @dirty-change="ticketDirty = $event" />
  </BaseDialog>
  <ConfirmDialog :show="discardTicketConfirm" :title="t('admin.accounts.stateTicket.discardTitle')" :message="t('admin.accounts.stateTicket.discardMessage')" danger @confirm="discardTicket" @cancel="discardTicketConfirm = false" />
  <AccountStatsModal :show="!!statsChannel" :account="statsAccount" :channel-id="statsChannel?.id" @close="statsChannel = null" />
  <ConfirmDialog :show="!!removeTarget" :title="t('admin.accounts.ipChannels.removeTitle')" :message="t('admin.accounts.ipChannels.removeConfirm', { name: removeTarget?.proxy?.name || t('admin.accounts.ipChannels.direct') })" danger @confirm="removeChannel" @cancel="!busy && (removeTarget = null)" />
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useIntervalFn } from '@vueuse/core'
import AccountTrafficControls from './AccountTrafficControls.vue'
import CodexAccountTicketSettings from './CodexAccountTicketSettings.vue'
import AccountStatsModal from '@/components/admin/account/AccountStatsModal.vue'
import { useI18n } from 'vue-i18n'
import type { Account, AccountListItem, AccountIPChannel, Proxy } from '@/types'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import ProxyMultiSelector from '@/components/common/ProxyMultiSelector.vue'
import { addAccountIPChannels, recoverAccountIPChannel, listAccountIPChannels, removeAccountIPChannel, updateAccountIPChannel } from '@/api/admin/accountChannels'
import { hasRecoverableIPChannelState, sortAccountIPChannels } from '@/utils/accountIPChannels'
import { accountIPChannelState, accountIPChannelStateKey, accountIPChannelStateClass } from '@/utils/accountIPChannelState'
import { hasAccountModelMismatch, isAccountModelMismatchQuarantined, isModelDegradationMarker } from '@/utils/accountModelMismatch'
import { hasUpstream429Observation, activeTemporaryCooldown, upstream429IsCoolingDown, upstream429Enforced } from '@/utils/upstream429'
import { isCodexImportProxyAvailable } from '@/utils/codexImport'
import { formatDateTime } from '@/utils/format'
import { useAppStore } from '@/stores/app'

const props = defineProps<{ show: boolean; account: AccountListItem | null; proxies: Proxy[] }>()
const emit = defineEmits<{ close: []; updated: []; test: [channelId: number]; schedule: [channelId: number] }>()
const { t } = useI18n()
const app = useAppStore()
const channels = ref<AccountIPChannel[]>([])
type ChannelDraft = { proxy_id: number; concurrency: number; priority: number; load_factor: number | string }
const drafts = ref<Record<number, ChannelDraft>>({})
let draftSnapshots: Record<number, ChannelDraft> = {}
const channelDraft = (channel: AccountIPChannel): ChannelDraft => ({ proxy_id: channel.proxy_id ?? 0, concurrency: channel.concurrency, priority: channel.priority, load_factor: channel.load_factor ?? '' })
const selectedProxyIds = ref<number[]>([])
const newConcurrency = ref(50)
const newPriority = ref(50)
const loading = ref(false)
const loaded = ref(false)
const busy = ref(false)
const error = ref('')
const removeTarget = ref<AccountIPChannel | null>(null)
let controller: AbortController | null = null
let generation = 0
const now = ref(Date.now())
const { pause: pauseClock, resume: resumeClock } = useIntervalFn(() => { now.value = Date.now() }, 1000, { immediate: false })
const statsChannel = ref<AccountIPChannel | null>(null)
const ticketChannel = ref<AccountIPChannel | null>(null)
const ticketBusy = ref(false)
const ticketDirty = ref(false)
const discardTicketConfirm = ref(false)
const statsAccount = computed(() => props.account && statsChannel.value ? { ...props.account, name: `${props.account.name} · ${statsChannel.value.proxy?.name || t('admin.accounts.ipChannels.direct')}`, status: (statsChannel.value.status === 'error' || statsChannel.value.status === 'inactive' ? statsChannel.value.status : 'active') as Account['status'] } : null)
const locked = computed(() => busy.value || loading.value || !loaded.value || ticketBusy.value)
const availableProxies = computed(() => props.proxies.filter(proxy => isCodexImportProxyAvailable(proxy) && !channels.value.some(channel => channel.proxy_id === proxy.id)))
const proxyChoices = (channel: AccountIPChannel) => {
  const choices = props.proxies.filter(proxy => proxy.id === channel.proxy_id || availableProxies.value.some(available => available.id === proxy.id))
  if (channel.proxy && !choices.some(proxy => proxy.id === channel.proxy_id)) return [channel.proxy, ...choices]
  return choices
}
const canChangeProxy = (channel: AccountIPChannel) => !channel.enabled && channel.current_concurrency === 0 && !!channel.disabled_at && now.value - new Date(channel.disabled_at).getTime() >= 120000
const canRemove = (channel: AccountIPChannel) => channels.value.length > 1 && canChangeProxy(channel)
const cooldownUntil = (channel: AccountIPChannel) => [upstream429Enforced(channel) ? channel.rate_limit_reset_at : null, channel.overload_until, activeTemporaryCooldown(channel)]
  .filter((value): value is string => !!value && new Date(value).getTime() > now.value)
  .sort((a, b) => new Date(b).getTime() - new Date(a).getTime())[0]
const errorMessage = (value: unknown) => value instanceof Error ? value.message : (value as { message?: string })?.message || t('admin.accounts.ipChannels.failed')

async function loadChannels(savedChannelId?: number) {
  controller?.abort()
  if (!props.show || !props.account) return
  const current = new AbortController()
  controller = current
  loading.value = true
  error.value = ''
  const id = props.account.id
  try {
    const items = await listAccountIPChannels(id, current.signal)
    if (current.signal.aborted || props.account?.id !== id || !props.show) return
    const snapshots = Object.fromEntries(items.map(channel => [channel.id, channelDraft(channel)]))
    const nextDrafts = Object.fromEntries(items.map(channel => {
      const draft = drafts.value[channel.id]
      const dirty = draft && JSON.stringify(draft) !== JSON.stringify(draftSnapshots[channel.id])
      return [channel.id, channel.id !== savedChannelId && dirty ? draft : { ...snapshots[channel.id] }]
    }))
    channels.value = sortAccountIPChannels(items)
    draftSnapshots = snapshots
    drafts.value = nextDrafts
    loaded.value = true
  } catch (failure) {
    if (current.signal.aborted) return
    loaded.value = false
    error.value = errorMessage(failure)
  } finally {
    if (controller === current) loading.value = false
  }
}

async function mutate(operation: () => Promise<void>, onSuccess?: () => void, savedChannelId?: number) {
  if (locked.value || !props.account) return
  const operationGeneration = generation
  busy.value = true
  error.value = ''
  try {
    await operation()
    if (operationGeneration !== generation || !props.show) return
    onSuccess?.()
    removeTarget.value = null
    emit('updated')
    app.showSuccess(t('admin.accounts.ipChannels.saved'))
    await loadChannels(savedChannelId)
  } catch (failure) {
    if (operationGeneration === generation && props.show) error.value = errorMessage(failure)
  } finally {
    if (operationGeneration === generation) busy.value = false
  }
}

function saveChannel(channel: AccountIPChannel) {
  const draft = drafts.value[channel.id]
  if (!props.account || !draft || !Number.isInteger(draft.concurrency) || draft.concurrency < 0 || draft.concurrency > 10000 || !Number.isInteger(draft.priority) || draft.priority < 0) return
  const factor = draft.load_factor === '' ? 0 : Number(draft.load_factor)
  if (!Number.isInteger(factor) || factor < 0 || factor > 10000) return
  const payload = { concurrency: draft.concurrency, priority: draft.priority, load_factor: factor, ...(draft.proxy_id !== (channel.proxy_id ?? 0) ? { proxy_id: draft.proxy_id } : {}) }
  const accountId = props.account.id
  void mutate(() => updateAccountIPChannel(accountId, channel.id, payload), undefined, channel.id)
}

function toggleChannel(channel: AccountIPChannel) {
  if (!props.account) return
  const accountId = props.account.id
  void mutate(() => updateAccountIPChannel(accountId, channel.id, { schedulable: !channel.enabled }))
}

function recoverChannel(channel: AccountIPChannel) {
  if (!props.account) return
  const accountId = props.account.id
  void mutate(() => recoverAccountIPChannel(accountId, channel.id))
}

function addChannels() {
  if (!props.account || !selectedProxyIds.value.length || !Number.isInteger(newConcurrency.value) || newConcurrency.value < 1 || newConcurrency.value > 10000 || !Number.isInteger(newPriority.value) || newPriority.value < 0) return
  const accountId = props.account.id
  const payload = { proxy_ids: [...selectedProxyIds.value], concurrency: newConcurrency.value, priority: newPriority.value }
  void mutate(() => addAccountIPChannels(accountId, payload), () => { selectedProxyIds.value = [] })
}

function removeChannel() {
  if (!props.account || !removeTarget.value || !canRemove(removeTarget.value)) return
  const accountId = props.account.id
  const channelId = removeTarget.value.id
  void mutate(() => removeAccountIPChannel(accountId, channelId))
}
function closeTicket() {
  if (ticketBusy.value) return
  if (ticketDirty.value) { discardTicketConfirm.value = true; return }
  ticketChannel.value = null
}
function discardTicket() {
  if (ticketBusy.value) return
  discardTicketConfirm.value = false
  ticketDirty.value = false
  ticketChannel.value = null
}
function close() {
  if (busy.value || ticketBusy.value) return
  if (ticketChannel.value) { closeTicket(); return }
  emit('close')
}
watch(() => [props.show, props.account?.id] as const, ([show]) => {
  generation++
  controller?.abort()
  statsChannel.value = null
  ticketChannel.value = null
  ticketBusy.value = false
  ticketDirty.value = false
  discardTicketConfirm.value = false
  now.value = Date.now()
  if (show) resumeClock(); else pauseClock()
  channels.value = []
  drafts.value = {}
  draftSnapshots = {}
  busy.value = false
  loaded.value = false
  removeTarget.value = null
  selectedProxyIds.value = []
  error.value = ''
  if (show && props.account) {
    newConcurrency.value = 50
    newPriority.value = props.account.priority
    void loadChannels()
  }
}, { immediate: true })
onBeforeUnmount(() => { generation++; controller?.abort(); pauseClock() })
</script>
