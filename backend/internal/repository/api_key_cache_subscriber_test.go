package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestAPIKeyCacheSubscriber_BlocksUntilContextCancellation(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	defer func() { _ = client.Close() }()
	cache := NewAPIKeyCache(client)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	received := make(chan string, 1)
	returned := make(chan error, 1)
	go func() {
		returned <- cache.SubscribeAuthCacheInvalidation(ctx, func(value string) { received <- value })
	}()

	// Wait for the subscription without publishing on every polling attempt:
	// duplicate messages can fill received and block the callback during cancel.
	require.Eventually(t, func() bool {
		counts, err := client.PubSubNumSub(ctx, authCacheInvalidateChannel).Result()
		return err == nil && counts[authCacheInvalidateChannel] == 1
	}, time.Second, 10*time.Millisecond)
	require.NoError(t, client.Publish(ctx, authCacheInvalidateChannel, "hash").Err())
	select {
	case value := <-received:
		require.Equal(t, "hash", value)
	case <-time.After(time.Second):
		t.Fatal("subscriber did not receive invalidation")
	}
	select {
	case err := <-returned:
		t.Fatalf("subscriber returned while connection was active: %v", err)
	default:
	}
	cancel()
	select {
	case err := <-returned:
		require.True(t, errors.Is(err, context.Canceled))
	case <-time.After(time.Second):
		t.Fatal("subscriber did not stop after context cancellation")
	}
}
