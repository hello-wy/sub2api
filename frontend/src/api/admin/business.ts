import { apiClient } from '../client'

export interface BusinessSettings {
 enabled_at: string | null
 timezone: string
 opening_revenue: string
 opening_private_revenue: string
 opening_cost: string
}
export interface BusinessDay {
 date: string
 balance_revenue: string
 subscription_revenue: string
 other_revenue: string
 manual_cost: string
 gift_cost: string
 revenue: string
 cost: string
 known_profit: string
 profit: string | null
 missing: number
}
export interface BusinessCapacity {
 group_id: number; name: string; platform: string; active_subscriptions: number
 available: string | null; known_available: string; unlimited: number
 seven_day_average: string; seven_day_peak: string; expiring_count: number
}
export interface BusinessOverview {
 settings: BusinessSettings; start_date: string; end_date: string; updated_at: string
 summary: BusinessDay; daily: BusinessDay[]
 cumulative_profit: string | null; cumulative_known_profit: string
 positions: {
  balance: string; frozen: string; debt: string; subscription_value: string | null
  known_subscription_value: string; suspended_value: string; suspended_count: number
  active_subscriptions: number; missing_subscriptions: number
 }
 capacity: BusinessCapacity[]
 rewards: { source: string; amount: string }[]
 pending_terms: number; manual_receipts: string
}
export type BusinessRecordKind = 'cost' | 'income' | 'receipt' | 'income_adjustment' | 'cost_adjustment'
export interface BusinessRecordInput {
 source_key: string; kind: BusinessRecordKind; amount: string; date: string; category: string; notes: string
}
export interface BusinessRecord extends BusinessRecordInput {
 id: number; reverses_id: number | null; reversed: boolean; created_at: string
}
export interface BusinessTerm {
 id: number; user_id: number; group_name: string; plan_name: string; funding: 'paid' | 'gift' | 'unknown'
 amount: string | null; quota: string | null; baseline_units: string; used_units: string
 starts_at: string; ends_at: string; opening: boolean
}
export interface BusinessTermInput { amount: string; quota: string; funding: 'paid' | 'gift'; notes: string }
export const businessAPI = {
 overview: async (params: {start_date?: string; end_date?: string}) => (await apiClient.get<BusinessOverview>('/admin/business/overview', {params})).data,
 settings: async () => (await apiClient.get<BusinessSettings>('/admin/business/settings')).data,
 initialize: async (input: Omit<BusinessSettings, 'enabled_at'>) => (await apiClient.post<BusinessSettings>('/admin/business/initialize', input)).data,
 records: async (before = 0) => (await apiClient.get<BusinessRecord[]>('/admin/business/records', {params: {before}})).data,
 record: async (input: BusinessRecordInput) => (await apiClient.post<BusinessRecord>('/admin/business/records', input)).data,
 reverse: async (id: number, reason: string) => (await apiClient.post('/admin/business/records/' + id + '/reverse', {reason})).data,
 terms: async (before = 0) => (await apiClient.get<BusinessTerm[]>('/admin/business/terms', {params: {before}})).data,
 configureTerm: async (id: number, input: BusinessTermInput) => (await apiClient.put('/admin/business/terms/' + id, input)).data,
}
