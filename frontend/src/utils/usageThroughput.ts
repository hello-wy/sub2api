import type { UsageLog } from '@/types'

type UsageThroughputRow = Pick<UsageLog, 'output_tokens' | 'duration_ms' | 'first_token_ms'>

/** Estimate post-first-token throughput; keep in sync with UsageLog.TokensPerSecond.
 * Derive from timing fields so older servers cannot supply the former total-duration rate.
 */
export function getUsageTokensPerSecond(row: UsageThroughputRow): number | null {
  const { output_tokens: tokens, duration_ms: duration, first_token_ms: firstToken } = row
  if (!Number.isFinite(tokens) || tokens <= 0 || duration == null || firstToken == null
    || !Number.isFinite(duration) || !Number.isFinite(firstToken) || firstToken < 0) return null
  const outputMs = duration - firstToken
  // Suppress unstable rates from buffered/terminal-only responses.
  if (outputMs < 100) return null
  const rate = tokens / (outputMs / 1000)
  return Number.isFinite(rate) ? rate : null
}

/** Match the compact usage display: one decimal below 100 t/s, integers above it. */
export function formatUsageTokensPerSecond(row: UsageThroughputRow): string | null {
  const tps = getUsageTokensPerSecond(row)
  if (tps == null || !Number.isFinite(tps) || tps <= 0) return null
  return String(tps >= 100 ? Math.round(tps) : tps.toFixed(1)) + ' t/s'
}
