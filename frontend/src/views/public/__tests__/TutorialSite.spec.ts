// @vitest-environment node
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { JSDOM } from 'jsdom'
import { afterEach, describe, expect, it, vi } from 'vitest'

const tutorialRoot = resolve(import.meta.dirname, '../../../../public/tutorial')
const html = readFileSync(resolve(tutorialRoot, 'index.html'), 'utf8')
const script = readFileSync(resolve(tutorialRoot, 'assets/app.js'), 'utf8')
const windows: JSDOM[] = []

function openTutorial(options: { apiBaseUrl?: string; usePublicSettings?: boolean; fetch?: ReturnType<typeof vi.fn> } = {}) {
  const dom = new JSDOM(html, {
    url: 'https://solidapi.example/tutorial/',
    runScripts: 'outside-only',
    pretendToBeVisual: true
  })
  windows.push(dom)
  const { window } = dom
  Object.assign(window, {
    TUTORIAL_CONFIG: { apiBaseUrl: 'https://solidapi.example', ...options },
    fetch: options.fetch ?? vi.fn().mockRejectedValue(new Error('offline')),
    matchMedia: () => ({ matches: true }),
    scrollTo: vi.fn()
  })
  window.HTMLElement.prototype.scrollIntoView = vi.fn()
  window.eval(script)
  return window
}

afterEach(() => {
  windows.splice(0).forEach(dom => dom.window.close())
})

describe('SolidAPI standalone tutorial configuration', () => {
  it('uses the full SolidAPI wordmark on desktop and mobile', () => {
    const window = openTutorial({ usePublicSettings: false })
    const logos = Array.from(window.document.querySelectorAll('.brand img'))
    expect(logos).toHaveLength(2)
    for (const logo of logos) {
      expect(logo.getAttribute('src')).toBe('/brand/solidapi-lockup-light.png')
      expect(logo.getAttribute('alt')).toBe('SolidAPI')
    }
  })

  it('enables the console tutorial button as a full-page link', () => {
    const header = readFileSync(resolve(tutorialRoot, '../../src/components/layout/AppHeader.vue'), 'utf8')
    const link = header.match(/<a\s[^>]*href="\/tutorial\/"[^>]*>[\s\S]*?<\/a>/)?.[0]
    expect(link).toContain("t('header.tutorial')")
    expect(link).not.toMatch(/disabled|aria-disabled/)
  })

  it.each(['https://api.solidapi.example', 'https://api.solidapi.example/v1/'])(
    'generates client-specific endpoints from %s', apiBaseUrl => {
      const fetch = vi.fn()
      const window = openTutorial({ apiBaseUrl, usePublicSettings: false, fetch })
      const document = window.document
      expect((document.querySelector('#api-base-url') as HTMLInputElement).value).toBe('https://api.solidapi.example/v1')
      expect(document.querySelector('#manual-config')?.textContent).toContain('base_url = "https://api.solidapi.example/v1"')
      expect(document.querySelector('#claude-macos-env')?.textContent).toContain("ANTHROPIC_BASE_URL='https://api.solidapi.example'")
      expect(document.querySelector('#claude-windows-env')?.textContent).toContain("'ANTHROPIC_BASE_URL', 'https://api.solidapi.example'")
      expect(document.querySelector('#claude-macos-env')?.textContent).toContain('ANTHROPIC_AUTH_TOKEN')
      expect(fetch).not.toHaveBeenCalled()
    }
  )

  it('loads the public API endpoint without overwriting input or the selected client', async () => {
    let finish!: (value: unknown) => void
    const fetch = vi.fn(() => new Promise(resolve => { finish = resolve }))
    const window = openTutorial({ fetch })
    const input = window.document.querySelector('#api-base-url') as HTMLInputElement
    input.value = 'https://custom.solidapi.example/v1'
    ;(window.document.querySelector('[data-client-tab="claude"]') as HTMLButtonElement).click()
    finish({ ok: true, json: async () => ({ code: 0, data: { api_base_url: 'https://configured.solidapi.example/v1/' } }) })
    await vi.waitFor(() => {
      expect(window.document.querySelector('#manual-config')?.textContent).toContain('https://configured.solidapi.example/v1')
    })
    expect(input.value).toBe('https://custom.solidapi.example/v1')
    expect(window.document.title).toBe('Claude Code 接入教程 - SolidAPI')
    expect(window.document.querySelector('#claude-macos-env')?.textContent).toContain("ANTHROPIC_BASE_URL='https://configured.solidapi.example'")
    expect(fetch).toHaveBeenCalledWith('/api/v1/settings/public', expect.objectContaining({ credentials: 'omit' }))
  })

  it('keeps usable local configuration if the settings endpoint is unavailable', async () => {
    const window = openTutorial()
    await Promise.resolve()
    expect(window.document.querySelector('#manual-config')?.textContent).toContain('https://solidapi.example/v1')
    expect((window.document.querySelector('#api-base-url') as HTMLInputElement).value).toBe('https://solidapi.example/v1')
    expect(window.document.querySelector('#keys-page-button')?.getAttribute('href')).toBe('/keys')
    expect(window.localStorage.length).toBe(0)
  })
})
