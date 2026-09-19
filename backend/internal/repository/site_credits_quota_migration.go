package repository

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/redis/go-redis/v9"
)

const siteCreditsMigration = "239_convert_usd_balances_to_credits.sql"
const legacyPlatformQuotaPrefix = "billing:user_platform_quota:"

func siteCreditsQuotaMigrationHook(cfg *config.Config) transactionMigrationHook {
	return func(ctx context.Context, tx *sql.Tx, name string) error {
		if name != siteCreditsMigration {
			return nil
		}
		// Only connect when 239 is pending. Redis must be readable before converting
		// the DB: a failed read must not silently discard unflushed usage.
		rdb := InitRedis(cfg)
		defer func() { _ = rdb.Close() }()
		return mergeLegacyPlatformQuotaUsage(ctx, tx, rdb)
	}
}

// mergeLegacyPlatformQuotaUsage runs BEFORE SQL divides the balances by ten.
// Redis is read-only: do not SPOP/delete/rename its keys, so a failed migration
// can retry with the same snapshots. Both the merge and the conversion roll back
// together. The migration ledger prevents reimporting old snapshots on restart.
// Scan hashes rather than only dirty members: a crashed flusher may have SPOP'ed
// a member without committing its snapshot. Old writers must be stopped for this
// unit migration, just as for the balance/subscription SQL conversion.
func mergeLegacyPlatformQuotaUsage(ctx context.Context, tx *sql.Tx, rdb *redis.Client) error {
	var cursor uint64
	for {
		keys, next, err := rdb.Scan(ctx, cursor, legacyPlatformQuotaPrefix+"*", 256).Result()
		if err != nil {
			return fmt.Errorf("scan legacy platform quota cache: %w", err)
		}
		for _, key := range keys {
			owner, ok := parseUserPlatformQuotaDirtyMember(strings.TrimPrefix(key, legacyPlatformQuotaPrefix))
			if !ok || owner.UserID <= 0 || owner.Platform == "" {
				return fmt.Errorf("invalid legacy platform quota key %q", key)
			}
			fields, err := rdb.HGetAll(ctx, key).Result()
			if err != nil {
				return fmt.Errorf("read legacy platform quota %s: %w", key, err)
			}
			for _, window := range []string{"daily", "weekly", "monthly"} {
				usage, start, err := legacyQuotaWindow(fields, window)
				if err != nil {
					return fmt.Errorf("read legacy platform quota %s: %w", key, err)
				}
				if start == nil {
					continue
				}
				// Column names come exclusively from the fixed window list above.
				// Same-window snapshots merge with MAX, not SUM, because Redis holds
				// absolute totals, including usage already flushed to the DB.
				// Redis timestamps have second precision; compare at that precision.
				query := fmt.Sprintf(`UPDATE user_platform_quotas
SET %[1]s_usage_usd = CASE
      WHEN %[1]s_window_start IS NULL OR date_trunc('second', %[1]s_window_start) < $4 THEN $3
      ELSE GREATEST(%[1]s_usage_usd, $3) END,
    %[1]s_window_start = CASE
      WHEN %[1]s_window_start IS NULL OR date_trunc('second', %[1]s_window_start) < $4 THEN $4
      ELSE %[1]s_window_start END,
    updated_at = NOW()
WHERE user_id = $1 AND platform = $2 AND deleted_at IS NULL
  AND (%[1]s_window_start IS NULL OR date_trunc('second', %[1]s_window_start) <= $4)`, window)
				if _, err := tx.ExecContext(ctx, query, owner.UserID, owner.Platform, usage, *start); err != nil {
					return fmt.Errorf("merge legacy platform quota %s %s: %w", key, window, err)
				}
			}
		}
		cursor = next
		if cursor == 0 {
			return nil
		}
	}
}

func legacyQuotaWindow(fields map[string]string, window string) (float64, *time.Time, error) {
	var usage float64
	if raw := fields[window+"_usage"]; raw != "" {
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
			return 0, nil, fmt.Errorf("invalid %s usage %q", window, raw)
		}
		usage = value
	}
	raw := fields[window+"_window_start"]
	if raw == "" && usage == 0 {
		return 0, nil, nil // Missing hash or an empty unlimited sentinel.
	}
	seconds, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || seconds <= 0 {
		return 0, nil, fmt.Errorf("invalid %s window start %q", window, raw)
	}
	start := time.Unix(seconds, 0).UTC()
	return usage, &start, nil
}
