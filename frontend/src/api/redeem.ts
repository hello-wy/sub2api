/**
 * Redeem code API endpoints
 * Handles redeem code redemption for users
 */

import { apiClient } from './client'
import type { RedeemCodeRequest } from '@/types'

export interface RedeemHistoryItem {
  id: number
  code: string
  type: string
  value: number
  status: string
  used_at: string
  created_at: string
  // Notes from admin for admin_balance/admin_concurrency types
  notes?: string
  // Subscription-specific fields
  group_id?: number
  validity_days?: number
  group?: {
    id: number
    name: string
  }
}

export interface SubscriptionOverwriteConfirmation {
  subscriptionId: number
  termVersion: number
  expiresAt: string
}

/**
 * Redeem a code
 * @param code - Redeem code string
 * @returns Redemption result with updated balance or concurrency
 */
export async function redeem(code: string, confirmation?: SubscriptionOverwriteConfirmation): Promise<{
  message: string
  type: string
  value: number
  new_balance?: number
  new_concurrency?: number
}> {
  const payload: RedeemCodeRequest = {
    code,
    confirm_subscription_overwrite: confirmation ? true : undefined,
    expected_subscription_id: confirmation?.subscriptionId,
    expected_subscription_term_version: confirmation?.termVersion,
    expected_subscription_expires_at: confirmation?.expiresAt,
  }

  const { data } = await apiClient.post<{
    message: string
    type: string
    value: number
    new_balance?: number
    new_concurrency?: number
  }>('/redeem', payload)

  return data
}

/**
 * Get user's redemption history
 * @returns List of redeemed codes
 */
export async function getHistory(limit = 25): Promise<RedeemHistoryItem[]> {
  const { data } = await apiClient.get<RedeemHistoryItem[]>('/redeem/history', {
    params: { limit },
  })
  return data
}

export const redeemAPI = {
  redeem,
  getHistory
}

export default redeemAPI
