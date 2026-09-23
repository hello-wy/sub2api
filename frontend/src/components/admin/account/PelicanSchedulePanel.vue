<template>
  <section class="space-y-4" :aria-label="t('admin.accounts.pelicanTest.schedule')">
    <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('admin.accounts.pelicanTest.scheduleHint') }}</p>
    <p v-if="error" role="alert" class="text-sm text-red-600 dark:text-red-400">{{ error }}</p>
    <div v-if="loading" class="text-sm text-gray-500">{{ t('common.loading') }}</div>
    <template v-else>
      <div v-if="plan" class="space-y-2 rounded-xl border border-gray-200 p-4 dark:border-dark-600">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <span class="font-medium" :class="plan.enabled ? 'text-emerald-600' : 'text-gray-500'">
            {{ t(plan.enabled ? 'admin.accounts.pelicanTest.scheduleEnabled' : 'admin.accounts.pelicanTest.schedulePaused') }}
            · {{ t('admin.accounts.pelicanTest.everyMinutes', { count: plan.pelican_config?.interval_minutes }) }}
          </span>
          <div class="flex gap-2">
            <button class="btn btn-secondary" :disabled="busy || disabled" @click="toggle">{{ t(plan.enabled ? 'admin.accounts.pelicanTest.pause' : 'admin.accounts.pelicanTest.resume') }}</button>
            <button class="btn btn-secondary" :disabled="busy || disabled" @click="edit">{{ t('common.edit') }}</button>
          </div>
        </div>
        <p class="text-sm text-gray-500">{{ plan.model_id }} / {{ plan.pelican_config?.reasoning_effort }} · {{ t('admin.accounts.pelicanTest.parallel') }} {{ plan.pelican_config?.parallel_count }}</p>
        <p class="text-sm text-gray-500">{{ t('admin.accounts.pelicanTest.nextRun') }}：{{ plan.enabled ? date(plan.next_run_at) : '—' }}</p>
        <p class="text-sm text-gray-500">{{ t('admin.accounts.pelicanTest.stopsAt') }}：{{ date(plan.expires_at) }}</p>
        <p v-if="isRunning" class="text-sm text-amber-600">{{ t('admin.accounts.pelicanTest.runningShort') }}</p>
        <p class="text-sm text-emerald-600">{{ t('admin.accounts.pelicanTest.lastSuccess') }}：{{ date(lastSuccess?.finished_at) }}</p>
      </div>
      <form v-if="!plan || editing" class="flex flex-wrap items-end gap-3" @submit.prevent="save">
        <Input v-model="interval" type="number" :label="t('admin.accounts.pelicanTest.intervalMinutes')" :disabled="busy" />
        <Input v-model="runForHours" type="number" :label="t('admin.accounts.pelicanTest.runForHours')" :disabled="busy" />
        <button class="btn btn-primary" type="submit" :disabled="busy || disabled || !valid">{{ t(plan ? 'common.save' : 'admin.accounts.pelicanTest.createSchedule') }}</button>
        <button v-if="plan" class="btn btn-secondary" type="button" :disabled="busy" @click="editing = false">{{ t('common.cancel') }}</button>
      </form>
      <p class="text-xs text-gray-500">{{ t('admin.accounts.pelicanTest.scheduleSaveHint') }}</p>
      <div class="space-y-2">
        <h3 class="text-sm font-medium text-gray-900 dark:text-gray-100">{{ t('admin.accounts.pelicanTest.scheduledHistory') }}</h3>
        <p v-if="!results.length" class="text-sm text-gray-500">{{ t('admin.accounts.pelicanTest.noHistory') }}</p>
        <button v-for="result in results" :key="result.id" type="button" :disabled="disabled || busy"
          class="flex w-full flex-wrap items-center justify-between gap-2 rounded-lg border border-gray-200 p-3 text-left dark:border-dark-600" @click="preview(result)">
          <span class="text-sm text-gray-700 dark:text-gray-200">
            {{ date(result.started_at) }} · {{ result.pelican_config?.model_id }} / {{ result.pelican_config?.reasoning_effort }}
            <span class="block text-xs text-gray-500">{{ (result.latency_ms / 1000).toFixed(1) }} s · {{ t('admin.accounts.pelicanTest.preview') }}</span>
            <span v-if="result.error_message" class="block break-all text-xs text-red-500">{{ result.error_message }}</span>
          </span>
          <span class="text-xs" :class="result.status === 'success' ? 'text-emerald-600' : 'text-red-500'">{{ t(result.status === 'success' ? 'admin.accounts.pelicanTest.success' : 'admin.accounts.pelicanTest.failed') }}</span>
        </button>
      </div>
    </template>
  </section>
</template>
<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Input from '@/components/common/Input.vue'
import { scheduledTestsAPI } from '@/api/admin/scheduledTests'
import type { PelicanTestConfig, ScheduledTestPlan, ScheduledTestResult } from '@/types'
const props = defineProps<{ accountId: number; modelId: string; prompt: string; reasoningEffort: string; parallelCount: number; disabled: boolean }>()
const emit = defineEmits<{ edit: [config: PelicanTestConfig, model: string]; preview: [result: ScheduledTestResult] }>()
const { t } = useI18n()
const plan = ref<ScheduledTestPlan | null>(null)
const results = ref<ScheduledTestResult[]>([])
const interval = ref<string | number>(30)
const runForHours = ref<string | number>(24)
const editing = ref(false)
const loading = ref(true)
const busy = ref(false)
const error = ref('')
let alive = true
let revision = 0
let timer: ReturnType<typeof setTimeout> | undefined
const lastSuccess = computed(() => results.value.find(result => result.status === 'success'))
const isRunning = computed(() => Boolean(plan.value?.running_until && new Date(plan.value.running_until).getTime() > Date.now()))
const valid = computed(() => props.prompt.trim() && props.modelId.trim() && Number.isInteger(Number(interval.value)) && Number(interval.value) >= 1 && Number(interval.value) <= 10080 && Number.isInteger(props.parallelCount) && props.parallelCount >= 1 && props.parallelCount <= 8 && Number.isInteger(Number(runForHours.value)) && Number(runForHours.value) >= 1 && Number(runForHours.value) <= 168 && Number(interval.value) < Number(runForHours.value) * 60)
function date(value?: string | null) { return value ? new Date(value).toLocaleString() : '—' }
function failure(e: unknown) { error.value = e instanceof Error ? e.message : t('admin.accounts.pelicanTest.scheduleError') }
async function refresh() {
  const currentRevision = revision
  try {
    const plans = await scheduledTestsAPI.listByAccount(props.accountId)
    const current = plans.find(item => item.pelican_config) || null
    const history = current ? await scheduledTestsAPI.listResults(current.id, 50, false) : []
    if (!alive || currentRevision !== revision) return
    plan.value = current
    results.value = history
    error.value = ''
  } catch (e) { if (alive && currentRevision === revision) failure(e) }
  finally { if (alive) loading.value = false }
}
async function poll() {
  if (!busy.value) await refresh()
  if (alive) timer = setTimeout(poll, 15000)
}
async function preview(result: ScheduledTestResult) {
  if (!plan.value || busy.value) return
  busy.value = true
  try {
    const full = await scheduledTestsAPI.getResult(plan.value.id, result.id)
    if (alive) emit('preview', full)
  } catch (e) { failure(e) }
  finally { busy.value = false }
}
function edit() {
  if (!plan.value?.pelican_config) return
  interval.value = plan.value.pelican_config.interval_minutes
  runForHours.value = plan.value.pelican_config.run_for_hours
  emit('edit', plan.value.pelican_config, plan.value.model_id)
  editing.value = true
}
async function save() {
  if (!valid.value || busy.value) return
  busy.value = true
  revision++
  error.value = ''
  const config: PelicanTestConfig = { run_for_hours: Number(runForHours.value), prompt: props.prompt, reasoning_effort: props.reasoningEffort, parallel_count: props.parallelCount, interval_minutes: Number(interval.value) }
  try {
    const data = { model_id: props.modelId.trim(), cron_expression: '*/30 * * * *', pelican_config: config, max_results: 50 }
    plan.value = plan.value ? await scheduledTestsAPI.update(plan.value.id, data) : await scheduledTestsAPI.create({ ...data, account_id: props.accountId, enabled: true })
    editing.value = false
  } catch (e) { failure(e) }
  finally { busy.value = false }
}
async function toggle() {
  if (!plan.value || busy.value) return
  busy.value = true
  revision++
  error.value = ''
  try { plan.value = await scheduledTestsAPI.update(plan.value.id, { enabled: !plan.value.enabled }) }
  catch (e) { failure(e) }
  finally { busy.value = false }
}
onMounted(poll)
onBeforeUnmount(() => { alive = false; clearTimeout(timer) })
</script>
