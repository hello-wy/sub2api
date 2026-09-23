<template>
  <BaseDialog :show="show" :title="t('admin.accounts.stateTicket.batch.title')" width="wide" :show-close-button="!busy" :close-on-escape="!busy" @close="close">
    <div class="space-y-4">
      <p class="text-sm text-gray-600 dark:text-gray-300">{{ t('admin.accounts.stateTicket.batch.scope', { count: selectedIds.length }) }}</p>
      <p class="text-xs text-gray-500">{{ t('admin.accounts.stateTicket.batch.fixedIPHint') }}</p>
      <p v-if="selectedIds.length > 100" role="alert" class="text-sm text-red-600">{{ t('admin.accounts.stateTicket.batch.limit') }}</p>
      <div class="grid gap-3 sm:grid-cols-2">
        <label>
          <span class="input-label">{{ t('admin.accounts.stateTicket.batch.action') }}</span>
          <select v-model="action" :disabled="busy" class="input" data-testid="state-batch-action">
            <option value="enable">{{ t('admin.accounts.stateTicket.batch.enable') }}</option>
            <option value="disable">{{ t('admin.accounts.stateTicket.batch.disable') }}</option>
          </select>
        </label>
        <template v-if="action === 'enable'">
          <label>
            <span class="input-label">{{ t('admin.accounts.stateTicket.plan') }}</span>
            <select v-model="plan" :disabled="busy" class="input" data-testid="state-batch-plan">
              <option value="pro">{{ t('admin.accounts.stateTicket.planPro') }}</option>
              <option value="team">{{ t('admin.accounts.stateTicket.planTeam') }}</option>
            </select>
          </label>
          <label>
            <span class="input-label">{{ t('admin.accounts.stateTicket.modelLabel') }}</span>
            <select v-model="model" :disabled="busy" class="input" data-testid="state-batch-model">
              <option value="gpt-6-astra">gpt-6-astra</option>
              <option value="gpt-5.6-sol">gpt-5.6-sol</option>
            </select>
          </label>
          <label class="flex items-center gap-2 text-sm">
            <input v-model="harvest" type="checkbox" :disabled="busy" data-testid="state-batch-harvest" />
            {{ t('admin.accounts.stateTicket.batch.harvest') }}
          </label>
        </template>
      </div>
      <template v-if="action === 'enable'">
        <p v-if="loading" class="text-sm text-gray-500">{{ t('common.loading') }}</p>
        <div v-else-if="!globalStatus?.enabled || !globalStatus.harvest_proxy_configured" class="rounded-lg bg-amber-50 p-3 text-sm text-amber-800 dark:bg-amber-900/20 dark:text-amber-200" data-testid="state-batch-prerequisites">
          {{ t('admin.accounts.stateTicket.batch.prerequisites') }}
          <a class="font-medium underline" href="/admin/settings?tab=gateway" target="_blank" rel="noopener noreferrer">{{ t('admin.accounts.stateTicket.gatewaySettings') }}</a>
          <button type="button" class="ml-3 underline" :disabled="busy" @click="load">{{ t('common.refresh') }}</button>
        </div>
        <p class="text-xs text-gray-500">{{ t('admin.accounts.stateTicket.batch.harvestHint') }}</p>
      </template>
      <p class="text-xs text-gray-500">{{ t('admin.accounts.stateTicket.quarantineHint') }}</p>
      <p v-if="error" role="alert" class="text-sm text-red-600" data-testid="state-batch-error">{{ error }}</p>
      <div v-if="result" class="space-y-3" aria-live="polite" data-testid="state-batch-results">
        <p class="text-sm font-medium">{{ t('admin.accounts.stateTicket.batch.summary', { total: result.total_channels, saved: result.saved, skipped: result.skipped, failed: result.failed }) }}</p>
        <p class="text-xs text-gray-500">{{ t('admin.accounts.stateTicket.batch.resultHint') }}</p>
        <div class="max-h-80 overflow-auto rounded-lg border border-gray-200 dark:border-dark-600">
          <table class="w-full text-left text-sm">
            <thead class="sticky top-0 bg-gray-50 dark:bg-dark-800"><tr><th class="p-2">{{ t('admin.accounts.stateTicket.batch.channel') }}</th><th class="p-2">{{ t('admin.accounts.stateTicket.batch.result') }}</th><th class="p-2">{{ t('admin.accounts.stateTicket.batch.status') }}</th></tr></thead>
            <tbody>
              <tr v-for="item in result.results" :key="`${item.logical_account_id}:${item.account_id}`" class="border-t border-gray-100 dark:border-dark-700">
                <td class="p-2 align-top">#{{ item.logical_account_id }}<span v-if="item.account_id !== item.logical_account_id"> / #{{ item.account_id }}</span></td>
                <td class="p-2 align-top"><span :class="item.outcome === 'error' ? 'text-red-600' : 'text-gray-700 dark:text-gray-200'">{{ t(`admin.accounts.stateTicket.batch.outcomes.${item.outcome}`) }}</span><p v-if="item.message" class="mt-1 break-words text-xs text-gray-500">{{ item.message }}</p></td>
                <td class="p-2 align-top">
                  <span v-if="item.status">{{ t(`admin.accounts.stateTicket.states.${item.status.state}`) }}</span>
                  <span v-else>—</span>
                  <button v-if="item.saved || item.status" type="button" class="ml-2 text-primary-600 underline disabled:opacity-50" :disabled="busy || refreshing.has(item.account_id)" :data-testid="`state-batch-refresh-${item.account_id}`" @click="refreshItem(item)">{{ t('common.refresh') }}</button>
                  <p v-if="item.status?.last_error" class="mt-1 text-xs text-amber-700">{{ item.status.last_error }}</p>
                  <p v-if="refreshErrors[item.account_id]" role="alert" class="mt-1 text-xs text-red-600">{{ refreshErrors[item.account_id] }}</p>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>
    <template #footer>
      <div class="flex justify-end gap-3">
        <button type="button" class="btn btn-secondary" :disabled="busy" @click="close">{{ t('common.close') }}</button>
        <button type="button" class="btn btn-primary" :disabled="!canSubmit" data-testid="state-batch-submit" @click="submit">{{ busy ? t('common.loading') : action === 'disable' ? t('admin.accounts.stateTicket.batch.disableButton') : t('admin.accounts.stateTicket.batch.enableButton') }}</button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { configureCodexTicketBatch, getCodexAccountTicket, getCodexTicketGlobalSettings, type CodexTicketBatchItem, type CodexTicketBatchResult, type CodexTicketGlobalSettings, type CodexTicketPlan } from '@/api/admin/codexTickets'
import { extractApiErrorMessage } from '@/utils/apiError'

const props = defineProps<{ show: boolean; accountIds: number[] }>()
const emit = defineEmits<{ close: []; updated: [] }>()
const { t } = useI18n()
const action = ref<'enable' | 'disable'>('enable')
const plan = ref<CodexTicketPlan>('pro')
const model = ref('gpt-6-astra')
const harvest = ref(true)
const busy = ref(false)
const loading = ref(false)
const error = ref('')
const globalStatus = ref<CodexTicketGlobalSettings | null>(null)
const result = ref<CodexTicketBatchResult | null>(null)
const refreshing = ref(new Set<number>())
const refreshErrors = ref<Record<number, string>>({})
const selectedIds = computed(() => [...new Set(props.accountIds)])
const canSubmit = computed(() => props.show && !busy.value && selectedIds.value.length > 0 && selectedIds.value.length <= 100 && (action.value === 'disable' || (!loading.value && !!globalStatus.value?.enabled && globalStatus.value.harvest_proxy_configured)))
let generation = 0
let readRevision = 0
let controller: AbortController | undefined

async function load() {
  if (!props.show || busy.value) return
  controller?.abort()
  const current = new AbortController()
  controller = current
  const token = generation
  loading.value = true
  globalStatus.value = null
  error.value = ''
  try {
    const next = await getCodexTicketGlobalSettings(current.signal)
    if (!current.signal.aborted && generation === token) globalStatus.value = next
  } catch (cause) {
    if (!current.signal.aborted && generation === token) error.value = extractApiErrorMessage(cause instanceof Error ? null : cause, t('admin.accounts.stateTicket.loadFailed'))
  } finally {
    if (generation === token && controller === current) loading.value = false
  }
}

async function submit() {
  if (!canSubmit.value) return
  controller?.abort()
  loading.value = false
  const token = generation
  readRevision++
  refreshing.value = new Set()
  refreshErrors.value = {}
  busy.value = true
  error.value = ''
  result.value = null
  try {
    const next = await configureCodexTicketBatch({ account_ids: [...selectedIds.value], enabled: action.value === 'enable', harvest: action.value === 'enable' && harvest.value, ...(action.value === 'enable' ? { ticket_plan: plan.value, model: model.value } : {}) })
    if (generation !== token) return
    result.value = next
    emit('updated')
  } catch (cause) {
    if (generation === token) {
      error.value = extractApiErrorMessage(cause instanceof Error ? null : cause, t('admin.accounts.stateTicket.batch.failed'))
      // A lost response does not prove that every mutation failed.
      emit('updated')
    }
  } finally {
    if (generation === token) busy.value = false
  }
}

async function refreshItem(item: CodexTicketBatchItem) {
  if (!props.show || busy.value || refreshing.value.has(item.account_id)) return
  const token = generation
  const revision = readRevision
  refreshing.value.add(item.account_id)
  delete refreshErrors.value[item.account_id]
  try {
    const next = await getCodexAccountTicket(item.account_id)
    if (generation === token && revision === readRevision) item.status = next
  } catch (cause) {
    if (generation === token && revision === readRevision) refreshErrors.value[item.account_id] = extractApiErrorMessage(cause instanceof Error ? null : cause, t('admin.accounts.stateTicket.loadFailed'))
  } finally {
    if (generation === token && revision === readRevision) refreshing.value.delete(item.account_id)
  }
}

function close() { if (!busy.value) emit('close') }
watch(() => [props.show, props.accountIds.join(',')] as const, () => {
  generation++
  controller?.abort()
  busy.value = false
  loading.value = false
  error.value = ''
  result.value = null
  globalStatus.value = null
  refreshing.value = new Set()
  refreshErrors.value = {}
  action.value = 'enable'
  plan.value = 'pro'
  model.value = 'gpt-6-astra'
  harvest.value = true
  if (props.show) void load()
}, { immediate: true })
onBeforeUnmount(() => { generation++; controller?.abort() })
</script>
