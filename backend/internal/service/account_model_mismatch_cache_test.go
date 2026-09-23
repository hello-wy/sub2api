//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTokenRefreshStateSyncPreservesConcurrentModelMismatch(t *testing.T) {
	stale := &Account{ID: 1, Status: StatusActive, Schedulable: true, Type: AccountTypeOAuth}
	fresh := *stale
	fresh.Schedulable = false
	fresh.Extra = map[string]any{AccountModelMismatchExtraKey: map[string]any{"expected_model": "gpt-6-astra", "actual_model": "gpt-5.6-luna"}}
	repo := &tokenRefreshAccountRepo{mockAccountRepoForGemini: mockAccountRepoForGemini{
		accountsByID: map[int64]*Account{1: &fresh},
	}}
	cache := &tokenRefreshSchedulerCache{}
	svc := &TokenRefreshService{accountRepo: repo, schedulerCache: cache}
	svc.postRefreshStateSync(context.Background(), stale)
	require.Equal(t, 1, cache.setAccountCalls)
	require.False(t, cache.lastAccount.Schedulable)
	require.True(t, cache.lastAccount.HasModelMismatch())
	repo.accountsByID = nil
	svc.postRefreshStateSync(context.Background(), stale)
	require.Equal(t, 1, cache.setAccountCalls, "a failed durable read must not publish the stale snapshot")
}

func TestAntigravityRateLimitCachePreservesConcurrentModelMismatch(t *testing.T) {
	stale := &Account{ID: 2, Status: StatusActive, Schedulable: true}
	fresh := *stale
	fresh.Schedulable = false
	fresh.Extra = map[string]any{AccountModelMismatchExtraKey: map[string]any{"expected_model": "gpt-6-astra", "actual_model": "gpt-5.6-luna"}}
	repo := &mockAccountRepoForGemini{accountsByID: map[int64]*Account{2: &fresh}}
	cache := &stubSchedulerCache{}
	svc := &AntigravityGatewayService{
		accountRepo:       repo,
		schedulerSnapshot: &SchedulerSnapshotService{cache: cache},
	}
	svc.updateAccountModelRateLimitInCache(context.Background(), stale, "claude-sonnet-4-5", time.Now().Add(time.Minute))
	require.Len(t, cache.setAccountCalls, 1)
	require.False(t, cache.setAccountCalls[0].Schedulable)
	require.True(t, cache.setAccountCalls[0].HasModelMismatch())
	repo.accountsByID = nil
	svc.updateAccountModelRateLimitInCache(context.Background(), stale, "claude-sonnet-4-5", time.Now().Add(time.Minute))
	require.Len(t, cache.setAccountCalls, 1, "a failed durable read must not publish the stale snapshot")
}
