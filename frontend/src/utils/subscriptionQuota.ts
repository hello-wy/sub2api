import type { UserSubscription } from '@/types'

const ONE_DAY_MS = 24 * 60 * 60 * 1000

export interface RemainingDurationParts {
  days: number
  hours: number
  minutes: number
}

type SubscriptionUsageFields = Pick<
  UserSubscription,
  'daily_usage_usd' | 'weekly_usage_usd' | 'monthly_usage_usd' | 'total_usage_usd' | 'group' | 'expires_at'
>

type SubscriptionQuotaWindowFields = Pick<UserSubscription, 'expires_at' | 'group'>

export function usesSubscriptionLifetimeQuota(
  subscription: SubscriptionQuotaWindowFields | null | undefined
): boolean {
  return Boolean(
    subscription?.expires_at &&
    subscription.group?.subscription_quota_reset_mode === 'until_subscription_expires'
  )
}

export function getHighestSubscriptionUsagePercentage(
  subscription: SubscriptionUsageFields | null | undefined
): number {
  if (!subscription) return 0

  if (usesSubscriptionLifetimeQuota(subscription)) {
    const limit = subscription.group?.subscription_total_limit_usd
    if (!limit || limit <= 0) return 0
    const used = Number.isFinite(subscription.total_usage_usd)
      ? Math.max(0, subscription.total_usage_usd)
      : 0
    return Math.round(Math.min(100, (used / limit) * 100))
  }

	const usagePairs: Array<[number, number | null | undefined]> = [
    [subscription.daily_usage_usd, subscription.group?.daily_limit_usd],
    [subscription.weekly_usage_usd, subscription.group?.weekly_limit_usd],
    [subscription.monthly_usage_usd, subscription.group?.monthly_limit_usd],
  ]
  const percentages = usagePairs.flatMap(([used, limit]) => {
    if (!limit || limit <= 0) return []
    const safeUsed = Number.isFinite(used) ? Math.max(0, used) : 0
    return [Math.min(100, (safeUsed / limit) * 100)]
  })

  return percentages.length ? Math.round(Math.max(...percentages)) : 0
}

export function isOneTimeDailyQuota(
  subscription: Pick<UserSubscription, 'starts_at' | 'expires_at'>
): boolean {
  if (!subscription.starts_at || !subscription.expires_at) return false

  const startsAt = new Date(subscription.starts_at).getTime()
  const expiresAt = new Date(subscription.expires_at).getTime()

  if (!Number.isFinite(startsAt) || !Number.isFinite(expiresAt)) return false

  return expiresAt <= startsAt + ONE_DAY_MS
}

export function getRemainingDurationParts(
  targetAt: Date | string,
  now: Date = new Date()
): RemainingDurationParts | null {
  const targetTime = targetAt instanceof Date ? targetAt.getTime() : new Date(targetAt).getTime()
  const nowTime = now.getTime()

  if (!Number.isFinite(targetTime) || !Number.isFinite(nowTime)) return null

  const diffMs = targetTime - nowTime
  if (diffMs <= 0) return null

  const totalMinutes = Math.floor(diffMs / (1000 * 60))
  const days = Math.floor(totalMinutes / (24 * 60))
  const hours = Math.floor((totalMinutes % (24 * 60)) / 60)
  const minutes = totalMinutes % 60

  return { days, hours, minutes }
}
