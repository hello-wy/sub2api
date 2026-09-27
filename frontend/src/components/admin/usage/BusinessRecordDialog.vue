<template>
  <BaseDialog :show="true" :title="businessKindLabels[type] || '登记账务'" width="wide" :show-close-button="!saving" :close-on-escape="!saving" @close="emit('close')">
    <form id="business-record-form" class="space-y-4" @submit.prevent="save">
      <p class="rounded-lg bg-gray-50 p-3 text-sm text-gray-600 dark:bg-dark-800 dark:text-dark-300">金额均为人民币；额度单位独立记录。已登记的凭据保留历史，更正通过补录或冲销完成。</p>
      <div v-if="target" class="text-sm">原始凭据 #{{ target.id }} · {{ businessKindLabels[target.event_type] || target.event_type }} · 用户 #{{ target.user_id }}</div>
      <div class="grid gap-4 sm:grid-cols-2">
        <label class="block text-sm">发生时间（本机时区）<input v-model="at" required type="datetime-local" class="input mt-1" /></label>
        <label v-if="!['reversal', 'expense_stop', 'supplier_loss'].includes(type)" class="block text-sm">{{ type === 'annotation' ? '核实人民币价值 / 实际成本' : '人民币金额' }}<input v-model="amount" required inputmode="decimal" class="input mt-1" placeholder="0.00" /></label>
        <template v-if="type === 'receipt'">
          <div><span class="mb-1 block text-sm">关联用户</span><BusinessEntitySelect v-model="userID" kind="users" label="搜索用户姓名或邮箱" /></div>
          <label class="flex items-center gap-2 text-sm"><input v-model="applyBalance" type="checkbox" />同时给用户增加余额</label>
          <label v-if="applyBalance" class="block text-sm">到账站内额度<input v-model="credits" required inputmode="decimal" class="input mt-1" /></label>
          <p class="text-xs text-gray-500 sm:col-span-2">仅登记收款不会重复充值。补登已充值的线下款项后，请在待核对列表中补录对应余额来源。</p>
        </template>
        <template v-if="['purchase', 'opening_pool', 'supplier_refund', 'supplier_loss'].includes(type)">
          <div><span class="mb-1 block text-sm">供应商成本池</span><Select v-model="poolID" :options="poolOptions" searchable placeholder="选择成本池" /></div>
          <label class="block text-sm">上游额度（{{ selectedPool?.unit || '按成本池单位' }}）<input v-model="credits" required inputmode="decimal" class="input mt-1" /></label>
        </template>
        <template v-if="type === 'expense' || type === 'reconciliation' || type === 'adjustment'">
          <label class="block text-sm">费用类别<select v-model="category" class="input mt-1"><option value="account_subscription">账号订阅 / 租赁</option><option value="server">服务器</option><option value="proxy">代理</option><option value="domain">域名</option><option value="staff">人工</option><option value="campaign">现金奖励 / 实物奖品</option><option value="commission">渠道佣金</option><option value="supplier">供应商账单</option></select></label>
          <div><span class="mb-1 block text-sm">指定账号（可选）</span><BusinessEntitySelect v-model="accountID" kind="accounts" label="搜索账号；留空表示不指定" /></div>
          <div><span class="mb-1 block text-sm">指定分组（可选）</span><BusinessEntitySelect v-model="groupID" kind="groups" label="搜索分组；均留空为公共费用" /></div>
          <template v-if="type === 'expense'"><label class="flex items-center gap-2 text-sm"><input v-model="accrue" type="checkbox" />按服务期摊销</label><template v-if="accrue"><label class="block text-sm">服务期开始<input v-model="starts" type="datetime-local" required class="input mt-1" /></label><label class="block text-sm">服务期结束（不含）<input v-model="ends" type="datetime-local" required class="input mt-1" /></label></template></template>
          <template v-else><label v-if="type === 'reconciliation'" class="flex items-center gap-2 text-sm"><input v-model="billMode" type="checkbox" />按供应商完整账单核对</label><template v-if="billMode && type === 'reconciliation'"><div><span class="mb-1 block text-sm">成本池</span><Select v-model="poolID" :options="allPoolOptions" searchable placeholder="选择供应商成本池" /></div><label class="text-sm">账单期开始<input v-model="starts" required type="datetime-local" class="input mt-1" /></label><label class="text-sm">账单期结束（不含）<input v-model="ends" required type="datetime-local" class="input mt-1" /></label><p class="text-xs text-gray-500 sm:col-span-2">金额填写完整账单实际金额，系统计算与已记成本的差额并保留核对范围。</p></template><label class="block text-sm">影响科目<select v-model="entryKind" class="input mt-1"><option value="usage_cost">上游成本差额</option><option value="operating_cost">经营费用差额</option><option value="refund_revenue">退款收入调整</option></select></label><p class="text-xs text-gray-500 sm:col-span-2">填写“账单实际金额减已入账金额”的差额；正数增加成本，负数减少成本。实际付款另登记费用时需选择仅现金付款，避免重复成本。</p></template>
        </template>
        <template v-if="type === 'annotation' && target">
          <template v-if="target.event_type === 'opening_unknown'">
            <p class="text-sm sm:col-span-2">期初总额度 {{ target.payload.credits }}，冻结 {{ target.payload.frozen || '0' }}。三种组成之和必须与总额度一致。</p>
            <label class="block text-sm">付费额度<input v-model="paid" required inputmode="decimal" class="input mt-1" /></label><label class="block text-sm">赠送额度<input v-model="gift" required inputmode="decimal" class="input mt-1" /></label><label class="block text-sm">仍未知额度<input v-model="unknown" required inputmode="decimal" class="input mt-1" /></label>
          </template>
          <label v-if="target.event_type === 'wallet'" class="block text-sm">来源分类<select v-model="classification" class="input mt-1"><option value="paid">有凭据的付费余额</option><option value="gift">赠送</option><option value="usage">确认已消费</option></select></label>
          <label v-if="target.event_type === 'user_subscriptions'" class="block text-sm">赠送资金比例（0–1）<input v-model="giftShare" inputmode="decimal" class="input mt-1" /></label>
          <p class="text-xs text-gray-500 sm:col-span-2">补录更新报表修订号，不产生现金收支、不修改原始用户余额。人民币价值应有实际收款或采购凭据支持。</p>
        </template>
      </div>
      <label v-if="type === 'expense'" class="flex items-center gap-2 text-sm"><input v-model="cashOnly" type="checkbox" />仅登记已入账成本的实际付款（如后付费账单）</label><label v-if="type === 'expense'" class="flex items-center gap-2 text-sm"><input v-model="openingExpense" type="checkbox" :disabled="cashOnly" />期初预付费用剩余价值（不新增现金付款）</label>
      <details v-if="type !== 'annotation' && type !== 'reversal'" class="text-sm"><summary class="cursor-pointer text-gray-500">原币与汇率凭据（可选）</summary><div class="mt-3 grid gap-3 sm:grid-cols-3"><label>原币<input v-model="currency" class="input mt-1" placeholder="CNY / USD / EUR" /></label><label>原币金额<input v-model="originalAmount" inputmode="decimal" class="input mt-1" /></label><label>实际登记汇率<input v-model="fxRate" inputmode="decimal" class="input mt-1" /></label></div><p class="mt-2 text-xs text-gray-500">经营金额以顶部实际人民币金额为准；原币与汇率作为凭据保存，不使用今天的汇率回算历史。</p></details>
      <label class="block text-sm">凭据 / 原因说明<textarea v-model="notes" required maxlength="2000" class="input mt-1" rows="3" placeholder="填写订单号、供应商账单号或业务原因" /></label>
      <p v-if="error" role="alert" class="text-sm text-red-600">{{ error }}</p>
    </form>
    <template #footer><button class="btn btn-secondary" :disabled="saving" @click="emit('close')">取消</button><button form="business-record-form" type="submit" class="btn btn-primary" :disabled="saving">{{ saving ? '正在登记…' : '登记并保留凭据' }}</button></template>
  </BaseDialog>
</template>
<script setup lang="ts">
import { computed, ref } from 'vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import BusinessEntitySelect from './BusinessEntitySelect.vue'
import { businessAPI, type BusinessEvent, type BusinessPool, type BusinessRecordInput } from '@/api/admin/business'
import { businessKindLabels, ledgerError, localDateTime } from '@/utils/business-ledger'
const props = defineProps<{ type: string; pools: BusinessPool[]; target?: BusinessEvent }>()
const emit = defineEmits<{ close: []; saved: [] }>()
const currency = ref('CNY'), originalAmount = ref(''), fxRate = ref('')
const at = ref(localDateTime()), amount = ref(''), notes = ref(''), credits = ref(''), userID = ref(0), accountID = ref(0), groupID = ref(0), poolID = ref<number | string | boolean | null>(null)
const paid = ref('0'), gift = ref('0'), unknown = ref(String(props.target?.payload.credits || '0')), giftShare = ref('0'), classification = ref('paid')
const openingExpense = ref(false)
const billMode = ref(false)
const allPoolOptions = computed(() => props.pools.map(p => ({ value: p.id, label: p.name })))
const applyBalance = ref(false), accrue = ref(false), cashOnly = ref(false), starts = ref(localDateTime()), ends = ref(''), category = ref('account_subscription'), entryKind = ref('usage_cost')
const saving = ref(false), error = ref('')
const poolOptions = computed(() => props.pools.filter(p => p.mode === 'prepaid').map(p => ({ value: p.id, label: `${p.name} · ${p.unit}` })))
const selectedPool = computed(() => props.pools.find(p => p.id === Number(poolID.value)))
let request: BusinessRecordInput | null = null
let submittedSignature = ''
async function save() {
  if (saving.value) return
  error.value = ''
  try {
    if (!['reversal', 'expense_stop', 'supplier_loss'].includes(props.type) && !/^-?\d+(\.\d{1,8})?$/.test(amount.value)) throw new Error('人民币金额最多保留 8 位小数')
    const payload: Record<string, unknown> = { amount_cny: amount.value || '0', notes: notes.value, currency: currency.value.trim().toUpperCase() }
    if (originalAmount.value) payload.original_amount = originalAmount.value
    if (fxRate.value) payload.fx_rate = fxRate.value
    if (props.type === 'receipt') Object.assign(payload, { apply_balance: applyBalance.value, credits: credits.value || '0' })
    if (['purchase', 'opening_pool', 'supplier_refund', 'supplier_loss'].includes(props.type)) Object.assign(payload, { pool_id: Number(poolID.value), credits: credits.value })
    if (['expense', 'reconciliation', 'adjustment'].includes(props.type)) {
      Object.assign(payload, { account_id: accountID.value, group_id: groupID.value, category: category.value })
      if (props.type === 'expense') {
        payload.cash_only = cashOnly.value
        payload.opening = openingExpense.value && !cashOnly.value
        if (accrue.value && !cashOnly.value) { if (!ends.value || ends.value <= starts.value) throw new Error('服务期结束必须晚于开始'); Object.assign(payload, { starts_at: new Date(starts.value).toISOString(), ends_at: new Date(ends.value).toISOString() }) }
      } else {
        payload.entry_kind = entryKind.value
        if (props.type === 'reconciliation' && billMode.value) {
          if (!ends.value || ends.value <= starts.value) throw new Error('账单期结束必须晚于开始')
          Object.assign(payload, { pool_id: Number(poolID.value), bill_amount_cny: amount.value, starts_at: new Date(starts.value).toISOString(), ends_at: new Date(ends.value).toISOString() })
        }
      }
    }
    if (props.type === 'annotation' && props.target) {
      const fields: Record<string, unknown> = { amount_cny: amount.value }
      if (props.target.event_type === 'opening_unknown') Object.assign(fields, { paid_credits: paid.value, gift_credits: gift.value, unknown_credits: unknown.value })
      if (props.target.event_type === 'wallet') fields.classification = classification.value
      if (props.target.event_type === 'user_subscriptions') fields.gift_share = giftShare.value
      if (props.target.event_type === 'usage') { delete fields.amount_cny; fields.actual_supplier_cost_cny = amount.value }
      Object.assign(payload, { source_event_id: props.target.id, fields })
    }
    if (props.type === 'expense_stop' && props.target) payload.source_event_id = props.target.id
    if (props.type === 'reversal' && props.target) payload.reverses_id = props.target.id
    const body = { type: props.type, occurred_at: new Date(at.value).toISOString(), user_id: userID.value || props.target?.user_id || 0, payload }
    const signature = JSON.stringify(body)
    // Keep the exact same key/body after a network failure. Changed form => new request.
    if (!request || signature !== submittedSignature) { request = { ...body, idempotency_key: crypto.randomUUID() }; submittedSignature = signature }
    saving.value = true
    await businessAPI.record(request)
    emit('saved')
  } catch (e) { error.value = ledgerError(e) }
  finally { saving.value = false }
}
</script>
