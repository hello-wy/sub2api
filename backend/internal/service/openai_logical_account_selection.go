package service

import (
	"context"
	"sort"
	"time"
)

// selectBalancedLogicalAccounts is shared by the advanced and legacy entry
// points, before ordinary session affinity. Fixed response continuations and
// established live connections keep their separate physical binding path.
func (s *OpenAIGatewayService) selectBalancedLogicalAccounts(ctx context.Context, req OpenAIAccountScheduleRequest) (*AccountSelectionResult, int, bool, error) {
	scope, _ := ctx.Value(openAIIPChannelSelectionKey{}).(*openAIIPChannelSelectionScope)
	if scope == nil || s.concurrencyService == nil {
		return nil, 0, false, nil
	}
	req.ExcludedIDs = cloneExcludedAccountIDs(req.ExcludedIDs)
	scope.req = req
	result, count, handled, err := scope.acquireBalanced(ctx)
	if err != nil || !handled {
		return nil, count, handled, err
	}
	if result == nil || result.SelectedAccount == nil {
		return nil, count, true, noAvailableOpenAISelectionError(req.RequestedModel, req.RequireCompact, "no eligible logical accounts")
	}
	selection := attachSelectionProfitGate(ctx, &AccountSelectionResult{Account: result.SelectedAccount, Acquired: result.Acquired, ReleaseFunc: result.ReleaseFunc})
	if result.Acquired {
		if req.SessionHash != "" && !req.PreserveStickyBinding {
			_ = s.bindOpenAIStickySessionDuringSelection(ctx, req.GroupID, req.SessionHash, result.SelectedAccount.ID)
		}
	} else {
		cfg := s.schedulingConfig()
		selection.WaitPlan = &AccountWaitPlan{AccountID: result.SelectedAccount.ID, MaxConcurrency: result.SelectedAccount.Mode1EffectiveConcurrency(), Timeout: cfg.FallbackWaitTimeout, MaxWaiting: cfg.FallbackMaxWaiting}
	}
	return selection, count, true, nil
}

func (scope *openAIIPChannelSelectionScope) acquireBalanced(ctx context.Context) (*AcquireResult, int, bool, error) {
	s, req := scope.service, scope.req
	ctx = withCodexTicketSelectionCache(s.withOpenAIQuotaAutoPauseContext(ctx))
	accounts, err := s.listSchedulableAccounts(ctx, req.GroupID, req.Platform)
	if err != nil {
		return nil, 0, true, err
	}
	ids := make([]int64, 0, len(accounts))
	for _, a := range accounts {
		ids = append(ids, a.ID)
	}
	families, err := s.accountRepo.(openAIIPChannelReader).GetAccountIPChannels(ctx, ids)
	if err != nil {
		return nil, 0, true, err
	}
	// Preserve existing scheduling for installations without any IP families.
	// Once a queued request joined this pool, it keeps rebalancing if channels
	// are removed while it waits, including the resulting standalone accounts.
	hasFamilies := false
	for _, members := range families {
		if len(members) > 0 {
			hasFamilies = true
			break
		}
	}
	if !hasFamilies && !scope.balanced {
		return nil, 0, false, nil
	}
	scope.balanced = true
	// Families are already hydrated in one repository batch. Hydrate ordinary
	// accounts in one batch too; a mixed pool must not add a DB round trip per
	// standalone candidate merely because one account has multiple IPs.
	var standalone map[int64]*Account
	if s.schedulerSnapshot != nil {
		standaloneIDs := make([]int64, 0, len(accounts))
		for _, a := range accounts {
			if len(families[a.ID]) == 0 {
				standaloneIDs = append(standaloneIDs, a.ID)
			}
		}
		standalone = make(map[int64]*Account, len(standaloneIDs))
		if len(standaloneIDs) > 0 {
			fresh, loadErr := s.accountRepo.GetByIDs(ctx, standaloneIDs)
			if loadErr != nil {
				return nil, 0, true, loadErr
			}
			for _, a := range fresh {
				if a != nil {
					standalone[a.ID] = a
				}
			}
		}
	}
	checker := &defaultOpenAIAccountScheduler{service: s}
	groups := make([]LogicalAccountConcurrency, 0, len(accounts))
	byID := map[int64]*Account{}
	seen := map[int64]bool{}
	for i := range accounts {
		a := &accounts[i]
		members := families[a.ID]
		logicalID := a.ID
		if len(members) > 0 {
			logicalID = members[0].LogicalAccountID
		}
		if seen[logicalID] {
			continue
		}
		seen[logicalID] = true
		group := LogicalAccountConcurrency{ID: logicalID}
		eligible := []*Account{}
		excluded := false
		if len(members) == 0 {
			// A standalone account has the same one vote as a ten-IP account.
			members = []AccountIPChannel{{Account: a, LogicalAccountID: a.ID, Enabled: true, LogicalEnabled: true}}
		} else {
			scope.mu.Lock()
			for _, member := range members {
				if member.Account != nil {
					scope.membership[member.Account.ID] = true
				}
			}
			scope.mu.Unlock()
		}
		for _, member := range members {
			candidate := member.Account
			if candidate == nil {
				continue
			}
			group.LoadAccountIDs = append(group.LoadAccountIDs, candidate.ID)
			if _, ok := req.ExcludedIDs[candidate.ID]; ok {
				excluded = true
			}
			if !member.Enabled || !member.LogicalEnabled {
				continue
			}
			if len(families[a.ID]) > 0 {
				if candidate.ProxyID == nil || candidate.Proxy == nil || !candidate.Proxy.IsActive() || candidate.Proxy.IsExpired(time.Now()) {
					continue
				}
			} else if standalone != nil {
				candidate = standalone[candidate.ID]
				if candidate == nil {
					continue
				}
			}
			cacheCodexTicketSelectionAccount(ctx, candidate, candidate, nil)
			if !s.openAIAccountMatchesSchedulingGroup(candidate, req.GroupID) ||
				!isOpenAICompatibleAccountEligibleForRequest(ctx, candidate, req.Platform, req.RequestedModel, req.RequireCompact, req.RequiredCapability) ||
				s.isOpenAIAccountBlockedBySchedulingThreshold(ctx, candidate) ||
				!checker.isAccountRequestCompatible(ctx, candidate, req) || !checker.isAccountTransportCompatible(candidate, req.RequiredTransport) {
				continue
			}
			eligible = append(eligible, candidate)
		}
		if excluded || len(eligible) == 0 {
			continue
		}
		// accounts.priority is the existing IP order control, never an outer
		// account weight. Import order and number of IPs cannot bias the pool.
		sort.Slice(eligible, func(i, j int) bool {
			if eligible[i].Priority != eligible[j].Priority {
				return eligible[i].Priority < eligible[j].Priority
			}
			return eligible[i].ID < eligible[j].ID
		})
		for _, candidate := range eligible {
			group.Channels = append(group.Channels, AccountWithConcurrency{ID: candidate.ID, MaxConcurrency: candidate.Mode1EffectiveConcurrency()})
			byID[candidate.ID] = candidate
		}
		groups = append(groups, group)
	}
	if len(groups) == 0 {
		return &AcquireResult{}, 0, true, nil
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].ID < groups[j].ID })
	id, result, err := s.concurrencyService.AcquireBalancedLogicalAccountSlot(ctx, groups)
	if result != nil {
		result.SelectedAccount = byID[id]
		if result.Acquired && result.SelectedAccount != nil && result.SelectedAccount.IsRandomProxy() {
			if resolveErr := ResolveRandomProxyFromSource(ctx, result.SelectedAccount, s.accountRepo); resolveErr != nil {
				result.ReleaseFunc()
				return nil, len(groups), true, resolveErr
			}
		}
	}
	return result, len(groups), true, err
}
