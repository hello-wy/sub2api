/** Canvas receives only its user's gateway key, never the panel login token. */
export const CANVAS_PROTOCOL = 'sub2api-canvas-v1'
export function trustedCanvasMessage(event: MessageEvent, frame: Window | null, url: string): boolean {
  if (!frame || event.source !== frame) return false
  try {
    return event.origin === new URL(url).origin && event.data?.protocol === CANVAS_PROTOCOL
  } catch { return false }
}

export function buildCanvasUrl(url: string, theme: string, language: string): string {
  const target = new URL(url)
  if (!['http:', 'https:'].includes(target.protocol)) throw new Error('画布地址必须使用 HTTP 或 HTTPS')
  target.searchParams.set('integration', 'sub2api')
  // The Canvas build may be served from a different host or have no build-time
  // Sub2API origin configured. Pass the actual embedding origin explicitly.
  if (typeof window !== 'undefined') target.searchParams.set('host_origin', window.location.origin)
  target.searchParams.set('theme', theme)
  target.searchParams.set('lang', language)
  return target.toString()
}
