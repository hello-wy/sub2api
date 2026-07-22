import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const dir = dirname(fileURLToPath(import.meta.url))
const componentSource = readFileSync(resolve(dir, '../AppHeader.vue'), 'utf8')
const styleSource = readFileSync(resolve(dir, '../../../style.css'), 'utf8')

describe('AppHeader mobile layout', () => {
  it('keeps the sidebar trigger as a non-shrinking mobile control', () => {
    expect(componentSource).toContain('app-header-menu-button btn-ghost btn-icon lg:hidden')
    expect(styleSource).toMatch(/\.app-header-menu-button\s*\{[\s\S]*?flex:\s*0 0 2\.5rem;/)
    expect(styleSource).toMatch(/@media \(max-width: 639px\)\s*\{[\s\S]*?\.app-header-leading\s*\{[\s\S]*?flex:\s*0 0 auto;/)
  })

  it('removes secondary toolbar controls before they can displace the menu', () => {
    expect(styleSource).toMatch(/\.header-version-control,[\s\S]*?\.header-checkin-button\s*\{\s*display:\s*none;/)
    expect(styleSource).toMatch(/\.header-user-balance\s*\{\s*display:\s*none;/)
  })
})
