import { describe, expect, it } from 'vitest'

import { durationSeverity, firstTokenSeverity, tpsSeverity } from '../latencyHealth'

describe('latencyHealth', () => {
  it('classifies first-token latency at 10s/30s/60s boundaries', () => {
    expect(firstTokenSeverity(0)).toBe('good')
    expect(firstTokenSeverity(9_999)).toBe('good')
    expect(firstTokenSeverity(10_000)).toBe('warn')
    expect(firstTokenSeverity(29_999)).toBe('warn')
    expect(firstTokenSeverity(30_000)).toBe('slow')
    expect(firstTokenSeverity(59_999)).toBe('slow')
    expect(firstTokenSeverity(60_000)).toBe('critical')
  })

  it('classifies total duration at 1min/3min/5min boundaries', () => {
    expect(durationSeverity(0)).toBe('good')
    expect(durationSeverity(59_999)).toBe('good')
    expect(durationSeverity(60_000)).toBe('warn')
    expect(durationSeverity(179_999)).toBe('warn')
    expect(durationSeverity(180_000)).toBe('slow')
    expect(durationSeverity(299_999)).toBe('slow')
    expect(durationSeverity(300_000)).toBe('critical')
  })

  it.each([25, 100, 1000, 10000])('classifies dynamic TPS boundaries for %i output tokens', (tokens) => {
    const yellow = tokens / (10 + tokens / 20)
    const orange = tokens / (30 + tokens / 10)
    const red = tokens / (60 + tokens / 5)
    expect(tpsSeverity(yellow + 0.001, tokens)).toBe('good')
    expect(tpsSeverity(yellow, tokens)).toBe('warn')
    expect(tpsSeverity(orange + 0.001, tokens)).toBe('warn')
    expect(tpsSeverity(orange, tokens)).toBe('slow')
    expect(tpsSeverity(red + 0.001, tokens)).toBe('slow')
    expect(tpsSeverity(red, tokens)).toBe('critical')
  })

  it('tolerates short output while applying stronger expectations to long output', () => {
    expect(tpsSeverity(5, 25)).toBe('good')
    expect(tpsSeverity(5, 100)).toBe('warn')
    expect(tpsSeverity(5, 1000)).toBe('slow')
    expect(tpsSeverity(3, 1000)).toBe('critical')
  })

  it('handles unusable inputs defensively', () => {
    expect(tpsSeverity(0, 100)).toBe('critical')
    expect(tpsSeverity(NaN, 100)).toBe('critical')
    expect(tpsSeverity(20, 0)).toBe('critical')
    expect(tpsSeverity(20, Infinity)).toBe('critical')
  })
})
