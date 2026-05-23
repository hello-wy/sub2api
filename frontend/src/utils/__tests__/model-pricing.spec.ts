import { describe, expect, it } from 'vitest'
import {
  applyRateMultiplier,
  formatPrice,
  resolveEffectiveRate,
  toDisplayTokenPrice,
} from '@/utils/model-pricing'

describe('model pricing utilities', () => {
  it('keeps empty prices empty when applying rate multipliers', () => {
    expect(applyRateMultiplier(null, 1.5)).toBeNull()
    expect(applyRateMultiplier(undefined, 1.5)).toBeNull()
  })

  it('uses user-specific group rate before the group default rate', () => {
    expect(resolveEffectiveRate({ groupId: 12, groupRate: 1.2, userGroupRates: { 12: 0.8 } })).toBe(0.8)
    expect(resolveEffectiveRate({ groupId: 12, groupRate: 1.2, userGroupRates: {} })).toBe(1.2)
  })

  it('converts backend per-token prices to displayed per-million-token prices', () => {
    expect(toDisplayTokenPrice(0.00000005)).toBe(0.05)
  })

  it('formats compact prices without converting null to zero', () => {
    expect(formatPrice(null)).toBe('')
    expect(formatPrice(0.01234567)).toBe('0.012346')
  })
})
