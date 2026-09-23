import type { AdminDataPayload } from '@/types'

export const DEFAULT_IMPORT_MODELS = ['codex-auto-review', 'gpt-5.6', 'gpt-5.6-sol', 'gpt-6', 'gpt-6-astra']
export const MAX_IMPORT_BYTES = 20 * 1024 * 1024
export const MAX_IMPORT_ACCOUNTS = 5000
export interface ImportPreviewRow { source: string; name: string; platform: string; type: string }
export interface AccountImportPreview {
  kind: 'backup' | 'codex'
  rows: ImportPreviewRow[]
  data?: AdminDataPayload
  contents?: string[]
}
export class ImportPreviewError extends Error {
  constructor(public key: string, public source = '') { super(key) }
}
const object = (v: unknown): v is Record<string, unknown> => !!v && typeof v === 'object' && !Array.isArray(v)
const label = (v: unknown, fallback: string) => typeof v === 'string' && v.trim() ? v.trim().slice(0, 120) : fallback
const validBackup = (v: Record<string, unknown>) =>
  (v.type === undefined || v.type === '' || v.type === 'sub2api-data' || v.type === 'sub2api-bundle') &&
  (v.version === undefined || [0, 1, 2].includes(v.version as number)) && Array.isArray(v.proxies) && Array.isArray(v.accounts)
const codexEntry = (v: unknown): boolean => {
  if (typeof v === 'string') return v.trim().length > 0
  if (!object(v)) return false
  const tokens = object(v.tokens) ? v.tokens : {}
  const credentials = object(v.credentials) ? v.credentials : {}
  return [v.access_token, v.accessToken, tokens.access_token, tokens.accessToken, credentials.access_token, credentials.accessToken]
    .some(t => typeof t === 'string' && t.trim()) || object(v.agent_identity) || object(v.agentIdentity) || v.auth_mode === 'agent_identity'
}
export function parseAccountImportFiles(files: Array<{ name: string; text: string }>): AccountImportPreview {
  const rows: ImportPreviewRow[] = []
  const backups: AdminDataPayload[] = []
  const contents: string[] = []
  let kind: AccountImportPreview['kind'] | undefined
  for (const file of files) {
    let parsed: unknown
    try { parsed = JSON.parse(file.text.replace(/^\uFEFF/, '')) } catch { throw new ImportPreviewError('parseFailed', file.name) }
    const backup = object(parsed) && ('accounts' in parsed || 'proxies' in parsed || parsed.type === 'sub2api-data' || parsed.type === 'sub2api-bundle')
    const nextKind = backup ? 'backup' : 'codex'
    if (kind && kind !== nextKind) throw new ImportPreviewError('mixedFormats')
    kind = nextKind
    if (backup) {
      if (!object(parsed) || !validBackup(parsed)) throw new ImportPreviewError('invalidFile', file.name)
      const payload = parsed as unknown as AdminDataPayload
      for (const account of payload.accounts) {
        if (!object(account)) throw new ImportPreviewError('invalidFile', file.name)
        rows.push({ source: file.name, name: label(account.name, file.name), platform: label(account.platform, '—'), type: label(account.type, '—') })
      }
      backups.push(payload)
    } else {
      const stack = [parsed]
      let index = 0
      while (stack.length) {
        const value = stack.pop()
        if (Array.isArray(value)) {
          if (value.length + stack.length > MAX_IMPORT_ACCOUNTS) throw new ImportPreviewError('tooMany')
          for (let i = value.length - 1; i >= 0; i--) stack.push(value[i])
          continue
        }
        if (!codexEntry(value)) throw new ImportPreviewError('invalidFile', file.name)
        const record = object(value) ? value : {}
        rows.push({ source: file.name, name: label(record.name || record.email, `${file.name} #${++index}`), platform: 'openai', type: 'oauth' })
        if (rows.length > MAX_IMPORT_ACCOUNTS) throw new ImportPreviewError('tooMany')
      }
      contents.push(file.text.replace(/^\uFEFF/, ''))
    }
    if (rows.length > MAX_IMPORT_ACCOUNTS) throw new ImportPreviewError('tooMany')
  }
  if (!rows.length) throw new ImportPreviewError('empty')
  if (kind === 'codex') return { kind, rows, contents }
  const data = backups.length === 1 ? backups[0]! : {
    type: backups.find(b => b.type)?.type,
    version: Math.max(...backups.map(b => b.version || 0)),
    exported_at: new Date().toISOString(), proxies: backups.flatMap(b => b.proxies), accounts: backups.flatMap(b => b.accounts)
  }
  return { kind: 'backup', rows, data }
}

export async function readAccountImportFile(file: File): Promise<string> {
  if (typeof file.text === 'function') return file.text()
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(String(reader.result ?? ''))
    reader.onerror = () => reject(reader.error)
    reader.readAsText(file)
  })
}
