//go:build unit

package service

import (
	"context"
	"math"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/stretchr/testify/require"
)

func TestBalanceSubscriptionPlanPriceUsesRechargeMultiplierWithoutLoyaltyDiscount(t *testing.T) {
	originalPrice := 100.0

	tests := []struct {
		name       string
		plan       *dbent.SubscriptionPlan
		multiplier float64
		cnyRate    float64
		want       float64
	}{
		{name: "twenty yuan needs two hundred balance", plan: &dbent.SubscriptionPlan{Price: 20}, multiplier: 10, want: 200},
		{name: "fifty yuan needs five hundred balance", plan: &dbent.SubscriptionPlan{Price: 50}, multiplier: 10, want: 500},
		{name: "ignores display original price", plan: &dbent.SubscriptionPlan{Price: 20, OriginalPrice: &originalPrice}, multiplier: 10, want: 200},
		{name: "applies configured subscription CNY conversion first", plan: &dbent.SubscriptionPlan{Price: 10}, multiplier: 10, cnyRate: 7.15, want: 715},
		{name: "normalizes invalid multiplier", plan: &dbent.SubscriptionPlan{Price: 20}, multiplier: math.NaN(), want: 200},
		{name: "handles missing plan", plan: nil, multiplier: 10, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, balanceSubscriptionPlanPrice(tt.plan, tt.multiplier, tt.cnyRate))
		})
	}
}

func TestRecordBalanceSubscriptionPaymentCreatesNegativeHistory(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	at := time.Date(2026, 7, 18, 18, 0, 0, 0, time.UTC)
	user, err := client.User.Create().
		SetEmail("subscription-history@example.com").
		SetPasswordHash("hash").
		SetUsername("subscription-history-user").
		Save(ctx)
	require.NoError(t, err)

	require.NoError(t, recordBalanceSubscriptionPayment(ctx, client, user.ID, 200, "轻度包月", at))
	record, err := client.RedeemCode.Query().Only(ctx)
	require.NoError(t, err)
	require.Equal(t, AdjustmentTypeSubscriptionPay, record.Type)
	require.Equal(t, -200.0, record.Value)
	require.Equal(t, StatusUsed, record.Status)
	require.Equal(t, user.ID, *record.UsedBy)
	require.Equal(t, at, *record.UsedAt)
	require.Equal(t, "轻度包月", *record.Notes)
}
