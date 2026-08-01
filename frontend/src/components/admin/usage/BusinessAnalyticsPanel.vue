<template>
  <div class="business-panel">
    <div v-if="loading" class="business-panel__loading"><LoadingSpinner /><span>正在汇总经营数据</span></div>

    <template v-else-if="overview">
      <div class="business-toolbar">
        <nav class="business-subnav" aria-label="经营分析二级菜单">
          <button
            v-for="item in businessViews"
            :key="item.key"
            type="button"
            :class="activeBusinessView === item.key ? 'business-subnav__item--active' : ''"
            :aria-current="activeBusinessView === item.key ? 'page' : undefined"
            @click="activeBusinessView = item.key"
          >
            <Icon :name="item.icon" size="sm" />
            <span>{{ item.label }}</span>
          </button>
        </nav>
        <span v-if="overview.snapshot_captured_at" class="business-panel__snapshot">承载快照 {{ formatDateTime(overview.snapshot_captured_at) }}</span>
      </div>

      <div v-if="!overview.profit_complete" class="business-data-warning">
        <Icon name="exclamationTriangle" size="sm" />
        <span>{{ usd(overview.unpriced_api_key_usage_cost_usd) }} API Key 计价分待定价</span>
        <button type="button" @click="openRateDialog">配置比例</button>
      </div>

      <div v-if="activeBusinessView === 'overview'" class="business-view">
        <div class="business-cumulative" :class="overview.cumulative.operating_profit_cny >= 0 ? 'business-cumulative--positive' : 'business-cumulative--negative'">
          <div class="business-cumulative__result">
            <span>{{ overview.cumulative.profit_complete ? '累计经营利润' : '累计已定价利润' }}</span>
            <strong>{{ cny(overview.cumulative.operating_profit_cny) }}</strong>
            <small>{{ formatDate(overview.cumulative.start_at) }} - {{ formatCostEndDate(overview.cumulative.end_at) }}</small>
          </div>
          <div><span>累计收入</span><strong>{{ cny(overview.cumulative.usage_revenue_cny) }}</strong></div>
          <div><span>累计成本</span><strong>{{ cny(overview.cumulative.total_cost_cny) }}</strong></div>
          <div><span>累计利润率</span><strong>{{ percent(overview.cumulative.operating_margin) }}</strong></div>
        </div>
        <div v-if="!overview.cumulative.profit_complete" class="business-inline-warning">累计仍有 {{ usd(overview.cumulative.unpriced_api_key_usage_cost_usd) }} 分待定价</div>

        <div class="business-rate-strip">
          <span>收入换算 <strong>¥1 = {{ decimal(overview.settings.balance_credits_per_cny, 2) }} 分</strong></span>
          <span>API Key 比例 <strong>{{ configuredAccountCount }} / {{ rateConfig.accounts.length }}</strong></span>
          <span>扣费分数 <strong>{{ usd(overview.usage_credits_usd) }}</strong></span>
        </div>

        <section class="business-section business-period">
          <div class="business-section__title"><span>本期经营</span></div>
          <div class="business-metrics">
            <div class="business-metric business-metric--revenue"><span>折算收入</span><strong>{{ cny(overview.usage_revenue_cny) }}</strong><small>本区间</small></div>
            <div class="business-metric business-metric--amber"><span>API Key 成本</span><strong>-{{ cny(overview.api_key_usage_cost_cny) }}</strong><small>{{ usd(overview.api_key_usage_cost_usd) }} 分</small></div>
            <div class="business-metric business-metric--amber"><span>福利发放</span><strong>-{{ cny(overview.welfare_cost_cny) }}</strong><small>{{ usd(overview.welfare_granted_usd) }} 分</small></div>
            <div class="business-metric business-metric--amber"><span>OAuth / 固定</span><strong>{{ overview.cost_ledger_configured ? `-${cny(overview.account_cost_cny)}` : '待录入' }}</strong><small>本区间摊销</small></div>
            <div class="business-metric" :class="overview.gross_profit_cny >= 0 ? 'business-metric--green' : 'business-metric--red'"><span>用量毛利</span><strong>{{ cny(overview.gross_profit_cny) }}</strong><small>{{ percent(overview.gross_margin) }}</small></div>
            <div class="business-metric" :class="overview.operating_profit_cny >= 0 ? 'business-metric--green' : 'business-metric--red'"><span>{{ overview.profit_complete ? '区间利润' : '已定价利润' }}</span><strong>{{ cny(overview.operating_profit_cny) }}</strong><small>{{ percent(overview.operating_margin) }}</small></div>
          </div>
        </section>

        <section class="business-section business-visuals">
          <BusinessAnalyticsCharts :daily="overview.daily" :groups="overview.groups" />
        </section>

        <section class="business-section business-section--last">
          <div class="business-section__title"><span>实际现金流</span></div>
          <div v-if="overview.cash_receipts.length" class="business-cash-list">
            <div v-for="item in overview.cash_receipts" :key="item.currency"><span>{{ item.currency }}</span><strong>{{ money(item.amount, item.currency) }}</strong></div>
          </div>
          <p v-else class="business-empty">当前区间没有外部支付收款</p>
        </section>
      </div>

      <div v-else-if="activeBusinessView === 'profit'" class="business-view">
        <section class="business-section">
          <div class="business-section__title"><span>每日利润对账</span></div>
          <div class="business-table-wrap">
            <table class="business-table business-table--daily">
              <thead><tr><th>日期</th><th>收入</th><th>API Key</th><th>福利</th><th>OAuth / 固定</th><th>经营利润</th></tr></thead>
              <tbody>
                <tr v-for="day in overview.daily" :key="day.date">
                  <td><strong>{{ day.date }}</strong></td>
                  <td>{{ cny(day.usage_revenue_cny) }}</td>
                  <td class="business-table__muted"><strong>{{ cny(day.api_key_usage_cost_cny) }}</strong><small v-if="day.unpriced_api_key_usage_cost_usd" class="business-table__warning">{{ usd(day.unpriced_api_key_usage_cost_usd) }} 待定价</small></td>
                  <td class="business-table__muted">{{ cny(day.welfare_cost_cny) }}</td>
                  <td class="business-table__muted">{{ cny(day.account_cost_cny) }}</td>
                  <td :class="day.operating_profit_cny >= 0 ? 'business-table__positive' : 'business-table__negative'"><strong>{{ cny(day.operating_profit_cny) }}</strong></td>
                </tr>
                <tr v-if="!overview.daily.length"><td colspan="6" class="business-empty">当前区间暂无经营数据</td></tr>
              </tbody>
            </table>
          </div>
        </section>

        <section class="business-section business-section--last">
          <div class="business-section__title"><span>分组盈利</span></div>
          <div class="business-table-wrap">
            <table class="business-table business-table--profit-groups">
              <thead><tr><th>分组</th><th>扣费倍率</th><th>收入</th><th>API Key</th><th>福利</th><th>OAuth / 固定</th><th>经营利润</th></tr></thead>
              <tbody>
                <tr v-for="group in overview.groups" :key="group.group_id">
                  <td><strong>{{ group.group_name || `分组 #${group.group_id}` }}</strong></td>
                  <td><strong>{{ decimal(group.effective_rate_multiplier, 3) }}x</strong><small>{{ usd(group.usage_credits_usd) }} 分</small></td>
                  <td><strong>{{ cny(group.usage_revenue_cny) }}</strong></td>
                  <td class="business-table__muted"><strong>{{ cny(group.api_key_usage_cost_cny) }}</strong><small v-if="group.unpriced_api_key_usage_cost_usd" class="business-table__warning">{{ usd(group.unpriced_api_key_usage_cost_usd) }} 待定价</small></td>
                  <td class="business-table__muted">{{ cny(group.allocated_welfare_cost_cny) }}</td>
                  <td class="business-table__muted">{{ cny(group.allocated_account_cost_cny) }}</td>
                  <td :class="group.operating_profit_cny >= 0 ? 'business-table__positive' : 'business-table__negative'"><strong>{{ cny(group.operating_profit_cny) }}</strong><small>{{ percent(group.gross_margin) }} 毛利率</small></td>
                </tr>
                <tr v-if="!overview.groups.length"><td colspan="7" class="business-empty">当前区间暂无分组数据</td></tr>
              </tbody>
            </table>
          </div>
        </section>
      </div>

      <div v-else-if="activeBusinessView === 'capacity'" class="business-view">
        <section class="business-section">
          <div class="business-section__title">
            <span>待履约权益</span>
            <button type="button" class="btn btn-secondary" :disabled="snapshotting" @click="captureSnapshot"><Icon name="refresh" size="sm" :class="snapshotting ? 'animate-spin' : ''" />刷新承载</button>
          </div>
          <div class="business-risk-grid">
            <div><span>余额计价分</span><strong>{{ usd(overview.liabilities.balance_credits_usd) }}</strong><small>{{ cny(overview.liabilities.balance_face_value_cny) }} 面值</small></div>
            <div><span>余额履约成本</span><strong>{{ cny(overview.liabilities.balance_estimated_cost_cny) }}</strong><small>含冻结余额</small></div>
            <div><span>订阅履约成本</span><strong>{{ cny(overview.liabilities.subscription_estimated_cost_cny) }}</strong><small>{{ overview.liabilities.active_subscriptions }} 个订阅</small></div>
            <div :class="overview.liabilities.unlimited_subscriptions ? 'business-risk-grid__alert' : ''"><span>无限订阅</span><strong>{{ overview.liabilities.unlimited_subscriptions }}</strong><small>需预测储备</small></div>
          </div>
        </section>

        <section class="business-section business-section--last">
          <div class="business-section__title"><span>分组容量预测</span></div>
          <div class="business-table-wrap">
            <table class="business-table business-table--capacity">
              <thead><tr><th>分组</th><th>P50 日需求</th><th>P95 日需求</th><th>单账号日承载</th><th>账号池</th><th>预测需要</th><th>需补账号</th></tr></thead>
              <tbody>
                <tr v-for="group in overview.groups" :key="group.group_id">
                  <td><strong>{{ group.group_name || `分组 #${group.group_id}` }}</strong><small>{{ group.concurrency_max }} 并发</small></td>
                  <td>{{ usd(group.forecast_p50_daily_cost_usd) }}</td>
                  <td><strong>{{ usd(group.forecast_p95_daily_cost_usd) }}</strong></td>
                  <td>{{ group.observed_capacity_per_account > 0 ? usd(group.observed_capacity_per_account) : '-' }}</td>
                  <td>{{ group.schedulable_accounts }} 个</td>
                  <td>{{ group.required_accounts }} 个</td>
                  <td :class="group.additional_accounts > 0 ? 'business-table__negative' : 'business-table__positive'"><strong>{{ group.additional_accounts > 0 ? `+${group.additional_accounts}` : '充足' }}</strong></td>
                </tr>
                <tr v-if="!overview.groups.length"><td colspan="7" class="business-empty">等待分组用量与承载数据</td></tr>
              </tbody>
            </table>
          </div>
        </section>
      </div>

      <div v-else class="business-view business-view--costs">
        <div class="business-config-actions">
          <button type="button" class="btn btn-secondary" @click="openRateDialog"><Icon name="cog" size="sm" />API Key 比例</button>
          <button type="button" class="btn btn-primary" @click="openCostDialog"><Icon name="plus" size="sm" />录入固定成本</button>
        </div>

        <section class="business-section">
          <div class="business-section__title"><span>API Key 分数兑换比例</span></div>
          <div class="business-table-wrap">
            <table class="business-table business-table--compact">
              <thead><tr><th>API Key 账号</th><th>分数兑换比例</th><th>备注</th><th></th></tr></thead>
              <tbody>
                <tr v-for="rate in rateConfig.rates" :key="rate.id"><td><strong>{{ rateAccount(rate.account_id)?.name || `账号 #${rate.account_id}` }}</strong><small>{{ rateAccount(rate.account_id)?.platform || '-' }}</small></td><td><strong>{{ creditsPerCNY(rate.credits_per_cny) }}</strong></td><td class="business-table__muted">{{ rate.notes || '-' }}</td><td><button type="button" class="business-icon-button" title="删除分数比例记录" @click="removeRate(rate.id)"><Icon name="trash" size="sm" /></button></td></tr>
                <tr v-if="!rateConfig.rates.length"><td colspan="4" class="business-empty">尚未配置 API Key 比例</td></tr>
              </tbody>
            </table>
          </div>
        </section>

        <section class="business-section business-section--last">
          <div class="business-section__title"><span>OAuth / 固定成本台账</span></div>
          <div class="business-table-wrap">
            <table class="business-table business-table--compact">
              <thead><tr><th>类型</th><th>金额</th><th>归属</th><th>有效期</th><th>备注</th><th></th></tr></thead>
              <tbody>
                <tr v-for="cost in costs" :key="cost.id"><td>{{ costType(cost.cost_type) }}</td><td><strong>{{ money(cost.amount, cost.currency) }}</strong><small>汇率 {{ decimal(cost.fx_rate || 1, 4) }}</small></td><td>{{ owner(cost) }}</td><td>{{ formatDate(cost.starts_at) }} - {{ formatCostEndDate(cost.ends_at) }}</td><td class="business-table__muted">{{ cost.notes || '-' }}</td><td><button type="button" class="business-icon-button" title="删除成本记录" @click="removeCost(cost.id)"><Icon name="trash" size="sm" /></button></td></tr>
                <tr v-if="!costs.length"><td colspan="6" class="business-empty">尚未录入固定成本</td></tr>
              </tbody>
            </table>
          </div>
        </section>
      </div>
    </template>

    <div v-else class="business-panel__loading"><span>经营分析暂时无法加载。</span><button class="btn btn-secondary" @click="load">重新加载</button></div>
  </div>

  <BaseDialog :show="rateDialogOpen" title="配置 API Key 分数兑换比例" width="normal" @close="rateDialogOpen = false">
    <form class="space-y-4" @submit.prevent="saveRate">
      <div class="business-dialog-note">
        <span>成本计算口径</span>
        <strong>人民币成本 = API Key 账号计价分 ÷ 每人民币计价分</strong>
        <small>这里的 $ 是平台计价分，不是法币美元。用户扣费倍率已经包含在收入中，不再参与 API Key 成本计算。</small>
      </div>
      <div>
        <label class="input-label">API Key 账号</label>
        <select v-model="rateForm.account_id" required class="input" @change="syncRateFormForAccount">
          <option value="" disabled>选择 API Key 账号</option>
          <option v-for="account in rateConfig.accounts" :key="account.id" :value="String(account.id)">
            {{ account.name }} · {{ account.platform }}
          </option>
        </select>
      </div>
      <div>
        <label class="input-label">每 1 元人民币可兑换的计价分</label>
        <input v-model.number="rateForm.credits_per_cny" required min="0.000001" step="any" type="number" class="input" placeholder="通常填 1，也可能填 10" />
        <p class="business-dialog-help">填 1 表示 ¥1 = $1 计价分；填 10 表示 ¥1 = $10 计价分。只应用于所选 API Key。</p>
      </div>
      <div><label class="input-label">备注</label><textarea v-model="rateForm.notes" class="input min-h-20" maxlength="1000" placeholder="例如采购渠道、账单批次" /></div>
      <div class="business-dialog-warning">重复保存同一账号会更新当前比例，并按新比例重新计算该账号的 API Key 用量成本。未配置比例的账号会标记为“待定价”，不会套用其他账号的比例。</div>
      <div class="flex justify-end gap-2"><button type="button" class="btn btn-secondary" @click="rateDialogOpen = false">取消</button><button type="submit" class="btn btn-primary" :disabled="savingRate || !rateConfig.accounts.length">{{ savingRate ? '保存中' : '保存比例' }}</button></div>
    </form>
  </BaseDialog>

  <BaseDialog :show="costDialogOpen" title="录入 OAuth / 固定成本" width="normal" @close="costDialogOpen = false">
    <form class="space-y-4" @submit.prevent="saveCost">
      <div class="grid grid-cols-1 gap-4 sm:grid-cols-2"><div><label class="input-label">成本类型</label><select v-model="costForm.cost_type" class="input"><option value="oauth_subscription">OAuth 包月 / 预付</option><option value="purchase">账号采购</option><option value="renewal">账号续费</option><option value="proxy">代理费用</option><option value="other">其他固定支出</option></select></div><div><label class="input-label">金额</label><input v-model.number="costForm.amount" required min="0.000001" step="any" type="number" class="input" /></div></div>
      <div class="grid grid-cols-1 gap-4 sm:grid-cols-3"><div><label class="input-label">币种</label><input v-model="costForm.currency" required maxlength="12" class="input" /></div><div><label class="input-label">兑换人民币汇率</label><input v-model.number="costForm.fx_rate" required min="0.000001" step="any" type="number" class="input" /></div><div><label class="input-label">归属分组 ID（可选）</label><input v-model="costForm.group_id" min="1" type="number" class="input" placeholder="留空按收入分摊" /></div></div>
      <div><label class="input-label">OAuth 账号 ID（可选）</label><input v-model="costForm.account_id" min="1" type="number" class="input" placeholder="留空表示共享固定成本" /></div>
      <div class="grid grid-cols-1 gap-4 sm:grid-cols-2"><div><label class="input-label">开始日期</label><input v-model="costForm.starts_at" required type="date" class="input" /></div><div><label class="input-label">结束日期（含）</label><input v-model="costForm.ends_at" required type="date" class="input" /></div></div>
      <div><label class="input-label">备注</label><textarea v-model="costForm.notes" class="input min-h-20" maxlength="1000" /></div>
      <div class="business-dialog-warning">API Key 账号按量成本已根据账号计价分与兑换比例自动计算，不能在此重复录入。OAuth 包月预付请填写覆盖的完整有效期。</div>
      <div class="flex justify-end gap-2"><button type="button" class="btn btn-secondary" @click="costDialogOpen = false">取消</button><button type="submit" class="btn btn-primary" :disabled="saving">{{ saving ? '保存中' : '保存成本' }}</button></div>
    </form>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useAppStore } from '@/stores/app'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import BusinessAnalyticsCharts from './BusinessAnalyticsCharts.vue'
import {
  captureBusinessCapacitySnapshot,
  createBusinessAPIKeyCostRate,
  createBusinessCost,
  deleteBusinessAPIKeyCostRate,
  deleteBusinessCost,
  getBusinessAPIKeyCostRates,
  getBusinessAnalytics,
  listBusinessCosts,
  type BusinessAccountCost,
  type BusinessAPIKeyAccount,
  type BusinessAPIKeyCostRateConfig,
  type BusinessAnalyticsOverview,
} from '@/api/admin/dashboard'

const props = defineProps<{ startDate: string; endDate: string }>()
const appStore = useAppStore()
const loading = ref(false)
const snapshotting = ref(false)
const saving = ref(false)
const savingRate = ref(false)
const overview = ref<BusinessAnalyticsOverview | null>(null)
const costs = ref<BusinessAccountCost[]>([])
const rateConfig = ref<BusinessAPIKeyCostRateConfig>({ accounts: [], rates: [] })
const costDialogOpen = ref(false)
const rateDialogOpen = ref(false)
type BusinessView = 'overview' | 'profit' | 'capacity' | 'costs'
const activeBusinessView = ref<BusinessView>('overview')
const businessViews = [
  { key: 'overview', label: '总览', icon: 'chartBar' },
  { key: 'profit', label: '利润明细', icon: 'document' },
  { key: 'capacity', label: '资源承载', icon: 'server' },
  { key: 'costs', label: '成本配置', icon: 'cog' },
] as const
const rateForm = ref({ account_id: '', credits_per_cny: 1, notes: '' })
const costForm = ref({ cost_type: 'oauth_subscription', amount: 0, currency: 'CNY', fx_rate: 1, group_id: '', account_id: '', starts_at: '', ends_at: '', notes: '' })

const configuredAccountCount = computed(() => new Set(rateConfig.value.rates.map((rate) => rate.account_id)).size)

const cny = (value: number) => `¥${Number(value || 0).toFixed(2)}`
const usd = (value: number) => `$${Number(value || 0).toFixed(2)}`
const decimal = (value: number, digits: number) => Number(value || 0).toFixed(digits)
const percent = (value: number) => `${Number(value || 0).toFixed(1)}%`
const money = (value: number, currency: string) => `${currency} ${Number(value || 0).toFixed(2)}`
const creditsPerCNY = (value: number) => `¥1 : $${Number(value || 0).toFixed(4)}`
const formatDate = (value: string) => value ? new Date(value).toLocaleDateString() : '-'
const formatCostEndDate = (value: string) => value ? new Date(new Date(value).getTime() - 1).toLocaleDateString() : '-'
const formatDateTime = (value: string) => value ? new Date(value).toLocaleString() : '-'
const rateAccount = (accountID: number): BusinessAPIKeyAccount | undefined => rateConfig.value.accounts.find((account) => account.id === accountID)
const owner = (cost: BusinessAccountCost) => cost.group_id ? `分组 #${cost.group_id}${cost.account_id ? ` / 账号 #${cost.account_id}` : ''}` : cost.account_id ? `账号 #${cost.account_id}（按收入分摊）` : '按收入分摊'
const costType = (value: string) => ({ oauth_subscription: 'OAuth 包月 / 预付', purchase: '账号采购', renewal: '账号续费', proxy: '代理费用', other: '其他固定支出', upstream_invoice: '上游账单（历史）' }[value] || value)

async function load() {
  loading.value = true
  try {
    const [nextOverview, nextCosts, nextRateConfig] = await Promise.all([
      getBusinessAnalytics({ start_date: props.startDate, end_date: props.endDate }),
      listBusinessCosts(),
      getBusinessAPIKeyCostRates(),
    ])
    overview.value = nextOverview
    costs.value = nextCosts
    rateConfig.value = nextRateConfig
  } catch (error) {
    console.error('Failed to load business analytics:', error)
    appStore.showError('经营分析加载失败')
  } finally { loading.value = false }
}

function openRateDialog() {
  rateForm.value = {
    account_id: rateConfig.value.accounts.length === 1 ? String(rateConfig.value.accounts[0].id) : '',
    credits_per_cny: 1,
    notes: '',
  }
  syncRateFormForAccount()
  rateDialogOpen.value = true
}

function syncRateFormForAccount() {
  const existing = rateConfig.value.rates.find((rate) => rate.account_id === Number(rateForm.value.account_id))
  rateForm.value.credits_per_cny = existing?.credits_per_cny ?? 1
  rateForm.value.notes = existing?.notes ?? ''
}

async function saveRate() {
  savingRate.value = true
  try {
    await createBusinessAPIKeyCostRate({
      account_id: Number(rateForm.value.account_id),
      credits_per_cny: rateForm.value.credits_per_cny,
      notes: rateForm.value.notes,
    })
    rateDialogOpen.value = false
    appStore.showSuccess('API Key 分数兑换比例已保存')
    await load()
  } catch (error) {
    console.error('Failed to create API key cost rate:', error)
    appStore.showError('API Key 分数比例保存失败，请检查账号和比例')
  } finally { savingRate.value = false }
}

function openCostDialog() {
  costForm.value = { cost_type: 'oauth_subscription', amount: 0, currency: 'CNY', fx_rate: 1, group_id: '', account_id: '', starts_at: props.startDate, ends_at: props.endDate, notes: '' }
  costDialogOpen.value = true
}

function localDateStartISO(value: string) {
  return new Date(`${value}T00:00:00`).toISOString()
}

function dayAfterISO(value: string) {
  const date = new Date(`${value}T00:00:00`)
  date.setDate(date.getDate() + 1)
  return date.toISOString()
}

async function saveCost() {
  saving.value = true
  try {
    await createBusinessCost({
      cost_type: costForm.value.cost_type,
      amount: costForm.value.amount,
      currency: costForm.value.currency.toUpperCase(),
      fx_rate: costForm.value.fx_rate,
      group_id: costForm.value.group_id ? Number(costForm.value.group_id) : undefined,
      account_id: costForm.value.account_id ? Number(costForm.value.account_id) : undefined,
      starts_at: localDateStartISO(costForm.value.starts_at),
      ends_at: dayAfterISO(costForm.value.ends_at),
      notes: costForm.value.notes,
    })
    costDialogOpen.value = false
    appStore.showSuccess('OAuth / 固定成本已录入')
    await load()
  } catch (error) {
    console.error('Failed to create business cost:', error)
    appStore.showError('固定成本保存失败；API Key 账号无需重复录入')
  } finally { saving.value = false }
}

async function removeCost(id: number) {
  if (!window.confirm('删除这条账号成本记录？')) return
  try { await deleteBusinessCost(id); await load(); appStore.showSuccess('账号成本已删除') }
  catch (error) { console.error('Failed to delete business cost:', error); appStore.showError('账号成本删除失败') }
}

async function removeRate(id: number) {
  if (!window.confirm('删除这条 API Key 分数比例记录？删除后该账号的 API Key 用量成本会变为待定价。')) return
  try { await deleteBusinessAPIKeyCostRate(id); await load(); appStore.showSuccess('API Key 分数比例已删除') }
  catch (error) { console.error('Failed to delete API key cost rate:', error); appStore.showError('API Key 分数比例删除失败') }
}

async function captureSnapshot() {
  snapshotting.value = true
  try { await captureBusinessCapacitySnapshot(); await load(); appStore.showSuccess('承载快照已刷新') }
  catch (error) { console.error('Failed to capture capacity snapshot:', error); appStore.showError('承载快照刷新失败') }
  finally { snapshotting.value = false }
}

onMounted(() => { void load() })
watch(() => [props.startDate, props.endDate], () => { void load() })
</script>

<style scoped>
.business-panel {
  background: rgb(255 255 255);
}

.business-panel__loading {
  display: flex;
  min-height: 18rem;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: .75rem;
  color: rgb(107 114 128);
  font-size: .875rem;
}

.business-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: .75rem 1.5rem;
  border-bottom: 1px solid rgb(229 231 235);
}

.business-subnav {
  display: inline-flex;
  max-width: 100%;
  gap: .25rem;
  overflow-x: auto;
  padding: .25rem;
  border-radius: .625rem;
  background: rgb(243 244 246);
}

.business-subnav button {
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
  gap: .4rem;
  min-width: 6.5rem;
  justify-content: center;
  padding: .5rem .75rem;
  border-radius: .5rem;
  color: rgb(107 114 128);
  font-size: .75rem;
  font-weight: 600;
  transition: background-color .2s ease, color .2s ease, box-shadow .2s ease;
}

.business-subnav button:hover {
  color: rgb(55 65 81);
}

.business-subnav button:focus-visible {
  outline: 2px solid rgb(59 130 246 / .5);
  outline-offset: 1px;
}

.business-subnav .business-subnav__item--active {
  background: rgb(255 255 255);
  color: rgb(37 99 235);
  box-shadow: 0 1px 2px rgb(15 23 42 / .08);
}

.business-panel__snapshot {
  flex: 0 0 auto;
  color: rgb(156 163 175);
  font-size: .75rem;
  white-space: nowrap;
}

.business-view {
  padding: 1.5rem;
  animation: business-view-in .18s ease-out;
}

.business-view .business-section + .business-section {
  border-top: 1px solid rgb(229 231 235);
}

@keyframes business-view-in {
  from { opacity: .45; transform: translateY(2px); }
  to { opacity: 1; transform: translateY(0); }
}

.business-data-warning {
  display: flex;
  align-items: center;
  gap: .625rem;
  margin: 1.25rem 1.5rem 0;
  padding: .75rem 1rem;
  border: 1px solid rgb(253 230 138);
  border-radius: .625rem;
  background: rgb(255 251 235);
  color: rgb(146 64 14);
  font-size: .8125rem;
}

.business-data-warning span { flex: 1; }
.business-data-warning button { color: rgb(146 64 14); font-weight: 600; white-space: nowrap; }
.business-data-warning button:hover { text-decoration: underline; }

.business-cumulative {
  display: grid;
  grid-template-columns: minmax(15rem, 1.55fr) repeat(3, minmax(8rem, 1fr));
  overflow: hidden;
  border: 1px solid rgb(219 234 254);
  border-radius: .75rem;
  background: rgb(248 250 252);
  font-variant-numeric: tabular-nums;
}

.business-cumulative > div {
  min-width: 0;
  padding: 1.15rem 1.25rem;
  border-left: 1px solid rgb(226 232 240);
}

.business-cumulative > div:first-child { border-left: 0; }
.business-cumulative span,
.business-cumulative small { display: block; color: rgb(107 114 128); font-size: .75rem; line-height: 1.4; }
.business-cumulative strong { display: block; margin-top: .35rem; color: rgb(31 41 55); font-size: 1.125rem; overflow-wrap: anywhere; }
.business-cumulative__result { background: rgb(239 246 255); }
.business-cumulative__result strong { margin: .35rem 0; color: rgb(5 150 105); font-size: 2rem; line-height: 1.15; }
.business-cumulative--negative .business-cumulative__result { background: rgb(254 242 242); }
.business-cumulative--negative .business-cumulative__result strong { color: rgb(220 38 38); }

.business-inline-warning {
  margin-top: .5rem;
  color: rgb(180 83 9);
  font-size: .75rem;
}

.business-rate-strip {
  display: flex;
  flex-wrap: wrap;
  gap: .5rem 1.5rem;
  padding: .875rem .25rem 0;
  color: rgb(107 114 128);
  font-size: .75rem;
}

.business-rate-strip strong { margin-left: .2rem; color: rgb(55 65 81); font-weight: 600; }

.business-section { padding: 1.5rem 0; }
.business-section--last { padding-bottom: 0; }
.business-period { padding-top: 1.25rem; }
.business-section__title { display: flex; align-items: center; justify-content: space-between; gap: 1rem; margin-bottom: 1rem; }
.business-section__title span { display: block; color: rgb(17 24 39); font-size: .875rem; font-weight: 600; }
.business-section__title .btn { padding: .5rem .75rem; border-radius: .5rem; font-size: .75rem; }
.business-visuals { padding-top: 0; }

.business-metrics {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: .75rem;
}

.business-metric {
  min-width: 0;
  padding: 1rem;
  border: 1px solid rgb(229 231 235);
  border-radius: .625rem;
  background: rgb(255 255 255);
}

.business-metric span,
.business-metric small { display: block; color: rgb(107 114 128); font-size: .75rem; line-height: 1.4; }
.business-metric strong { display: block; margin: .3rem 0; color: rgb(17 24 39); font-size: 1.125rem; font-variant-numeric: tabular-nums; overflow-wrap: anywhere; }
.business-metric--revenue strong { color: rgb(37 99 235); }
.business-metric--green strong { color: rgb(5 150 105); }
.business-metric--amber strong { color: rgb(217 119 6); }
.business-metric--red strong { color: rgb(220 38 38); }

.business-cash-list { display: flex; flex-wrap: wrap; gap: .75rem; }
.business-cash-list div { min-width: 8rem; padding: .75rem 1rem; border: 1px solid rgb(229 231 235); border-radius: .625rem; background: rgb(255 255 255); }
.business-cash-list span,
.business-cash-list strong { display: block; font-size: .8125rem; }
.business-cash-list span { color: rgb(107 114 128); }
.business-cash-list strong { margin-top: .2rem; color: rgb(5 150 105); font-variant-numeric: tabular-nums; }

.business-risk-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: .75rem; }
.business-risk-grid div { min-width: 0; padding: 1rem; border: 1px solid rgb(229 231 235); border-radius: .625rem; background: rgb(255 255 255); }
.business-risk-grid__alert { border-color: rgb(254 202 202) !important; background: rgb(254 242 242) !important; }
.business-risk-grid span,
.business-risk-grid small { display: block; color: rgb(107 114 128); font-size: .75rem; line-height: 1.4; }
.business-risk-grid strong { display: block; margin: .3rem 0; color: rgb(31 41 55); font-size: 1rem; font-variant-numeric: tabular-nums; }

.business-table-wrap {
  max-width: 100%;
  overflow-x: auto;
  border: 1px solid rgb(229 231 235);
  border-radius: .75rem;
  background: rgb(255 255 255);
}

.business-table { width: 100%; min-width: 850px; border-collapse: collapse; font-size: .875rem; }
.business-table--daily { min-width: 720px; }
.business-table--profit-groups { min-width: 900px; }
.business-table--capacity { min-width: 820px; }
.business-table--compact { min-width: 760px; }
.business-table th {
  padding: .75rem 1rem;
  border-bottom: 1px solid rgb(229 231 235);
  background: rgb(249 250 251);
  color: rgb(107 114 128);
  font-size: .75rem;
  font-weight: 500;
  text-align: right;
  white-space: nowrap;
}
.business-table th:first-child,
.business-table td:first-child { text-align: left; }
.business-table td {
  padding: .75rem 1rem;
  border-bottom: 1px solid rgb(229 231 235);
  color: rgb(75 85 99);
  text-align: right;
  font-variant-numeric: tabular-nums;
  vertical-align: middle;
}
.business-table tbody tr { transition: background-color .15s ease; }
.business-table tbody tr:hover { background: rgb(249 250 251); }
.business-table tbody tr:last-child td { border-bottom: 0; }
.business-table td strong,
.business-table td small { display: block; }
.business-table td strong { color: rgb(31 41 55); font-weight: 500; }
.business-table td small { margin-top: .15rem; color: rgb(156 163 175); font-size: .75rem; }
.business-table__muted { color: rgb(107 114 128) !important; }
.business-table__warning { color: rgb(180 83 9) !important; font-weight: 600; }
.business-table__positive,
.business-table__positive strong { color: rgb(5 150 105) !important; }
.business-table__negative,
.business-table__negative strong { color: rgb(220 38 38) !important; }
.business-icon-button { display: inline-flex; padding: .375rem; border-radius: .375rem; color: rgb(107 114 128); transition: background-color .15s ease, color .15s ease; }
.business-icon-button:hover { background: rgb(254 242 242); color: rgb(220 38 38); }
.business-icon-button:focus-visible { outline: 2px solid rgb(59 130 246 / .5); outline-offset: 1px; }

.business-config-actions { display: flex; justify-content: flex-end; gap: .5rem; padding-bottom: 1rem; }
.business-empty { padding: 2.5rem 1rem !important; color: rgb(156 163 175) !important; font-size: .875rem; text-align: center !important; }
.business-dialog-note { padding: .75rem; border-left: 3px solid rgb(59 130 246); background: rgb(239 246 255); }
.business-dialog-note span,
.business-dialog-note strong,
.business-dialog-note small { display: block; }
.business-dialog-note span,
.business-dialog-note small,
.business-dialog-help { color: rgb(107 114 128); font-size: .75rem; }
.business-dialog-note strong { margin: .2rem 0; color: rgb(17 24 39); font-size: .875rem; }
.business-dialog-help { margin: .35rem 0 0; }
.business-dialog-warning { padding: .75rem; border-left: 3px solid rgb(217 119 6); background: rgb(255 251 235); color: rgb(146 64 14); font-size: .75rem; }

:global(.dark) .business-panel,
:global(.dark) .business-toolbar { background: rgb(17 24 39); }
:global(.dark) .business-toolbar,
:global(.dark) .business-view .business-section + .business-section { border-color: rgb(55 65 81); }
:global(.dark) .business-subnav { background: rgb(31 41 55); }
:global(.dark) .business-subnav button:hover { color: rgb(229 231 235); }
:global(.dark) .business-subnav .business-subnav__item--active { background: rgb(55 65 81); color: rgb(96 165 250); box-shadow: none; }
:global(.dark) .business-cumulative { border-color: rgb(30 64 175 / .55); background: rgb(31 41 55); }
:global(.dark) .business-cumulative > div { border-color: rgb(55 65 81); }
:global(.dark) .business-cumulative__result { background: rgb(30 58 138 / .22); }
:global(.dark) .business-cumulative--negative .business-cumulative__result { background: rgb(127 29 29 / .2); }
:global(.dark) .business-cumulative strong,
:global(.dark) .business-section__title span,
:global(.dark) .business-metric strong,
:global(.dark) .business-risk-grid strong,
:global(.dark) .business-rate-strip strong { color: rgb(243 244 246); }
:global(.dark) .business-cumulative--positive .business-cumulative__result strong { color: rgb(52 211 153); }
:global(.dark) .business-cumulative--negative .business-cumulative__result strong { color: rgb(248 113 113); }
:global(.dark) .business-metric--revenue strong { color: rgb(96 165 250); }
:global(.dark) .business-metric--green strong { color: rgb(52 211 153); }
:global(.dark) .business-metric--amber strong { color: rgb(251 191 36); }
:global(.dark) .business-metric--red strong { color: rgb(248 113 113); }
:global(.dark) .business-metric,
:global(.dark) .business-cash-list div,
:global(.dark) .business-risk-grid div,
:global(.dark) .business-table-wrap { border-color: rgb(55 65 81); background: rgb(17 24 39); }
:global(.dark) .business-risk-grid__alert { border-color: rgb(127 29 29) !important; background: rgb(127 29 29 / .18) !important; }
:global(.dark) .business-table th { border-color: rgb(55 65 81); background: rgb(31 41 55); color: rgb(156 163 175); }
:global(.dark) .business-table td { border-color: rgb(55 65 81); color: rgb(209 213 219); }
:global(.dark) .business-table td strong { color: rgb(243 244 246); }
:global(.dark) .business-table tbody tr:hover { background: rgb(31 41 55 / .65); }
:global(.dark) .business-icon-button:hover { background: rgb(127 29 29 / .2); color: rgb(248 113 113); }
:global(.dark) .business-data-warning,
:global(.dark) .business-dialog-warning { border-color: rgb(146 64 14); background: rgb(120 53 15 / .25); color: rgb(253 186 116); }
:global(.dark) .business-data-warning button { color: rgb(253 186 116); }
:global(.dark) .business-dialog-note { background: rgb(30 58 138 / .2); }
:global(.dark) .business-dialog-note strong { color: rgb(243 244 246); }

@media (max-width: 900px) {
  .business-toolbar { align-items: flex-start; }
  .business-cumulative { grid-template-columns: repeat(3, minmax(0, 1fr)); }
  .business-cumulative__result { grid-column: 1 / -1; }
  .business-cumulative > div:nth-child(2) { border-left: 0; }
  .business-cumulative > div:nth-child(n + 2) { border-top: 1px solid rgb(226 232 240); }
  .business-metrics,
  .business-risk-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}

@media (max-width: 640px) {
  .business-toolbar { padding: .75rem 1rem; }
  .business-panel__snapshot { display: none; }
  .business-subnav { width: 100%; }
  .business-subnav button { min-width: max-content; flex: 1 0 auto; padding-right: .65rem; padding-left: .65rem; }
  .business-view { padding: 1rem; }
  .business-data-warning { margin: 1rem 1rem 0; }
  .business-cumulative { grid-template-columns: 1fr; }
  .business-cumulative__result { grid-column: auto; }
  .business-cumulative > div { border-top: 1px solid rgb(226 232 240); border-left: 0; }
  .business-cumulative > div:first-child { border-top: 0; }
  .business-cumulative__result strong { font-size: 1.75rem; }
  .business-metric { padding: .875rem; }
  .business-metric strong { font-size: 1rem; }
  .business-risk-grid { grid-template-columns: 1fr; }
  .business-config-actions { align-items: stretch; }
  .business-config-actions .btn { flex: 1; padding-right: .75rem; padding-left: .75rem; }
}
</style>
