import { apiClient } from '../client'

export interface BusinessEvent {
  id: number; source_key: string; event_type: string; transaction_id: number
  user_id: number; occurred_at: string; recorded_at: string; actor_id: number; reverses_id: number
  payload: Record<string, unknown>
}
export interface BusinessEntry {
  id: string; event_id: number; occurred_at: string; kind: string
  user_id: number; account_id: number; group_id: number; plan_id: number; model: string
  amount_cny: string | null; credits: string; quality: string; detail: Record<string, unknown>
}
export interface BusinessBreakdown {
  cash_in_cny?: string; cash_refund_cny?: string; cash_out_cny?: string; cash_net_cny?: string
  key: string; name: string; revenue_cny: string; cost_cny: string; profit_cny: string
  missing_count: number; estimated_count?: number; allocation: string
}
export interface BusinessOverview {
  start_at: string; end_at: string; enabled_at: string; updated_at: string; revision: number
  currency: string; timezone: string
  cash_in_cny: string; cash_refund_cny: string; cash_out_cny: string; cash_net_cny: string
  recognized_revenue_cny: string; usage_cost_cny: string; fixed_cost_cny: string; operating_cost_cny: string
  known_profit_cny: string; profit_cny: string | null; margin: string | null
  gift_granted_credits: string; gift_used_credits: string; gift_cost_cny: string; discount_cny: string
  wallet_deferred_cny: string; subscription_deferred_cny: string; prepaid_supplier_cny: string
  unknown_wallet_credits: string; prepaid_expense_cny: string; gift_outstanding_credits: string
  quality: { cash: string; revenue: string; cost: string; attribution: string; missing_count: number; estimated_count: number; processing_count?: number }
  entries: BusinessEntry[]; daily: BusinessBreakdown[]; groups: BusinessBreakdown[]
  models: BusinessBreakdown[]; accounts: BusinessBreakdown[]; plans: BusinessBreakdown[]
}
export interface BusinessPool { id: number; name: string; supplier: string; unit: string; mode: string; accounting_locked?: boolean }
export interface BusinessBinding { id: number; account_id: number; account_name: string; pool_id: number; effective_at: string }
export interface BusinessRule {
  service_tier: string; image_size: string; video_resolution: string; cache_write_1h_price: string
  id: number; pool_id: number; model: string; effective_at: string; basis: string
  unit_price: string; input_price: string; output_price: string; cache_read_price: string
  cache_write_price: string; cny_per_unit: string; quality: string; notes: string
}
export interface BusinessConfiguration {
  pools: BusinessPool[]; bindings: BusinessBinding[]; rules: BusinessRule[]; enabled_at: string; timezone: string
}
export interface BusinessRecordInput {
  idempotency_key: string; type: string; occurred_at: string; user_id?: number; payload: Record<string, unknown>
}
export interface BusinessEntity { id: number; name: string; kind: string }
export interface BusinessIssue {
	object_types?: string[]
  period_start_at?: string; period_end_at?: string
  key: string; kind: string; account_id: number; pool_id: number; user_id: number; name: string; model: string; period: string
  affected_count: number; source_count: number; known_amount_cny: string; first_at: string; last_at: string
}
export interface BusinessIssueSummary {
  starts_at?: string; ends_at?: string
  items: BusinessIssue[]; total: number; processing_count: number; calculation_error: boolean; updated_at: string; revision: number
}
export interface BusinessRepairInput {
  starts_at: string; ends_at: string; account_ids: number[]; pool_id: number; model: string
  through_event_id?: number; fingerprint?: string; idempotency_key?: string; notes?: string
}
export interface BusinessRepairPreview {
  through_event_id: number; fingerprint: string; repairable_count: number; missing_binding_count: number; missing_rule_count: number; protected_count: number
}
export interface BusinessRepairJob { id: number; count: number; status: string; notes: string; created_at: string }
export interface BusinessRecordDefaults { pool_id?: number; account_id?: number; starts_at?: string; ends_at?: string; bill_mode?: boolean }
export const businessAPI = {
  overview: async (params: { start_date?: string; end_date?: string }) => (await apiClient.get<BusinessOverview>('/admin/business/overview', { params })).data,
  records: async (before = 0) => (await apiClient.get<BusinessEvent[]>('/admin/business/records', { params: { before, limit: 100 } })).data,
  record: async (input: BusinessRecordInput) => (await apiClient.post<BusinessEvent>('/admin/business/records', input)).data,
  configuration: async () => (await apiClient.get<BusinessConfiguration>('/admin/business/configuration')).data,
  pool: async (input: Omit<BusinessPool, 'id'>) => (await apiClient.post<BusinessPool>('/admin/business/pools', input)).data,
  updatePool: async (id: number, input: Omit<BusinessPool, 'id'>) => (await apiClient.put<BusinessPool>(`/admin/business/pools/${id}`, input)).data,
  binding: async (input: Omit<BusinessBinding, 'id' | 'account_name'>) => (await apiClient.post<BusinessBinding>('/admin/business/bindings', input)).data,
  rule: async (input: Omit<BusinessRule, 'id'>) => (await apiClient.post<BusinessRule>('/admin/business/rules', input)).data,
  entities: async (kind: string, q = '') => (await apiClient.get<BusinessEntity[]>('/admin/business/entities', { params: { kind, q } })).data,
  pending: async (before = 0, userID = 0) => (await apiClient.get<BusinessEvent[]>('/admin/business/pending', { params: { before, user_id: userID } })).data,
  issues: async (params: { start_date: string; end_date: string; offset?: number }) => (await apiClient.get<BusinessIssueSummary>('/admin/business/issues', { params })).data,
  previewRepair: async (input: BusinessRepairInput) => (await apiClient.post<BusinessRepairPreview>('/admin/business/cost-repairs/preview', input)).data,
  repair: async (input: BusinessRepairInput) => (await apiClient.post<BusinessRepairJob>('/admin/business/cost-repairs', input)).data,
  repairJobs: async () => (await apiClient.get<BusinessRepairJob[]>('/admin/business/cost-repairs')).data,
  bindAccounts: async (input: { account_ids: number[]; pool_id: number; effective_at: string }) => (await apiClient.post<{ count: number }>('/admin/business/bindings/batch', input)).data,
  trace: async (id: number) => (await apiClient.get<BusinessEvent[]>(`/admin/business/events/${id}`)).data,
}
