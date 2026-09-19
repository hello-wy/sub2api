//go:build integration

package repository

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	dbmigrations "github.com/Wei-Shaw/sub2api/migrations"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestSiteCreditsMigrationRatesRewardsAndRollback(t *testing.T) {
	before, err := os.ReadFile("testdata/site_credits_before.sql")
	require.NoError(t, err)
	after, err := os.ReadFile("testdata/site_credits_after.sql")
	require.NoError(t, err)
	migration, err := dbmigrations.FS.ReadFile("239_convert_usd_balances_to_credits.sql")
	require.NoError(t, err)
	for _, shape := range []string{"array", "object"} {
		t.Run(shape, func(t *testing.T) {
			tx := testTx(t)
			ctx := context.Background()
			_, err := tx.ExecContext(ctx, string(before))
			require.NoError(t, err)
			if shape == "object" {
				_, err = tx.ExecContext(ctx, `UPDATE settings SET value = jsonb_build_object('version', 'keep', 'prizes', value::jsonb)::text WHERE key = 'lottery_prize_pool'`)
				require.NoError(t, err)
			}
			_, err = tx.ExecContext(ctx, "SAVEPOINT unit_conversion")
			require.NoError(t, err)
			_, err = tx.ExecContext(ctx, string(migration))
			require.NoError(t, err)
			_, err = tx.ExecContext(ctx, string(after))
			require.NoError(t, err)
			// A failure after conversion must roll back both data and DDL.
			_, err = tx.ExecContext(ctx, "SELECT 1 / 0")
			require.Error(t, err)
			_, err = tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT unit_conversion")
			require.NoError(t, err)
			var balance float64
			require.NoError(t, tx.QueryRowContext(ctx, `SELECT balance FROM users WHERE email = 'migration239@example.test'`).Scan(&balance))
			require.Equal(t, 100.0, balance)
			// Retry after rollback converts exactly once.
			_, err = tx.ExecContext(ctx, string(migration))
			require.NoError(t, err)
			_, err = tx.ExecContext(ctx, string(after))
			require.NoError(t, err)
		})
	}
}

func TestSiteCreditsMigrationPreservesUnflushedPlatformQuota(t *testing.T) {
	ctx := context.Background()
	migration, err := dbmigrations.FS.ReadFile(siteCreditsMigration)
	require.NoError(t, err)
	start := time.Now().UTC().Truncate(time.Second)
	for _, tc := range []struct {
		name                      string
		dbUsage, redisUsage, want float64
		dbStart, redisStart       time.Time
		deleted                   bool
	}{
		{"unflushed usage", 80, 100, 10, start.Add(500 * time.Millisecond), start, false},
		{"DB ahead in same window", 120, 100, 12, start, start, false},
		{"stale Redis window", 80, 100, 8, start, start.Add(-24 * time.Hour), false},
		{"new Redis window", 80, 5, 0.5, start.Add(-24 * time.Hour), start, false},
		{"deleted quota", 80, 100, 80, start, start, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx := testTx(t)
			var uid int64
			require.NoError(t, tx.QueryRowContext(ctx, `INSERT INTO users (email,password_hash) VALUES ('quota239@example.test','test') RETURNING id`).Scan(&uid))
			_, err := tx.ExecContext(ctx, `INSERT INTO user_platform_quotas
(user_id,platform,daily_limit_usd,weekly_limit_usd,monthly_limit_usd,
daily_usage_usd,weekly_usage_usd,monthly_usage_usd,daily_window_start,weekly_window_start,monthly_window_start,deleted_at)
VALUES ($1,'openai',1000,2000,3000,$2,$2,$2,$3,$3,$3,CASE WHEN $4 THEN NOW() ELSE NULL END)`, uid, tc.dbUsage, tc.dbStart, tc.deleted)
			require.NoError(t, err)
			mr := miniredis.RunT(t)
			rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
			t.Cleanup(func() { _ = rdb.Close() })
			key := fmt.Sprintf("%s%d:openai", legacyPlatformQuotaPrefix, uid)
			fields := map[string]any{}
			for _, window := range []string{"daily", "weekly", "monthly"} {
				fields[window+"_usage"] = tc.redisUsage
				fields[window+"_window_start"] = strconv.FormatInt(tc.redisStart.Unix(), 10)
			}
			require.NoError(t, rdb.HSet(ctx, key, fields).Err())
			_, err = tx.ExecContext(ctx, "SAVEPOINT quota_handoff")
			require.NoError(t, err)
			// A failed attempt keeps both original DB units and Redis snapshots.
			require.NoError(t, mergeLegacyPlatformQuotaUsage(ctx, tx, rdb))
			_, err = tx.ExecContext(ctx, string(migration))
			require.NoError(t, err)
			_, err = tx.ExecContext(ctx, "SELECT 1 / 0")
			require.Error(t, err)
			_, err = tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT quota_handoff")
			require.NoError(t, err)
			var usage float64
			require.NoError(t, tx.QueryRowContext(ctx, `SELECT daily_usage_usd FROM user_platform_quotas WHERE user_id=$1`, uid).Scan(&usage))
			require.Equal(t, tc.dbUsage, usage)
			require.NoError(t, mergeLegacyPlatformQuotaUsage(ctx, tx, rdb))
			// Duplicate SCAN results/retries must merge absolute totals, never add twice.
			require.NoError(t, mergeLegacyPlatformQuotaUsage(ctx, tx, rdb))
			_, err = tx.ExecContext(ctx, string(migration))
			require.NoError(t, err)
			for _, window := range []string{"daily", "weekly", "monthly"} {
				require.NoError(t, tx.QueryRowContext(ctx, "SELECT "+window+"_usage_usd FROM user_platform_quotas WHERE user_id=$1", uid).Scan(&usage))
				require.InDelta(t, tc.want, usage, 1e-10)
			}
			raw, err := rdb.HGet(ctx, key, "daily_usage").Float64()
			require.NoError(t, err)
			require.Equal(t, tc.redisUsage, raw)
		})
	}
}

func TestSiteCreditsMigrationPreservesEmptyRewardConfigs(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()
	migration, err := dbmigrations.FS.ReadFile("239_convert_usd_balances_to_credits.sql")
	require.NoError(t, err)
	keys := []string{"daily_checkin_reward_ranges", "daily_checkin_streak_rules", "lottery_prize_pool"}
	for _, key := range keys {
		_, err = tx.ExecContext(ctx, `INSERT INTO settings (key, value) VALUES ($1, '[]') ON CONFLICT (key) DO UPDATE SET value = '[]'`, key)
		require.NoError(t, err)
	}
	_, err = tx.ExecContext(ctx, string(migration))
	require.NoError(t, err)
	for _, key := range keys {
		var value string
		require.NoError(t, tx.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = $1`, key).Scan(&value))
		require.JSONEq(t, "[]", value)
	}
}
