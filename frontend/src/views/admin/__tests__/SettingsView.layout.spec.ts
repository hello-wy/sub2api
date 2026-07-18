import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const testDirectory = dirname(fileURLToPath(import.meta.url))
const settingsSource = readFileSync(resolve(testDirectory, '../SettingsView.vue'), 'utf8')
const globalStyles = readFileSync(resolve(testDirectory, '../../../style.css'), 'utf8')

describe('SettingsView layout and button surfaces', () => {
  it('keeps the settings tabs in normal flow without a glass pseudo-element', () => {
    const shellRule = settingsSource.match(/\.settings-tabs-shell\s*\{([^}]*)\}/)?.[1] ?? ''

    expect(shellRule).not.toContain('sticky')
    expect(shellRule).not.toContain('backdrop-blur')
    expect(settingsSource).not.toContain('.settings-tab::before')
  })

  it('disables liquid-glass layers for teleported dialog buttons', () => {
    const dialogOverrideIndex = globalStyles.indexOf('.modal-content :where(button, .btn)')
    const componentLayerIndex = globalStyles.indexOf('@layer components')

    expect(dialogOverrideIndex).toBeGreaterThan(-1)
    expect(dialogOverrideIndex).toBeLessThan(componentLayerIndex)
    expect(globalStyles).toMatch(/\.modal-content :where\(button, \.btn\)\s*\{[^}]*backdrop-filter: none;/s)
    expect(globalStyles).toMatch(/\.modal-content :where\(button, \.btn\)::before\s*\{[^}]*content: none;/s)
  })
})
