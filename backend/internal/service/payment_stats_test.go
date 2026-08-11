//go:build unit

package service

import (
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

func TestComputeBasicStatsGroupsAmountsByCurrency(t *testing.T) {
	t.Parallel()

	todayStart := time.Date(2026, time.July, 25, 0, 0, 0, 0, time.UTC)
	yesterday := todayStart.Add(-time.Hour)
	today := todayStart.Add(time.Hour)
	orders := []*dbent.PaymentOrder{
		paymentStatsTestOrder(1, "alice@example.com", "CNY", 10, &today),
		paymentStatsTestOrder(2, "bob@example.com", "USD", 10, &today),
		paymentStatsTestOrder(1, "alice@example.com", "CNY", 5, &yesterday),
	}

	stats := &DashboardStats{}
	computeBasicStats(stats, orders, todayStart)

	require.Equal(t, CurrencyAmounts{"CNY": 15, "USD": 10}, stats.TotalAmount)
	require.Equal(t, CurrencyAmounts{"CNY": 10, "USD": 10}, stats.TodayAmount)
	require.Equal(t, CurrencyAmounts{"CNY": 7.5, "USD": 10}, stats.AvgAmount)
	require.Equal(t, 3, stats.TotalCount)
	require.Equal(t, 2, stats.TodayCount)
}

func TestPaymentDashboardBreakdownsGroupAmountsAndRankingsByCurrency(t *testing.T) {
	t.Parallel()

	firstDay := time.Date(2026, time.July, 24, 12, 0, 0, 0, time.UTC)
	secondDay := firstDay.AddDate(0, 0, 1)
	orders := []*dbent.PaymentOrder{
		paymentStatsTestOrder(1, "alice@example.com", "CNY", 5.555, &firstDay),
		paymentStatsTestOrder(2, "bob@example.com", "CNY", 10, &firstDay),
		paymentStatsTestOrder(1, "alice@example.com", "USD", 20, &secondDay),
		paymentStatsTestOrder(2, "bob@example.com", "USD", 10, &secondDay),
	}
	orders[0].PaymentType = "stripe"
	orders[1].PaymentType = "stripe"
	orders[2].PaymentType = "stripe"
	orders[3].PaymentType = "alipay"

	daily := buildDailySeries(orders, firstDay.AddDate(0, 0, -1), 2)
	require.Equal(t, []DailyStats{
		{Date: "2026-07-24", Amount: CurrencyAmounts{"CNY": 15.56}, Count: 2},
		{Date: "2026-07-25", Amount: CurrencyAmounts{"USD": 30}, Count: 2},
	}, daily)

	methods := buildMethodDistribution(orders)
	require.Equal(t, []PaymentMethodStat{
		{Type: "alipay", Amount: CurrencyAmounts{"USD": 10}, Count: 1},
		{Type: "stripe", Amount: CurrencyAmounts{"CNY": 15.56, "USD": 20}, Count: 3},
	}, methods)

	users := buildTopUsers(orders)
	require.Equal(t, TopUsersByCurrency{
		"CNY": {
			{UserID: 2, Email: "bob@example.com", Amount: 10},
			{UserID: 1, Email: "alice@example.com", Amount: 5.56},
		},
		"USD": {
			{UserID: 1, Email: "alice@example.com", Amount: 20},
			{UserID: 2, Email: "bob@example.com", Amount: 10},
		},
	}, users)
}

func TestBuildSubscriptionPlanDistribution(t *testing.T) {
	t.Parallel()

	firstPlanID := int64(11)
	secondPlanID := int64(22)
	orders := []*dbent.PaymentOrder{
		{PaymentType: "balance", OrderType: payment.OrderTypeSubscription, PlanID: &firstPlanID},
		{OrderType: payment.OrderTypeSubscription, PlanID: &secondPlanID},
		{OrderType: payment.OrderTypeSubscription, PlanID: &firstPlanID},
		{OrderType: payment.OrderTypeSubscription},
		{OrderType: "balance", PlanID: &firstPlanID},
	}

	stats := buildSubscriptionPlanDistribution(orders, map[int64]string{firstPlanID: "专业版"})

	require.Equal(t, []SubscriptionPlanPurchaseStat{
		{PlanID: firstPlanID, PlanName: "专业版", Count: 2},
		{PlanID: secondPlanID, Count: 1},
	}, stats)
}

func TestBuildGroupRevenueEfficiencySeparatesExpectedAndActualSubscriptionUsage(t *testing.T) {
	t.Parallel()

	alphaGroupID := int64(11)
	betaGroupID := int64(22)
	usageOnlyGroupID := int64(33)
	orders := []*dbent.PaymentOrder{
		{OrderType: payment.OrderTypeSubscription, SubscriptionGroupID: &alphaGroupID, PayAmount: 100},
		{OrderType: payment.OrderTypeSubscription, SubscriptionGroupID: &alphaGroupID, PayAmount: 100},
		{OrderType: payment.OrderTypeSubscription, SubscriptionGroupID: &betaGroupID, PayAmount: 40},
		{OrderType: payment.OrderTypeBalance, SubscriptionGroupID: &alphaGroupID, PayAmount: 500},
	}

	expectedQuota := 155.0
	stats := buildGroupRevenueEfficiency(orders, map[int64]groupRevenueUsage{
		alphaGroupID:     {UserUsage: 2000, BaseUsage: 800},
		usageOnlyGroupID: {UserUsage: 180, BaseUsage: 90},
	}, map[int64]groupRevenueMetadata{
		alphaGroupID:     {Name: "Alpha", RateMultiplier: 2, ExpectedQuotaPerPurchase: &expectedQuota},
		betaGroupID:      {Name: "Beta"},
		usageOnlyGroupID: {Name: "Gamma"},
	})

	require.Len(t, stats, 3)
	require.Equal(t, alphaGroupID, stats[0].GroupID)
	require.Equal(t, "Alpha", stats[0].GroupName)
	require.Equal(t, 2.0, stats[0].RateMultiplier)
	require.Equal(t, 200.0, stats[0].Revenue)
	require.Equal(t, 2000.0, stats[0].UserUsage)
	require.Equal(t, 800.0, stats[0].BaseUsage)
	require.NotNil(t, stats[0].ExpectedQuota)
	require.InDelta(t, 310, *stats[0].ExpectedQuota, 1e-12)
	require.NotNil(t, stats[0].UnitRevenue)
	require.InDelta(t, 0.2, *stats[0].UnitRevenue, 1e-12, "unit revenue is calculated from user usage divided by the group multiplier")

	require.Equal(t, betaGroupID, stats[1].GroupID)
	require.Nil(t, stats[1].ExpectedQuota)
	require.Nil(t, stats[1].UnitRevenue)
	require.Equal(t, usageOnlyGroupID, stats[2].GroupID)
	require.Nil(t, stats[2].UnitRevenue)
}

func TestSubscriptionGroupIDsDeduplicatesSubscriptionOrderSnapshots(t *testing.T) {
	t.Parallel()

	firstGroupID := int64(11)
	secondGroupID := int64(22)
	ids := subscriptionGroupIDs([]*dbent.PaymentOrder{
		{OrderType: payment.OrderTypeSubscription, SubscriptionGroupID: &firstGroupID},
		{OrderType: payment.OrderTypeSubscription, SubscriptionGroupID: &firstGroupID},
		{OrderType: payment.OrderTypeSubscription, SubscriptionGroupID: &secondGroupID},
		{OrderType: payment.OrderTypeBalance, SubscriptionGroupID: &secondGroupID},
	})

	require.ElementsMatch(t, []int64{firstGroupID, secondGroupID}, ids)
}

func paymentStatsTestOrder(userID int64, email, currency string, amount float64, paidAt *time.Time) *dbent.PaymentOrder {
	return &dbent.PaymentOrder{
		UserID:           userID,
		UserEmail:        email,
		PayAmount:        amount,
		PaidAt:           paidAt,
		ProviderSnapshot: map[string]any{"currency": currency},
	}
}
