import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const dir = dirname(fileURLToPath(import.meta.url))
const homeSource = readFileSync(resolve(dir, '../HomeView.vue'), 'utf8')

describe('Home inline login', () => {
  it('opens the embedded login panel from both start actions', () => {
    expect(homeSource).toContain('const showInlineLogin = ref(false)')
    expect(homeSource).toContain('@click="openInlineLogin"')
    expect(homeSource).toContain('<LoginView :embedded="true" />')
    expect(homeSource).toContain('class="home-inline-login"')
  })

  it('restores the hero copy after closing the panel', () => {
    expect(homeSource).toContain('@click="closeInlineLogin"')
    expect(homeSource).toContain('v-if="!showInlineLogin"')
    expect(homeSource).toContain('name="home-login" mode="out-in"')
  })
})
