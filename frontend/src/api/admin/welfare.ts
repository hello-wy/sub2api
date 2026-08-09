/**
 * Admin Welfare Records API endpoints
 */

import { apiClient } from '../client'
import type { BasePaginationResponse } from '@/types'

export interface WelfareRecord {
  id: number
  user_id: number
  user_email: string
  amount: number
  remarks: string
  status: 'success' | 'revoked' | string
  type: 'leaderboard' | 'checkin' | string
  created_at: string
  updated_at: string
}

export interface WelfareSummary {
  total_count: number
  total_amount: number
  checkin_amount: number
  leaderboard_amount: number
  lottery_amount: number
}

export type WelfareBenefitType = 'leaderboard' | 'checkin' | 'lottery'
export type WelfareRecordStatus = 'success' | 'revoked'

export interface WelfareListResponse extends BasePaginationResponse<WelfareRecord> {
  summary?: WelfareSummary
}

export async function list(
  page: number = 1,
  pageSize: number = 20,
  email?: string,
  options?: {
    startDate?: string
    endDate?: string
    type?: WelfareBenefitType
    status?: WelfareRecordStatus
    signal?: AbortSignal
  }
): Promise<WelfareListResponse> {
  const { data } = await apiClient.get<WelfareListResponse>('/admin/welfare-records', {
    params: {
      page,
      page_size: pageSize,
      email,
      start_date: options?.startDate,
      end_date: options?.endDate,
      type: options?.type,
      status: options?.status
    },
    signal: options?.signal
  })
  return data
}

export async function revoke(id: number, type?: WelfareBenefitType | string): Promise<{ message: string }> {
  const { data } = await apiClient.post<{ message: string }>(`/admin/welfare-records/${id}/revoke`, null, {
    params: { type }
  })
  return data
}

const welfareAPI = {
  list,
  revoke
}

export default welfareAPI
