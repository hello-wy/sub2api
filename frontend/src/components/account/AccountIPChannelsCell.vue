<template>
  <div class="flex min-w-[10rem] flex-col items-start gap-1" data-testid="account-ip-channels">
    <button
      v-for="channel in visibleChannels"
      :key="channel.id"
      type="button"
      class="inline-flex max-w-[18rem] items-center gap-1.5 rounded-full px-2 py-0.5 text-xs transition-opacity hover:opacity-80"
      :class="channelClass(channel)"
      :title="channelTitle(channel)"
      :aria-label="channelTitle(channel)"
      @click.stop="emit('manage')"
    >
      <Icon name="grid" size="xs" />
      <span class="shrink-0 font-mono tabular-nums">{{ channel.current_concurrency ?? '—' }} / {{ channel.concurrency > 0 ? channel.concurrency : '∞' }}</span>
      <span class="truncate">{{ channel.proxy?.name || t('admin.accounts.ipChannels.direct') }}</span>
      <span v-if="hasUpstream429Observation(channel)" class="shrink-0 text-amber-700">{{ t(upstream429IsCoolingDown(channel) ? 'admin.accounts.status.rateLimited' : 'admin.accounts.status.upstream429Recorded') }}</span>
      <span v-if="accountIPChannelState(channel) !== 'available'" class="shrink-0" :data-testid="`ip-channel-state-${channel.id}`">{{ t(accountIPChannelStateKey(accountIPChannelState(channel))) }}</span>
    </button>
    <button type="button" class="text-xs text-primary-600 hover:underline dark:text-primary-400" @click.stop="emit('manage')">
      {{ channels.length > 3 ? t('admin.accounts.ipChannels.manageCount', { count: channels.length }) : t('admin.accounts.ipChannels.manage') }}
    </button>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { AccountIPChannel } from '@/types'
import { sortAccountIPChannels } from '@/utils/accountIPChannels'
import { hasUpstream429Observation, upstream429IsCoolingDown } from '@/utils/upstream429'
import { accountIPChannelState, accountIPChannelStateKey, accountIPChannelStateClass } from '@/utils/accountIPChannelState'

const props = defineProps<{ channels: AccountIPChannel[] }>()
const emit = defineEmits<{ manage: [] }>()
const { t } = useI18n()
const visibleChannels = computed(() => sortAccountIPChannels(props.channels).slice(0, 3))
const channelClass = (channel: AccountIPChannel) => {
  const state = accountIPChannelState(channel)
  if (state !== 'available') return accountIPChannelStateClass(state)
  if (channel.current_concurrency == null) return 'bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-gray-400'
  if ((channel.current_concurrency ?? 0) > 0) return 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
  return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300'
}
const channelTitle = (channel: AccountIPChannel) => [
  t(accountIPChannelStateKey(accountIPChannelState(channel))),
  channel.proxy ? `${channel.proxy.name} · ${channel.proxy.host}:${channel.proxy.port}` : t('admin.accounts.ipChannels.direct'),
  t('admin.accounts.ipChannels.load', { current: channel.current_concurrency ?? '—', max: channel.concurrency > 0 ? channel.concurrency : '∞' }),
  hasUpstream429Observation(channel) ? upstream429IsCoolingDown(channel) ? t('admin.accounts.status.rateLimitedUntil', { time: channel.rate_limit_reset_at }) : t('admin.accounts.status.upstream429ObservationHint') : '',
  channel.error_message
].filter(Boolean).join(' · ')
</script>
