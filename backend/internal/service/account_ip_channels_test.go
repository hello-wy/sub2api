//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type ipChannelBindingRepo struct {
	AccountRepository
	AccountIPChannelRepository
	members map[int64][]AccountIPChannel
}

func (r ipChannelBindingRepo) GetAccountIPChannels(_ context.Context, ids []int64) (map[int64][]AccountIPChannel, error) {
	out := map[int64][]AccountIPChannel{}
	for _, id := range ids {
		if m, ok := r.members[id]; ok {
			out[id] = m
		}
	}
	return out, nil
}

func TestAccountIPChannelsRequestBindingAndRetryExclusion(t *testing.T) {
	members := []AccountIPChannel{{Account: &Account{ID: 1}, LogicalAccountID: 1}, {Account: &Account{ID: 2}, LogicalAccountID: 1}, {Account: &Account{ID: 3}, LogicalAccountID: 1}}
	repo := ipChannelBindingRepo{members: map[int64][]AccountIPChannel{1: members, 2: members, 3: members}}
	ctx := withAccountIPRequestBinding(context.Background())
	require.NoError(t, rememberAccountIPRequestBinding(ctx, repo, &AccountSelectionResult{Account: &Account{ID: 2}}))
	excluded := applyAccountIPRequestBinding(ctx, nil)
	require.Contains(t, excluded, int64(1))
	require.Contains(t, excluded, int64(3))
	require.NotContains(t, excluded, int64(2))
	require.Empty(t, applyAccountIPRequestBinding(withAccountIPRequestBinding(context.Background()), nil), "a different request can independently use every fixed channel")
	failed, err := expandIPChannelExclusions(ctx, repo, map[int64]struct{}{2: {}})
	require.NoError(t, err)
	require.Len(t, failed, 3)
}

func TestAccountIPChannelsRuntimeExtraPreserved(t *testing.T) {
	for _, key := range []string{"codex_fingerprint_seed", "codex_primary_used_percent", "quota_used", "proxy_fallback_origin_id"} {
		require.True(t, isIPChannelRuntimeExtra(key))
	}
	for _, key := range []string{"codex_cli_only", "quota_daily_limit", "base_rpm"} {
		require.False(t, isIPChannelRuntimeExtra(key))
	}
}
