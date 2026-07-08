import { describe, expect, it } from 'vitest'
import {
  findLoyaltyPointsDefinitions,
  permanentLoyaltyRules,
  readLoyaltyPoints,
  resolveLoyaltyProgress,
  weeklyLoyaltyRules,
  type LoyaltyScope,
} from '@/utils/loyalty'
import type { UserAttributeDefinition, UserAttributeValue } from '@/types'

function definition(scope: LoyaltyScope, overrides: Partial<UserAttributeDefinition> = {}): UserAttributeDefinition {
  return {
    id: scope === 'weekly' ? 1 : 2,
    key: scope === 'weekly' ? 'loyalty_weekly_points' : 'loyalty_permanent_points',
    name: '会员积分',
    description: '',
    type: 'number',
    options: [],
    required: false,
    validation: {},
    placeholder: '',
    display_order: 0,
    enabled: true,
    created_at: '2026-07-04T00:00:00Z',
    updated_at: '2026-07-04T00:00:00Z',
    ...overrides,
  }
}

function value(overrides: Partial<UserAttributeValue> = {}): UserAttributeValue {
  return {
    id: 10,
    user_id: 42,
    attribute_id: 1,
    value: '880',
    created_at: '2026-07-04T00:00:00Z',
    updated_at: '2026-07-04T00:00:00Z',
    ...overrides,
  }
}

describe('loyalty points attribute', () => {
  const now = new Date('2026-07-04T12:00:00+08:00')

  it('finds the enabled number attributes for weekly and permanent points', () => {
    const found = findLoyaltyPointsDefinitions([
      definition('weekly', { id: 9, key: 'loyalty_note', type: 'text' }),
      definition('weekly'),
      definition('permanent'),
    ])

    expect(found.weekly?.id).toBe(1)
    expect(found.permanent?.id).toBe(2)
  })

  it('treats missing, disabled, wrong-type, and invalid values as zero', () => {
    expect(readLoyaltyPoints([], [], 'weekly', now)).toBe(0)
    expect(readLoyaltyPoints([definition('weekly', { enabled: false })], [value()], 'weekly', now)).toBe(0)
    expect(readLoyaltyPoints([definition('weekly', { type: 'text' })], [value()], 'weekly', now)).toBe(0)
    expect(readLoyaltyPoints([definition('weekly')], [value({ value: 'not-a-number' })], 'weekly', now)).toBe(0)
    expect(readLoyaltyPoints([definition('weekly')], [value({ value: '-5' })], 'weekly', now)).toBe(0)
  })

  it('reads valid numeric values from matched weekly and permanent attributes', () => {
    expect(readLoyaltyPoints([definition('weekly')], [
      value({ attribute_id: 99, value: '999' }),
      value({ value: '4000' }),
    ], 'weekly', now)).toBe(4000)

    expect(readLoyaltyPoints([definition('permanent')], [
      value({ attribute_id: 2, value: '5000' }),
    ], 'permanent', now)).toBe(5000)
  })

  it('resets weekly points when the value was last updated before the current week', () => {
    expect(readLoyaltyPoints([definition('weekly')], [
      value({ value: '800', updated_at: '2026-06-28T23:59:59+08:00' }),
    ], 'weekly', now)).toBe(0)

    expect(readLoyaltyPoints([definition('permanent')], [
      value({ attribute_id: 2, value: '800', updated_at: '2026-06-28T23:59:59+08:00' }),
    ], 'permanent', now)).toBe(800)
  })
})

describe('loyalty progress', () => {
  it('resolves weekly tier boundaries', () => {
    expect(resolveLoyaltyProgress(19, weeklyLoyaltyRules).current).toBeNull()
    expect(resolveLoyaltyProgress(20, weeklyLoyaltyRules).current?.level).toBe('L1')
    expect(resolveLoyaltyProgress(200, weeklyLoyaltyRules).current?.level).toBe('L2')
    expect(resolveLoyaltyProgress(800, weeklyLoyaltyRules).current?.level).toBe('L4')
  })

  it('resolves permanent tier boundaries', () => {
    expect(resolveLoyaltyProgress(799, permanentLoyaltyRules).current).toBeNull()
    expect(resolveLoyaltyProgress(800, permanentLoyaltyRules).current?.level).toBe('L2')
    expect(resolveLoyaltyProgress(4000, permanentLoyaltyRules).current?.level).toBe('L3')
    expect(resolveLoyaltyProgress(8000, permanentLoyaltyRules).current?.level).toBe('L4')
  })

  it('reports remaining points and max-level progress', () => {
    const weekly = resolveLoyaltyProgress(100, weeklyLoyaltyRules)
    expect(weekly.next?.level).toBe('L2')
    expect(weekly.remainingPoints).toBe(100)
    expect(Math.round(weekly.progressPercent)).toBe(44)

    const maxed = resolveLoyaltyProgress(9000, permanentLoyaltyRules)
    expect(maxed.next).toBeNull()
    expect(maxed.progressPercent).toBe(100)
    expect(maxed.remainingPoints).toBe(0)
  })
})
