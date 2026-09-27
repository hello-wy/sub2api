<template>
  <div class="space-y-6">
    <p class="text-sm text-gray-500">启用 {{ formatDate(configuration.enabled_at) }} · 报表时区 {{ configuration.timezone }}。成本绑定和规则按生效时间保存版本，新增配置后，可在下方预览并补算历史缺失记录；已有价格快照保留。</p>
    <div class="flex flex-wrap gap-2"><button v-for="item in actions" :key="item.key" class="btn btn-secondary" @click="open({ mode: item.key })">{{ item.label }}</button></div>
    <div class="overflow-auto"><table class="w-full text-left text-sm"><thead><tr><th>成本池 / 供应商</th><th>方式</th><th>计价单位</th></tr></thead><tbody><tr v-for="p in configuration.pools" :key="p.id" class="border-t dark:border-dark-700"><td class="py-3">{{ p.name }} · {{ p.supplier }} <small>#{{ p.id }}</small></td><td>{{ modeLabels[p.mode] }}</td><td>{{ p.unit }}</td></tr></tbody></table><p v-if="!configuration.pools.length" class="py-4 text-sm text-gray-500">先建立供应商成本池，再绑定账号并配置采购价。</p></div>
    <details><summary class="cursor-pointer text-sm font-medium">账号绑定历史（{{ configuration.bindings.length }}）</summary><ul class="mt-3 space-y-2 text-sm"><li v-for="b in configuration.bindings" :key="b.id">{{ b.account_name || `账号 #${b.account_id}` }} → {{ poolName(b.pool_id) }} · 生效 {{ formatDate(b.effective_at) }}</li></ul></details>
    <details><summary class="cursor-pointer text-sm font-medium">价格规则历史（{{ configuration.rules.length }}）</summary><div v-for="r in configuration.rules" :key="r.id" class="mt-3 border-t py-3 text-sm dark:border-dark-700">{{ poolName(r.pool_id) }} · {{ r.model }} · {{ basisLabels[r.basis] }} · {{ businessQuality(r.quality) }} · 生效 {{ formatDate(r.effective_at) }}<p class="mt-1 text-xs text-gray-500">每单位 ¥{{ r.cny_per_unit }}；基础单价 {{ r.unit_price }}；输入 / 输出 / 缓存读 / 缓存写：{{ r.input_price }} / {{ r.output_price }} / {{ r.cache_read_price }} / {{ r.cache_write_price }}。{{ r.notes }}</p></div></details>
    <BaseDialog :show="!!mode" :title="actions.find(a => a.key === mode)?.label || ''" width="wide" :show-close-button="!saving" :close-on-escape="!saving" @close="mode = ''">
      <form id="business-config-form" class="grid gap-4 sm:grid-cols-2" @submit.prevent="save">
        <template v-if="mode === 'pool'">
          <label class="text-sm">成本池名称<input v-model="name" class="input mt-1" required maxlength="200" /></label><label class="text-sm">供应商名称<input v-model="supplier" class="input mt-1" required /></label>
          <label class="text-sm">成本方式<select v-model="poolMode" class="input mt-1"><option value="prepaid">预充值、按消耗结转</option><option value="postpaid">后付费、按合同暂记</option><option value="fixed">仅固定账号费用</option></select></label>
          <label class="text-sm">上游额度单位<select v-model="unit" class="input mt-1"><option value="upstream_credit">供应商额度</option><option value="USD">法币 USD</option><option value="CNY">法币 CNY</option></select></label>
          <p class="text-xs text-gray-500 sm:col-span-2">固定月租可以与按量成本同时存在：账号绑定预充值或后付费成本池，再单独登记账号服务期费用。</p>
        </template>
        <template v-else>
          <div><span class="mb-1 block text-sm">成本池</span><Select v-model="poolID" :options="poolOptions" searchable placeholder="选择供应商成本池" /></div>
          <label class="text-sm">生效时间（本机时区）<input v-model="effective" type="datetime-local" required class="input mt-1" /></label>
          <div v-if="mode === 'binding'" class="sm:col-span-2"><span class="mb-1 block text-sm">账号</span><BusinessEntitySelect v-model="accountID" kind="accounts" label="搜索并添加账号，可添加多个" @select="accountNames[$event.id] = $event.name" />
            <div class="mt-2 flex flex-wrap gap-2"><button v-for="id in accountIDs" :key="id" type="button" class="rounded bg-gray-100 px-2 py-1 text-xs dark:bg-dark-700" :aria-label="'移除账号 ' + id" @click="accountIDs = accountIDs.filter(value => value !== id)">{{ accountNames[id] || configuration.bindings.find(b => b.account_id === id)?.account_name || `账号 #${id}` }} ×</button></div><p class="mt-2 text-xs text-gray-500">已选 {{ accountIDs.length }} 个账号，将按同一生效时间批量绑定。</p></div>
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
      <template #footer><button class="btn btn-secondary" :disabled="saving" @click="mode = ''">取消</button><button type="submit" form="business-config-form" class="btn btn-primary" :disabled="saving">{{ saving ? '保存中…' : '新增并保存版本' }}</button></template>
    </BaseDialog>
  </div>
</template>
<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import Select from '@/components/common/Select.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import BusinessEntitySelect from './BusinessEntitySelect.vue'
import type { BusinessConfigureRequest } from './BusinessIssuesPanel.vue'
import { businessAPI, type BusinessConfiguration } from '@/api/admin/business'
import { businessQuality, ledgerError, localDateTime } from '@/utils/business-ledger'
const props = defineProps<{ configuration: BusinessConfiguration }>()
const emit = defineEmits<{ saved: [] }>()
const actions = [{ key: 'pool', label: '新增成本池' }, { key: 'binding', label: '批量绑定账号' }, { key: 'rule', label: '新增价格版本' }]
const modeLabels: Record<string, string> = { prepaid: '预充值', postpaid: '后付费', fixed: '固定费用' }
const basisLabels: Record<string, string> = { tokens: '按 token', request: '按请求', image: '按图片', video_second: '按视频秒数', account_stats: '现有账号统计价格（暂估）' }
const accountIDs = ref<number[]>([]), accountNames = ref<Record<number, string>>({})
const serviceTier = ref(''), imageSize = ref(''), videoResolution = ref('')
const mode = ref(''), error = ref(''), saving = ref(false), name = ref(''), supplier = ref(''), poolMode = ref('prepaid'), unit = ref('upstream_credit')
const poolID = ref<number | string | boolean | null>(null), accountID = ref(0), effective = ref(localDateTime()), model = ref('*'), basis = ref('tokens'), quality = ref('contract'), notes = ref(''), unitPrice = ref('1'), cnyPerUnit = ref('1')
const prices = ref<Record<string, string>>({ input_price: '0', output_price: '0', cache_read_price: '0', cache_write_price: '0', cache_write_1h_price: '0' })
const tokenFields = [{ key: 'input_price', label: '输入' }, { key: 'output_price', label: '输出' }, { key: 'cache_read_price', label: '缓存读取' }, { key: 'cache_write_price', label: '缓存写入（短期）' }, { key: 'cache_write_1h_price', label: '缓存写入（1小时）' }]
const poolOptions = computed(() => props.configuration.pools.map(p => ({ value: p.id, label: p.name })))
const poolName = (id: number) => props.configuration.pools.find(p => p.id === id)?.name || `成本池 #${id}`
const formatDate = (s: string) => new Date(s).toLocaleString('zh-CN')
watch(accountID, id => { if (id && !accountIDs.value.includes(id)) accountIDs.value.push(id) })
function open(request: BusinessConfigureRequest) {
  mode.value = request.mode; error.value = ''; accountID.value = 0; accountIDs.value = [...(request.accountIDs || [])]; accountNames.value = { ...(request.accountNames || {}) }
  poolID.value = request.poolID || null; model.value = request.model || '*'
  const at = request.effectiveAt ? new Date(request.effectiveAt) : new Date()
  effective.value = new Date(at.getTime() - at.getTimezoneOffset() * 60000).toISOString().slice(0, 16)
}
defineExpose({ open })
async function save() {
  if (saving.value) return
  saving.value = true; error.value = ''
  try {
    if (mode.value === 'pool') await businessAPI.pool({ name: name.value, supplier: supplier.value, mode: poolMode.value, unit: unit.value })
    else if (mode.value === 'binding') await businessAPI.bindAccounts({ account_ids: accountIDs.value, pool_id: Number(poolID.value), effective_at: new Date(effective.value).toISOString() })
    else await businessAPI.rule({ service_tier: serviceTier.value, image_size: imageSize.value, video_resolution: videoResolution.value, cache_write_1h_price: prices.value.cache_write_1h_price, pool_id: Number(poolID.value), model: model.value, effective_at: new Date(effective.value).toISOString(), basis: basis.value, unit_price: unitPrice.value, input_price: prices.value.input_price, output_price: prices.value.output_price, cache_read_price: prices.value.cache_read_price, cache_write_price: prices.value.cache_write_price, cny_per_unit: cnyPerUnit.value, quality: basis.value === 'account_stats' ? 'estimated' : quality.value, notes: notes.value })
    mode.value = ''; emit('saved')
  } catch (e) { error.value = ledgerError(e) }
  finally { saving.value = false }
}
</script>
