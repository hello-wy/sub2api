package service

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

func (s *RateLimitService) handleOpenAIOAuth401(ctx context.Context, account *Account, code, message string) (handled, blocked bool) {
	repo, ok := s.accountRepo.(OpenAIOAuthHealthRepository)
	if !ok || account == nil || account.Platform != PlatformOpenAI || account.Type != AccountTypeOAuth || account.IsCredentialShadow() {
		return false, false
	}
	expected := *account
	if token, _ := ctx.Value(openAIUpstreamAccessTokenContextKey{}).(string); token != "" {
		expected.Credentials = shallowCopyMap(account.Credentials)
		expected.Credentials["access_token"] = token
	}
	code = strings.ToLower(strings.TrimSpace(code))
	permanent := code == "token_invalidated" || code == "token_revoked" || code == "invalid_grant"
	reason := "OAuth 401: invalid or expired credentials; refresh required"
	if permanent {
		reason = "OAuth 401: credentials revoked or refresh token unavailable; reauthorization required"
	}
	minutes := 10
	if s.cfg != nil && s.cfg.RateLimit.OAuth401CooldownMinutes > 0 {
		minutes = s.cfg.RateLimit.OAuth401CooldownMinutes
	}
	until := time.Now().Add(time.Duration(minutes) * time.Minute)
	stateCtx, cancel := openAIAccountStateContext(ctx)
	defer cancel()
	changed, err := repo.ApplyOpenAIOAuthHealth(stateCtx, &expected, OpenAIOAuthHealthMutation{Reason: reason, Permanent: permanent, AuthCooldownUntil: &until})
	if err != nil {
		slog.Warn("openai_401_shared_state_failed", "account_id", account.ID, "error", err)
		return true, true
	}
	if s.tokenCacheInvalidator != nil {
		if err := s.tokenCacheInvalidator.InvalidateToken(stateCtx, &expected); err != nil {
			slog.Warn("openai_401_invalidate_cache_failed", "account_id", account.ID, "error", err)
		}
	}
	if len(changed) == 0 {
		// State CAS missed a newer generation, but an account-ID cache can
		// still hold the rejected token. Evict that cache without touching the
		// current credentials/status; versioned group keys target only old AT.
		slog.Debug("openai_401_stale_token_ignored", "account_id", account.ID)
		return true, false
	}
	for _, member := range changed {
		blockUntil := until
		if member.Status == StatusError {
			blockUntil = time.Time{}
		}
		s.notifyAccountSchedulingBlocked(member, blockUntil, "oauth_401")
	}
	return true, true
}

// recordUpstream429Observation stores only display metadata. Scheduling and
// account health never change in response to an upstream HTTP 429.
func (s *RateLimitService) recordUpstream429Observation(ctx context.Context, account *Account, headers http.Header, body []byte) {
	if s == nil || account == nil {
		return
	}
	slog.Info("upstream_429_observed", "account_id", account.ID, "platform", account.Platform)
	if s.accountRepo == nil {
		return
	}
	now := time.Now()
	var resetAt *time.Time
	if account.Platform == PlatformOpenAI && !account.IsShadow() {
		_, resetAt = classifyOpenAIOAuth429(headers, body)
		persistOpenAI429PlanType(ctx, s.accountRepo, account, body)
		if snapshot := ParseCodexRateLimitHeaders(headers); snapshot != nil {
			if updates := buildCodexUsageExtraUpdates(snapshot, now); len(updates) > 0 {
				_ = s.accountRepo.UpdateExtra(ctx, account.ID, updates)
			}
		}
	} else if account.Platform == PlatformAnthropic {
		if result := calculateAnthropic429ResetTime(headers); result != nil {
			resetAt = &result.resetAt
		}
	} else if account.Platform == PlatformGemini || account.Platform == PlatformAntigravity {
		if unix := ParseGeminiRateLimitResetTime(body); unix != nil {
			value := time.Unix(*unix, 0)
			resetAt = &value
		}
	}
	if retry := parseRetryAfterResetTime(headers, now); retry != nil && retry.After(now) && (resetAt == nil || retry.After(*resetAt)) {
		resetAt = retry
	}
	observation := map[string]any{"status_code": 429, "observed_at": now.UTC().Format(time.RFC3339), "ignored": true}
	if resetAt != nil && resetAt.After(now) {
		observation["reset_at"] = resetAt.UTC().Format(time.RFC3339)
		// Retained for existing status displays. Scheduler readers explicitly
		// ignore upstream rate-limit metadata, including old database rows.
		if err := s.accountRepo.SetRateLimited(ctx, account.ID, *resetAt); err != nil {
			slog.Warn("upstream_429_observation_failed", "account_id", account.ID, "error", err)
		}
	}
	if err := s.accountRepo.UpdateExtra(ctx, account.ID, map[string]any{"upstream_429_observation": observation}); err != nil {
		slog.Warn("upstream_429_observation_failed", "account_id", account.ID, "error", err)
	}
}

func (s *RateLimitService) applyOpenAIOAuth429(ctx context.Context, account *Account, headers http.Header, body []byte) {
	if !Context429Enforcement(ctx) {
		s.recordUpstream429Observation(ctx, account, headers, body)
		return
	}
	if account == nil || account.IsShadow() {
		return
	}
	persistOpenAI429PlanType(ctx, s.accountRepo, account, body)
	s.persistOpenAICodexSnapshot(ctx, account, headers)
	_, resetAt := classifyOpenAIOAuth429(headers, body)
	now := time.Now()
	until := now.Add(openAIOAuth429FallbackCooldown)
	if cooldown, enabled := s.get429FallbackCooldown(ctx, account); enabled && cooldown > openAIOAuth429FallbackCooldown {
		until = now.Add(cooldown)
	}
	if resetAt != nil && resetAt.After(until) {
		until = *resetAt
	}
	stateCtx, cancel := openAIAccountStateContext(ctx)
	defer cancel()
	if repo, ok := s.accountRepo.(OpenAIOAuthHealthRepository); ok {
		members, err := repo.ApplyOpenAIOAuthHealth(stateCtx, account, OpenAIOAuthHealthMutation{RateLimitUntil: &until, Reason: "429"})
		if err != nil {
			slog.Warn("openai_429_shared_state_failed", "account_id", account.ID, "error", err)
			s.notifyAccountSchedulingBlocked(account, until, "429")
			return
		}
		for _, member := range members {
			if member.RateLimitResetAt != nil {
				s.notifyAccountSchedulingBlocked(member, *member.RateLimitResetAt, "429")
			}
		}
		return
	}
	s.notifyAccountSchedulingBlocked(account, until, "429")
	if s.accountRepo != nil {
		if err := s.accountRepo.SetRateLimited(stateCtx, account.ID, until); err != nil {
			slog.Warn("rate_limit_set_failed", "account_id", account.ID, "error", err)
		}
	}
}
