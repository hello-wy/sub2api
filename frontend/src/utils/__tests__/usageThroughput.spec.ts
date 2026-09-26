import { describe, expect, it } from 'vitest'
import { formatUsageTokensPerSecond, getUsageTokensPerSecond } from '../usageThroughput'

describe('getUsageTokensPerSecond', () => {
  it('prefers the server-calculated value', () => {
    expect(getUsageTokensPerSecond({ tps: 12.5, output_tokens: 50, duration_ms: 2500 })).toBe(12.5)
  })

  it('derives TPS for legacy API payloads', () => {
    expect(getUsageTokensPerSecond({ tps: null, output_tokens: 50, duration_ms: 2500 })).toBe(20)
  })

  it('returns null when the request has no usable throughput inputs', () => {
    expect(getUsageTokensPerSecond({ tps: null, output_tokens: 0, duration_ms: 2500 })).toBeNull()
    expect(getUsageTokensPerSecond({ tps: null, output_tokens: 50, duration_ms: null })).toBeNull()
  })
})

describe('formatUsageTokensPerSecond', () => {
  it.each([[9.99, '10.0 t/s'], [12.345, '12.3 t/s'], [99.9, '99.9 t/s'], [100.6, '101 t/s']])('formats %s with the reference display precision', (tps, expected) => {
    expect(formatUsageTokensPerSecond({ tps: tps as number, output_tokens: 0, duration_ms: null })).toBe(expected)
  })

  it('preserves the API throughput value and the legacy fallback', () => {
    expect(formatUsageTokensPerSecond({ tps: 12.5, output_tokens: 50, duration_ms: 2500 })).toBe('12.5 t/s')
    expect(formatUsageTokensPerSecond({ tps: null, output_tokens: 50, duration_ms: 2500 })).toBe('20.0 t/s')
  })

  it('omits an unavailable or non-finite speed', () => {
    expect(formatUsageTokensPerSecond({ tps: null, output_tokens: 0, duration_ms: null })).toBeNull()
    expect(formatUsageTokensPerSecond({ tps: null, output_tokens: Infinity, duration_ms: 1000 })).toBeNull()
  })
})
