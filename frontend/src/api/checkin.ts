import { apiClient } from './client'

export interface CheckinRewardRule {
  day_count: number
  extra_reward: number
}

export interface CheckinDayStatus {
  date: string
  checked_in: boolean
  is_today?: boolean
  streak_days?: number
  reward?: number
  label?: string
}

export interface CheckinHistoryItem {
  id: number
  checkin_date: string
  checked_at?: string
  streak_days: number
  base_reward: number
  extra_reward: number
  total_reward: number
  timezone?: string
  created_at?: string
}

export interface CheckinStatusResponse {
  can_checkin: boolean
  wechat_bound: boolean
  already_checked_in: boolean
  today_date: string
  timezone?: string
  current_streak: number
  month_checkins: number
  total_reward: number
  base_reward: number
  extra_reward: number
  today_reward: number
  next_reward_day_count?: number | null
  next_reward_extra?: number | null
  reward_rules: CheckinRewardRule[]
  recent_days: CheckinDayStatus[]
  recent_history: CheckinHistoryItem[]
}

export interface CheckinClaimResponse {
  message: string
  checked_in: boolean
  today_date: string
  base_reward: number
  extra_reward: number
  total_reward: number
  new_balance: number
  current_streak: number
  month_checkins: number
  timezone?: string
  record?: CheckinHistoryItem
}

export interface CheckinHistoryResponse {
  items: CheckinHistoryItem[]
  total: number
  page: number
  page_size: number
  pages: number
}

export async function getCheckinStatus(): Promise<CheckinStatusResponse> {
  const { data } = await apiClient.get<CheckinStatusResponse>('/user/checkin/status')
  return data
}

export async function checkin(): Promise<CheckinClaimResponse> {
  const { data } = await apiClient.post<CheckinClaimResponse>('/user/checkin')
  return data
}

export async function getCheckinHistory(page = 1, pageSize = 20): Promise<CheckinHistoryResponse> {
  const { data } = await apiClient.get<CheckinHistoryResponse>('/user/checkin/history', {
    params: { page, page_size: pageSize }
  })
  return data
}

export const checkinAPI = {
  getCheckinStatus,
  checkin,
  getCheckinHistory,
}

export default checkinAPI
