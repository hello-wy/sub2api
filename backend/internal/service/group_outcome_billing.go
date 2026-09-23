package service

import "context"

// GroupOutcomeBillingRequestID resolves the same identity as the money path.
// It never changes billing's durable IDs or its idempotency rules.
func GroupOutcomeBillingRequestID(ctx context.Context, upstreamID string) string {
	return resolveUsageBillingRequestID(ctx, upstreamID)
}
