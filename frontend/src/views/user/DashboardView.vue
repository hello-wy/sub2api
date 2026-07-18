<template>
  <AppLayout>
    <div class="dashboard-page">
      <div v-if="loading" class="flex items-center justify-center py-20"><LoadingSpinner /></div>
      <template v-else-if="stats">
        <header class="dashboard-header">
          <div>
            <p class="dashboard-kicker">个人工作台</p>
            <h1>晚上好，{{ user?.username || user?.email || '朋友' }} <span>👋</span></h1>
            <p>集中查看你的 API 使用、余额与模型偏好。</p>
          </div>
          <div class="dashboard-actions">
            <label class="range-label"><Icon name="calendar" size="sm" /><select v-model="timeRange" aria-label="统计时间范围" @change="applyTimeRange"><option v-for="option in rangeOptions" :key="option.value" :value="option.value">{{ option.label }}</option></select></label>
            <button type="button" class="refresh-button" :disabled="loadingCharts" @click="refreshAll"><Icon name="refresh" size="sm" :class="loadingCharts ? 'animate-spin' : ''" /> 刷新</button>
          </div>
        </header>

        <section class="hero-metrics">
          <article class="hero-card hero-card--primary" @pointermove="updatePrimaryCardSpotlight" @pointerleave="hidePrimaryCardSpotlight"><span>账户余额</span><strong>${{ formatCost(user?.balance || 0) }}</strong><small class="points-breakdown"><span>周积分 {{ formatLoyaltyPoints(weeklyPoints) }}</span><span>永久积分 {{ formatLoyaltyPoints(permanentPoints) }}</span></small></article>
          <article class="hero-card"><span>今日 API 调用</span><strong>{{ formatNumber(stats.today_requests) }}</strong><small>全部请求 {{ formatNumber(stats.total_requests) }} 次</small><svg v-if="hasTrendData" class="metric-sparkline metric-sparkline--mint" viewBox="0 0 300 78" preserveAspectRatio="none"><polyline :points="sparklinePoints" /></svg></article>
          <article class="hero-card"><span>今日 Token</span><div class="token-pair" aria-label="今日 Token 输入输出"><p><span>输入</span><strong>{{ formatTokens(stats.today_input_tokens) }}</strong></p><p><span>输出</span><strong>{{ formatTokens(stats.today_output_tokens) }}</strong></p></div><small>今日消费 ${{ formatCost(stats.today_actual_cost) }}</small><svg v-if="hasTrendData" class="metric-sparkline metric-sparkline--violet" viewBox="0 0 300 78" preserveAspectRatio="none"><polyline :points="sparklinePoints" /></svg></article>
        </section>

        <section class="personal-strip" aria-label="个人资源状态">
          <div class="strip-item"><span class="strip-icon"><Icon name="key" size="md" /></span><p>我的 API Key<strong>{{ stats.total_api_keys }}</strong><small>{{ stats.active_api_keys }} 个可用</small></p></div>
          <div v-if="!authStore.isSimpleMode" class="strip-item"><span class="strip-icon"><Icon name="creditCard" size="md" /></span><p>账户余额<strong>${{ formatCost(user?.balance || 0) }}</strong><small>个人账户可用</small></p></div>
          <div class="strip-item"><span class="strip-icon"><Icon name="cube" size="md" /></span><p>累计 Token<strong>输入 {{ formatTokens(stats.total_input_tokens) }}</strong><small>输出 {{ formatTokens(stats.total_output_tokens) }}</small></p></div>
          <div class="strip-item"><span class="strip-icon"><Icon name="chart" size="md" /></span><p>当前速率<strong>{{ formatNumber(stats.rpm) }} RPM</strong><small>{{ formatNumber(stats.tpm) }} TPM</small></p></div>
          <div class="strip-item"><span class="strip-icon"><Icon name="clock" size="md" /></span><p>平均响应<strong>{{ formatDuration(stats.average_duration_ms) }}</strong><small>个人请求表现</small></p></div>
        </section>

        <section v-if="!authStore.isSimpleMode && configuredPlatformQuotas.length" class="dashboard-card quota-card" aria-label="平台配额">
          <div class="card-heading"><div><h2>平台配额</h2><p>当前账号各平台用量与限额</p></div></div>
          <div class="quota-grid">
            <div v-for="quota in configuredPlatformQuotas" :key="quota.platform" class="quota-row">
              <strong>{{ platformLabel(quota.platform) }}</strong>
              <span v-if="quota.daily_limit_usd != null">日 {{ formatQuota(quota.daily_usage_usd, quota.daily_limit_usd) }}</span>
              <span v-if="quota.weekly_limit_usd != null">周 {{ formatQuota(quota.weekly_usage_usd, quota.weekly_limit_usd) }}</span>
              <span v-if="quota.monthly_limit_usd != null">月 {{ formatQuota(quota.monthly_usage_usd, quota.monthly_limit_usd) }}</span>
            </div>
          </div>
        </section>

        <section class="dashboard-grid">
          <article class="dashboard-card trend-card"><div class="card-heading"><div><h2>Token 使用趋势</h2><p>按选定时间范围汇总的个人 Token 用量</p></div><span class="chart-key"><i /> 总量（Token）</span></div><template v-if="hasTrendData"><div class="trend-chart"><svg viewBox="0 0 700 260" preserveAspectRatio="none"><defs><linearGradient id="user-area" x1="0" x2="0" y1="0" y2="1"><stop offset="0" stop-color="#2869ff" stop-opacity=".24"/><stop offset="1" stop-color="#2869ff" stop-opacity="0"/></linearGradient></defs><path :d="areaPath" fill="url(#user-area)"/><polyline :points="largeSparklinePoints" /></svg></div><div class="chart-axis"><span>{{ startDate }}</span><span>{{ endDate }}</span></div></template><div v-else class="chart-empty">该时间范围暂无 Token 使用数据</div></article>
          <article class="dashboard-card distribution-card"><div class="card-heading"><div><h2>我的模型偏好</h2><p>按真实 Token 使用量排序</p></div><router-link to="/usage">查看明细</router-link></div><div v-if="modelStats.length" class="model-list"><div v-for="model in modelStats.slice(0, 5)" :key="model.model" class="model-row"><div><strong>{{ model.model }}</strong><span><i :style="{ width: `${modelWidth(model.total_tokens)}%` }" /></span></div><b>{{ formatTokens(model.total_tokens) }}</b></div></div><div v-else class="empty-copy">该时间范围暂无模型使用数据</div></article>
          <article class="dashboard-card recent-card"><div class="card-heading"><div><h2>最近使用</h2><p>仅展示你的真实请求记录</p></div><router-link to="/usage">查看全部</router-link></div><div v-if="loadingUsage" class="flex items-center justify-center py-10"><LoadingSpinner size="sm" /></div><div v-else-if="recentUsage.length" class="recent-list"><div v-for="log in recentUsage" :key="log.id" class="recent-row"><span class="model-mark">{{ log.model.slice(0, 1).toUpperCase() }}</span><strong :title="log.model">{{ log.model }}</strong><span>{{ formatTokens(log.input_tokens + log.output_tokens) }}</span><time>{{ relativeTime(log.created_at) }}</time></div></div><div v-else class="empty-copy">该时间范围暂无调用记录。</div></article>
        </section>
      </template>
      <div v-else class="dashboard-load-error"><strong>仪表盘数据暂不可用</strong><span>{{ errorMessage || '请稍后重试。' }}</span><button type="button" class="refresh-button" @click="refreshAll">重新加载</button></div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { usageAPI, type UserDashboardStats as UserStatsType } from '@/api/usage'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Icon from '@/components/icons/Icon.vue'
import type { ModelStat, PlatformQuotaItem, TrendDataPoint, UsageLog, UserAttributeDefinition, UserAttributeValue } from '@/types'
import { getMyAttributes, getMyPlatformQuotas } from '@/api/user'
import { formatLoyaltyPoints, readLoyaltyPoints } from '@/utils/loyalty'

const authStore = useAuthStore()
const user = computed(() => authStore.user)
const stats = ref<UserStatsType | null>(null)
const loading = ref(false)
const loadingUsage = ref(false)
const loadingCharts = ref(false)
const errorMessage = ref('')
const trendData = ref<TrendDataPoint[]>([])
const modelStats = ref<ModelStat[]>([])
const recentUsage = ref<UsageLog[]>([])
const platformQuotas = ref<PlatformQuotaItem[] | null>(null)
const attributeDefinitions = ref<UserAttributeDefinition[]>([])
const attributeValues = ref<UserAttributeValue[]>([])
const timeRange = ref<'24h' | '7d' | '30d'>('7d')
const rangeOptions = [{ value: '24h', label: '最近 24 小时' }, { value: '7d', label: '最近 7 天' }, { value: '30d', label: '最近 30 天' }]
const startDate = ref('')
const endDate = ref('')
const granularity = computed<'hour' | 'day'>(() => timeRange.value === '24h' ? 'hour' : 'day')
const trendValues = computed(() => trendData.value.map((item) => item.total_tokens || 0))
const hasTrendData = computed(() => trendData.value.length > 0)
const sparklinePoints = computed(() => buildPoints(trendValues.value, 300, 78))
const largeSparklinePoints = computed(() => buildPoints(trendValues.value, 700, 220))
const areaPath = computed(() => hasTrendData.value ? `M 0 240 L ${largeSparklinePoints.value} L 700 240 Z` : '')
const maxModelTokens = computed(() => Math.max(...modelStats.value.map((item) => item.total_tokens), 1))
const configuredPlatformQuotas = computed(() => (platformQuotas.value ?? []).filter((quota) =>
  quota.daily_limit_usd != null || quota.weekly_limit_usd != null || quota.monthly_limit_usd != null
))
const weeklyPoints = computed(() => readLoyaltyPoints(attributeDefinitions.value, attributeValues.value, 'weekly'))
const permanentPoints = computed(() => readLoyaltyPoints(attributeDefinitions.value, attributeValues.value, 'permanent'))
const platformLabels: Record<string, string> = {
  anthropic: 'Claude',
  openai: 'OpenAI',
  gemini: 'Gemini',
  antigravity: 'Antigravity',
  grok: 'Grok'
}

const formatLocalDate = (date: Date) => `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
function buildPoints(values: number[], width: number, height: number) { if (!values.length) return ''; const maximum = Math.max(...values, 1); const minimum = Math.min(...values); const range = maximum - minimum || 1; return values.map((value, index) => `${(index / Math.max(values.length - 1, 1)) * width},${height - 9 - ((value - minimum) / range) * (height - 22)}`).join(' ') }
const modelWidth = (tokens: number) => tokens > 0 ? (tokens / maxModelTokens.value) * 100 : 0
const platformLabel = (platform: string) => platformLabels[platform] || platform
const formatNumber = (value: number) => Number(value || 0).toLocaleString()
const formatTokens = (value: number) => value >= 1_000_000 ? `${(value / 1_000_000).toFixed(2)}M` : value >= 1_000 ? `${(value / 1_000).toFixed(1)}K` : formatNumber(value)
const formatCost = (value: number) => Number(value || 0).toFixed(2)
const formatQuota = (usage: number, limit: number) => limit === 0 ? '已禁用' : `$${formatCost(usage)} / $${formatCost(limit)}`
const formatDuration = (value: number) => value >= 1000 ? `${(value / 1000).toFixed(2)}s` : `${Math.round(value || 0)}ms`
const relativeTime = (value: string) => { const minutes = Math.max(0, Math.round((Date.now() - new Date(value).getTime()) / 60000)); return minutes < 1 ? '刚刚' : minutes < 60 ? `${minutes} 分钟前` : `${Math.floor(minutes / 60)} 小时前` }
const updateDateRange = () => { const end = new Date(); const start = new Date(end); start.setHours(start.getHours() - (timeRange.value === '24h' ? 24 : timeRange.value === '7d' ? 6 * 24 : 29 * 24)); startDate.value = formatLocalDate(start); endDate.value = formatLocalDate(end) }
const loadStats = async () => { loading.value = true; errorMessage.value = ''; try { await authStore.refreshUser(); stats.value = await usageAPI.getDashboardStats() } catch (error) { stats.value = null; errorMessage.value = '无法获取个人统计数据'; console.error('Failed to load dashboard stats:', error) } finally { loading.value = false } }
const loadCharts = async () => { loadingCharts.value = true; try { const result = await Promise.all([usageAPI.getDashboardTrend({ start_date: startDate.value, end_date: endDate.value, granularity: granularity.value }), usageAPI.getDashboardModels({ start_date: startDate.value, end_date: endDate.value })]); trendData.value = result[0].trend || []; modelStats.value = result[1].models || [] } catch (error) { trendData.value = []; modelStats.value = []; console.error('Failed to load charts:', error) } finally { loadingCharts.value = false } }
const loadRecent = async () => { loadingUsage.value = true; try { const result = await usageAPI.getByDateRange(startDate.value, endDate.value); recentUsage.value = result.items.slice(0, 6) } catch (error) { recentUsage.value = []; console.error('Failed to load recent usage:', error) } finally { loadingUsage.value = false } }
const loadPlatformQuotas = async () => { try { const data = await getMyPlatformQuotas(); platformQuotas.value = data.platform_quotas ?? [] } catch (error) { platformQuotas.value = []; console.warn('Failed to load platform quotas:', error) } }
const loadLoyaltyPoints = async () => { try { const data = await getMyAttributes(); attributeDefinitions.value = data.definitions; attributeValues.value = data.values } catch (error) { attributeDefinitions.value = []; attributeValues.value = []; console.warn('Failed to load loyalty points:', error) } }
function updatePrimaryCardSpotlight(event: PointerEvent) { const card = event.currentTarget as HTMLElement; const bounds = card.getBoundingClientRect(); card.style.setProperty('--spotlight-x', `${event.clientX - bounds.left}px`); card.style.setProperty('--spotlight-y', `${event.clientY - bounds.top}px`); card.style.setProperty('--spotlight-opacity', '1') }
function hidePrimaryCardSpotlight(event: PointerEvent) { const card = event.currentTarget as HTMLElement; card.style.setProperty('--spotlight-opacity', '0') }
const refreshAll = () => { void loadStats(); void loadCharts(); void loadRecent(); void loadPlatformQuotas(); void loadLoyaltyPoints() }
const applyTimeRange = () => { updateDateRange(); refreshAll() }
onMounted(() => { updateDateRange(); refreshAll() })
</script>

<style scoped>
.dashboard-page { --ink:#10214a; --muted:#7180a0; --line:#dce7f5; --blue:#2869ff; padding:22px; color:var(--ink); }.dashboard-liquid-shell{min-height:calc(100vh - 126px);overflow:hidden;border:1px solid rgba(255,255,255,.64);border-radius:24px;background:linear-gradient(145deg,rgba(255,255,255,.62),rgba(235,247,255,.46));box-shadow:0 22px 60px rgba(50,94,155,.1),inset 0 1px rgba(255,255,255,.82)}.dashboard-header{display:flex;align-items:flex-start;justify-content:space-between;gap:24px;padding:8px 2px 26px}.dashboard-kicker{margin:0 0 4px;color:var(--blue);font-size:12px;font-weight:700;letter-spacing:.08em}.dashboard-header h1{margin:0;font-size:32px;font-weight:750;letter-spacing:-.04em}.dashboard-header h1 span{font-size:25px}.dashboard-header p{margin:8px 0 0;color:var(--muted);font-size:14px}.dashboard-actions{display:flex;gap:10px;align-items:center}.range-label,.refresh-button{display:inline-flex;align-items:center;gap:8px;border:1px solid rgba(255,255,255,.72);border-radius:10px;background:rgba(255,255,255,.56);padding:10px 13px;color:#344461;font-size:13px;font-weight:650;box-shadow:0 4px 14px rgba(34,56,104,.06)}.range-label select{max-width:120px;border:0;outline:0;background:transparent;color:inherit;font:inherit;cursor:pointer}.refresh-button:hover{border-color:#b9c9ee;color:var(--blue)}.refresh-button:disabled{opacity:.6}.hero-metrics{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:18px}.hero-card{position:relative;min-height:204px;overflow:hidden;border:1px solid rgba(255,255,255,.74);border-radius:18px;background:rgba(255,255,255,.58);padding:25px 26px;box-shadow:0 12px 30px rgba(30,61,122,.055),inset 0 1px rgba(255,255,255,.76)}.hero-card>span{color:#50617f;font-size:14px;font-weight:650}.hero-card strong{display:block;margin-top:12px;color:#10214a;font-size:34px;font-weight:760;letter-spacing:-.045em}.hero-card small{display:block;margin-top:8px;color:#75839e;font-size:13px}.hero-card--primary{border-color:rgba(178,224,255,.92);background:linear-gradient(120deg,rgba(53,122,247,.86) 0%,rgba(96,174,251,.72) 48%,rgba(184,241,241,.72) 100%)}.hero-card--primary>span,.hero-card--primary strong,.hero-card--primary small{color:#fff}.metric-sparkline{position:absolute;right:20px;bottom:13px;width:calc(100% - 40px);height:80px;opacity:.82}.metric-sparkline polyline{fill:none;stroke:#fff;stroke-width:1.7;vector-effect:non-scaling-stroke}.metric-sparkline--mint polyline{stroke:#12b999}.metric-sparkline--violet polyline{stroke:#8257f7}.personal-strip{display:grid;grid-template-columns:repeat(5,minmax(0,1fr));margin:20px 0;overflow:hidden;border:1px solid rgba(255,255,255,.72);border-radius:16px;background:rgba(255,255,255,.48);box-shadow:0 10px 28px rgba(30,61,122,.04),inset 0 1px rgba(255,255,255,.72)}.strip-item{display:flex;align-items:center;gap:11px;min-width:0;padding:18px;border-right:1px solid rgba(217,229,244,.76)}.strip-item:last-child{border-right:0}.strip-icon{display:grid;flex:0 0 auto;place-items:center;width:36px;height:36px;border-radius:50%;background:rgba(237,244,255,.78);color:var(--blue)}.strip-item p,.strip-item strong,.strip-item small{display:block;margin:0}.strip-item p{min-width:0;color:#74819b;font-size:11px}.strip-item strong{margin:2px 0;overflow:hidden;color:#17294e;font-size:17px;font-weight:750;text-overflow:ellipsis;white-space:nowrap}.strip-item small{color:#8190aa;font-size:10px}.dashboard-grid{display:grid;grid-template-columns:1.18fr .92fr .92fr;gap:18px}.dashboard-card{min-width:0;border:1px solid rgba(255,255,255,.72);border-radius:16px;background:rgba(255,255,255,.5);padding:21px;box-shadow:0 10px 28px rgba(30,61,122,.04),inset 0 1px rgba(255,255,255,.74)}.card-heading{display:flex;align-items:flex-start;justify-content:space-between;gap:12px}.card-heading h2{margin:0;color:#17294e;font-size:15px;font-weight:750;letter-spacing:-.02em}.card-heading p{margin:5px 0 0;color:#8290a9;font-size:11px}.card-heading a{color:var(--blue);font-size:12px;font-weight:650;text-decoration:none}.chart-key{display:flex;align-items:center;gap:6px;color:#7786a0;font-size:11px}.chart-key i{width:7px;height:7px;border-radius:50%;background:var(--blue)}.trend-chart{height:218px;margin-top:13px}.trend-chart svg{width:100%;height:100%;overflow:visible}.trend-chart polyline{fill:none;stroke:var(--blue);stroke-width:3;vector-effect:non-scaling-stroke}.chart-axis{display:flex;justify-content:space-between;color:#8c99b0;font-size:10px}.chart-empty{display:grid;min-height:236px;place-items:center;color:#8290a9;font-size:13px}.model-list{display:grid;gap:17px;margin-top:22px}.model-row{display:flex;align-items:center;gap:10px}.model-row>div{flex:1;min-width:0}.model-row strong{display:block;overflow:hidden;color:#33425f;font-size:12px;text-overflow:ellipsis;white-space:nowrap}.model-row span{display:block;height:8px;margin-top:8px;overflow:hidden;border-radius:999px;background:rgba(228,235,246,.76)}.model-row i{display:block;height:100%;border-radius:inherit;background:linear-gradient(90deg,#2d73ff,#51a7ed)}.model-row:nth-child(2) i{background:linear-gradient(90deg,#14ba94,#50beab)}.model-row:nth-child(3) i{background:linear-gradient(90deg,#8157f6,#a78afa)}.model-row b{color:#53617a;font-size:11px;font-weight:650}.recent-list{margin-top:17px}.recent-row{display:grid;grid-template-columns:25px minmax(0,1fr) auto auto;align-items:center;gap:9px;padding:9px 0;border-bottom:1px solid rgba(225,232,242,.82)}.recent-row:last-child{border-bottom:0}.model-mark{display:grid;place-items:center;width:22px;height:22px;border-radius:6px;background:rgba(233,242,255,.82);color:#3774ec;font-size:10px;font-weight:800}.recent-row strong{overflow:hidden;color:#35435e;font-size:11px;text-overflow:ellipsis;white-space:nowrap}.recent-row span:not(.model-mark),.recent-row time{color:#7f8da6;font-size:10px;white-space:nowrap}.empty-copy{padding:45px 4px;color:#8290a9;font-size:13px;text-align:center}.dashboard-load-error{display:grid;min-height:360px;place-content:center;justify-items:center;gap:10px;color:#7180a0}.dashboard-load-error strong{color:#17294e;font-size:17px}.dashboard-load-error .refresh-button{margin-top:8px}@media (max-width:1100px){.dashboard-grid{grid-template-columns:1fr 1fr}.recent-card{grid-column:span 2}.personal-strip{grid-template-columns:repeat(3,1fr)}.strip-item:nth-child(3){border-right:0}}@media (max-width:720px){.dashboard-page{padding:14px}.dashboard-header{flex-direction:column}.hero-metrics,.dashboard-grid{grid-template-columns:1fr}.recent-card{grid-column:auto}.personal-strip{grid-template-columns:1fr 1fr}.strip-item{border-bottom:1px solid var(--line)}.strip-item:nth-child(2n){border-right:0}.dashboard-actions{width:100%}.range-label,.refresh-button{flex:1;justify-content:center}}
.dashboard-page { min-height:calc(100vh - 126px); padding:4px 0 24px; background:#fbfdff; }
.dashboard-page.dashboard-liquid-shell { border:0; border-radius:0; box-shadow:none; background:#fbfdff; }
.dashboard-page .hero-card,.dashboard-page .personal-strip,.dashboard-page .dashboard-card { background:#fff; border-color:var(--line); }
.dashboard-page .range-label,.dashboard-page .refresh-button { background:#fff; border-color:var(--line); box-shadow:0 4px 12px rgba(34,56,104,.04); }
.quota-card { margin:20px 0; }
.quota-grid { display:grid;grid-template-columns:repeat(auto-fit,minmax(220px,1fr));gap:10px;margin-top:16px; }
.quota-row { display:grid;grid-template-columns:minmax(86px,1fr) auto;gap:5px 12px;border:1px solid var(--line);border-radius:10px;padding:12px 14px;color:#7180a0;font-size:11px; }
.quota-row strong { grid-row:1 / span 3;color:#17294e;font-size:13px;align-self:start; }
.quota-row span { text-align:right;font-variant-numeric:tabular-nums; }
.points-breakdown { display:flex!important; flex-wrap:wrap; gap:4px 14px; font-variant-numeric:tabular-nums; }
.token-pair { position:relative; z-index:1; display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:18px; margin:13px 0 9px; }
.token-pair p { min-width:0; margin:0; }
.token-pair p > span { display:block; margin-bottom:5px; color:#75839e; font-size:10px; font-weight:650; }
.token-pair strong { overflow:hidden; margin:0; font-size:26px; line-height:1; text-overflow:ellipsis; white-space:nowrap; }
.dashboard-page .hero-card.hero-card--primary { --spotlight-x:-200px; --spotlight-y:-200px; --spotlight-opacity:0; isolation:isolate; border-color:rgba(141,184,255,.76); background:linear-gradient(120deg,rgba(53,122,247,.86) 0%,rgba(83,164,248,.7) 50%,rgba(111,207,224,.58) 100%); box-shadow:0 16px 36px rgba(51,117,217,.2),inset 0 1px rgba(255,255,255,.38); backdrop-filter:blur(14px) saturate(122%); transition:border-color .24s ease,box-shadow .24s ease; }
.hero-card--primary::after { position:absolute; z-index:0; inset:0; background:radial-gradient(circle 165px at var(--spotlight-x) var(--spotlight-y),rgba(255,255,255,.34) 0%,rgba(191,231,255,.14) 44%,transparent 76%); content:""; opacity:var(--spotlight-opacity); pointer-events:none; transition:opacity .22s ease; }
.hero-card--primary > * { position:relative; z-index:1; }
.hero-card--primary:hover { border-color:rgba(186,224,255,.96); box-shadow:0 19px 42px rgba(45,117,221,.25),inset 0 1px rgba(255,255,255,.48); }
@media (prefers-reduced-motion:reduce) { .hero-card--primary,.hero-card--primary::after { transition:none; } }
</style>

<style>
.dark .dashboard-page {
  --ink: #eef4ff;
  --muted: #9aa9c3;
  --line: rgba(152, 180, 224, 0.16);
  background: transparent;
  color: var(--ink);
}

.dark .dashboard-page.dashboard-liquid-shell {
  background: transparent;
}

.dark .dashboard-page .hero-card:not(.hero-card--primary),
.dark .dashboard-page .personal-strip,
.dark .dashboard-page .dashboard-card,
.dark .dashboard-page .range-label,
.dark .dashboard-page .refresh-button,
.dark .dashboard-page .quota-row {
  border-color: var(--line);
  background: #0e192b;
  box-shadow: 0 12px 30px rgba(0, 0, 0, 0.18);
}

.dark .dashboard-page .dashboard-header h1,
.dark .dashboard-page .hero-card strong,
.dark .dashboard-page .strip-item strong,
.dark .dashboard-page .card-heading h2,
.dark .dashboard-page .recent-row strong,
.dark .dashboard-page .dashboard-load-error strong,
.dark .dashboard-page .quota-row strong {
  color: #eef4ff;
}

.dark .dashboard-page .hero-card > span,
.dark .dashboard-page .hero-card small,
.dark .dashboard-page .strip-item p,
.dark .dashboard-page .strip-item small,
.dark .dashboard-page .card-heading p,
.dark .dashboard-page .empty-copy,
.dark .dashboard-page .chart-empty,
.dark .dashboard-page .quota-row {
  color: #9aa9c3;
}

.dark .dashboard-page .strip-item,
.dark .dashboard-page .recent-row {
  border-color: var(--line);
}

.dark .dashboard-page .strip-icon,
.dark .dashboard-page .model-mark,
.dark .dashboard-page .model-row span {
  background: rgba(77, 132, 224, 0.14);
}
</style>
