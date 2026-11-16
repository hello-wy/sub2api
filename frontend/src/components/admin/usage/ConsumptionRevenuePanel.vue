<template>
  <section class="consumption-report" aria-labelledby="consumption-heading" :aria-busy="loading">
    <header class="report-heading">
      <div><p class="eyebrow">人民币消费估算</p><h2 id="consumption-heading">消费折算组成</h2><p class="caption">与首页使用相同口径，可查看余额及各套餐分组的贡献。</p></div>
      <button type="button" class="refresh" :disabled="loading" @click="refresh">{{ loading ? '更新中…' : '刷新折算' }}</button>
    </header>
    <p v-if="error" class="report-error" role="alert">{{ error }} <button type="button" @click="refresh">重试</button></p>
    <p v-else-if="loading && !report" class="caption" role="status">正在核对消费与支付依据…</p>
    <template v-if="report && !error">
      <p class="period">{{ report.start_date }} 至 {{ report.end_date }} · {{ report.timezone }}<span v-if="loading"> · 正在更新</span></p>
      <div class="revenue-summary">
        <div class="total"><span>消费折合{{ partial ? '（已知部分）' : '' }}</span><strong>{{ cny(report.summary.total_cny) }}</strong><small>消费对应的价值估算，与同期收款不同</small></div>
        <dl><div><dt>余额消费折合</dt><dd>{{ cny(report.summary.balance_cny) }}</dd></div><div><dt>套餐消费折合</dt><dd>{{ cny(report.summary.subscription_cny) }}</dd></div><div><dt>原始实际扣费</dt><dd>${{ dollars(report.actual_credits) }}</dd></div><div :class="{ pending: partial }"><dt>待折算额度</dt><dd>${{ dollars(report.summary.unpriced_credits) }}</dd></div></dl>
      </div>
      <p v-if="partial" class="pending-note">还有 ${{ dollars(report.summary.unpriced_credits) }} 的消费未计入人民币合计，具体原因见下表。缺少依据的金额保留待折算。</p>
      <div v-if="report.components.length" class="report-table">
        <table><thead><tr><th>消费组成</th><th>换算依据 / 待补原因</th><th>计费请求</th><th>原始扣费（$）</th><th>折合金额（¥）</th></tr></thead>
          <tbody><tr v-for="row in report.components" :key="`${row.source}-${row.group_id}-${row.reason}`" :class="{ 'pending-row': row.reason !== 'priced' }"><td><b>{{ row.source === 'balance' ? '账户余额' : row.group_name || '未知套餐分组' }}</b><small>{{ row.source === 'balance' ? '余额消费' : '套餐消费' }}</small></td><td class="basis">{{ basis(row) }}</td><td>{{ row.requests.toLocaleString() }}</td><td>{{ dollars(row.credits) }}</td><td><strong>{{ row.reason === 'priced' ? cny(row.revenue_cny) : '待折算' }}</strong></td></tr></tbody>
          <tfoot><tr><th>合计</th><td>{{ partial ? '已知金额合计' : '人民币消费估算' }}</td><td>{{ report.requests.toLocaleString() }}</td><td>{{ dollars(report.actual_credits) }}</td><td>{{ cny(report.summary.total_cny) }}</td></tr></tfoot>
        </table>
      </div>
      <p v-else class="empty">所选时间范围暂无产生扣费的消费记录。</p>
      <details class="report-method"><summary>查看计算方式与估算范围</summary><p>余额消费：原始扣费 × 历史已履约人民币充值净实付 ÷ 净到账额度；到账额度含充值赠送。</p><p>套餐消费：关联订单净实付 × 本期消费比例，每期累计封顶至实付金额；余额购买套餐先按购买时的充值比例折算。</p><p>独立赠送余额未逐笔追踪；套餐采用当前分组整期额度。退款、分组配置和历史用量清理可能影响估算。本区不依赖下方经营记账是否启用，也不计入下方经营账的利润。</p></details>
    </template>
  </section>
</template>
<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { businessAPI, type ConsumptionRevenueComponent, type ConsumptionRevenueReport } from '@/api/admin/business'
const props = defineProps<{ startDate?: string; endDate?: string }>()
const report = ref<ConsumptionRevenueReport | null>(null)
const loading = ref(false)
const error = ref('')
let requestID = 0
const partial = computed(() => Number(report.value?.summary.unpriced_credits || 0) > 0)
const cny = (value: string) => new Intl.NumberFormat('zh-CN', {style: 'currency', currency: 'CNY'}).format(Number(value))
const dollars = (value: string) => new Intl.NumberFormat('en-US', {minimumFractionDigits: 2, maximumFractionDigits: 4}).format(Number(value))
const reasons: Record<string, string> = {missing_recharge: '缺少可换算的人民币充值依据', missing_subscription: '缺少订阅关联记录', missing_order: '缺少本消费时段的有效关联订单', unsupported_currency: '订单币种无法直接折合人民币', missing_period_quota: '缺少明确的整期额度'}
const basis = (row: ConsumptionRevenueComponent) => row.reason === 'priced' ? row.source === 'balance' ? '历史充值实付 / 到账额度加权均价' : '订单实付 / 整期额度 · 每期封顶' : reasons[row.reason] || '换算依据待补充'
async function refresh() {
  const id = ++requestID
  loading.value = true
  error.value = ''
  try {
    const result = await businessAPI.consumptionRevenue({start_date: props.startDate || undefined, end_date: props.endDate || undefined})
    if (id === requestID) report.value = result
  } catch (e) {
    if (id === requestID) { report.value = null; error.value = (e as {response?: {data?: {message?: string}}; message?: string}).response?.data?.message || (e as Error).message || '消费折算暂不可用，请稍后重试' }
  } finally {
    if (id === requestID) loading.value = false
  }
}
watch(() => [props.startDate, props.endDate], () => { report.value = null; void refresh() }, {immediate: true})
defineExpose({refresh})
</script>
<style scoped>
.consumption-report { @apply overflow-hidden rounded-2xl border border-gray-200 bg-white text-gray-800 dark:border-dark-700 dark:bg-dark-800 dark:text-gray-200; font-variant-numeric: tabular-nums; }
.report-heading { @apply flex flex-wrap items-start justify-between gap-4 px-5 pt-6 sm:px-6; }
.eyebrow { @apply mb-2 text-xs font-medium tracking-wider text-teal-700 dark:text-teal-300; }
h2 { @apply text-lg font-semibold tracking-tight text-gray-950 dark:text-white; }
.caption,.period { @apply mt-2 text-xs leading-6 text-gray-500 dark:text-gray-400; }
.period { @apply px-5 sm:px-6; }
.refresh { @apply rounded-lg border border-gray-200 px-3 py-2 text-xs font-medium hover:bg-gray-50 disabled:opacity-50 dark:border-dark-600 dark:hover:bg-dark-700; }
button:focus-visible,summary:focus-visible { @apply outline-none ring-2 ring-teal-500 ring-offset-2 dark:ring-offset-dark-800; }
.revenue-summary { @apply my-5 grid gap-6 px-5 sm:px-6 lg:grid-cols-[1fr_1.4fr]; }
.total { @apply rounded-xl bg-teal-50 p-5 dark:bg-teal-950/30; }
.total span { @apply text-sm text-teal-800 dark:text-teal-200; }
.total strong { @apply my-2 block text-3xl font-semibold tracking-tight text-teal-950 dark:text-teal-100; overflow-wrap:anywhere; }
.total small { @apply text-xs text-teal-700 dark:text-teal-300; }
dl { @apply grid grid-cols-2 gap-x-6 gap-y-5 self-center; }
dt { @apply text-xs text-gray-500 dark:text-gray-400; }
dd { @apply mt-1 text-xl font-semibold tracking-tight; overflow-wrap:anywhere; }
.pending dd { @apply text-amber-700 dark:text-amber-300; }
.pending-note { @apply mx-5 mb-5 rounded-lg bg-amber-50 px-4 py-3 text-xs leading-6 text-amber-900 dark:bg-amber-950/30 dark:text-amber-200 sm:mx-6; }
.report-table { @apply overflow-x-auto; }
table { @apply w-full whitespace-nowrap text-right text-sm; }
th { @apply bg-gray-50 px-5 py-3 text-xs font-medium text-gray-500 dark:bg-dark-900/40 dark:text-gray-400; }
td { @apply border-t border-gray-100 px-5 py-4 dark:border-dark-700; }
th:first-child,td:first-child { @apply text-left; }
td b { @apply font-medium; }
td small { @apply mt-1 block text-xs text-gray-500 dark:text-gray-400; }
.basis { @apply max-w-xs text-left text-xs leading-5 text-gray-500 dark:text-gray-400; white-space:normal; min-width:180px; }
.pending-row td:last-child { @apply text-amber-700 dark:text-amber-300; }
tfoot td { @apply bg-gray-50 font-medium dark:bg-dark-900/40; }
.report-method { @apply border-t border-gray-100 px-5 py-4 text-xs dark:border-dark-700 sm:px-6; }
.report-method summary { @apply cursor-pointer font-medium text-gray-600 dark:text-gray-300; }
.report-method p { @apply mt-3 leading-6 text-gray-500 dark:text-gray-400; }
.report-error { @apply m-5 rounded-lg bg-red-50 p-4 text-sm text-red-700 dark:bg-red-950/30 dark:text-red-300; }
.report-error button { @apply ml-2 underline; }
.empty { @apply px-5 py-8 text-center text-sm text-gray-500 dark:text-gray-400; }
.consumption-report > .caption { @apply px-5 pb-5; }
</style>
