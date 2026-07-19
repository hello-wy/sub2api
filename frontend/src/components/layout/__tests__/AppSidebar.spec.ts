import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const componentPath = resolve(dirname(fileURLToPath(import.meta.url)), '../AppSidebar.vue')
const componentSource = readFileSync(componentPath, 'utf8')
const headerSource = readFileSync(resolve(dirname(fileURLToPath(import.meta.url)), '../AppHeader.vue'), 'utf8')
const stylePath = resolve(dirname(fileURLToPath(import.meta.url)), '../../../style.css')
const styleSource = readFileSync(stylePath, 'utf8')

describe('AppSidebar custom SVG styles', () => {
  it('does not override uploaded SVG fill or stroke colors', () => {
    expect(componentSource).toContain('.sidebar-svg-icon {')
    expect(componentSource).toContain('color: currentColor;')
    expect(componentSource).toContain('display: block;')
    expect(componentSource).not.toContain('stroke: currentColor;')
    expect(componentSource).not.toContain('fill: none;')
  })
})

describe('AppSidebar scroll position persistence', () => {
  it('binds a template ref to the sidebar nav element', () => {
    expect(componentSource).toContain('ref="sidebarNavRef"')
    expect(componentSource).toContain('sidebar-nav')
  })

  it('declares sidebarNavRef in script setup', () => {
    expect(componentSource).toContain("const sidebarNavRef = ref<HTMLElement | null>(null)")
  })

  it('saves scroll position on beforeUnmount', () => {
    expect(componentSource).toContain('onBeforeUnmount')
    expect(componentSource).toContain('appStore.sidebarScrollTop')
    expect(componentSource).toContain('sidebarNavRef.value.scrollTop')
  })

  it('restores scroll position on mount', () => {
    expect(componentSource).toContain('onMounted')
    expect(componentSource).toContain('appStore.sidebarScrollTop')
    expect(componentSource).toContain('nextTick')
  })
})

describe('AppSidebar header styles', () => {
  it('moves the admin-only version control into the top toolbar', () => {
    expect(componentSource).not.toContain('VersionBadge')
    expect(headerSource).toMatch(/<VersionBadge[\s\S]*?v-if="authStore\.isAdmin"[\s\S]*?toolbar/)
    expect(headerSource.indexOf('<VersionBadge')).toBeLessThan(headerSource.indexOf('<AnnouncementBell'))
  })

  it('does not clip the sidebar logo', () => {
    const sidebarHeaderBlockMatch = styleSource.match(/\.sidebar-header\s*\{[\s\S]*?\n {2}\}/)

    expect(sidebarHeaderBlockMatch).not.toBeNull()
    expect(sidebarHeaderBlockMatch?.[0]).not.toContain('@apply overflow-hidden;')
    expect(sidebarHeaderBlockMatch?.[0]).not.toContain('@apply border-b')
  })
})

describe('AppSidebar liquid glass states', () => {
  it('keeps nested sidebar buttons flat until their navigation item is active', () => {
    expect(componentSource).toContain(
      ':global(.sidebar-liquid-shell button:not(.sidebar-link-active))'
    )
    expect(componentSource).toContain(
      ':global(.sidebar-liquid-shell button:not(.sidebar-link-active)::before)'
    )
    expect(styleSource).toMatch(/\.sidebar-link-active\s*\{[\s\S]*?backdrop-filter: blur\(12px\)/)
  })

  it('does not draw a vertical divider on the sidebar shell', () => {
    expect(componentSource).toContain(
      'class="sidebar-liquid-shell relative flex h-full flex-col"'
    )
    expect(componentSource).toContain('border-color: transparent;')
    expect(componentSource).toContain('box-shadow: none;')
  })
})

describe('AppSidebar user navigation', () => {
  it('does not include legacy image generation or recharge address menu entries', () => {
    expect(componentSource).not.toContain("path: '/image-generation'")
    expect(componentSource).not.toContain("path: '/recharge-address'")
    expect(componentSource).not.toContain("t('nav.imageGeneration')")
    expect(componentSource).not.toContain("t('nav.rechargeAddress')")
  })

  it('includes the membership page behind the payment feature flag', () => {
    expect(componentSource).toMatch(
      /\{ path: '\/membership', label: t\('nav\.loyalty'\), icon: GiftIcon, hideInSimpleMode: true, featureFlag: flagPayment \}/
    )
  })

  it('places the wallet first in personal navigation and uses the card icon', () => {
    const sharedItems = componentSource.slice(
      componentSource.indexOf('function buildSelfNavItems'),
      componentSource.indexOf('// finalizeNav'),
    )
    expect(sharedItems.indexOf("path: '/wallet'")).toBeLessThan(sharedItems.indexOf("path: '/keys'"))
    expect(sharedItems).toContain("icon: CreditCardIcon")
  })

  it('places membership in the shared My Account navigation used by admins', () => {
    expect(componentSource).toMatch(
      /{{ t\('nav\.myAccount'\) }}[\s\S]*v-for="item in personalNavItems"/
    )
    expect(componentSource).toContain(
      'const personalNavItems = computed((): NavItem[] => finalizeNav(buildSelfNavItems(false)))'
    )
    expect(componentSource).toMatch(
      /function buildSelfNavItems\(withDashboard: boolean\): NavItem\[] \{[\s\S]*path: '\/membership'/
    )
  })
})
