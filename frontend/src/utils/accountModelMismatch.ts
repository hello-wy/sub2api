import type { Account, AccountModelMismatch } from '@/types'

export function getAccountModelMismatch(account: Pick<Account, 'extra'> | null | undefined): AccountModelMismatch | null {
  const marker = rawAccountModelMismatch(account)
  return isModelDegradationMarker(marker) ? marker : null
}

export function isModelDegradationMarker(marker: AccountModelMismatch | null | undefined): boolean {
  return typeof marker?.expected_model === 'string' && typeof marker?.actual_model === 'string'
    && marker.expected_model.trim().toLowerCase() === 'gpt-6-astra'
    && marker.actual_model.trim().toLowerCase() === 'gpt-5.6-luna'
}

function rawAccountModelMismatch(account: Pick<Account, 'extra'> | null | undefined): AccountModelMismatch | null {
  const marker = account?.extra?.model_mismatch
  return marker && typeof marker === 'object' && !Array.isArray(marker) && Object.keys(marker).length > 0 ? marker : null
}

export function getLegacyAccountModelMismatch(account: Pick<Account, 'extra'> | null | undefined): AccountModelMismatch | null {
  const marker = rawAccountModelMismatch(account)
  return marker && !isModelDegradationMarker(marker) ? marker : null
}

export function hasLegacyAccountModelMismatch(account: Pick<Account, 'extra'> | null | undefined): boolean {
  return getLegacyAccountModelMismatch(account) !== null
}

export function canRestoreLegacyModelMismatch(account: Pick<Account, 'extra' | 'schedulable'> | null | undefined): boolean {
  const marker = getLegacyAccountModelMismatch(account)
  return !!account && !account.schedulable && marker !== null && marker.quarantined !== false
}

export function hasAccountModelMismatch(account: Pick<Account, 'extra'> | null | undefined): boolean {
  return getAccountModelMismatch(account) !== null
}

export function isAccountModelMismatchQuarantined(account: Pick<Account, 'extra'> | null | undefined): boolean {
  const marker = getAccountModelMismatch(account)
  return marker !== null && marker.quarantined !== false
}
