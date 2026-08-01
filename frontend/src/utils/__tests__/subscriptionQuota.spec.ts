import { describe, expect, it } from 'vitest'
import {
  getExpirationDateRelation,
  getHighestSubscriptionUsagePercentage,
  getRemainingExpiryDuration,
  getRemainingDurationParts,
  usesSubscriptionLifetimeQuota,
} from '../subscriptionQuota'
import type { UserSubscription } from '@/types'

function subscriptionFixture(overrides: Partial<UserSubscription> = {}): UserSubscription {
  return {
    id: 1,
    user_id: 1,
    group_id: 1,
    status: 'active',
    starts_at: '2026-07-01T00:00:00Z',
    expires_at: '2026-08-01T00:00:00Z',
    daily_usage_usd: 2,
    weekly_usage_usd: 30,
    monthly_usage_usd: 25,
    total_usage_usd: 40,
    daily_window_start: null,
    weekly_window_start: null,
    monthly_window_start: null,
    created_at: '2026-07-01T00:00:00Z',
    updated_at: '2026-07-18T00:00:00Z',
    group: {
      id: 1,
      name: 'Pro',
      daily_limit_usd: 10,
      weekly_limit_usd: 50,
      monthly_limit_usd: 100,
    },
    ...overrides,
  } as UserSubscription
}

describe('getHighestSubscriptionUsagePercentage', () => {
  it('uses the highest configured quota window', () => {
    expect(getHighestSubscriptionUsagePercentage(subscriptionFixture())).toBe(60)
  })

  it('caps usage at one hundred percent', () => {
    expect(getHighestSubscriptionUsagePercentage(subscriptionFixture({ daily_usage_usd: 15 }))).toBe(100)
  })

  it('returns zero for subscriptions without quota limits', () => {
    expect(getHighestSubscriptionUsagePercentage(subscriptionFixture({
      group: {
        id: 1,
        name: 'Unlimited',
        daily_limit_usd: null,
        weekly_limit_usd: null,
        monthly_limit_usd: null,
      },
    } as Partial<UserSubscription>))).toBe(0)
  })

  it('uses the dedicated total quota for lifetime subscriptions', () => {
    expect(getHighestSubscriptionUsagePercentage(subscriptionFixture({
      total_usage_usd: 30,
      group: {
        id: 1,
        name: 'Single quota',
        subscription_quota_reset_mode: 'until_subscription_expires',
        subscription_total_limit_usd: 120,
      },
    } as Partial<UserSubscription>))).toBe(25)
  })
})

describe('usesSubscriptionLifetimeQuota', () => {
  it('identifies a subscription whose quota ends at expiration', () => {
    expect(usesSubscriptionLifetimeQuota(subscriptionFixture({
      group: {
        id: 1,
        name: 'Single quota',
        subscription_quota_reset_mode: 'until_subscription_expires',
      },
    } as Partial<UserSubscription>))).toBe(true)
  })

  it('keeps rolling quota as the compatibility default', () => {
    expect(usesSubscriptionLifetimeQuota(subscriptionFixture())).toBe(false)
  })
})

describe('getRemainingDurationParts', () => {
  const now = new Date('2026-07-26T08:00:00Z')

  it('keeps the concrete hours instead of rounding a partial day up', () => {
    expect(getRemainingDurationParts('2026-07-28T09:30:00Z', now)).toEqual({
      days: 2,
      hours: 1,
      minutes: 30,
    })
  })

  it('returns hours and minutes for durations shorter than one day', () => {
    expect(getRemainingDurationParts('2026-07-26T15:45:00Z', now)).toEqual({
      days: 0,
      hours: 7,
      minutes: 45,
    })
  })

  it('returns null after expiration', () => {
    expect(getRemainingDurationParts('2026-07-26T07:59:00Z', now)).toBeNull()
  })
})

describe('subscription expiry timing', () => {
  it('uses local calendar dates for today and tomorrow', () => {
    const now = new Date(2026, 2, 7, 23, 30)

    expect(getExpirationDateRelation(new Date(2026, 2, 7, 23, 45), now)).toBe('today')
    expect(getExpirationDateRelation(new Date(2026, 2, 8, 3, 30), now)).toBe('tomorrow')
  })

  it('treats the exact expiry instant and elapsed expiries as expired', () => {
    const now = new Date(2026, 6, 30, 9, 0)

    expect(getExpirationDateRelation(now, now)).toBe('expired')
    expect(getRemainingExpiryDuration(now, now)).toBeNull()
    expect(getExpirationDateRelation(new Date(2026, 6, 30, 8, 59), now)).toBe('expired')
    expect(getRemainingExpiryDuration(new Date(2026, 6, 30, 8, 59), now)).toBeNull()
  })

  it('rejects invalid target and current dates', () => {
    const invalid = new Date('invalid')
    const valid = new Date(2026, 6, 30, 9, 0)

    expect(getExpirationDateRelation(invalid, valid)).toBeNull()
    expect(getExpirationDateRelation(valid, invalid)).toBeNull()
    expect(getRemainingExpiryDuration(invalid, valid)).toBeNull()
    expect(getRemainingExpiryDuration(valid, invalid)).toBeNull()
  })

  it('returns rounded-up hours and minutes for an expiry under 24 hours away', () => {
    const now = new Date(2026, 6, 30, 9, 0)

    expect(getRemainingExpiryDuration(new Date(2026, 6, 31, 8, 30), now)).toEqual({
      unit: 'hoursMinutes',
      hours: 23,
      minutes: 30
    })
    expect(getRemainingExpiryDuration(new Date(now.getTime() + 1), now)).toEqual({
      unit: 'hoursMinutes',
      hours: 0,
      minutes: 1
    })
    expect(getRemainingExpiryDuration(new Date(now.getTime() + 23 * 60 * 60 * 1000 + 1), now)).toEqual({
      unit: 'hoursMinutes',
      hours: 23,
      minutes: 1
    })
  })

  it('preserves rounded-up day display from 24 hours onward', () => {
    const now = new Date(2026, 6, 30, 9, 0)

    expect(getRemainingExpiryDuration(new Date(now.getTime() + 24 * 60 * 60 * 1000), now)).toEqual({
      unit: 'days',
      days: 1
    })
    expect(getRemainingExpiryDuration(new Date(now.getTime() + 24 * 60 * 60 * 1000 + 1), now)).toEqual({
      unit: 'days',
      days: 2
  })
})
})
