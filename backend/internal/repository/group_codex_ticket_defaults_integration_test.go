//go:build unit

package repository

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type groupDefaultsTicketStarter struct{ calls []int64 }

func (*groupDefaultsTicketStarter) BlockAccountScheduling(*service.Account, time.Time, string) {}
func (*groupDefaultsTicketStarter) ClearAccountSchedulingBlock(int64)                          {}
func (s *groupDefaultsTicketStarter) HarvestCodexAccountTicket(_ context.Context, id int64) (*service.CodexAccountTicketStatus, error) {
	s.calls = append(s.calls, id)
	return nil, errors.New("no harvest pool in local fixture")
}

func TestGroupCodexTicketDefaultsPostgresPersistenceAndImport(t *testing.T) {
	r, c, ctx := ipChannelIntegrationRepo(t)
	migration, err := os.ReadFile("../../migrations/256_group_codex_ticket_defaults.sql")
	require.NoError(t, err)
	_, err = r.sql.ExecContext(ctx, string(migration))
	require.NoError(t, err)
	_, err = r.sql.ExecContext(ctx, string(migration))
	require.NoError(t, err)
	groups := newGroupRepositoryWithSQL(c, r.sql)
	defaults := service.GroupCodexTicketDefaults{Enabled: true, TicketPlan: "team", Model: "gpt-5.6-sol"}
	g := &service.Group{Name: "ticket-defaults", Platform: service.PlatformOpenAI, Status: service.StatusActive, RateMultiplier: 1, CodexTicketDefaults: defaults}
	require.NoError(t, groups.Create(ctx, g))
	read, err := groups.GetByID(ctx, g.ID)
	require.NoError(t, err)
	require.Equal(t, defaults, read.CodexTicketDefaults)
	read.CodexTicketDefaults.Model = "gpt-6-astra"
	require.NoError(t, groups.Update(ctx, read))
	read, err = groups.GetByID(ctx, g.ID)
	require.NoError(t, err)
	require.Equal(t, "gpt-6-astra", read.CodexTicketDefaults.Model)

	admin := service.NewAdminService(&config.Config{}, nil, groups, r, newProxyRepositoryWithSQL(c, r.sql), nil, nil, nil, nil, nil, nil, nil, nil, c, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	a, err := admin.CreateAccount(ctx, &service.CreateAccountInput{Name: "default-inherited", Platform: service.PlatformOpenAI, Type: service.AccountTypeSetupToken, GroupIDs: []int64{g.ID}, Extra: map[string]any{"codex_ticket_config": map[string]any{"revision": "forged", "enabled": true}, "codex_turn_ticket:gpt-6-astra": map[string]any{"state": "forged"}, "codex_turn_ticket:v2:gpt-6-astra": map[string]any{"state": "forged-v2"}}})
	require.NoError(t, err, "missing pool and fixed proxy must not lose newly imported accounts")
	a, err = r.GetByID(ctx, a.ID)
	require.NoError(t, err)
	settings := a.Extra["codex_ticket_config"].(map[string]any)
	require.Equal(t, true, settings["enabled"])
	require.Equal(t, "team", settings["ticket_plan"])
	require.Equal(t, "gpt-6-astra", settings["model"])
	require.NotEmpty(t, settings["revision"])
	require.NotEqual(t, "forged", settings["revision"])
	require.NotContains(t, a.Extra, "codex_turn_ticket:gpt-6-astra")
	require.NotContains(t, a.Extra, "codex_turn_ticket:v2:gpt-6-astra")
	// A re-import/shared-field edit must retain explicit per-account settings,
	// including an explicit opt-out after inheriting a group default.
	require.NoError(t, r.MutateOpenAICodexTicketExtra(ctx, a.ID, func(*service.Account) (map[string]any, error) {
		return map[string]any{"codex_ticket_config": map[string]any{"enabled": false, "ticket_plan": "pro", "model": "gpt-5.6-sol", "revision": "manual-opt-out"}}, nil
	}))
	groupIDs := []int64{g.ID}
	_, err = admin.UpdateAccount(ctx, a.ID, &service.UpdateAccountInput{Name: "updated", GroupIDs: &groupIDs})
	require.NoError(t, err)
	a, err = r.GetByID(ctx, a.ID)
	require.NoError(t, err)
	require.Equal(t, "manual-opt-out", a.Extra["codex_ticket_config"].(map[string]any)["revision"])

	var channels []service.AccountIPChannelBackup
	for i := 0; i < 2; i++ {
		proxy, err := c.Proxy.Create().SetName("ticket-proxy").SetProtocol("http").SetHost("192.0.2.11").SetPort(9000 + i).SetStatus(service.StatusActive).Save(ctx)
		require.NoError(t, err)
		channels = append(channels, service.AccountIPChannelBackup{ProxyID: proxy.ID, IsRoot: i == 0, Concurrency: 5, Enabled: true, Status: service.StatusActive, Extra: map[string]any{"codex_ticket_config": map[string]any{"revision": "forged"}}})
	}
	root, err := admin.(service.AccountIPChannelBackupAdmin).RestoreAccountIPChannels(ctx, &service.CreateAccountInput{Name: "multi", Platform: service.PlatformOpenAI, Type: service.AccountTypeOAuth, Credentials: map[string]any{"access_token": "local-fixture"}, GroupIDs: []int64{g.ID}, SkipDefaultGroupBind: true}, channels, true, false)
	require.NoError(t, err)
	members, err := r.GetAccountIPChannels(ctx, []int64{root.ID})
	require.NoError(t, err)
	require.Len(t, members[root.ID], 2)
	revisions := map[any]bool{}
	for _, member := range members[root.ID] {
		settings := member.Account.Extra["codex_ticket_config"].(map[string]any)
		require.Equal(t, "team", settings["ticket_plan"])
		require.NotEmpty(t, settings["revision"])
		require.False(t, revisions[settings["revision"]], "each fixed IP must own its ticket revision")
		revisions[settings["revision"]] = true
	}

	starter := &groupDefaultsTicketStarter{}
	withStarter := service.NewAdminService(&config.Config{}, nil, groups, r, newProxyRepositoryWithSQL(c, r.sql), nil, nil, nil, nil, nil, nil, nil, nil, c, nil, nil, nil, nil, starter, nil, nil, nil, nil)
	transactions := withStarter.(service.AccountImportTransactionAdmin)
	var committedID, rolledBackID int64
	err = transactions.WithAccountImportTransaction(ctx, func(txctx context.Context, admin service.AdminService) error {
		created, err := admin.CreateAccount(txctx, &service.CreateAccountInput{Name: "commit", Platform: service.PlatformOpenAI, Type: service.AccountTypeSetupToken, GroupIDs: groupIDs})
		if err != nil {
			return err
		}
		committedID = created.ID
		require.Empty(t, starter.calls, "upstream collection cannot start before PostgreSQL commit")
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, []int64{committedID}, starter.calls)
	err = transactions.WithAccountImportTransaction(ctx, func(txctx context.Context, admin service.AdminService) error {
		created, err := admin.CreateAccount(txctx, &service.CreateAccountInput{Name: "rollback", Platform: service.PlatformOpenAI, Type: service.AccountTypeSetupToken, GroupIDs: groupIDs})
		if err != nil {
			return err
		}
		rolledBackID = created.ID
		return errors.New("abort import")
	})
	require.ErrorContains(t, err, "abort import")
	require.Equal(t, []int64{committedID}, starter.calls, "rolled-back imports never collect tickets")
	_, err = r.GetByID(ctx, rolledBackID)
	require.ErrorIs(t, err, service.ErrAccountNotFound)

	// Adding an IP later must inherit the group's template on the new channel,
	// while retaining explicit opt-outs on the old channel and collecting only
	// after the encompassing import transaction commits.
	starter.calls = nil
	require.NoError(t, r.MutateOpenAICodexTicketExtra(ctx, root.ID, func(*service.Account) (map[string]any, error) {
		return map[string]any{"codex_ticket_config": map[string]any{"enabled": false, "ticket_plan": "pro", "model": "gpt-5.6-sol", "revision": "existing-manual-off"}}, nil
	}))
	newProxy, err := c.Proxy.Create().SetName("new-ip").SetProtocol("http").SetHost("192.0.2.12").SetPort(9002).SetStatus(service.StatusActive).Save(ctx)
	require.NoError(t, err)
	err = transactions.WithAccountImportTransaction(ctx, func(txctx context.Context, admin service.AdminService) error {
		err := admin.(service.AccountIPChannelAdmin).AddAccountIPChannels(txctx, root.ID, []int64{newProxy.ID}, nil, nil)
		require.Empty(t, starter.calls)
		return err
	})
	require.NoError(t, err)
	require.Len(t, starter.calls, 1)
	added, err := r.GetByID(ctx, starter.calls[0])
	require.NoError(t, err)
	require.Equal(t, newProxy.ID, *added.ProxyID)
	addedConfig := added.Extra["codex_ticket_config"].(map[string]any)
	require.Equal(t, true, addedConfig["enabled"])
	require.Equal(t, "team", addedConfig["ticket_plan"])
	require.False(t, revisions[addedConfig["revision"]])
	old, err := r.GetByID(ctx, root.ID)
	require.NoError(t, err)
	require.Equal(t, "existing-manual-off", old.Extra["codex_ticket_config"].(map[string]any)["revision"])

	conflicting := &service.Group{Name: "conflicting-defaults", Platform: service.PlatformOpenAI, Status: service.StatusActive, RateMultiplier: 1, CodexTicketDefaults: service.GroupCodexTicketDefaults{Enabled: true, TicketPlan: "pro", Model: "gpt-6-astra"}}
	require.NoError(t, groups.Create(ctx, conflicting))
	require.NoError(t, r.BindGroups(ctx, root.ID, []int64{g.ID, conflicting.ID}))
	unusedProxy, err := c.Proxy.Create().SetName("conflict-ip").SetProtocol("http").SetHost("192.0.2.13").SetPort(9003).SetStatus(service.StatusActive).Save(ctx)
	require.NoError(t, err)
	err = withStarter.(service.AccountIPChannelAdmin).AddAccountIPChannels(ctx, root.ID, []int64{unusedProxy.ID}, nil, nil)
	require.ErrorContains(t, err, "冲突")
	members, err = r.GetAccountIPChannels(ctx, []int64{root.ID})
	require.NoError(t, err)
	require.Len(t, members[root.ID], 3)
	require.Len(t, starter.calls, 1)
}
