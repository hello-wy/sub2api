package repository

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

// Both layers are decided with the lease insertion on the Redis primary. A
// cached load snapshot followed by a separate reservation can stampede one
// logical account, even when each individual channel reservation is atomic.
var acquireBalancedLogicalAccountScript = redis.NewScript(`
redis.replicate_commands()
local now = tonumber(redis.call('TIME')[1])
local ttl, requestID = tonumber(ARGV[1]), ARGV[2]
local groups = cjson.decode(ARGV[3])
local n = tonumber(ARGV[4])
local counts = {}
for i = 1, n do
    local key, liveKey = KEYS[2*i-1], KEYS[2*i]
    redis.call('ZREMRANGEBYSCORE', key, '-inf', now-ttl)
    redis.call('ZREMRANGEBYSCORE', liveKey, '-inf', now-60)
    if redis.call('ZSCORE', key, requestID) ~= false then
        redis.call('ZADD', key, now, requestID)
        redis.call('EXPIRE', key, ttl)
        return {i, now}
    end
    counts[i] = redis.call('ZCARD', key) + redis.call('ZCARD', liveKey)
end
local best, waiting = nil, nil
local function better(a,b)
    if b == nil then return true end
    if a.priority ~= b.priority then return a.priority < b.priority end
    if a.load ~= b.load then return a.load < b.load end
    if a.last ~= b.last then return a.last < b.last end
    return a.id < b.id
end
for gi, group in ipairs(groups) do
    local load, channel = 0, nil
    for _, index in ipairs(group.loads) do load = load + counts[index] end
    for _, candidate in ipairs(group.channels) do
        if candidate.limit <= 0 or counts[candidate.index] < candidate.limit then
            channel = candidate.index
            break
        end
    end
    local candidate = {id=group.id, priority=group.priority, load=load,
        last=tonumber(redis.call('GET', KEYS[2*n+1+gi])) or 0,
        group=gi, channel=channel or group.channels[1].index}
    if channel and better(candidate,best) then best = candidate end
    if better(candidate,waiting) then waiting = candidate end
end
local chosen = best or waiting
if chosen == nil then return {0,now} end
local sequence = redis.call('INCR', KEYS[2*n+1])
redis.call('SET', KEYS[2*n+1+chosen.group], sequence, 'EX', 86400)
if best == nil then return {-chosen.channel,now} end
redis.call('ZADD', KEYS[2*chosen.channel-1], now, requestID)
redis.call('EXPIRE', KEYS[2*chosen.channel-1], ttl)
return {chosen.channel,now}
`)

func (c *concurrencyCache) AcquireBalancedLogicalAccountSlot(ctx context.Context, groups []service.LogicalAccountConcurrency, requestID string) (int64, bool, error) {
	if len(groups) == 0 {
		return 0, false, nil
	}
	type channel struct {
		Index int `json:"index"`
		Limit int `json:"limit"`
	}
	type group struct {
		ID       int64     `json:"id"`
		Priority int       `json:"priority"`
		Loads    []int     `json:"loads"`
		Channels []channel `json:"channels"`
	}
	keys := []string{}
	ids := []int64{}
	indices := map[int64]int{}
	owners := map[int64]int64{}
	seenGroups := map[int64]bool{}
	encoded := make([]group, 0, len(groups))
	for _, g := range groups {
		if g.ID <= 0 || seenGroups[g.ID] || len(g.Channels) == 0 {
			return 0, false, errors.New("invalid or duplicate logical account")
		}
		seenGroups[g.ID] = true
		v := group{ID: g.ID, Priority: g.Priority}
		seenLoads := map[int64]bool{}
		for _, id := range append(append([]int64(nil), g.LoadAccountIDs...), logicalChannelIDs(g.Channels)...) {
			if id <= 0 || (owners[id] != 0 && owners[id] != g.ID) {
				return 0, false, errors.New("invalid or overlapping logical account channel")
			}
			owners[id] = g.ID
			if indices[id] == 0 {
				ids = append(ids, id)
				indices[id] = len(ids)
				keys = append(keys, accountSlotKey(id), liveAccountSlotKey(id))
			}
			if !seenLoads[id] {
				v.Loads = append(v.Loads, indices[id])
				seenLoads[id] = true
			}
		}
		seenChannels := map[int64]bool{}
		for _, a := range g.Channels {
			if seenChannels[a.ID] {
				return 0, false, errors.New("duplicate eligible logical account channel")
			}
			seenChannels[a.ID] = true
			v.Channels = append(v.Channels, channel{indices[a.ID], a.MaxConcurrency})
		}
		encoded = append(encoded, v)
	}
	keys = append(keys, "concurrency:logical:sequence")
	for _, g := range groups {
		keys = append(keys, "concurrency:logical:last:"+strconv.FormatInt(g.ID, 10))
	}
	payload, err := json.Marshal(encoded)
	if err != nil {
		return 0, false, err
	}
	index, now, err := runScriptInt64Pair(ctx, c.rdb, acquireBalancedLogicalAccountScript, keys, c.slotTTLSeconds, requestID, string(payload), len(ids))
	if err != nil || index == 0 {
		return 0, false, err
	}
	acquired := index > 0
	if index < 0 {
		index = -index
	}
	if index > int64(len(ids)) {
		return 0, false, errors.New("invalid logical account reservation")
	}
	id := ids[index-1]
	if acquired {
		c.touchActiveIndexAt(ctx, accountActiveIndexKey, id, now+int64(c.slotTTLSeconds))
	}
	return id, acquired, nil
}

func logicalChannelIDs(channels []service.AccountWithConcurrency) []int64 {
	ids := make([]int64, 0, len(channels))
	for _, channel := range channels {
		ids = append(ids, channel.ID)
	}
	return ids
}
