package repository

import (
	"context"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func upstreamQuotaObservationSQL(reason string) string {
	return "(COALESCE(" + reason + ", '') ~* '" + service.UpstreamQuotaObservationReasonPattern + "' AND NOT (COALESCE(" + reason + ", '') ~* '" + service.NonQuotaSchedulingReasonPattern + "'))"
}

func effectiveTempUnschedulableSQL(ctx context.Context, until, reason, now string) string {
	if service.Context429Enforcement(ctx) {
		return "(" + until + " > " + now + ")"
	}
	return "(" + until + " > " + now + " AND NOT " + upstreamQuotaObservationSQL(reason) + ")"
}

func noEffectiveTempUnschedulableSQL(ctx context.Context, until, reason, now string) string {
	if service.Context429Enforcement(ctx) {
		return "(" + until + " IS NULL OR " + until + " <= " + now + ")"
	}
	return "(" + until + " IS NULL OR " + until + " <= " + now + " OR " + upstreamQuotaObservationSQL(reason) + ")"
}

func noEffectiveRateLimitSQL(ctx context.Context, until, now string) string {
	if !service.Context429Enforcement(ctx) {
		return "TRUE"
	}
	return "(" + until + " IS NULL OR " + until + " <= " + now + ")"
}

func effectiveRateLimitSQL(ctx context.Context, until, now string) string {
	if !service.Context429Enforcement(ctx) {
		return "FALSE"
	}
	return "(" + until + " > " + now + ")"
}
