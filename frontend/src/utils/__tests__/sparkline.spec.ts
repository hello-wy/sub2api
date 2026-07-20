import { describe, expect, it } from 'vitest'

import { buildSmoothSparklinePath } from '../sparkline'

describe('buildSmoothSparklinePath', () => {
  it('returns an empty path without data', () => {
    expect(buildSmoothSparklinePath([], 300, 78)).toBe('')
  })

  it('uses quadratic curves for multi-point data', () => {
    const path = buildSmoothSparklinePath([10, 30, 20, 45], 300, 78)

    expect(path).toMatch(/^M /)
    expect(path).toContain(' Q ')
    expect(path).toMatch(/Q 300 .+ 300 .+$/)
    expect(path).not.toContain('NaN')
  })

  it('draws a stable horizontal line for one point', () => {
    expect(buildSmoothSparklinePath([42], 300, 78)).toMatch(/^M 0 .+ L 300 /)
  })
})
