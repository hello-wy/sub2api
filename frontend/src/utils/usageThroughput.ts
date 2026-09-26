import type { UsageLog } from '@/types'

type UsageThroughputRow = Pick<UsageLog, 'tps' | 'output_tokens' | 'duration_ms'>

/** Return the API value, or derive TPS for legacy API payloads. */
export function getUsageTokensPerSecond(row: UsageThroughputRow): number | null {
  if (row.tps != null && Number.isFinite(row.tps) && row.tps > 0) {
    return row.tps
  }
  if (row.output_tokens <= 0 || row.duration_ms == null || row.duration_ms <= 0) {
    return null
  }
  return row.output_tokens / (row.duration_ms / 1000)
}

/** Match the compact usage display: one decimal below 100 t/s, integers above it. */
export function formatUsageTokensPerSecond(row: UsageThroughputRow): string | null {
  const tps = getUsageTokensPerSecond(row)
  if (tps == null || !Number.isFinite(tps) || tps <= 0) return null
  return String(tps >= 100 ? Math.round(tps) : tps.toFixed(1)) + ' t/s'
}
