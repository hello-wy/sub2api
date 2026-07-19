package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestValidateBillableModelPricingRejectsUnknownModel(t *testing.T) {
	billing := NewBillingService(&config.Config{}, nil)
	err := validateBillableModelPricing(context.Background(), billing, nil, nil, "unpriced-attacker-model")
	require.ErrorIs(t, err, ErrModelPricingUnavailable)
}

func TestValidateBillableModelPricingAcceptsFallbackModel(t *testing.T) {
	billing := NewBillingService(&config.Config{}, nil)
	require.NoError(t, validateBillableModelPricing(context.Background(), billing, nil, nil, "claude-sonnet-4"))
}

func TestValidateBillableModelCandidatesAcceptsMappedPricingModel(t *testing.T) {
	billing := NewBillingService(&config.Config{}, nil)
	require.NoError(t, validateBillableModelCandidates(
		context.Background(),
		billing,
		nil,
		nil,
		"custom-unpriced-alias",
		"claude-sonnet-4",
	))
}

func TestGatewayCalculateRecordUsageCostFailsClosed(t *testing.T) {
	billing := NewBillingService(&config.Config{}, nil)
	svc := &GatewayService{billingService: billing}

	cost, err := svc.calculateRecordUsageCost(
		context.Background(),
		&ForwardResult{Model: "unpriced-attacker-model", Usage: ClaudeUsage{InputTokens: 100, OutputTokens: 50}},
		&APIKey{},
		"unpriced-attacker-model",
		1,
		1,
		&recordUsageOpts{},
	)
	require.Nil(t, cost)
	require.True(t, errors.Is(err, ErrModelPricingUnavailable), "unexpected error: %v", err)
}
