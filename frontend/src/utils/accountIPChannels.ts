import type { Account, AccountIPChannel } from '@/types'
import { activeTemporaryCooldown, upstream429IsCoolingDown } from './upstream429'

export function sortAccountIPChannels(channels: AccountIPChannel[]): AccountIPChannel[] {
  return [...channels].sort((a, b) => a.priority - b.priority || a.id - b.id)
}

type IPChannelAccount = Pick<Account,
  'platform' | 'type' | 'parent_account_id' | 'logical_account_id' | 'ip_channels' | 'proxy_id' | 'extra'
>

export function canManageAccountIPChannels(account: IPChannelAccount): boolean {
  if (account.platform !== 'openai' || account.type !== 'oauth' || account.parent_account_id) {
    return false
  }
  // An existing logical account can outlive its original routing channel.
  if ((account.logical_account_id ?? 0) > 0 || account.ip_channels?.length) {
    return true
  }
  const proxyMode = account.extra?.proxy_mode
  if (typeof proxyMode === 'string' && proxyMode.trim().toLowerCase() === 'random') {
    return false
  }
  return Number.isInteger(account.proxy_id) && (account.proxy_id ?? 0) > 0
}


export function buildAccountIPChannelsRefreshKey(account: Pick<Account, 'ip_channels' | 'channel_count' | 'channel_status' | 'logical_account_id' | 'ip_channels_unavailable'>): string {
  return JSON.stringify([account.ip_channels_unavailable, account.logical_account_id, account.channel_count, account.channel_status,
    [...(account.ip_channels ?? [])].sort((a, b) => a.id - b.id)])
}

export function hasRecoverableIPChannelState(channel: AccountIPChannel): boolean {
  return channel.recoverable === true || channel.status === 'error' || upstream429IsCoolingDown(channel) || !!channel.overload_until || !!activeTemporaryCooldown(channel)
}
