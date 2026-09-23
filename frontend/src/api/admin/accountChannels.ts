import apiClient from '../client'
import type { AccountIPChannel, AccountUsageStatsResponse } from '@/types'

export interface AddAccountIPChannelsRequest {
  proxy_ids: number[]
  concurrency?: number
  priority?: number
}

export interface UpdateAccountIPChannelRequest {
  proxy_id?: number
  concurrency?: number
  priority?: number
  schedulable?: boolean
  load_factor?: number
}

export async function getAccountIPChannelStats(accountId: number, channelId: number, days = 30): Promise<AccountUsageStatsResponse> {
  const { data } = await apiClient.get<AccountUsageStatsResponse>(`/admin/accounts/${accountId}/ip-channels/${channelId}/stats`, { params: { days } })
  return data
}

export async function listAccountIPChannels(accountId: number, signal?: AbortSignal): Promise<AccountIPChannel[]> {
  const { data } = await apiClient.get<{ items: AccountIPChannel[] }>(`/admin/accounts/${accountId}/ip-channels`, { signal })
  return data.items
}

export async function addAccountIPChannels(accountId: number, body: AddAccountIPChannelsRequest): Promise<void> {
  await apiClient.post(`/admin/accounts/${accountId}/ip-channels`, body)
}

export async function updateAccountIPChannel(accountId: number, channelId: number, body: UpdateAccountIPChannelRequest): Promise<void> {
  await apiClient.patch(`/admin/accounts/${accountId}/ip-channels/${channelId}`, body)
}

export async function removeAccountIPChannel(accountId: number, channelId: number): Promise<void> {
  await apiClient.delete(`/admin/accounts/${accountId}/ip-channels/${channelId}`)
}

export async function recoverAccountIPChannel(accountId: number, channelId: number): Promise<void> {
  await apiClient.post(`/admin/accounts/${accountId}/ip-channels/${channelId}/recover-state`)
}
