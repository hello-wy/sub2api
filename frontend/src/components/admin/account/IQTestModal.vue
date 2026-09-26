<template>
  <PelicanRecordsDashboard v-if="show && dashboardOpen" :accounts="props.accounts || []" :account="props.account" :manual-record="records[0] || null" @close="dashboardOpen = false" />
  <BaseDialog :show="show && !dashboardOpen" :title="t('admin.accounts.pelicanTest.title')" width="extra-wide" :fullscreen="viewingScheduled" @close="handleClose">
    <div class="space-y-6">
      <div v-if="account" class="flex flex-wrap items-center justify-between gap-3 rounded-xl bg-gray-50 px-4 py-3 dark:bg-dark-900/50">
        <div class="flex min-w-0 items-center gap-3">
          <div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-primary-100 text-primary-600 dark:bg-primary-500/15 dark:text-primary-400">
            <Icon name="brain" size="md" />
          </div>
          <div class="min-w-0">
            <div class="truncate font-semibold text-gray-900 dark:text-gray-100">{{ account.name }}</div>
            <div class="mt-0.5 flex items-center gap-2 text-xs text-gray-500 dark:text-dark-400">
              <span class="uppercase">{{ account.platform }}</span><span aria-hidden="true">·</span><span>{{ account.type }}</span>
            </div>
          </div>
        </div>
        <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.accounts.pelicanTest.subtitle') }}</span>
      </div>

      <nav class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-200 pb-3 dark:border-dark-700" :aria-label="t('admin.accounts.pelicanTest.title')">
        <div class="inline-flex rounded-xl bg-gray-100 p-1 dark:bg-dark-900">
          <button type="button" data-testid="manual-tab" :aria-pressed="activeTab === 'results'" :disabled="running"
            class="inline-flex items-center gap-2 rounded-lg px-3 py-2 text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 disabled:cursor-not-allowed disabled:opacity-60"
            :class="activeTab === 'results' ? 'bg-white text-primary-600 shadow-sm dark:bg-dark-700 dark:text-primary-400' : 'text-gray-500 hover:text-gray-900 dark:text-dark-400 dark:hover:text-gray-100'"
            @click="openManualResults"><Icon name="play" size="sm" />{{ t(viewingScheduled ? 'admin.accounts.pelicanTest.scheduledPreview' : 'admin.accounts.pelicanTest.manualTab') }}</button>
          <button type="button" data-testid="schedule-tab" :aria-pressed="activeTab === 'schedule'" :disabled="running"
            class="inline-flex items-center gap-2 rounded-lg px-3 py-2 text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 disabled:cursor-not-allowed disabled:opacity-60"
            :class="activeTab === 'schedule' ? 'bg-white text-primary-600 shadow-sm dark:bg-dark-700 dark:text-primary-400' : 'text-gray-500 hover:text-gray-900 dark:text-dark-400 dark:hover:text-gray-100'"
            @click="activeTab = 'schedule'"><Icon name="clock" size="sm" />{{ t('admin.accounts.pelicanTest.schedule') }}</button>
        </div>
        <button type="button" data-testid="history-button" class="btn btn-ghost px-3 py-2" :disabled="running" @click="dashboardOpen = true">
          <Icon name="chart" size="sm" />{{ t('admin.accounts.pelicanTest.history') }}
          <span v-if="records.length" class="rounded-md bg-gray-100 px-1.5 py-0.5 text-xs tabular-nums dark:bg-dark-700">{{ records.length }}</span>
        </button>
      </nav>

      <ScheduledTestsPanel v-if="show && account && activeTab === 'schedule'" :key="account.id" :show="true" embedded
        :account-id="account.id" :default-model="modelId" :model-options="[{ value: modelId, label: modelId }]"
        :pelican-config="{ question_kind: questionKind, prompt, reasoning_effort: reasoningEffort, parallel_count: Number(parallelCount) }"
        :disabled="running" @preview="previewScheduled" />

      <template v-else>
        <section v-if="!viewingScheduled" class="grid gap-5 lg:grid-cols-[minmax(0,1fr)_280px]" :aria-label="t('admin.accounts.pelicanTest.configuration')">
          <div class="min-w-0 space-y-4">
            <div>
              <label id="iq-question-label" class="input-label">{{ t('admin.accounts.pelicanTest.question') }}</label>
              <Select data-testid="question-select" aria-labelledby="iq-question-label" :model-value="questionKind" :options="questionOptions" :disabled="running" @update:model-value="selectQuestion" />
            </div>
            <TextArea v-model="prompt" :label="t('admin.accounts.pelicanTest.promptLabel')" :disabled="running" :rows="5" :hint="t('admin.accounts.pelicanTest.promptHint')" />
            <p v-if="questionKind === 'candy'" class="flex items-start gap-2 text-xs leading-relaxed text-gray-500 dark:text-dark-400">
              <Icon name="infoCircle" size="sm" class="mt-0.5 shrink-0" />{{ t('admin.accounts.pelicanTest.candyHint') }}
            </p>
          </div>
          <div class="space-y-4 border-gray-200 lg:border-l lg:pl-5 dark:border-dark-700">
            <Input v-model="modelId" :label="t('admin.accounts.pelicanTest.model')" :disabled="running" :hint="t('admin.accounts.pelicanTest.modelHint')" />
            <div class="grid grid-cols-2 gap-4 lg:grid-cols-1">
              <div>
                <label id="iq-effort-label" class="input-label">{{ t('admin.accounts.pelicanTest.reasoning') }}</label>
                <Select v-model="reasoningEffort" aria-labelledby="iq-effort-label" :options="reasoningOptions" :disabled="running" />
              </div>
              <Input v-model="parallelCount" type="number" :label="t('admin.accounts.pelicanTest.parallel')" :disabled="running" :hint="t('admin.accounts.pelicanTest.parallelHint')" />
            </div>
            <div class="flex items-start gap-2 rounded-xl bg-primary-50/70 p-3 text-xs leading-relaxed text-primary-700 dark:bg-primary-500/10 dark:text-primary-300">
              <Icon name="shield" size="sm" class="mt-0.5 shrink-0" /><span>{{ deliveryContract }}</span>
            </div>
          </div>
        </section>

        <section class="space-y-3" :aria-label="t('admin.accounts.pelicanTest.resultTitle')" :aria-busy="running">
          <div class="flex flex-wrap items-center justify-between gap-2">
            <h4 class="text-sm font-semibold text-gray-900 dark:text-gray-100">{{ t('admin.accounts.pelicanTest.resultTitle') }}</h4>
            <span v-if="runs.length" role="status" class="flex items-center gap-2 text-xs tabular-nums text-gray-500 dark:text-dark-400">
              <Icon v-if="running" name="refresh" size="sm" class="animate-spin text-primary-500" />
              {{ t('admin.accounts.pelicanTest.progress', { completed: completedCount, total: runs.length }) }}
            </span>
          </div>
          <div v-if="runs.length === 0" class="flex flex-col items-center rounded-xl border border-dashed border-gray-200 bg-gray-50/50 px-5 py-8 text-center dark:border-dark-600 dark:bg-dark-900/20">
            <Icon name="chat" size="lg" class="mb-3 text-gray-400 dark:text-dark-500" />
            <p class="text-sm font-medium text-gray-700 dark:text-gray-200">{{ t('admin.accounts.pelicanTest.emptyTitle') }}</p>
            <p class="mt-1 text-xs leading-relaxed text-gray-500 dark:text-dark-400">{{ t('admin.accounts.pelicanTest.emptyResults') }}</p>
          </div>
          <div v-else class="grid min-w-0 grid-cols-1 gap-4" :class="{ 'xl:grid-cols-2': runs.length > 1 }">
            <article v-for="(run, index) in runs" :key="run.id" class="min-w-0 overflow-hidden rounded-xl border border-gray-200 dark:border-dark-700">
              <header class="flex items-center justify-between gap-2 border-b border-gray-100 px-4 py-3 dark:border-dark-700">
                <span class="text-sm font-semibold text-gray-900 dark:text-gray-100">{{ t('admin.accounts.pelicanTest.output') }} <span class="ml-1 tabular-nums text-gray-400">{{ String(index + 1).padStart(2, '0') }}</span></span>
                <div class="flex items-center gap-2">
                  <span class="rounded-md px-2 py-1 text-xs font-medium" :class="run.status === 'running' ? 'bg-primary-50 text-primary-700 dark:bg-primary-500/15 dark:text-primary-300' : run.status === 'success' ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-400' : 'bg-red-50 text-red-700 dark:bg-red-500/15 dark:text-red-400'">
                    {{ t('admin.accounts.pelicanTest.' + (run.status === 'running' ? 'runningShort' : run.status === 'success' ? 'success' : 'failed')) }}
                  </span>
                  <button v-if="run.output" type="button" class="rounded-lg p-2 text-gray-500 transition-colors hover:bg-gray-100 hover:text-primary-600 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 dark:text-dark-400 dark:hover:bg-dark-700" :aria-label="t('admin.accounts.pelicanTest.download') + ' ' + (index + 1)" :title="t('admin.accounts.pelicanTest.download')" @click="downloadHtml(run)"><Icon name="download" size="sm" /></button>
                </div>
              </header>
              <div class="grid gap-1 bg-gray-50/70 px-4 py-3 text-xs leading-relaxed text-gray-500 dark:bg-dark-900/40 dark:text-dark-400" data-testid="run-metadata">
                <div class="break-words font-medium text-gray-700 dark:text-gray-300">{{ run.modelId || '—' }} / {{ run.reasoningEffort || '—' }}</div>
                <div>{{ t(run.source === 'scheduled' ? 'admin.accounts.pelicanTest.sourceScheduled' : 'admin.accounts.pelicanTest.sourceManual') }} · {{ t('admin.accounts.pelicanTest.generatedAt') }} {{ run.startedAt ? formatDate(run.startedAt) : '—' }}</div>
                <div class="tabular-nums">{{ t('admin.accounts.pelicanTest.duration') }} {{ run.durationMs == null ? '—' : (run.durationMs / 1000).toFixed(1) + ' s' }}</div>
              </div>
              <div v-if="run.html" class="aspect-[16/10] bg-white"><iframe :srcdoc="run.html" class="h-full w-full border-0" sandbox="allow-scripts" referrerpolicy="no-referrer" :title="t('admin.accounts.pelicanTest.output') + ' ' + (index + 1)" /></div>
              <pre v-else class="max-h-72 min-h-28 overflow-auto whitespace-pre-wrap break-words p-5 leading-relaxed text-gray-900 dark:text-gray-100" :class="run.questionKind === 'candy' ? 'font-mono text-xl' : 'font-mono text-sm'">{{ run.output || (run.status === 'running' ? t('admin.accounts.pelicanTest.waiting') : '') }}</pre>
              <p v-if="run.error" role="alert" class="border-t border-red-100 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900/40 dark:bg-red-500/10 dark:text-red-300">{{ run.error }}</p>
              <details v-if="run.html" class="border-t border-gray-100 dark:border-dark-700">
                <summary class="cursor-pointer px-4 py-3 text-xs font-medium text-gray-500 hover:text-primary-600 dark:text-dark-400">{{ t('admin.accounts.pelicanTest.rawOutput') }}</summary>
                <pre class="max-h-60 overflow-auto whitespace-pre-wrap break-words bg-gray-950 p-4 font-mono text-xs leading-relaxed text-gray-200">{{ run.output }}</pre>
              </details>
            </article>
          </div>
        </section>
      </template>
    </div>
    <template #footer>
      <div class="flex w-full flex-col-reverse gap-3 sm:flex-row sm:items-center sm:justify-between">
        <button v-if="activeTab !== 'schedule'" type="button" class="btn btn-ghost" :disabled="running || !hasDownloadable" @click="downloadAll"><Icon name="download" size="sm" />{{ t('admin.accounts.pelicanTest.downloadAll') }}</button>
        <div v-else class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.accounts.pelicanTest.scheduleEvaluationHint') }}</div>
        <div class="flex gap-3">
          <button type="button" class="btn btn-secondary flex-1 sm:flex-none" :disabled="running" @click="handleClose">{{ t('common.close') }}</button>
          <button v-if="activeTab !== 'schedule'" type="button" class="btn btn-primary flex-1 sm:flex-none" :disabled="running || !canStart" @click="startTest">
            <Icon v-if="running" name="refresh" size="sm" class="animate-spin" /><Icon v-else name="play" size="sm" />{{ t(running ? 'admin.accounts.pelicanTest.generating' : 'admin.accounts.pelicanTest.start') }}
          </button>
        </div>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { questionPrompt, questionContract, type IntelligenceQuestion } from '@/utils/intelligenceTest'
import { useI18n } from 'vue-i18n'
import { extractPelicanHtml as extractHtml } from '@/utils/pelicanHtml'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Input from '@/components/common/Input.vue'
import TextArea from '@/components/common/TextArea.vue'
import Select from '@/components/common/Select.vue'
import { Icon } from '@/components/icons'
import { buildApiUrl } from '@/api/client'
import { ADMIN_UI_REQUEST_HEADER } from '@/api/adminUIRequest'
import type { Account, AccountListItem, PelicanTestConfig, ScheduledTestResult } from '@/types'
import ScheduledTestsPanel from './ScheduledTestsPanel.vue'
import PelicanRecordsDashboard from './PelicanRecordsDashboard.vue'

const { t } = useI18n()

const STORAGE_PREFIX = 'sub2api-pelican-test:'

type RunStatus = 'running' | 'success' | 'error'
interface TestRun {
  questionKind?: IntelligenceQuestion
  id: string
  status: RunStatus
  output: string
  html: string
  error: string
  source?: 'manual' | 'scheduled'
  startedAt?: string
  finishedAt?: string
  durationMs?: number
  modelId?: string
  reasoningEffort?: string
}
interface TestRecord {
  questionKind?: IntelligenceQuestion
  id: string
  createdAt: string
  prompt: string
  modelId: string
  reasoningEffort: string
  runs: TestRun[]
}

const props = defineProps<{ show: boolean; account: Account | null; accounts?: AccountListItem[] }>()
const emit = defineEmits<{ (event: 'close'): void }>()

const questionKind = ref<IntelligenceQuestion>('candy')
const prompt = ref(questionPrompt('candy'))
const modelId = ref('gpt-6-astra')
const reasoningEffort = ref('medium')
const parallelCount = ref<string | number>(1)
const activeTab = ref<'results' | 'schedule'>('results')
const running = ref(false)
const viewingScheduled = ref(false)
const dashboardOpen = ref(false)
const runs = ref<TestRun[]>([])
const records = ref<TestRecord[]>([])
const controllers = new Map<string, AbortController>()
let generation = 0

const completedCount = computed(() => runs.value.filter(run => run.status !== 'running').length)

const deliveryContract = computed(() => questionContract(questionKind.value))
const questionOptions = computed(() => ['candy', 'pelican'].map(value => ({ value, label: t(`admin.accounts.pelicanTest.${value}Question`) })))
function selectQuestion(value: string | number | boolean | null) {
  if (running.value || (value !== 'candy' && value !== 'pelican')) return
  questionKind.value = value
  prompt.value = questionPrompt(value)
}
const reasoningOptions = computed(() => [
  { value: 'low', label: t('admin.accounts.pelicanTest.reasoningLow') },
  { value: 'medium', label: t('admin.accounts.pelicanTest.reasoningMedium') },
  { value: 'high', label: t('admin.accounts.pelicanTest.reasoningHigh') }
])
const canStart = computed(() => Boolean(props.account && prompt.value.trim() && modelId.value.trim() && normalizeCount() > 0))
const hasDownloadable = computed(() => runs.value.some((run) => Boolean(run.output)))

const storageKey = computed(() => `${STORAGE_PREFIX}${props.account?.id ?? 'unknown'}`)

function normalizeCount(): number {
  const value = Number(parallelCount.value)
  if (!Number.isFinite(value)) return 1
  return Math.min(8, Math.max(1, Math.floor(value)))
}

function readRecords() {
  try {
    const parsed = JSON.parse(localStorage.getItem(storageKey.value) || '[]')
    records.value = Array.isArray(parsed) ? parsed : []
  } catch {
    records.value = []
  }
}

function saveRecords() {
  try {
    localStorage.setItem(storageKey.value, JSON.stringify(records.value.slice(0, 8)))
  } catch {
    // A large model response must not prevent the current result from being shown.
  }
}

function formatDate(value: string) {
  if (!Number.isFinite(Date.parse(value))) return '—'
  return new Intl.DateTimeFormat(undefined, { dateStyle: 'short', timeStyle: 'medium' }).format(new Date(value))
}


function editSchedule(config: PelicanTestConfig, model: string) {
  if (running.value) return
  questionKind.value = config.question_kind || 'pelican'
  prompt.value = config.prompt
  modelId.value = model
  reasoningEffort.value = config.reasoning_effort
  parallelCount.value = config.parallel_count
}

function openManualResults() {
  if (running.value) return
  if (viewingScheduled.value) runs.value = []
  viewingScheduled.value = false
  activeTab.value = 'results'
}

function previewScheduled(result: ScheduledTestResult) {
  if (running.value) return
  const config = result.pelican_config
  if (config) editSchedule(config, config.model_id || modelId.value)
  const html = config?.question_kind === 'candy' ? '' : extractHtml(result.response_text)
  runs.value = [{ id: `scheduled-${result.id}`, questionKind: config?.question_kind || 'pelican', status: result.status === 'success' ? 'success' : 'error', output: result.response_text, html, error: result.error_message,
    source: 'scheduled', startedAt: result.started_at, finishedAt: result.finished_at,
    durationMs: result.latency_ms, modelId: config?.model_id, reasoningEffort: config?.reasoning_effort
  }]
  viewingScheduled.value = true
  activeTab.value = 'results'
}

function cancelRuns() {
  generation++
  for (const controller of controllers.values()) controller.abort()
  controllers.clear()
  running.value = false
}

function handleClose() {
  cancelRuns()
  dashboardOpen.value = false
  emit('close')
}

async function consumeRun(run: TestRun, signal: AbortSignal) {
  const response = await fetch(buildApiUrl(`/admin/accounts/${props.account!.id}/pelican-test`), {
    method: 'POST',
    headers: {
      Authorization: `Bearer ${localStorage.getItem('auth_token')}`,
      'Content-Type': 'application/json',
      [ADMIN_UI_REQUEST_HEADER]: '1'
    },
    body: JSON.stringify({
      model_id: modelId.value.trim(),
      prompt: `${prompt.value.trim()}\n\n${deliveryContract.value}`,
      mode: 'default',
      reasoning_effort: reasoningEffort.value
    }),
    signal
  })
  if (!response.ok) throw new Error(`HTTP ${response.status}`)
  const reader = response.body?.getReader()
  if (!reader) throw new Error(t('admin.accounts.pelicanTest.noResponseBody'))
  const decoder = new TextDecoder()
  let buffer = ''
  let completed = false
  const consumeLine = (line: string) => {
    if (!line.startsWith('data:')) return
    const json = line.replace(/^data:\s*/, '').trim()
    if (!json) return
    let event: { type?: string; text?: string; success?: boolean; error?: string }
    try {
      event = JSON.parse(json) as { type?: string; text?: string; success?: boolean; error?: string }
    } catch {
      return
    }
    if (event.type === 'content' && event.text) run.output += event.text
    if (event.type === 'test_complete') {
      completed = true
      if (!event.success) throw new Error(event.error || t('admin.accounts.pelicanTest.failed'))
    }
    if (event.type === 'error') throw new Error(event.error || t('admin.accounts.pelicanTest.failed'))
  }
  while (true) {
    const { done, value } = await reader.read()
    if (done) break
    buffer += decoder.decode(value, { stream: true })
    const lines = buffer.split('\n')
    buffer = lines.pop() || ''
    for (const line of lines) consumeLine(line.trim())
  }
  if (buffer.trim()) consumeLine(buffer.trim())
  if (!completed || !run.output.trim()) throw new Error(t('admin.accounts.pelicanTest.emptyResponse'))
  run.html = run.questionKind === 'candy' ? '' : extractHtml(run.output)
  if (run.questionKind !== 'candy' && !run.html) throw new Error(t('admin.accounts.pelicanTest.invalidHtml'))
  run.status = 'success'
}

async function startOne(run: TestRun) {
  const started = performance.now()
  run.startedAt = new Date().toISOString()
  const controller = new AbortController()
  controllers.set(run.id, controller)
  try {
    await consumeRun(run, controller.signal)
  } catch (error) {
    if (error instanceof DOMException && error.name === 'AbortError') return
    run.status = 'error'
    run.error = error instanceof Error ? error.message : t('admin.accounts.pelicanTest.failed')
  } finally {
    run.durationMs = Math.max(0, Math.round(performance.now() - started))
    run.finishedAt = new Date().toISOString()
    controllers.delete(run.id)
  }
}

async function startTest() {
  if (running.value || !props.account || !canStart.value) return
  viewingScheduled.value = false
  const currentGeneration = ++generation
  const count = normalizeCount()
  parallelCount.value = count
  runs.value = Array.from({ length: count }, (_, index) => ({
    id: `${Date.now()}-${index}`,
    questionKind: questionKind.value,
    status: 'running',
    output: '',
    html: '',
    error: '',
    source: 'manual',
    modelId: modelId.value.trim(),
    reasoningEffort: reasoningEffort.value
  }))
  activeTab.value = 'results'
  running.value = true
  await Promise.all(runs.value.map((run) => startOne(run)))
  // A closed or switched account must never receive results from an older run.
  if (currentGeneration !== generation) return
  running.value = false
  const record: TestRecord = {
    id: `${Date.now()}`,
    createdAt: new Date().toISOString(),
    questionKind: questionKind.value,
    prompt: prompt.value.trim(),
    modelId: modelId.value.trim(),
    reasoningEffort: reasoningEffort.value,
    runs: runs.value.map((run) => ({ ...run }))
  }
  records.value = [record, ...records.value.filter((item) => item.id !== record.id)]
  saveRecords()
}

function downloadHtml(run: TestRun) {
  const content = run.questionKind === 'candy' ? run.output : run.html || extractHtml(run.output)
  if (!content) return
  const url = URL.createObjectURL(new Blob([content], { type: run.questionKind === 'candy' ? 'text/plain;charset=utf-8' : 'text/html;charset=utf-8' }))
  const link = document.createElement('a')
  link.href = url
  link.download = `intelligence-test-${new Date().toISOString().replace(/[:.]/g, '-')}.${run.questionKind === 'candy' ? 'txt' : 'html'}`
  link.click()
  URL.revokeObjectURL(url)
}

function downloadAll() {
  runs.value.filter((run) => run.output).forEach((run) => downloadHtml(run))
}

onBeforeUnmount(cancelRuns)

watch(() => [props.show, props.account?.id] as const, ([show]) => {
  cancelRuns()
  dashboardOpen.value = false
  if (show) {
    readRecords()
    activeTab.value = 'results'
    viewingScheduled.value = false
    questionKind.value = 'candy'
    prompt.value = questionPrompt('candy')
    modelId.value = 'gpt-6-astra'
    reasoningEffort.value = 'medium'
    parallelCount.value = 1
    runs.value = []
  }
}, { immediate: true })
</script>
