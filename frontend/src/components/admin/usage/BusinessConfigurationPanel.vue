<template>
  <div class="space-y-6">
    <div class="config-context"><Icon name="clock" size="sm" /><span>经营账启用 {{ formatDate(configuration.enabled_at) }}<span class="mx-3 text-gray-300">/</span>报表时区 {{ configuration.timezone }}</span></div>

    <section class="config-card" aria-label="成本池">
      <header class="config-heading">
        <div class="config-title"><span class="config-icon"><Icon name="grid" size="md" /></span><div><h4>成本池 <span class="config-count">{{ configuration.pools.length }}</span></h4><p>按供应商管理采购额度，多个账号可以共用一个成本池。</p></div></div>
        <button class="btn btn-secondary" @click="open({ mode: 'pool' })"><Icon name="plus" size="sm" class="mr-1.5" />新增成本池</button>
      </header>
      <div class="config-table-wrap"><table v-if="configuration.pools.length" class="config-table"><thead><tr><th>成本池 / 供应商</th><th>成本方式</th><th>计价单位</th><th class="text-right">操作</th></tr></thead><tbody>
        <tr v-for="p in configuration.pools" :key="p.id"><td><strong>{{ p.name }}</strong><small>{{ p.supplier || '未填写供应商' }} · #{{ p.id }}</small></td><td><span class="config-mode">{{ modeLabels[p.mode] }}</span></td><td>{{ unitLabels[p.unit] || p.unit }}</td><td class="text-right"><button class="config-edit" :aria-label="'编辑成本池 ' + p.name" @click="editPool(p)"><Icon name="edit" size="sm" />编辑</button></td></tr>
      </tbody></table><p v-else class="config-empty">还没有成本池。先添加供应商，再关联账号和价格规则。</p></div>
    </section>

    <section class="config-card" aria-label="账号绑定历史">
      <header class="config-heading">
        <div class="config-title"><span class="config-icon"><Icon name="link" size="md" /></span><div><h4>账号绑定历史 <span class="config-count">{{ configuration.bindings.length }}</span></h4><p>每次变更独立保存，成本按请求发生时的有效绑定归属。</p></div></div>
        <button class="btn btn-secondary" @click="open({ mode: 'binding' })">批量绑定账号</button>
      </header>
      <div class="config-table-wrap config-history"><table v-if="configuration.bindings.length" class="config-table"><thead><tr><th>账号</th><th>成本池</th><th>生效时间</th></tr></thead><tbody><tr v-for="b in configuration.bindings" :key="b.id"><td><strong>{{ b.account_name || `账号 #${b.account_id}` }}</strong><small>#{{ b.account_id }}</small></td><td>{{ poolName(b.pool_id) }}</td><td class="whitespace-nowrap tabular-nums">{{ formatDate(b.effective_at) }}</td></tr></tbody></table><p v-else class="config-empty">尚未绑定账号。支持一次选择多个账号并设置统一生效时间。</p></div>
    </section>

    <section class="config-card" aria-label="价格规则历史">
      <header class="config-heading">
        <div class="config-title"><span class="config-icon"><Icon name="document" size="md" /></span><div><h4>价格规则历史 <span class="config-count">{{ configuration.rules.length }}</span></h4><p>保留每个生效版本，已记录的历史价格不会被新规则覆盖。</p></div></div>
        <button class="btn btn-secondary" @click="open({ mode: 'rule' })">新增价格版本</button>
      </header>
      <div class="config-table-wrap config-history"><table v-if="configuration.rules.length" class="config-table"><thead><tr><th>成本池 / 模型</th><th>计价与单价</th><th>价格依据</th><th>生效时间</th></tr></thead><tbody><tr v-for="r in configuration.rules" :key="r.id">
        <td><strong>{{ poolName(r.pool_id) }}</strong><small>{{ r.model === '*' ? '所有模型（默认）' : r.model }}</small><small v-if="r.service_tier || r.image_size || r.video_resolution">{{ [r.service_tier, r.image_size, r.video_resolution].filter(Boolean).join(' · ') }}</small></td>
        <td><span>{{ basisLabels[r.basis] }}</span><small v-if="r.basis === 'tokens'">每百万 token · 输入 {{ r.input_price }} / 输出 {{ r.output_price }}<br />缓存读 {{ r.cache_read_price }} / 写 {{ r.cache_write_price }} / 1h {{ r.cache_write_1h_price }}</small><small v-else>单位价格 {{ r.unit_price }}</small><small>每上游单位 ¥{{ r.cny_per_unit }}</small></td>
        <td>{{ businessQuality(r.quality) }}<small class="max-w-xs break-words">{{ r.notes }}</small></td><td class="whitespace-nowrap tabular-nums">{{ formatDate(r.effective_at) }}</td>
      </tr></tbody></table><p v-else class="config-empty">尚未配置价格规则。按供应商价目和实际生效时间添加价格版本。</p></div>
    </section>
    <BaseDialog :show="!!mode" :title="editingPool ? '编辑成本池' : actions.find(a => a.key === mode)?.label || ''" width="wide" :show-close-button="!saving" :close-on-escape="!saving" @close="mode = ''">
      <form id="business-config-form" class="grid gap-4 sm:grid-cols-2" @submit.prevent="save">
        <template v-if="mode === 'pool'">
          <label class="text-sm">成本池名称<input v-model="name" aria-label="成本池名称" class="input mt-1" required maxlength="200" /></label><label class="text-sm">供应商名称<input v-model="supplier" aria-label="供应商名称" class="input mt-1" maxlength="200" /></label>
          <label class="text-sm">成本方式<select v-model="poolMode" aria-label="成本方式" :disabled="poolLocked" class="input mt-1"><option value="prepaid">预充值、按消耗结转</option><option value="postpaid">后付费、按合同暂记</option><option value="fixed">仅固定账号费用</option></select></label>
          <label class="text-sm">上游额度单位<select v-model="unit" aria-label="上游额度单位" :disabled="poolLocked" class="input mt-1"><option value="upstream_credit">供应商额度</option><option value="USD">法币 USD</option><option value="CNY">法币 CNY</option></select></label>
          <p v-if="poolLocked" class="rounded-lg bg-gray-50 p-3 text-xs leading-5 text-gray-500 dark:bg-dark-900 sm:col-span-2">此成本池已有绑定、价格或凭据，可修改名称和供应商。若需变更成本方式或单位，请新建成本池并重新绑定账号。</p>
          <p class="text-xs text-gray-500 sm:col-span-2">固定月租可以与按量成本同时存在：账号绑定预充值或后付费成本池，再单独登记账号服务期费用。</p>
        </template>
        <template v-else>
          <div><span class="mb-1 block text-sm">成本池</span><Select v-model="poolID" aria-label="成本池" :options="poolOptions" searchable placeholder="选择供应商成本池" /></div>
          <label class="text-sm">生效时间（本机时区）<input v-model="effective" aria-label="生效时间" type="datetime-local" required class="input mt-1" /></label>
          <div v-if="mode === 'binding'" class="sm:col-span-2">
            <span class="mb-1 block text-sm">添加账号</span>
            <BusinessEntitySelect :model-value="0" kind="accounts" label="搜索账号名称或 ID" :selected-ids="accountIDs" :disabled-reasons="disabledAccounts" :option-hints="bindingHints" @select="addAccount" />
            <div class="selected-accounts">
              <div class="flex items-center justify-between gap-2 text-xs"><span class="font-medium">本次已选 <strong class="text-primary-600">{{ accountIDs.length }}</strong> 个账号</span><button v-if="accountIDs.length" type="button" class="text-gray-500 hover:text-gray-800 dark:hover:text-gray-200" @click="accountIDs = []">清空选择</button></div>
              <div v-if="accountIDs.length" class="mt-3 flex flex-wrap gap-2"><button v-for="id in accountIDs" :key="id" type="button" class="account-chip" :aria-label="'移除账号 ' + id" @click="accountIDs = accountIDs.filter(value => value !== id)">{{ accountNames[id] || configuration.bindings.find(b => b.account_id === id)?.account_name || `账号 #${id}` }}<Icon name="x" size="xs" /></button></div>
              <p v-else class="mt-2 text-xs text-gray-400">从上方搜索并添加账号，已选账号会保留在这里。</p>
            </div>
            <p class="mt-2 text-xs leading-5 text-gray-500">列表会标记已选及已有绑定。在所选生效时间已绑定当前成本池的账号无需重复添加；选择其他成本池可新增绑定版本。</p>
          </div>
          <template v-if="mode === 'rule'">
            <label class="text-sm">上游模型（* 表示默认）<input v-model="model" class="input mt-1" required /></label>
            <label class="text-sm">服务等级（留空通用）<input v-model="serviceTier" class="input mt-1" placeholder="priority / flex" /></label><label class="text-sm">图片尺寸（留空通用）<input v-model="imageSize" class="input mt-1" placeholder="1K / 2K / 4K" /></label><label class="text-sm">视频分辨率（留空通用）<input v-model="videoResolution" class="input mt-1" placeholder="720p / 1080p" /></label>
            <label class="text-sm">上游计价依据<select v-model="basis" class="input mt-1"><option v-for="(label, key) in basisLabels" :key="key" :value="key">{{ label }}</option></select></label>
            <template v-if="basis === 'tokens'"><label v-for="price in tokenFields" :key="price.key" class="text-sm">{{ price.label }}（每百万 token）<input v-model="prices[price.key]" required inputmode="decimal" class="input mt-1" /></label></template>
            <label v-else class="text-sm">{{ basis === 'account_stats' ? '统计计价系数（只可标暂估）' : '单位价格（上游额度）' }}<input v-model="unitPrice" required inputmode="decimal" class="input mt-1" /></label>
            <label class="text-sm">每上游单位人民币成本<input v-model="cnyPerUnit" required inputmode="decimal" class="input mt-1" /><span class="text-xs text-gray-500">预充值池按采购加权成本，此值用于后付费。</span></label>
            <label class="text-sm">价格依据<select v-model="quality" class="input mt-1"><option value="contract">有合同 / 供应商价目</option><option value="estimated">暂估，等待账单核对</option></select></label>
            <label class="text-sm sm:col-span-2">价格凭据与适用范围<textarea v-model="notes" required class="input mt-1" rows="2" /></label>
          </template>
        </template>
        <p v-if="error" role="alert" class="text-sm text-red-600 sm:col-span-2">{{ error }}</p>
      </form>
      <template #footer><button class="btn btn-secondary" :disabled="saving" @click="mode = ''">取消</button><button type="submit" form="business-config-form" class="btn btn-primary" :disabled="saving">{{ saving ? '保存中…' : editingPool ? '保存修改' : mode === 'pool' ? '创建成本池' : '新增并保存版本' }}</button></template>
    </BaseDialog>
  </div>
</template>
<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import Select from '@/components/common/Select.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import BusinessEntitySelect from './BusinessEntitySelect.vue'
import Icon from '@/components/icons/Icon.vue'
import type { BusinessConfigureRequest } from './BusinessIssuesPanel.vue'
import { businessAPI, type BusinessConfiguration, type BusinessPool, type BusinessBinding, type BusinessEntity } from '@/api/admin/business'
import { businessQuality, ledgerError, localDateTime } from '@/utils/business-ledger'
const props = defineProps<{ configuration: BusinessConfiguration }>()
const emit = defineEmits<{ saved: [] }>()
const actions = [{ key: 'pool', label: '新增成本池' }, { key: 'binding', label: '批量绑定账号' }, { key: 'rule', label: '新增价格版本' }]
const modeLabels: Record<string, string> = { prepaid: '预充值', postpaid: '后付费', fixed: '固定费用' }
const basisLabels: Record<string, string> = { tokens: '按 token', request: '按请求', image: '按图片', video_second: '按视频秒数', account_stats: '现有账号统计价格（暂估）' }
const editingPool = ref<BusinessPool | null>(null)
const savedBindings = ref<BusinessBinding[]>([])
const poolLocked = computed(() => !!editingPool.value && (editingPool.value.accounting_locked || props.configuration.bindings.some(b => b.pool_id === editingPool.value?.id) || props.configuration.rules.some(r => r.pool_id === editingPool.value?.id)))
const unitLabels: Record<string, string> = { upstream_credit: '供应商额度', USD: '法币 USD', CNY: '人民币 CNY' }
const accountIDs = ref<number[]>([]), accountNames = ref<Record<number, string>>({})
const serviceTier = ref(''), imageSize = ref(''), videoResolution = ref('')
const mode = ref(''), error = ref(''), saving = ref(false), name = ref(''), supplier = ref(''), poolMode = ref('prepaid'), unit = ref('upstream_credit')
const poolID = ref<number | string | boolean | null>(null), effective = ref(localDateTime()), model = ref('*'), basis = ref('tokens'), quality = ref('contract'), notes = ref(''), unitPrice = ref('1'), cnyPerUnit = ref('1')
const prices = ref<Record<string, string>>({ input_price: '0', output_price: '0', cache_read_price: '0', cache_write_price: '0', cache_write_1h_price: '0' })
const tokenFields = [{ key: 'input_price', label: '输入' }, { key: 'output_price', label: '输出' }, { key: 'cache_read_price', label: '缓存读取' }, { key: 'cache_write_price', label: '缓存写入（短期）' }, { key: 'cache_write_1h_price', label: '缓存写入（1小时）' }]
const poolOptions = computed(() => props.configuration.pools.map(p => ({ value: p.id, label: p.name })))
const poolName = (id: number) => props.configuration.pools.find(p => p.id === id)?.name || `成本池 #${id}`
const formatDate = (s: string) => new Date(s).toLocaleString('zh-CN', { timeZone: props.configuration.timezone })
const effectiveBindings = computed(() => {
  const at = new Date(effective.value).getTime()
  const latest = new Map<number, BusinessBinding>()
  for (const b of [...props.configuration.bindings, ...savedBindings.value]) {
    const previous = latest.get(b.account_id)
    if (new Date(b.effective_at).getTime() <= at && (!previous || new Date(b.effective_at) > new Date(previous.effective_at) || (b.effective_at === previous.effective_at && b.id > previous.id))) latest.set(b.account_id, b)
  }
  return latest
})
const bindingHints = computed(() => Object.fromEntries([...effectiveBindings.value].map(([id, b]) => [id, `已绑定 ${poolName(b.pool_id)}`])))
const disabledAccounts = computed(() => Object.fromEntries([...effectiveBindings.value].filter(([, b]) => b.pool_id === Number(poolID.value)).map(([id]) => [id, '已绑定当前成本池'])))
watch(disabledAccounts, disabled => { accountIDs.value = accountIDs.value.filter(id => !disabled[id]) })
function addAccount(account: BusinessEntity) {
  if (disabledAccounts.value[account.id] || accountIDs.value.includes(account.id)) return
  accountNames.value[account.id] = account.name
  accountIDs.value.push(account.id)
}
function editPool(pool: BusinessPool) {
  open({ mode: 'pool' })
  editingPool.value = pool
  name.value = pool.name; supplier.value = pool.supplier; poolMode.value = pool.mode; unit.value = pool.unit
}
function open(request: BusinessConfigureRequest) {
  mode.value = request.mode; error.value = ''; editingPool.value = null; name.value = ''; supplier.value = ''; poolMode.value = 'prepaid'; unit.value = 'upstream_credit'; accountIDs.value = [...new Set(request.accountIDs || [])]; accountNames.value = { ...(request.accountNames || {}) }
  poolID.value = request.poolID || (props.configuration.pools.length === 1 ? props.configuration.pools[0].id : null); model.value = request.model || '*'
  const at = request.effectiveAt ? new Date(request.effectiveAt) : new Date()
  effective.value = new Date(at.getTime() - at.getTimezoneOffset() * 60000).toISOString().slice(0, 16)
}
defineExpose({ open })
async function save() {
  if (saving.value) return
  saving.value = true; error.value = ''
  try {
    if (mode.value === 'pool') {
      const input = { name: name.value, supplier: supplier.value, mode: poolMode.value, unit: unit.value }
      if (editingPool.value) await businessAPI.updatePool(editingPool.value.id, input)
      else await businessAPI.pool(input)
    } else if (mode.value === 'binding') {
      const ids = accountIDs.value.filter(id => !disabledAccounts.value[id])
      if (!Number(poolID.value) || !ids.length) throw new Error('请选择成本池，并添加至少一个尚未绑定该成本池的账号')
      const at = new Date(effective.value).toISOString()
      await businessAPI.bindAccounts({ account_ids: ids, pool_id: Number(poolID.value), effective_at: at })
      savedBindings.value.push(...ids.map((id, index) => ({ id: Number.MAX_SAFE_INTEGER - index, account_id: id, account_name: accountNames.value[id] || '', pool_id: Number(poolID.value), effective_at: at })))
    }
    else await businessAPI.rule({ service_tier: serviceTier.value, image_size: imageSize.value, video_resolution: videoResolution.value, cache_write_1h_price: prices.value.cache_write_1h_price, pool_id: Number(poolID.value), model: model.value, effective_at: new Date(effective.value).toISOString(), basis: basis.value, unit_price: unitPrice.value, input_price: prices.value.input_price, output_price: prices.value.output_price, cache_read_price: prices.value.cache_read_price, cache_write_price: prices.value.cache_write_price, cny_per_unit: cnyPerUnit.value, quality: basis.value === 'account_stats' ? 'estimated' : quality.value, notes: notes.value })
    mode.value = ''; emit('saved')
  } catch (e) { error.value = ledgerError(e) }
  finally { saving.value = false }
}
</script>

<style scoped>
.config-context { @apply flex flex-wrap items-center gap-2 px-1 text-xs leading-5 text-gray-500 dark:text-gray-400; }
.config-card { @apply min-w-0 overflow-hidden rounded-xl border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800; }
.config-heading { @apply flex flex-wrap items-center justify-between gap-4 border-b border-gray-100 p-5 dark:border-dark-700; }
.config-title { @apply flex items-start gap-3; }
.config-title h4 { @apply flex items-center gap-2 text-sm font-semibold text-gray-900 dark:text-white; }
.config-title p { @apply mt-1 text-xs leading-5 text-gray-500 dark:text-gray-400; }
.config-icon { @apply flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-gray-300; }
.config-count { @apply rounded-md bg-gray-100 px-1.5 py-0.5 text-[11px] font-medium tabular-nums text-gray-500 dark:bg-dark-700 dark:text-gray-300; }
.config-table-wrap { @apply overflow-x-auto; }
.config-history { @apply max-h-96 overflow-y-auto; }
.config-table { @apply w-full min-w-[620px] text-left text-sm; }
.config-table th { @apply sticky top-0 z-[1] whitespace-nowrap bg-gray-50 px-5 py-3 text-xs font-medium text-gray-500 dark:bg-dark-900 dark:text-gray-400; }
.config-table td { @apply border-t border-gray-100 px-5 py-4 align-top text-gray-600 dark:border-dark-700 dark:text-gray-300; }
.config-table strong { @apply font-medium text-gray-800 dark:text-gray-100; }
.config-table small { @apply mt-1 block text-xs leading-5 text-gray-500 dark:text-gray-400; }
.config-mode { @apply inline-flex rounded-md bg-gray-100 px-2 py-1 text-xs dark:bg-dark-700; }
.config-edit { @apply inline-flex items-center gap-1.5 rounded px-2 py-1 text-xs font-medium text-primary-600 hover:bg-primary-50 dark:text-primary-300 dark:hover:bg-dark-700; }
.config-empty { @apply px-5 py-9 text-center text-sm leading-6 text-gray-500 dark:text-gray-400; }
.selected-accounts { @apply mt-3 rounded-lg border border-gray-200 bg-gray-50 p-3 dark:border-dark-600 dark:bg-dark-900; }
.account-chip { @apply inline-flex items-center gap-2 rounded-md border border-primary-200 bg-white px-2.5 py-1.5 text-xs text-primary-700 hover:bg-primary-50 dark:border-primary-800 dark:bg-dark-800 dark:text-primary-300; }
</style>
