import type { ModelPlazaGroup, PlazaModel } from '@/api/modelPlaza'
import type { UserPricingInterval } from '@/api/channels'
import type { UserMonitorView } from '@/api/channelMonitor'
import type { HealthState, MonitorMatrixResponse } from '@/api/channelMonitorV2'
import { resolveIntervalPrices } from '@/utils/pricing'

export type PlazaPriceMode = 'paid' | 'official'
export interface PlazaPriceRow {
  label: string
  input: number | null
  output: number | null
  cacheRead: number | null
  cacheWrite: number | null
  cacheWrite1h: number | null
  request: number | null
}
export function plazaRate(group: ModelPlazaGroup, model?: PlazaModel): number {
  return model?.pricing?.billing_mode === 'image' && group.image_rate_independent
    ? group.image_rate_multiplier ?? 1
    : group.user_rate_multiplier ?? group.rate_multiplier
}
export function plazaTierLabel(iv: UserPricingInterval): string {
  const count = (n: number) => n >= 1e6 ? n / 1e6 + 'M' : n >= 1e3 ? n / 1e3 + 'K' : String(n)
  return iv.tier_label || (iv.max_tokens == null ? '>' + count(iv.min_tokens) : '≤' + count(iv.max_tokens))
}
/** Same billing rules as the existing pricing table, in unformatted USD display units. */
export function plazaPrices(model: PlazaModel, group: ModelPlazaGroup, mode: PlazaPriceMode, periodMultiplier = 1): PlazaPriceRow[] {
  const source = mode === 'official' ? model.official_pricing : model.pricing
  const request = mode === 'paid' && model.pricing?.billing_mode !== 'token' && !!model.pricing
  const rate = mode === 'official' ? 1 : plazaRate(group, model) * periodMultiplier
  const intervals = [...(source?.intervals ?? [])].sort((a, b) => a.min_tokens - b.min_tokens)
  const usable = request ? intervals.filter(iv => iv.per_request_price != null) : intervals
  const rows = usable.length ? usable.map(iv => resolveIntervalPrices(iv, source!)) : [source]
  const scale = (value: number | null | undefined, unit = 1e6) => value == null ? null : value * rate * unit
  return rows.map((row, index) => ({
    label: usable[index] ? plazaTierLabel(usable[index]) : '',
    input: request ? null : scale(row?.input_price),
    output: request ? null : scale(row?.output_price),
    cacheRead: request ? null : scale(row?.cache_read_price),
    cacheWrite: request ? null : scale(row?.cache_write_price),
    cacheWrite1h: request ? null : scale(row?.cache_write_1h_price),
    request: request && row && 'per_request_price' in row ? scale(row.per_request_price, 1) : null,
  }))
}

export interface PlazaStatus {
  health: HealthState
  throughput: number | null
  latency: number | null
  availability: number | null
  updatedAt: string | null
  window: '24h' | '7d'
  history: Array<{ health: HealthState; at: string }>
}
export function unknownPlazaStatus(window: '24h' | '7d' = '24h'): PlazaStatus {
  return { health: 'unknown', throughput: null, latency: null, availability: null, updatedAt: null, window, history: [] }
}
const identity = (s: string) => s.trim().normalize('NFKC').toLowerCase()
const probeHealth = (status: string): HealthState => ({ operational: 'healthy', degraded: 'warning', failed: 'critical', error: 'critical' } as Record<string, HealthState>)[status] ?? 'unknown'

/** Exact group + platform + model only: never borrow another group's health. */
export function plazaV2Status(model: PlazaModel, group: ModelPlazaGroup, matrix: MonitorMatrixResponse | null): PlazaStatus {
  const candidates = matrix?.items.filter(row => row.group_id === group.id
    && identity(row.platform) === identity(model.platform)
    && identity(row.model ?? '') === identity(model.name)) ?? []
  if (candidates.length !== 1) return unknownPlazaStatus()
  const row = candidates[0]
  return {
    health: row.health.overall,
    throughput: row.metrics.request_count > 0 ? row.metrics.tpm / 60 : null,
    latency: row.metrics.duration.p50_ms ?? row.metrics.duration.avg_ms,
    availability: row.metrics.request_count > 0 ? Math.max(0, Math.min(100, (1 - row.metrics.error_rate) * 100)) : null,
    updatedAt: matrix!.coverage.data_through,
    window: '24h',
    history: row.buckets.map(b => ({ health: b.health.overall, at: b.bucket_start })),
  }
}
/** V1 has no group id/channel identity in plaza: accept only an unambiguous named-group probe. */
export function plazaV1Status(model: PlazaModel, group: ModelPlazaGroup, monitors: UserMonitorView[]): PlazaStatus {
  const candidates = monitors.filter(row => identity(row.provider) === identity(model.platform)
    && identity(row.group_name) === identity(group.name)
    && identity(row.primary_model) === identity(model.name))
  if (candidates.length !== 1) return unknownPlazaStatus('7d')
  const row = candidates[0]
  const history = [...row.timeline].sort((a, b) => a.checked_at.localeCompare(b.checked_at))
  return {
    health: probeHealth(row.primary_status),
    throughput: row.primary_throughput_tps,
    latency: row.primary_latency_ms,
    availability: history.length ? row.availability_7d : null,
    updatedAt: history.at(-1)?.checked_at ?? null,
    window: '7d',
    history: history.map(b => ({ health: probeHealth(b.status), at: b.checked_at })),
  }
}

/** Compress dense monitor timelines without dropping an outage between sampled points. */
export function plazaHistory(history: PlazaStatus['history'], limit = 24) {
  if (history.length <= limit) return history.map(point => ({ ...point, endAt:point.at }))
  const severity: Record<HealthState, number> = { unknown:0, healthy:1, warning:2, critical:3 }
  return Array.from({ length:limit }, (_, index) => {
    const points = history.slice(Math.floor(index * history.length / limit), Math.floor((index + 1) * history.length / limit))
    return {
      at:points[0].at,
      endAt:points[points.length - 1].at,
      health:points.reduce((worst, point) => severity[point.health] > severity[worst] ? point.health : worst, 'unknown' as HealthState),
    }
  })
}
