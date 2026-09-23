<template>
  <BaseDialog :show="true" :title="`${t('groupStatus.probe.title')} · ${group.group_name}`" width="wide" :show-close-button="!saving" :close-on-escape="!saving" @close="close">
    <div v-if="loading" class="flex justify-center py-12" role="status"><LoadingSpinner /></div>
    <div v-else-if="loadError" class="rounded-xl bg-rose-50 p-4 text-sm text-rose-700 dark:bg-rose-950/30 dark:text-rose-300" role="alert">{{ t('groupStatus.probe.loadFailed') }}</div>
    <div v-else class="space-y-6">
      <p class="text-sm leading-relaxed text-gray-500 dark:text-gray-400">{{ t('groupStatus.probe.note') }}</p>
      <form ref="formElement" class="space-y-4" @submit.prevent="submit(false)">
        <label class="flex items-center gap-3 rounded-xl bg-gray-50 p-3 text-sm font-medium dark:bg-dark-700">
          <input v-model="form.enabled" type="checkbox" class="h-4 w-4 rounded text-primary-600 focus:ring-primary-500" data-testid="schedule-enabled" :disabled="saving" />
          {{ t('groupStatus.probe.schedule') }}
        </label>
        <div class="grid gap-4 sm:grid-cols-2">
          <label class="block sm:col-span-2">
            <span class="mb-1.5 block text-sm font-medium">{{ t('groupStatus.probe.model') }}</span>
            <input v-model="form.model" :list="modelListID" type="text" required maxlength="200" :placeholder="t('groupStatus.probe.modelPlaceholder')" class="input w-full" :disabled="saving" data-testid="probe-model" />
            <datalist :id="modelListID"><option v-for="model in models" :key="model" :value="model" /></datalist>
          </label>
          <label class="block">
            <span class="mb-1.5 block text-sm font-medium">{{ t('groupStatus.probe.reasoning') }}</span>
            <select v-model="form.reasoning_effort" class="input w-full" :disabled="saving" data-testid="probe-reasoning">
              <option v-for="option in reasoningOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
            </select>
          </label>
          <label class="block">
            <span class="mb-1.5 block text-sm font-medium">{{ t('groupStatus.probe.interval') }}</span>
            <input v-model.number="form.interval_seconds" type="number" min="60" max="86400" step="1" required class="input w-full" :disabled="saving" data-testid="probe-interval" />
            <span class="mt-1 block text-xs text-gray-400">60–86400</span>
          </label>
          <label class="block">
            <span class="mb-1.5 block text-sm font-medium">{{ t('groupStatus.probe.timeout') }}</span>
            <input v-model.number="form.timeout_seconds" type="number" min="5" max="120" step="1" required class="input w-full" :disabled="saving" data-testid="probe-timeout" />
            <span class="mt-1 block text-xs text-gray-400">5–120</span>
          </label>
          <label class="block">
            <span class="mb-1.5 block text-sm font-medium">{{ t('groupStatus.probe.maxOutput') }}</span>
            <input v-model.number="form.max_output_tokens" type="number" min="16" max="1024" step="1" required class="input w-full" :disabled="saving" data-testid="probe-output" />
            <span class="mt-1 block text-xs text-gray-400">16–1024</span>
          </label>
          <label class="block sm:col-span-2">
            <span class="mb-1.5 block text-sm font-medium">{{ t('groupStatus.probe.dailyBudget') }}</span>
            <input v-model.number="form.daily_token_budget" type="number" :min="Number(form.max_output_tokens) + 256" max="1000000" step="1" required class="input w-full" :disabled="saving" data-testid="probe-budget" />
            <span class="mt-1 block text-xs text-gray-400">{{ t('groupStatus.probe.budgetMinimum') }} {{ t('groupStatus.probe.budgetHint') }}</span>
          </label>
        </div>
      </form>
      <p class="text-xs text-gray-500 dark:text-gray-400">
        {{ savedConfig?.enabled && savedConfig.next_run_at ? t('groupStatus.probe.nextRun', { time: formatGroupTime(savedConfig.next_run_at, locale) }) : t('groupStatus.probe.manualOnly') }}
      </p>
      <p v-if="actionError" role="alert" class="text-sm text-rose-600 dark:text-rose-400">{{ actionError }}</p>
      <p v-if="notice" role="status" class="text-sm text-emerald-700 dark:text-emerald-400">{{ notice }}</p>

      <section class="border-t border-gray-100 pt-5 dark:border-dark-700" :aria-label="t('groupStatus.probe.history')">
        <h3 class="mb-3 text-sm font-semibold">{{ t('groupStatus.probe.history') }}</h3>
        <p v-if="historyError" role="alert" class="text-sm text-rose-600 dark:text-rose-400">{{ t('groupStatus.probe.loadFailed') }}</p>
        <p v-else-if="!history.length" class="py-4 text-center text-sm text-gray-400">{{ t('groupStatus.probe.historyEmpty') }}</p>
        <div v-else class="max-h-64 space-y-2 overflow-y-auto">
          <div v-for="run in history" :key="run.id" class="flex flex-wrap items-center justify-between gap-2 rounded-xl bg-gray-50 p-3 text-xs dark:bg-dark-700" data-testid="probe-history-row">
            <div class="min-w-0 space-y-1">
              <p class="break-words font-medium">{{ run.model }}</p>
              <p class="text-gray-500 dark:text-gray-400">{{ formatGroupTime(run.checked_at, locale) }}</p>
            </div>
            <div class="space-y-1 text-right">
              <p :class="run.status === 'success' ? 'text-emerald-700 dark:text-emerald-400' : run.status === 'failed' ? 'text-rose-600 dark:text-rose-400' : 'text-gray-500 dark:text-gray-400'">{{ t(`groupStatus.probe.status.${run.status}`) }}</p>
              <p class="text-gray-500 dark:text-gray-400">{{ formatGroupLatency(run.latency_ms) }} · {{ t('groupStatus.probe.tokens') }} {{ run.usage_recorded ? run.input_tokens + run.output_tokens : '—' }}</p>
            </div>
          </div>
        </div>
      </section>
    </div>
    <template #footer>
      <button type="button" class="btn btn-secondary" :disabled="saving" @click="close">{{ t('common.close') }}</button>
      <button type="button" class="btn btn-secondary" :disabled="loading || loadError || saving" data-testid="save-probe" @click="submit(false)">{{ t('groupStatus.probe.save') }}</button>
      <button type="button" class="btn btn-primary" :disabled="loading || loadError || saving || running" data-testid="run-probe" @click="submit(true)">
        <LoadingSpinner v-if="saving || running" size="sm" />
        {{ running ? t('groupStatus.probe.running') : t('groupStatus.probe.saveAndRun') }}
      </button>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import { getGroupProbeConfigs, getGroupProbeHistory, runGroupProbe, saveGroupProbeConfig, isGroupStatusDisabledError } from '@/api/groupStatus'
import type { GroupProbeConfig, GroupProbeRun, GroupProbeSettings, GroupServiceStatus } from '@/api/groupStatus'
import { formatGroupLatency, formatGroupTime } from './format'

const props = defineProps<{ group: GroupServiceStatus; models: string[]; selectedModel: string }>()
const emit = defineEmits<{ close: []; changed: [] }>()
const { t, locale } = useI18n()
const formElement = ref<HTMLFormElement>()
const modelListID = `group-probe-models-${props.group.group_id}`
const loading = ref(true)
const loadError = ref(false)
const historyError = ref(false)
const saving = ref(false)
const actionError = ref('')
const notice = ref('')
const savedConfig = ref<GroupProbeConfig | null>(null)
const history = ref<GroupProbeRun[]>([])
const running = computed(() => history.value.some(run => run.status === 'running'))
const form = reactive<GroupProbeSettings>({
  enabled: false,
  interval_seconds: 300,
  model: props.selectedModel,
  reasoning_effort: '',
  timeout_seconds: 60,
  max_output_tokens: 64,
  daily_token_budget: 20000,
})
const reasoningOptions = computed(() => [
  { value: '', label: t('groupStatus.probe.reasoningDefault') },
  { value: 'minimal', label: t('groupStatus.probe.reasoningMinimal') },
  { value: 'low', label: t('groupStatus.probe.reasoningLow') },
  { value: 'medium', label: t('groupStatus.probe.reasoningMedium') },
  { value: 'high', label: t('groupStatus.probe.reasoningHigh') },
  { value: 'xhigh', label: t('groupStatus.probe.reasoningXhigh') },
])
const controller = new AbortController()
let timer: ReturnType<typeof setTimeout> | undefined
let disposed = false

function close() { if (!saving.value) emit('close') }

function validSettings() {
  return form.model.trim().length > 0 && form.model.trim().length <= 200 && form.daily_token_budget >= form.max_output_tokens + 256 && [
    [form.interval_seconds, 60, 86400],
    [form.timeout_seconds, 5, 120],
    [form.max_output_tokens, 16, 1024],
    [form.daily_token_budget, 256, 1000000],
  ].every(([value, min, max]) => Number.isInteger(value) && value >= min && value <= max)
}

async function submit(run: boolean) {
  if (saving.value || loading.value || loadError.value || (run && running.value)) return
  actionError.value = ''
  notice.value = ''
  if (!validSettings() || formElement.value?.reportValidity() === false) {
    actionError.value = t('groupStatus.probe.invalid')
    return
  }
  saving.value = true
  let saved = false
  try {
    savedConfig.value = await saveGroupProbeConfig(props.group.group_id, { ...form, model: form.model.trim() })
    saved = true
    emit('changed')
    if (run) {
      const result = await runGroupProbe(props.group.group_id)
      history.value = [result, ...history.value.filter(item => item.id !== result.id)]
      notice.value = t('groupStatus.probe.started')
      await refreshHistory()
      emit('changed')
    } else {
      notice.value = t('groupStatus.probe.saved')
    }
  } catch (error: unknown) {
    const status = typeof error === 'object' && error !== null && 'status' in error ? error.status : undefined
    const budgetReached = status === 429 && typeof error === 'object' && error !== null && 'message' in error && error.message === 'daily probe token budget exhausted'
    actionError.value = t(isGroupStatusDisabledError(error) ? 'groupStatus.disabledAdmin' : status === 409 ? 'groupStatus.probe.busy' : budgetReached && saved ? 'groupStatus.probe.budgetExhausted' : saved ? 'groupStatus.probe.runFailed' : 'groupStatus.probe.saveFailed')
  } finally {
    saving.value = false
  }
}

async function refreshHistory() {
  if (disposed) return
  try {
    const latest = await getGroupProbeHistory(props.group.group_id, controller.signal)
    if (disposed) return
    const wasRunning = running.value
    history.value = latest
    historyError.value = false
    if (wasRunning && !running.value) emit('changed')
  } catch {
    if (!disposed) historyError.value = true
  }
}

function scheduleHistory() {
  if (disposed) return
  timer = setTimeout(async () => {
    if (!document.hidden) await refreshHistory()
    scheduleHistory()
  }, 5000)
}

onMounted(async () => {
  try {
    const [configs, runs] = await Promise.all([
      getGroupProbeConfigs(controller.signal),
      getGroupProbeHistory(props.group.group_id, controller.signal),
    ])
    if (disposed) return
    const config = configs.find(item => item.group_id === props.group.group_id)
    if (config) {
      savedConfig.value = config
      for (const key of Object.keys(form) as Array<keyof GroupProbeSettings>) {
        Object.assign(form, { [key]: config[key] })
      }
    }
    history.value = runs
    scheduleHistory()
  } catch {
    if (!disposed) loadError.value = true
  } finally {
    if (!disposed) loading.value = false
  }
})
onUnmounted(() => {
  disposed = true
  controller.abort()
  if (timer) clearTimeout(timer)
})
</script>
