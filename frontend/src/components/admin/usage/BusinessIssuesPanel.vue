<template>
  <section class="space-y-4" aria-label="经营账待处理问题">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div><h4 class="font-semibold">待处理问题</h4><p class="mt-1 text-sm text-gray-500">同一原因合并处理，调用明细仅用于追溯。资金来源含历史期初，其余按所选日期归集。</p></div>
      <div class="flex flex-wrap gap-2"><button class="btn btn-secondary" :disabled="loading" @click="refresh()">刷新状态</button><button class="btn btn-primary" @click="repairIssue = undefined; repairOpen = true">补算历史缺失成本</button></div>
    </div>
    <div v-if="summary" class="flex flex-wrap gap-4 text-sm" role="status">
      <span><strong>{{ summary.total }}</strong> 项待处理问题</span>
      <span v-if="summary.processing_count" class="text-primary-600">后台正在计算 {{ summary.processing_count }} 条记录，无需逐条处理</span>
      <span v-if="summary.calculation_error" class="text-amber-700">后台计算未完成，系统会自动重试</span>
    </div>
    <p v-if="error" role="alert" class="text-sm text-red-600">{{ error }}</p>
    <p v-if="loading && !summary" class="text-sm text-gray-500">正在归集问题…</p>
    <div v-if="selectedAccounts.length" class="flex items-center gap-3 text-sm"><span>已选 {{ selectedAccounts.length }} 个缺配置账号</span><button class="btn btn-secondary" @click="emit('configure', { mode: 'binding', accountIDs: selectedAccounts, accountNames: selectedNames, effectiveAt: selectedEffective })">批量绑定成本池</button></div>
    <div class="overflow-x-auto rounded-lg border border-gray-200 dark:border-dark-700">
      <table class="w-full text-left text-sm"><thead class="bg-gray-50 text-gray-500 dark:bg-dark-800"><tr><th class="p-3">问题 / 对象</th><th class="p-3">影响范围</th><th class="p-3">处理</th></tr></thead>
        <tbody><tr v-for="issue in summary?.items || []" :key="issue.key" class="border-t border-gray-100 dark:border-dark-700">
          <td class="p-3"><label class="flex items-center gap-2"><input v-if="issue.kind === 'cost_binding' && issue.account_id" v-model="selected" type="checkbox" :value="issue.account_id" :aria-label="'选择' + issue.name" /><strong>{{ labels[issue.kind] || '其他成本缺口' }}</strong></label><p class="mt-1 text-gray-500">{{ issue.name }}<span v-if="issue.model"> · {{ issue.model }}</span></p></td>
          <td class="p-3"><p>{{ issue.period || '资金来源' }}</p><p v-if="issue.affected_count">影响 {{ issue.affected_count.toLocaleString('zh-CN') }} 条记录</p><p v-if="issue.source_count">{{ issue.source_count }} 笔原始来源待确认</p><p v-if="['invoice_needed', 'invoice_changed'].includes(issue.kind)" class="text-gray-500">已记成本 {{ cny(issue.known_amount_cny) }}</p></td>
          <td class="p-3"><div class="flex flex-wrap gap-3"><button class="text-primary-600 underline" @click="handle(issue)">{{ actions[issue.kind] || '查看成本配置' }}</button><button v-if="['cost_binding','cost_rule'].includes(issue.kind)" class="text-gray-500 underline" @click="repairIssue = issue; repairOpen = true">补算缺失记录</button></div></td>
        </tr></tbody>
      </table>
      <p v-if="summary && !summary.items.length" class="p-8 text-center text-sm text-gray-500">{{ summary.processing_count ? '新记录正在计算，稍后自动更新问题状态。' : '暂无待处理问题。请继续按实际发生登记采购、费用并核对供应商账单。' }}</p>
    </div>
    <div v-if="summary && summary.total > 100" class="flex items-center gap-3"><button class="btn btn-secondary" :disabled="loading || offset === 0" @click="offset -= 100; load()">上一页</button><span class="text-sm">第 {{ offset / 100 + 1 }} 页，共 {{ summary.total }} 项</span><button class="btn btn-secondary" :disabled="loading || offset + 100 >= summary.total" @click="offset += 100; load()">下一页</button></div>
    <details v-if="jobs.length" open class="text-sm"><summary class="cursor-pointer font-medium">最近补算任务</summary><ul class="mt-3 space-y-2"><li v-for="job in jobs" :key="job.id" class="flex flex-wrap justify-between gap-2"><span>#{{ job.id }} · {{ job.count }} 次请求 · {{ job.notes }}</span><span :class="job.status === 'completed' ? 'text-green-600' : 'text-primary-600'">{{ job.status === 'completed' ? '补算完成' : job.status === 'retrying' ? '等待自动重试' : '后台计算中' }}</span></li></ul></details>
    <p v-if="jobsError" role="alert" class="text-sm text-red-600">{{ jobsError }}</p>
    <BusinessCostRepairDialog v-if="repairOpen" :issue="repairIssue" :start-at="summary?.starts_at" :end-at="summary?.ends_at" :start-date="startDate" :end-date="endDate" @close="repairOpen = false" @queued="queued" />
    <BaseDialog :show="!!sourceIssue" title="确认资金来源" width="wide" @close="sourceIssue = undefined">
      <p class="mb-4 text-sm text-gray-500">补齐原始余额、收款或订阅的依据后，系统统一回算相关消费。这里不需要逐请求填写收入或成本。</p>
      <p v-if="sourcesError" role="alert" class="text-sm text-red-600">{{ sourcesError }}</p>
      <p v-if="sourcesLoading" class="text-sm text-gray-500">读取原始来源…</p>
      <div v-for="event in sources" :key="event.id" class="mb-3 flex flex-wrap items-center justify-between gap-3 border-b pb-3 text-sm dark:border-dark-700"><span>#{{ event.id }} · {{ businessKindLabels[event.event_type] }} · {{ event.payload.user_name || sourceIssue?.name }}</span><div class="flex gap-3"><button class="text-primary-600 underline" @click="emit('annotate', event)">确认此来源</button><button class="text-gray-500 underline" @click="emit('trace', event.id)">查看凭据</button></div></div>
      <p v-if="!sources.length && !sourcesLoading && !sourcesError" class="text-sm text-gray-500">没有可直接确认的原始来源。请先检查该用户的历史收款或余额差错；系统不会将未知消费自动认定为付费收入。</p>
      <button v-if="sourcesMore" class="btn btn-secondary" :disabled="sourcesLoading" @click="loadSources(true)">加载更多原始来源</button>
    </BaseDialog>
  </section>
</template>
<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import BusinessCostRepairDialog from './BusinessCostRepairDialog.vue'
import { businessAPI, type BusinessEvent, type BusinessIssue, type BusinessIssueSummary, type BusinessRepairJob, type BusinessRecordDefaults } from '@/api/admin/business'
import { businessKindLabels, cny, ledgerError } from '@/utils/business-ledger'
export interface BusinessConfigureRequest { mode: string; accountIDs?: number[]; accountNames?: Record<number, string>; poolID?: number; model?: string; effectiveAt?: string }
const props = defineProps<{ startDate: string; endDate: string; refreshKey?: number }>()
const emit = defineEmits<{ configure: [request: BusinessConfigureRequest]; record: [type: string, defaults: BusinessRecordDefaults]; annotate: [event: BusinessEvent]; trace: [id: number]; settled: []; summary: [summary: BusinessIssueSummary] }>()
const summary = ref<BusinessIssueSummary | null>(null), loading = ref(false), error = ref(''), offset = ref(0)
const jobs = ref<BusinessRepairJob[]>([]), jobsError = ref(''), selected = ref<number[]>([])
const repairOpen = ref(false), repairIssue = ref<BusinessIssue>()
const sourceIssue = ref<BusinessIssue>(), sources = ref<BusinessEvent[]>([]), sourcesLoading = ref(false), sourcesError = ref(''), sourcesMore = ref(false)
const labels: Record<string, string> = { cost_binding: '调用缺少成本绑定', cost_rule: '缺少有效期内价格', procurement: '采购额度或期初不足', fixed_cost: '缺少账号服务期费用', invoice_needed: '暂估成本待账单核对', invoice_changed: '账单覆盖成本已修订', funding_source: '资金来源待确认', cost_other: '其他成本缺口' }
const actions: Record<string, string> = { cost_binding: '绑定成本池', cost_rule: '补充价格', procurement: '登记采购', fixed_cost: '登记账号费用', invoice_needed: '核对整期账单', invoice_changed: '查看原核对记录', funding_source: '确认资金来源' }
const selectedAccounts = computed(() => [...new Set(selected.value)])
const selectedNames = computed(() => Object.fromEntries((summary.value?.items || []).filter(i => selectedAccounts.value.includes(i.account_id)).map(i => [i.account_id, i.name])))
const selectedEffective = computed(() => summary.value?.items.filter(i => selectedAccounts.value.includes(i.account_id)).map(i => i.first_at).sort()[0])
let sequence = 0, sourceSequence = 0, timer: ReturnType<typeof setTimeout> | undefined
let stopped = false
function schedule() { clearTimeout(timer); if (!stopped && (summary.value?.processing_count || summary.value?.calculation_error || jobs.value.some(j => j.status !== 'completed'))) timer = setTimeout(() => void refresh(false), 5000) }
async function load() {
  const seq = ++sequence; loading.value = true; error.value = ''
  try {
    const data = await businessAPI.issues({ start_date: props.startDate, end_date: props.endDate, offset: offset.value })
    if (seq !== sequence) return
    const previous = summary.value
    summary.value = data; emit('summary', data)
    if (previous && data.revision !== previous.revision && data.processing_count === 0) emit('settled')
  } catch (e) { if (seq === sequence) error.value = ledgerError(e) } finally { if (seq === sequence) { loading.value = false; schedule() } }
}
async function loadJobs() {
  jobsError.value = ''
  try { jobs.value = await businessAPI.repairJobs() } catch (e) { jobsError.value = ledgerError(e) }
  schedule()
}
async function refresh(reset = true) { if (reset) { offset.value = 0; selected.value = [] } await Promise.allSettled([load(), loadJobs()]) }
function handle(issue: BusinessIssue) {
  if (issue.kind === 'funding_source') { sourceIssue.value = issue; void loadSources(); return }
  if (issue.kind === 'invoice_changed') { emit('record', 'view_reconciliations', { pool_id: issue.pool_id }); return }
  if (issue.kind === 'procurement') { emit('record', 'purchase', { pool_id: issue.pool_id }); return }
  if (issue.kind === 'fixed_cost') { emit('record', 'expense', { account_id: issue.account_id, starts_at: issue.first_at }); return }
  if (issue.kind === 'invoice_needed') {
    // Invoice boundaries are editable; use a full calendar month, not request times.
    const start = new Date(issue.period + '-01T00:00:00'), end = new Date(start); end.setMonth(end.getMonth() + 1)
    emit('record', 'reconciliation', { pool_id: issue.pool_id, starts_at: issue.period_start_at || start.toISOString(), ends_at: issue.period_end_at || end.toISOString(), bill_mode: true }); return
  }
  emit('configure', { mode: issue.kind === 'cost_rule' ? 'rule' : 'binding', accountIDs: issue.account_id ? [issue.account_id] : [], accountNames: issue.account_id ? { [issue.account_id]: issue.name } : {}, poolID: issue.pool_id, model: issue.model, effectiveAt: issue.first_at })
}
async function loadSources(more = false) {
  if (!sourceIssue.value) return
  const seq = ++sourceSequence; sourcesLoading.value = true; sourcesError.value = ''
  if (!more) sources.value = []
  try {
    const data = await businessAPI.pending(more ? sources.value.at(-1)?.id : 0, sourceIssue.value.user_id)
    if (seq !== sourceSequence) return
    sources.value = more ? [...sources.value, ...data] : data; sourcesMore.value = data.length === 100
  } catch (e) { if (seq === sourceSequence) sourcesError.value = ledgerError(e) } finally { if (seq === sourceSequence) sourcesLoading.value = false }
}
function queued(job: BusinessRepairJob) { repairOpen.value = false; jobs.value = [job, ...jobs.value.filter(j => j.id !== job.id)]; void refresh(false) }
watch(() => [props.startDate, props.endDate], () => { summary.value = null; void refresh() }, { immediate: true })
watch(() => props.refreshKey, () => { void refresh(); if (sourceIssue.value) void loadSources() })
watch(sourceIssue, issue => { if (!issue) sourceSequence++ })
onUnmounted(() => { stopped = true; sequence++; sourceSequence++; clearTimeout(timer) })
defineExpose({ refresh })
</script>
