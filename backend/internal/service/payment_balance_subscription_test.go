//go:build unit

package service

import (
	"math"
	"testing"

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
