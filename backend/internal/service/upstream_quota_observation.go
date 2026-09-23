package service

import (
	"context"
	"regexp"
	"strings"
	"sync/atomic"
	"time"
)

// Historical rate-limit metadata remains visible but does not control routing.
// Keep this expression compatible with PostgreSQL so persisted and cached
// accounts apply exactly the same recognition rules without parsing unsafe JSON.
const UpstreamQuotaObservationReasonPattern = `(^[[:space:]]*(429([^[:digit:]]|$)|http[[:space:]]+429([^[:digit:]]|$)|upstream[^[:cntrl:]]*429([^[:digit:]]|$)|rate[_ -]?limit|quota[_ -]?auto[_ -]?pause)|"status_code"[[:space:]]*:[[:space:]]*"?429([^[:digit:]]|$)|"source"[[:space:]]*:[[:space:]]*"(account_scheduling_threshold|quota_auto_pause|upstream_429)"|^openai_codex_spark_rate_limit$)`
const NonQuotaSchedulingReasonPattern = `(^[[:space:]]*(manual|admin|oauth[[:space:]]+401|authentication|token refresh|overload|model_mismatch)|"source"[[:space:]]*:[[:space:]]*"(manual|admin|oauth_401|overload|model_mismatch)"|"status_code"[[:space:]]*:[[:space:]]*"?(401|403|5[[:digit:]][[:digit:]])([^[:digit:]]|$))`

var upstreamQuotaObservationReason = regexp.MustCompile("(?i)" + UpstreamQuotaObservationReasonPattern)
var nonQuotaSchedulingReason = regexp.MustCompile("(?i)" + NonQuotaSchedulingReasonPattern)

func IsUpstreamQuotaObservationReason(reason string) bool {
	reason = strings.TrimSpace(reason)
	return !nonQuotaSchedulingReason.MatchString(reason) && upstreamQuotaObservationReason.MatchString(reason)
}

func (a *Account) IsTemporarilyUnschedulable() bool {
	return a.IsTemporarilyUnschedulableWithContext(context.Background())
}

func (a *Account) IsTemporarilyUnschedulableWithContext(ctx context.Context) bool {
	return a != nil && a.TempUnschedulableUntil != nil && time.Now().Before(*a.TempUnschedulableUntil) && (Context429Enforcement(ctx) || !IsUpstreamQuotaObservationReason(a.TempUnschedulableReason))
}

// Local billing quotas, administrator enable switches, and explicit request
// concurrency/RPM caps are independent of upstream usage observation policy.
func UpstreamQuotaObservationsAffectScheduling() bool {
	return Context429Enforcement(context.Background())
}

// The zero value preserves existing enforcement. A request snapshot overrides
// the process fallback so an administrative toggle cannot split one decision.
var upstream429EnforcementDisabled atomic.Bool

type upstream429EnforcementContextKey struct{}

func Context429Enforcement(ctx context.Context) bool {
	if ctx != nil {
		if enabled, ok := ctx.Value(upstream429EnforcementContextKey{}).(bool); ok {
			return enabled
		}
	}
	return !upstream429EnforcementDisabled.Load()
}

func With429Enforcement(ctx context.Context, enabled bool) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, upstream429EnforcementContextKey{}, enabled)
}

func Set429EnforcementEnabled(enabled bool) { upstream429EnforcementDisabled.Store(!enabled) }
