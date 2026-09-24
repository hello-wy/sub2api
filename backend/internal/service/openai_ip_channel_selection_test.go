package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type orderedIPTestRepo struct {
	schedulerTestOpenAIAccountRepo
	disabled        map[int64]bool
	logicalDisabled bool
	logicalIDs      map[int64]int64
}

func (r *orderedIPTestRepo) GetAccountIPChannels(_ context.Context, ids []int64) (map[int64][]AccountIPChannel, error) {
	out := map[int64][]AccountIPChannel{}
	for _, id := range ids {
		for i := range r.accounts {
			if r.accounts[i].ID == id {
				logicalID := int64(301)
				if r.logicalIDs != nil {
					logicalID = r.logicalIDs[id]
					if logicalID == 0 {
						continue
					}
				}
				for j := range r.accounts {
					a := r.accounts[j]
					if r.logicalIDs != nil && r.logicalIDs[a.ID] != logicalID {
						continue
					}
					out[id] = append(out[id], AccountIPChannel{Account: &a, LogicalAccountID: logicalID, Enabled: !r.disabled[a.ID], LogicalEnabled: !r.logicalDisabled})
				}
			}
		}
	}
	return out, nil
}

type orderedIPTestCache struct {
	schedulerTestConcurrencyCache
	mu            sync.Mutex
	leases        map[int64]map[string]bool
	orders        [][]int64
	physicalCalls []int64
	last          map[int64]int
	sequence      int
}

func (c *orderedIPTestCache) AcquireBalancedLogicalAccountSlot(_ context.Context, groups []LogicalAccountConcurrency, req string) (int64, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.last == nil {
		c.last = map[int64]int{}
	}
	bestGroup, bestID, bestLoad := int64(0), int64(0), int(^uint(0)>>1)
	for _, group := range groups {
		load := 0
		for _, id := range group.LoadAccountIDs {
			load += len(c.leases[id])
		}
		id := int64(0)
		order := []int64{}
		for _, a := range group.Channels {
			order = append(order, a.ID)
			if id == 0 && (a.MaxConcurrency <= 0 || len(c.leases[a.ID]) < a.MaxConcurrency) {
				id = a.ID
			}
		}
		c.orders = append(c.orders, order)
		if id != 0 && (bestID == 0 || load < bestLoad || (load == bestLoad && c.last[group.ID] < c.last[bestGroup])) {
			bestGroup, bestID, bestLoad = group.ID, id, load
		}
	}
	if bestID == 0 {
		if len(groups) > 0 {
			return groups[0].Channels[0].ID, false, nil
		}
		return 0, false, nil
	}
	c.sequence++
	c.last[bestGroup] = c.sequence
	if c.leases[bestID] == nil {
		c.leases[bestID] = map[string]bool{}
	}
	c.leases[bestID][req] = true
	return bestID, true, nil
}

func (c *orderedIPTestCache) AcquireOrderedAccountSlot(_ context.Context, limits []AccountWithConcurrency, req string) (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	order := []int64{}
	for _, a := range limits {
		order = append(order, a.ID)
	}
	c.orders = append(c.orders, order)
	for _, a := range limits {
		if len(c.leases[a.ID]) < a.MaxConcurrency || a.MaxConcurrency <= 0 {
			if c.leases[a.ID] == nil {
				c.leases[a.ID] = map[string]bool{}
			}
			c.leases[a.ID][req] = true
			return a.ID, nil
		}
	}
	return 0, nil
}

func (c *orderedIPTestCache) AcquireAccountSlot(_ context.Context, id int64, max int, req string) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.physicalCalls = append(c.physicalCalls, id)
	if max > 0 && len(c.leases[id]) >= max {
		return false, nil
	}
	if c.leases[id] == nil {
		c.leases[id] = map[string]bool{}
	}
	c.leases[id][req] = true
	return true, nil
}

func (c *orderedIPTestCache) ReleaseAccountSlot(_ context.Context, id int64, req string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.leases[id], req)
	return nil
}

func newOrderedIPTestService() (*OpenAIGatewayService, *orderedIPTestRepo, *orderedIPTestCache) {
	repo := &orderedIPTestRepo{disabled: map[int64]bool{}}
	for id := int64(301); id <= 303; id++ {
		proxyID := id + 1000
		repo.accounts = append(repo.accounts, Account{ID: id, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 2, ProxyID: &proxyID, Proxy: &Proxy{ID: proxyID, Status: StatusActive, Protocol: "http", Host: "127.0.0.1", Port: 8080}, Credentials: map[string]any{"access_token": "local-test-token"}})
	}
	cache := &orderedIPTestCache{leases: map[int64]map[string]bool{}}
	cfg := &config.Config{}
	cfg.Gateway.Scheduling.LoadBatchEnabled = true
	cfg.Gateway.Scheduling.FallbackWaitTimeout = time.Second
	cfg.Gateway.Scheduling.FallbackMaxWaiting = 10
	cfg.Gateway.OpenAIWS.LBTopK = 1
	svc := &OpenAIGatewayService{accountRepo: repo, concurrencyService: NewConcurrencyService(cache), cfg: cfg, cache: &schedulerTestGatewayCache{sessionBindings: map[string]int64{}}}
	return svc, repo, cache
}

func TestOrderedIPSelectionAllSchedulersFillBeforeOverflow(t *testing.T) {
	for _, mode := range []string{"advanced", "legacy-batch", "legacy-no-batch"} {
		t.Run(mode, func(t *testing.T) {
			svc, _, cache := newOrderedIPTestService()
			ctx := context.Background()
			scheduler := newDefaultOpenAIAccountScheduler(svc, nil)
			svc.cfg.Gateway.Scheduling.LoadBatchEnabled = mode != "legacy-no-batch"
			var releases []func()
			for i, want := range []int64{301, 301, 302, 302, 303, 303} {
				var selection *AccountSelectionResult
				var err error
				if mode == "advanced" {
					selection, _, err = scheduler.Select(ctx, OpenAIAccountScheduleRequest{Platform: PlatformOpenAI, RequestedModel: "gpt-6-astra"})
				} else {
					selection, err = svc.SelectAccountWithLoadAwareness(ctx, nil, "", "gpt-6-astra", nil)
				}
				require.NoError(t, err, "request %d", i)
				require.NotNil(t, selection)
				require.True(t, selection.Acquired)
				require.Equal(t, want, selection.Account.ID)
				releases = append(releases, selection.ReleaseFunc)
			}
			require.Empty(t, cache.physicalCalls, "IP families must never reserve a random physical channel first")
			for _, release := range releases {
				release()
			}
		})
	}
}

func TestOrderedIPSelectionExpandsTopKAndLogicalSticky(t *testing.T) {
	svc, _, cache := newOrderedIPTestService()
	// Cached load says the first two channels are full; only the third can
	// appear in TopK. The atomic reservation still sees and chooses IP1.
	cache.loadMap = map[int64]*AccountLoadInfo{301: {AccountID: 301, CurrentConcurrency: 2, LoadRate: 100}, 302: {AccountID: 302, CurrentConcurrency: 2, LoadRate: 100}, 303: {AccountID: 303}}
	scheduler := newDefaultOpenAIAccountScheduler(svc, nil)
	selection, _, err := scheduler.Select(context.Background(), OpenAIAccountScheduleRequest{Platform: PlatformOpenAI, RequestedModel: "gpt-6-astra"})
	require.NoError(t, err)
	require.Equal(t, int64(301), selection.Account.ID)
	selection.ReleaseFunc()
	selectionCache, ok := svc.cache.(*schedulerTestGatewayCache)
	require.True(t, ok)
	selectionCache.sessionBindings["sticky"] = 303
	selection, decision, err := scheduler.Select(context.Background(), OpenAIAccountScheduleRequest{Platform: PlatformOpenAI, RequestedModel: "gpt-6-astra", SessionHash: "sticky", StickyAccountID: 303})
	require.NoError(t, err)
	require.Equal(t, openAIAccountScheduleLayerLoadBalance, decision.Layer)
	require.Equal(t, int64(301), selection.Account.ID)
	require.Equal(t, int64(301), selectionCache.sessionBindings[svc.openAISessionCacheKey("sticky")])
	selection.ReleaseFunc()
}

func TestOrderedIPSelectionEligibilityPriorityAndQueuedRefresh(t *testing.T) {
	svc, repo, cache := newOrderedIPTestService()
	ctx := svc.withOrderedIPChannelSelection(context.Background(), OpenAIAccountScheduleRequest{Platform: PlatformOpenAI, RequestedModel: "gpt-6-astra"})
	repo.accounts[0].Priority = 10
	repo.accounts[1].Priority = 10
	result, err := svc.tryAcquireAccountSlot(ctx, 301, 2)
	require.NoError(t, err)
	require.Equal(t, int64(303), result.SelectedAccount.ID)
	result.ReleaseFunc()
	repo.disabled[303] = true
	until := time.Now().Add(time.Hour)
	repo.accounts[0].RateLimitResetAt = &until
	result, err = svc.tryAcquireAccountSlot(ctx, 301, 2)
	require.NoError(t, err)
	require.Equal(t, int64(302), result.SelectedAccount.ID)
	result.ReleaseFunc()
	repo.accounts[0].RateLimitResetAt = nil
	for _, id := range []int64{301, 302} {
		cache.leases[id] = map[string]bool{"a": true, "b": true}
	}
	selection := attachSelectionProfitGate(ctx, &AccountSelectionResult{Account: &repo.accounts[1], WaitPlan: &AccountWaitPlan{AccountID: 302, MaxConcurrency: 2}})
	result, err = selection.TryAcquireIPChannelSlot(context.Background())
	require.NoError(t, err)
	require.False(t, result.Acquired)
	delete(cache.leases[301], "a")
	result, err = selection.TryAcquireIPChannelSlot(context.Background())
	require.NoError(t, err)
	require.True(t, result.Acquired)
	require.Equal(t, int64(301), selection.Account.ID, "wait retries must choose the now-free earlier IP")
	result.ReleaseFunc()
	// A pause applied during the wait must be loaded, not retained in a plan.
	repo.disabled[301] = true
	delete(cache.leases[302], "a")
	result, err = selection.TryAcquireIPChannelSlot(context.Background())
	require.NoError(t, err)
	require.True(t, result.Acquired)
	require.Equal(t, int64(302), selection.Account.ID)
	result.ReleaseFunc()
}

func TestOrderedIPSelectionExclusionsAndPhysicalContinuation(t *testing.T) {
	svc, repo, cache := newOrderedIPTestService()
	ctx := svc.withOrderedIPChannelSelection(context.Background(), OpenAIAccountScheduleRequest{Platform: PlatformOpenAI, RequestedModel: "gpt-6-astra", ExcludedIDs: map[int64]struct{}{302: {}}})
	result, err := svc.tryAcquireAccountSlot(ctx, 301, 2)
	require.NoError(t, err)
	require.False(t, result.Acquired)
	require.Empty(t, cache.orders, "failed logical-account exclusions must not be bypassed")
	// A fixed continuation/established WS turn explicitly keeps its physical
	// channel even when an earlier channel is free. Fresh selection never does.
	ctx = withoutOrderedIPChannelSelection(ctx)
	result, err = svc.tryAcquireAccountSlot(ctx, 303, 2)
	require.NoError(t, err)
	require.True(t, result.Acquired)
	require.Equal(t, []int64{303}, cache.physicalCalls)
	selection := attachSelectionProfitGate(ctx, &AccountSelectionResult{Account: &repo.accounts[2]})
	require.Nil(t, selection.ipChannelSelection)
	result.ReleaseFunc()
}

func TestOrderedIPSelectionSkipsUnavailableChannels(t *testing.T) {
	for _, state := range []string{"paused", "quarantined", "proxy-expired", "ticket-missing", "model-incompatible", "group-incompatible"} {
		t.Run(state, func(t *testing.T) {
			svc, repo, _ := newOrderedIPTestService()
			a := &repo.accounts[0]
			switch state {
			case "paused":
				a.Schedulable = false
			case "quarantined":
				a.Extra = map[string]any{AccountModelMismatchExtraKey: map[string]any{"expected_model": "gpt-6-astra", "actual_model": "gpt-5.6-luna", "quarantined": true}}
			case "proxy-expired":
				expired := time.Now().Add(-time.Minute)
				a.Proxy.ExpiresAt = &expired
			case "ticket-missing":
				svc.cfg.Gateway.OpenAICodexTicket.Enabled = true
				a.Extra = map[string]any{codexAccountTicketConfigKey: map[string]any{"enabled": true, "ticket_plan": "pro", "model": "gpt-6-astra", "revision": "test"}}
			case "model-incompatible":
				a.Credentials["model_mapping"] = map[string]any{"other-model": "other-model"}
			case "group-incompatible":
				a.GroupIDs = []int64{999}
			}
			ctx := svc.withOrderedIPChannelSelection(context.Background(), OpenAIAccountScheduleRequest{Platform: PlatformOpenAI, RequestedModel: "gpt-6-astra"})
			result, err := svc.tryAcquireAccountSlot(ctx, 303, 2)
			require.NoError(t, err)
			require.True(t, result.Acquired)
			require.Equal(t, int64(302), result.SelectedAccount.ID)
			result.ReleaseFunc()
		})
	}
}

func TestOrderedIPSelectionPreviousResponsePinsOnlyWhenNecessary(t *testing.T) {
	for _, canMove := range []bool{false, true} {
		svc, repo, cache := newOrderedIPTestService()
		svc.cfg = newSchedulerTestOpenAIWSV2Config()
		for i := range repo.accounts {
			repo.accounts[i].Extra = map[string]any{"openai_oauth_responses_websockets_v2_enabled": true}
		}
		require.NoError(t, svc.getOpenAIWSStateStore().BindResponseAccount(context.Background(), 0, "resp_ip_ordered", 303, time.Hour))
		selection, decision, err := newDefaultOpenAIAccountScheduler(svc, nil).Select(context.Background(), OpenAIAccountScheduleRequest{Platform: PlatformOpenAI, RequestedModel: "gpt-6-astra", PreviousResponseID: "resp_ip_ordered", PreviousResponseCanMove: canMove})
		require.NoError(t, err)
		if canMove {
			require.Equal(t, openAIAccountScheduleLayerLoadBalance, decision.Layer)
			require.Equal(t, int64(301), selection.Account.ID)
			require.Empty(t, cache.physicalCalls)
		} else {
			require.Equal(t, openAIAccountScheduleLayerPreviousResponse, decision.Layer)
			require.Equal(t, int64(303), selection.Account.ID)
			require.Nil(t, selection.ipChannelSelection, "the physically pinned continuation must remain pinned while queued")
			require.Equal(t, []int64{303}, cache.physicalCalls)
		}
		selection.ReleaseFunc()
	}
}

func TestOrderedIPSelectionStickyCanUseSiblingWhenBoundIPPaused(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		svc, repo, _ := newOrderedIPTestService()
		repo.accounts[2].Schedulable = false
		selectionCache, ok := svc.cache.(*schedulerTestGatewayCache)
		require.True(t, ok)
		selectionCache.sessionBindings[svc.openAISessionCacheKey("paused-sticky")] = 303
		var selection *AccountSelectionResult
		var err error
		if legacy {
			selection, err = svc.SelectAccountWithLoadAwareness(context.Background(), nil, "paused-sticky", "gpt-6-astra", nil)
		} else {
			selection, _, err = newDefaultOpenAIAccountScheduler(svc, nil).Select(context.Background(), OpenAIAccountScheduleRequest{Platform: PlatformOpenAI, RequestedModel: "gpt-6-astra", SessionHash: "paused-sticky", StickyAccountID: 303})
		}
		require.NoError(t, err)
		require.Equal(t, int64(301), selection.Account.ID)
		selection.ReleaseFunc()
	}
}

func TestOrderedIPSelectionKeepsFreshFixedProxyAfterReservation(t *testing.T) {
	svc, repo, _ := newOrderedIPTestService()
	ctx := svc.withOrderedIPChannelSelection(context.Background(), OpenAIAccountScheduleRequest{Platform: PlatformOpenAI, RequestedModel: "gpt-6-astra"})
	result, err := svc.tryAcquireAccountSlot(ctx, 303, 2)
	require.NoError(t, err)
	stale := repo.accounts[0]
	oldProxyID := int64(999)
	stale.ProxyID = &oldProxyID
	svc.schedulerSnapshot = &SchedulerSnapshotService{cache: &openAISnapshotCacheStub{accountsByID: map[int64]*Account{301: &stale}}}
	selection, err := svc.newAcquiredSelectionResult(ctx, result.SelectedAccount, result.ReleaseFunc)
	require.NoError(t, err)
	require.Equal(t, *repo.accounts[0].ProxyID, *selection.Account.ProxyID)
	selection.ReleaseFunc()
	selection.Acquired = false
	selection.WaitPlan = &AccountWaitPlan{AccountID: 301, MaxConcurrency: 2}
	result, err = selection.TryAcquireIPChannelSlot(context.Background())
	require.NoError(t, err)
	require.True(t, result.Acquired)
	require.Equal(t, *repo.accounts[0].ProxyID, *selection.Account.ProxyID)
	result.ReleaseFunc()
}
