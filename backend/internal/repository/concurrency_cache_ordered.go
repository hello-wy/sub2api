package repository

import (
	"context"
	"errors"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

// All keys use the existing regular/live account lease namespace. This script
// runs on the application's single Redis primary, just like acquireLiveLease.
// A channel is never selected from a previously read/cached concurrency count.
var acquireOrderedAccountScript = redis.NewScript(`
	redis.replicate_commands()
	local now = tonumber(redis.call('TIME')[1])
	local ttl = tonumber(ARGV[1])
	local requestID = ARGV[2]
	local n = #KEYS / 2
	for i = 1, n do
		local key = KEYS[2*i-1]
		local liveKey = KEYS[2*i]
		redis.call('ZREMRANGEBYSCORE', key, '-inf', now-ttl)
		redis.call('ZREMRANGEBYSCORE', liveKey, '-inf', now-60)
	end
	-- A retried reservation must stay on its original channel, even if an
	-- earlier channel has become free, otherwise one request owns two slots.
	for i = 1, n do
		local key = KEYS[2*i-1]
		if redis.call('ZSCORE', key, requestID) ~= false then
			redis.call('ZADD', key, now, requestID)
			redis.call('EXPIRE', key, ttl)
			return {i, now}
		end
	end
	for i = 1, n do
		local key = KEYS[2*i-1]
		local count = redis.call('ZCARD', key) + redis.call('ZCARD', KEYS[2*i])
		local limit = tonumber(ARGV[i+2])
		if limit <= 0 or count < limit then
			redis.call('ZADD', key, now, requestID)
			redis.call('EXPIRE', key, ttl)
			return {i, now}
		end
	end
	return {0, now}
`)

func (c *concurrencyCache) AcquireOrderedAccountSlot(ctx context.Context, accounts []service.AccountWithConcurrency, requestID string) (int64, error) {
	if len(accounts) == 0 {
		return 0, nil
	}
	keys := make([]string, 0, 2*len(accounts))
	args := []interface{}{c.slotTTLSeconds, requestID}
	seen := make(map[int64]bool, len(accounts))
	for _, account := range accounts {
		if account.ID <= 0 || seen[account.ID] {
			return 0, errors.New("invalid or duplicate ordered IP channel")
		}
		seen[account.ID] = true
		keys = append(keys, accountSlotKey(account.ID), liveAccountSlotKey(account.ID))
		args = append(args, account.MaxConcurrency)
	}
	index, now, err := runScriptInt64Pair(ctx, c.rdb, acquireOrderedAccountScript, keys, args...)
	if err != nil || index == 0 {
		return 0, err
	}
	if index < 1 || index > int64(len(accounts)) {
		return 0, errors.New("invalid ordered IP channel reservation")
	}
	id := accounts[index-1].ID
	c.touchActiveIndexAt(ctx, accountActiveIndexKey, id, now+int64(c.slotTTLSeconds))
	return id, nil
}
