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

  it('renders one solid login card over a CSS gradient background', () => {
    expect(layoutSource).toContain('class="auth-minimal-card"')
    expect(layoutSource).toContain('class="auth-minimal-brand"')
    expect(layoutSource).toContain('class="auth-minimal-logo"')
    expect(layoutSource).toMatch(/\.auth-minimal-card\s*\{[\s\S]*?background: #ffffff;/)
    expect(layoutSource).toMatch(/\.auth-minimal-shell\s*\{[\s\S]*?linear-gradient\(/)
    expect(layoutSource).not.toContain('/home/solid-api-blue-core-light.webp')
    expect(layoutSource).not.toContain('/home/solid-api-blue-core.webp')
    expect(layoutSource).not.toContain('backdrop-filter:')
  })

  it('does not render peripheral navigation or decorative content', () => {
    expect(layoutSource).not.toContain('class="auth-home-header"')
    expect(layoutSource).not.toContain('class="auth-home-brand"')
    expect(layoutSource).not.toContain('@click="toggleTheme"')
    expect(layoutSource).toContain('class="auth-minimal-footer"')
  })

  it('never exposes Google login on the login page', () => {
    expect(loginSource).toContain(':google-enabled="false"')
    expect(loginSource).not.toContain('googleOAuthEnabled')
    expect(loginSource).not.toContain('settings.google_oauth_enabled')
  })
})
