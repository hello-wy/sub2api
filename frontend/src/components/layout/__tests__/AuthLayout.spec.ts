import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const dir = dirname(fileURLToPath(import.meta.url))
const layoutSource = readFileSync(resolve(dir, '../AuthLayout.vue'), 'utf8')
const loginSource = readFileSync(resolve(dir, '../../../views/auth/LoginView.vue'), 'utf8')

describe('AuthLayout home-inspired login variant', () => {
  it('uses the dedicated home variant only for the login page', () => {
    expect(loginSource).toContain(":variant=\"embedded ? 'embedded' : 'home'\"")
    expect(layoutSource).toContain("variant?: 'default' | 'home' | 'embedded'")
    expect(layoutSource).toContain('class="auth-embedded-shell"')
    expect(layoutSource).toContain("variant: 'default'")
  })

  it('reuses both home hero assets and renders a solid login panel', () => {
    expect(layoutSource).toContain('/home/solid-api-blue-core-light.webp')
    expect(layoutSource).toContain('/home/solid-api-blue-core.webp')
    expect(layoutSource).toContain('class="auth-home-card"')
    expect(layoutSource).toMatch(/\.auth-home-card\s*\{[\s\S]*?background: #ffffff;/)
    expect(layoutSource).not.toMatch(/\.auth-home-card\s*\{[\s\S]*?backdrop-filter:/)
  })

  it('provides home navigation and an accessible theme switch', () => {
    expect(layoutSource).toContain('to="/home" class="auth-home-link"')
    expect(layoutSource).toContain("t('auth.backToHome')")
    expect(layoutSource).toContain("t('home.switchToLight')")
    expect(layoutSource).toContain("t('home.switchToDark')")
    expect(layoutSource).toContain('@click="toggleTheme"')
  })
})
