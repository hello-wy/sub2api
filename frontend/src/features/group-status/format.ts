import type { GroupServiceState } from '@/api/groupStatus'

export const serviceStates: GroupServiceState[] = ['operational', 'degraded', 'unavailable', 'unknown']

export function formatAvailability(value: number | null | undefined): string {
  if (value == null || !Number.isFinite(value)) return '—'
  return `${(Math.min(1, Math.max(0, value)) * 100).toFixed(2)}%`
}

export function formatGenerationSpeed(value: number | null | undefined): string {
  return value == null || !Number.isFinite(value) || value < 0 ? '—' : `${value.toFixed(1)} t/s`
}

export function formatGroupLatency(value: number | null | undefined): string {
  if (value == null || !Number.isFinite(value) || value < 0) return '—'
  return value < 1000 ? `${Math.round(value)} ms` : `${(value / 1000).toFixed(2)} s`
}

export function formatGroupTime(value: string | null | undefined, locale?: string): string {
  if (!value) return '—'
  const date = new Date(value)
  if (!Number.isFinite(date.getTime())) return '—'
  return date.toLocaleString(locale, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })
}

export function serviceState(value: string | null | undefined): GroupServiceState {
  return serviceStates.includes(value as GroupServiceState) ? value as GroupServiceState : 'unknown'
}

export function stateColor(value: string): string {
  switch (serviceState(value)) {
    case 'operational': return 'bg-emerald-500'
    case 'degraded': return 'bg-amber-500'
    case 'unavailable': return 'bg-rose-500'
    default: return 'bg-gray-200 dark:bg-dark-600'
  }
}

export function stateBadge(value: string): string {
  switch (serviceState(value)) {
    case 'operational': return 'bg-emerald-500/10 text-emerald-700 ring-emerald-500/25 dark:text-emerald-400'
    case 'degraded': return 'bg-amber-500/10 text-amber-700 ring-amber-500/25 dark:text-amber-400'
    case 'unavailable': return 'bg-rose-500/10 text-rose-700 ring-rose-500/25 dark:text-rose-400'
    default: return 'bg-gray-100 text-gray-500 ring-gray-300/50 dark:bg-dark-700 dark:text-gray-400 dark:ring-dark-600'
  }
}
