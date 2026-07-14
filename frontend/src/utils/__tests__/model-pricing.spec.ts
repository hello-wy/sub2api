import { describe, expect, it } from 'vitest'
import {
  applyRateMultiplier,
  basePrice,
  discountRatio,
  equivalentPrice,
  formatDiscount,
  formatPrice,
  normalizeRechargeMultiplier,
  quotaPrice,
  resolveCardGroup,
  resolveCardGroupRate,
  resolveEffectiveRate,
  toDisplayTokenPrice,
} from '@/utils/model-pricing'

describe('model pricing utilities', () => {
  it('keeps empty prices empty when applying rate multipliers', () => {
    expect(applyRateMultiplier(null, 1.5)).toBeNull()
    expect(applyRateMultiplier(undefined, 1.5)).toBeNull()
  })

  it('uses a positive user-specific group rate before the group default rate', () => {
    expect(resolveEffectiveRate({ groupId: 12, groupRate: 1.2, userGroupRates: { 12: 0.8 } })).toBe(0.8)
    expect(resolveEffectiveRate({ groupId: 12, groupRate: 1.2, userGroupRates: {} })).toBe(1.2)
    expect(resolveEffectiveRate({ groupId: 12, groupRate: 1.2, userGroupRates: { 12: 0 } })).toBe(1.2)
    expect(resolveEffectiveRate({ groupId: 12, groupRate: -1, userGroupRates: { 12: Number.NaN } })).toBe(1)
  })

  it('uses the selected card group, otherwise the lowest valid effective rate', () => {
    const groups = [
      { id: 1, rate_multiplier: 1.2 },
      { id: 2, rate_multiplier: 0.8 },
      { id: 3, rate_multiplier: 0 },
    ]

    expect(resolveCardGroupRate({ groups, selectedGroupId: '1', userGroupRates: { 1: 0.7 } })).toBe(0.7)
    expect(resolveCardGroupRate({ groups, selectedGroupId: 99, userGroupRates: { 2: 0.6 } })).toBe(0.6)
    expect(resolveCardGroupRate({ groups: [], selectedGroupId: 1 })).toBe(1)
  })

  it('returns the active group and resolves equal rates deterministically', () => {
    const groups = [
      { id: 3, name: 'Zulu', rate_multiplier: 0.8 },
      { id: 2, name: 'Alpha', rate_multiplier: 0.8 },
      { id: 1, name: 'Alpha', rate_multiplier: 0.8 },
    ]

    expect(resolveCardGroup({ groups, selectedGroupId: '3' })).toBe(groups[0])
    expect(resolveCardGroup({ groups, selectedGroupId: 'missing' })).toBe(groups[2])
    expect(resolveCardGroup({ groups, userGroupRates: { 3: 0.5 } })).toBe(groups[0])
    expect(resolveCardGroup({ groups: [] })).toBeUndefined()
  })

  it('normalizes recharge multipliers and formats only actual discounts', () => {
    expect(normalizeRechargeMultiplier(2)).toBe(2)
    expect(normalizeRechargeMultiplier(0)).toBe(1)
    expect(normalizeRechargeMultiplier(Number.POSITIVE_INFINITY)).toBe(1)
    expect(discountRatio(10, 2)).toBe(5)
    expect(formatDiscount(2, 10)).toBe('2折')
    expect(formatDiscount(10, 2)).toBe('')
    expect(formatDiscount(2, 2)).toBe('')
  })

  it('provides base, quota, and recharge-equivalent token prices', () => {
    expect(basePrice(0.00000005)).toBe(0.05)
    expect(quotaPrice(0.00000005, 2)).toBe(0.1)
    expect(equivalentPrice(0.00000005, 2, 10)).toBe(0.01)
    expect(equivalentPrice(null, 2, 10)).toBeNull()
  })

  it('converts backend per-token prices to displayed per-million-token prices', () => {
    expect(toDisplayTokenPrice(0.00000005)).toBe(0.05)
  })

  it('formats compact prices without converting null to zero', () => {
    expect(formatPrice(null)).toBe('')
    expect(formatPrice(0.01234567)).toBe('0.012346')
  })
})
