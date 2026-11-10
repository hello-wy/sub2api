import type { UsageLog } from '@/types'

type UsageThroughputRow = Pick<UsageLog, 'output_tokens' | 'duration_ms'>

/** Calculate output throughput over the total duration; keep in sync with UsageLog.TokensPerSecond.
 * Derive from timing fields so older servers cannot supply a rate that excludes the initial wait.
 */
export function getUsageTokensPerSecond(row: UsageThroughputRow): number | null {
  const { output_tokens: tokens, duration_ms: duration } = row
  if (!Number.isFinite(tokens) || tokens <= 0 || duration == null
    || !Number.isFinite(duration) || duration <= 0) return null
  const rate = tokens / (duration / 1000)
  return Number.isFinite(rate) ? rate : null
}

/** Match the compact usage display: one decimal below 100 t/s, integers above it. */
export function formatUsageTokensPerSecond(row: UsageThroughputRow): string | null {
  const tps = getUsageTokensPerSecond(row)
  if (tps == null || !Number.isFinite(tps) || tps <= 0) return null
  return String(tps >= 100 ? Math.round(tps) : tps.toFixed(1)) + ' t/s'
}
