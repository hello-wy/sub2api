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
  created_at: string
  updated_at: string
}

export async function list(
  page: number = 1,
  pageSize: number = 20,
  email?: string,
  options?: {
    signal?: AbortSignal
  }
): Promise<BasePaginationResponse<WelfareRecord>> {
  const { data } = await apiClient.get<BasePaginationResponse<WelfareRecord>>('/admin/welfare-records', {
    params: { page, page_size: pageSize, email },
    signal: options?.signal
  })
  return data
}

export async function revoke(id: number): Promise<{ message: string }> {
  const { data } = await apiClient.post<{ message: string }>(`/admin/welfare-records/${id}/revoke`)
  return data
}

const welfareAPI = {
  list,
  revoke
}

export default welfareAPI
