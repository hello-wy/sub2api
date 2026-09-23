import type { AccountIPChannel } from '@/types'
import { activeTemporaryCooldown, upstream429IsCoolingDown } from './upstream429'

export type AccountIPChannelState = 'available' | 'manualPaused' | 'accountPaused' | 'authenticationUnavailable' | 'refreshPending' | 'verificationPending' | 'cooldown' | 'unavailable'

// enabled/logical_enabled are operator intent; schedulable is also changed by
// credential health. Never label a rejected credential as a manual pause.
export function accountIPChannelState(channel: AccountIPChannel, now = Date.now()): AccountIPChannelState {
  const temporary = activeTemporaryCooldown(channel)
  const hasTemporaryCooldown = !!temporary && Date.parse(temporary) > now
  if (hasTemporaryCooldown && channel.temp_unschedulable_reason?.startsWith('token refresh retry exhausted:')) return 'refreshPending'
  const authenticationReason = (reason?: string | null) => /^(OAuth 401:|Authentication failed \(401\):|OAuth refresh rejected;|Token revoked \(401\):)/.test(reason ?? '')
  if (channel.codex_ticket?.authentication_blocked ||
      (channel.status === 'error' && authenticationReason(channel.error_message)) ||
      (hasTemporaryCooldown && authenticationReason(channel.temp_unschedulable_reason))) return 'authenticationUnavailable'
  if (channel.enabled === false) return 'manualPaused'
  if (channel.logical_enabled === false) return 'accountPaused'
  const ticket = channel.codex_ticket
  if (ticket?.enabled && (ticket.ticket_usable !== true || ticket.credential_current === false || ticket.identity_current === false || !ticket.expires_at || Date.parse(ticket.expires_at) <= now)) return 'verificationPending'
  if (upstream429IsCoolingDown(channel, now) || hasTemporaryCooldown || (channel.overload_until && Date.parse(channel.overload_until) > now)) return 'cooldown'
  if (!channel.schedulable || channel.status !== 'active' || channel.healthy === false) return 'unavailable'
  return 'available'
}

export function accountIPChannelStateKey(state: AccountIPChannelState): string {
  if (state === 'available') return 'admin.accounts.ipChannels.enabled'
  if (state === 'unavailable') return 'admin.accounts.ipChannels.unavailable'
  return `accountIPChannelState.${state}`
}

export function accountIPChannelStateClass(state: AccountIPChannelState): string {
  if (state === 'available') return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300'
  if (state === 'manualPaused' || state === 'accountPaused') return 'bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-gray-400'
  if (state === 'verificationPending' || state === 'refreshPending' || state === 'cooldown') return 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
  return 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300'
}
