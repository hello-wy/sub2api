import { describe, expect, it } from 'vitest'

import { resolvePostLoginRedirect } from '../postLoginRedirect'

describe('resolvePostLoginRedirect', () => {
  it('routes each role directly to its dashboard by default', () => {
    expect(resolvePostLoginRedirect(undefined, true)).toBe('/admin/dashboard')
    expect(resolvePostLoginRedirect(undefined, false)).toBe('/dashboard')
  })

  it('avoids the intermediate user dashboard redirect for admins', () => {
    expect(resolvePostLoginRedirect('/dashboard', true)).toBe('/admin/dashboard')
  })

  it('keeps users out of admin redirects', () => {
    expect(resolvePostLoginRedirect('/admin/users', false)).toBe('/dashboard')
  })

  it('rejects external redirect targets', () => {
    expect(resolvePostLoginRedirect('https://example.com', true)).toBe('/admin/dashboard')
    expect(resolvePostLoginRedirect('//example.com', false)).toBe('/dashboard')
  })
})
