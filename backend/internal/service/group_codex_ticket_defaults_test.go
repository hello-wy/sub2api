//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGroupCodexTicketDefaultsResolveAndValidate(t *testing.T) {
	pro := GroupCodexTicketDefaults{Enabled: true, TicketPlan: "pro", Model: "gpt-6-astra"}
	team := GroupCodexTicketDefaults{Enabled: true, TicketPlan: "team", Model: "gpt-5.6-sol"}
	repo := &groupRepoStubForAdmin{getByIDByID: map[int64]*Group{
		1: {ID: 1, Name: "Pro", Platform: PlatformOpenAI, CodexTicketDefaults: pro},
		2: {ID: 2, Name: "Same", Platform: PlatformOpenAI, CodexTicketDefaults: pro},
		3: {ID: 3, Name: "Team", Platform: PlatformOpenAI, CodexTicketDefaults: team},
		4: {ID: 4, Name: "Off", Platform: PlatformOpenAI},
	}}
	svc := &adminServiceImpl{groupRepo: repo}
	got, err := svc.resolveCodexTicketGroupDefaults(context.Background(), []int64{1, 2, 1, 4})
	require.NoError(t, err)
	require.Equal(t, pro, *got)
	err = svc.ValidateCodexTicketGroupDefaults(context.Background(), []int64{1, 3})
	require.ErrorContains(t, err, "冲突")
	got, err = svc.resolveCodexTicketGroupDefaults(context.Background(), []int64{4})
	require.NoError(t, err)
	require.Nil(t, got)
	_, err = normalizeGroupCodexTicketDefaults(PlatformOpenAI, GroupCodexTicketDefaults{Enabled: true, TicketPlan: "312"})
	require.Error(t, err)
	_, err = normalizeGroupCodexTicketDefaults(PlatformOpenAI, GroupCodexTicketDefaults{Enabled: true, Model: "unverified-model"})
	require.Error(t, err)
	normalized, err := normalizeGroupCodexTicketDefaults(PlatformAnthropic, pro)
	require.NoError(t, err)
	require.False(t, normalized.Enabled)
}

type groupTicketCreateRepo struct {
	AccountRepository
	created []*Account
}

func (r *groupTicketCreateRepo) Create(_ context.Context, a *Account) error {
	a.ID = int64(len(r.created) + 1)
	r.created = append(r.created, a)
	return nil
}
func (r *groupTicketCreateRepo) BindGroups(context.Context, int64, []int64) error { return nil }

type groupTicketStarter struct{ calls []int64 }

func (*groupTicketStarter) BlockAccountScheduling(*Account, time.Time, string) {}
func (*groupTicketStarter) ClearAccountSchedulingBlock(int64)                  {}
func (s *groupTicketStarter) HarvestCodexAccountTicket(_ context.Context, id int64) (*CodexAccountTicketStatus, error) {
	s.calls = append(s.calls, id)
	return nil, errors.New("global pool not configured")
}

func TestGroupCodexTicketDefaultsCreateDefersCollectionAndPreservesSavedAccount(t *testing.T) {
	defaults := GroupCodexTicketDefaults{Enabled: true, TicketPlan: "team", Model: "gpt-5.6-sol"}
	groups := &groupRepoStubForAdmin{getByIDByID: map[int64]*Group{1: {ID: 1, Platform: PlatformOpenAI, CodexTicketDefaults: defaults}}}
	accounts := &groupTicketCreateRepo{}
	starter := &groupTicketStarter{}
	svc := &adminServiceImpl{groupRepo: groups, accountRepo: accounts, runtimeBlocker: starter}
	hooks := &accountImportCommitHooks{admin: svc}
	ctx := context.WithValue(context.Background(), accountImportCommitKey{}, hooks)
	a, err := svc.CreateAccount(ctx, &CreateAccountInput{Name: "new", Platform: PlatformOpenAI, Type: AccountTypeSetupToken, GroupIDs: []int64{1}, SkipDefaultGroupBind: true})
	require.NoError(t, err)
	require.Len(t, accounts.created, 1)
	require.True(t, codexAccountTicketConfigOf(a).Enabled)
	require.Equal(t, "team", codexAccountTicketConfigOf(a).TicketPlan)
	require.Empty(t, starter.calls, "uncommitted/rolled-back accounts must never reach the upstream")
	require.Len(t, hooks.hooks, 1)
	hooks.hooks[0]()
	require.Equal(t, []int64{a.ID}, starter.calls)
	require.True(t, codexAccountTicketConfigOf(accounts.created[0]).Enabled, "pool unavailability must not erase saved configuration")
	groups.getByIDByID[2] = &Group{ID: 2, Platform: PlatformOpenAI, CodexTicketDefaults: GroupCodexTicketDefaults{Enabled: true, TicketPlan: "pro", Model: "gpt-6-astra"}}
	_, err = svc.CreateAccount(context.Background(), &CreateAccountInput{Platform: PlatformOpenAI, Type: AccountTypeSetupToken, GroupIDs: []int64{1, 2}})
	require.Error(t, err)
	require.Len(t, accounts.created, 1, "conflict must be checked before the first insert")
}
