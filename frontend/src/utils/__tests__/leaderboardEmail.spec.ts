import { describe, expect, it } from 'vitest'

import { maskLeaderboardEmail } from '@/utils/leaderboardEmail'

describe('maskLeaderboardEmail', () => {
  it('shows only the first local character and the character before @', () => {
    expect(maskLeaderboardEmail('abcdef@example.com')).toBe('a***f@example.com')
  })

  it('keeps short local parts explicit', () => {
    expect(maskLeaderboardEmail('a@example.com')).toBe('a@example.com')
    expect(maskLeaderboardEmail('ab@example.com')).toBe('ab@example.com')
  })

  it('surfaces empty and invalid input without silent degradation', () => {
    expect(maskLeaderboardEmail('')).toBe('***')
    expect(maskLeaderboardEmail('not-an-email')).toBe('not-an-email')
  })
})
