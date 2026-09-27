<template>
  <section class="business-ledger" aria-label="人民币经营账">
    <header v-if="props.showHeader" class="business-ledger-header flex flex-wrap items-center justify-end gap-4 border-b pb-5 dark:border-dark-700">
      <div class="flex flex-wrap gap-2">
        <button class="btn btn-primary inline-flex items-center gap-1.5" @click="openRecord('expense')"><Icon name="dollar" size="sm" />录入费用</button>
        <button class="btn btn-secondary inline-flex items-center gap-1.5" @click="openRecord('receipt')"><Icon name="plus" size="sm" />登记收款</button>
        <button class="btn btn-secondary inline-flex items-center gap-1.5" @click="openRecord('purchase')"><Icon name="database" size="sm" />登记采购</button>
        <button class="btn btn-secondary inline-flex items-center gap-1.5" :disabled="loading" @click="refresh"><Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />刷新</button>
      </div>
    </header>

    <div v-if="props.showTabs" class="business-navigation">
      <BusinessPanelNavigation v-model="tab" />
      <span v-if="overview" class="business-updated" :title="date(overview.updated_at)"><Icon name="clock" size="xs" aria-hidden="true" />更新于 {{ updatedTime }} · 修订 #{{ overview.revision }}</span>
    </div>

    <template v-if="tab === 'overview' || tab === 'profit' || tab === 'benefits'">
      <div class="business-ledger-stack">
        <div v-if="overviewError || loading" class="business-status-stack">
          <p v-if="overviewError" role="alert" class="notice">{{ overviewError }}。收支台账、配置与录入仍可使用。<button class="ml-3 underline" @click="loadOverview">重试汇总</button></p>
          <p v-if="loading" role="status" class="business-loading">正在核对经营账…</p>
        </div>

        <template v-if="overview">
          <section class="business-status" :class="{ 'business-status--pending': needsReview }" aria-label="账目核对状态">
            <div class="business-status-main">
              <span class="business-status-icon"><Icon :name="needsReview ? 'exclamationTriangle' : 'checkCircle'" size="md" aria-hidden="true" /></span>
              <div class="min-w-0">
                <div class="business-status-title"><strong>{{ overview.quality.processing_count ? '账目计算中' : overview.profit_cny === null ? '利润待核对' : needsReview ? '部分账目待核对' : '本期账目已核对' }}</strong><span v-if="overview.quality.missing_count" class="business-status-badge">{{ overview.quality.missing_count }} 条记录缺少依据</span></div>
                <p>{{ reviewDescription }}</p>
                <div class="business-status-meta"><span>收入：{{ businessQuality(overview.quality.revenue) }}</span><span>成本：{{ businessQuality(overview.quality.cost) }}</span><span>归因：{{ businessQuality(overview.quality.attribution) }}</span><span>暂估 {{ overview.quality.estimated_count }} 项</span></div>
                <p v-if="overview.quality.cash !== 'confirmed'">现金收支含待核实金额，当前仅显示已知收付款。</p>
              </div>
            </div>
            <button type="button" class="business-status-action" @click="tab = 'reconcile'">{{ needsReview ? '查看待处理问题' : '查看核对记录' }}<Icon name="arrowRight" size="xs" aria-hidden="true" /></button>
          </section>

          <template v-if="tab === 'overview'">
            <section aria-label="本期经营">
              <div class="business-overview-heading">
                <div class="business-inline-heading"><h3>本期经营</h3><p>{{ periodLabel }} · 人民币</p></div>
                <div class="business-segmented-control" :style="{ '--segment-count': 2, '--segment-index': view === 'cash' ? 1 : 0 }" role="group" aria-label="经营视图">
                  <span class="business-segmented-control__slider" aria-hidden="true"></span>
                  <button type="button" :class="{ 'is-active': view === 'profit' }" :aria-pressed="view === 'profit'" @click="view = 'profit'">经营利润</button>
                  <button type="button" :class="{ 'is-active': view === 'cash' }" :aria-pressed="view === 'cash'" @click="view = 'cash'">现金收支</button>
                </div>
              </div>
              <div class="business-metrics">
                <button v-for="(metric, index) in metrics" :key="metric.label" type="button" class="metric business-summary-metric" :class="{ 'metric--primary': index === 0, 'metric--pending': index === 3 && metric.value === null }" :aria-label="metric.label + '，查看明细'" @click="showEntries(metric.kinds)">
                  <span class="metric-heading">{{ metric.label }}<Icon name="infoCircle" size="sm" aria-hidden="true" /></span>
                  <strong>{{ metric.value === null && index !== 3 ? '—' : cny(metric.value) }}</strong>
                  <small>{{ metric.hint }}<Icon v-if="index === 0" name="arrowRight" size="xs" aria-hidden="true" /></small>
                </button>
              </div>
            </section>

            <div class="business-middle">
              <section class="business-surface business-trend" aria-label="收支趋势">
                <div class="business-surface-heading"><h3>{{ view === 'cash' ? '现金收支趋势' : '收入与成本趋势' }}</h3><span class="business-surface-hint">{{ overview.timezone }} · 每日</span></div>
                <div v-if="!hasTrendData" class="business-empty-trend">
                  <span class="business-empty-icon"><Icon name="chartBar" size="lg" aria-hidden="true" /></span>
                  <h4>本期暂无已入账记录</h4>
                  <p>登记收支或完成期初补录后，在这里查看每日变化</p>
                  <div class="business-empty-actions"><button type="button" class="business-text-link" @click="openRecord('receipt')">登记收款<Icon name="arrowRight" size="xs" aria-hidden="true" /></button><button type="button" class="business-text-link" @click="tab = 'ledger'">查看收支台账<Icon name="arrowRight" size="xs" aria-hidden="true" /></button></div>
                </div>
                <div v-else class="business-chart-card"><BusinessLedgerCharts :daily="overview.daily" :view="view" /></div>
              </section>
              <section class="business-surface business-tasks" aria-label="完善经营账">
                <div class="business-surface-heading"><h3>完善经营账</h3><span class="business-surface-hint">从这里开始</span></div>
                <ol class="business-task-list">
                  <li :class="{ 'is-priority': hasUnknownBalance }"><span class="business-task-number">01</span><div><h4>确认期初余额来源</h4><p>区分付费价值与赠送额度，确认收入依据</p></div><button type="button" class="business-text-link" @click="goToReconciliation('sources')">确认来源<Icon name="arrowRight" size="xs" aria-hidden="true" /></button></li>
                  <li><span class="business-task-number">02</span><div><h4>核对上游采购与价格</h4><p>按供应商成本池维护采购凭据与价格规则</p></div><button type="button" class="business-text-link" @click="goToReconciliation('configuration')">去核对<Icon name="arrowRight" size="xs" aria-hidden="true" /></button></li>
                  <li><span class="business-task-number">03</span><div><h4>登记账号与日常费用</h4><p>账号月租、服务器及实际发生的活动支出</p></div><button type="button" class="business-text-link" @click="openRecord('expense')">去录入<Icon name="arrowRight" size="xs" aria-hidden="true" /></button></li>
                </ol>
              </section>
            </div>

            <section class="business-surface business-positions" aria-label="余额与待确认价值">
              <div class="business-overview-heading"><div class="business-inline-heading"><h3>余额与待确认价值</h3><p>截至最近入账 · 非所选区间期末值</p></div><button type="button" class="business-text-link" @click="positionsOpen = true">查看组成<Icon name="arrowRight" size="xs" aria-hidden="true" /></button></div>
              <dl class="business-position-grid"><div v-for="position in positions" :key="position.label" class="business-position"><dt>{{ position.label }}</dt><dd>{{ cny(position.value) }}</dd><small>{{ position.hint }}</small></div></dl>
              <p class="business-position-note"><Icon name="infoCircle" size="xs" aria-hidden="true" /><span>这里只汇总已入账价值；<template v-if="hasUnknownBalance">来源未知的 {{ credits(overview.unknown_wallet_credits) }} 额度仍需补录，不视为已确认的人民币价值。</template><template v-else>赠送额度不计为收款，余额变化请以原始凭据为准。</template></span></p>
            </section>

            <footer class="business-footer">
              <p><Icon name="infoCircle" size="xs" aria-hidden="true" />{{ view === 'cash' ? '现金按实际收付款日统计；余额购买订阅不再计一次收款。' : '经营利润仅涵盖已纳入台账的收入与费用；赠送兑现成本不重复扣减。' }}</p>
              <details class="business-history-panel" @toggle="loadLegacy"><summary>历史估算 · 只读</summary><div class="business-history-content"><p class="text-xs text-amber-700 dark:text-amber-300">旧报表使用当前充值比例及成本比例，仅供历史参考，不参与新账。</p><p v-if="legacyError" class="mt-3 text-sm text-red-600">{{ legacyError }}</p><div v-if="legacy" class="mt-3 flex flex-wrap gap-5 text-sm"><span>折算收入 {{ cny(String(legacy.usage_revenue_cny)) }}</span><span>旧估算成本 {{ cny(String(legacy.api_key_usage_cost_cny + legacy.welfare_cost_cny + legacy.account_cost_cny)) }}</span><span>旧估算差额 {{ cny(String(legacy.operating_profit_cny)) }}</span></div></div></details>
            </footer>
          </template>

          <section v-if="tab === 'profit'" class="business-section">
            <div class="business-section-heading"><div><h3>盈利明细</h3><p>按分组、套餐、模型或账号查看收入与成本归属。</p></div><div class="business-segmented-control business-segmented-control--dimensions" :style="{ '--segment-count': dimensions.length, '--segment-index': dimensionIndex }" role="group" aria-label="盈利明细维度"><span class="business-segmented-control__slider" aria-hidden="true"></span><button v-for="d in dimensions" :key="d.key" type="button" :class="{ 'is-active': dimension === d.key }" :aria-pressed="dimension === d.key" @click="dimension = d.key">{{ d.label }}</button></div></div>
            <p class="business-section-help">点击任一行查看对应明细及凭据。订阅模型收入标为分摊；公共、闲置、未归属项保留，确保与全站对平。</p>
            <div class="business-table-card overflow-x-auto"><table class="ledger-table"><thead><tr><th>归属</th><th>收入</th><th>已入账成本</th><th>贡献差额</th><th>状态</th></tr></thead><tbody><tr v-for="row in breakdown" :key="row.key"><td><button class="text-primary-600 underline" @click="showBreakdown(row.key)">{{ row.name }}</button></td><td>{{ cny(row.revenue_cny) }}</td><td>{{ cny(row.cost_cny) }}</td><td>{{ cny(row.profit_cny) }}</td><td>{{ row.missing_count ? row.missing_count + ' 项待核对' : row.estimated_count ? row.estimated_count + ' 项暂估' : businessQuality(row.allocation) }}</td></tr></tbody></table><p v-if="!breakdown.length" class="py-10 text-center text-sm text-gray-500">当前区间暂无已入账明细</p></div>
          </section>

          <section v-if="tab === 'benefits'" class="business-section">
            <div class="business-section-heading"><div><h3>福利与优惠</h3><p>区分赠送权益、成交让利和实际兑现成本，避免重复扣减。</p></div></div>
            <div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4"><div class="metric"><span>赠送额度发放</span><strong>{{ overview.gift_granted_credits }}</strong></div><div class="metric"><span>赠送额度已消费</span><strong>{{ overview.gift_used_credits }}</strong></div><button class="metric text-left" @click="showEntries(['gift_cost'])"><span>赠送服务兑现成本</span><strong>{{ cny(overview.gift_cost_cny) }}</strong><small>已包含在总服务成本中 ↗</small></button><button class="metric text-left" @click="showEntries(['discount'])"><span>成交优惠让利</span><strong>{{ cny(overview.discount_cny) }}</strong><small>已体现在较低实收中 ↗</small></button></div>
            <p class="notice">发放面值不直接扣利润；赠送兑现成本不重复扣减。现金奖励、实物奖品、渠道佣金请登记实际费用。</p>
            <div class="business-subsection business-benefit-footer"><p>最新未使用赠送余额：{{ overview.gift_outstanding_credits || '0' }} 额度。赠送订阅随服务期确认，未使用订阅收入保留在独立归属。</p><button class="btn btn-secondary" @click="showEntries(['gift_grant', 'gift_use', 'gift_cost', 'discount'])">查看福利与优惠明细</button></div>
          </section>
        </template>
      </div>
    </template>

    <template v-if="tab === 'ledger'">
      <section class="business-section">
        <div class="business-section-heading"><div><h3>收支与成本凭据</h3><p>按登记顺序列出全部台账，保留原始记录与更正。</p></div><button class="btn btn-secondary" @click="importOpen = true">CSV 批量导入</button></div>
        <p v-if="recordPoolFilter" class="notice">仅显示成本池「{{ configuration?.pools.find(p => p.id === recordPoolFilter)?.name || recordPoolFilter }}」的账单核对记录。<button class="ml-2 underline" @click="recordPoolFilter = 0">显示全部凭据</button></p>
        <p v-if="recordsError" role="alert" class="notice">{{ recordsError }}<button class="ml-2 underline" @click="loadRecords()">重试台账</button></p>
        <div class="business-table-card overflow-x-auto"><table class="ledger-table"><thead><tr><th>发生时间</th><th>类别</th><th>人民币 / 原币</th><th>凭据说明</th><th>操作</th></tr></thead><tbody><tr v-for="record in visibleRecords" :key="record.id"><td>{{ date(record.occurred_at) }}</td><td>{{ businessKindLabels[record.event_type] }}</td><td>{{ record.payload.amount_cny != null ? cny(String(record.payload.amount_cny)) : String(record.payload.pay_amount ?? '—') + ' ' + String(record.payload.currency || '') }}</td><td class="max-w-xs break-words">{{ record.payload.notes || record.payload.status || record.source_key }}</td><td><button class="text-primary-600 underline" @click="trace(record.id)">凭据</button><button v-if="['purchase','expense','adjustment','reconciliation','expense_stop'].includes(record.event_type)" class="ml-3 text-gray-500 underline" @click="openRecord('reversal', record)">冲销</button><button v-if="record.event_type === 'expense' && record.payload.ends_at" class="ml-3 text-gray-500 underline" @click="openRecord('expense_stop', record)">提前终止</button></td></tr></tbody></table><p v-if="!visibleRecords.length && !recordsError" class="py-10 text-center text-sm text-gray-500">{{ recordPoolFilter ? '当前已加载凭据中没有该成本池的核对记录，可继续加载更早凭据。' : '暂无台账，从顶部开始登记收款或成本。' }}</p></div>
        <button v-if="recordsMore" class="btn btn-secondary" :disabled="recordsLoading" @click="loadRecords(true)">加载更早凭据</button>
      </section>
    </template>

    <template v-if="tab === 'reconcile'">
      <section class="business-reconcile-stack">
        <p v-if="configurationNotice" role="status" class="notice">{{ configurationNotice }}</p>
        <p v-if="configError" role="alert" class="notice">{{ configError }}<button class="ml-2 underline" @click="loadConfiguration">重试配置</button></p>
        <div class="business-adjustments"><div class="business-subsection-heading"><h4>期初与账单调整</h4><p>登记期初和账单变动，保留独立凭据。</p></div><div class="flex flex-wrap gap-2"><button class="btn btn-secondary" @click="openRecord('opening_pool')">登记期初采购</button><button class="btn btn-secondary" @click="openRecord('reconciliation')">登记账单差异</button><button class="btn btn-secondary" @click="openRecord('supplier_refund')">登记采购退款</button><button class="btn btn-secondary" @click="openRecord('supplier_loss')">登记采购失效</button></div></div>
        <div ref="configurationSection" class="business-configuration-panel" tabindex="-1"><template v-if="configuration"><BusinessConfigurationPanel ref="configurationPanel" :configuration="configuration" @saved="configurationSaved" /></template></div>
        <div ref="sourcesSection" class="business-section" tabindex="-1"><BusinessIssuesPanel :start-date="startDate" :end-date="endDate" :refresh-key="issuesRefreshKey" @configure="configureIssue" @record="issueRecord" @annotate="openRecord('annotation', $event)" @trace="trace" @settled="loadOverview(); loadRecords()" @summary="issueSummary = $event" /></div>
      </section>
    </template>

    <p v-if="overview && !props.showTabs" class="business-updated" :title="date(overview.updated_at)"><Icon name="clock" size="xs" aria-hidden="true" />更新于 {{ updatedTime }} · 修订 #{{ overview.revision }}</p>

    <BaseDialog :show="positionsOpen" title="余额与待确认价值组成" width="wide" @close="positionsOpen = false">
      <p class="business-section-help">截至最近入账的已识别价值，不代表所选区间的期末余额。</p>
      <dl class="business-position-details"><div v-for="position in positions" :key="position.label"><dt>{{ position.label }}<small>{{ position.hint }}</small></dt><dd>{{ cny(position.value) }}</dd></div></dl>
      <p v-if="overview" class="notice">来源未知余额 {{ credits(overview.unknown_wallet_credits) }} 额度 · 未使用赠送余额 {{ credits(overview.gift_outstanding_credits) }} 额度。两者均不计为已确认的人民币价值。</p>
      <template #footer><button type="button" class="btn btn-secondary" @click="positionsOpen = false">关闭</button><button type="button" class="btn btn-primary" @click="positionsOpen = false; goToReconciliation('sources')">核对余额来源</button></template>
    </BaseDialog>
    <BusinessRecordDialog v-if="recordType" :type="recordType" :pools="configuration?.pools || []" :target="recordTarget" :defaults="recordDefaults" @close="recordType = ''" @saved="recordType = ''; refresh()" /><BusinessImportDialog v-if="importOpen" @close="importOpen = false" @saved="refresh" />
    <BaseDialog :show="entriesOpen" title="收入、成本与分摊明细" width="extra-wide" @close="entriesOpen = false"><div class="max-h-[65vh] overflow-auto"><table class="ledger-table"><thead><tr><th>时间 / 模型</th><th>科目</th><th>人民币</th><th>依据</th></tr></thead><tbody><tr v-for="entry in filteredEntries.slice(0, entriesLimit)" :key="entry.id"><td>{{ date(entry.occurred_at) }}<small class="block">{{ entry.model }}</small></td><td>{{ businessKindLabels[entry.kind] || entry.kind }}<small class="block text-gray-500">{{ entry.detail.allocation }}</small></td><td>{{ cny(entry.amount_cny) }}</td><td><button :disabled="!entry.event_id" class="text-primary-600 underline disabled:text-gray-400" @click="trace(entry.event_id)">{{ businessQuality(entry.quality) }} · #{{ entry.event_id }}</button></td></tr></tbody></table><button v-if="filteredEntries.length > entriesLimit" class="btn btn-secondary mt-3" @click="entriesLimit += 100">加载更多明细</button><p v-if="!filteredEntries.length" class="py-6 text-sm text-gray-500">无匹配记录</p></div></BaseDialog>
    <BaseDialog :show="traceOpen" title="原始凭据与修订链" width="wide" :z-index="60" @close="traceOpen = false"><p v-if="traceError" role="alert" class="text-red-600">{{ traceError }}</p><p v-if="traceLoading" class="text-sm text-gray-500">读取凭据…</p><div v-for="event in traceEvents" :key="event.id" class="mb-4 rounded-lg border p-4 dark:border-dark-700"><h4 class="text-sm font-semibold">#{{ event.id }} · {{ businessKindLabels[event.event_type] || event.event_type }}</h4><p class="mt-1 text-xs text-gray-500">发生 {{ date(event.occurred_at) }} · 登记 {{ date(event.recorded_at) }} · 操作人 {{ event.actor_id || '系统' }}</p><pre class="mt-3 max-h-80 overflow-auto whitespace-pre-wrap break-all text-xs">{{ JSON.stringify(event.payload, null, 2) }}</pre><button v-if="['wallet','opening_unknown','user_subscriptions','payment_orders'].includes(event.event_type)" class="btn btn-secondary mt-3" @click="traceOpen = false; openRecord('annotation', event)">确认此来源</button><details v-if="event.event_type === 'usage'" class="mt-3 text-sm"><summary class="cursor-pointer text-gray-500">高级：单次供应商实际成本修正</summary><button class="btn btn-secondary mt-2" @click="traceOpen = false; openRecord('annotation', event)">登记实际扣费凭据</button></details></div></BaseDialog>
  </section>
</template>
<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import BusinessRecordDialog from './BusinessRecordDialog.vue'
import BusinessImportDialog from './BusinessImportDialog.vue'
import BusinessConfigurationPanel from './BusinessConfigurationPanel.vue'
import BusinessIssuesPanel, { type BusinessConfigureRequest } from './BusinessIssuesPanel.vue'
import BusinessLedgerCharts from './BusinessLedgerCharts.vue'
import BusinessPanelNavigation, { businessTabs, type BusinessTab } from './BusinessPanelNavigation.vue'
import Icon from '@/components/icons/Icon.vue'
import { businessAPI, type BusinessOverview, type BusinessEvent, type BusinessEntry, type BusinessConfiguration, type BusinessRecordDefaults, type BusinessIssueSummary } from '@/api/admin/business'
import { getBusinessAnalytics, type BusinessAnalyticsOverview } from '@/api/admin/dashboard'
import { businessKindLabels, businessQuality, cny, ledgerError } from '@/utils/business-ledger'
const props = withDefaults(defineProps<{ startDate: string; endDate: string; showHeader?: boolean; showTabs?: boolean }>(), {
  showHeader: true,
  showTabs: true,
})
const dimensions = [{ key: 'groups', label: '分组' }, { key: 'plans', label: '套餐' }, { key: 'models', label: '模型' }, { key: 'accounts', label: '账号' }] as const
type Dimension = typeof dimensions[number]['key']
const activeTab = defineModel<BusinessTab>('activeTab', { default: 'overview' })
const tab = computed<BusinessTab>({
  get: () => activeTab.value ?? 'overview',
  set: value => { activeTab.value = value },
})
const view = ref('profit'), dimension = ref<Dimension>('groups')
const dimensionIndex = computed(() => Math.max(0, dimensions.findIndex((item) => item.key === dimension.value)))
const overview = ref<BusinessOverview | null>(null), configuration = ref<BusinessConfiguration | null>(null), records = ref<BusinessEvent[]>([])
const loading = ref(false), recordsLoading = ref(false), overviewError = ref(''), recordsError = ref(''), configError = ref(''), recordsMore = ref(false)
const configurationNotice = ref('')
const issuesRefreshKey = ref(0), issueSummary = ref<BusinessIssueSummary | null>(null)
const configurationPanel = ref<InstanceType<typeof BusinessConfigurationPanel> | null>(null)
const recordDefaults = ref<BusinessRecordDefaults>({}), recordPoolFilter = ref(0)
const visibleRecords = computed(() => recordPoolFilter.value ? records.value.filter(r => r.event_type === 'reconciliation' && Number(r.payload.pool_id) === recordPoolFilter.value) : records.value)
const recordType = ref(''), recordTarget = ref<BusinessEvent>(), importOpen = ref(false)
const legacy = ref<BusinessAnalyticsOverview | null>(null), legacyError = ref('')
const entriesOpen = ref(false), entriesLimit = ref(100), entryKinds = ref<string[]>([]), selectedDimension = ref<Dimension | ''>(''), selectedKey = ref('')
const positionsOpen = ref(false)
const sourcesSection = ref<HTMLElement | null>(null), configurationSection = ref<HTMLElement | null>(null)
const traceOpen = ref(false), traceLoading = ref(false), traceError = ref(''), traceEvents = ref<BusinessEvent[]>([])
let overviewSequence = 0, traceSequence = 0
const date = (s: string) => s ? new Date(s).toLocaleString('zh-CN', { timeZone: overview.value?.timezone || configuration.value?.timezone || 'Asia/Shanghai' }) : '—'
const breakdown = computed(() => overview.value?.[dimension.value] || [])
const gaps = computed(() => overview.value?.entries.filter(e => e.quality === 'unknown') || [])
const positions = computed(() => {
  const o = overview.value
  if (!o) return []
  return [
    { label: '用户未消费付费价值', value: o.wallet_deferred_cny, hint: '已识别付费部分 · 含冻结及抽奖券' },
    { label: '订阅待确认收入', value: o.subscription_deferred_cny, hint: '随剩余服务期确认' },
    { label: '未消耗上游采购', value: o.prepaid_supplier_cny, hint: '待实际消耗后计入成本' },
    { label: '预付费用剩余价值', value: o.prepaid_expense_cny, hint: '按费用归属期间摊销' },
  ]
})
const credits = (value: string) => new Intl.NumberFormat('zh-CN', { maximumFractionDigits: 2 }).format(Number(value || '0'))
const hasUnknownBalance = computed(() => Number(overview.value?.unknown_wallet_credits || '0') > 0)
const needsReview = computed(() => overview.value?.profit_cny === null || overview.value?.quality.cash !== 'confirmed' || hasUnknownBalance.value)
const updatedTime = computed(() => overview.value ? new Date(overview.value.updated_at).toLocaleTimeString('zh-CN', { timeZone: overview.value.timezone, hour: '2-digit', minute: '2-digit' }) : '')
const periodLabel = computed(() => `${props.startDate.replace(/-/g, '.')} — ${props.endDate.replace(/-/g, '.')}`)
const reviewDescription = computed(() => {
  const o = overview.value
  if (!o) return ''
  const parts: string[] = []
  if (o.quality.processing_count) parts.push(`后台正在计算 ${o.quality.processing_count} 条记录，无需逐条处理。`)
  if (issueSummary.value?.total) parts.push(`已按原因归集为 ${issueSummary.value.total} 项待处理问题。`)
  if (hasUnknownBalance.value) parts.push(`有 ${credits(o.unknown_wallet_credits)} 额度的余额来源待补录，消费后的收入暂无法准确确认。`)
  const reason = gaps.value.find(entry => typeof entry.detail.reason === 'string')?.detail.reason
  if (reason) parts.push(String(reason))
  if (!parts.length) parts.push(o.profit_cny === null ? '部分收入或成本缺少依据，请按问题补齐配置、确认资金来源或核对供应商账单。' : '本期已纳入台账的收入与成本已有核对依据，请继续按实际发生登记收支。')
  return parts.join(' ')
})
const hasTrendData = computed(() => {
  const o = overview.value
  if (!o) return false
  const kinds = view.value === 'cash' ? ['cash_in', 'cash_out', 'cash_refund'] : ['wallet_revenue', 'subscription_revenue', 'subscription_close_revenue', 'ticket_revenue', 'refund_revenue', 'usage_cost', 'fixed_cost', 'operating_cost']
  const fields = view.value === 'cash' ? ['cash_in_cny', 'cash_out_cny', 'cash_refund_cny'] as const : ['revenue_cny', 'cost_cny'] as const
  return o.entries.some(entry => kinds.includes(entry.kind) && entry.amount_cny != null) || o.daily.some(day => fields.some(key => Number(day[key] || '0') !== 0))
})
async function goToReconciliation(section: 'sources' | 'configuration') {
  tab.value = 'reconcile'
  await nextTick()
  const target = section === 'sources' ? sourcesSection.value : configurationSection.value
  target?.scrollIntoView({ block: 'start' })
  target?.focus({ preventScroll: true })
}
const revenueKinds = ['wallet_revenue', 'subscription_revenue', 'subscription_close_revenue', 'ticket_revenue', 'refund_revenue', 'revenue_gap']
const costKinds = ['usage_cost', 'fixed_cost', 'operating_cost', 'cost_gap']
const metrics = computed(() => {
  const o = overview.value; if (!o) return []
  const costUnverified = o.quality.cost === 'unknown' || o.quality.cost === 'pending'
  // Only combine values for display; accounting amounts remain server decimals.
  const expenses = String(Number(o.fixed_cost_cny) + Number(o.operating_cost_cny))
  if (view.value === 'cash') return [
    { label: '实际收款', value: o.cash_in_cny, hint: '线上 + 线下', kinds: ['cash_in', 'cash_gap'] },
    { label: '实际退款', value: o.cash_refund_cny, hint: '按退款发生日', kinds: ['cash_refund'] },
    { label: '实际经营付款', value: o.cash_out_cny, hint: '含采购与预付费用', kinds: ['cash_out'] },
    { label: '经营现金净流入', value: o.cash_net_cny, hint: '收款 − 退款 − 付款', kinds: ['cash_in', 'cash_refund', 'cash_out', 'cash_gap'] },
  ]
  return [
    { label: '已确认收入', value: o.recognized_revenue_cny, hint: o.quality.revenue === 'confirmed' ? '已入账收入 · 查看凭据' : `已入账部分 · 收入${businessQuality(o.quality.revenue)}`, kinds: revenueKinds },
    { label: '上游消耗成本', value: costUnverified ? null : o.usage_cost_cny, hint: o.quality.cost === 'confirmed' ? '采购加权成本 / 合同计价' : `${businessQuality(o.quality.cost)} · 已入账 ${cny(o.usage_cost_cny)}`, kinds: ['usage_cost', 'cost_gap'] },
    { label: '账号及经营费用', value: costUnverified ? null : expenses, hint: costUnverified ? `${businessQuality(o.quality.cost)} · 已入账 ${cny(expenses)}` : `账号 ${cny(o.fixed_cost_cny)} · 其他 ${cny(o.operating_cost_cny)}`, kinds: ['fixed_cost', 'operating_cost'] },
    { label: '本期经营利润', value: o.profit_cny, hint: o.profit_cny === null ? `已知收支差额 ${cny(o.known_profit_cny)}，不能视为完整利润` : '台账范围经营利润 · 查看凭据', kinds: [...revenueKinds, ...costKinds] },
  ]
})
const filteredEntries = computed(() => (overview.value?.entries || []).filter((e: BusinessEntry) => {
  if (e.kind === 'usage_weight') return false
  if (entryKinds.value.length && !entryKinds.value.includes(e.kind)) return false
  if (!selectedDimension.value) return true
  const key = selectedDimension.value === 'groups' ? String(e.group_id) : selectedDimension.value === 'accounts' ? String(e.account_id) : selectedDimension.value === 'plans' ? String(e.plan_id) : e.model
  return key === selectedKey.value
}))
function showEntries(kinds: string[]) { entryKinds.value = kinds; selectedDimension.value = ''; entriesLimit.value = 100; entriesOpen.value = true }
function showBreakdown(key: string) { showEntries([...revenueKinds, ...costKinds]); selectedDimension.value = dimension.value; selectedKey.value = key }
function openRecord(type: string, target?: BusinessEvent) { recordDefaults.value = {}; recordTarget.value = target; recordType.value = type }
function issueRecord(type: string, defaults: BusinessRecordDefaults) {
  if (type === 'view_reconciliations') { recordPoolFilter.value = defaults.pool_id || 0; tab.value = 'ledger'; void loadRecords(); return }
  openRecord(type); recordDefaults.value = defaults
}
async function configureIssue(request: BusinessConfigureRequest) {
  await goToReconciliation('configuration')
  if (!configurationPanel.value) { configError.value = '请先重试加载成本配置，再处理此问题'; return }
  configurationPanel.value.open(request)
}
async function configurationSaved() { await loadConfiguration(); configurationNotice.value = '配置已保存。历史调用的缺口可在「待处理问题」中预览并补算缺失记录。'; issuesRefreshKey.value++ }
async function loadOverview() {
  const sequence = ++overviewSequence; loading.value = true; overviewError.value = ''
  try { const result = await businessAPI.overview({ start_date: props.startDate, end_date: props.endDate }); if (sequence === overviewSequence) overview.value = result }
  catch (e) { if (sequence === overviewSequence) { overview.value = null; overviewError.value = ledgerError(e) } }
  finally { if (sequence === overviewSequence) loading.value = false }
}
async function loadRecords(more = false) {
  if (recordsLoading.value) return
  recordsLoading.value = true; recordsError.value = ''
  try { const data = await businessAPI.records(more ? records.value.at(-1)?.id : 0); records.value = more ? [...records.value, ...data] : data; recordsMore.value = data.length === 100 }
  catch (e) { recordsError.value = ledgerError(e) }
  finally { recordsLoading.value = false }
}
async function loadConfiguration() { configError.value = ''; try { configuration.value = await businessAPI.configuration() } catch (e) { configError.value = ledgerError(e) } }
async function refresh() { issuesRefreshKey.value++; await Promise.allSettled([loadOverview(), loadRecords(), loadConfiguration()]) }
async function trace(id: number) {
  const sequence = ++traceSequence; traceOpen.value = true; traceLoading.value = true; traceError.value = ''; traceEvents.value = []
  try { const result = await businessAPI.trace(id); if (sequence === traceSequence) traceEvents.value = result } catch (e) { if (sequence === traceSequence) traceError.value = ledgerError(e) }
  finally { if (sequence === traceSequence) traceLoading.value = false }
}
async function loadLegacy(event: Event) {
  if (!(event.target as HTMLDetailsElement).open || legacy.value) return
  try { legacy.value = await getBusinessAnalytics({ start_date: props.startDate, end_date: props.endDate }) } catch (e) { legacyError.value = ledgerError(e) }
}
watch(() => [props.startDate, props.endDate], () => { legacy.value = null; void loadOverview() }, { immediate: true })
void Promise.allSettled([loadRecords(), loadConfiguration()])

function selectTab(value: string) {
  const item = businessTabs.find(item => item.key === value)
  if (item) tab.value = item.key
}

defineExpose({ openRecord, refresh, selectTab })
</script>
<style scoped>
.business-ledger { @apply min-w-0 space-y-6; }
.business-navigation { @apply flex min-w-0 items-center justify-between gap-4 border-b border-gray-200 pb-3 dark:border-dark-700; }
.business-updated { @apply flex shrink-0 items-center gap-1.5 text-[11px] text-gray-500 dark:text-gray-400; }
.business-navigation .business-updated { @apply hidden 2xl:inline-flex; }
.business-ledger-stack { @apply space-y-6; }
.business-status-stack { @apply space-y-3; }
.business-loading { @apply rounded-xl border border-gray-200 bg-white px-4 py-5 text-center text-sm text-gray-500 dark:border-dark-700 dark:bg-dark-800 dark:text-gray-400; }
.business-status { @apply flex flex-wrap items-center justify-between gap-4 rounded-xl border border-gray-200 bg-white px-5 py-4 text-gray-600 dark:border-dark-700 dark:bg-dark-800 dark:text-gray-300; }
.business-status--pending { @apply border-amber-200/70 bg-amber-50/70 text-amber-900 dark:border-amber-900/60 dark:bg-amber-950/20 dark:text-amber-200; }
.business-status-main { @apply flex min-w-0 flex-1 items-start gap-3.5; }
.business-status-icon { @apply inline-flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-gray-300; }
.business-status--pending .business-status-icon { @apply bg-amber-100/80 text-amber-600 dark:bg-amber-900/40 dark:text-amber-400; }
.business-status-title { @apply flex flex-wrap items-center gap-2.5 text-sm leading-6; }
.business-status-title strong { @apply font-semibold; }
.business-status-badge { @apply rounded bg-amber-100/80 px-2 py-0.5 text-[10px] font-medium text-amber-800 dark:bg-amber-900/50 dark:text-amber-200; }
.business-status-main p { @apply mt-1 text-xs leading-5; }
.business-status-meta { @apply mt-2 flex flex-wrap items-center gap-x-3 gap-y-1 text-[11px] opacity-80; }
.business-status-meta span + span { @apply border-l border-gray-300/60 pl-3 dark:border-dark-500; }
.business-status-action { @apply inline-flex h-9 shrink-0 items-center gap-2 rounded-lg border border-gray-200 bg-white px-3 text-xs font-medium transition-colors hover:bg-gray-50 dark:border-dark-600 dark:bg-dark-800 dark:hover:bg-dark-700; }
.business-status--pending .business-status-action { @apply border-amber-200 bg-white/70 hover:bg-white dark:border-amber-900 dark:bg-amber-950/30 dark:hover:bg-amber-900/40; }
.business-overview-heading { @apply mb-4 flex flex-wrap items-center justify-between gap-3; }
.business-inline-heading { @apply flex flex-wrap items-baseline gap-x-3 gap-y-1; }
.business-inline-heading h3 { @apply text-base font-semibold text-gray-900 dark:text-white; }
.business-inline-heading p { @apply text-[11px] leading-5 text-gray-500 dark:text-gray-400; }
.business-metrics { @apply grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4; }
.metric { @apply min-w-0 rounded-xl border border-gray-200 bg-white p-5 text-left text-gray-900 dark:border-dark-700 dark:bg-dark-800 dark:text-white; }
button.metric { @apply transition-colors hover:border-primary-300 dark:hover:border-primary-700; }
.metric > span { @apply block text-xs text-gray-500 dark:text-gray-400; }
.metric > strong { @apply mt-4 block break-words text-[28px] font-semibold leading-9 tracking-tight tabular-nums; }
.metric > small { @apply mt-2 flex items-center gap-1 text-[11px] leading-5 text-gray-500 dark:text-gray-400; }
.metric .metric-heading { @apply flex items-center justify-between gap-2; }
.metric-heading svg { @apply shrink-0 text-gray-400 dark:text-gray-500; }
.business-summary-metric { @apply min-h-[155px]; }
.metric--primary { @apply border-t-[3px] border-t-blue-500 pt-[18px] dark:border-t-blue-400; }
.metric--pending { @apply border-amber-200/70 bg-amber-50/40 dark:border-amber-900/60 dark:bg-amber-950/20; }
.metric--pending > strong { @apply text-2xl text-amber-700 dark:text-amber-300; }
.metric--pending > span, .metric--pending > small { @apply text-amber-800/80 dark:text-amber-200/80; }
.business-middle { @apply grid min-w-0 gap-5 xl:grid-cols-[minmax(0,1.65fr)_minmax(340px,1fr)]; }
.business-surface { @apply min-w-0 rounded-xl border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800; }
.business-surface-heading { @apply flex flex-wrap items-center justify-between gap-2 px-5 pt-5; }
.business-surface-heading h3 { @apply text-sm font-semibold text-gray-900 dark:text-white; }
.business-surface-hint { @apply text-[11px] text-gray-500 dark:text-gray-400; }
.business-empty-trend { @apply flex min-h-[245px] flex-col items-center justify-center px-5 py-6 text-center; }
.business-empty-icon { @apply mb-3 inline-flex h-12 w-12 items-center justify-center rounded-xl border border-blue-100/70 bg-blue-50/70 text-blue-300 dark:border-dark-600 dark:bg-dark-700 dark:text-blue-400; }
.business-empty-trend h4 { @apply text-sm font-medium text-gray-600 dark:text-gray-300; }
.business-empty-trend p { @apply mt-2 text-xs leading-5 text-gray-500 dark:text-gray-400; }
.business-empty-actions { @apply mt-4 flex flex-wrap justify-center gap-x-5 gap-y-2; }
.business-empty-actions button + button { @apply border-l border-gray-200 pl-5 dark:border-dark-600; }
.business-text-link { @apply inline-flex shrink-0 items-center gap-1.5 rounded text-xs font-medium text-primary-600 transition-colors hover:text-primary-800 dark:text-primary-300 dark:hover:text-primary-200; }
.business-chart-card { @apply min-w-0 p-4; }
.business-task-list { @apply px-5; }
.business-task-list li { @apply flex items-center gap-3 border-b border-gray-100 py-5 last:border-0 dark:border-dark-700; }
.business-task-list li > div { @apply min-w-0 flex-1; }
.business-task-number { @apply inline-flex h-6 w-6 shrink-0 items-center justify-center rounded-full border border-gray-200 bg-gray-50 text-[10px] tabular-nums text-gray-500 dark:border-dark-600 dark:bg-dark-900 dark:text-gray-400; }
.is-priority .business-task-number { @apply border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-900 dark:bg-amber-950/40 dark:text-amber-300; }
.business-task-list h4 { @apply text-xs font-medium leading-5 text-gray-700 dark:text-gray-200; }
.business-task-list p { @apply mt-1 text-[11px] leading-5 text-gray-500 dark:text-gray-400; }
.business-positions { @apply p-5; }
.business-positions h3 { @apply text-sm; }
.business-position-grid { @apply grid grid-cols-1 gap-y-5 sm:grid-cols-2 xl:grid-cols-4; }
.business-position { @apply min-w-0 sm:border-l sm:border-gray-200 sm:px-5 sm:dark:border-dark-700; }
.business-position:first-child { @apply border-l-0 pl-0; }
.business-position dt { @apply text-xs leading-5 text-gray-500 dark:text-gray-400; }
.business-position dd { @apply mt-2 break-words text-[22px] font-semibold tabular-nums text-gray-700 dark:text-gray-100; }
.business-position small { @apply mt-1 block text-[11px] leading-5 text-gray-500 dark:text-gray-400; }
.business-position-note { @apply mt-5 flex items-start gap-1.5 border-t border-gray-100 pt-3 text-[11px] leading-5 text-gray-500 dark:border-dark-700 dark:text-gray-400; }
.business-position-note svg, .business-footer > p svg { @apply mt-1 shrink-0; }
.business-position-details { @apply my-5 divide-y divide-gray-100 dark:divide-dark-700; }
.business-position-details > div { @apply flex flex-wrap items-center justify-between gap-3 py-3 text-sm; }
.business-position-details small { @apply mt-1 block text-xs text-gray-500 dark:text-gray-400; }
.business-position-details dd { @apply font-semibold tabular-nums; }
.business-footer { @apply flex flex-wrap items-start justify-between gap-4 text-gray-500 dark:text-gray-400; }
.business-footer > p { @apply flex flex-1 items-start gap-1.5 text-[11px] leading-5; }
.business-history-panel { @apply max-w-full text-xs; }
.business-history-panel[open] { @apply basis-full; }
.business-history-panel summary { @apply cursor-pointer text-right text-xs leading-5 hover:text-gray-800 dark:hover:text-gray-200; }
.business-history-content { @apply mt-3 rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800; }
.business-section { @apply space-y-5 rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800 sm:p-5; }
.business-section-heading { @apply flex flex-col gap-4 border-b border-gray-100 pb-4 dark:border-dark-700 md:flex-row md:items-start md:justify-between; }
.business-section-heading h3 { @apply text-base font-semibold text-gray-900 dark:text-white; }
.business-section-heading p { @apply mt-1 text-xs leading-5 text-gray-500 dark:text-gray-400; }
.business-segmented-control { --segment-count: 2; --segment-index: 0; @apply relative grid min-w-[180px] grid-cols-2 overflow-hidden rounded-lg bg-gray-200/60 p-[3px] dark:bg-dark-900; }
.business-segmented-control--dimensions { @apply min-w-[260px] grid-cols-4; }
.business-segmented-control__slider { @apply pointer-events-none absolute inset-y-[3px] left-[3px] rounded-md bg-white shadow-sm transition-transform duration-200 ease-out dark:bg-dark-700; width: calc((100% - 6px) / var(--segment-count)); transform: translateX(calc(var(--segment-index) * 100%)); }
.business-segmented-control button { @apply relative z-[1] min-w-0 px-3 py-1.5 text-xs font-medium text-gray-500 transition-colors hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-100; }
.business-segmented-control button.is-active { @apply text-primary-700 dark:text-primary-200; }
.business-section-help { @apply text-xs leading-5 text-gray-500 dark:text-gray-400; }
.business-subsection { @apply space-y-4 border-t border-gray-100 pt-5 dark:border-dark-700; scroll-margin-top: 1rem; }
.business-subsection-heading h4 { @apply text-sm font-semibold text-gray-900 dark:text-white; }
.business-subsection-heading p { @apply mt-1 text-xs leading-5 text-gray-500 dark:text-gray-400; }
.business-table-card { @apply overflow-x-auto rounded-lg border border-gray-100 dark:border-dark-700; }
.business-table-card .ledger-table { @apply min-w-[760px]; }
.business-benefit-footer { @apply flex flex-col gap-3 md:flex-row md:items-center md:justify-between; }
.business-benefit-footer p { @apply text-sm leading-6 text-gray-600 dark:text-gray-300; }
.business-reconcile-stack { @apply space-y-6; }
.business-adjustments { @apply flex flex-wrap items-center justify-between gap-4 rounded-xl border border-gray-200 bg-white p-5 dark:border-dark-700 dark:bg-dark-800; }
.business-configuration-panel { @apply min-w-0 rounded-lg; scroll-margin-top: 1rem; }
.business-gap-list { @apply space-y-2 rounded-lg border border-amber-100 bg-amber-50/60 p-4 dark:border-amber-900/50 dark:bg-amber-950/20; }
.business-gap-list h5 { @apply mb-2 text-sm font-semibold text-amber-900 dark:text-amber-200; }
.notice { @apply rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm leading-relaxed text-amber-900 dark:border-amber-900 dark:bg-amber-950/30 dark:text-amber-200; }
.ledger-table { @apply w-full text-left text-sm; }
.ledger-table th { @apply whitespace-nowrap border-b border-gray-100 bg-gray-50/70 px-3 py-3 text-xs font-medium text-gray-500 dark:border-dark-700 dark:bg-dark-900/30 dark:text-gray-400; }
.ledger-table td { @apply border-b border-gray-100 px-3 py-3 tabular-nums dark:border-dark-700; }
.business-ledger button:focus-visible, .business-ledger summary:focus-visible { @apply outline-none ring-2 ring-primary-400 ring-offset-2 dark:ring-offset-dark-900; }
@media (min-width: 640px) and (max-width: 1279px) { .business-position:nth-child(3) { @apply border-l-0 pl-0; } }
@media (max-width: 639px) {
  .business-status-main { @apply basis-full; }
  .business-status-action { @apply ml-auto; }
  .business-task-list li { @apply flex-wrap; }
  .business-task-list li > .business-text-link { @apply ml-9; }
  .business-footer > p { @apply basis-full; }
}
@media (prefers-reduced-motion: reduce) { .business-segmented-control__slider { transition: none; } }
</style>
