import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const dir = dirname(fileURLToPath(import.meta.url))
const componentSource = readFileSync(resolve(dir, '../AppLayout.vue'), 'utf8')
const styleSource = readFileSync(resolve(dir, '../../../style.css'), 'utf8')

describe('AppLayout workspace scrolling', () => {
  it('keeps the app in the viewport and delegates scrolling to the content panel', () => {
    expect(componentSource).toContain('class="app-shell h-dvh overflow-hidden"')
    expect(componentSource).toContain('class="app-content-stage flex min-h-0 flex-1"')
    expect(styleSource).toMatch(/\.app-page-panel\s*\{[\s\S]*?overflow-y: auto;/)
    expect(styleSource).toMatch(/\.app-page-panel\s*\{[\s\S]*?scrollbar-gutter: stable;/)
  })
})
