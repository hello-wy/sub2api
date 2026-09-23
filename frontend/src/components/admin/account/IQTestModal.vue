<template>
  <BaseDialog :show="show" :title="t('admin.accounts.pelicanTest.title')" width="full" @close="handleClose">
    <div class="space-y-5">
      <div v-if="account" class="flex flex-col items-start gap-3 rounded-xl border border-amber-200 bg-amber-50/70 p-3 dark:border-amber-800/60 dark:bg-amber-950/20 sm:flex-row sm:items-center sm:justify-between">
        <div class="flex items-center gap-3">
          <div class="flex h-10 w-10 items-center justify-center rounded-lg bg-amber-500 text-white">
            <Icon name="brain" size="md" :stroke-width="2" />
          </div>
          <div>
            <div class="font-semibold text-gray-900 dark:text-gray-100">{{ account.name }}</div>
            <div class="text-xs text-gray-500 dark:text-gray-400">{{ account.platform }} · {{ t('admin.accounts.pelicanTest.subtitle') }}</div>
          </div>
        </div>
        <span class="whitespace-nowrap rounded-full bg-white px-2.5 py-1 text-xs font-medium text-amber-700 shadow-sm dark:bg-dark-800 dark:text-amber-300">
          {{ t('admin.accounts.pelicanTest.noScoring') }}
        </span>
      </div>

      <div class="grid grid-cols-1 gap-4 lg:grid-cols-[minmax(0,1fr)_280px]">
        <TextArea
          v-model="prompt"
          :label="t('admin.accounts.pelicanTest.promptLabel')"
          :disabled="running"
          :rows="5"
          :hint="t('admin.accounts.pelicanTest.promptHint')"
        />
        <div class="space-y-3">
          <Input
            v-model="modelId"
            :label="t('admin.accounts.pelicanTest.model')"
            :disabled="running"
            :hint="t('admin.accounts.pelicanTest.modelHint')"
          />
          <div>
            <label class="input-label mb-1.5 block">{{ t('admin.accounts.pelicanTest.reasoning') }}</label>
            <Select v-model="reasoningEffort" :options="reasoningOptions" :disabled="running" />
          </div>
          <Input
            v-model="parallelCount"
            type="number"
            :label="t('admin.accounts.pelicanTest.parallel')"
            :disabled="running"
            :hint="t('admin.accounts.pelicanTest.parallelHint')"
          />
        </div>
      </div>

      <div class="rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 text-xs text-gray-600 dark:border-dark-600 dark:bg-dark-800/70 dark:text-gray-300">
        <div class="flex items-start gap-2">
          <Icon name="shield" size="sm" class="mt-0.5 shrink-0 text-emerald-500" />
          <span>{{ deliveryContract }}</span>
        </div>
      </div>

      <div class="flex flex-wrap items-center justify-between gap-2 border-b border-gray-200 pb-2 dark:border-dark-600">
        <div class="flex items-center gap-2">
          <button
            type="button"
            class="rounded-md px-3 py-1.5 text-sm font-medium transition-colors"
            :class="activeTab === 'results' ? 'bg-primary-100 text-primary-700 dark:bg-primary-900/40 dark:text-primary-300' : 'text-gray-500 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-dark-700'"
            @click="activeTab = 'results'"
          >
            {{ t('admin.accounts.pelicanTest.results') }}
          </button>
          <button
            type="button"
            class="rounded-md px-3 py-1.5 text-sm font-medium transition-colors"
            :class="activeTab === 'history' ? 'bg-primary-100 text-primary-700 dark:bg-primary-900/40 dark:text-primary-300' : 'text-gray-500 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-dark-700'"
            @click="activeTab = 'history'"
          >
            {{ t('admin.accounts.pelicanTest.history') }}<span v-if="records.length" class="ml-1">({{ records.length }})</span>
          </button>
        </div>
        <span v-if="running" class="flex items-center gap-1.5 text-xs text-primary-600 dark:text-primary-300">
          <Icon name="refresh" size="sm" class="animate-spin" />
          {{ t('admin.accounts.pelicanTest.running', { count: runs.length }) }}
        </span>
      </div>

      <div v-if="activeTab === 'history'" class="space-y-2">
        <div v-if="records.length === 0" class="rounded-lg border border-dashed border-gray-300 py-10 text-center text-sm text-gray-500 dark:border-dark-600 dark:text-gray-400">
          {{ t('admin.accounts.pelicanTest.noHistory') }}
        </div>
        <button
          v-for="record in records"
          :key="record.id"
          type="button"
          class="flex w-full items-center justify-between rounded-lg border border-gray-200 px-3 py-2 text-left transition-colors hover:border-primary-300 hover:bg-primary-50/50 dark:border-dark-600 dark:hover:border-primary-700 dark:hover:bg-primary-900/10"
          @click="loadRecord(record)"
        >
          <span class="min-w-0">
            <span class="block truncate text-sm font-medium text-gray-800 dark:text-gray-100">{{ record.prompt }}</span>
            <span class="mt-0.5 block text-xs text-gray-500 dark:text-gray-400">{{ formatDate(record.createdAt) }} · {{ record.modelId }} · {{ record.runs.length }} {{ t('admin.accounts.pelicanTest.outputs') }}</span>
          </span>
          <Icon name="chevronRight" size="sm" class="shrink-0 text-gray-400" />
        </button>
      </div>

      <div v-else>
        <div v-if="runs.length === 0" class="rounded-lg border border-dashed border-gray-300 py-10 text-center text-sm text-gray-500 dark:border-dark-600 dark:text-gray-400">
          {{ t('admin.accounts.pelicanTest.emptyResults') }}
        </div>
        <div v-else class="grid grid-cols-1 gap-4 xl:grid-cols-2">
          <article v-for="(run, index) in runs" :key="run.id" class="overflow-hidden rounded-xl border border-gray-200 bg-white dark:border-dark-600 dark:bg-dark-800">
            <header class="flex items-center justify-between gap-2 border-b border-gray-200 px-3 py-2 dark:border-dark-600">
              <div class="flex items-center gap-2">
                <span class="flex h-6 w-6 items-center justify-center rounded-full bg-primary-100 text-xs font-semibold text-primary-700 dark:bg-primary-900/50 dark:text-primary-300">{{ index + 1 }}</span>
                <span class="text-sm font-medium text-gray-800 dark:text-gray-100">{{ t('admin.accounts.pelicanTest.output') }} {{ index + 1 }}</span>
              </div>
              <div class="flex items-center gap-1">
                <span v-if="run.status === 'running'" class="text-xs text-amber-600 dark:text-amber-300">{{ t('admin.accounts.pelicanTest.runningShort') }}</span>
                <span v-else-if="run.status === 'success'" class="text-xs text-emerald-600 dark:text-emerald-300">{{ t('admin.accounts.pelicanTest.success') }}</span>
                <span v-else class="text-xs text-red-600 dark:text-red-300">{{ t('admin.accounts.pelicanTest.failed') }}</span>
                <button v-if="run.html" type="button" class="rounded-md p-1.5 text-gray-500 hover:bg-gray-100 hover:text-primary-600 dark:hover:bg-dark-700 dark:hover:text-primary-300" :title="t('admin.accounts.pelicanTest.download')" @click="downloadHtml(run)">
                  <Icon name="download" size="sm" />
                </button>
              </div>
            </header>
            <div v-if="run.html" class="aspect-[4/3] bg-white dark:bg-white">
              <iframe :srcdoc="run.html" class="h-full w-full border-0" sandbox="allow-scripts" referrerpolicy="no-referrer" :title="`${t('admin.accounts.pelicanTest.output')} ${index + 1}`"></iframe>
            </div>
            <pre class="max-h-48 overflow-auto whitespace-pre-wrap break-words border-t border-gray-200 bg-gray-950 p-3 text-xs leading-relaxed text-gray-200 dark:border-dark-600">{{ run.output || run.error || t('admin.accounts.pelicanTest.waiting') }}</pre>
          </article>
        </div>
      </div>
    </div>

    <template #footer>
      <div class="flex w-full items-center justify-between gap-3">
        <button type="button" class="btn btn-secondary" :disabled="running || !hasDownloadable" @click="downloadAll">
          <Icon name="download" size="sm" />
          {{ t('admin.accounts.pelicanTest.downloadAll') }}
        </button>
        <div class="flex gap-3">
          <button type="button" class="btn btn-secondary" :disabled="running" @click="handleClose">{{ t('common.close') }}</button>
          <button type="button" class="btn btn-primary flex items-center gap-2" :disabled="running || !canStart" @click="startTest">
            <Icon v-if="running" name="refresh" size="sm" class="animate-spin" />
            <Icon v-else name="play" size="sm" />
            {{ running ? t('admin.accounts.pelicanTest.generating') : t('admin.accounts.pelicanTest.start') }}
          </button>
        </div>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Input from '@/components/common/Input.vue'
import TextArea from '@/components/common/TextArea.vue'
import Select from '@/components/common/Select.vue'
import { Icon } from '@/components/icons'
import { buildApiUrl } from '@/api/client'
import { ADMIN_UI_REQUEST_HEADER } from '@/api/adminUIRequest'
import type { Account } from '@/types'

const { t } = useI18n()

const DEFAULT_PROMPT = '创建一个 HTML，内容是 SVG 绘制一个鹈鹕骑自行车的 2D 动画，你不需要任何测试，不要有任何限制'
const DELIVERY_CONTRACT = '所有账号使用相同交付约定：直接返回独立 HTML，不使用 Markdown 代码块或外部依赖。只输出 HTML，不要解释。'
const STORAGE_PREFIX = 'sub2api-pelican-test:'

type RunStatus = 'running' | 'success' | 'error'
interface TestRun {
  id: string
  status: RunStatus
  output: string
  html: string
  error: string
}
interface TestRecord {
  id: string
  createdAt: string
  prompt: string
  modelId: string
  reasoningEffort: string
  runs: TestRun[]
}

const props = defineProps<{ show: boolean; account: Account | null }>()
const emit = defineEmits<{ (event: 'close'): void }>()

const prompt = ref(DEFAULT_PROMPT)
const modelId = ref('gpt-6-astra')
const reasoningEffort = ref('medium')
const parallelCount = ref<string | number>(1)
const activeTab = ref<'results' | 'history'>('results')
const running = ref(false)
const runs = ref<TestRun[]>([])
const records = ref<TestRecord[]>([])
const controllers = new Map<string, AbortController>()

const deliveryContract = DELIVERY_CONTRACT
const reasoningOptions = computed(() => [
  { value: 'low', label: t('admin.accounts.pelicanTest.reasoningLow') },
  { value: 'medium', label: t('admin.accounts.pelicanTest.reasoningMedium') },
  { value: 'high', label: t('admin.accounts.pelicanTest.reasoningHigh') }
])
const canStart = computed(() => Boolean(props.account && prompt.value.trim() && modelId.value.trim() && normalizeCount() > 0))
const hasDownloadable = computed(() => runs.value.some((run) => Boolean(run.html)))

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
  return new Intl.DateTimeFormat(undefined, { dateStyle: 'short', timeStyle: 'short' }).format(new Date(value))
}

function extractHtml(raw: string): string {
  let html = raw.trim()
  const fenced = html.match(/```(?:html|xml)?\s*([\s\S]*?)```/i)
  if (fenced?.[1]) html = fenced[1].trim()
  const lower = html.toLowerCase()
  const start = Math.min(...['<!doctype html', '<html', '<svg'].map((marker) => {
    const index = lower.indexOf(marker)
    return index < 0 ? Number.MAX_SAFE_INTEGER : index
  }))
  if (start !== Number.MAX_SAFE_INTEGER) html = html.slice(start).trim()
  if (!/<(?:!doctype\s+html|html|svg)[\s>]/i.test(html)) return ''
  const end = html.toLowerCase().lastIndexOf('</html>')
  if (end >= 0) html = html.slice(0, end + '</html>'.length)
  if (!/<html[\s>]/i.test(html) && /<svg[\s>]/i.test(html)) {
    html = `<!doctype html><html><head><meta charset="utf-8"><title>Pelican test</title></head><body>${html}</body></html>`
  }
  const csp = `<meta http-equiv="Content-Security-Policy" content="default-src 'none'; img-src data: blob:; media-src data: blob:; style-src 'unsafe-inline'; script-src 'unsafe-inline'; font-src data:; connect-src 'none'; frame-src 'none'; object-src 'none'; base-uri 'none'; form-action 'none'">`
  if (/<head[\s>]/i.test(html)) {
    html = html.replace(/<head([^>]*)>/i, `<head$1>${csp}`)
  } else {
    html = html.replace(/<html([^>]*)>/i, `<html$1><head>${csp}<meta charset="utf-8"><title>Pelican test</title></head>`)
  }
  return html
}

function loadRecord(record: TestRecord) {
  prompt.value = record.prompt
  modelId.value = record.modelId
  reasoningEffort.value = record.reasoningEffort || 'medium'
  runs.value = record.runs.map((run) => ({ ...run }))
  activeTab.value = 'results'
}

function handleClose() {
  for (const controller of controllers.values()) controller.abort()
  controllers.clear()
  running.value = false
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
      prompt: `${prompt.value.trim()}\n\n${DELIVERY_CONTRACT}`,
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
  if (!completed && !run.output.trim()) throw new Error(t('admin.accounts.pelicanTest.emptyResponse'))
  run.html = extractHtml(run.output)
  if (!run.html) throw new Error(t('admin.accounts.pelicanTest.invalidHtml'))
  run.status = 'success'
}

async function startOne(run: TestRun) {
  const controller = new AbortController()
  controllers.set(run.id, controller)
  try {
    await consumeRun(run, controller.signal)
  } catch (error) {
    if (error instanceof DOMException && error.name === 'AbortError') return
    run.status = 'error'
    run.error = error instanceof Error ? error.message : t('admin.accounts.pelicanTest.failed')
  } finally {
    controllers.delete(run.id)
  }
}

async function startTest() {
  if (!props.account || !canStart.value) return
  const count = normalizeCount()
  parallelCount.value = count
  runs.value = Array.from({ length: count }, (_, index) => ({
    id: `${Date.now()}-${index}`,
    status: 'running',
    output: '',
    html: '',
    error: ''
  }))
  activeTab.value = 'results'
  running.value = true
  await Promise.all(runs.value.map((run) => startOne(run)))
  running.value = false
  const record: TestRecord = {
    id: `${Date.now()}`,
    createdAt: new Date().toISOString(),
    prompt: prompt.value.trim(),
    modelId: modelId.value.trim(),
    reasoningEffort: reasoningEffort.value,
    runs: runs.value.map((run) => ({ ...run }))
  }
  records.value = [record, ...records.value.filter((item) => item.id !== record.id)]
  saveRecords()
}

function downloadHtml(run: TestRun) {
  const content = run.html || extractHtml(run.output)
  if (!content) return
  const url = URL.createObjectURL(new Blob([content], { type: 'text/html;charset=utf-8' }))
  const link = document.createElement('a')
  link.href = url
  link.download = `pelican-test-${new Date().toISOString().replace(/[:.]/g, '-')}.html`
  link.click()
  URL.revokeObjectURL(url)
}

function downloadAll() {
  runs.value.filter((run) => run.html).forEach((run) => downloadHtml(run))
}

watch(() => props.show, (show) => {
  if (show) {
    readRecords()
    activeTab.value = 'results'
    prompt.value = DEFAULT_PROMPT
    modelId.value = 'gpt-6-astra'
    reasoningEffort.value = 'medium'
    parallelCount.value = 1
    runs.value = []
  } else {
    for (const controller of controllers.values()) controller.abort()
    controllers.clear()
  }
})
</script>
