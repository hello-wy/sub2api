<template>
  <div v-if="supported" class="min-w-[12rem] space-y-1 text-xs" data-testid="account-ticket-cell">
    <div v-for="item in visibleItems" :key="item.id" :data-testid="`account-ticket-${item.id}`" :title="item.status?.last_error || ''">
      <span v-if="items.length > 1" class="mr-1 text-gray-500">{{ item.name }}:</span>
      <span :class="usable(item.status) ? 'text-emerald-600 dark:text-emerald-400' : 'text-gray-500 dark:text-gray-400'">
        {{ ticketLabel(item.status) }}
      </span>
      <span class="ml-1 text-gray-500">{{ stateLabel(item.status) }}</span>
      <span v-if="usable(item.status) && item.status?.verified_at" class="ml-1 text-gray-500" :title="item.status.verified_at" data-testid="ticket-verified-at">{{ t('admin.accounts.stateTicket.verifiedAt', { time: formatDateTime(item.status.verified_at) }) }}</span>
      <span v-if="usable(item.status)" class="ml-1 font-mono tabular-nums text-emerald-600 dark:text-emerald-400" :title="item.status?.expires_at" data-testid="ticket-time">{{ remaining(item.status!.expires_at!) }}</span>
      <span v-else-if="retryTime(item.status)" class="ml-1 font-mono tabular-nums text-amber-600">{{ remaining(item.status!.retry_after!) }}</span>
    </div>
    <div class="flex gap-3">
      <button v-if="items.length > 3" type="button" class="text-primary-600 hover:underline" @click="expanded = !expanded">{{ expanded ? t('admin.accounts.stateTicket.list.collapse') : t('admin.accounts.stateTicket.list.more', { count: items.length }) }}</button>
      <button type="button" class="text-primary-600 hover:underline" data-testid="account-ticket-configure" @click="emit('configure')">{{ t('admin.accounts.stateTicket.list.configure') }}</button>
    </div>
  </div>
  <span v-else class="text-gray-400">—</span>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { AccountListItem } from '@/types'
import type { CodexAccountTicketStatus } from '@/api/admin/codexTickets'
import { ticketVerificationBlocked, ticketVerificationStateKey } from '@/utils/codexTicketStatus'
import { formatDateTime } from '@/utils/format'
import { sortAccountIPChannels } from '@/utils/accountIPChannels'
const props = defineProps<{ account: AccountListItem; now: number }>()
const emit = defineEmits<{ configure: [] }>()
const { t } = useI18n()
const expanded = ref(false)
const supported = computed(() => props.account.platform === 'openai' && ['oauth', 'setup-token'].includes(props.account.type) && !props.account.parent_account_id)
const items = computed(() => props.account.ip_channels?.length
  ? sortAccountIPChannels(props.account.ip_channels).map(channel => ({ id: channel.id, name: channel.proxy?.name || `#${channel.id}`, status: channel.codex_ticket }))
  : [{ id: props.account.id, name: '', status: props.account.codex_ticket }])
const visibleItems = computed(() => expanded.value ? items.value : items.value.slice(0, 3))
const future = (value?: string) => !!value && Date.parse(value) > props.now
const usable = (status?: CodexAccountTicketStatus) => status?.ticket_usable === true && !ticketVerificationBlocked(status) && future(status.expires_at)
const retryTime = (status?: CodexAccountTicketStatus) => future(status?.retry_after)
function remaining(value: string) {
  const seconds = Math.max(0, Math.ceil((Date.parse(value) - props.now) / 1000))
  return `${Math.floor(seconds / 60)}:${String(seconds % 60).padStart(2, '0')}`
}
function ticketLabel(status?: CodexAccountTicketStatus) {
  if (!status) return t('admin.accounts.stateTicket.list.unknown')
  if (!status.enabled) return 'STATE'
  if (usable(status) && status.actual_length) return t('admin.accounts.stateTicket.list.acquired', { length: status.actual_length })
  return t('admin.accounts.stateTicket.list.target', { length: status.target_length })
}
function stateLabel(status?: CodexAccountTicketStatus) {
  if (!status) return ''
  const verificationState = ticketVerificationStateKey(status)
  if (verificationState) return t(`admin.accounts.stateTicket.${verificationState}`)
  if (status.ticket_usable && status.expires_at && !future(status.expires_at)) return t('admin.accounts.stateTicket.list.expired')
  if (retryTime(status) && !usable(status)) return t('admin.accounts.stateTicket.list.cooldown')
  if (status.state === 'harvesting' && usable(status)) return t('admin.accounts.stateTicket.list.renewing')
  return t(`admin.accounts.stateTicket.states.${status.state}`)
}
</script>
