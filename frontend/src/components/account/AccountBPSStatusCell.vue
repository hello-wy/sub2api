<template>
  <div v-if="status !== 'not_applicable'" class="flex flex-col items-start gap-1">
    <span
      class="badge whitespace-nowrap text-xs"
      :class="statusClass"
      :title="status === 'disabled_403' ? t('admin.accounts.bpsStatus.autoDisabledHint') : undefined"
      data-testid="bps-status"
    >
      {{ t(`admin.accounts.bpsStatus.${status}`) }}
    </span>
    <time
      v-if="disabledAt"
      :datetime="disabledAt"
      :title="t('admin.accounts.bpsStatus.disabledAt', { time: formatDateTime(disabledAt) })"
      class="whitespace-nowrap text-[11px] text-gray-500 dark:text-gray-400"
      data-testid="bps-disabled-at"
    >
      {{ formatDateTime(disabledAt) }}
    </time>
  </div>
  <span
    v-else
    class="whitespace-nowrap text-xs text-gray-400 dark:text-dark-500"
    :title="t('admin.accounts.bpsStatus.notApplicableHint')"
    data-testid="bps-not-applicable"
  >
    {{ t('admin.accounts.bpsStatus.notApplicable') }}
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { formatDateTime } from '@/utils/format'
import type { Account } from '@/types'

const props = defineProps<{
  account: Pick<Account, 'platform' | 'type' | 'parent_account_id' | 'credentials' | 'excel_bps_status' | 'extra'>
}>()
const { t } = useI18n()
const status = computed(() => {
  const account = props.account
  if (account.excel_bps_status) return account.excel_bps_status

  // Older servers omit excel_bps_status. Derive the switch from their existing
  // account fields; an absent new DTO field does not mean BPS is unsupported.
  if (account.platform !== 'openai' || account.type !== 'oauth' || account.parent_account_id != null) {
    return 'not_applicable'
  }
  const authMode = typeof account.credentials?.auth_mode === 'string'
    ? account.credentials.auth_mode.trim().toLowerCase() : ''
  const legacyAuthMode = typeof account.credentials?.openai_auth_mode === 'string'
    ? account.credentials.openai_auth_mode.trim().toLowerCase() : ''
  const isPersonalAccessToken = (value: string) => value === 'personalaccesstoken' || value === 'personal_access_token'
  if (authMode === 'agentidentity' || isPersonalAccessToken(authMode) || isPersonalAccessToken(legacyAuthMode)) {
    return 'not_applicable'
  }
  if (account.extra?.openai_excel_bps === true) return 'enabled'
  if (account.extra?.openai_excel_bps_disabled_reason === 'http_403') return 'disabled_403'
  return 'disabled'
})
const statusClass = computed(() => ({
  enabled: 'badge-success',
  disabled: 'badge-gray',
  disabled_403: 'badge-warning',
  not_applicable: 'badge-gray'
})[status.value])
const disabledAt = computed(() => {
  const value = props.account.extra?.openai_excel_bps_disabled_at
  return status.value === 'disabled_403' && typeof value === 'string' && Number.isFinite(Date.parse(value))
    ? value
    : null
})
</script>
