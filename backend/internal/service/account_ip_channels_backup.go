package service

import (
	"context"
	"errors"
)

// Backups restore configuration into new local IDs. Historical usage IDs in the
// source database are never reassigned or treated as portable identities.
type AccountIPChannelBackup struct {
	ProxyID     int64
	IsRoot      bool
	Concurrency int
	Priority    int
	LoadFactor  *int
	Enabled     bool
	Status      string
	Extra       map[string]any
}

type AccountIPChannelBackupAdmin interface {
	RestoreAccountIPChannels(context.Context, *CreateAccountInput, []AccountIPChannelBackup, bool, bool) (*Account, error)
}

type AccountIPChannelBackupRepository interface {
	RestoreAccountIPChannels(context.Context, *Account, []AccountIPChannelBackup, bool, bool) error
}

func (s *adminServiceImpl) RestoreAccountIPChannels(ctx context.Context, input *CreateAccountInput, channels []AccountIPChannelBackup, logicalEnabled, rootRetired bool) (*Account, error) {
	if input.Platform != PlatformOpenAI || input.Type != AccountTypeOAuth || len(channels) < 1 || len(channels) > 50 {
		return nil, errors.New("backup requires 1 to 50 fixed OpenAI OAuth IP channels")
	}
	seen := map[int64]bool{}
	roots := 0
	for _, c := range channels {
		if c.ProxyID <= 0 || seen[c.ProxyID] || c.Concurrency < 0 || c.Concurrency > 10000 || c.Priority < 0 || (c.LoadFactor != nil && (*c.LoadFactor < 0 || *c.LoadFactor > 10000)) {
			return nil, errors.New("invalid or duplicate IP channel configuration")
		}
		if c.Status != StatusActive && c.Status != StatusDisabled && c.Status != StatusError {
			return nil, errors.New("invalid IP channel status")
		}
		if (&Account{Extra: c.Extra}).IsRandomProxy() {
			return nil, errors.New("IP channels require fixed proxies")
		}
		if err := ValidateAccountProtectionConfiguration(&Account{Platform: input.Platform, Type: input.Type, Concurrency: c.Concurrency, Extra: c.Extra}); err != nil {
			return nil, err
		}
		if err := ValidateUpstreamRequestIDHeaderExtra(c.Extra); err != nil {
			return nil, err
		}
		if err := ValidateQuotaResetConfig(c.Extra); err != nil {
			return nil, err
		}
		if _, err := s.proxyRepo.GetByID(ctx, c.ProxyID); err != nil {
			return nil, err
		}
		seen[c.ProxyID] = true
		if c.IsRoot {
			roots++
		}
	}
	if (rootRetired && roots != 0) || (!rootRetired && roots != 1) {
		return nil, errors.New("invalid root IP channel configuration")
	}
	credentials, err := cloneAccountJSONMap(input.Credentials)
	if err != nil {
		return nil, err
	}
	inputCopy := *input
	inputCopy.Credentials = credentials
	if err = NormalizeHeaderOverrideCredentials(credentials); err != nil {
		return nil, err
	}
	inputCopy.Credentials = SanitizeStoredCredentials(input.Platform, credentials)
	extra, err := cloneAccountJSONMap(input.Extra)
	if err != nil {
		return nil, err
	}
	extra, err = normalizeOpenAILongContextBillingExtra(input.Platform, extra)
	if err != nil {
		return nil, err
	}
	extra, err = normalizeOpenAIAutoResetCreditExtra(input.Platform, input.Type, false, extra)
	if err != nil {
		return nil, err
	}
	if err = ValidateUpstreamRequestIDHeaderExtra(extra); err != nil {
		return nil, err
	}
	if (&Account{Extra: extra}).IsRandomProxy() {
		return nil, errors.New("IP channels require fixed proxies")
	}
	root, err := buildAccountForCreate(&inputCopy, extra)
	if err != nil {
		return nil, err
	}
	root.GroupIDs = append([]int64(nil), input.GroupIDs...)
	if len(root.GroupIDs) == 0 && !input.SkipDefaultGroupBind {
		groups, err := s.groupRepo.ListActiveByPlatform(ctx, input.Platform)
		if err != nil {
			return nil, err
		}
		for _, g := range groups {
			if g.Name == input.Platform+"-default" {
				root.GroupIDs = []int64{g.ID}
				break
			}
		}
	}
	if err = s.ValidateAccountGroupBindings(ctx, root.GroupIDs); err != nil {
		return nil, err
	}
	if len(root.GroupIDs) > 0 {
		if err = s.checkMixedChannelRisk(ctx, 0, input.Platform, root.GroupIDs); err != nil {
			return nil, err
		}
	}
	defaults, err := s.resolveCodexTicketGroupDefaults(ctx, root.GroupIDs)
	if err != nil {
		return nil, err
	}
	root.InitialCodexTicketDefaults = defaults
	channels = append([]AccountIPChannelBackup(nil), channels...)
	for i := range channels {
		channels[i].Extra = RedactOpenAICodexTicketExtra(channels[i].Extra)
	}
	repo, ok := s.accountRepo.(AccountIPChannelBackupRepository)
	if !ok {
		return nil, errors.New("IP channel backup repository unavailable")
	}
	// Resolve consumed token generations before restoration, as with multi-IP import.
	if root.GetCredential("refresh_token") != "" {
		root.Credentials, err = s.RegisterOpenAIOAuthCredentialGroup(ctx, nil, root.Credentials)
		if err != nil {
			return nil, err
		}
	}
	if err = repo.RestoreAccountIPChannels(ctx, root, channels, logicalEnabled, rootRetired); err != nil {
		return nil, err
	}
	if defaults != nil && defaults.Enabled {
		members, err := s.GetAccountIPChannels(ctx, []int64{root.ID})
		if err != nil {
			return nil, err
		}
		for _, member := range members[root.ID] {
			s.scheduleInheritedCodexTicket(ctx, member.Account.ID)
		}
	}
	return s.GetAccount(ctx, root.ID)
}
