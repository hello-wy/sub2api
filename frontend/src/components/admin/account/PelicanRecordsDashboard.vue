<template>
  <div class="fixed inset-0 z-[60] overflow-y-auto bg-[#f6f8f7] p-4 dark:bg-dark-950 sm:p-6" role="dialog" aria-modal="true" :aria-label="t('admin.accounts.pelicanTest.history')" @keydown.esc.stop="selected = null">
    <div class="mx-auto max-w-[1800px]">
      <header class="mb-4 flex items-center justify-between gap-4">
        <div>
          <h2 class="text-xl font-semibold">{{ t('admin.accounts.pelicanTest.history') }}</h2>
          <p class="text-sm text-gray-500">{{ t('admin.accounts.pelicanTest.dashboardHint') }}</p>
        </div>
        <button type="button" class="btn btn-secondary" @click="$emit('close')">{{ t('common.close') }}</button>
      </header>
      <p v-if="loadError" role="alert" class="mb-4 text-sm text-red-600">{{ loadError }}</p>
      <div v-if="loading" class="py-20 text-center text-sm text-gray-500">{{ t('common.loading') }}...</div>
      <div v-else-if="cards.length" class="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-4">
        <article v-for="card in cards" :key="card.account.id" class="relative overflow-hidden rounded-2xl border border-gray-200 bg-white shadow-sm transition hover:border-primary-300 hover:shadow-md dark:border-dark-700 dark:bg-dark-800" data-testid="pelican-record-card">
          <div class="p-4">
            <h3 class="font-semibold">{{ card.account.name }}</h3>
            <p class="mt-1 text-xs text-gray-500">#{{ card.account.id }}</p>
            <div class="mt-3 flex justify-between gap-2 text-xs">
              <span :class="card.record.status === 'success' ? 'text-emerald-600' : 'text-red-500'">{{ t(card.record.status === 'success' ? 'admin.accounts.pelicanTest.success' : 'admin.accounts.pelicanTest.failed') }}</span>
              <span class="text-gray-500">{{ duration(card.record.durationMs) }} · {{ sourceLabel(card.record.source) }}</span>
            </div>
            <div class="mt-2 space-y-1 text-xs text-gray-500">
              <p>{{ card.record.modelId || '—' }} / {{ card.record.reasoningEffort || '—' }}</p>
              <p>{{ t('admin.accounts.pelicanTest.generatedAt') }}：{{ formatDate(card.record.startedAt) }}</p>
            </div>
            <div class="mt-3 aspect-[4/3] overflow-hidden rounded-xl bg-gray-50">
              <iframe v-if="card.record.html" :srcdoc="card.record.html" class="pointer-events-none h-full w-full border-0" tabindex="-1" sandbox="allow-scripts" referrerpolicy="no-referrer" :title="card.account.name" />
              <p v-else class="p-4 text-sm text-red-500">{{ card.record.error || t('admin.accounts.pelicanTest.invalidHtml') }}</p>
            </div>
          </div>
          <div class="border-t border-gray-100 px-4 py-3 text-sm text-primary-600 dark:border-dark-700">{{ t('admin.accounts.pelicanTest.preview') }}</div>
          <button type="button" class="absolute inset-0 z-10 rounded-2xl focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary-500" :aria-label="`${card.account.name} · ${t('admin.accounts.pelicanTest.preview')}`" @click="selected = card" />
        </article>
      </div>
      <div v-else class="rounded-xl border border-dashed border-gray-300 bg-white py-20 text-center text-sm text-gray-500 dark:bg-dark-800">{{ t('admin.accounts.pelicanTest.noHistory') }}</div>
    </div>
    <div v-if="selected" class="fixed inset-0 z-[70] flex items-center justify-center bg-black/70 p-4" data-testid="record-detail" @click.self="selected = null">
      <div class="flex h-full w-full max-w-6xl flex-col overflow-hidden rounded-xl bg-white dark:bg-dark-800">
        <header class="flex items-center justify-between gap-3 border-b px-4 py-3">
          <div class="text-sm">
            <strong>{{ selected.account.name }}</strong>
            <p>{{ sourceLabel(selected.record.source) }} · {{ selected.record.modelId || '—' }} / {{ selected.record.reasoningEffort || '—' }} · {{ duration(selected.record.durationMs) }}</p>
            <p class="text-xs text-gray-500">{{ formatDate(selected.record.startedAt) }}</p>
          </div>
          <button type="button" class="btn btn-secondary" @click="selected = null">{{ t('common.close') }}</button>
        </header>
        <iframe v-if="selected.record.html" :srcdoc="selected.record.html" class="min-h-0 w-full flex-1 border-0" sandbox="allow-scripts" referrerpolicy="no-referrer" :title="selected.account.name" />
        <pre v-else class="overflow-auto whitespace-pre-wrap p-4 text-sm">{{ selected.record.error || selected.record.output }}</pre>
      </div>
    </div>
  </div>
</template>
<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { scheduledTestsAPI } from '@/api/admin/scheduledTests'
import { extractPelicanHtml } from '@/utils/pelicanHtml'
import type { Account, AccountListItem, ScheduledTestResult } from '@/types'

interface ManualRun {
  html?: string
  output?: string
  error?: string
  status?: string
  durationMs?: number
  startedAt?: string
  modelId?: string
  reasoningEffort?: string
}
interface ManualRecord {
  createdAt: string
  modelId: string
  reasoningEffort: string
  runs: ManualRun[]
}
interface DisplayRecord {
  source: 'manual' | 'scheduled'
  startedAt?: string
  durationMs?: number
  modelId?: string
  reasoningEffort?: string
  status: string
  html: string
  output: string
  error: string
}
interface Card {
  account: Pick<AccountListItem, 'id' | 'name'>
  record: DisplayRecord
}
const props = defineProps<{ accounts: AccountListItem[]; account: Account | null; manualRecord: ManualRecord | null }>()
defineEmits<{ close: [] }>()
const { t } = useI18n()
const loading = ref(true)
const selected = ref<Card | null>(null)
const cards = ref<Card[]>([])
const loadError = ref('')
let alive = true
function sourceLabel(source: DisplayRecord['source']) { return t(`admin.accounts.pelicanTest.${source === 'manual' ? 'sourceManual' : 'sourceScheduled'}`) }
function formatDate(value?: string) {
  if (!value || !Number.isFinite(Date.parse(value))) return '—'
  return new Intl.DateTimeFormat(undefined, { dateStyle: 'short', timeStyle: 'medium' }).format(new Date(value))
}
function duration(value?: number) { return typeof value === 'number' && Number.isFinite(value) ? `${(value / 1000).toFixed(1)} s` : '—' }
function timestamp(record: DisplayRecord) { return record.startedAt ? Date.parse(record.startedAt) || 0 : 0 }
function manualResults(accountId: number): DisplayRecord[] {
  let records: ManualRecord[] = []
  try {
    const saved = JSON.parse(localStorage.getItem(`sub2api-pelican-test:${accountId}`) || '[]')
    if (Array.isArray(saved)) records = saved
  } catch { /* A corrupt browser record must not hide server results. */ }
  if (accountId === props.account?.id && props.manualRecord) records.unshift(props.manualRecord)
  return records.flatMap(record => Array.isArray(record?.runs) ? record.runs.filter(run => run && typeof run === 'object').map(run => {
    const output = run.output || run.html || ''
    const html = extractPelicanHtml(output)
    return { source: 'manual' as const, startedAt: run.startedAt || record.createdAt, durationMs: run.durationMs,
      modelId: run.modelId || record.modelId, reasoningEffort: run.reasoningEffort || record.reasoningEffort,
      status: run.status || (html ? 'success' : 'error'), html, output, error: run.error || '' }
  }) : [])
}
function serverResult(result: ScheduledTestResult): DisplayRecord {
  return { source: 'scheduled', startedAt: result.started_at, durationMs: result.latency_ms,
    modelId: result.pelican_config?.model_id, reasoningEffort: result.pelican_config?.reasoning_effort,
    status: result.status, output: result.response_text, html: extractPelicanHtml(result.response_text), error: result.error_message }
}
onMounted(async () => {
  const accounts = new Map(props.accounts.map(account => [account.id, account]))
  if (props.account) accounts.set(props.account.id, props.account as unknown as AccountListItem)
  const all = [...accounts.values()]
  const loaded: Card[] = []
  let cursor = 0
  // Bound requests so a large account page does not flood the admin API.
  await Promise.all(Array.from({ length: Math.min(4, all.length) }, async () => {
    while (alive && cursor < all.length) {
      const account = all[cursor++]
      const records = manualResults(account.id)
      try {
        const plans = (await scheduledTestsAPI.listByAccount(account.id)).filter(plan => plan.pelican_config)
        for (const plan of plans) {
          if (!alive) return
          const results = await scheduledTestsAPI.listResults(plan.id, 1, false)
          const result = results[0]
          if (result && (!records.length || Date.parse(result.started_at) > Math.max(...records.map(timestamp)))) {
            records.push(serverResult(await scheduledTestsAPI.getResult(plan.id, result.id)))
          }
        }
      } catch {
        if (alive) loadError.value = t('admin.accounts.pelicanTest.historyLoadError')
      }
      const latest = records.sort((a, b) => timestamp(b) - timestamp(a))[0]
      if (latest) loaded.push({ account, record: latest })
    }
  }))
  if (alive) { cards.value = loaded.sort((a, b) => timestamp(b.record) - timestamp(a.record)); loading.value = false }
})
onBeforeUnmount(() => { alive = false })
</script>
