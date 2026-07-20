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

  it('does not inject liquid-glass layers into generic application buttons', () => {
    expect(globalStyles).not.toMatch(/(?:^|\n):where\(button, \.btn\)\s*\{/)
    expect(globalStyles).not.toMatch(/(?:^|\n):where\(button, \.btn\)::before\s*\{/)
    expect(globalStyles).toContain('.liquid-glass-button')
  })
})
