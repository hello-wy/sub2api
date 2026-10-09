import { describe, expect, it } from 'vitest'
import { formatUsageTokensPerSecond, getUsageTokensPerSecond } from '../usageThroughput'

describe('getUsageTokensPerSecond', () => {
  it('excludes initial waiting and ignores the legacy server average', () => {
    const row = { tps: 8, output_tokens: 200, duration_ms: 25000, first_token_ms: 20000 }
    expect(getUsageTokensPerSecond(row)).toBe(40)
    expect(getUsageTokensPerSecond({ ...row, duration_ms: 65000, first_token_ms: 60000 })).toBe(40)
  })

  it.each([
    { output_tokens: 0 }, { output_tokens: Infinity }, { duration_ms: null },
    { first_token_ms: null }, { first_token_ms: -1 }, { first_token_ms: Infinity },
    { duration_ms: NaN }, { first_token_ms: 2600 }, { first_token_ms: 2500 },
    { first_token_ms: 2499 }, { first_token_ms: 2490 }, { first_token_ms: 2401 },
  ])('omits unusable timing or usage: %o', (overrides) => {
    expect(getUsageTokensPerSecond({ output_tokens: 50, duration_ms: 2500, first_token_ms: 2000, ...overrides })).toBeNull()
  })

  it('accepts a 100ms output window and zero initial wait', () => {
    expect(getUsageTokensPerSecond({ output_tokens: 5, duration_ms: 2500, first_token_ms: 2400 })).toBe(50)
    expect(getUsageTokensPerSecond({ output_tokens: 50, duration_ms: 2500, first_token_ms: 0 })).toBe(20)
  })
})

describe('formatUsageTokensPerSecond', () => {
  it.each([[9.99, '10.0 t/s'], [12.345, '12.3 t/s'], [99.9, '99.9 t/s'], [100.6, '101 t/s']])('formats %s with the reference display precision', (rate, expected) => {
    expect(formatUsageTokensPerSecond({ output_tokens: rate as number, duration_ms: 2000, first_token_ms: 1000 })).toBe(expected)
  })

  it('omits an unavailable speed', () => {
    expect(formatUsageTokensPerSecond({ output_tokens: 50, duration_ms: 2500, first_token_ms: null })).toBeNull()
  })
})
