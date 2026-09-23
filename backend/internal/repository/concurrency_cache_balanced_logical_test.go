package repository

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func logicalFixture() []service.LogicalAccountConcurrency {
	return []service.LogicalAccountConcurrency{
		{ID: 11, LoadAccountIDs: []int64{11, 12}, Channels: []service.AccountWithConcurrency{{ID: 11, MaxConcurrency: 2}, {ID: 12, MaxConcurrency: 2}}},
		{ID: 21, LoadAccountIDs: []int64{21, 22, 23}, Channels: []service.AccountWithConcurrency{{ID: 21, MaxConcurrency: 2}, {ID: 22, MaxConcurrency: 2}, {ID: 23, MaxConcurrency: 2}}},
	}
}

func TestBalancedLogicalAccountSlotsAlternateAndFillIPs(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	cache := NewConcurrencyCache(client, 15, 900)
	balanced := cache.(service.BalancedLogicalAccountSlotCache)
	ctx := context.Background()
	groups := logicalFixture()
	for i, want := range []int64{11, 21, 11, 21, 12, 22, 12, 22, 23, 23} {
		id, acquired, err := balanced.AcquireBalancedLogicalAccountSlot(ctx, groups, fmt.Sprintf("req-%d", i))
		require.NoError(t, err)
		require.True(t, acquired)
		require.Equal(t, want, id)
	}
	id, acquired, err := balanced.AcquireBalancedLogicalAccountSlot(ctx, groups, "full")
	require.NoError(t, err)
	require.False(t, acquired)
	require.Positive(t, id, "full pools return a wait placeholder, never an extra reservation")
	require.NoError(t, cache.ReleaseAccountSlot(ctx, 11, "req-0"))
	id, acquired, err = balanced.AcquireBalancedLogicalAccountSlot(ctx, groups, "refill")
	require.NoError(t, err)
	require.True(t, acquired)
	require.EqualValues(t, 11, id)
	// Retrying the same reservation must not create a second lease elsewhere.
	id, acquired, err = balanced.AcquireBalancedLogicalAccountSlot(ctx, groups, "req-4")
	require.NoError(t, err)
	require.True(t, acquired)
	require.EqualValues(t, 12, id)
	count, err := cache.GetAccountConcurrency(ctx, 12)
	require.NoError(t, err)
	require.Equal(t, 2, count)
}

func TestBalancedLogicalAccountSlotsRotateAtZeroLoad(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	cache := NewConcurrencyCache(client, 15, 900)
	balanced := cache.(service.BalancedLogicalAccountSlotCache)
	for i := 0; i < 20; i++ {
		request := fmt.Sprintf("short-%d", i)
		id, acquired, err := balanced.AcquireBalancedLogicalAccountSlot(context.Background(), logicalFixture(), request)
		require.NoError(t, err)
		require.True(t, acquired)
		require.EqualValues(t, 11+(i%2)*10, id)
		require.NoError(t, cache.ReleaseAccountSlot(context.Background(), id, request))
	}
}

func TestBalancedLogicalAccountSlotsMultiInstanceAtomicity(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	groups := logicalFixture()
	for i := range groups {
		for j := range groups[i].Channels {
			groups[i].Channels[j].MaxConcurrency = 50
		}
	}
	// Independent cache instances model simultaneous app processes. The larger
	// account must receive half the requests, not three fifths of the traffic.
	type result struct {
		id       int64
		acquired bool
		err      error
	}
	results := make(chan result, 120)
	var wg sync.WaitGroup
	for i := 0; i < 120; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			cache := NewConcurrencyCache(client, 15, 900).(service.BalancedLogicalAccountSlotCache)
			id, acquired, err := cache.AcquireBalancedLogicalAccountSlot(context.Background(), groups, fmt.Sprintf("concurrent-%d", i))
			results <- result{id, acquired, err}
		}(i)
	}
	wg.Wait()
	close(results)
	counts := map[int64]int{}
	for result := range results {
		require.NoError(t, result.err)
		require.True(t, result.acquired)
		counts[result.id]++
	}
	require.Equal(t, map[int64]int{11: 50, 12: 10, 21: 50, 22: 10}, counts)
}

func TestBalancedLogicalAccountSlotsCountDisabledAndLiveChannels(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	cache := NewConcurrencyCache(client, 15, 900)
	ctx := context.Background()
	groups := logicalFixture()
	groups[0].Channels = groups[0].Channels[1:]
	_, err := cache.AcquireAccountSlot(ctx, 11, 2, "draining-disabled")
	require.NoError(t, err)
	live := cache.(service.LiveConcurrencyCache)
	ok, err := live.AcquireLiveLease(ctx, 11, 2, 100, 10, 200, "live", false)
	require.NoError(t, err)
	require.True(t, ok)
	balanced := cache.(service.BalancedLogicalAccountSlotCache)
	for i, want := range []int64{21, 21, 12} {
		id, acquired, err := balanced.AcquireBalancedLogicalAccountSlot(ctx, groups, fmt.Sprintf("new-%d", i))
		require.NoError(t, err)
		require.True(t, acquired)
		require.Equal(t, want, id)
	}
	groups[1].LoadAccountIDs = append(groups[1].LoadAccountIDs, 11)
	_, _, err = balanced.AcquireBalancedLogicalAccountSlot(ctx, groups, "overlap")
	require.Error(t, err, "one physical lease may not count as multiple accounts")
}
