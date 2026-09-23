import { apiClient } from '../client'
import type { CodexTicketPlan } from './codexTickets'

export interface AccountTicketDefaults {
  enabled: boolean
  ticket_plan: CodexTicketPlan
}

export async function getAccountTicketDefaults(signal?: AbortSignal): Promise<AccountTicketDefaults> {
  const { data } = await apiClient.get<AccountTicketDefaults>('/admin/settings/codex-ticket-defaults', { signal })
  return data
}

export async function saveAccountTicketDefaults(value: AccountTicketDefaults): Promise<AccountTicketDefaults> {
  const { data } = await apiClient.put<AccountTicketDefaults>('/admin/settings/codex-ticket-defaults', value)
  return data
}
