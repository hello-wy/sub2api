import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const dir = dirname(fileURLToPath(import.meta.url))
const layoutSource = readFileSync(resolve(dir, '../AuthLayout.vue'), 'utf8')
const loginSource = readFileSync(resolve(dir, '../../../views/auth/LoginView.vue'), 'utf8')

describe('AuthLayout branded login variant', () => {
  it('uses the dedicated minimal variant only for the login page', () => {
    expect(loginSource).toContain('<AuthLayout variant="minimal">')
    expect(layoutSource).toContain("variant?: 'default' | 'minimal'")
    expect(layoutSource).toContain("variant: 'default'")
  })

  it('renders one frosted login card over the application shell background', () => {
    expect(layoutSource).toContain('class="auth-minimal-card"')
    expect(layoutSource).toContain('class="auth-minimal-brand"')
    expect(layoutSource).toContain('class="auth-minimal-logo"')
    expect(layoutSource).toMatch(/\.auth-minimal-card\s*\{[\s\S]*?border-radius: 20px;/)
    expect(layoutSource).toMatch(/\.auth-minimal-card\s*\{[\s\S]*?background: rgba\(255, 255, 255, 0\.58\);/)
    expect(layoutSource).toMatch(/\.auth-minimal-card\s*\{[\s\S]*?backdrop-filter: blur\(22px\)/)
    expect(layoutSource).toMatch(/\.auth-minimal-shell\s*\{[\s\S]*?background-color: #f0f7ff;/)
    expect(layoutSource).toMatch(/\.dark \.auth-minimal-shell\s*\{[\s\S]*?background-color: #0c1d31;/)
    expect(layoutSource).not.toContain('/home/solid-api-blue-core-light.webp')
    expect(layoutSource).not.toContain('/home/solid-api-blue-core.webp')
  })

  it('uses rounded translucent inputs and a pill-shaped primary action', () => {
    expect(loginSource).toMatch(/\.login-form \.input\s*\{[\s\S]*?border-radius: 14px;/)
    expect(loginSource).toMatch(/\.login-form \.input\s*\{[\s\S]*?background: rgba\(248, 251, 255, 0\.7\);/)
    expect(loginSource).toMatch(/\.login-submit\s*\{[\s\S]*?border-radius: 999px;/)
  })

  it('does not restore the previous full-page hero navigation', () => {
    expect(layoutSource).not.toContain('class="auth-home-header"')
    expect(layoutSource).not.toContain('class="auth-home-brand"')
    expect(layoutSource).toContain('class="auth-minimal-footer"')
  })

  it('shows a corner brand and provides locale and theme controls on the panel', () => {
    expect(layoutSource).toContain('class="auth-corner-brand"')
    expect(layoutSource).toContain('<LocaleSwitcher class="auth-minimal-locale" toolbar />')
    expect(layoutSource).toContain('class="auth-minimal-tool-button"')
    expect(layoutSource).toContain('@click="toggleTheme"')
  })

  it('never exposes Google login on the login page', () => {
    expect(loginSource).toContain(':google-enabled="false"')
    expect(loginSource).not.toContain('googleOAuthEnabled')
    expect(loginSource).not.toContain('settings.google_oauth_enabled')
  })
})
