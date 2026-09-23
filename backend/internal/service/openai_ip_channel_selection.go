package service

import (
	"context"
	"sort"
	"sync"
	"time"
)

type openAIIPChannelSelectionKey struct{}

type openAIIPChannelSelectionScope struct {
	service    *OpenAIGatewayService
	req        OpenAIAccountScheduleRequest
	mu         sync.Mutex
	membership map[int64]bool
	balanced   bool
}

type openAIIPChannelReader interface {
	GetAccountIPChannels(context.Context, []int64) (map[int64][]AccountIPChannel, error)
}

func (s *OpenAIGatewayService) withOrderedIPChannelSelection(ctx context.Context, req OpenAIAccountScheduleRequest) context.Context {
	ctx = withCodexTicketSelectionCache(ctx)
	if s == nil || NormalizeOpenAICompatiblePlatform(req.Platform) != PlatformOpenAI {
		return ctx
	}
	if _, ok := s.accountRepo.(openAIIPChannelReader); !ok {
		return ctx
	}
	// The scheduler-disabled path must retain the outer transport/image gates.
	if scope, _ := ctx.Value(openAIIPChannelSelectionKey{}).(*openAIIPChannelSelectionScope); scope != nil {
		return ctx
	}
	req.ExcludedIDs = cloneExcludedAccountIDs(req.ExcludedIDs)
	return context.WithValue(ctx, openAIIPChannelSelectionKey{}, &openAIIPChannelSelectionScope{service: s, req: req, membership: map[int64]bool{}})
}

func withoutOrderedIPChannelSelection(ctx context.Context) context.Context {
	return context.WithValue(ctx, openAIIPChannelSelectionKey{}, (*openAIIPChannelSelectionScope)(nil))
}

func (scope *openAIIPChannelSelectionScope) isKnownChannel(id int64) bool {
	if scope == nil {
		return false
	}
	scope.mu.Lock()
	defer scope.mu.Unlock()
	return scope.membership[id]
}

// acquire expands the chosen logical account before reserving a slot. All its
// channels participate even when the load balancer's TopK contained just one.
// Membership and eligibility are reloaded on every queued attempt, so a pause,
// expired ticket or removed proxy cannot remain eligible throughout a wait.
func (scope *openAIIPChannelSelectionScope) acquire(ctx context.Context, id int64) (*AcquireResult, bool, error) {
	ctx = withCodexTicketSelectionCache(ctx)
	scope.mu.Lock()
	wasChannel, known := scope.membership[id]
	scope.mu.Unlock()
	if known && !wasChannel {
		return nil, false, nil
	}
	ctx = scope.service.withOpenAIQuotaAutoPauseContext(ctx)
	groups, err := scope.service.accountRepo.(openAIIPChannelReader).GetAccountIPChannels(ctx, []int64{id})
	if err != nil {
		return nil, true, err
	}
	members := groups[id]
	scope.mu.Lock()
	if len(members) > 0 {
		for _, member := range members {
			if member.Account != nil {
				scope.membership[member.Account.ID] = true
			}
		}
	} else if !known {
		scope.membership[id] = false
	}
	scope.mu.Unlock()
	if len(members) == 0 {
		if wasChannel {
			return &AcquireResult{}, true, nil
		}
		return nil, false, nil
	}
	s := scope.service
	req := scope.req
	// One failed upstream attempt excludes the complete logical account. A
	// retry must not walk its other IPs, including through a sticky shortcut.
	for _, member := range members {
		if member.Account != nil {
			if _, excluded := req.ExcludedIDs[member.Account.ID]; excluded {
				return &AcquireResult{}, true, nil
			}
		}
	}
	checker := &defaultOpenAIAccountScheduler{service: s}
	accounts := make([]*Account, 0, len(members))
	for _, member := range members {
		a := member.Account
		if a == nil || !member.Enabled || !member.LogicalEnabled || !a.IsSchedulableWithContext(ctx) {
			continue
		}
		if _, excluded := req.ExcludedIDs[a.ID]; excluded {
			continue
		}
		if a.ProxyID == nil || a.Proxy == nil || !a.Proxy.IsActive() || a.Proxy.IsExpired(time.Now()) {
			continue
		}
		// Channel expansion already returned full, current database accounts.
		// Reuse them instead of doing another ticket lookup for each fixed IP.
		cacheCodexTicketSelectionAccount(ctx, a, a, nil)
		if !s.openAIAccountMatchesSchedulingGroup(a, req.GroupID) ||
			!isOpenAICompatibleAccountEligibleForRequest(ctx, a, req.Platform, req.RequestedModel, req.RequireCompact, req.RequiredCapability) ||
			s.isOpenAIAccountBlockedBySchedulingThreshold(ctx, a) ||
			!checker.isAccountRequestCompatible(ctx, a, req) || !checker.isAccountTransportCompatible(a, req.RequiredTransport) {
			continue
		}
		accounts = append(accounts, a)
	}
	sort.Slice(accounts, func(i, j int) bool {
		if accounts[i].Priority != accounts[j].Priority {
			return accounts[i].Priority < accounts[j].Priority
		}
		return accounts[i].ID < accounts[j].ID
	})
	if len(accounts) == 0 {
		return &AcquireResult{}, true, nil
	}
	limits := make([]AccountWithConcurrency, 0, len(accounts))
	for _, a := range accounts {
		limits = append(limits, AccountWithConcurrency{ID: a.ID, MaxConcurrency: a.Mode1EffectiveConcurrency()})
	}
	selectedID, result, err := s.concurrencyService.AcquireOrderedAccountSlot(ctx, limits)
	if err != nil || result == nil || !result.Acquired {
		if err == nil && result != nil {
			result.SelectedAccount = accounts[0]
		}
		return result, true, err
	}
	for _, a := range accounts {
		if a.ID == selectedID {
			result.SelectedAccount = a
			break
		}
	}
	return result, true, nil
}

func (s *defaultOpenAIAccountScheduler) selectOrderedIPSticky(ctx context.Context, req OpenAIAccountScheduleRequest, id int64) (*AccountSelectionResult, bool, bool, error) {
	scope, _ := ctx.Value(openAIIPChannelSelectionKey{}).(*openAIIPChannelSelectionScope)
	if scope == nil {
		return nil, false, false, nil
	}
	result, handled, err := scope.acquire(ctx, id)
	if err != nil || !handled {
		return nil, handled, false, err
	}
	if result == nil || result.SelectedAccount == nil {
		return nil, true, false, nil
	}
	account := result.SelectedAccount
	escape := s.service.openAIStickyEscapeConfig()
	_, _, _, unhealthy := s.shouldEscapeStickyAccount(account.ID, escape)
	if unhealthy || (escape.enabled && !result.Acquired) {
		if result.ReleaseFunc != nil {
			result.ReleaseFunc()
		}
		return nil, true, true, nil
	}
	selection := &AccountSelectionResult{Account: account, Acquired: result.Acquired, ReleaseFunc: result.ReleaseFunc}
	if result.Acquired {
		if !req.PreserveStickyBinding {
			_ = s.service.bindOpenAIStickySessionDuringSelection(ctx, req.GroupID, req.SessionHash, account.ID)
		}
	} else {
		cfg := s.service.schedulingConfig()
		selection.WaitPlan = &AccountWaitPlan{AccountID: account.ID, MaxConcurrency: account.Mode1EffectiveConcurrency(), Timeout: cfg.StickySessionWaitTimeout, MaxWaiting: cfg.StickySessionMaxWaiting}
	}
	return attachSelectionProfitGate(ctx, selection), true, false, nil
}

func acquiredOpenAIAccount(result *AcquireResult, fallback *Account) *Account {
	if result != nil && result.SelectedAccount != nil {
		return result.SelectedAccount
	}
	return fallback
}

// TryAcquireIPChannelSlot is used by handler fast retries and queue waits.
// nil means this is an ordinary account or a physically pinned continuation.
// A successful result can choose a different channel than the original wait
// placeholder; the caller must use selection.Account after admission.
func (r *AccountSelectionResult) TryAcquireIPChannelSlot(ctx context.Context) (*AcquireResult, error) {
	if r == nil || r.Account == nil || r.ipChannelSelection == nil {
		return nil, nil
	}
	ctx = ContextWithSelectionProfitGate(ctx, r)
	// A previously standalone account can be enrolled while waiting. Cache
	// misses only for a scheduler pass, never across a queued admission retry.
	r.ipChannelSelection.mu.Lock()
	if !r.ipChannelSelection.membership[r.Account.ID] {
		delete(r.ipChannelSelection.membership, r.Account.ID)
	}
	r.ipChannelSelection.mu.Unlock()
	var result *AcquireResult
	var handled bool
	var err error
	if r.ipChannelSelection.balanced {
		result, _, handled, err = r.ipChannelSelection.acquireBalanced(ctx)
	} else {
		result, handled, err = r.ipChannelSelection.acquire(ctx, r.Account.ID)
	}
	if err != nil || !handled || result == nil || !result.Acquired {
		return result, err
	}
	// GetAccountIPChannels returned complete, fresh database accounts. Do not
	// overwrite their fixed proxy with a possibly older scheduler projection.
	r.Account = acquiredOpenAIAccount(result, r.Account)
	if err = rememberAccountIPRequestBinding(ctx, r.ipChannelSelection.service.accountRepo, r); err != nil {
		result.ReleaseFunc()
		return nil, err
	}
	return result, nil
}
