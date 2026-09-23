package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

func codexTicketCoordinationKeys(id int64) []string {
	return []string{fmt.Sprintf("{codex-ticket}:lease:%d", id), "{codex-ticket}:active", fmt.Sprintf("{codex-ticket}:cooldown:%d", id), "{codex-ticket}:legacy-harvester"}
}

var claimCodexTicketHarvestScript = redis.NewScript(`
-- During the first rolling upgrade the legacy worker does not know our lease
-- protocol. Wait for its drain without disabling ticket use by business traffic.
if redis.call('EXISTS',KEYS[4])==1 then return {0,0,0,''} end
local t=redis.call('TIME')
local now=tonumber(t[1])*1000+math.floor(tonumber(t[2])/1000)
local cooldown=redis.call('GET',KEYS[3])
if cooldown then
 local saved=cjson.decode(cooldown)
 if saved.source==ARGV[2] and (ARGV[6]=='1' or not string.find(saved.reason or '', '429', 1, true)) and tonumber(saved.retry_after)>now then
  return {0,0,saved.retry_after,saved.reason}
 end
end
local owner=redis.call('GET',KEYS[1])
if owner then return {0,1,0,''} end
redis.call('ZREMRANGEBYSCORE',KEYS[2],'-inf',now)
if redis.call('ZCARD',KEYS[2])>=tonumber(ARGV[4]) then return {0,0,0,''} end
redis.call('SET',KEYS[1],ARGV[3],'PX',ARGV[5])
redis.call('ZADD',KEYS[2],now+tonumber(ARGV[5]),ARGV[1])
redis.call('PEXPIRE',KEYS[2],tonumber(ARGV[5])*2)
return {1,1,0,''}
`)

var refreshCodexTicketHarvestScript = redis.NewScript(`
if redis.call('GET',KEYS[1])~=ARGV[2] then return 0 end
local t=redis.call('TIME')
local now=tonumber(t[1])*1000+math.floor(tonumber(t[2])/1000)
redis.call('PEXPIRE',KEYS[1],ARGV[3])
redis.call('ZADD',KEYS[2],now+tonumber(ARGV[3]),ARGV[1])
redis.call('PEXPIRE',KEYS[2],tonumber(ARGV[3])*2)
return 1
`)

var finishCodexTicketHarvestScript = redis.NewScript(`
if redis.call('GET',KEYS[1])~=ARGV[3] then return 0 end
if tonumber(ARGV[4])>0 and (ARGV[6]=='1' or not string.find(ARGV[5], '429', 1, true)) then
 local t=redis.call('TIME')
 local now=tonumber(t[1])*1000+math.floor(tonumber(t[2])/1000)
 redis.call('SET',KEYS[3],cjson.encode({source=ARGV[2],retry_after=now+tonumber(ARGV[4]),reason=ARGV[5]}),'PX',ARGV[4])
else
 redis.call('DEL',KEYS[3])
end
redis.call('DEL',KEYS[1])
redis.call('ZREM',KEYS[2],ARGV[1])
return 1
`)

var readCodexTicketHarvestScript = redis.NewScript(`
local t=redis.call('TIME')
local now=tonumber(t[1])*1000+math.floor(tonumber(t[2])/1000)
local cooldown=redis.call('GET',KEYS[3])
if cooldown then
 local saved=cjson.decode(cooldown)
 if ARGV[2]~='1' and saved.source==ARGV[1] and string.find(saved.reason or '', '429', 1, true) then return {0,redis.call('EXISTS',KEYS[1]),0,saved.reason} end
 if saved.source==ARGV[1] and tonumber(saved.retry_after)>now then return {0,0,saved.retry_after,saved.reason} end
end
return {0,redis.call('EXISTS',KEYS[1]),0,''}
`)

func codexTicketHarvestState(values []interface{}, err error) (service.CodexTicketHarvestState, error) {
	if err != nil {
		return service.CodexTicketHarvestState{}, err
	}
	if len(values) != 4 {
		return service.CodexTicketHarvestState{}, errors.New("invalid STATE coordination response")
	}
	acquired, a := values[0].(int64)
	running, b := values[1].(int64)
	retry, c := values[2].(int64)
	reason, d := values[3].(string)
	if !a || !b || !c || !d {
		return service.CodexTicketHarvestState{}, errors.New("invalid STATE coordination response types")
	}
	state := service.CodexTicketHarvestState{Acquired: acquired == 1, Running: running == 1, LastError: reason}
	if retry > 0 {
		state.RetryAfter = time.UnixMilli(retry)
	}
	return state, nil
}

func (c *gatewayCache) ClaimCodexTicketHarvest(ctx context.Context, id int64, source, owner string, limit int, ttl time.Duration) (service.CodexTicketHarvestState, error) {
	if id <= 0 || source == "" || owner == "" || limit <= 0 || ttl <= 0 {
		return service.CodexTicketHarvestState{}, errors.New("invalid STATE harvest claim")
	}
	values, err := claimCodexTicketHarvestScript.Run(ctx, c.rdb, codexTicketCoordinationKeys(id), id, source, owner, limit, ttl.Milliseconds(), boolToInt429(service.Context429Enforcement(ctx))).Slice()
	return codexTicketHarvestState(values, err)
}
func (c *gatewayCache) RefreshCodexTicketHarvest(ctx context.Context, id int64, owner string, ttl time.Duration) (bool, error) {
	if id <= 0 || owner == "" || ttl <= 0 {
		return false, errors.New("invalid STATE harvest renewal")
	}
	n, err := refreshCodexTicketHarvestScript.Run(ctx, c.rdb, codexTicketCoordinationKeys(id), id, owner, ttl.Milliseconds()).Int()
	return n == 1, err
}
func (c *gatewayCache) FinishCodexTicketHarvest(ctx context.Context, id int64, source, owner string, cooldown time.Duration, reason string) (bool, error) {
	if id <= 0 || source == "" || owner == "" {
		return false, errors.New("invalid STATE harvest release")
	}
	if cooldown < 0 {
		cooldown = 0
	}
	n, err := finishCodexTicketHarvestScript.Run(ctx, c.rdb, codexTicketCoordinationKeys(id), id, source, owner, cooldown.Milliseconds(), reason, boolToInt429(service.Context429Enforcement(ctx))).Int()
	return n == 1, err
}
func (c *gatewayCache) GetCodexTicketHarvestState(ctx context.Context, id int64, source string) (service.CodexTicketHarvestState, error) {
	values, err := readCodexTicketHarvestScript.Run(ctx, c.rdb, codexTicketCoordinationKeys(id), source, boolToInt429(service.Context429Enforcement(ctx))).Slice()
	return codexTicketHarvestState(values, err)
}

var _ service.CodexTicketHarvestCoordinator = (*gatewayCache)(nil)
