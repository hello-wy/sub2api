import type { Proxy } from '@/types'

export function isCodexImportProxyAvailable(proxy: Proxy, now = Date.now()): boolean {
  if (proxy.status !== 'active') return false
  return !proxy.expires_at || new Date(proxy.expires_at).getTime() > now
}

export function countCodexImportEntries(content: string): number {
  const trimmed = content.trim()
  if (!trimmed) return 0
  const countValue = (value: unknown): number => Array.isArray(value)
    ? value.reduce((count, item) => count + countValue(item), 0)
    : 1
  try {
    return countValue(JSON.parse(trimmed))
  } catch {
    const lines = trimmed.split('\n').map((line) => line.trim()).filter(Boolean)
    let count = 0
    for (const line of lines) {
      if (/^[{["]/.test(line)) {
        try { count += countValue(JSON.parse(line)) } catch { return 0 }
      } else {
        count++
      }
    }
    return count
  }
}
