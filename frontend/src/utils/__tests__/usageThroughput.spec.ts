import { describe, expect, it } from 'vitest'
import { getUsageTokensPerSecond } from '../usageThroughput'

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
