package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSiteCreditsMigrationConvertsUserFacingAmounts(t *testing.T) {
	content, err := FS.ReadFile("239_convert_usd_balances_to_credits.sql")
	require.NoError(t, err)
	sql := strings.Join(strings.Fields(string(content)), " ")

	for _, table := range []string{
		"users",
		"user_subscriptions",
		"settings",
		"user_platform_quotas",
		"payment_orders",
		"redeem_codes",
		"promo_codes",
		"promo_code_usages",
		"daily_checkin_records",
		"welfare_records",
		"lottery_draws",
		"user_affiliate_ledger",
		"balance_transactions",
		"usage_logs",
		"batch_image_items",
		"billing_usage_entries",
		"usage_dashboard_hourly",
		"usage_dashboard_daily",
		"usage_group_daily_rollups",
		"batch_image_jobs",
		"groups",
		"user_group_rate_multipliers",
	} {
		require.Contains(t, sql, "UPDATE "+table)
	}
	require.Contains(t, sql, "INSERT INTO audit_logs")
	require.Contains(t, sql, "image_rate_multiplier = image_rate_multiplier / 10.0")
	require.Contains(t, sql, "video_rate_multiplier = video_rate_multiplier / 10.0")
	require.Contains(t, sql, "billable_unit_price = billable_unit_price / 10.0")
	require.NotContains(t, sql, "input_cost = input_cost / 10.0")
	require.NotContains(t, sql, "peak_rate_multiplier = peak_rate_multiplier / 10.0")
	require.Contains(t, sql, "frozen_balance = frozen_balance / 10.0")
	require.Contains(t, sql, "usage_5h = usage_5h / 10.0")
	require.Contains(t, sql, "billed_amount = billed_amount / 10.0")
	require.Contains(t, sql, "balance_notify_threshold = balance_notify_threshold / 10.0")
	require.Contains(t, sql, "default_platform_quotas")
	require.Contains(t, sql, "ON CONFLICT (key) DO UPDATE SET value = '1'")
	require.Contains(t, sql, "SITE_CREDITS_UNIT_MIGRATION")
	require.Contains(t, sql, "value::numeric / 10.0")
}
