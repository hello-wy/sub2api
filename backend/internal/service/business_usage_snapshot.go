package service

import "time"

// NewBusinessUsageSnapshot deliberately omits personal request contents and
// survives usage-log retention. The charged transaction fills applied amounts.
func NewBusinessUsageSnapshot(u *UsageLog) map[string]any {
	at := u.CreatedAt
	if at.IsZero() {
		at = time.Now().UTC()
	}
	return map[string]any{
		"user_id": u.UserID, "api_key_id": u.APIKeyID, "account_id": u.AccountID,
		"request_id": u.RequestID, "model": u.Model, "created_at": at,
		"group_id": u.GroupID, "subscription_id": u.SubscriptionID, "billing_type": u.BillingType,
		"requested_model": u.RequestedModel, "upstream_model": u.UpstreamModel,
		"input_tokens": u.InputTokens, "output_tokens": u.OutputTokens,
		"cache_read_tokens": u.CacheReadTokens, "cache_creation_tokens": u.CacheCreationTokens,
		"cache_creation_5m_tokens": u.CacheCreation5mTokens, "cache_creation_1h_tokens": u.CacheCreation1hTokens,
		"image_size": u.ImageSize, "image_size_breakdown": u.ImageSizeBreakdown, "video_resolution": u.VideoResolution, "billing_mode": u.BillingMode,
		"image_count": u.ImageCount, "video_duration_seconds": u.VideoDurationSeconds,
		"total_cost": u.TotalCost, "actual_cost": u.ActualCost, "rate_multiplier": u.RateMultiplier,
		"account_stats_cost": u.AccountStatsCost, "account_rate_multiplier": u.AccountRateMultiplier,
		"service_tier": u.ServiceTier, "reasoning_effort": u.ReasoningEffort,
	}
}
