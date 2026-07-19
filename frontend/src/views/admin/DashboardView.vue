<template>
  <AppLayout>
    <div class="dashboard-page">
      <div v-if="loading" class="dashboard-state dashboard-state--loading">
        <LoadingSpinner />
      </div>

      <template v-else-if="stats">
        <header class="dashboard-header">
          <div>
            <h1>{{ greeting }}，{{ displayName }} <span aria-hidden="true">👋</span></h1>
            <p>这里是 SolidAPI 平台今日的运行概览</p>
          </div>
          <div class="dashboard-actions">
            <DashboardRangeSelect v-model="timeRange" :options="rangeOptions" @change="applyTimeRange" />
            <button type="button" class="refresh-button" :disabled="chartsLoading" @click="loadDashboard">
              <Icon name="refresh" size="sm" :class="chartsLoading ? 'animate-spin' : ''" />
              刷新
            </button>
          </div>
        </header>

        <section class="hero-metrics" aria-label="今日核心指标">
          <article
            class="hero-card hero-card--primary hero-card--with-sparkline"
            @pointermove="updatePrimaryCardSpotlight"
            @pointerleave="hidePrimaryCardSpotlight"
          >
            <span>今日 Token</span>
            <div class="token-pair" aria-label="今日 Token 输入输出">
              <p><span>输入</span><strong>{{ formatTokens(stats.today_input_tokens) }}</strong></p>
              <p><span>输出</span><strong>{{ formatTokens(stats.today_output_tokens) }}</strong></p>
            </div>
            <small class="cost-breakdown">
              <span class="cost-breakdown__actual">实际 ${{ formatCost(stats.today_actual_cost) }}</span>
              <span class="cost-breakdown__account">成本 ${{ formatCost(stats.today_account_cost) }}</span>
              <span class="cost-breakdown__standard">标准 ${{ formatCost(stats.today_cost) }}</span>
            </small>
            <svg v-if="hasTrendData" class="metric-sparkline" viewBox="0 0 300 78" preserveAspectRatio="none" aria-hidden="true">
              <path :d="tokenSparklinePath" />
            </svg>
          </article>

          <article class="hero-card hero-card--with-sparkline">
            <span>今日 API 调用</span>
            <strong>{{ formatNumber(stats.today_requests) }}</strong>
            <small>累计 {{ formatNumber(stats.total_requests) }} 次</small>
            <svg v-if="hasTrendData" class="metric-sparkline metric-sparkline--mint" viewBox="0 0 300 78" preserveAspectRatio="none" aria-hidden="true">
              <path :d="requestSparklinePath" />
            </svg>
          </article>

          <article class="hero-card hero-card--with-sparkline">
            <span>今日消耗</span>
            <strong>${{ formatCost(stats.today_actual_cost) }}</strong>
            <small>标准成本 ${{ formatCost(stats.today_cost) }}</small>
            <svg v-if="hasTrendData" class="metric-sparkline metric-sparkline--violet" viewBox="0 0 300 78" preserveAspectRatio="none" aria-hidden="true">
              <path :d="costSparklinePath" />
            </svg>
          </article>
        </section>

        <section class="operation-strip" aria-label="平台运行指标">
          <div class="strip-item">
            <span class="strip-icon"><Icon name="key" size="md" /></span>
            <p>API Keys<strong>{{ formatNumber(stats.total_api_keys) }}</strong><small>{{ formatNumber(stats.active_api_keys) }} 已启用</small></p>
          </div>
          <div class="strip-item">
            <span class="strip-icon"><Icon name="server" size="md" /></span>
            <p>账号池<strong>{{ formatNumber(stats.total_accounts) }}</strong><small>{{ formatNumber(stats.normal_accounts) }} 正常</small></p>
          </div>
          <div class="strip-item">
            <span class="strip-icon"><Icon name="users" size="md" /></span>
            <p>活跃用户<strong>{{ formatNumber(stats.active_users) }}</strong><small>累计 {{ formatNumber(stats.total_users) }}</small></p>
          </div>
          <div class="strip-item">
            <span class="strip-icon"><Icon name="bolt" size="md" /></span>
            <p>实时 RPM<strong>{{ formatNumber(stats.rpm) }}</strong><small>近 5 分钟平均</small></p>
          </div>
          <div class="strip-item">
            <span class="strip-icon"><Icon name="bolt" size="md" /></span>
            <p>实时 TPM<strong>{{ formatNumber(stats.tpm) }}</strong><small>近 5 分钟平均</small></p>
          </div>
          <div class="strip-item">
            <span class="strip-icon"><Icon name="clock" size="md" /></span>
            <p>平均响应<strong>{{ formatDuration(stats.average_duration_ms) }}</strong><small>系统请求耗时</small></p>
          </div>
        </section>

        <section class="dashboard-grid">
          <article class="dashboard-card trend-card">
            <div class="card-heading">
              <div><h2>Token 使用趋势</h2><p>按选定时间范围汇总的平台 Token 用量</p></div>
            </div>
            <TokenUsageTrend :trend-data="trendData" :loading="chartsLoading" embedded />
          </article>

          <article class="dashboard-card distribution-card">
            <div class="card-heading">
              <div><h2>模型使用分布</h2><p>按 Token 使用量排序</p></div>
              <router-link to="/admin/usage">查看更多</router-link>
            </div>
            <div v-if="modelStats.length" class="model-list">
              <div v-for="model in modelStats.slice(0, 4)" :key="model.model" class="model-row">
                <div><strong>{{ model.model }}</strong><span><i :style="{ width: `${modelWidth(model.total_tokens)}%` }" /></span></div>
                <em>{{ modelPercent(model.total_tokens) }}</em><b>{{ formatTokens(model.total_tokens) }}</b>
              </div>
            </div>
            <div v-else class="compact-empty">该时间范围暂无模型使用数据</div>
          </article>

          <article class="dashboard-card recent-card">
            <div class="card-heading">
              <div><h2>高用量用户（Top 6）</h2><p>当前时间范围内消耗前六</p></div>
              <router-link to="/admin/usage">查看全部</router-link>
            </div>
            <div v-if="rankingLoading" class="compact-loading"><LoadingSpinner size="sm" /></div>
            <div v-else-if="rankingItems.length" class="recent-list">
              <button v-for="(item, index) in rankingItems.slice(0, 6)" :key="item.user_id" type="button" class="recent-row" @click="goToUserUsage(item.user_id)">
                <span class="model-mark">{{ index + 1 }}</span><strong :title="getUserLabel(item)">{{ getUserLabel(item) }}</strong><span>{{ formatTokens(item.tokens) }}</span><time>¥{{ formatCost(item.actual_cost) }}</time>
              </button>
            </div>
            <div v-else class="compact-empty">该时间范围暂无用户使用数据</div>
          </article>
        </section>
      </template>

      <div v-else class="dashboard-state dashboard-state--error">
        <strong>仪表盘数据暂时无法加载</strong><span>请检查网络后重试。</span><button type="button" class="refresh-button" @click="loadDashboard"><Icon name="refresh" size="sm" />重新加载</button>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { adminAPI } from '@/api/admin'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Icon from '@/components/icons/Icon.vue'
import DashboardRangeSelect, { type DashboardTimeRange } from '@/components/dashboard/DashboardRangeSelect.vue'
import TokenUsageTrend from '@/components/charts/TokenUsageTrend.vue'
import { useAuthStore } from '@/stores/auth'
import { buildSmoothSparklinePath } from '@/utils/sparkline'
import type { DashboardStats, ModelStat, TrendDataPoint, UserSpendingRankingItem } from '@/types'

const router = useRouter()
const authStore = useAuthStore()
const stats = ref<DashboardStats | null>(null)
const loading = ref(false)
const chartsLoading = ref(false)
const rankingLoading = ref(false)
const trendData = ref<TrendDataPoint[]>([])
const modelStats = ref<ModelStat[]>([])
const rankingItems = ref<UserSpendingRankingItem[]>([])

const formatLocalDate = (date: Date) => `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
const timeRange = ref<DashboardTimeRange>('24h')
const rangeOptions: Array<{ value: DashboardTimeRange; label: string }> = [{ value: '24h', label: '最近 24 小时' }, { value: '7d', label: '最近 7 天' }, { value: '30d', label: '最近 30 天' }]
const endDate = ref('')
const startDate = ref('')
const granularity = computed<'hour' | 'day'>(() => timeRange.value === '24h' ? 'hour' : 'day')
const trendValues = computed(() => trendData.value.map((item) => item.total_tokens || 0))
const requestTrendValues = computed(() => trendData.value.map((item) => item.requests || 0))
const costTrendValues = computed(() => trendData.value.map((item) => item.actual_cost || 0))
const hasTrendData = computed(() => trendData.value.length > 0)
const tokenSparklinePath = computed(() => buildSmoothSparklinePath(trendValues.value, 300, 78))
const requestSparklinePath = computed(() => buildSmoothSparklinePath(requestTrendValues.value, 300, 78))
const costSparklinePath = computed(() => buildSmoothSparklinePath(costTrendValues.value, 300, 78))
const maxModelTokens = computed(() => Math.max(...modelStats.value.map((item) => item.total_tokens), 1))
const totalModelTokens = computed(() => modelStats.value.reduce((total, item) => total + item.total_tokens, 0))
const displayName = computed(() => authStore.user?.username || authStore.user?.email?.split('@')[0] || '管理员')
const greeting = computed(() => {
  const hour = new Date().getHours()
  if (hour < 6) return '夜深了'
  if (hour < 12) return '早上好'
  if (hour < 18) return '下午好'
  return '晚上好'
})
const modelWidth = (tokens: number) => tokens > 0 ? (tokens / maxModelTokens.value) * 100 : 0
const modelPercent = (tokens: number) => totalModelTokens.value > 0 ? `${Math.round((tokens / totalModelTokens.value) * 100)}%` : '0%'
const formatNumber = (value: number) => Number(value || 0).toLocaleString()
const formatTokens = (value: number) => value >= 1_000_000 ? `${(value / 1_000_000).toFixed(2)}M` : value >= 1_000 ? `${(value / 1_000).toFixed(1)}K` : formatNumber(value)
const formatCost = (value: number) => Number(value || 0).toFixed(2)
const formatDuration = (value: number) => value >= 1000 ? `${(value / 1000).toFixed(2)}s` : `${Math.round(value || 0)}ms`

function updatePrimaryCardSpotlight(event: PointerEvent) {
  const card = event.currentTarget as HTMLElement
  const bounds = card.getBoundingClientRect()
  card.style.setProperty('--spotlight-x', `${event.clientX - bounds.left}px`)
  card.style.setProperty('--spotlight-y', `${event.clientY - bounds.top}px`)
  card.style.setProperty('--spotlight-opacity', '1')
}

function hidePrimaryCardSpotlight(event: PointerEvent) {
  const card = event.currentTarget as HTMLElement
  card.style.setProperty('--spotlight-opacity', '0')
}
const getUserLabel = (item: UserSpendingRankingItem) => item.email?.trim() || `用户 #${item.user_id}`
const goToUserUsage = (userId: number) => void router.push({ path: '/admin/usage', query: { user_id: String(userId), start_date: startDate.value, end_date: endDate.value } })
const updateDateRange = () => {
  const end = new Date()
  const start = new Date(end)
  start.setHours(start.getHours() - (timeRange.value === '24h' ? 24 : timeRange.value === '7d' ? 6 * 24 : 29 * 24))
  startDate.value = formatLocalDate(start)
  endDate.value = formatLocalDate(end)
}

const loadSnapshot = async () => {
  chartsLoading.value = true
  if (!stats.value) loading.value = true
  try {
    const response = await adminAPI.dashboard.getSnapshotV2({ start_date: startDate.value, end_date: endDate.value, granularity: granularity.value, include_stats: true, include_trend: true, include_model_stats: true, include_group_stats: false, include_users_trend: false })
    stats.value = response.stats || null
    trendData.value = response.trend || []
    modelStats.value = response.models || []
  } catch (error) {
    stats.value = null
    trendData.value = []
    modelStats.value = []
    console.error('Failed to load admin dashboard snapshot:', error)
  } finally {
    loading.value = false
    chartsLoading.value = false
  }
}
const loadRanking = async () => {
  rankingLoading.value = true
  try {
    const response = await adminAPI.dashboard.getUserSpendingRanking({ start_date: startDate.value, end_date: endDate.value, limit: 6 })
    rankingItems.value = response.ranking || []
  } catch (error) {
    rankingItems.value = []
    console.error('Failed to load user ranking:', error)
  } finally {
    rankingLoading.value = false
  }
}
const loadDashboard = () => { void loadSnapshot(); void loadRanking() }
const applyTimeRange = () => { updateDateRange(); loadDashboard() }

onMounted(() => { updateDateRange(); loadDashboard() })
</script>

<style scoped>
.dashboard-page {
  --dashboard-ink: #112146;
  --dashboard-muted: #7f8ca5;
  --dashboard-line: #e4ebf5;
  --dashboard-blue: #347cff;
  --dashboard-surface: #ffffff;
  min-height: calc(100vh - 126px);
  padding: 10px 0 26px;
  color: var(--dashboard-ink);
  font-variant-numeric: tabular-nums;
}
.dashboard-header { display:flex; align-items:flex-start; justify-content:space-between; gap:24px; margin:0 0 22px; }
.dashboard-header h1 { margin:0; color:var(--dashboard-ink); font-size:30px; font-weight:760; letter-spacing:-.05em; line-height:1.15; }
.dashboard-header h1 span { font-size:.7em; letter-spacing:0; }
.dashboard-header p { margin:8px 0 0; color:var(--dashboard-muted); font-size:13px; }
.dashboard-actions { display:flex; align-items:center; gap:10px; flex-shrink:0; }
.refresh-button { display:inline-flex; align-items:center; gap:8px; height:40px; border:1px solid var(--dashboard-line); border-radius:10px; background:var(--dashboard-surface); color:#53627d; padding:0 13px; box-shadow:0 4px 12px rgba(28,56,112,.035); font-size:13px; font-weight:650; }
.refresh-button { cursor:pointer; transition:background .16s ease, border-color .16s ease, transform .16s ease; }
.refresh-button:hover:not(:disabled) { border-color:#bfd3fc; background:#f6f9ff; transform:translateY(-1px); }
.refresh-button:disabled { cursor:wait; opacity:.66; }
.hero-metrics { display:grid; grid-template-columns:1.18fr .9fr .9fr; gap:16px; }
.hero-card { position:relative; min-height:174px; overflow:hidden; border:1px solid var(--dashboard-line); border-radius:16px; background:var(--dashboard-surface); padding:24px; box-shadow:0 10px 28px rgba(27,59,113,.055); }
.hero-card--with-sparkline { min-height:196px; padding-bottom:78px; }
.hero-card--with-sparkline > :not(.metric-sparkline) { position:relative; z-index:2; }
.hero-card > span { display:block; color:#61708a; font-size:13px; font-weight:650; }
.hero-card strong { display:block; margin:8px 0 9px; color:var(--dashboard-ink); font-size:31px; font-weight:760; letter-spacing:-.045em; line-height:1; }
.hero-card small { color:var(--dashboard-muted); font-size:12px; }
.token-pair { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:20px; margin:12px 0 10px; }
.token-pair p { min-width:0; margin:0; }
.token-pair p > span { display:block; margin-bottom:5px; color:rgba(255,255,255,.76); font-size:10px; font-weight:650; }
.token-pair strong { overflow:hidden; margin:0; font-size:25px; text-overflow:ellipsis; white-space:nowrap; }
.cost-breakdown { display:flex; flex-wrap:wrap; gap:4px 12px; font-variant-numeric:tabular-nums; }
.cost-breakdown__actual { color:#bbf7d0; }
.cost-breakdown__account { color:#fed7aa; }
.cost-breakdown__standard { color:#e2e8f0; }
.hero-card--primary { --spotlight-x:-200px; --spotlight-y:-200px; --spotlight-opacity:0; isolation:isolate; border-color:rgba(141,184,255,.76); background:linear-gradient(118deg,rgba(52,124,255,.88) 0%,rgba(76,158,246,.72) 52%,rgba(111,207,224,.62) 100%); box-shadow:0 15px 34px rgba(66,135,232,.2),inset 0 1px rgba(255,255,255,.38); backdrop-filter:blur(14px) saturate(122%); transition:border-color .24s ease,box-shadow .24s ease; }
.hero-card--primary::after { position:absolute; z-index:0; inset:0; background:radial-gradient(circle 150px at var(--spotlight-x) var(--spotlight-y),rgba(255,255,255,.32) 0%,rgba(191,231,255,.14) 44%,transparent 76%); content:""; opacity:var(--spotlight-opacity); transition:opacity .22s ease; pointer-events:none; }
.hero-card--primary > * { position:relative; z-index:1; }
.hero-card--primary:hover { border-color:rgba(186,224,255,.96); box-shadow:0 18px 40px rgba(45,117,221,.25),inset 0 1px rgba(255,255,255,.48); }
.hero-card--primary > span,.hero-card--primary small,.hero-card--primary strong { color:#fff; }
.metric-sparkline { position:absolute; z-index:1; right:20px; bottom:13px; left:20px; width:calc(100% - 40px); height:46px; opacity:.72; pointer-events:none; }
.metric-sparkline path { fill:none; stroke:#fff; stroke-width:1.65; stroke-linecap:round; stroke-linejoin:round; vector-effect:non-scaling-stroke; }
.metric-sparkline--mint path { stroke:#2bc4ad; }
.metric-sparkline--violet path { stroke:#9570f7; }
.operation-strip { display:grid; grid-template-columns:repeat(6,minmax(0,1fr)); margin:16px 0; overflow:hidden; border:1px solid var(--dashboard-line); border-radius:15px; background:var(--dashboard-surface); box-shadow:0 8px 20px rgba(28,56,104,.035); }
.strip-item { display:flex; align-items:center; min-width:0; gap:10px; padding:15px 14px; border-right:1px solid var(--dashboard-line); }
.strip-item:last-child { border-right:0; }
.strip-icon { display:grid; width:34px; height:34px; flex:0 0 auto; place-items:center; border-radius:50%; background:#edf4ff; color:var(--dashboard-blue); }
.strip-item p,.strip-item strong,.strip-item small { display:block; margin:0; }
.strip-item p { min-width:0; color:#8190a9; font-size:10px; line-height:1.3; }
.strip-item strong { overflow:hidden; margin:2px 0; color:#243457; font-size:17px; font-weight:760; line-height:1.1; text-overflow:ellipsis; white-space:nowrap; }
.strip-item small { color:#8a9ab4; font-size:10px; }
.dashboard-grid { display:grid; grid-template-columns:1.18fr .92fr .92fr; gap:16px; }
.dashboard-card { min-width:0; min-height:254px; border:1px solid var(--dashboard-line); border-radius:16px; background:var(--dashboard-surface); padding:20px; box-shadow:0 8px 22px rgba(28,56,104,.04); }
.card-heading { display:flex; align-items:flex-start; justify-content:space-between; gap:12px; }
.card-heading h2 { margin:0; color:#253556; font-size:15px; font-weight:760; letter-spacing:-.025em; }
.card-heading p { margin:5px 0 0; color:var(--dashboard-muted); font-size:11px; }
.card-heading a { color:var(--dashboard-blue); font-size:11px; font-weight:700; text-decoration:none; white-space:nowrap; }
.model-list { display:grid; gap:15px; margin-top:20px; }
.model-row { display:grid; grid-template-columns:minmax(0,1fr) 31px auto; align-items:center; gap:8px; }
.model-row strong { display:block; overflow:hidden; color:#3a4964; font-size:11px; font-weight:650; text-overflow:ellipsis; white-space:nowrap; }
.model-row span { display:block; height:7px; margin-top:7px; overflow:hidden; border-radius:999px; background:#edf1f7; }
.model-row i { display:block; height:100%; border-radius:inherit; background:linear-gradient(90deg,#2974fc,#58a6f2); }
.model-row:nth-child(2) i { background:linear-gradient(90deg,#1dbe9f,#59c6b7); }
.model-row:nth-child(3) i { background:linear-gradient(90deg,#8c65ef,#b293fc); }
.model-row:nth-child(4) i { background:linear-gradient(90deg,#ffac20,#ffc64b); }
.model-row em,.model-row b { color:#7b89a2; font-size:10px; font-style:normal; text-align:right; white-space:nowrap; }
.model-row b { color:#55647c; font-weight:650; }
.compact-empty,.compact-loading { display:grid; min-height:174px; place-items:center; color:var(--dashboard-muted); font-size:12px; text-align:center; }
.recent-list { margin-top:13px; }
.recent-row { display:grid; width:100%; grid-template-columns:24px minmax(0,1fr) auto auto; align-items:center; gap:8px; border:0; border-bottom:1px solid #edf1f7; background:transparent; padding:8px 0; color:inherit; cursor:pointer; font:inherit; text-align:left; }
.recent-row:last-child { border-bottom:0; }
.recent-row:hover strong { color:var(--dashboard-blue); }
.model-mark { display:grid; width:21px; height:21px; place-items:center; border-radius:6px; background:#eef4ff; color:#4b82ef; font-size:10px; font-weight:760; }
.recent-row strong { overflow:hidden; color:#40506a; font-size:11px; font-weight:650; text-overflow:ellipsis; white-space:nowrap; }
.recent-row > span:not(.model-mark),.recent-row time { color:#8390a6; font-size:10px; white-space:nowrap; }
.dashboard-state { display:grid; min-height:330px; place-content:center; justify-items:center; gap:10px; color:var(--dashboard-muted); }
.dashboard-state--error strong { color:var(--dashboard-ink); font-size:17px; }
.dashboard-state--error .refresh-button { margin-top:4px; }
@media (max-width:1180px) { .dashboard-grid { grid-template-columns:1fr 1fr; }.recent-card { grid-column:span 2; }.operation-strip { grid-template-columns:repeat(3,1fr); }.strip-item:nth-child(3) { border-right:0; }.strip-item:nth-child(-n+3) { border-bottom:1px solid var(--dashboard-line); } }
@media (max-width:760px) { .dashboard-page { padding-top:0; }.dashboard-header { flex-direction:column; gap:16px; }.dashboard-header h1 { font-size:27px; }.dashboard-actions { width:100%; }.refresh-button { flex:1; justify-content:center; }.hero-metrics,.dashboard-grid { grid-template-columns:1fr; }.operation-strip { grid-template-columns:1fr 1fr; }.strip-item { border-bottom:1px solid var(--dashboard-line); }.strip-item:nth-child(2n) { border-right:0; }.recent-card { grid-column:auto; } }
@media (prefers-reduced-motion:reduce) { .hero-card--primary,.hero-card--primary::after { transition:none; } }
.dark .dashboard-page { --dashboard-ink:#eef4ff; --dashboard-muted:#9aa9c3; --dashboard-line:rgba(152,180,224,.16); --dashboard-surface:#0e192b; }
.dark .hero-card:not(.hero-card--primary),.dark .operation-strip,.dark .dashboard-card,.dark .refresh-button { background:#0e192b; }
.dark .hero-card strong,.dark .strip-item strong,.dark .card-heading h2,.dark .recent-row strong { color:#eef4ff; }
.dark .hero-card--primary { border-color:rgba(124,185,255,.72); background:linear-gradient(118deg,#255fc8 0%,#2c82d2 52%,#277c94 100%); box-shadow:0 16px 38px rgba(17,76,164,.32),inset 0 1px rgba(255,255,255,.26); }
.dark .hero-card--primary > span { color:#f5f9ff; }
.dark .hero-card--primary .token-pair p > span { color:#dbeafe; }
.dark .hero-card--primary strong { color:#fff; text-shadow:0 2px 12px rgba(3,19,48,.3); }
.dark .hero-card--primary .cost-breakdown__actual { color:#a7f3d0; }
.dark .hero-card--primary .cost-breakdown__account { color:#fed7aa; }
.dark .hero-card--primary .cost-breakdown__standard { color:#e8eef8; }
.dark .recent-row { border-color:rgba(152,180,224,.13); }
.dark .model-row span,.dark .model-mark,.dark .strip-icon { background:rgba(77,132,224,.14); }
</style>
