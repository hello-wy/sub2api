<template>
  <span v-if="!isSupportedAccount" class="text-sm text-gray-400 dark:text-dark-500">-</span>
  <span
    v-else
    data-testid="openai-risk-control-status"
    :data-status="snapshot?.status ?? 'unchecked'"
    :class="['inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-xs font-medium', statusMeta.className]"
    :title="statusTitle"
  >
    <Icon :name="statusMeta.icon" size="xs" :stroke-width="2" />
    {{ t(statusMeta.labelKey) }}
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { AccountListItem } from '@/types'
import { formatDateTime } from '@/utils/format'

const props = defineProps<{
  account: AccountListItem
}>()

const { t } = useI18n()

const isSupportedAccount = computed(() =>
  props.account.platform === 'openai'
  && props.account.type === 'oauth'
  && props.account.parent_account_id == null
)
const snapshot = computed(() => props.account.extra?.openai_risk_control)

const statusMeta = computed(() => {
  switch (snapshot.value?.status) {
    case 'normal':
      return {
        labelKey: 'admin.accounts.riskControl.normal',
        icon: 'checkCircle' as const,
        className: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/35 dark:text-emerald-300'
      }
    case 'suspected':
      return {
        labelKey: 'admin.accounts.riskControl.suspected',
        icon: 'exclamationTriangle' as const,
        className: 'bg-amber-100 text-amber-800 dark:bg-amber-900/35 dark:text-amber-300'
      }
    case 'abnormal':
      return {
        labelKey: 'admin.accounts.riskControl.abnormal',
        icon: 'exclamationTriangle' as const,
        className: 'bg-red-100 text-red-700 dark:bg-red-900/35 dark:text-red-300'
      }
    case 'missing':
      return {
        labelKey: 'admin.accounts.riskControl.missing',
        icon: 'questionCircle' as const,
        className: 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
      }
    default:
      return {
        labelKey: 'admin.accounts.riskControl.unchecked',
        icon: 'infoCircle' as const,
        className: 'bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-gray-400'
      }
  }
})

const statusTitle = computed(() => {
  if (!snapshot.value) return t('admin.accounts.riskControl.uncheckedTitle')
  const key = snapshot.value.status === 'suspected'
    ? 'admin.accounts.riskControl.badgeTitle'
    : snapshot.value.status === 'abnormal'
      ? 'admin.accounts.riskControl.abnormalBadgeTitle'
      : 'admin.accounts.riskControl.detailTitle'
  return t(key, {
    length: snapshot.value.state_length ?? '-',
    time: formatDateTime(snapshot.value.checked_at)
  })
})
</script>
