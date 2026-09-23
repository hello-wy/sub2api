interface Upstream429Source {
  rate_limit_429_enabled?: boolean
  rate_limited_at?: string | null
  rate_limit_reset_at?: string | null
  temp_unschedulable_reason?: string | null
  upstream_429_observation?: unknown
  extra?: Record<string, unknown>
}

export function upstream429Enforced(account: Upstream429Source): boolean {
  return account.rate_limit_429_enabled !== false
}

export function upstream429IsCoolingDown(account: Upstream429Source, now = Date.now()): boolean {
  return upstream429Enforced(account) && !!account.rate_limit_reset_at && Date.parse(account.rate_limit_reset_at) > now
}

export function activeTemporaryCooldown(account: Upstream429Source & { temp_unschedulable_until?: string | null }): string | null | undefined {
  return !upstream429Enforced(account) && isUpstream429Reason(account.temp_unschedulable_reason) ? null : account.temp_unschedulable_until
}

export function isUpstream429Reason(reason?: string | null): boolean {
  if (!reason) return false
  if (/^\s*(manual|admin|oauth\s+401|authentication|token refresh|overload|model_mismatch)|"source"\s*:\s*"(manual|admin|oauth_401|overload|model_mismatch)"|"status_code"\s*:\s*"?(401|403|5\d\d)(\D|$)/i.test(reason)) return false
  try {
    const value = JSON.parse(reason) as { status_code?: number; upstream_status?: number }
    if (value?.status_code === 429 || value?.upstream_status === 429) return true
  } catch { /* Legacy cooldown reasons were plain text. */ }
  return /^\s*(429\b|http\s+429\b|upstream[^\r\n]*429\b|rate[_ -]?limit|quota[_ -]?auto[_ -]?pause)|"source"\s*:\s*"(account_scheduling_threshold|quota_auto_pause|upstream_429)"|^openai_codex_spark_rate_limit$|legacy grok rate limited/i.test(reason)
}

export function hasUpstream429Observation(account: Upstream429Source): boolean {
  return !!account.rate_limited_at || !!account.rate_limit_reset_at || !!account.upstream_429_observation || !!account.extra?.upstream_429_observation || isUpstream429Reason(account.temp_unschedulable_reason)
}
