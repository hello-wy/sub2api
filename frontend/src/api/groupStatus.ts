import { apiClient } from './client'

export type GroupStatusRange = '24h' | '7d' | '15d' | '30d'
export type GroupServiceState = 'operational' | 'degraded' | 'unavailable' | 'unknown'
export type GroupProbeState = 'success' | 'failed' | 'skipped' | 'running'
export type GroupProbeReasoning = '' | 'minimal' | 'low' | 'medium' | 'high' | 'xhigh'

export interface GroupProbeSummary {
  status: GroupProbeState
  checked_at: string
  latency_ms?: number
  model: string
}

export interface GroupStatusBucket {
  start: string
  end: string
  status: GroupServiceState
  success_rate: number | null
}

export interface GroupServiceStatus {
  group_id: number
  group_name: string
  platform: string
  status: GroupServiceState
  metrics: {
    success_rate: number | null
    output_tokens_per_second: number | null
    ttft_ms: number | null
  }
  buckets: GroupStatusBucket[]
  last_probe?: GroupProbeSummary | null
}

export interface GroupStatusResponse {
  /** Missing during a rolling upgrade means the old enabled response. */
  enabled?: boolean
  updated_at: string
  models: string[]
  groups: GroupServiceStatus[]
}

export function isGroupStatusDisabledError(error: unknown): boolean {
  if (typeof error !== 'object' || error === null || !('reason' in error)) return false
  return error.reason === 'CHANNEL_MONITOR_DISABLED' || error.reason === 'CHANNEL_MONITOR_MODE_MISMATCH'
}

export interface GroupProbeConfig {
  group_id: number
  enabled: boolean
  interval_seconds: number
  model: string
  reasoning_effort: GroupProbeReasoning
  timeout_seconds: number
  max_output_tokens: number
  daily_token_budget: number
  updated_at?: string
  next_run_at?: string | null
}

export interface GroupProbeRun extends GroupProbeSummary {
  id: string
  group_id: number
  finished_at?: string | null
  input_tokens: number
  output_tokens: number
  usage_recorded: boolean
  estimated_cost_usd?: number | null
  scheduled?: boolean
  error_code?: string
}

export type GroupProbeSettings = Omit<GroupProbeConfig, 'group_id' | 'updated_at' | 'next_run_at'>

export async function getGroupStatus(range: GroupStatusRange, model: string, admin: boolean, signal?: AbortSignal) {
  const { data } = await apiClient.get<GroupStatusResponse>(admin ? '/admin/group-status' : '/group-status', {
    params: { range, model: model || undefined },
    signal,
  })
  return data
}

export async function getGroupProbeConfigs(signal?: AbortSignal) {
  const { data } = await apiClient.get<{ items: GroupProbeConfig[] }>('/admin/group-status/probes', { signal })
  return data.items
}

export async function saveGroupProbeConfig(groupID: number, config: GroupProbeSettings) {
  const { data } = await apiClient.put<GroupProbeConfig>(`/admin/group-status/probes/${groupID}`, config)
  return data
}

export async function runGroupProbe(groupID: number) {
  const { data } = await apiClient.post<GroupProbeRun>(`/admin/group-status/probes/${groupID}/run`)
  return data
}

export async function getGroupProbeHistory(groupID: number, signal?: AbortSignal) {
  const { data } = await apiClient.get<{ items: GroupProbeRun[] }>(`/admin/group-status/probes/${groupID}/history`, { signal })
  return data.items
}
