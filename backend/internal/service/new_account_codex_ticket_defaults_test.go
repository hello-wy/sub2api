//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func newAutomaticTicketSettings(t *testing.T, plan string) (*SettingService, *codexTicketSettingRepo) {
	t.Helper()
	repo := &codexTicketSettingRepo{}
	svc := NewSettingService(repo, nil)
	_, err := svc.UpdateNewAccountCodexTicketDefaults(context.Background(), NewAccountCodexTicketDefaults{Enabled: true, TicketPlan: plan})
	require.NoError(t, err)
	return svc, repo
}

func TestNewAccountCodexTicketDefaultsOverrideGroupsAndRevertOnDisable(t *testing.T) {
	settings, _ := newAutomaticTicketSettings(t, "team")
	groups := &groupRepoStubForAdmin{getByIDByID: map[int64]*Group{
		1: {ID: 1, Name: "Pro", Platform: PlatformOpenAI, CodexTicketDefaults: GroupCodexTicketDefaults{Enabled: true, TicketPlan: "pro", Model: "gpt-6-astra"}},
		2: {ID: 2, Name: "Team", Platform: PlatformOpenAI, CodexTicketDefaults: GroupCodexTicketDefaults{Enabled: true, TicketPlan: "team", Model: "gpt-5.6-sol"}},
	}}
	svc := &adminServiceImpl{settingService: settings, groupRepo: groups}
	for _, ids := range [][]int64{nil, {1}, {1, 2}} {
		value, err := svc.resolveCodexTicketGroupDefaults(context.Background(), ids)
		require.NoError(t, err)
		require.Equal(t, &GroupCodexTicketDefaults{Enabled: true, TicketPlan: "team", Model: "gpt-6-astra", RequireVerified: true}, value)
	}
	require.NoError(t, svc.ValidateCodexTicketGroupDefaults(context.Background(), []int64{1, 2}), "active global policy resolves group ticket-plan conflicts")
	_, err := settings.UpdateNewAccountCodexTicketDefaults(context.Background(), NewAccountCodexTicketDefaults{Enabled: false, TicketPlan: "team"})
	require.NoError(t, err)
	value, err := svc.resolveCodexTicketGroupDefaults(context.Background(), nil)
	require.NoError(t, err)
	require.Nil(t, value)
	value, err = svc.resolveCodexTicketGroupDefaults(context.Background(), []int64{1})
	require.NoError(t, err)
	require.Equal(t, "pro", value.TicketPlan)
	require.False(t, value.RequireVerified, "legacy group defaults keep their existing scope")
	require.Error(t, svc.ValidateCodexTicketGroupDefaults(context.Background(), []int64{1, 2}))
}

func TestNewAccountCodexTicketDefaultsCreateAndImportHooks(t *testing.T) {
	settings, settingRepo := newAutomaticTicketSettings(t, "team")
	accounts := &groupTicketCreateRepo{}
	starter := &groupTicketStarter{}
	svc := &adminServiceImpl{settingService: settings, accountRepo: accounts, runtimeBlocker: starter}
	hooks := &accountImportCommitHooks{admin: svc}
	ctx := context.WithValue(context.Background(), accountImportCommitKey{}, hooks)
	a, err := svc.CreateAccount(ctx, &CreateAccountInput{Name: "ungrouped OAuth", Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"access_token": "synthetic"}, SkipDefaultGroupBind: true, Extra: map[string]any{codexAccountTicketConfigKey: map[string]any{"enabled": false, "ticket_plan": "pro"}}})
	require.NoError(t, err)
	config := codexAccountTicketConfigOf(a)
	require.True(t, config.Enabled)
	require.True(t, config.RequireVerified)
	require.Equal(t, "team", config.TicketPlan)
	require.Equal(t, "gpt-6-astra", config.Model)
	require.NotEmpty(t, config.Revision)
	require.Empty(t, starter.calls, "no upstream side effects before import commit")
	require.Len(t, hooks.hooks, 2, "ticket kickoff plus existing OAuth privacy hook")
	hooks.hooks[0]()
	require.Equal(t, []int64{a.ID}, starter.calls)
	require.Equal(t, config, codexAccountTicketConfigOf(a), "a missing harvest pool must not erase the account's verification gate")
	_, err = svc.CreateAccount(ctx, &CreateAccountInput{Name: "API key", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, SkipDefaultGroupBind: true})
	require.NoError(t, err)
	require.False(t, codexAccountTicketConfigOf(accounts.created[1]).Enabled, "API keys do not inherit OAuth STATE")
	settingRepo.readErr = errors.New("settings DB unavailable")
	_, err = svc.CreateAccount(ctx, &CreateAccountInput{Name: "read failure", Platform: PlatformOpenAI, Type: AccountTypeSetupToken, SkipDefaultGroupBind: true})
	require.Error(t, err)
	require.Len(t, accounts.created, 2, "unable to read required defaults fails before creating an unprotected account")
}

type automaticTicketIPRepo struct {
	AccountRepository
	AccountIPChannelRepository
	root      *Account
	inherited *GroupCodexTicketDefaults
}

func (r *automaticTicketIPRepo) GetByID(context.Context, int64) (*Account, error) { return r.root, nil }
func (r *automaticTicketIPRepo) AddAccountIPChannelsWithTicketDefaults(_ context.Context, _ int64, _ []int64, _ *int, _ *int, value *GroupCodexTicketDefaults) ([]int64, error) {
	r.inherited = value
	return []int64{202, 203}, nil
}

type automaticTicketProxyRepo struct{ ProxyRepository }

func (r *automaticTicketProxyRepo) GetByID(_ context.Context, id int64) (*Proxy, error) {
	return &Proxy{ID: id, Status: StatusActive}, nil
}

func TestNewAccountCodexTicketDefaultsNewIPInheritsWithoutReactivatingRoot(t *testing.T) {
	settings, _ := newAutomaticTicketSettings(t, "pro")
	root := &Account{ID: 201, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusError, Schedulable: false, ErrorMessage: "token_revoked", Extra: map[string]any{AccountModelMismatchExtraKey: map[string]any{"quarantined": true}}}
	accounts := &automaticTicketIPRepo{root: root}
	starter := &groupTicketStarter{}
	svc := &adminServiceImpl{settingService: settings, accountRepo: accounts, proxyRepo: &automaticTicketProxyRepo{}, runtimeBlocker: starter}
	hooks := &accountImportCommitHooks{admin: svc}
	ctx := context.WithValue(context.Background(), accountImportCommitKey{}, hooks)
	require.NoError(t, svc.AddAccountIPChannels(ctx, 201, []int64{2, 3}, nil, nil))
	require.Equal(t, &GroupCodexTicketDefaults{Enabled: true, TicketPlan: "pro", Model: "gpt-6-astra", RequireVerified: true}, accounts.inherited)
	require.Empty(t, starter.calls)
	require.Len(t, hooks.hooks, 2)
	for _, hook := range hooks.hooks {
		hook()
	}
	require.Equal(t, []int64{202, 203}, starter.calls)
	require.False(t, root.Schedulable)
	require.Equal(t, StatusError, root.Status)
	require.Equal(t, "token_revoked", root.ErrorMessage)
	require.False(t, codexAccountTicketConfigOf(root).Enabled, "adding IPs must not rewrite the existing root ticket configuration")
	clone := *root
	clone.InitialCodexTicketDefaults = accounts.inherited
	clone.Extra = map[string]any{AccountModelMismatchExtraKey: map[string]any{"quarantined": true}}
	require.True(t, PrepareNewAccountCodexTicketDefaults(&clone))
	require.False(t, clone.Schedulable)
	require.Equal(t, StatusError, clone.Status)
	require.True(t, clone.Extra[AccountModelMismatchExtraKey].(map[string]any)["quarantined"].(bool))
}
