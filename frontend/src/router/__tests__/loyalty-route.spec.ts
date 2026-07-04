import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const routerSource = readFileSync(
  resolve(dirname(fileURLToPath(import.meta.url)), '../index.ts'),
  'utf8',
)

describe('loyalty route', () => {
  it('registers the user loyalty page behind the payment guard', () => {
    expect(routerSource).toMatch(
      /path: '\/loyalty'[\s\S]*name: 'Loyalty'[\s\S]*titleKey: 'loyalty\.title'[\s\S]*requiresPayment: true/
    )
  })

  it('keeps loyalty reachable when backend mode allows account pages', () => {
    expect(routerSource).toContain("'/loyalty'")
    expect(routerSource).toMatch(
      /BACKEND_MODE_ALLOWED_PATHS = \[[^\]]*'\/loyalty'[^\]]*\]/
    )
  })

  it('uses the shared payment feature flag for payment-gated routes', () => {
    expect(routerSource).toContain("isFeatureFlagEnabled(FeatureFlags.payment)")
  })
})
