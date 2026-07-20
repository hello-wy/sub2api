import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const componentPath = resolve(dirname(fileURLToPath(import.meta.url)), '../ScrollablePageLayout.vue')
const componentSource = readFileSync(componentPath, 'utf8')

describe('ScrollablePageLayout', () => {
  it('bounds route content and assigns vertical scrolling to one child', () => {
    expect(componentSource).toContain('flex h-full min-h-0 min-w-0 flex-col')
    expect(componentSource).toContain('min-h-0 min-w-0 flex-1 overflow-y-auto')
  })

  it('provides responsive default page padding and custom content classes', () => {
    expect(componentSource).toContain("contentClass: 'p-4 md:p-6 lg:p-8'")
    expect(componentSource).toContain("['min-h-0 min-w-0 flex-1 overflow-y-auto', contentClass]")
  })
})
