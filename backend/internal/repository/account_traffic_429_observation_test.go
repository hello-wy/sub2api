package repository

import (
	"context"
	"fmt"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestTraffic429OnlyCountsWithoutReducingAutomaticConcurrency(t *testing.T) {
	_, cache, plan := trafficFixture(t)
	plan.Policy.StrictRPMEnabled = false
	plan.Policy.AdaptiveEnabled = true
	plan.Policy.AdaptiveMode = "automatic"
	plan.Policy.FailureThreshold = 1
	ctx := service.With429Enforcement(context.Background(), false)
	// A prior version may already have reduced this account after 429s.
	require.NoError(t, cache.rdb.HSet(ctx, trafficKeys(plan.AccountID)[0], "recommended", 1, "failures", 100, "last_bad", 1).Err())
	for i := 0; i < 12; i++ {
		id := fmt.Sprintf("limited-%d", i)
		admission, err := cache.Acquire(ctx, plan, id)
		require.NoError(t, err)
		require.True(t, admission.Allowed)
		require.Equal(t, plan.HardLimit, admission.State.EffectiveConcurrency)
		require.NoError(t, cache.Finish(ctx, plan, id, 429, 100))
	}
	state, err := cache.Snapshot(ctx, plan)
	require.NoError(t, err)
	require.EqualValues(t, 12, state.Upstream429)
	require.Equal(t, plan.HardLimit, state.RecommendedConcurrency)
	require.Equal(t, plan.HardLimit, state.EffectiveConcurrency)
	require.Nil(t, state.LastAdjustmentAt)
	for i := 0; i < plan.HardLimit; i++ {
		admission, err := cache.Acquire(ctx, plan, fmt.Sprintf("full-capacity-%d", i))
		require.NoError(t, err)
		require.True(t, admission.Allowed)
	}
}

func TestTraffic429SwitchChangesEnforcementAndKeepsTelemetry(t *testing.T) {
	_, cache, plan := trafficFixture(t)
	plan.Policy.StrictRPMEnabled = false
	plan.Policy.AdaptiveEnabled = true
	plan.Policy.AdaptiveMode = "automatic"
	plan.Policy.FailureThreshold = 1
	on := service.With429Enforcement(context.Background(), true)
	off := service.With429Enforcement(context.Background(), false)
	admission, err := cache.Acquire(on, plan, "limited-on")
	require.NoError(t, err)
	require.True(t, admission.Allowed)
	require.NoError(t, cache.Finish(on, plan, "limited-on", 429, 100))
	state, err := cache.Snapshot(on, plan)
	require.NoError(t, err)
	require.Equal(t, 4, state.EffectiveConcurrency)
	state, err = cache.Snapshot(off, plan)
	require.NoError(t, err)
	require.Equal(t, 8, state.EffectiveConcurrency, "OFF must ignore the earlier 429 reduction")
	require.EqualValues(t, 1, state.Upstream429)
	state, err = cache.Snapshot(on, plan)
	require.NoError(t, err)
	require.Equal(t, 4, state.EffectiveConcurrency, "ON restores its enforcement state")
}

func TestTraffic429SwitchDoesNotClearExisting5xxProtection(t *testing.T) {
	_, cache, plan := trafficFixture(t)
	plan.Policy.StrictRPMEnabled = false
	plan.Policy.AdaptiveEnabled = true
	plan.Policy.AdaptiveMode = "automatic"
	plan.Policy.FailureThreshold = 1
	on := service.With429Enforcement(context.Background(), true)
	off := service.With429Enforcement(context.Background(), false)
	_, err := cache.Acquire(on, plan, "upstream-overload")
	require.NoError(t, err)
	require.NoError(t, cache.Finish(on, plan, "upstream-overload", 503, 100))
	state, err := cache.Snapshot(off, plan)
	require.NoError(t, err)
	require.Equal(t, 4, state.EffectiveConcurrency, "turning OFF 429 must not erase a 503-based reduction")
}
