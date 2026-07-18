//go:build unit

package service

import (
	"math"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/stretchr/testify/require"
)

func TestBalanceSubscriptionPlanPriceDoesNotApplyPlanDiscount(t *testing.T) {
	originalPrice := 100.0
	lowerOriginalPrice := 10.0
	notANumber := math.NaN()

	tests := []struct {
		name string
		plan *dbent.SubscriptionPlan
		want float64
	}{
		{name: "uses original price", plan: &dbent.SubscriptionPlan{Price: 20, OriginalPrice: &originalPrice}, want: 100},
		{name: "falls back to sale price", plan: &dbent.SubscriptionPlan{Price: 20}, want: 20},
		{name: "never charges below sale price", plan: &dbent.SubscriptionPlan{Price: 20, OriginalPrice: &lowerOriginalPrice}, want: 20},
		{name: "ignores invalid original price", plan: &dbent.SubscriptionPlan{Price: 20, OriginalPrice: &notANumber}, want: 20},
		{name: "handles missing plan", plan: nil, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, balanceSubscriptionPlanPrice(tt.plan))
		})
	}
}
