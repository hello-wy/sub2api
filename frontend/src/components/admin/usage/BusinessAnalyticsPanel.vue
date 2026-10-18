<template>
  <section class="operating" aria-label="经营统计">
    <p class="help">{{ op('summary') }}</p>
    <div class="toolbar">
      <nav aria-label="经营分析子页" class="tabs">
        <button v-for="item in tabs" :key="item.key" type="button" :aria-current="tab === item.key ? 'page' : undefined" :class="{ selected: tab === item.key }" @click="tab = item.key">{{ item.label }}</button>
      </nav>
      <div class="actions"><button class="quiet-button" :disabled="loading" @click="refresh">{{ loading ? '更新中…' : '刷新' }}</button><button class="secondary-button" :disabled="!settings?.enabled_at" @click="startRecord('income')">记收入</button><button class="primary-button" :disabled="!settings?.enabled_at" @click="startRecord('cost')">记一笔成本</button></div>
    </div>

    <p v-if="settingsError" class="error" role="alert">{{ settingsError }} <button @click="refresh">重试</button></p>
    <div v-if="settings && !settings.enabled_at && tab !== 'settings'" class="welcome">
      <span class="welcome-label">开始记经营账</span><h2>历史用一个总数接上，今天开始清楚记录。</h2>
      <p>设置历史成本和已确认收入后，系统自动汇总消费与赠送。用户现有余额和订阅不会被重置。</p>
      <button class="primary-button" @click="tab = 'settings'">设置期初</button>
    </div>

    <template v-if="tab === 'overview'">
      <p v-if="overviewError" class="error" role="alert">{{ overviewError }} <button @click="loadOverview">重新加载</button></p>
      <div v-if="loading && !overview" class="metric-grid" aria-label="统计加载中" aria-busy="true"><div v-for="n in 4" :key="n" class="skeleton" /></div>
      <template v-if="overview && settings?.enabled_at">
        <div class="period-heading"><span>{{ overview.start_date }} 至 {{ overview.end_date }} <span class="muted">· {{ settings.timezone }} · 人民币</span></span><span class="muted">更新于 {{ timestamp(overview.updated_at) }}</span></div>
        <div v-if="overview.pending_terms" class="notice"><span>{{ overview.pending_terms }} 个订阅权益期需要补充计价基准，相关金额暂未计全。</span><button @click="tab = 'settings'">补充基准 →</button></div>
        <div class="metric-grid">
          <article class="metric profit-metric"><span class="metric-label">本期经营估算利润</span><strong :class="{ negative: Number(overview.summary.profit) < 0 }">{{ money(overview.summary.profit) }}</strong><p v-if="overview.summary.profit === null">已知差额 {{ money(overview.summary.known_profit) }}，待补齐订阅基准</p><p v-else>确认收入 − 实际成本 − 赠送成本</p></article>
          <article class="metric"><span class="metric-label">{{ overview.summary.missing ? '已知确认收入' : '确认收入' }}</span><strong>{{ money(overview.summary.revenue) }}</strong><p>余额 {{ money(overview.summary.balance_revenue) }} · 订阅 {{ money(overview.summary.subscription_revenue) }}</p><small>其他收入与调整 {{ money(overview.summary.other_revenue) }}</small></article>
          <article class="metric"><span class="metric-label">实际成本</span><strong>{{ money(overview.summary.manual_cost) }}</strong><p>采购、账号、服务器等手工支出</p><small>以已登记费用为准</small></article>
          <article class="metric"><span class="metric-label">赠送成本</span><strong>{{ money(overview.summary.gift_cost) }}</strong><p>实际到账时按赠送价值计入</p><small>包含返利、签到、抽奖和注册奖励</small></article>
        </div>
        <div class="cumulative"><span>累计经营估算利润 <small>含期初 · 截至当前</small></span><strong>{{ money(overview.cumulative_profit) }}</strong><span v-if="overview.cumulative_profit === null" class="muted">已知累计差额 {{ money(overview.cumulative_known_profit) }}</span></div>

        <section class="surface positions">
          <div class="section-title"><h2>还需要提供多少服务</h2><span>当前存量 · 不随统计区间变化</span></div>
          <div class="position-grid">
            <div><span>用户可用余额</span><strong>{{ money(overview.positions.balance) }}</strong><small>含赠送余额，按人民币 1:1</small></div>
            <div><span>冻结待结算</span><strong>{{ money(overview.positions.frozen) }}</strong><small>预留中的额度，尚未确认消费</small></div>
            <div><span>订阅待消费价值</span><strong>{{ money(overview.positions.subscription_value) }}</strong><small>{{ overview.positions.active_subscriptions }} 个有效订阅<span v-if="overview.positions.missing_subscriptions"> · {{ overview.positions.missing_subscriptions }} 个基准待补</span></small></div>
          </div>
          <div class="position-foot"><span>用户欠款 {{ money(overview.positions.debt) }}，单列不抵减余额。</span><span v-if="overview.positions.suspended_count">暂停订阅 {{ overview.positions.suspended_count }} 个，已知剩余价值 {{ money(overview.positions.suspended_value) }}。</span></div>
        </section>

        <section class="surface"><div class="section-title"><h2>收入与成本走势</h2><span>按结算日统计 · 赠送计入总成本</span></div><BusinessTrendChart :daily="overview.daily" /></section>
        <section class="surface">
          <div class="section-title"><h2>每日经营账</h2><span>成本补录会更新对应日期</span></div>
          <div class="table-scroll"><table><thead><tr><th>日期</th><th>余额消费</th><th>订阅折算</th><th>其他收入</th><th>实际成本</th><th>赠送成本</th><th>利润</th></tr></thead><tbody><tr v-for="day in visibleDays" :key="day.date"><td>{{ day.date }}</td><td>{{ money(day.balance_revenue) }}</td><td>{{ money(day.subscription_revenue) }}</td><td>{{ money(day.other_revenue) }}</td><td>{{ money(day.manual_cost) }}</td><td>{{ money(day.gift_cost) }}</td><td class="font-semibold" :class="{ negative: Number(day.profit) < 0 }">{{ money(day.profit) }}</td></tr></tbody><tfoot><tr><th>本期合计</th><td>{{ money(overview.summary.balance_revenue) }}</td><td>{{ money(overview.summary.subscription_revenue) }}</td><td>{{ money(overview.summary.other_revenue) }}</td><td>{{ money(overview.summary.manual_cost) }}</td><td>{{ money(overview.summary.gift_cost) }}</td><td>{{ money(overview.summary.profit) }}</td></tr></tfoot></table></div>
          <button v-if="overview.daily.length > dayLimit" class="load-more" @click="dayLimit += 31">显示更早日期</button>
        </section>
        <section class="surface">
          <div class="section-title"><h2>账号准备参考</h2><span>额度是服务需求，不是上游采购成本</span></div>
          <p class="help">通用用户余额不重复分配到各组；不同平台的额度不直接相加。日均与峰值取最近 7 个完整自然日。</p>
          <div v-if="overview.capacity.length" class="table-scroll"><table><thead><tr><th>平台 / 分组</th><th>有效订阅</th><th>当前可用额度</th><th>7 日日均消耗</th><th>7 日日峰值</th><th>7 日内到期</th></tr></thead><tbody><tr v-for="row in overview.capacity" :key="row.group_id"><td><b>{{ row.name }}</b><small>{{ row.platform || '未指定平台' }}</small></td><td>{{ row.active_subscriptions }}</td><td>{{ row.available === null ? '含不限量权益' : units(row.available) }}<small v-if="row.unlimited">已知有限额度 {{ units(row.known_available) }}</small></td><td>{{ units(row.seven_day_average) }}</td><td>{{ units(row.seven_day_peak) }}</td><td>{{ row.expiring_count }}</td></tr></tbody></table></div>
          <p v-else class="empty">暂无有效订阅或最近用量。开始产生消费后，这里会显示准备账号所需的参考。</p>
        </section>
        <details class="surface explanation"><summary>赠送明细与统计口径</summary><div class="reward-list"><span v-for="reward in overview.rewards" :key="reward.source">{{ rewardName(reward.source) }} <b>{{ money(reward.amount) }}</b></span></div><p>余额消费按 1:1 计收入；订阅按整期计价金额 × 消费比例确认。赠送发放时记成本，消费时计收入。充值和余额购买套餐不重复计收入。未消费额度退款不冲减已确认收入；已消费部分的退款请登记收入调整。</p><p>本期仅作收款记录的线下金额：{{ money(overview.manual_receipts) }}，不加入经营估算利润。经营估算利润与现金净流入不同。</p></details>
      </template>
    </template>

    <template v-if="tab === 'records'">
      <form v-if="recordOpen" class="surface entry-form" @submit.prevent="saveRecord">
        <div class="section-title"><h2>{{ recordForm.kind === 'cost' ? '记一笔成本' : '登记手工账目' }}</h2><button type="button" class="quiet-button" @click="recordOpen = false">收起</button></div>
        <div class="form-grid"><label>类型<select v-model="recordForm.kind"><option value="cost">实际成本</option><option value="income">其他已确认收入</option><option value="receipt">线下收款（不计利润）</option><option value="income_adjustment">收入调整（可负数）</option><option value="cost_adjustment">成本调整（可负数）</option></select></label><label>金额（人民币）<input v-model="recordForm.amount" data-test="record-amount" required type="number" step="0.00000001" :min="recordForm.kind.endsWith('adjustment') ? undefined : '0.00000001'" /></label><label>业务日期<input v-model="recordForm.date" required type="date" :min="openingDate" :max="today" /></label><label>类别<input v-model="recordForm.category" required maxlength="100" placeholder="例如：账号采购、服务收入" /></label></div>
        <label>备注<textarea v-model="recordForm.notes" rows="2" maxlength="2000" placeholder="补充用途或凭据，方便日后核对" /></label>
        <p class="help">{{ recordHelp }}</p><p v-if="formError" class="error" role="alert">{{ formError }}</p><button class="primary-button" :disabled="saving">{{ saving ? '保存中…' : '保存账目' }}</button>
      </form>
      <section class="surface"><div class="section-title"><h2>手工账目</h2><span>原始记录保留，更正通过冲销完成</span></div><p v-if="recordsError" class="error" role="alert">{{ recordsError }} <button @click="loadRecords(false)">重试</button></p>
        <p v-if="!records.length && !recordsError" class="empty">还没有手工账目。采购、服务器支出和独立的私下收入从这里登记。</p>
        <div v-else class="table-scroll"><table><thead><tr><th>业务日期 / 类型</th><th>类别与备注</th><th>金额</th><th>状态 / 操作</th></tr></thead><tbody><template v-for="record in records" :key="record.id"><tr><td>{{ record.date }}<small>{{ kindName(record.kind) }}</small></td><td class="notes-cell"><b>{{ record.category }}</b><small>{{ record.notes || '无备注' }}</small></td><td>{{ money(record.amount) }}</td><td><span v-if="record.reverses_id">冲销 #{{ record.reverses_id }}</span><span v-else-if="record.reversed" class="muted">已冲销</span><button v-else class="text-button" @click="reverseID = record.id; reverseReason = ''">冲销</button></td></tr><tr v-if="reverseID === record.id"><td colspan="4"><form class="reverse-form" @submit.prevent="reverseRecord(record.id)"><label>冲销原因<input v-model="reverseReason" required maxlength="2000" /></label><button class="secondary-button" :disabled="saving">确认冲销</button><button type="button" class="quiet-button" @click="reverseID = null">取消</button></form><p v-if="formError" class="error" role="alert">{{ formError }}</p></td></tr></template></tbody></table></div>
        <button v-if="moreRecords" class="load-more" :disabled="recordsLoading" @click="loadRecords(true)">{{ recordsLoading ? '加载中…' : '加载更多账目' }}</button>
      </section>
    </template>

    <template v-if="tab === 'settings'">
      <section class="surface opening">
        <div class="section-title"><h2>{{ settings?.enabled_at ? '期初已启用' : '一次设置，开始每日统计' }}</h2><span v-if="settings?.enabled_at">{{ timestamp(settings.enabled_at) }}</span></div>
        <p class="help">历史成本用一个总数接续。历史收入必须是已确认收入，不能直接填写累计充值或尚未消费的套餐收款。</p>
        <form @submit.prevent="initialize"><div class="form-grid"><label>历史系统已确认收入（元）<input v-model="openingForm.opening_revenue" type="number" min="0" step="0.00000001" required :disabled="!!settings?.enabled_at" /></label><label>未包含的历史私下收入（元）<input v-model="openingForm.opening_private_revenue" type="number" min="0" step="0.00000001" required :disabled="!!settings?.enabled_at" /></label><label>历史总成本（元）<input v-model="openingForm.opening_cost" type="number" min="0" step="0.00000001" required :disabled="!!settings?.enabled_at" /><small>含已统计的历史赠送，不会再次扣除</small></label><label>统计时区<select v-model="openingForm.timezone" :disabled="!!settings?.enabled_at"><option value="Asia/Shanghai">北京时间 · Asia/Shanghai</option><option value="UTC">UTC</option><option value="America/New_York">纽约 · America/New_York</option><option value="Europe/London">伦敦 · Europe/London</option></select></label></div>
          <p class="opening-total">期初估算利润 <strong>{{ money(openingProfit) }}</strong></p>
          <template v-if="!settings?.enabled_at"><label class="check-label"><input v-model="openingConfirmed" type="checkbox" required />我确认以上为同口径历史总数，不包含尚未消费的预收款。</label><p class="help">从保存时刻开始统计，今天只包含启用后的消费。基线固定后，以调整记录更正，不覆盖历史。</p><p v-if="formError" class="error" role="alert">{{ formError }}</p><button class="primary-button" :disabled="saving || !openingConfirmed || !settings">{{ saving ? '启用中…' : '从现在开始统计' }}</button></template>
        </form>
      </section>
      <section v-if="settings?.enabled_at" class="surface"><div class="section-title"><h2>订阅换算基准</h2><span>仅需补充系统无法确定的权益期</span></div><p class="help">整期收入按「计价金额 × 已消费额度 ÷ 本期总额度」计算。当前日/周窗口的百分比不能代替整期消费比例。</p><p v-if="termsError" class="error" role="alert">{{ termsError }} <button @click="loadTerms(false)">重试</button></p><p v-if="!terms.length && !termsError" class="empty">当前没有需要补充的订阅基准。新购买的可识别套餐会自动冻结成交金额与额度。</p>
        <article v-for="term in terms" :key="term.id" class="term-row"><div><h3>{{ term.plan_name }} <span class="muted">用户 #{{ term.user_id }}</span></h3><p>{{ term.group_name }} · {{ term.opening ? '期初存量' : '新权益期' }} · 到期 {{ timestamp(term.ends_at) }}</p><p>本期计价 {{ money(term.amount) }} · 折算总额度 {{ term.quota === null ? '待补充' : units(term.quota) }} · 已消费 {{ units(term.used_units) }}</p></div><button class="secondary-button" @click="editTerm(term)">补充基准</button>
          <form v-if="editingTerm?.id === term.id" class="term-form" @submit.prevent="saveTerm"><div class="form-grid"><label>本期总计价金额（元）<input v-model="termForm.amount" type="number" min="0" step="0.00000001" required :readonly="term.amount !== null" /></label><label>本期折算总额度<input v-model="termForm.quota" type="number" min="0.00000001" step="0.00000001" required :readonly="term.quota !== null" /></label><label>权益来源<select v-model="termForm.funding" required :disabled="term.funding === 'paid'"><option disabled value="">请选择</option><option value="paid">付费购买</option><option value="gift">赠送权益</option></select></label><label>确认依据<input v-model="termForm.notes" required maxlength="2000" placeholder="订单或赠送价值依据" /></label></div><p class="help">期初已消费 {{ units(term.baseline_units) }} 额度不会再记收入。已冻结的金额和额度不能覆盖。</p><p v-if="termError" class="error" role="alert">{{ termError }}</p><button class="primary-button" :disabled="saving">保存基准</button><button class="quiet-button" type="button" @click="editingTerm = null">取消</button></form>
        </article><button v-if="moreTerms" class="load-more" @click="loadTerms(true)">加载更多权益期</button>
      </section>
    </template>
  </section>
</template>
<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { businessAPI, type BusinessSettings, type BusinessOverview, type BusinessRecord, type BusinessRecordInput, type BusinessRecordKind, type BusinessTerm } from '@/api/admin/business'
import BusinessTrendChart from './BusinessTrendChart.vue'
const { t } = useI18n()
const op = (key: string) => t("admin.businessAnalytics.operating." + key)
const props = defineProps<{ startDate?: string; endDate?: string }>()
const emit = defineEmits<{ dates: [start: string, end: string] }>()
const tabs = computed(() => [{ key: 'overview', label: op('overview') }, { key: 'records', label: op('records') }, { key: 'settings', label: op('settings') }] as const)
const tab = ref<'overview' | 'records' | 'settings'>('overview')
const settings = ref<BusinessSettings | null>(null)
const overview = ref<BusinessOverview | null>(null)
const records = ref<BusinessRecord[]>([])
const terms = ref<BusinessTerm[]>([])
const loading = ref(false), saving = ref(false), recordsLoading = ref(false)
const settingsError = ref(''), overviewError = ref(''), recordsError = ref(''), termsError = ref(''), formError = ref(''), termError = ref('')
const moreRecords = ref(false), moreTerms = ref(false), dayLimit = ref(31)
const recordOpen = ref(false), openingConfirmed = ref(false)
const reverseID = ref<number | null>(null), reverseReason = ref('')
const editingTerm = ref<BusinessTerm | null>(null)
const openingForm = ref({ timezone: 'Asia/Shanghai', opening_revenue: '0', opening_private_revenue: '0', opening_cost: '0' })
const termForm = ref({ amount: '', quota: '', funding: '' as '' | 'paid' | 'gift', notes: '' })
const dateInZone = (value: Date, zone = settings.value?.timezone || 'Asia/Shanghai') => new Intl.DateTimeFormat('en-CA', {timeZone: zone, year: 'numeric', month: '2-digit', day: '2-digit'}).format(value)
const today = computed(() => dateInZone(new Date()))
const openingDate = computed(() => settings.value?.enabled_at ? dateInZone(new Date(settings.value.enabled_at)) : '')
const recordForm = ref<BusinessRecordInput>({ source_key: '', kind: 'cost', amount: '', date: '', category: '', notes: '' })
const money = (value: string | number | null) => value === null ? '待确认' : new Intl.NumberFormat('zh-CN', {style: 'currency', currency: 'CNY', minimumFractionDigits: 2, maximumFractionDigits: 2}).format(Number(value))
const units = (value: string) => new Intl.NumberFormat('zh-CN', {maximumFractionDigits: 2}).format(Number(value))
const timestamp = (value: string) => new Intl.DateTimeFormat('zh-CN', {timeZone: settings.value?.timezone || 'Asia/Shanghai', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit'}).format(new Date(value))
const openingProfit = computed(() => Number(openingForm.value.opening_revenue) + Number(openingForm.value.opening_private_revenue) - Number(openingForm.value.opening_cost))
const visibleDays = computed(() => [...(overview.value?.daily || [])].reverse().slice(0, dayLimit.value))
const kindName = (kind: BusinessRecordKind) => ({ cost: '实际成本', income: '其他收入', receipt: '仅收款', income_adjustment: '收入调整', cost_adjustment: '成本调整' })[kind]
const rewardName = (source: string) => ({daily_checkin: '每日签到', usage_rebate: '用量返利', affiliate: '邀请返利', promo: '活动赠送', registration: '注册赠送', qq_binding_reward: '绑定奖励', admin_gift: '管理员赠送', lottery_reward: '抽奖奖励', qq_bind_welcome_bonus: 'QQ 绑定奖励', recharge_bonus: '充值赠送', balance: '抽奖兑换额度'} as Record<string, string>)[source] || source
const recordHelp = computed(() => recordForm.value.kind === 'income' ? '仅登记已确认、与站内余额或套餐消费不重复的收入。线下充值请选择“线下收款”。' : recordForm.value.kind === 'receipt' ? '仅记录线下收到的钱，不增加利润、不自动给用户充值。用户消费时再确认收入。' : recordForm.value.kind.endsWith('adjustment') ? '正数增加该项，负数冲减。退款未消费余额不应再次冲减已确认收入。' : '按所选业务日期计入实际成本。系统自动统计的赠送无需重复录入。')
const errorText = (e: unknown) => {const v = e as {response?: {data?: {message?: string}}, message?: string}; return v.response?.data?.message || v.message || '操作失败，请重试'}
let overviewRequest = 0
async function loadOverview() {
 const request = ++overviewRequest; loading.value = true; overviewError.value = ''
 try { const data = await businessAPI.overview({start_date: props.startDate || undefined, end_date: props.endDate || undefined}); if (request === overviewRequest) { overview.value = data; emit('dates', data.start_date, data.end_date) } }
 catch(e) { if (request === overviewRequest) overviewError.value = errorText(e) }
 finally { if (request === overviewRequest) loading.value = false }
}
async function loadSettings() {
 settingsError.value = ''
 try { settings.value = await businessAPI.settings(); if (settings.value.enabled_at) { const {timezone, opening_revenue, opening_private_revenue, opening_cost} = settings.value; openingForm.value = {timezone, opening_revenue, opening_private_revenue, opening_cost} } }
 catch(e) { settingsError.value = errorText(e) }
}
async function loadRecords(append: boolean) {
 if (recordsLoading.value) return
 recordsLoading.value = true; recordsError.value = ''
 try { const next = await businessAPI.records(append ? records.value.at(-1)?.id : 0); records.value = append ? [...records.value, ...next] : next; moreRecords.value = next.length === 50 }
 catch(e) { recordsError.value = errorText(e) }
 finally { recordsLoading.value = false }
}
async function loadTerms(append: boolean) {
 termsError.value = ''
 try { const next = await businessAPI.terms(append ? terms.value.at(-1)?.id : 0); terms.value = append ? [...terms.value, ...next] : next; moreTerms.value = next.length === 50 }
 catch(e) { termsError.value = errorText(e) }
}
async function refresh() { await Promise.all([loadSettings(), loadOverview(), loadRecords(false), loadTerms(false)]) }
function startRecord(kind: BusinessRecordKind) { tab.value = 'records'; recordOpen.value = true; formError.value = ''; recordForm.value = {source_key: crypto.randomUUID(), kind, amount: '', date: today.value, category: '', notes: ''} }
async function saveRecord() {
 if (saving.value) return
 saving.value = true; formError.value = ''
 try {await businessAPI.record({...recordForm.value, amount: String(recordForm.value.amount)}); recordOpen.value = false; await Promise.all([loadRecords(false),loadOverview()])} catch(e) {formError.value = errorText(e)} finally {saving.value = false}
}
async function reverseRecord(id: number) {
 if (saving.value) return
 saving.value = true; formError.value = ''
 try {await businessAPI.reverse(id, reverseReason.value); reverseID.value = null; await Promise.all([loadRecords(false),loadOverview()])} catch(e) {formError.value = errorText(e)} finally {saving.value = false}
}
async function initialize() {
 if (saving.value || !openingConfirmed.value) return
 saving.value = true; formError.value = ''
 try {await businessAPI.initialize({...openingForm.value, opening_revenue: String(openingForm.value.opening_revenue), opening_private_revenue: String(openingForm.value.opening_private_revenue), opening_cost: String(openingForm.value.opening_cost)}); await refresh(); tab.value = 'overview'} catch(e) {formError.value = errorText(e)} finally {saving.value = false}
}
function editTerm(term: BusinessTerm) {editingTerm.value = term; termError.value = ''; termForm.value = {amount: term.amount || '', quota: term.quota || '', funding: term.funding === 'unknown' ? '' : term.funding, notes: ''}}
async function saveTerm() {
 if (!editingTerm.value || !termForm.value.funding || saving.value) return
 saving.value = true; termError.value = ''
 try {await businessAPI.configureTerm(editingTerm.value.id, {...termForm.value, amount: String(termForm.value.amount), quota: String(termForm.value.quota), funding: termForm.value.funding}); editingTerm.value = null; await Promise.all([loadTerms(false),loadOverview()])} catch(e) {termError.value = errorText(e)} finally {saving.value = false}
}
watch(() => [props.startDate, props.endDate], () => {dayLimit.value = 31; void loadOverview()})
onMounted(refresh)
defineExpose({refresh})
</script>
<style scoped>
.operating { @apply space-y-5 text-gray-800 dark:text-gray-200; font-variant-numeric: tabular-nums; }
.toolbar { @apply flex flex-wrap items-center justify-between gap-4; }
.tabs { @apply flex max-w-full gap-1 overflow-x-auto rounded-xl bg-gray-100 p-1 dark:bg-dark-900; }
.tabs button { @apply whitespace-nowrap rounded-lg px-4 py-2 text-sm font-medium text-gray-500 transition-colors hover:text-gray-900 dark:text-gray-400 dark:hover:text-gray-100; }
.tabs button.selected { @apply bg-white text-gray-900 shadow-sm dark:bg-dark-700 dark:text-white; }
.actions { @apply flex flex-wrap items-center gap-2; }
.primary-button,.secondary-button,.quiet-button { @apply inline-flex items-center justify-center whitespace-nowrap rounded-lg px-4 py-2 text-sm font-medium transition-colors disabled:cursor-not-allowed disabled:opacity-50; }
.primary-button { @apply bg-primary-600 text-white hover:bg-primary-700; }
.secondary-button { @apply border border-gray-200 bg-white text-gray-700 hover:bg-gray-50 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-200 dark:hover:bg-dark-700; }
.quiet-button { @apply text-gray-500 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-dark-700; }
button:focus-visible,summary:focus-visible { @apply outline-none ring-2 ring-primary-500 ring-offset-2 dark:ring-offset-dark-900; }
.text-button,.load-more { @apply text-sm font-medium text-primary-600 dark:text-primary-400; }
.load-more { @apply block w-full py-4 text-center hover:bg-gray-50 dark:hover:bg-dark-700; }
.muted,.help { @apply text-gray-500 dark:text-gray-400; }
.help { @apply mb-4 max-w-4xl text-sm leading-relaxed; }
.period-heading { @apply flex flex-wrap justify-between gap-2 text-xs; }
.metric-grid { @apply grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-4; }
.metric { @apply rounded-xl border border-gray-200 bg-white p-5 dark:border-dark-700 dark:bg-dark-800; min-width: 0; }
.profit-metric { @apply border-primary-200 bg-primary-50/50 dark:border-primary-800 dark:bg-primary-950/20; }
.metric-label { @apply text-sm font-medium text-gray-600 dark:text-gray-300; }
.metric strong { @apply my-3 block text-3xl font-semibold tracking-tight text-gray-950 dark:text-white; overflow-wrap: anywhere; }
.metric p,.metric small { @apply text-xs leading-5 text-gray-500 dark:text-gray-400; }
.negative { @apply !text-red-600 dark:!text-red-400; }
.cumulative { @apply flex flex-wrap items-baseline gap-x-4 gap-y-2 px-1 text-sm; }
.cumulative small { @apply ml-2 text-xs text-gray-500; }
.cumulative strong { @apply text-lg font-semibold; }
.surface { @apply overflow-hidden rounded-xl border border-gray-200 bg-white p-5 dark:border-dark-700 dark:bg-dark-800 sm:p-6; }
.section-title { @apply mb-5 flex flex-wrap items-center justify-between gap-2; }
.section-title h2 { @apply text-base font-semibold tracking-tight; }
.section-title>span { @apply text-xs text-gray-500 dark:text-gray-400; }
.position-grid { @apply grid gap-5 sm:grid-cols-3; }
.position-grid>div { @apply space-y-2; }
.position-grid span { @apply text-sm text-gray-600 dark:text-gray-400; }
.position-grid strong { @apply block text-2xl font-semibold tracking-tight; }
.position-grid small { @apply block text-xs text-gray-500 dark:text-gray-400; }
.position-foot { @apply mt-5 flex flex-wrap gap-4 border-t border-gray-100 pt-4 text-xs text-gray-500 dark:border-dark-700 dark:text-gray-400; }
.table-scroll { @apply -mx-5 overflow-x-auto sm:-mx-6; }
table { @apply w-full border-collapse whitespace-nowrap text-sm; }
th { @apply bg-gray-50/80 px-5 py-3 text-right text-xs font-medium text-gray-500 dark:bg-dark-900/40 dark:text-gray-400; }
td { @apply border-b border-gray-100 px-5 py-3.5 text-right dark:border-dark-700/70; }
th:first-child,td:first-child { @apply text-left; }
tbody tr:hover { @apply bg-gray-50/70 dark:bg-dark-700/25; }
td small { @apply mt-1 block text-xs text-gray-500 dark:text-gray-400; }
td b { @apply font-medium; }
tfoot td,tfoot th { @apply border-0 bg-gray-50 py-4 font-semibold dark:bg-dark-900/50; }
.notes-cell { @apply max-w-xs text-left; white-space: normal; overflow-wrap: anywhere; }
.empty { @apply px-3 py-9 text-center text-sm leading-relaxed text-gray-500 dark:text-gray-400; }
.notice { @apply flex flex-wrap items-center justify-between gap-3 rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-900 dark:border-amber-800 dark:bg-amber-950/20 dark:text-amber-200; }
.notice button { @apply font-semibold; }
.error { @apply my-3 rounded-lg bg-red-50 px-4 py-3 text-sm text-red-700 dark:bg-red-950/20 dark:text-red-300; }
.error button { @apply ml-2 underline; }
.welcome { @apply rounded-xl border border-primary-200 bg-primary-50/40 px-6 py-9 dark:border-primary-800 dark:bg-primary-950/20; }
.welcome-label { @apply text-xs font-medium text-primary-700 dark:text-primary-300; }
.welcome h2 { @apply my-3 text-xl font-semibold tracking-tight; }
.welcome p { @apply mb-5 max-w-2xl text-sm leading-relaxed text-gray-500 dark:text-gray-400; }
.form-grid { @apply mb-4 grid gap-4 sm:grid-cols-2 xl:grid-cols-4; }
label { @apply block text-sm font-medium; }
input:not([type='checkbox']),select,textarea { @apply mt-2 block w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm font-normal text-gray-900 outline-none focus:border-primary-500 focus:ring-2 focus:ring-primary-200 disabled:bg-gray-50 disabled:text-gray-500 dark:border-dark-600 dark:bg-dark-900 dark:text-gray-100 dark:focus:ring-primary-800; }
input[readonly] { @apply bg-gray-50 text-gray-500 dark:bg-dark-800; }
label small { @apply mt-2 block text-xs font-normal text-gray-500; }
.entry-form>.help { @apply mt-3; }
.check-label { @apply my-4 flex items-start gap-3 font-normal leading-relaxed; }
.check-label input { @apply mt-1 accent-primary-600; }
.opening-total { @apply my-5 flex items-baseline gap-4 text-sm; }
.opening-total strong { @apply text-2xl font-semibold; }
.term-row { @apply flex flex-wrap items-center justify-between gap-4 border-t border-gray-100 py-5 dark:border-dark-700; }
.term-row h3 { @apply text-sm font-medium; }
.term-row p { @apply mt-2 text-xs leading-relaxed text-gray-500 dark:text-gray-400; }
.term-form { @apply w-full rounded-lg bg-gray-50 p-4 dark:bg-dark-900; }
.reverse-form { @apply flex flex-wrap items-end gap-3 text-left; }
.reverse-form label { @apply min-w-48 flex-1; }
.explanation summary { @apply cursor-pointer text-sm font-medium; }
.explanation p { @apply mt-3 text-xs leading-6 text-gray-500 dark:text-gray-400; }
.reward-list { @apply mt-4 flex flex-wrap gap-x-6 gap-y-2 text-sm; }
.reward-list b { @apply ml-2 font-medium; }
.skeleton { @apply h-40 animate-pulse rounded-xl bg-gray-100 dark:bg-dark-800; }
@media(prefers-reduced-motion:reduce) { .skeleton { animation:none; } button { transition:none; } }
</style>
