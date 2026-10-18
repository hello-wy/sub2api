package service

import (
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestOperatingConsumption(t *testing.T) {
	d := decimal.RequireFromString
	for _, tc := range []struct{ name, p, q, baseline, previous, used, want string }{
		{"十个百分点", "100", "1000", "0", "0", "100", "10"},
		{"期初不重记", "100", "1000", "400", "0", "100", "10"},
		{"收入封顶", "100", "1000", "400", "500", "300", "10"},
		{"循环额度整期比例", "100", "3000", "0", "0", "10", "0.33333333"},
		{"极小扣款保留精度", "100", "1000", "0", "0", "0.0000001", "0.00000001"},
		{"无分母不除零", "100", "0", "0", "0", "10", "0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.True(t, OperatingConsumptionDelta(d(tc.p), d(tc.q), d(tc.baseline), d(tc.previous), d(tc.used)).Equal(d(tc.want)))
		})
	}
}
func TestOperatingAvailableResetsAndLifetime(t *testing.T) {
	now := time.Now()
	old := now.Add(-48 * time.Hour)
	limit := 100.0
	total := 1000.0
	sub := &UserSubscription{StartsAt: old, ExpiresAt: now.Add(30 * 24 * time.Hour), DailyWindowStart: &old, DailyUsageUSD: 80, TotalUsageUSD: 950}
	g := &Group{DailyLimitUSD: &limit, SubscriptionTotalLimitUSD: &total}
	require.Equal(t, "50", operatingAvailable(sub, g, now).String())
	g.SubscriptionQuotaResetMode = "until_subscription_expires"
	require.Equal(t, "50", operatingAvailable(sub, g, now).String())
}
func TestOperatingProfitIncludesGiftsOnce(t *testing.T) {
	day := BusinessDay{BalanceRevenue: decimal.NewFromInt(10), GiftCost: decimal.NewFromInt(10), ManualCost: decimal.NewFromInt(2)}
	day.finish()
	require.Equal(t, "-2", day.Profit.String())
	day.Missing = 1
	day.finish()
	require.Nil(t, day.Profit)
	require.Equal(t, "-2", day.KnownProfit.String())
}
