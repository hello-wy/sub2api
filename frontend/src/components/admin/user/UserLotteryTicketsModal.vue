<template>
  <BaseDialog :show="show" :title="t('admin.users.adjustLotteryTickets')" width="narrow" @close="$emit('close')">
    <form v-if="user" id="lottery-tickets-form" class="space-y-5" @submit.prevent="submit">
      <div class="flex items-center gap-3 rounded-xl bg-gray-50 p-4 dark:bg-dark-700">
        <div class="flex h-10 w-10 items-center justify-center rounded-full bg-primary-100"><span class="text-lg font-medium text-primary-700">{{ user.email.charAt(0).toUpperCase() }}</span></div>
        <div class="flex-1"><p class="font-medium text-gray-900 dark:text-gray-100">{{ user.email }}</p><p class="text-sm text-gray-500 dark:text-gray-400">{{ t('admin.users.currentLotteryTickets') }}: {{ availableTickets }}</p></div>
      </div>
      <div>
        <label class="input-label">{{ t('admin.users.targetLotteryTickets') }}</label>
        <input v-model.number="form.targetTickets" type="number" min="0" step="1" required class="input" />
      </div>
      <div>
        <label class="input-label">{{ t('admin.users.lotteryTicketReason') }}</label>
        <textarea v-model="form.reason" rows="3" maxlength="500" :required="ticketDelta !== 0" class="input" />
      </div>
      <div v-if="form.targetTickets !== null" class="rounded-xl border border-blue-200 bg-blue-50 p-4 dark:border-blue-800 dark:bg-blue-950">
        <div class="flex items-center justify-between text-sm"><span class="text-gray-700 dark:text-gray-300">{{ t('admin.users.newLotteryTickets') }}:</span><span class="font-bold text-gray-900 dark:text-gray-100">{{ form.targetTickets }}</span></div>
      </div>
    </form>
    <template #footer>
      <div class="flex justify-end gap-3">
        <button class="btn btn-secondary" @click="$emit('close')">{{ t('common.cancel') }}</button>
        <button type="submit" form="lottery-tickets-form" :disabled="submitting || !isValidTarget || (ticketDelta !== 0 && !form.reason.trim())" class="btn btn-primary">{{ submitting ? t('common.saving') : t('common.confirm') }}</button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import type { AdminUser } from '@/types'
import BaseDialog from '@/components/common/BaseDialog.vue'

const props = defineProps<{ show: boolean; user: AdminUser | null; availableTickets: number }>()
const emit = defineEmits(['close', 'success'])
const { t } = useI18n()
const appStore = useAppStore()
const submitting = ref(false)
const operationIdempotencyKey = ref('')
const form = reactive({ targetTickets: null as number | null, reason: '' })

const isValidTarget = computed(() => Number.isInteger(form.targetTickets) && form.targetTickets !== null && form.targetTickets >= 0)
const ticketDelta = computed(() => isValidTarget.value ? form.targetTickets! - props.availableTickets : 0)

watch(() => props.show, (visible) => {
  if (!visible) return
  form.targetTickets = props.availableTickets
  form.reason = ''
  operationIdempotencyKey.value = ''
})
watch([() => form.targetTickets, () => form.reason], () => {
  if (!submitting.value) operationIdempotencyKey.value = ''
})

const submit = async () => {
  if (!props.user || !isValidTarget.value) {
    appStore.showError(t('admin.users.lotteryTicketTargetRequired'))
    return
  }
  if (ticketDelta.value === 0) {
    emit('close')
    return
  }
  if (!form.reason.trim()) {
    appStore.showError(t('admin.users.lotteryTicketReasonRequired'))
    return
  }

  const operation = ticketDelta.value > 0 ? 'add' : 'subtract'
  const count = Math.abs(ticketDelta.value)
  submitting.value = true
  try {
    if (!operationIdempotencyKey.value) {
      operationIdempotencyKey.value = globalThis.crypto?.randomUUID?.() ?? `${Date.now()}-${Math.random().toString(36).slice(2)}`
    }
    await adminAPI.users.adjustLotteryTickets(props.user.id, count, operation, form.reason.trim(), operationIdempotencyKey.value)
    appStore.showSuccess(t('common.success'))
    emit('success')
    emit('close')
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('common.error'))
  } finally {
    submitting.value = false
  }
}
</script>
