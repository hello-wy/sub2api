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

type RawCheckinHistoryItem = CheckinHistoryItem & {
  bonus_reward?: number
}

export interface CheckinStatusResponse {
  can_checkin: boolean
  qq_bound: boolean
  wechat_bound?: boolean
  already_checked_in: boolean
  today_date: string
  timezone?: string
  current_streak: number
  month_checkins: number
  total_reward: number
  base_reward: number
  base_reward_min?: number
  base_reward_max?: number
  extra_reward: number
  today_reward: number
  today_reward_min?: number
  today_reward_max?: number
  next_reward_day_count?: number | null
  next_reward_extra?: number | null
  reward_cycle_number?: number
  reward_cycle_days?: number
  reward_cycle_day?: number
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

interface RawCheckinRule {
  threshold?: number
  bonus?: number
  day_count?: number
  extra_reward?: number
}

interface RawCheckinSummary {
  timezone?: string
  today?: string
  qq_bound?: boolean
  wechat_bound?: boolean
  can_check_in?: boolean
  checked_in_today?: boolean
  streak_days?: number
  this_month_count?: number
  total_reward?: number
  base_reward?: number
  base_reward_min?: number
  base_reward_max?: number
  bonus_reward?: number
  today_reward?: number
  today_reward_min?: number
  today_reward_max?: number
  balance?: number
  recent_records?: RawCheckinHistoryItem[]
  reward_rules?: RawCheckinRule[]
  reward_cycle_number?: number
  reward_cycle_days?: number
  reward_cycle_day?: number
}

interface RawCheckinStatusResponse extends Partial<CheckinStatusResponse> {
  summary?: RawCheckinSummary
  balance?: number
}

interface RawCheckinClaimResponse extends Partial<CheckinClaimResponse> {
  summary?: RawCheckinSummary
  balance?: number
  record?: RawCheckinHistoryItem
}

const defaultRewardRules: CheckinRewardRule[] = [
  { day_count: 3, extra_reward: 3 },
  { day_count: 7, extra_reward: 6 },
  { day_count: 14, extra_reward: 12 },
  { day_count: 30, extra_reward: 24 },
]

const defaultBaseRewardMin = 1
const defaultBaseRewardMax = 3

function readNumber(value: unknown): number | undefined {
  if (value === null || value === undefined || value === '') return undefined
  const n = Number(value)
  return Number.isFinite(n) ? n : undefined
}

function normalizeDateKey(value: unknown): string {
  if (value === null || value === undefined || value === '') return ''
  if (value instanceof Date && Number.isFinite(value.getTime())) {
    const year = value.getFullYear()
    const month = String(value.getMonth() + 1).padStart(2, '0')
    const day = String(value.getDate()).padStart(2, '0')
    return `${year}-${month}-${day}`
  }

  const text = String(value)
  const match = text.match(/^(\d{4})-(\d{2})-(\d{2})/)
  if (match) return `${match[1]}-${match[2]}-${match[3]}`

  const date = new Date(text)
  if (!Number.isFinite(date.getTime())) return text

  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

function normalizeRewardRules(rules?: RawCheckinRule[]): CheckinRewardRule[] {
  if (!rules?.length) return defaultRewardRules
  return rules.map((rule) => ({
    day_count: Number(rule.day_count ?? rule.threshold ?? 0),
    extra_reward: Number(rule.extra_reward ?? rule.bonus ?? 0),
  })).filter((rule) => rule.day_count > 0)
}

function normalizeRecord(record: RawCheckinHistoryItem): CheckinHistoryItem {
  return {
    ...record,
    checkin_date: normalizeDateKey(record.checkin_date),
    extra_reward: Number(record.extra_reward ?? record.bonus_reward ?? 0),
  }
}

function normalizeStatus(raw: RawCheckinStatusResponse): CheckinStatusResponse {
  const summary = raw.summary
  const recentHistory = (raw.recent_history ?? summary?.recent_records ?? []).map(normalizeRecord)
  const baseRewardMin = readNumber(raw.base_reward_min ?? summary?.base_reward_min) ?? defaultBaseRewardMin
  const baseRewardMax = readNumber(raw.base_reward_max ?? summary?.base_reward_max) ?? defaultBaseRewardMax
  const extraReward = readNumber(raw.extra_reward ?? summary?.bonus_reward) ?? 0
  const todayRewardMin = readNumber(raw.today_reward_min ?? summary?.today_reward_min) ?? baseRewardMin + extraReward
  const todayRewardMax = readNumber(raw.today_reward_max ?? summary?.today_reward_max) ?? baseRewardMax + extraReward
  return {
    can_checkin: Boolean(raw.can_checkin ?? summary?.can_check_in),
    qq_bound: Boolean(raw.qq_bound ?? summary?.qq_bound),
    wechat_bound: Boolean(raw.wechat_bound ?? summary?.wechat_bound),
    already_checked_in: Boolean(raw.already_checked_in ?? summary?.checked_in_today),
    today_date: normalizeDateKey(raw.today_date ?? summary?.today),
    timezone: raw.timezone ?? summary?.timezone,
    current_streak: Number(raw.current_streak ?? summary?.streak_days ?? 0),
    month_checkins: Number(raw.month_checkins ?? summary?.this_month_count ?? 0),
    total_reward: Number(raw.total_reward ?? summary?.total_reward ?? 0),
    base_reward: Number(raw.base_reward ?? summary?.base_reward ?? 0),
    base_reward_min: baseRewardMin,
    base_reward_max: baseRewardMax,
    extra_reward: extraReward,
    today_reward: Number(raw.today_reward ?? summary?.today_reward ?? 0),
    today_reward_min: todayRewardMin,
    today_reward_max: todayRewardMax,
    next_reward_day_count: raw.next_reward_day_count ?? null,
    next_reward_extra: raw.next_reward_extra ?? null,
    reward_cycle_number: Number(raw.reward_cycle_number ?? summary?.reward_cycle_number ?? 1),
    reward_cycle_days: Number(raw.reward_cycle_days ?? summary?.reward_cycle_days ?? 30),
    reward_cycle_day: Number(raw.reward_cycle_day ?? summary?.reward_cycle_day ?? 1),
    reward_rules: normalizeRewardRules(raw.reward_rules ?? summary?.reward_rules),
    recent_days: (raw.recent_days ?? []).map((day) => ({
      ...day,
      date: normalizeDateKey(day.date),
    })),
    recent_history: recentHistory,
  }
}

function normalizeClaim(raw: RawCheckinClaimResponse): CheckinClaimResponse {
  const summary = raw.summary
  return {
    message: raw.message ?? '签到成功',
    checked_in: Boolean(raw.checked_in ?? summary?.checked_in_today),
    today_date: normalizeDateKey(raw.today_date ?? summary?.today),
    base_reward: Number(raw.base_reward ?? summary?.base_reward ?? 0),
    extra_reward: Number(raw.extra_reward ?? summary?.bonus_reward ?? 0),
    total_reward: Number(raw.total_reward ?? raw.record?.total_reward ?? summary?.today_reward ?? 0),
    new_balance: Number(raw.new_balance ?? raw.balance ?? summary?.balance ?? 0),
    current_streak: Number(raw.current_streak ?? summary?.streak_days ?? 0),
    month_checkins: Number(raw.month_checkins ?? summary?.this_month_count ?? 0),
    timezone: raw.timezone ?? summary?.timezone,
    record: raw.record ? normalizeRecord(raw.record) : undefined,
  }
}

export async function getCheckinStatus(): Promise<CheckinStatusResponse> {
  const { data } = await apiClient.get<RawCheckinStatusResponse>('/user/checkin/status')
  return normalizeStatus(data)
}

export async function checkin(): Promise<CheckinClaimResponse> {
  const { data } = await apiClient.post<RawCheckinClaimResponse>('/user/checkin')
  return normalizeClaim(data)
}

export async function getCheckinHistory(page = 1, pageSize = 20): Promise<CheckinHistoryResponse> {
  const { data } = await apiClient.get<CheckinHistoryResponse>('/user/checkin/history', {
    params: { page, page_size: pageSize }
  })
  return {
    ...data,
    items: data.items.map(normalizeRecord),
  }
}

export const checkinAPI = {
  getCheckinStatus,
  checkin,
  getCheckinHistory,
}

export default checkinAPI
