<template>
  <BaseDialog :show="show" :title="t(legacy ? 'accountModelMismatch.legacyLabel' : 'accountModelMismatch.title')" width="normal" :close-on-click-outside="!busy" :close-on-escape="!busy" @close="close">
    <div class="space-y-4">
      <div class="rounded-lg border p-4" :class="legacy ? 'border-gray-200 bg-gray-50 text-gray-700 dark:border-gray-700 dark:bg-gray-900 dark:text-gray-300' : 'border-red-200 bg-red-50 text-red-700 dark:border-red-900 dark:bg-red-950/30 dark:text-red-300'">
        <p class="font-medium">{{ account?.name }} · {{ t(legacy ? 'accountModelMismatch.legacyLabel' : quarantined ? 'accountModelMismatch.paused' : 'accountModelMismatch.observed') }}</p>
        <p class="mt-2 text-sm leading-6">{{ t(legacy ? 'accountModelMismatch.legacyExplanation' : quarantined ? 'accountModelMismatch.explanation' : 'accountModelMismatch.observedExplanation') }}</p>
      </div>
      <dl class="grid grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-3 text-sm">
        <dt class="text-gray-500">{{ t('accountModelMismatch.expected') }}</dt><dd class="break-all font-mono">{{ textValue(marker?.expected_model) }}</dd>
        <dt class="text-gray-500">{{ t('accountModelMismatch.actual') }}</dt><dd class="break-all font-mono" :class="legacy ? '' : 'text-red-600 dark:text-red-400'">{{ textValue(marker?.actual_model) }}</dd>
        <dt class="text-gray-500">{{ t('accountModelMismatch.detectedAt') }}</dt><dd>{{ detectedAt }}</dd>
        <dt class="text-gray-500">{{ t('accountModelMismatch.requestId') }}</dt><dd class="break-all font-mono">{{ textValue(marker?.request_id) }}</dd>
      </dl>
      <p v-if="canRestore" class="text-sm leading-6 text-gray-600 dark:text-gray-400">{{ t(legacy ? 'accountModelMismatch.legacyRestoreHint' : 'accountModelMismatch.restoreHint') }}</p>
      <p v-if="error" role="alert" class="text-sm text-red-600 dark:text-red-400">{{ error }}</p>
    </div>
    <template #footer>
      <div class="flex justify-end gap-3">
        <button type="button" class="btn btn-secondary" :disabled="busy" @click="close">{{ t('common.close') }}</button>
        <button v-if="canRestore" type="button" class="btn btn-primary" data-testid="restore-model-mismatch" :disabled="busy || !marker" @click="restore">{{ t(busy ? 'common.loading' : 'accountModelMismatch.confirmRestore') }}</button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { adminAPI } from '@/api/admin'
import type { Account } from '@/types'
import { getAccountModelMismatch, getLegacyAccountModelMismatch, canRestoreLegacyModelMismatch, isAccountModelMismatchQuarantined } from '@/utils/accountModelMismatch'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatDateTime } from '@/utils/format'

const props = defineProps<{ show: boolean; account: Account | null }>()
const emit = defineEmits<{ close: []; restored: [account: Account] }>()
const { t } = useI18n()
const legacy = computed(() => getLegacyAccountModelMismatch(props.account))
const marker = computed(() => getAccountModelMismatch(props.account) ?? legacy.value)
const quarantined = computed(() => isAccountModelMismatchQuarantined(props.account))
const canRestore = computed(() => quarantined.value || canRestoreLegacyModelMismatch(props.account))
const busy = ref(false)
const error = ref('')
let generation = 0
const textValue = (value: unknown): string => typeof value === 'string' && value.trim() ? value : '—'
const detectedAt = computed(() => {
  const value = marker.value?.detected_at
  return typeof value === 'string' && Number.isFinite(Date.parse(value)) ? formatDateTime(value) : '—'
})
watch(() => [props.show, props.account?.id], () => {
  generation++
  busy.value = false
  error.value = ''
})
onBeforeUnmount(() => { generation++ })
function close() { if (!busy.value) emit('close') }
async function restore() {
  if (busy.value || !props.show || !props.account || !canRestore.value) return
  const token = ++generation
  const id = props.account.id
  busy.value = true
  error.value = ''
  try {
    const updated = await adminAPI.accounts.setSchedulable(id, true)
    if (token !== generation) return
    emit('restored', updated)
    emit('close')
  } catch (cause) {
    if (token === generation) error.value = extractApiErrorMessage(cause, t('accountModelMismatch.restoreFailed'))
  } finally {
    if (token === generation) busy.value = false
  }
}
</script>
