//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestQuoteUsageCostPropagatesTokenPricingError(t *testing.T) {
	const unknownModel = "unpriced-cache-model"
	gateway := &GatewayService{billingService: NewBillingService(&config.Config{}, nil)}
	input := &RecordUsageInput{
		Result: &ForwardResult{
			Model:                  unknownModel,
			Usage:                  ClaudeUsage{InputTokens: 100},
			CacheStorageTokenHours: 100,
		},
		APIKey: &APIKey{User: &User{}},
	}

	amount, err := gateway.QuoteUsageCost(context.Background(), input)

	require.ErrorIs(t, err, ErrModelPricingUnavailable)
	require.Zero(t, amount)
}
