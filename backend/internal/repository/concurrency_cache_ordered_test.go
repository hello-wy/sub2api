package repository

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestOrderedAccountSlotsConcurrentFillAndRefill(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	cache := NewConcurrencyCache(client, 15, 900)
	ordered, ok := cache.(service.OrderedAccountSlotCache)
	require.True(t, ok)
	ctx := context.Background()
	channels := []service.AccountWithConcurrency{{ID: 31, MaxConcurrency: 20}, {ID: 32, MaxConcurrency: 20}, {ID: 33, MaxConcurrency: 20}}
	type reservation struct {
		id      int64
		request string
		err     error
	}
	results := make(chan reservation, 70)
	var wg sync.WaitGroup
	for i := 0; i < 70; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			request := fmt.Sprintf("simultaneous-%d", i)
			id, err := ordered.AcquireOrderedAccountSlot(ctx, channels, request)
			results <- reservation{id, request, err}
		}(i)
	}
	wg.Wait()
	close(results)
	counts := map[int64]int{}
	firstRequest := ""
	for result := range results {
		require.NoError(t, result.err)
		counts[result.id]++
		if result.id == 31 {
			firstRequest = result.request
		}
	}
	require.Equal(t, map[int64]int{0: 10, 31: 20, 32: 20, 33: 20}, counts)
	for _, channel := range channels {
		count, err := cache.GetAccountConcurrency(ctx, channel.ID)
		require.NoError(t, err)
		require.Equal(t, 20, count)
	}
	require.NoError(t, cache.ReleaseAccountSlot(ctx, 31, firstRequest))
	id, err := ordered.AcquireOrderedAccountSlot(ctx, channels, "refill")
	require.NoError(t, err)
	require.Equal(t, int64(31), id, "new work returns to the first IP after a slot is freed")
}

func TestOrderedAccountSlotsCountLiveAndRetryIdempotently(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	cache := NewConcurrencyCache(client, 15, 900)
	ordered, ok := cache.(service.OrderedAccountSlotCache)
	require.True(t, ok)
	live, ok := cache.(service.LiveConcurrencyCache)
	require.True(t, ok)
	ctx := context.Background()
	channels := []service.AccountWithConcurrency{{ID: 41, MaxConcurrency: 1}, {ID: 42, MaxConcurrency: 1}}
	ok, err := live.AcquireLiveLease(ctx, 41, 1, 50, 10, 60, "live", false)
	require.NoError(t, err)
	require.True(t, ok)
	id, err := ordered.AcquireOrderedAccountSlot(ctx, channels, "regular")
	require.NoError(t, err)
	require.Equal(t, int64(42), id)
	require.NoError(t, live.ReleaseLiveLease(ctx, 41, 50, 60, "live"))
	id, err = ordered.AcquireOrderedAccountSlot(ctx, channels, "regular")
	require.NoError(t, err)
	require.Equal(t, int64(42), id, "retry preserves the existing lease even when the first IP is free")
	id, err = ordered.AcquireOrderedAccountSlot(ctx, channels, "new-request")
	require.NoError(t, err)
	require.Equal(t, int64(41), id)
	require.EqualValues(t, 1, client.ZCard(ctx, accountSlotKey(42)).Val())
}

func TestOrderedAccountSlotsExpireAndUnlimited(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	cache := NewConcurrencyCache(client, 15, 900)
	ordered, ok := cache.(service.OrderedAccountSlotCache)
	require.True(t, ok)
	ctx := context.Background()
	now := time.Now().Unix()
	require.NoError(t, client.ZAdd(ctx, accountSlotKey(71), redis.Z{Score: float64(now - 901), Member: "expired"}).Err())
	require.NoError(t, client.ZAdd(ctx, liveAccountSlotKey(71), redis.Z{Score: float64(now - 61), Member: "expired-live"}).Err())
	id, err := ordered.AcquireOrderedAccountSlot(ctx, []service.AccountWithConcurrency{{ID: 71, MaxConcurrency: 1}, {ID: 72, MaxConcurrency: 1}}, "fresh")
	require.NoError(t, err)
	require.Equal(t, int64(71), id)
	channels := []service.AccountWithConcurrency{{ID: 81, MaxConcurrency: 0}, {ID: 82, MaxConcurrency: 1}}
	for i := 0; i < 5; i++ {
		id, err := ordered.AcquireOrderedAccountSlot(ctx, channels, fmt.Sprintf("unlimited-%d", i))
		require.NoError(t, err)
		require.Equal(t, int64(81), id)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = ordered.AcquireOrderedAccountSlot(canceled, channels, "canceled")
	require.Error(t, err)
	require.EqualValues(t, 5, client.ZCard(ctx, accountSlotKey(81)).Val())
}
