//go:build unit

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestSchedulerCacheModelMismatchRejectsDelayedAccountAndBucketWrites(t *testing.T) {
	ctx := context.Background()
	cache := newSchedulerCacheUnit(t)
	before := service.Account{
		ID: 765, Platform: service.PlatformOpenAI, Type: service.AccountTypeOAuth,
		Status: service.StatusActive, Schedulable: true,
		UpdatedAt: time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC),
	}
	blocked := before
	blocked.Schedulable = false
	blocked.UpdatedAt = before.UpdatedAt.Add(time.Microsecond)
	blocked.Extra = map[string]any{service.AccountModelMismatchExtraKey: map[string]any{
		"expected_model": "gpt-6-astra", "actual_model": "gpt-5.6-luna",
	}}
	require.NoError(t, cache.SetAccount(ctx, &blocked))
	// Delayed post-refresh state and a delayed initial bucket rebuild both
	// carry an older committed row. Neither may clear the quarantine.
	require.NoError(t, cache.SetAccount(ctx, &before))
	bucket := service.SchedulerBucket{GroupID: 765, Platform: service.PlatformOpenAI, Mode: service.SchedulerModeSingle}
	token, err := cache.CaptureBucketWriteToken(ctx, bucket)
	require.NoError(t, err)
	require.NoError(t, cache.SetSnapshot(ctx, bucket, token, []service.Account{before}))
	cached, err := cache.GetAccount(ctx, before.ID)
	require.NoError(t, err)
	require.NotNil(t, cached)
	require.False(t, cached.Schedulable)
	require.True(t, cached.HasModelMismatch())
	snapshot, hit, err := cache.GetSnapshot(ctx, bucket)
	require.NoError(t, err)
	require.True(t, hit)
	require.Len(t, snapshot, 1)
	require.True(t, snapshot[0].HasModelMismatch(), "selection metadata must retain the quarantine")
	require.False(t, snapshot[0].IsSchedulable())

	// Explicit recovery produces a newer durable revision. A late quarantine
	// snapshot must not undo that recovery either.
	resumed := before
	resumed.UpdatedAt = blocked.UpdatedAt.Add(time.Microsecond)
	require.NoError(t, cache.SetAccount(ctx, &resumed))
	require.NoError(t, cache.SetAccount(ctx, &blocked))
	cached, err = cache.GetAccount(ctx, before.ID)
	require.NoError(t, err)
	require.True(t, cached.IsSchedulable())
	require.False(t, cached.HasModelMismatch())
}

func TestSchedulerCacheDeleteRemovesRevision(t *testing.T) {
	ctx := context.Background()
	cache, mr := newSchedulerCacheUnitWithRedis(t)
	account := &service.Account{ID: 766, UpdatedAt: time.Now()}
	require.NoError(t, cache.SetAccount(ctx, account))
	require.True(t, mr.Exists(schedulerAccountRevisionKey("766")))
	require.NoError(t, cache.DeleteAccount(ctx, account.ID))
	require.False(t, mr.Exists(schedulerAccountRevisionKey("766")))
}

func TestSchedulerCacheModelMismatchObservationRetainsEvidenceWithoutBlocking(t *testing.T) {
	ctx := context.Background()
	cache := newSchedulerCacheUnit(t)
	account := service.Account{ID: 769, Platform: service.PlatformOpenAI, Type: service.AccountTypeOAuth,
		Status: service.StatusActive, Schedulable: true, UpdatedAt: time.Now(),
		Extra: map[string]any{service.AccountModelMismatchExtraKey: map[string]any{
			"expected_model": "gpt-6-astra", "actual_model": "gpt-5.6-luna", "quarantined": false,
		}}}
	require.NoError(t, cache.SetAccount(ctx, &account))
	bucket := service.SchedulerBucket{GroupID: 769, Platform: service.PlatformOpenAI, Mode: service.SchedulerModeSingle}
	token, err := cache.CaptureBucketWriteToken(ctx, bucket)
	require.NoError(t, err)
	require.NoError(t, cache.SetSnapshot(ctx, bucket, token, []service.Account{account}))
	cached, err := cache.GetAccount(ctx, account.ID)
	require.NoError(t, err)
	require.True(t, cached.HasModelMismatch())
	require.False(t, cached.IsModelMismatchQuarantined())
	require.True(t, cached.IsSchedulable())
	snapshot, hit, err := cache.GetSnapshot(ctx, bucket)
	require.NoError(t, err)
	require.True(t, hit)
	require.Len(t, snapshot, 1)
	require.True(t, snapshot[0].HasModelMismatch())
	require.True(t, snapshot[0].IsSchedulable())
}
