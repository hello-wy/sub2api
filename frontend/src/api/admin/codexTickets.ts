// Account STATE controls adapted from wangyunjeff/sub2api-state-kit (LGPL-3.0).
import { apiClient } from '../client'

export type CodexTicketPlan = 'pro' | 'team'

export interface CodexAccountTicketWatchdog {
  enabled: boolean
  trigger_count: number
  last_reason?: 'model_mismatch' | 'state_312'
  last_triggered_at?: string
}

export interface CodexAccountTicketStatus {
  enabled: boolean
  require_verified?: boolean
  global_enabled: boolean
  model: string
  ticket_plan: CodexTicketPlan
  target_length: number
  actual_length?: number
  proxy_configured: boolean
  proxy_display: string
  fixed_proxy_configured: boolean
  state: 'disabled' | 'global_disabled' | 'waiting' | 'harvesting' | 'ready' | 'error' | 'authentication_blocked'
  remaining_seconds: number
  // These fields are optional so older API responses and test fixtures remain valid.
  ticket_usable?: boolean
  captured_at?: string
  verified_at?: string
  verified_model?: string
  verification_scope?: 'fixed_business_route'
  expiry_kind?: 'local_cache_ttl'
  credential_current?: boolean
  identity_current?: boolean
  authentication_blocked?: boolean
  expires_at?: string
  refreshing?: boolean
  retry_after?: string
  last_error: string
  attempts: number
  watchdog: CodexAccountTicketWatchdog
}

export interface CodexAccountTicketSettings {
  enabled: boolean
  model?: string
  ticket_plan?: CodexTicketPlan
}

export async function getCodexAccountTicket(accountId: number, signal?: AbortSignal): Promise<CodexAccountTicketStatus> {
  const { data } = await apiClient.get<CodexAccountTicketStatus>(`/admin/accounts/${accountId}/codex-ticket`, { signal })
  return data
}

export async function saveCodexAccountTicket(accountId: number, settings: CodexAccountTicketSettings): Promise<CodexAccountTicketStatus> {
  const { data } = await apiClient.put<CodexAccountTicketStatus>(`/admin/accounts/${accountId}/codex-ticket`, settings)
  return data
}

export async function harvestCodexAccountTicket(accountId: number): Promise<CodexAccountTicketStatus> {
  const { data } = await apiClient.post<CodexAccountTicketStatus>(`/admin/accounts/${accountId}/codex-ticket/harvest`)
  return data
}

export interface CodexTicketGlobalSettings {
  enabled: boolean
  harvest_proxy_configured: boolean
  harvest_proxy_display: string
}

export interface CodexTicketGlobalUpdate {
  enabled: boolean
  harvest_proxy_url?: string
  clear_proxy?: boolean
}

export async function getCodexTicketGlobalSettings(signal?: AbortSignal): Promise<CodexTicketGlobalSettings> {
  const { data } = await apiClient.get<CodexTicketGlobalSettings>('/admin/settings/codex-ticket', { signal })
  return data
}

export async function saveCodexTicketGlobalSettings(settings: CodexTicketGlobalUpdate): Promise<CodexTicketGlobalSettings> {
  const { data } = await apiClient.put<CodexTicketGlobalSettings>('/admin/settings/codex-ticket', settings)
  return data
}

export interface CodexTicketBatchRequest extends CodexAccountTicketSettings {
  account_ids: number[]
  harvest: boolean
}

export interface CodexTicketBatchItem {
  account_id: number
  logical_account_id: number
  saved: boolean
  outcome: 'saved' | 'harvesting' | 'ready' | 'waiting' | 'cooldown' | 'skipped' | 'error'
  message: string
  status?: CodexAccountTicketStatus
}

export interface CodexTicketBatchResult {
  selected_accounts: number
  total_channels: number
  saved: number
  harvesting: number
  waiting: number
  cooldown: number
  skipped: number
  failed: number
  results: CodexTicketBatchItem[]
}

export async function configureCodexTicketBatch(settings: CodexTicketBatchRequest): Promise<CodexTicketBatchResult> {
  const { data } = await apiClient.post<CodexTicketBatchResult>('/admin/accounts/codex-ticket/batch', settings, { timeout: 120_000 })
  return data
}
