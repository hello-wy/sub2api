import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const routerSource = readFileSync(
  resolve(dirname(fileURLToPath(import.meta.url)), '../index.ts'),
  'utf8',
)

describe('membership route', () => {
  it('registers the user membership page behind the payment guard', () => {
    expect(routerSource).toMatch(
      /path: '\/membership'[\s\S]*name: 'Membership'[\s\S]*titleKey: 'loyalty\.title'[\s\S]*requiresPayment: true/
    )
  })

  it('keeps membership reachable when backend mode allows account pages', () => {
    expect(routerSource).toContain("'/membership'")
    expect(routerSource).toContain("'/loyalty'")
    expect(routerSource).toMatch(
      /BACKEND_MODE_ALLOWED_PATHS = \[[^\]]*'\/membership'[^\]]*'\/loyalty'[^\]]*\]/
    )
  })

  it('redirects the old loyalty path to membership', () => {
    expect(routerSource).toMatch(
      /path: '\/loyalty'[\s\S]*redirect: '\/membership'/
    )
  })

  it('marks payment-gated routes with the shared route meta flag', () => {
    expect(routerSource).toContain('requiresPayment: true')
  })
})
