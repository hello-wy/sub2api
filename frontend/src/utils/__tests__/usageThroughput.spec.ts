import { describe, expect, it } from 'vitest'
import { formatUsageTokensPerSecond, getUsageTokensPerSecond } from '../usageThroughput'

describe('getUsageTokensPerSecond', () => {
  it('includes initial waiting and ignores a stale server rate', () => {
    const row = { tps: 40, output_tokens: 200, duration_ms: 25000, first_token_ms: 20000 }
    expect(getUsageTokensPerSecond(row)).toBe(8)
    expect(getUsageTokensPerSecond({ ...row, duration_ms: 65000, first_token_ms: 60000 })).toBeCloseTo(200 / 65)
  })

  it.each([
    { output_tokens: 0 }, { output_tokens: -1 }, { output_tokens: NaN }, { output_tokens: Infinity },
    { duration_ms: null }, { duration_ms: NaN }, { duration_ms: Infinity },
    { duration_ms: 0 }, { duration_ms: -1 },
  ])('omits unusable timing or usage: %o', (overrides) => {
    expect(getUsageTokensPerSecond({ output_tokens: 50, duration_ms: 2500, ...overrides })).toBeNull()
  })

  it.each([null, -1, Infinity, 2600, 2500, 2499, 2490, 2401, 2400, 0])(
    'does not depend on first-token timing: %s', (first_token_ms) => {
      const row = { output_tokens: 50, duration_ms: 2500, first_token_ms }
      expect(getUsageTokensPerSecond(row)).toBe(20)
    }
  )

  it('works without first-token timing and with short positive durations', () => {
    expect(getUsageTokensPerSecond({ output_tokens: 50, duration_ms: 2500 })).toBe(20)
    expect(getUsageTokensPerSecond({ output_tokens: 5, duration_ms: 1 })).toBe(5000)
  })
})

describe('formatUsageTokensPerSecond', () => {
  it.each([[9.99, '10.0 t/s'], [12.345, '12.3 t/s'], [99.9, '99.9 t/s'], [100.6, '101 t/s']])('formats %s with the reference display precision', (rate, expected) => {
    expect(formatUsageTokensPerSecond({ output_tokens: rate as number, duration_ms: 1000 })).toBe(expected)
  })

  it('omits an unavailable speed', () => {
    expect(formatUsageTokensPerSecond({ output_tokens: 50, duration_ms: null })).toBeNull()
  })
})
