package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

// IP channels retain their original account IDs: scheduler leases, request logs,
// and OAuth rotation coordination remain attached to the same durable records.
type AccountIPChannel struct {
	Account          *Account
	LogicalAccountID int64
	Enabled          bool
	DisabledAt       *time.Time
	LogicalEnabled   bool
}

type AccountIPChannelPatch struct {
	ProxyID     *int64 `json:"proxy_id"`
	Concurrency *int   `json:"concurrency"`
	Priority    *int   `json:"priority"`
	LoadFactor  *int   `json:"load_factor"`
	Schedulable *bool  `json:"schedulable"`
}

type AccountIPChannelRepository interface {
	ListIPChannelRoutingAccounts(context.Context, string, string, string, string, int64, string) ([]Account, error)
	AccountIPChannelHistoryIDs(context.Context, []int64) (map[int64][]int64, error)
	ListLogicalAccounts(context.Context, pagination.PaginationParams, string, string, string, string, int64, string) ([]Account, *pagination.PaginationResult, error)
	GetAccountIPChannels(context.Context, []int64) (map[int64][]AccountIPChannel, error)
	JoinAccountIPChannels(context.Context, []int64) (int64, error)
	AddAccountIPChannels(context.Context, int64, []int64, *int, *int) error
	PatchAccountIPChannel(context.Context, int64, int64, AccountIPChannelPatch) error
	RemoveAccountIPChannel(context.Context, int64, int64) error
	SetLogicalAccountSchedulable(context.Context, int64, bool) (bool, error)
	WithAccountIPChannelTransaction(context.Context, int64, func(context.Context, AccountRepository, []AccountIPChannel) error) error
}

type AccountIPChannelAdmin interface {
	UpdateAccountIPImport(context.Context, int64, *UpdateAccountInput) (*Account, error)
	AccountIPChannelHistoryIDs(context.Context, []int64) (map[int64][]int64, error)
	GetAccountIPChannels(context.Context, []int64) (map[int64][]AccountIPChannel, error)
	JoinAccountIPChannels(context.Context, []int64) (int64, error)
	AddAccountIPChannels(context.Context, int64, []int64, *int, *int) error
	PatchAccountIPChannel(context.Context, int64, int64, AccountIPChannelPatch) error
	RemoveAccountIPChannel(context.Context, int64, int64) error
	ListRoutingAccounts(context.Context, string, string, string, string, int64, string) ([]Account, error)
}

func (s *adminServiceImpl) GetAccountIPChannels(ctx context.Context, ids []int64) (map[int64][]AccountIPChannel, error) {
	if r, ok := s.accountRepo.(AccountIPChannelRepository); ok {
		return r.GetAccountIPChannels(ctx, ids)
	}
	return map[int64][]AccountIPChannel{}, nil
}

func (s *adminServiceImpl) AccountIPChannelHistoryIDs(ctx context.Context, ids []int64) (map[int64][]int64, error) {
	if r, ok := s.accountRepo.(AccountIPChannelRepository); ok {
		return r.AccountIPChannelHistoryIDs(ctx, ids)
	}
	result := map[int64][]int64{}
	for _, id := range ids {
		result[id] = []int64{id}
	}
	return result, nil
}

func (s *adminServiceImpl) ListRoutingAccounts(ctx context.Context, platform, typ, status, search string, groupID int64, privacy string) ([]Account, error) {
	if r, ok := s.accountRepo.(AccountIPChannelRepository); ok {
		return r.ListIPChannelRoutingAccounts(ctx, platform, typ, status, search, groupID, privacy)
	}
	return s.accountRepo.ListAllWithFilters(ctx, platform, typ, status, search, groupID, privacy)
}

func (s *adminServiceImpl) JoinAccountIPChannels(ctx context.Context, ids []int64) (int64, error) {
	r, ok := s.accountRepo.(AccountIPChannelRepository)
	if !ok {
		return 0, errors.New("IP channel repository unavailable")
	}
	return r.JoinAccountIPChannels(ctx, ids)
}

func (s *adminServiceImpl) AddAccountIPChannels(ctx context.Context, id int64, proxyIDs []int64, concurrency, priority *int) error {
	if len(proxyIDs) == 0 || len(proxyIDs) > 50 {
		return infraerrors.BadRequest("IP_CHANNEL_LIMIT", "请选择 1 至 50 个固定 IP")
	}
	if concurrency != nil && (*concurrency < 0 || *concurrency > 10000) || priority != nil && *priority < 0 {
		return infraerrors.BadRequest("IP_CHANNEL_SETTINGS", "并发和优先级不能为负数")
	}
	for _, proxyID := range proxyIDs {
		if proxyID <= 0 {
			return infraerrors.BadRequest("IP_CHANNEL_PROXY", "IP 通道必须使用固定代理")
		}
		p, err := s.proxyRepo.GetByID(ctx, proxyID)
		if err != nil {
			return err
		}
		if !p.IsActive() || p.IsExpired(time.Now()) {
			return infraerrors.BadRequest("IP_CHANNEL_PROXY", "代理已停用或已过期")
		}
	}
	r, ok := s.accountRepo.(AccountIPChannelRepository)
	if !ok {
		return errors.New("IP channel repository unavailable")
	}
	account, err := s.accountRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	defaults, err := s.resolveCodexTicketGroupDefaults(ctx, account.GroupIDs)
	if err != nil {
		return err
	}
	ticketRepo, canInherit := s.accountRepo.(AccountIPChannelTicketDefaultsRepository)
	if defaults != nil && defaults.Enabled && !canInherit {
		return errors.New("IP channel STATE defaults repository unavailable")
	}
	if account.GetCredential("refresh_token") != "" && account.GetCredential(OpenAIOAuthCredentialGroupKey) == "" {
		if _, err = s.RegisterOpenAIOAuthCredentialGroup(ctx, []int64{id}, account.Credentials); err != nil {
			return err
		}
	}
	if defaults != nil && defaults.Enabled {
		createdIDs, err := ticketRepo.AddAccountIPChannelsWithTicketDefaults(ctx, id, proxyIDs, concurrency, priority, defaults)
		if err != nil {
			return err
		}
		for _, createdID := range createdIDs {
			s.scheduleInheritedCodexTicket(ctx, createdID)
		}
		return nil
	}
	return r.AddAccountIPChannels(ctx, id, proxyIDs, concurrency, priority)
}

func (s *adminServiceImpl) PatchAccountIPChannel(ctx context.Context, id, channelID int64, patch AccountIPChannelPatch) error {
	if patch.Concurrency != nil && (*patch.Concurrency < 0 || *patch.Concurrency > 10000) || patch.Priority != nil && *patch.Priority < 0 {
		return infraerrors.BadRequest("IP_CHANNEL_SETTINGS", "并发和优先级不能为负数")
	}
	if patch.LoadFactor != nil && (*patch.LoadFactor < 0 || *patch.LoadFactor > 10000) {
		return infraerrors.BadRequest("IP_CHANNEL_SETTINGS", "调度权重须在 0 至 10000 之间")
	}
	if patch.ProxyID != nil {
		if *patch.ProxyID <= 0 {
			return infraerrors.BadRequest("IP_CHANNEL_PROXY", "IP 通道必须使用固定代理")
		}
		p, err := s.proxyRepo.GetByID(ctx, *patch.ProxyID)
		if err != nil {
			return err
		}
		if !p.IsActive() || p.IsExpired(time.Now()) {
			return infraerrors.BadRequest("IP_CHANNEL_PROXY", "代理已停用或已过期")
		}
	}
	r, ok := s.accountRepo.(AccountIPChannelRepository)
	if !ok {
		return errors.New("IP channel repository unavailable")
	}
	return r.PatchAccountIPChannel(ctx, id, channelID, patch)
}

func (s *adminServiceImpl) UpdateAccountIPImport(ctx context.Context, id int64, input *UpdateAccountInput) (*Account, error) {
	channels, err := s.GetAccountIPChannels(ctx, []int64{id})
	if err != nil {
		return nil, err
	}
	if len(channels[id]) == 0 {
		return s.UpdateAccount(ctx, id, input)
	}
	root := channels[id][0].LogicalAccountID
	for _, c := range channels[id] {
		if c.Account.ID == id && input.ProxyID != nil && (c.Account.ProxyID == nil || *input.ProxyID != *c.Account.ProxyID) {
			return nil, infraerrors.Conflict("IP_CHANNEL_FIXED_PROXY", "导入不能移动已有通道的固定 IP")
		}
	}
	shared := *input
	shared.ProxyID = nil
	shared.Concurrency = nil
	shared.Priority = nil
	shared.LoadFactor = nil
	if _, err = s.UpdateAccount(ctx, root, &shared); err != nil {
		return nil, err
	}
	if input.Concurrency != nil || input.Priority != nil || input.LoadFactor != nil {
		if err = s.PatchAccountIPChannel(ctx, root, id, AccountIPChannelPatch{Concurrency: input.Concurrency, Priority: input.Priority, LoadFactor: input.LoadFactor}); err != nil {
			return nil, err
		}
	}
	return s.GetAccount(ctx, id)
}

func (s *adminServiceImpl) RemoveAccountIPChannel(ctx context.Context, id, channelID int64) error {
	r, ok := s.accountRepo.(AccountIPChannelRepository)
	if !ok {
		return errors.New("IP channel repository unavailable")
	}
	return r.RemoveAccountIPChannel(ctx, id, channelID)
}

type accountIPChannelWriteKey struct{}

func isAccountIPChannelWrite(ctx context.Context) bool {
	v, _ := ctx.Value(accountIPChannelWriteKey{}).(bool)
	return v
}

func WithAccountIPChannelWrite(ctx context.Context) context.Context {
	return context.WithValue(ctx, accountIPChannelWriteKey{}, true)
}

// All configuration checks continue through the existing admin edit path, but
// the per-channel writes and group bindings commit in one database transaction.
func (s *adminServiceImpl) updateLogicalAccount(ctx context.Context, id int64, input *UpdateAccountInput) (*Account, bool, error) {
	if isAccountIPChannelWrite(ctx) {
		return nil, false, nil
	}
	r, ok := s.accountRepo.(AccountIPChannelRepository)
	if !ok {
		return nil, false, nil
	}
	channels, err := r.GetAccountIPChannels(ctx, []int64{id})
	if err != nil {
		return nil, true, err
	}
	if len(channels[id]) == 0 {
		return nil, false, nil
	}
	if channels[id][0].LogicalAccountID != id {
		return nil, true, infraerrors.BadRequest("IP_CHANNEL_USE_PARENT", "请从逻辑账号编辑共享配置")
	}
	if input.ProxyID != nil || (input.Concurrency != nil && !ProtectionManagedWrite(ctx)) || input.Priority != nil || input.LoadFactor != nil {
		// Older editors post the full account even when only its name or groups
		// changed. Ignore unchanged routing fields, but never fan a real routing
		// change out across the logical account's independent IP channels.
		current, err := s.accountRepo.GetByID(ctx, id)
		if err != nil {
			return nil, true, err
		}
		if current == nil {
			return nil, true, ErrAccountNotFound
		}
		clean := *input
		proxyID := int64(0)
		if current.ProxyID != nil {
			proxyID = *current.ProxyID
		}
		loadFactor := 0
		if current.LoadFactor != nil {
			loadFactor = *current.LoadFactor
		}
		if (input.ProxyID != nil && *input.ProxyID != proxyID) ||
			(input.Concurrency != nil && !ProtectionManagedWrite(ctx) && *input.Concurrency != current.Concurrency) ||
			(input.Priority != nil && *input.Priority != current.Priority) ||
			(input.LoadFactor != nil && *input.LoadFactor != loadFactor) {
			return nil, true, infraerrors.BadRequest("IP_CHANNEL_USE_ENDPOINT", "请在 IP 通道中编辑代理、并发和优先级")
		}
		clean.ProxyID, clean.Priority, clean.LoadFactor = nil, nil, nil
		if !ProtectionManagedWrite(ctx) {
			clean.Concurrency = nil
		}
		input = &clean
	}
	if (&Account{Extra: input.Extra}).IsRandomProxy() {
		return nil, true, infraerrors.BadRequest("IP_CHANNEL_FIXED_PROXY", "多 IP 通道使用固定代理")
	}
	if input.Type != "" && input.Type != AccountTypeOAuth {
		return nil, true, infraerrors.BadRequest("IP_CHANNEL_IMMUTABLE_TYPE", "请先移除 IP 通道再修改账号类型")
	}
	err = r.WithAccountIPChannelTransaction(ctx, id, func(txctx context.Context, repo AccountRepository, members []AccountIPChannel) error {
		copyService := *s
		copyService.accountRepo = repo
		copyService.accountBillingRepo, _ = repo.(AccountBillingSettingsRepository)
		rootPresent := false
		for _, member := range members {
			if member.Account.ID == id {
				rootPresent = true
			}
		}
		if !rootPresent {
			a, err := repo.GetByID(txctx, id)
			if err != nil {
				return err
			}
			members = append(members, AccountIPChannel{Account: a})
		}
		for _, member := range members {
			memberInput := *input
			memberCtx := WithAccountIPChannelWrite(txctx)
			if ProtectionManagedWrite(txctx) {
				memberInput.Concurrency = nil
				if expected, ok := GetProtectionWriteExpectation(txctx); ok && member.Account.ID != id {
					expected.AccountID = member.Account.ID
					expected.UpdatedAt = member.Account.UpdatedAt
					memberCtx = context.WithValue(memberCtx, protectionWriteExpectationKey{}, expected)
				}
			}
			if input.Extra != nil {
				memberInput.Extra = make(map[string]any, len(input.Extra))
				for k, v := range input.Extra {
					memberInput.Extra[k] = v
				}
				delete(memberInput.Extra, AccountTrafficPolicyKey)
				if v, ok := member.Account.Extra[AccountTrafficPolicyKey]; ok {
					memberInput.Extra[AccountTrafficPolicyKey] = v
				}
				// Provider observations and local quota counters are per routing
				// record. A stale parent edit must never fan them out to siblings.
				for k := range memberInput.Extra {
					if isIPChannelRuntimeExtra(k) {
						delete(memberInput.Extra, k)
					}
				}
				for k, v := range member.Account.Extra {
					if isIPChannelRuntimeExtra(k) {
						memberInput.Extra[k] = v
					}
				}
			}
			if _, err := copyService.UpdateAccount(memberCtx, member.Account.ID, &memberInput); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, true, err
	}
	updated, err := s.accountRepo.GetByID(ctx, id)
	return updated, true, err
}

func isIPChannelRuntimeExtra(k string) bool {
	return IsOpenAICodexTicketPrivateExtraKey(k) || k == "codex_fingerprint_seed" || k == "codex_usage_updated_at" || k == "model_rate_limits" || k == "session_window_utilization" || strings.HasPrefix(k, "codex_primary_") || strings.HasPrefix(k, "codex_secondary_") || strings.HasPrefix(k, "codex_5h_") || strings.HasPrefix(k, "codex_7d_") || strings.HasPrefix(k, "passive_usage_") || strings.HasPrefix(k, "proxy_fallback_") || k == "quota_used" || k == "quota_daily_used" || k == "quota_weekly_used" || strings.HasPrefix(k, "upstream_billing_probe") || strings.HasPrefix(k, "ollama_cloud_usage")
}

// A retry may use a different real account, but cannot multiply attempts by
// walking the fixed IPs of the same logical account after one of them failed.
func expandIPChannelExclusions(ctx context.Context, repo AccountRepository, excluded map[int64]struct{}) (map[int64]struct{}, error) {
	if len(excluded) == 0 {
		return excluded, nil
	}
	r, ok := repo.(AccountIPChannelRepository)
	if !ok {
		return excluded, nil
	}
	ids := make([]int64, 0, len(excluded))
	result := make(map[int64]struct{}, len(excluded))
	for id := range excluded {
		ids = append(ids, id)
		result[id] = struct{}{}
	}
	channels, err := r.GetAccountIPChannels(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, members := range channels {
		for _, c := range members {
			result[c.Account.ID] = struct{}{}
		}
	}
	return result, nil
}

type accountIPRequestBindingKey struct{}
type accountIPRequestBinding struct {
	mu       sync.Mutex
	excluded map[int64]struct{}
}

// Scope is exactly one HTTP request or WS turn, never an enduring session.
func withAccountIPRequestBinding(ctx context.Context) context.Context {
	return context.WithValue(ctx, accountIPRequestBindingKey{}, &accountIPRequestBinding{excluded: map[int64]struct{}{}})
}

func applyAccountIPRequestBinding(ctx context.Context, excluded map[int64]struct{}) map[int64]struct{} {
	b, ok := ctx.Value(accountIPRequestBindingKey{}).(*accountIPRequestBinding)
	if !ok {
		return excluded
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.excluded) == 0 {
		return excluded
	}
	result := make(map[int64]struct{}, len(excluded)+len(b.excluded))
	for id := range excluded {
		result[id] = struct{}{}
	}
	for id := range b.excluded {
		result[id] = struct{}{}
	}
	return result
}

func rememberAccountIPRequestBinding(ctx context.Context, repo AccountRepository, selection *AccountSelectionResult) error {
	b, ok := ctx.Value(accountIPRequestBindingKey{}).(*accountIPRequestBinding)
	if !ok || selection == nil || selection.Account == nil {
		return nil
	}
	r, ok := repo.(AccountIPChannelRepository)
	if !ok {
		return nil
	}
	id := selection.Account.ID
	if lightweight, ok := repo.(interface {
		AccountIPChannelSiblingIDs(context.Context, int64) ([]int64, error)
	}); ok {
		ids, err := lightweight.AccountIPChannelSiblingIDs(ctx, id)
		if err != nil {
			return err
		}
		b.mu.Lock()
		defer b.mu.Unlock()
		for _, memberID := range ids {
			if memberID != id {
				b.excluded[memberID] = struct{}{}
			}
		}
		return nil
	}
	members, err := r.GetAccountIPChannels(ctx, []int64{id})
	if err != nil {
		return err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, c := range members[id] {
		if c.Account.ID != id {
			b.excluded[c.Account.ID] = struct{}{}
		}
	}
	return nil
}

func LogicalAccountDisplayName(name string) string {
	if i := strings.LastIndex(name, " [IP #"); i >= 0 && strings.HasSuffix(name, "]") {
		return name[:i]
	}
	return name
}

func (s *adminServiceImpl) applyLogicalAccountRuntimeAction(ctx context.Context, id int64, action func(*adminServiceImpl, context.Context, int64) error) (bool, error) {
	if isAccountIPChannelWrite(ctx) {
		return false, nil
	}
	r, ok := s.accountRepo.(AccountIPChannelRepository)
	if !ok {
		return false, nil
	}
	channels, err := r.GetAccountIPChannels(ctx, []int64{id})
	if err != nil {
		return true, err
	}
	if len(channels[id]) == 0 || channels[id][0].LogicalAccountID != id {
		return false, nil
	}
	err = r.WithAccountIPChannelTransaction(ctx, id, func(ctx context.Context, repo AccountRepository, members []AccountIPChannel) error {
		copyService := *s
		copyService.accountRepo = repo
		for _, c := range members {
			if err := action(&copyService, WithAccountIPChannelWrite(ctx), c.Account.ID); err != nil {
				return err
			}
		}
		return nil
	})
	return true, err
}

func (s *adminServiceImpl) bulkUpdateLogicalAccounts(ctx context.Context, input *BulkUpdateAccountsInput) (*BulkUpdateAccountsResult, bool, error) {
	channels, err := s.GetAccountIPChannels(ctx, input.AccountIDs)
	if err != nil {
		return nil, true, err
	}
	if len(channels) == 0 {
		return nil, false, nil
	}
	if input.ProxyID != nil || input.Concurrency != nil || input.Priority != nil || input.LoadFactor != nil {
		return nil, true, infraerrors.BadRequest("IP_CHANNEL_USE_ENDPOINT", "批量编辑包含多 IP 账号，请分别在通道中修改 IP、并发和优先级")
	}
	result := &BulkUpdateAccountsResult{SuccessIDs: []int64{}, FailedIDs: []int64{}, Results: []BulkUpdateAccountResult{}}
	for _, id := range input.AccountIDs {
		update := &UpdateAccountInput{Name: input.Name, Status: input.Status, GroupIDs: input.GroupIDs, RateMultiplier: input.RateMultiplier, ProbeEnabled: input.ProbeEnabled, SkipMixedChannelCheck: input.SkipMixedChannelCheck}
		a, err := s.GetAccount(ctx, id)
		if err == nil && input.Extra != nil {
			update.Extra, err = cloneAccountJSONMap(a.Extra)
			if update.Extra == nil {
				update.Extra = map[string]any{}
			}
			for k, v := range input.Extra {
				update.Extra[k] = v
			}
		}
		if err == nil && input.Credentials != nil {
			update.Credentials, err = cloneAccountJSONMap(a.Credentials)
			if update.Credentials == nil {
				update.Credentials = map[string]any{}
			}
			for k, v := range input.Credentials {
				update.Credentials[k] = v
			}
		}
		if err == nil {
			_, err = s.UpdateAccount(ctx, id, update)
		}
		if err == nil && input.Schedulable != nil {
			_, err = s.SetAccountSchedulable(ctx, id, *input.Schedulable)
		}
		entry := BulkUpdateAccountResult{AccountID: id, Success: err == nil}
		if err != nil {
			entry.Error = err.Error()
			result.Failed++
			result.FailedIDs = append(result.FailedIDs, id)
		} else {
			result.Success++
			result.SuccessIDs = append(result.SuccessIDs, id)
		}
		result.Results = append(result.Results, entry)
	}
	return result, true, nil
}

// ResolveAccountIPUpstreamRoute preserves the logical credential identity while
// selecting only an enrolled, enabled fixed proxy for an administrative request.
// Error/cooldown states do not prevent token repair, but operator pauses do.
func ResolveAccountIPUpstreamRoute(ctx context.Context, repo AccountRepository, account *Account) (*Account, error) {
	channelsRepo, ok := repo.(AccountIPChannelRepository)
	if !ok || account == nil || account.Platform != PlatformOpenAI || account.Type != AccountTypeOAuth {
		return account, nil
	}
	groups, err := channelsRepo.GetAccountIPChannels(ctx, []int64{account.ID})
	if err != nil {
		return nil, err
	}
	members := groups[account.ID]
	if len(members) == 0 {
		return account, nil
	}
	ordered := append([]AccountIPChannel(nil), members...)
	// Prefer the requested physical channel if it still exists.
	for i, c := range ordered {
		if c.Account.ID == account.ID {
			ordered[0], ordered[i] = ordered[i], ordered[0]
			break
		}
	}
	for _, c := range ordered {
		p := c.Account.Proxy
		if !c.Enabled || !c.LogicalEnabled || p == nil || !p.IsActive() || p.IsExpired(time.Now()) {
			continue
		}
		route := *account
		route.ProxyID = c.Account.ProxyID
		route.Proxy = p
		return &route, nil
	}
	return nil, infraerrors.Conflict("IP_CHANNEL_NO_AVAILABLE_ROUTE", "没有启用且代理有效的 IP 通道，请先恢复通道后重试")
}
