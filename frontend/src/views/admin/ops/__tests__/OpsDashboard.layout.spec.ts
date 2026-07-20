import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const componentPath = resolve(dirname(fileURLToPath(import.meta.url)), '../OpsDashboard.vue')
const componentSource = readFileSync(componentPath, 'utf8')

describe('OpsDashboard viewport scrolling', () => {
  it('keeps the normal dashboard within AppLayout while owning vertical scrolling', () => {
    expect(componentSource).toContain(":is=\"isFullscreen ? 'div' : AppLayout\"")
    expect(componentSource).toContain("'flex h-full min-h-0 flex-col'")
    expect(componentSource).toContain("'min-h-0 flex-1 overflow-y-auto p-4 md:p-6'")
  })

  it('keeps fullscreen viewport-sized, top-aligned, and scrollable', () => {
    expect(componentSource).toContain("'flex min-h-screen flex-col bg-gray-50 dark:bg-dark-950'")
    expect(componentSource).toContain("'flex min-h-0 flex-1 flex-col'")
    expect(componentSource).not.toContain('justify-center')
  })
})
