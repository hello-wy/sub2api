import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const componentPath = resolve(dirname(fileURLToPath(import.meta.url)), '../TablePageLayout.vue')
const componentSource = readFileSync(componentPath, 'utf8')

describe('TablePageLayout responsive table scrolling', () => {
  it('supports an opt-in toolbar that keeps filters and actions in one row', () => {
    expect(componentSource).toContain('unifiedToolbar')
    expect(componentSource).toContain('class="layout-toolbar"')
    expect(componentSource).toContain('class="layout-toolbar-filters"')
    expect(componentSource).toContain('class="layout-toolbar-actions"')
  })

  it('bounds the table layout and delegates overflow to the table wrapper', () => {
    expect(componentSource).toContain('@apply flex h-full min-h-0 min-w-0 flex-col gap-6;')
    expect(componentSource).not.toContain('height: calc(100vh')
    expect(componentSource).toContain('@apply flex min-h-0 min-w-0 flex-1 flex-col;')
    expect(componentSource).toContain('@apply min-h-0 min-w-0 flex-1 overflow-x-auto overflow-y-auto;')
  })

  it('keeps the bounded table container in mobile mode', () => {
    expect(componentSource).not.toContain('h-auto overflow-visible')
    expect(componentSource).not.toContain('flex-none min-h-fit')
  })

  it('does not disable the table horizontal scroll container in mobile mode', () => {
    const tableWrapperBlocks = Array.from(
      componentSource.matchAll(/([^{}]*:deep\(\.table-wrapper\)[^{}]*)\{([^{}]*)\}/g)
    )

    expect(tableWrapperBlocks.length).toBeGreaterThan(0)

    const baseBlock = tableWrapperBlocks.find(([selector]) => !selector.includes('.mobile-mode'))
    const mobileBlocks = tableWrapperBlocks.filter(([selector]) => selector.includes('.mobile-mode'))

    expect(baseBlock?.[2]).toContain('overflow-x-auto')
    expect(mobileBlocks.every(([, , declarations]) => !declarations.includes('overflow-visible'))).toBe(
      true
    )
  })
})
