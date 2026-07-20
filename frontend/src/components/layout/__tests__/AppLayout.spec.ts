import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const dir = dirname(fileURLToPath(import.meta.url))
const componentSource = readFileSync(resolve(dir, '../AppLayout.vue'), 'utf8')
const appSource = readFileSync(resolve(dir, '../../../App.vue'), 'utf8')
const styleSource = readFileSync(resolve(dir, '../../../style.css'), 'utf8')

describe('AppLayout viewport containment', () => {
  it('keeps the app in the viewport without page-panel scrolling or shared padding', () => {
    expect(componentSource).toContain('class="app-shell h-dvh overflow-hidden"')
    expect(componentSource).toContain('class="app-content-stage flex min-h-0 min-w-0 flex-1 overflow-hidden"')
    expect(componentSource).toContain('class="app-page-content app-page-panel min-h-0 min-w-0 flex-1 overflow-hidden"')
    expect(componentSource).not.toContain('p-4 md:p-6 lg:p-8')
    expect(styleSource).toMatch(/\.app-content-stage\s*\{[^}]*min-width: 0;/)
    expect(styleSource).not.toMatch(/\.app-content-stage\s*\{[^}]*padding:/)
    expect(styleSource).toMatch(/\.app-page-panel\s*\{[\s\S]*?overflow: hidden;/)
    expect(styleSource).not.toMatch(/\.app-page-panel\s*\{[\s\S]*?overflow-y: auto;/)
    expect(styleSource).not.toMatch(/\.app-page-panel\s*\{[\s\S]*?scrollbar-gutter:/)
  })

  it('uses a flat shared canvas without page-panel scrolling', () => {
    expect(componentSource).not.toContain('flatPanel')
    expect(componentSource).not.toContain('app-page-panel--flat')
    expect(styleSource).toMatch(/\.app-page-panel\s*\{[\s\S]*?background: transparent;/)
    expect(styleSource).not.toContain('.app-page-panel--flat')
  })

  it('mounts async route content without waiting for the previous page to leave', () => {
    expect(appSource).toContain('<Transition name="page-fade" :duration="pageTransitionDuration">')
    expect(appSource).not.toContain('mode="out-in"')
  })

  it('overlays route shells and hands off the stable toolbar before revealing new content', () => {
    expect(appSource).toContain('{ enter: 480, leave: 190 }')
    expect(styleSource).toMatch(/\.page-fade-enter-active,\s*\n\.page-fade-leave-active\s*\{[\s\S]*?position: fixed;/)
    expect(styleSource).toContain('page-content-fade-in 290ms cubic-bezier(0.16, 1, 0.3, 1) 190ms both')
    expect(styleSource).toContain('page-content-fade-out 190ms cubic-bezier(0.4, 0, 1, 1) both')
    expect(styleSource).toContain('route-toolbar-handoff 1ms step-end 189ms both')
  })
})
