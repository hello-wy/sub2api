/**
 * 请求延迟健康度分档（用于用量明细"延迟"列的纵向健康扫视）。
 *
 * 首 Token（TTFT）：10s 内正常，10-30s 偏慢，30-60s 缓慢，60s 及以上严重。
 * 总耗时：流式请求整体时长天然更长，阈值放宽为 1min / 3min / 5min。
 * TPS：按输出量动态分四档；输出阶段耗时预算 = 10/30/60s 余量 + Token 数 ÷ 20/10/5 t/s。
 */
export type LatencySeverity = 'good' | 'warn' | 'slow' | 'critical'

export const FIRST_TOKEN_THRESHOLDS_MS = {
  warn: 10_000,
  slow: 30_000,
  critical: 60_000,
} as const

export const DURATION_THRESHOLDS_MS = {
  warn: 60_000,
  slow: 180_000,
  critical: 300_000,
} as const

interface Thresholds {
  warn: number
  slow: number
  critical: number
}

const classify = (ms: number, thresholds: Thresholds): LatencySeverity => {
  if (ms >= thresholds.critical) return 'critical'
  if (ms >= thresholds.slow) return 'slow'
  if (ms >= thresholds.warn) return 'warn'
  return 'good'
}

export const firstTokenSeverity = (ms: number): LatencySeverity =>
  classify(ms, FIRST_TOKEN_THRESHOLDS_MS)

export const durationSeverity = (ms: number): LatencySeverity =>
  classify(ms, DURATION_THRESHOLDS_MS)

// Long-output reference rates. Short outputs receive a fixed time allowance,
// using the same 10/30/60s scale as first-token health (not measured TTFT).
export const TPS_REFERENCE_RATES = {
  warn: 20,
  slow: 10,
  critical: 5,
} as const

export const tpsSeverity = (tps: number, outputTokens: number): LatencySeverity => {
  if (!Number.isFinite(tps) || tps <= 0 || !Number.isFinite(outputTokens) || outputTokens <= 0) {
    return 'critical'
  }
  // N / (allowance + N / referenceRate): continuous thresholds that approach
  // the reference rates for long outputs, without abrupt token-count buckets.
  const threshold = (level: keyof typeof TPS_REFERENCE_RATES) =>
    outputTokens / (FIRST_TOKEN_THRESHOLDS_MS[level] / 1000 + outputTokens / TPS_REFERENCE_RATES[level])
  if (tps <= threshold('critical')) return 'critical'
  if (tps <= threshold('slow')) return 'slow'
  if (tps <= threshold('warn')) return 'warn'
  return 'good'
}

export const LATENCY_TEXT_CLASSES: Record<LatencySeverity, string> = {
  good: 'text-emerald-600 dark:text-emerald-400',
  warn: 'text-amber-600 dark:text-amber-400',
  slow: 'text-orange-600 dark:text-orange-400',
  critical: 'text-red-600 dark:text-red-400',
}

/** 无首字数据时的纯色色条（仅按总耗时档着色）。 */
export const LATENCY_BAR_CLASSES: Record<LatencySeverity, string> = {
  good: 'bg-emerald-500',
  warn: 'bg-amber-400',
  slow: 'bg-orange-500',
  critical: 'bg-red-500',
}

/** 渐变色条上端（首字档）；与中段和下端颜色组合，平滑过渡健康度档位。 */
export const LATENCY_BAR_FROM_CLASSES: Record<LatencySeverity, string> = {
  good: 'from-emerald-500',
  warn: 'from-amber-400',
  slow: 'from-orange-500',
  critical: 'from-red-500',
}

/** 渐变色条中段（总耗时档）。 */
export const LATENCY_BAR_VIA_CLASSES: Record<LatencySeverity, string> = {
  good: 'via-emerald-500',
  warn: 'via-amber-400',
  slow: 'via-orange-500',
  critical: 'via-red-500',
}

/** 渐变色条下端（TPS 档；无 TPS 时沿用总耗时档）。 */
export const LATENCY_BAR_TO_CLASSES: Record<LatencySeverity, string> = {
  good: 'to-emerald-500',
  warn: 'to-amber-400',
  slow: 'to-orange-500',
  critical: 'to-red-500',
}
