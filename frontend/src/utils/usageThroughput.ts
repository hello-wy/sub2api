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
