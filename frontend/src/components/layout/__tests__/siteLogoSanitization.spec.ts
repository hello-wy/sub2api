import { existsSync, readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const dir = dirname(fileURLToPath(import.meta.url))
const sidebarSource = readFileSync(resolve(dir, '../AppSidebar.vue'), 'utf8')
const homeViewSource = readFileSync(resolve(dir, '../../../views/HomeView.vue'), 'utf8')
const keyUsageViewSource = readFileSync(resolve(dir, '../../../views/KeyUsageView.vue'), 'utf8')

describe('site logo handling', () => {
  it('uses the SolidAPI brand lockups on the public home and authenticated sidebar', () => {
    for (const src of [sidebarSource, homeViewSource]) {
      expect(src).toContain('/brand/solidapi-lockup-light.png')
      expect(src).toContain('/brand/solidapi-lockup-dark.png')
    }

    expect(sidebarSource).toContain('/brand/solidapi-mark.png')
    expect(existsSync(resolve(dir, '../../../../public/brand/solidapi-lockup-light.png'))).toBe(true)
    expect(existsSync(resolve(dir, '../../../../public/brand/solidapi-lockup-dark.png'))).toBe(true)
    expect(existsSync(resolve(dir, '../../../../public/brand/solidapi-mark.png'))).toBe(true)
  })

  it('KeyUsageView applies sanitizeUrl to siteLogo', () => {
    expect(keyUsageViewSource).toContain('sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo')
  })

  it('keeps sanitization options for the remaining configurable logo surface', () => {
    expect(keyUsageViewSource).toContain('allowRelative: true')
    expect(keyUsageViewSource).toContain('allowDataUrl: true')
  })
})
