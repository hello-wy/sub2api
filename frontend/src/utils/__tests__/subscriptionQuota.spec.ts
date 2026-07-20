import { describe, expect, it } from 'vitest'
import { getHighestSubscriptionUsagePercentage } from '../subscriptionQuota'
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
})
