/**
 * Admin Dashboard API endpoints
 * Provides system-wide statistics and metrics
 */

import { apiClient } from '../client'
import type {
  DashboardStats,
  TrendDataPoint,
  ModelStat,
  GroupStat,
  ApiKeyUsageTrendPoint,
  UserUsageTrendPoint,
  UserSpendingRankingResponse,
  UserBreakdownItem,
  UsageRequestType
} from '@/types'

export interface BusinessCurrencyAmount {
  currency: string
  amount: number
}

export interface BusinessLiabilitySummary {
  balance_credits_usd: number
  balance_face_value_cny: number
  balance_estimated_cost_cny: number
  active_subscriptions: number
  subscription_commitment_usd: number
  subscription_estimated_cost_cny: number
  unlimited_subscriptions: number
}

export interface BusinessAnalyticsSettings {
  balance_credits_per_cny: number
}

export interface BusinessProfitSummary {
  start_at: string
  end_at: string
  usage_revenue_cny: number
  api_key_usage_cost_cny: number
  welfare_cost_cny: number
  account_cost_cny: number
  total_cost_cny: number
  operating_profit_cny: number
  operating_margin: number
  unpriced_api_key_usage_cost_usd: number
  profit_complete: boolean
}

export interface BusinessDailyAnalytics {
  date: string
  usage_revenue_cny: number
  api_key_usage_cost_cny: number
  unpriced_api_key_usage_cost_usd: number
  profit_complete: boolean
  welfare_granted_usd: number
  welfare_cost_cny: number
  account_cost_cny: number
  operating_profit_cny: number
}

export interface BusinessGroupAnalytics {
  group_id: number
  group_name: string
  effective_rate_multiplier: number
  usage_credits_usd: number
  usage_revenue_cny: number
  api_key_usage_cost_usd: number
  api_key_usage_cost_cny: number
  unpriced_api_key_usage_cost_usd: number
  profit_complete: boolean
  gross_profit_cny: number
  gross_margin: number
  allocated_welfare_cost_cny: number
  allocated_account_cost_cny: number
  operating_profit_cny: number
  forecast_p50_daily_cost_usd: number
  forecast_p95_daily_cost_usd: number
  observed_capacity_per_account: number
  schedulable_accounts: number
  concurrency_max: number
  required_accounts: number
  additional_accounts: number
}

export interface BusinessAnalyticsOverview {
  start_at: string
  end_at: string
  settings: BusinessAnalyticsSettings
  usage_credits_usd: number
  usage_revenue_cny: number
  api_key_usage_cost_usd: number
  api_key_usage_cost_cny: number
  unpriced_api_key_usage_cost_usd: number
  profit_complete: boolean
  gross_profit_cny: number
  gross_margin: number
  welfare_granted_usd: number
  welfare_cost_cny: number
  account_cost_cny: number
  operating_profit_cny: number
  operating_margin: number
  cumulative: BusinessProfitSummary
  cost_ledger_configured: boolean
  cash_receipts: BusinessCurrencyAmount[]
  liabilities: BusinessLiabilitySummary
  daily: BusinessDailyAnalytics[]
  groups: BusinessGroupAnalytics[]
  snapshot_captured_at?: string
}

export interface BusinessAccountCost {
  id: number
  account_id?: number
  group_id?: number
  cost_type: string
  amount: number
  currency: string
  fx_rate: number
  starts_at: string
  ends_at: string
  notes: string
  created_at: string
}

export interface CreateBusinessAccountCostInput {
  account_id?: number
  group_id?: number
  cost_type: string
  amount: number
  currency: string
  fx_rate: number
  starts_at: string
  ends_at: string
  notes?: string
}

export interface BusinessAPIKeyAccount {
  id: number
  name: string
  platform: string
}

export interface BusinessAPIKeyCostRate {
  id: number
  account_id: number
  credits_per_cny: number
  notes: string
  created_at: string
}

export interface BusinessAPIKeyCostRateConfig {
  accounts: BusinessAPIKeyAccount[]
  rates: BusinessAPIKeyCostRate[]
}

export interface CreateBusinessAPIKeyCostRateInput {
  account_id: number
  credits_per_cny: number
  notes?: string
}

/**
 * Get dashboard statistics
 * @returns Dashboard statistics including users, keys, accounts, and token usage
 */
export async function getStats(): Promise<DashboardStats> {
  const { data } = await apiClient.get<DashboardStats>('/admin/dashboard/stats')
  return data
}

export async function getBusinessAnalytics(params?: { start_date?: string; end_date?: string }): Promise<BusinessAnalyticsOverview> {
  const { data } = await apiClient.get<BusinessAnalyticsOverview>('/admin/dashboard/business-analytics', { params })
  return data
}

export async function getBusinessAPIKeyCostRates(): Promise<BusinessAPIKeyCostRateConfig> {
  const { data } = await apiClient.get<BusinessAPIKeyCostRateConfig>('/admin/dashboard/business-api-key-cost-rates')
  return data
}

export async function createBusinessAPIKeyCostRate(input: CreateBusinessAPIKeyCostRateInput): Promise<BusinessAPIKeyCostRate> {
  const { data } = await apiClient.post<BusinessAPIKeyCostRate>('/admin/dashboard/business-api-key-cost-rates', input)
  return data
}

export async function deleteBusinessAPIKeyCostRate(id: number): Promise<void> {
  await apiClient.delete(`/admin/dashboard/business-api-key-cost-rates/${id}`)
}

export async function listBusinessCosts(): Promise<BusinessAccountCost[]> {
  const { data } = await apiClient.get<BusinessAccountCost[]>('/admin/dashboard/business-costs')
  return data
}

export async function createBusinessCost(input: CreateBusinessAccountCostInput): Promise<BusinessAccountCost> {
  const { data } = await apiClient.post<BusinessAccountCost>('/admin/dashboard/business-costs', input)
  return data
}

export async function deleteBusinessCost(id: number): Promise<void> {
  await apiClient.delete(`/admin/dashboard/business-costs/${id}`)
}

export async function captureBusinessCapacitySnapshot(): Promise<{ captured_at: string }> {
  const { data } = await apiClient.post<{ captured_at: string }>('/admin/dashboard/business-capacity-snapshot')
  return data
}

/**
 * Get real-time metrics
 * @returns Real-time system metrics
 */
export async function getRealtimeMetrics(): Promise<{
  active_requests: number
  requests_per_minute: number
  average_response_time: number
  error_rate: number
}> {
  const { data } = await apiClient.get<{
    active_requests: number
    requests_per_minute: number
    average_response_time: number
    error_rate: number
  }>('/admin/dashboard/realtime')
  return data
}

export interface TrendParams {
  start_date?: string
  end_date?: string
  granularity?: 'day' | 'hour'
  user_id?: number
  api_key_id?: number
  model?: string
  account_id?: number
  group_id?: number
  request_type?: UsageRequestType
  stream?: boolean
  billing_type?: number | null
}

export interface TrendResponse {
  trend: TrendDataPoint[]
  start_date: string
  end_date: string
  granularity: string
}

/**
 * Get usage trend data
 * @param params - Query parameters for filtering
 * @returns Usage trend data
 */
export async function getUsageTrend(params?: TrendParams): Promise<TrendResponse> {
  const { data } = await apiClient.get<TrendResponse>('/admin/dashboard/trend', { params })
  return data
}

export interface ModelStatsParams {
  start_date?: string
  end_date?: string
  user_id?: number
  api_key_id?: number
  model?: string
  model_source?: 'requested' | 'upstream' | 'mapping'
  account_id?: number
  group_id?: number
  request_type?: UsageRequestType
  stream?: boolean
  billing_type?: number | null
}

export interface ModelStatsResponse {
  models: ModelStat[]
  start_date: string
  end_date: string
}

/**
 * Get model usage statistics
 * @param params - Query parameters for filtering
 * @returns Model usage statistics
 */
export async function getModelStats(params?: ModelStatsParams): Promise<ModelStatsResponse> {
  const { data } = await apiClient.get<ModelStatsResponse>('/admin/dashboard/models', { params })
  return data
}

export interface GroupStatsParams {
  start_date?: string
  end_date?: string
  user_id?: number
  api_key_id?: number
  account_id?: number
  group_id?: number
  request_type?: UsageRequestType
  stream?: boolean
  billing_type?: number | null
}

export interface GroupStatsResponse {
  groups: GroupStat[]
  start_date: string
  end_date: string
}

export interface DashboardSnapshotV2Params extends TrendParams {
  include_stats?: boolean
  include_trend?: boolean
  include_model_stats?: boolean
  include_group_stats?: boolean
  include_users_trend?: boolean
  users_trend_limit?: number
}

export interface DashboardSnapshotV2Stats extends DashboardStats {
  uptime: number
}

export interface DashboardSnapshotV2Response {
  generated_at: string
  start_date: string
  end_date: string
  granularity: string
  stats?: DashboardSnapshotV2Stats
  trend?: TrendDataPoint[]
  models?: ModelStat[]
  groups?: GroupStat[]
  users_trend?: UserUsageTrendPoint[]
}

/**
 * Get group usage statistics
 * @param params - Query parameters for filtering
 * @returns Group usage statistics
 */
export async function getGroupStats(params?: GroupStatsParams): Promise<GroupStatsResponse> {
  const { data } = await apiClient.get<GroupStatsResponse>('/admin/dashboard/groups', { params })
  return data
}

export interface UserBreakdownParams {
  start_date?: string
  end_date?: string
  group_id?: number
  model?: string
  model_source?: 'requested' | 'upstream' | 'mapping'
  endpoint?: string
  endpoint_type?: 'inbound' | 'upstream' | 'path'
  limit?: number
  // Sort column for the ranking (allowlisted server-side; falls back to actual_cost)
  sort_by?: 'total_tokens' | 'input_tokens' | 'output_tokens' | 'cache_tokens' | 'requests' | 'cost' | 'actual_cost'
  // Additional filter conditions
  user_id?: number
  api_key_id?: number
  account_id?: number
  request_type?: UsageRequestType
  stream?: boolean
  billing_type?: number | null
}

export interface UserBreakdownResponse {
  users: UserBreakdownItem[]
  start_date: string
  end_date: string
}

export async function getUserBreakdown(params: UserBreakdownParams): Promise<UserBreakdownResponse> {
  const { data } = await apiClient.get<UserBreakdownResponse>('/admin/dashboard/user-breakdown', {
    params
  })
  return data
}

/**
 * Get dashboard snapshot v2 (aggregated response for heavy admin pages).
 */
export async function getSnapshotV2(params?: DashboardSnapshotV2Params): Promise<DashboardSnapshotV2Response> {
  const { data } = await apiClient.get<DashboardSnapshotV2Response>('/admin/dashboard/snapshot-v2', {
    params
  })
  return data
}

export interface ApiKeyTrendParams extends TrendParams {
  limit?: number
}

export interface ApiKeyTrendResponse {
  trend: ApiKeyUsageTrendPoint[]
  start_date: string
  end_date: string
  granularity: string
}

/**
 * Get API key usage trend data
 * @param params - Query parameters for filtering
 * @returns API key usage trend data
 */
export async function getApiKeyUsageTrend(
  params?: ApiKeyTrendParams
): Promise<ApiKeyTrendResponse> {
  const { data } = await apiClient.get<ApiKeyTrendResponse>('/admin/dashboard/api-keys-trend', {
    params
  })
  return data
}

export interface UserTrendParams extends TrendParams {
  limit?: number
}

export interface UserTrendResponse {
  trend: UserUsageTrendPoint[]
  start_date: string
  end_date: string
  granularity: string
}

export interface UserSpendingRankingParams
  extends Pick<TrendParams, 'start_date' | 'end_date'> {
  limit?: number
}

/**
 * Get user usage trend data
 * @param params - Query parameters for filtering
 * @returns User usage trend data
 */
export async function getUserUsageTrend(params?: UserTrendParams): Promise<UserTrendResponse> {
  const { data } = await apiClient.get<UserTrendResponse>('/admin/dashboard/users-trend', {
    params
  })
  return data
}

/**
 * Get user spending ranking data
 * @param params - Query parameters for filtering
 * @returns User spending ranking data
 */
export async function getUserSpendingRanking(
  params?: UserSpendingRankingParams
): Promise<UserSpendingRankingResponse> {
  const { data } = await apiClient.get<UserSpendingRankingResponse>('/user/dashboard/users-ranking', {
    params
  })
  return data
}

export interface PlatformUsage {
  platform: string
  today_actual_cost: number
  total_actual_cost: number
}

export interface BatchUserUsageStats {
  user_id: number
  today_actual_cost: number
  total_actual_cost: number
  by_platform?: PlatformUsage[]
}

export interface BatchUsersUsageResponse {
  stats: Record<string, BatchUserUsageStats>
}

/**
 * Get batch usage stats for multiple users
 * @param userIds - Array of user IDs
 * @returns Usage stats map keyed by user ID
 */
export async function getBatchUsersUsage(userIds: number[]): Promise<BatchUsersUsageResponse> {
  const { data } = await apiClient.post<BatchUsersUsageResponse>('/admin/dashboard/users-usage', {
    user_ids: userIds
  })
  return data
}

export interface BatchApiKeyUsageStats {
  api_key_id: number
  today_actual_cost: number
  total_actual_cost: number
}

export interface BatchApiKeysUsageResponse {
  stats: Record<string, BatchApiKeyUsageStats>
}

/**
 * Get batch usage stats for multiple API keys
 * @param apiKeyIds - Array of API key IDs
 * @returns Usage stats map keyed by API key ID
 */
export async function getBatchApiKeysUsage(
  apiKeyIds: number[]
): Promise<BatchApiKeysUsageResponse> {
  const { data } = await apiClient.post<BatchApiKeysUsageResponse>(
    '/admin/dashboard/api-keys-usage',
    {
      api_key_ids: apiKeyIds
    }
  )
  return data
}

export const dashboardAPI = {
  getStats,
  getBusinessAnalytics,
  getBusinessAPIKeyCostRates,
  createBusinessAPIKeyCostRate,
  deleteBusinessAPIKeyCostRate,
  listBusinessCosts,
  createBusinessCost,
  deleteBusinessCost,
  captureBusinessCapacitySnapshot,
  getRealtimeMetrics,
  getUsageTrend,
  getModelStats,
  getGroupStats,
  getSnapshotV2,
  getApiKeyUsageTrend,
  getUserUsageTrend,
  getUserSpendingRanking,
  getBatchUsersUsage,
  getBatchApiKeysUsage
}

export default dashboardAPI
