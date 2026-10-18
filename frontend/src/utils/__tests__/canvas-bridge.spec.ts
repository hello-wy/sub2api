import { describe, expect, it } from 'vitest'
import { buildCanvasUrl, CANVAS_PROTOCOL, trustedCanvasMessage } from '../canvas-bridge'

describe('Canvas bridge trust boundary', () => {
  const frame = {} as Window
  const message = (origin: string, source: Window | null = frame, protocol = CANVAS_PROTOCOL) =>
    ({ origin, source, data: { protocol } }) as MessageEvent

  it('requires both the exact configured origin and iframe window', () => {
    expect(trustedCanvasMessage(message('https://canvas.example'), frame, 'https://canvas.example/')).toBe(true)
    expect(trustedCanvasMessage(message('https://evil.example'), frame, 'https://canvas.example/')).toBe(false)
    expect(trustedCanvasMessage(message('https://canvas.example', {} as Window), frame, 'https://canvas.example/')).toBe(false)
    expect(trustedCanvasMessage(message('https://canvas.example', frame, 'other'), frame, 'https://canvas.example/')).toBe(false)
    expect(trustedCanvasMessage(message('https://canvas.example'), null, 'https://canvas.example/')).toBe(false)
  })

  it('does not put identity, login tokens or API keys in the iframe URL', () => {
    const url = new URL(buildCanvasUrl('https://canvas.example/', 'dark', 'zh-CN'))
    expect([...url.searchParams.keys()].sort()).toEqual(['host_origin', 'integration', 'lang', 'theme'])
    expect(url.searchParams.get('integration')).toBe('sub2api')
    expect(() => buildCanvasUrl('javascript:alert(1)', 'light', 'zh-CN')).toThrow()
  })
})
