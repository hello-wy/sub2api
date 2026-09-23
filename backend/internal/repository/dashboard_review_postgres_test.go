//go:build unit

package repository

import (
	"os"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestDashboardReviewPostgresLogicalCountsWithRetiredRoot(t *testing.T) {
	r, c, ctx := ipChannelIntegrationRepo(t)
	a := createIPChannelFixture(t, r, c, ctx, "dashboard", 3)
	root, err := r.JoinAccountIPChannels(ctx, []int64{a[0].ID, a[1].ID, a[2].ID})
	require.NoError(t, err)
	usage := newUsageLogRepositoryWithSQL(c, r.sql)
	stats := &DashboardStats{}
	now := time.Now()
	require.NoError(t, usage.fillDashboardEntityStats(ctx, stats, now.Add(-time.Hour), now))
	require.EqualValues(t, 1, stats.TotalAccounts)
	require.EqualValues(t, 1, stats.NormalAccounts)
	require.EqualValues(t, 3, stats.TotalIPChannels)
	require.EqualValues(t, 3, stats.AvailableIPChannels)
	off := false
	require.NoError(t, r.PatchAccountIPChannel(ctx, root, root, service.AccountIPChannelPatch{Schedulable: &off}))
	_, err = r.sql.ExecContext(ctx, `UPDATE account_ip_channels SET disabled_at=$2 WHERE account_id=$1`, root, now.Add(-3*time.Minute))
	require.NoError(t, err)
	require.NoError(t, r.RemoveAccountIPChannel(ctx, root, root))
	_, err = c.Proxy.UpdateOneID(*a[1].ProxyID).SetExpiresAt(now.Add(-time.Hour)).Save(ctx)
	require.NoError(t, err)
	stats = &DashboardStats{}
	require.NoError(t, usage.fillDashboardEntityStats(ctx, stats, now.Add(-time.Hour), now))
	require.EqualValues(t, 1, stats.TotalAccounts)
	require.EqualValues(t, 1, stats.NormalAccounts)
	require.EqualValues(t, 2, stats.TotalIPChannels)
	require.EqualValues(t, 1, stats.AvailableIPChannels)
	_, err = r.SetLogicalAccountSchedulable(ctx, root, false)
	require.NoError(t, err)
	require.NoError(t, usage.fillDashboardEntityStats(ctx, stats, now.Add(-time.Hour), now))
	require.EqualValues(t, 1, stats.TotalAccounts)
	require.Zero(t, stats.NormalAccounts)
	require.Zero(t, stats.AvailableIPChannels)
}

func TestDashboardReviewPostgresDurationSamplesAndCoverage(t *testing.T) {
	r, c, ctx := ipChannelIntegrationRepo(t)
	previousZone, previousLocal := timezone.Name(), time.Local
	if previousZone == "Local" {
		previousZone = "UTC"
	}
	require.NoError(t, timezone.Init("Asia/Hong_Kong"))
	t.Cleanup(func() { _ = timezone.Init(previousZone); time.Local = previousLocal })
	_, err := r.sql.ExecContext(ctx, `CREATE TABLE channels(id BIGSERIAL PRIMARY KEY)`)
	require.NoError(t, err)
	for _, name := range []string{"034_usage_dashboard_aggregation_tables.sql", "101_add_account_stats_pricing.sql", "107_add_account_cost_to_dashboard_tables.sql", "255_dashboard_duration_samples.sql"} {
		migration, err := os.ReadFile("../../migrations/" + name)
		require.NoError(t, err)
		_, err = r.sql.ExecContext(ctx, string(migration))
		require.NoError(t, err)
	}
	a := createIPChannelFixture(t, r, c, ctx, "durations", 1)[0]
	u, err := c.User.Create().SetEmail("dashboard@example.invalid").SetPasswordHash("test").Save(ctx)
	require.NoError(t, err)
	k, err := c.APIKey.Create().SetUserID(u.ID).SetName("test").SetKey("dashboard-test").Save(ctx)
	require.NoError(t, err)
	start := time.Date(2026, 9, 16, 0, 0, 0, 0, timezone.Location())
	end := start.AddDate(0, 0, 1)
	for i, duration := range []*int{nil, new(int), func() *int { v := 1000; return &v }()} {
		_, err = c.UsageLog.Create().SetUserID(u.ID).SetAPIKeyID(k.ID).SetAccountID(a.ID).SetRequestID(string(rune('a' + i))).SetModel("test").SetNillableDurationMs(duration).SetCreatedAt(start.Add(time.Hour)).Save(ctx)
		require.NoError(t, err)
	}
	usage := newUsageLogRepositoryWithSQL(c, r.sql)
	stats := &DashboardStats{}
	require.NoError(t, usage.fillDashboardUsageStatsFromUsageLogs(ctx, stats, start, end, start, start.Add(time.Hour)))
	require.EqualValues(t, 3, stats.TotalRequests)
	require.EqualValues(t, 2, stats.DurationSamples)
	require.Equal(t, float64(500), stats.AverageDurationMs)
	require.Equal(t, "2026-09-16", stats.UsagePeriodStart)
	require.Equal(t, "2026-09-16", stats.UsagePeriodEnd)
	stats = &DashboardStats{}
	require.NoError(t, usage.fillDashboardUsageStatsFromUsageLogs(ctx, stats, start.AddDate(0, 0, -30), end, start, start.Add(time.Hour)))
	require.Equal(t, "2026-09-16", stats.UsagePeriodStart, "coverage must come from retained records, not the requested range")
	agg := newDashboardAggregationRepositoryWithSQL(r.sql)
	require.NoError(t, agg.AggregateRange(ctx, start, end))
	stats = &DashboardStats{}
	require.NoError(t, usage.fillDashboardUsageStatsAggregated(ctx, stats, start, start.Add(time.Hour)))
	require.EqualValues(t, 2, stats.DurationSamples)
	require.Equal(t, float64(500), stats.AverageDurationMs)
	// Existing aggregate rows have no valid timing count. Including their
	// durations would distort the measured average, even if request totals exist.
	_, err = r.sql.ExecContext(ctx, `INSERT INTO usage_dashboard_hourly(bucket_start,total_requests,total_duration_ms) VALUES($1,100,900000)`, start.Add(-time.Hour))
	require.NoError(t, err)
	require.NoError(t, agg.upsertDailyAggregates(ctx, start.AddDate(0, 0, -1), start))
	stats = &DashboardStats{}
	require.NoError(t, usage.fillDashboardUsageStatsAggregated(ctx, stats, start, start.Add(time.Hour)))
	require.EqualValues(t, 103, stats.TotalRequests)
	require.EqualValues(t, 2, stats.DurationSamples)
	require.Equal(t, float64(500), stats.AverageDurationMs)
	require.Equal(t, "2026-09-15", stats.UsagePeriodStart)
	require.Equal(t, "2026-09-16", stats.UsagePeriodEnd)
}
