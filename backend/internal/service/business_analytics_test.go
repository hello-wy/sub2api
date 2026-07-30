package service

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestBuildDailyAnalyticsUsesCNYProfitBridge(t *testing.T) {
	items := map[string]*businessDailyAccumulator{
		"2026-07-30": {
			date:               "2026-07-30",
			usageCredits:       10,
			apiKeyUsageCostUSD: 0.05,
			apiKeyUsageCostCNY: 0.36,
			welfareGrantedUSD:  1,
			accountCostCNY:     0.2,
		},
	}

	result := buildDailyAnalytics(items, BusinessAnalyticsSettings{
		BalanceCreditsPerCNY: 10,
	})

	require.Len(t, result, 1)
	require.InDelta(t, 1, result[0].UsageRevenueCNY, 1e-9)
	require.InDelta(t, 0.36, result[0].APIKeyUsageCostCNY, 1e-9)
	require.InDelta(t, 0.1, result[0].WelfareCostCNY, 1e-9)
	require.InDelta(t, 0.34, result[0].OperatingProfitCNY, 1e-9)
	require.True(t, result[0].ProfitComplete)
}

func TestBuildDailyAnalyticsMarksMissingAPIKeyRateAsIncomplete(t *testing.T) {
	items := map[string]*businessDailyAccumulator{
		"2026-07-30": {
			date:              "2026-07-30",
			usageCredits:      10,
			unpricedAPIKeyUSD: 0.25,
		},
	}

	result := buildDailyAnalytics(items, BusinessAnalyticsSettings{BalanceCreditsPerCNY: 10})

	require.Len(t, result, 1)
	require.False(t, result[0].ProfitComplete)
	require.InDelta(t, 0.25, result[0].UnpricedAPIKeyUSD, 1e-9)
}

func TestAddAccruedCostByDaySplitsCostAcrossLocalCalendarDays(t *testing.T) {
	location := time.FixedZone("CST", 8*60*60)
	reportStart := time.Date(2026, 7, 30, 0, 0, 0, 0, location)
	reportEnd := reportStart.AddDate(0, 0, 2)
	items := make(map[string]*businessDailyAccumulator)

	addAccruedCostByDay(items, reportStart, reportEnd, reportStart, reportEnd, 20)

	require.InDelta(t, 10, items["2026-07-30"].accountCostCNY, 1e-9)
	require.InDelta(t, 10, items["2026-07-31"].accountCostCNY, 1e-9)
}

func TestCreditsToCNYFallsBackToDefaultRechargeMultiplier(t *testing.T) {
	require.InDelta(t, 1, creditsToCNY(10, 0), 1e-9)
}

func TestBuildProfitSummaryCalculatesCumulativeOperatingProfit(t *testing.T) {
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)

	result := buildProfitSummary(start, end, 1000, 24, 0, 30, 16, BusinessAnalyticsSettings{BalanceCreditsPerCNY: 10})

	require.Equal(t, start, result.StartAt)
	require.Equal(t, end, result.EndAt)
	require.InDelta(t, 100, result.UsageRevenueCNY, 1e-9)
	require.InDelta(t, 3, result.WelfareCostCNY, 1e-9)
	require.InDelta(t, 43, result.TotalCostCNY, 1e-9)
	require.InDelta(t, 57, result.OperatingProfitCNY, 1e-9)
	require.InDelta(t, 57, result.OperatingMargin, 1e-9)
	require.True(t, result.ProfitComplete)
}

func TestBuildProfitSummaryMarksUnpricedUsageIncomplete(t *testing.T) {
	result := buildProfitSummary(time.Time{}, time.Now(), 10, 0, 0.25, 0, 0, BusinessAnalyticsSettings{BalanceCreditsPerCNY: 10})

	require.False(t, result.ProfitComplete)
	require.InDelta(t, 0.25, result.UnpricedAPIKeyUSD, 1e-9)
}

func TestLoadGroupUsageDoesNotApplyAccountMultiplierToAPIKeyCost(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	start := time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 0, 1)
	mock.ExpectQuery(`(?s)CASE WHEN a\.type = 'apikey'\s+THEN COALESCE\(ul\.account_stats_cost, ul\.total_cost\)\s+ELSE 0 END.*COALESCE\(ul\.account_stats_cost, ul\.total_cost\) / cost_rate\.credits_per_cny.*JOIN users u ON u\.id = ul\.user_id.*LEFT JOIN accounts a ON a\.id = ul\.account_id.*LEFT JOIN business_api_key_cost_rates cost_rate ON cost_rate\.account_id = ul\.account_id.*u\.role <> 'admin'`).
		WithArgs(start, end, "UTC").
		WillReturnRows(sqlmock.NewRows([]string{
			"group_id", "group_name", "day", "base_credits", "usage_credits", "api_key_cost_usd", "api_key_cost_cny", "unpriced_api_key_usd", "capacity_usage", "account_count",
		}).AddRow(7, "standard", start, 1.0, 10.0, 0.25, 0.025, 0.05, 0.9, 2))

	groups, daily, err := NewBusinessAnalyticsService(db).loadGroupUsage(context.Background(), start, end)
	require.NoError(t, err)
	require.InDelta(t, 0.25, groups[7].apiKeyUsageCost, 1e-9)
	require.InDelta(t, 0.025, groups[7].apiKeyCostCNY, 1e-9)
	require.InDelta(t, 0.05, groups[7].unpricedAPIKey, 1e-9)
	require.Equal(t, []float64{0.9}, groups[7].dailyCapacityUsage)
	require.InDelta(t, 0.25, daily["2026-07-30"].apiKeyUsageCostUSD, 1e-9)
	require.InDelta(t, 0.025, daily["2026-07-30"].apiKeyUsageCostCNY, 1e-9)
	require.InDelta(t, 0.05, daily["2026-07-30"].unpricedAPIKeyUSD, 1e-9)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateAPIKeyCostRateUpsertsCurrentAccountRate(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	accountID := int64(42)
	createdAt := time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT type FROM accounts WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`)).
		WithArgs(accountID).
		WillReturnRows(sqlmock.NewRows([]string{"type"}).AddRow("apikey"))
	mock.ExpectQuery(`(?s)INSERT INTO business_api_key_cost_rates.*ON CONFLICT \(account_id\) DO UPDATE.*RETURNING id, account_id, credits_per_cny, notes, created_at`).
		WithArgs(accountID, 10.0, "渠道 A").
		WillReturnRows(sqlmock.NewRows([]string{"id", "account_id", "credits_per_cny", "notes", "created_at"}).
			AddRow(9, accountID, 10.0, "渠道 A", createdAt))
	mock.ExpectCommit()

	item, err := NewBusinessAnalyticsService(db).CreateAPIKeyCostRate(context.Background(), BusinessAPIKeyCostRateInput{
		AccountID:     accountID,
		CreditsPerCNY: 10,
		Notes:         "渠道 A",
	})

	require.NoError(t, err)
	require.Equal(t, int64(9), item.ID)
	require.InDelta(t, 10, item.CreditsPerCNY, 1e-9)
	require.Equal(t, createdAt, item.CreatedAt)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateAPIKeyCostRateRejectsOAuthAccount(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	accountID := int64(77)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT type FROM accounts WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`)).
		WithArgs(accountID).
		WillReturnRows(sqlmock.NewRows([]string{"type"}).AddRow("oauth"))
	mock.ExpectRollback()

	_, err = NewBusinessAnalyticsService(db).CreateAPIKeyCostRate(context.Background(), BusinessAPIKeyCostRateInput{
		AccountID:     accountID,
		CreditsPerCNY: 1,
	})

	require.EqualError(t, err, "cost rate can only be configured for API key accounts")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateAccountCostRejectsAPIKeyAccount(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	accountID := int64(42)
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT type FROM accounts WHERE id = $1 AND deleted_at IS NULL`)).
		WithArgs(accountID).
		WillReturnRows(sqlmock.NewRows([]string{"type"}).AddRow("apikey"))

	_, err = NewBusinessAnalyticsService(db).CreateAccountCost(context.Background(), BusinessAccountCostInput{
		AccountID: &accountID,
		Amount:    100,
		Currency:  "CNY",
		FXRate:    1,
		StartsAt:  start,
		EndsAt:    start.AddDate(0, 1, 0),
	})
	require.EqualError(t, err, "API key account cost is calculated automatically from usage and cannot be entered again")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestLoadLiabilitiesExcludesAdminUsers(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT COALESCE(SUM(balance + frozen_balance), 0) FROM users WHERE deleted_at IS NULL AND role <> 'admin'`)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(20.0))
	mock.ExpectQuery(`(?s)FROM user_subscriptions us\s+JOIN users u ON u\.id = us\.user_id\s+JOIN groups g ON g\.id = us\.group_id\s+WHERE .*u\.role <> 'admin'`).
		WillReturnRows(sqlmock.NewRows([]string{
			"expires_at", "total_usage_usd", "subscription_quota_reset_mode", "subscription_total_limit_usd", "daily_limit_usd", "weekly_limit_usd", "monthly_limit_usd",
		}))

	result, err := NewBusinessAnalyticsService(db).loadLiabilities(
		context.Background(),
		2,
		10,
		BusinessAnalyticsSettings{BalanceCreditsPerCNY: 10},
	)

	require.NoError(t, err)
	require.InDelta(t, 20, result.BalanceCreditsUSD, 1e-9)
	require.InDelta(t, 2, result.BalanceFaceValueCNY, 1e-9)
	require.InDelta(t, 4, result.BalanceEstimatedCostCNY, 1e-9)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCaptureCapacitySnapshotExcludesAdminUsage(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectExec(`(?s)INSERT INTO business_account_capacity_snapshots.*JOIN users u ON u\.id = ul\.user_id.*u\.role <> 'admin'`).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	capturedAt, err := NewBusinessAnalyticsService(db).CaptureCapacitySnapshot(context.Background())

	require.NoError(t, err)
	require.NotNil(t, capturedAt)
	require.NoError(t, mock.ExpectationsWereMet())
}
