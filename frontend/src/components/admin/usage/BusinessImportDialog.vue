<template>
  <BaseDialog :show="true" title="批量导入收支台账" width="wide" :show-close-button="!running" :close-on-escape="!running" @close="emit('close')">
    <div class="space-y-4">
      <p class="text-sm text-gray-500">UTF-8 CSV，每次最多 500 行。先核对预览再导入；每行幂等标识固定，失败后可安全重试同一文件。批量收款不会增加用户余额。</p>
      <button class="btn btn-secondary" @click="downloadTemplate">下载 CSV 模板</button>
      <input type="file" accept=".csv,text/csv" :disabled="running" aria-label="选择台账 CSV" @change="read" />
      <p v-if="error" role="alert" class="text-sm text-red-600">{{ error }}</p>
      <div v-if="rows.length" class="max-h-72 overflow-auto"><table class="w-full text-left text-sm"><thead><tr><th>行</th><th>类型</th><th>人民币</th><th>凭据</th><th>状态</th></tr></thead><tbody><tr v-for="(row, i) in rows" :key="row.idempotency_key" class="border-t dark:border-dark-700"><td class="py-2">{{ i + 2 }}</td><td>{{ businessKindLabels[row.type] }}</td><td>{{ cny(String(row.payload.amount_cny)) }}</td><td>{{ row.payload.notes }}</td><td>{{ done.has(row.idempotency_key) ? '已登记' : '待登记' }}</td></tr></tbody></table></div>
      <p v-if="rows.length" class="text-sm">已登记 {{ done.size }} / {{ rows.length }} 行</p>
    </div>
    <template #footer><button class="btn btn-secondary" :disabled="running" @click="emit('close')">关闭</button><button class="btn btn-primary" :disabled="running || !rows.length || done.size === rows.length" @click="importRows">{{ running ? '正在逐行登记…' : '确认导入 / 继续重试' }}</button></template>
  </BaseDialog>
</template>
<script setup lang="ts">
import { ref } from 'vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { businessAPI, type BusinessRecordInput } from '@/api/admin/business'
import { businessKindLabels, cny, ledgerError, parseLedgerCSV } from '@/utils/business-ledger'
const emit = defineEmits<{ close: []; saved: [] }>()
const rows = ref<BusinessRecordInput[]>([]), done = ref(new Set<string>()), running = ref(false), error = ref('')
async function read(event: Event) {
  error.value = ''; rows.value = []; done.value = new Set()
  const file = (event.target as HTMLInputElement).files?.[0]; if (!file) return
  try { if (file.size > 2 * 1024 * 1024) throw new Error('文件不能超过 2MB'); rows.value = parseLedgerCSV(await file.text()) } catch (e) { error.value = ledgerError(e) }
}
async function importRows() {
  running.value = true; error.value = ''
  try { for (const row of rows.value) if (!done.value.has(row.idempotency_key)) { await businessAPI.record(row); done.value.add(row.idempotency_key) } }
  catch (e) { error.value = `已保留成功记录。${ledgerError(e)}` }
  finally { running.value = false; emit('saved') }
}
function downloadTemplate() {
  const text = '\uFEFFidempotency_key,type,occurred_at,amount_cny,notes,user_id,pool_id,credits,account_id,group_id,starts_at,ends_at,category,entry_kind\r\n'
  const url = URL.createObjectURL(new Blob([text], { type: 'text/csv;charset=utf-8' }))
  const link = document.createElement('a'); link.href = url; link.download = 'business-ledger-template.csv'; link.click(); URL.revokeObjectURL(url)
}
</script>
