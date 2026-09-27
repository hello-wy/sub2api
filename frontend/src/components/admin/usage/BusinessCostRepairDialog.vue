<template>
  <BaseDialog :show="true" title="补算历史缺失成本" width="wide" :show-close-button="!busy" :close-on-escape="!busy" @close="emit('close')">
    <div class="space-y-4">
      <p class="text-sm text-gray-600 dark:text-dark-300">按调用发生时有效的账号绑定和价格补齐缺失记录。已保存的价格、实际成本和已核对账单会保留；采购不足和月租缺失仍需登记相应凭据。</p>
      <p class="text-sm font-medium">范围：{{ scopeLabel }}</p>
      <div class="grid gap-4 sm:grid-cols-2">
        <label class="text-sm">开始时间（本机时区）<input v-model="starts" type="datetime-local" class="input mt-1" :disabled="busy" /></label>
        <label class="text-sm">结束时间（不含）<input v-model="ends" type="datetime-local" class="input mt-1" :disabled="busy" /></label>
      </div>
      <label class="block text-sm">凭据与适用说明<textarea v-model="notes" class="input mt-1" maxlength="2000" rows="2" placeholder="例如：供应商合同自 9 月 1 日生效，补齐该期间遗漏的成本" :disabled="busy" /></label>
      <div v-if="preview" aria-label="补算预览" class="rounded-lg border border-gray-200 p-4 text-sm dark:border-dark-700">
        <p>可补算 <strong>{{ preview.repairable_count }}</strong> 次请求</p>
        <p class="mt-2 text-gray-500">仍缺账号绑定 {{ preview.missing_binding_count }} 次 · 仍缺价格 {{ preview.missing_rule_count }} 次 · 受实际成本或账单保护 {{ preview.protected_count }} 次</p>
        <p v-if="!preview.repairable_count" class="mt-2">请先补齐适用期间内的成本配置，再重新预览。</p>
        <p v-else class="mt-2">提交后统一在后台计算，可关闭窗口。暂估成本仍需按供应商账单核对。</p>
      </div>
      <p v-if="error" role="alert" class="text-sm text-red-600">{{ error }}</p>
    </div>
    <template #footer>
      <button class="btn btn-secondary" :disabled="busy" @click="emit('close')">取消</button>
      <button class="btn btn-secondary" :disabled="busy" @click="loadPreview">{{ busy && !submitting ? '正在预览…' : '预览影响范围' }}</button>
      <button class="btn btn-primary" :disabled="busy || !preview?.repairable_count || !notes.trim()" @click="submit">{{ submitting ? '正在提交…' : '确认并后台补算' }}</button>
    </template>
  </BaseDialog>
</template>
<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { businessAPI, type BusinessIssue, type BusinessRepairInput, type BusinessRepairJob, type BusinessRepairPreview } from '@/api/admin/business'
import { ledgerError } from '@/utils/business-ledger'
const props = defineProps<{ issue?: BusinessIssue; startDate: string; endDate: string; startAt?: string; endAt?: string }>()
const emit = defineEmits<{ close: []; queued: [job: BusinessRepairJob] }>()
function local(value: Date) { return new Date(value.getTime() - value.getTimezoneOffset() * 60000).toISOString().slice(0, 16) }
const starts = ref(local(props.issue ? new Date(props.issue.first_at) : new Date(props.startAt || props.startDate + 'T00:00:00')))
const end = props.issue ? new Date(props.issue.last_at) : new Date(props.endAt || props.endDate + 'T00:00:00')
if (props.issue) end.setMinutes(end.getMinutes() + 1); else if (!props.endAt) end.setDate(end.getDate() + 1)
const ends = ref(local(end)), notes = ref(''), busy = ref(false), submitting = ref(false), error = ref('')
const preview = ref<BusinessRepairPreview | null>(null)
const scopeLabel = computed(() => props.issue ? [props.issue.name, props.issue.model].filter(Boolean).join(' · ') : '所有账号的缺失成本')
let request: BusinessRepairInput | null = null
let signature = ''
const scope = (): BusinessRepairInput => ({ starts_at: new Date(starts.value).toISOString(), ends_at: new Date(ends.value).toISOString(), account_ids: props.issue?.account_id ? [props.issue.account_id] : [], pool_id: props.issue?.pool_id || 0, model: props.issue?.model || '' })
watch([starts, ends], () => { preview.value = null; request = null })
async function loadPreview() {
  busy.value = true; error.value = ''; preview.value = null
  try {
    if (!starts.value || !ends.value || ends.value <= starts.value) throw new Error('结束时间必须晚于开始时间')
    preview.value = await businessAPI.previewRepair(scope()); request = null
  } catch (e) { error.value = ledgerError(e) } finally { busy.value = false }
}
async function submit() {
  if (busy.value || !preview.value?.repairable_count) return
  busy.value = true; submitting.value = true; error.value = ''
  try {
    const body = { ...scope(), through_event_id: preview.value.through_event_id, fingerprint: preview.value.fingerprint, notes: notes.value.trim() }
    const next = JSON.stringify(body)
    if (!request || signature !== next) { request = { ...body, idempotency_key: crypto.randomUUID() }; signature = next }
    emit('queued', await businessAPI.repair(request))
  } catch (e) { error.value = ledgerError(e) } finally { busy.value = false; submitting.value = false }
}
</script>
