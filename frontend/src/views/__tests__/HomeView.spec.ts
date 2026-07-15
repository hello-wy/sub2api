import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const dir = dirname(fileURLToPath(import.meta.url))
const homeViewSource = readFileSync(resolve(dir, '../HomeView.vue'), 'utf8')

describe('HomeView navigation', () => {
  it('renders internal navigation items with router-link', () => {
    expect(homeViewSource).toContain('v-if="item.internal"')
    expect(homeViewSource).toContain(':to="item.href"')
    expect(homeViewSource).toContain("href: '/models', icon: 'creditCard', internal: true")
    expect(homeViewSource).not.toContain('<a\n          v-for="item in navItems"')
  })

  it('keeps documentation links as safe native anchors', () => {
    expect(homeViewSource).toContain(':href="item.href"')
    expect(homeViewSource).toContain(":target=\"item.external ? '_blank' : undefined\"")
    expect(homeViewSource).toContain(":rel=\"item.external ? 'noopener noreferrer' : undefined\"")
  })
})
