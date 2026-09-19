package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"testing"
	"testing/fstest"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestLegacyQuotaWindowRejectsUnrecoverableSnapshots(t *testing.T) {
	for _, fields := range []map[string]string{
		{"daily_usage": "NaN"}, {"daily_usage": "+Inf"}, {"daily_usage": "-1"},
		{"daily_usage": "bad"}, {"daily_usage": "10"},
		{"daily_usage": "10", "daily_window_start": "invalid"},
	} {
		_, _, err := legacyQuotaWindow(fields, "daily")
		require.Error(t, err)
	}
	usage, start, err := legacyQuotaWindow(nil, "daily")
	require.NoError(t, err)
	require.Zero(t, usage)
	require.Nil(t, start)
	usage, start, err = legacyQuotaWindow(map[string]string{"daily_usage": "12.345", "daily_window_start": "1750000000"}, "daily")
	require.NoError(t, err)
	require.Equal(t, 12.345, usage)
	require.Equal(t, time.Unix(1750000000, 0).UTC(), *start)
}

func TestMergeLegacyQuotaScansUnmarkedHashesWithoutConsumingRedis(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	ctx := context.Background()
	key := legacyPlatformQuotaPrefix + "42:openai"
	fields := map[string]any{"daily_usage": "100", "daily_window_start": "1750000000"}
	require.NoError(t, rdb.HSet(ctx, key, fields).Err())
	require.NoError(t, rdb.Expire(ctx, key, time.Hour).Err())
	// No dirty member: this also covers a flusher that crashed after SPOP.
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	for i := 0; i < 2; i++ {
		mock.ExpectBegin()
		mock.ExpectExec("UPDATE user_platform_quotas").
			WithArgs(int64(42), "openai", 100.0, time.Unix(1750000000, 0).UTC()).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectRollback()
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		require.NoError(t, mergeLegacyPlatformQuotaUsage(ctx, tx, rdb))
		require.NoError(t, tx.Rollback())
	}
	got, err := rdb.HGetAll(ctx, key).Result()
	require.NoError(t, err)
	require.Equal(t, map[string]string{"daily_usage": "100", "daily_window_start": "1750000000"}, got)
	require.Equal(t, time.Hour, mr.TTL(key))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMergeLegacyQuotaDoesNotIgnoreRedisFailure(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr(), MaxRetries: -1})
	t.Cleanup(func() { _ = rdb.Close() })
	mr.SetError("ERR unavailable")
	err := mergeLegacyPlatformQuotaUsage(context.Background(), nil, rdb)
	require.ErrorContains(t, err, "scan legacy platform quota cache")
}

func TestMigrationHookSharesTransactionAndSkipsCompletedMigration(t *testing.T) {
	const content = "SELECT 239;"
	sum := sha256.Sum256([]byte(content))
	for _, mode := range []string{"success", "hook failure", "SQL failure", "already applied"} {
		t.Run(mode, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { _ = db.Close() })
			prepareMigrationsBootstrapExpectations(mock)
			lookup := mock.ExpectQuery("SELECT checksum FROM schema_migrations WHERE filename = \\$1").WithArgs(siteCreditsMigration)
			if mode == "already applied" {
				lookup.WillReturnRows(sqlmock.NewRows([]string{"checksum"}).AddRow(hex.EncodeToString(sum[:])))
			} else {
				lookup.WillReturnError(sql.ErrNoRows)
				mock.ExpectBegin()
				mock.ExpectExec("SELECT 238").WillReturnResult(sqlmock.NewResult(0, 1))
				switch mode {
				case "hook failure":
					mock.ExpectRollback()
				case "SQL failure":
					mock.ExpectExec("SELECT 239").WillReturnError(errors.New("SQL failed"))
					mock.ExpectRollback()
				default:
					mock.ExpectExec("SELECT 239").WillReturnResult(sqlmock.NewResult(0, 1))
					mock.ExpectExec("INSERT INTO schema_migrations").WithArgs(siteCreditsMigration, hex.EncodeToString(sum[:])).WillReturnResult(sqlmock.NewResult(1, 1))
					mock.ExpectCommit()
				}
			}
			mock.ExpectExec("SELECT pg_advisory_unlock").WithArgs(migrationsAdvisoryLockID).WillReturnResult(sqlmock.NewResult(0, 1))
			calls := 0
			err = applyMigrationsFS(context.Background(), db, fstest.MapFS{
				siteCreditsMigration: &fstest.MapFile{Data: []byte(content)},
			}, func(ctx context.Context, tx *sql.Tx, name string) error {
				calls++
				require.Equal(t, siteCreditsMigration, name)
				_, err := tx.ExecContext(ctx, "SELECT 238")
				require.NoError(t, err)
				if mode == "hook failure" {
					return errors.New("Redis snapshot failed")
				}
				return nil
			})
			if mode == "hook failure" || mode == "SQL failure" {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			if mode == "already applied" {
				require.Zero(t, calls)
			} else {
				require.Equal(t, 1, calls)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
