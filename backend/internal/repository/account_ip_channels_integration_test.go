//go:build unit

package repository

import (
	"context"
	"os"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func ipChannelIntegrationRepo(t *testing.T) (*accountRepository, *dbent.Client, context.Context) {
	t.Helper()
	r, c, ctx := openAICredentialGroupIntegrationRepo(t)
	sql, err := os.ReadFile("../../migrations/251_account_ip_channels.sql")
	require.NoError(t, err)
	_, err = r.sql.ExecContext(ctx, string(sql))
	require.NoError(t, err)
	_, err = r.sql.ExecContext(ctx, `CREATE TABLE scheduled_test_plans(id BIGSERIAL PRIMARY KEY,account_id BIGINT)`)
	require.NoError(t, err)
	return r, c, ctx
}

func createIPChannelFixture(t *testing.T, r *accountRepository, c *dbent.Client, ctx context.Context, name string, n int) []*service.Account {
	t.Helper()
	accounts := []*service.Account{}
	for i := 0; i < n; i++ {
		p, err := c.Proxy.Create().SetName(name).SetProtocol("http").SetHost("192.0.2.10").SetPort(8000 + i).SetStatus(service.StatusActive).Save(ctx)
		require.NoError(t, err)
		a := createGroupTestAccount(t, ctx, c, name, "rt-"+name, "at-"+name)
		a.ProxyID = &p.ID
		a.Concurrency = 10 + i
		a.Schedulable = true
		a.Status = service.StatusActive
		a.Name = name
		require.NoError(t, r.Update(ctx, a))
		accounts = append(accounts, a)
	}
	return accounts
}

func TestAccountIPChannelsPostgresLogicalPaginationAndIndependentDispatch(t *testing.T) {
	r, c, ctx := ipChannelIntegrationRepo(t)
	a := createIPChannelFixture(t, r, c, ctx, "alpha", 3)
	root, err := r.JoinAccountIPChannels(ctx, []int64{a[2].ID, a[0].ID, a[1].ID})
	require.NoError(t, err)
	require.Equal(t, a[0].ID, root)
	p := pagination.PaginationParams{Page: 1, PageSize: 1, SortBy: "name", SortOrder: "asc"}
	rows, page, err := r.ListLogicalAccounts(ctx, p, "openai", "", "", "", 0, "")
	require.NoError(t, err)
	require.EqualValues(t, 1, page.Total)
	require.Len(t, rows, 1)
	routing, err := r.ListSchedulable(ctx)
	require.NoError(t, err)
	require.Len(t, routing, 3)
	off := false
	require.NoError(t, r.PatchAccountIPChannel(ctx, root, a[0].ID, service.AccountIPChannelPatch{Schedulable: &off}))
	rows, page, err = r.ListLogicalAccounts(ctx, p, "openai", "", "active", "", 0, "")
	require.NoError(t, err)
	require.EqualValues(t, 1, page.Total)
	require.Equal(t, root, rows[0].ID)
	handled, err := r.SetLogicalAccountSchedulable(ctx, root, false)
	require.True(t, handled)
	require.NoError(t, err)
	routing, err = r.ListSchedulable(ctx)
	require.NoError(t, err)
	require.Empty(t, routing)
	_, err = r.SetLogicalAccountSchedulable(ctx, root, true)
	require.NoError(t, err)
	routing, err = r.ListSchedulable(ctx)
	require.NoError(t, err)
	require.Len(t, routing, 2)
	channels, err := r.GetAccountIPChannels(ctx, []int64{root})
	require.NoError(t, err)
	require.False(t, channels[root][0].Enabled)
	require.True(t, channels[root][1].Enabled)
}

func TestAccountIPChannelsPostgresConflictsRollbackAndRetiredHistory(t *testing.T) {
	r, c, ctx := ipChannelIntegrationRepo(t)
	a := createIPChannelFixture(t, r, c, ctx, "alpha", 2)
	root, err := r.JoinAccountIPChannels(ctx, []int64{a[0].ID, a[1].ID})
	require.NoError(t, err)
	require.Error(t, r.PatchAccountIPChannel(ctx, root, a[1].ID, service.AccountIPChannelPatch{ProxyID: a[0].ProxyID}))
	require.Error(t, r.RemoveAccountIPChannel(ctx, root, a[0].ID))
	off := false
	require.NoError(t, r.PatchAccountIPChannel(ctx, root, a[0].ID, service.AccountIPChannelPatch{Schedulable: &off}))
	_, err = r.sql.ExecContext(ctx, `UPDATE account_ip_channels SET disabled_at=$2 WHERE account_id=$1`, a[0].ID, time.Now().Add(-3*time.Minute))
	require.NoError(t, err)
	require.NoError(t, r.RemoveAccountIPChannel(ctx, root, a[0].ID))
	cs, err := r.GetAccountIPChannels(ctx, []int64{root})
	require.NoError(t, err)
	require.Len(t, cs[root], 1)
	require.Equal(t, a[1].ID, cs[root][0].Account.ID)
	ids, err := r.AccountIPChannelHistoryIDs(ctx, []int64{root})
	require.NoError(t, err)
	require.Equal(t, []int64{a[0].ID, a[1].ID}, ids[root])
	require.Error(t, r.RemoveAccountIPChannel(ctx, root, a[1].ID))
	rows, page, err := r.ListLogicalAccounts(ctx, pagination.PaginationParams{Page: 1, PageSize: 20}, "", "", "", "", 0, "")
	require.NoError(t, err)
	require.EqualValues(t, 1, page.Total)
	require.Equal(t, root, rows[0].ID)
}

func TestAccountIPChannelsPostgresAddAtomicAndPreservesFixedIP(t *testing.T) {
	r, c, ctx := ipChannelIntegrationRepo(t)
	a := createIPChannelFixture(t, r, c, ctx, "alpha", 1)
	root, err := r.JoinAccountIPChannels(ctx, []int64{a[0].ID})
	require.NoError(t, err)
	p, err := c.Proxy.Create().SetName("new").SetProtocol("http").SetHost("192.0.2.20").SetPort(8080).SetStatus(service.StatusActive).Save(ctx)
	require.NoError(t, err)
	require.Error(t, r.AddAccountIPChannels(ctx, root, []int64{p.ID, p.ID}, nil, nil))
	cs, err := r.GetAccountIPChannels(ctx, []int64{root})
	require.NoError(t, err)
	require.Len(t, cs[root], 1)
	off := false
	require.NoError(t, r.PatchAccountIPChannel(ctx, root, root, service.AccountIPChannelPatch{Schedulable: &off}))
	require.NoError(t, r.AddAccountIPChannels(ctx, root, []int64{p.ID}, nil, nil))
	cs, err = r.GetAccountIPChannels(ctx, []int64{root})
	require.NoError(t, err)
	require.Len(t, cs[root], 2)
	require.True(t, cs[root][1].Enabled)
	require.True(t, cs[root][1].Account.Schedulable)
	require.Equal(t, *a[0].ProxyID, *cs[root][0].Account.ProxyID)
	require.Equal(t, p.ID, *cs[root][1].Account.ProxyID)
}

func TestAccountIPChannelsPostgresJoinRejectsDistinctUsersAndSettings(t *testing.T) {
	r, c, ctx := ipChannelIntegrationRepo(t)
	a := createIPChannelFixture(t, r, c, ctx, "alpha", 2)
	a[1].Credentials["chatgpt_user_id"] = "another-user"
	_, err := c.Account.UpdateOneID(a[1].ID).SetCredentials(a[1].Credentials).Save(ctx)
	require.NoError(t, err)
	_, err = r.JoinAccountIPChannels(ctx, []int64{a[0].ID, a[1].ID})
	require.Error(t, err)
	cs, err := r.GetAccountIPChannels(ctx, []int64{a[0].ID})
	require.NoError(t, err)
	require.Empty(t, cs)
}

func TestAccountIPChannelsPostgresSharedEditTransactionAndRecoveryGuard(t *testing.T) {
	r, c, ctx := ipChannelIntegrationRepo(t)
	a := createIPChannelFixture(t, r, c, ctx, "alpha", 2)
	root, err := r.JoinAccountIPChannels(ctx, []int64{a[0].ID, a[1].ID})
	require.NoError(t, err)
	err = r.WithAccountIPChannelTransaction(ctx, root, func(ctx context.Context, repo service.AccountRepository, members []service.AccountIPChannel) error {
		for _, m := range members {
			m.Account.Name = "updated"
			require.NoError(t, repo.Update(ctx, m.Account))
			require.NoError(t, repo.BindGroups(ctx, m.Account.ID, nil))
		}
		return context.Canceled
	})
	require.ErrorIs(t, err, context.Canceled)
	fresh, err := r.GetByID(ctx, root)
	require.NoError(t, err)
	require.Equal(t, "alpha", fresh.Name)
	off := false
	require.NoError(t, r.PatchAccountIPChannel(ctx, root, a[1].ID, service.AccountIPChannelPatch{Schedulable: &off}))
	require.NoError(t, r.ClearError(ctx, a[1].ID))
	require.NoError(t, r.SetSchedulable(ctx, a[1].ID, true))
	fresh, err = r.GetByID(ctx, a[1].ID)
	require.NoError(t, err)
	require.False(t, fresh.Schedulable)
	_, err = c.Account.UpdateOneID(root).SetProxyID(*a[1].ProxyID).Save(ctx)
	require.Error(t, err)
	err = r.WithAccountIPChannelTransaction(ctx, root, func(ctx context.Context, repo service.AccountRepository, members []service.AccountIPChannel) error {
		for _, m := range members {
			m.Account.Name = "updated"
			if err := repo.Update(ctx, m.Account); err != nil {
				return err
			}
		}
		return nil
	})
	require.NoError(t, err)
	for _, original := range a {
		fresh, err = r.GetByID(ctx, original.ID)
		require.NoError(t, err)
		require.Equal(t, "updated", fresh.Name)
		require.Equal(t, original.Concurrency, fresh.Concurrency)
		require.Equal(t, original.ProxyID, fresh.ProxyID)
	}
}

func TestAccountIPChannelsPostgresExpiredProxyDoesNotRotate(t *testing.T) {
	r, c, ctx := ipChannelIntegrationRepo(t)
	a := createIPChannelFixture(t, r, c, ctx, "alpha", 2)
	root, err := r.JoinAccountIPChannels(ctx, []int64{a[0].ID, a[1].ID})
	require.NoError(t, err)
	proxyRepo := &proxyRepository{client: c, sql: r.sql}
	_, err = proxyRepo.sweepOneExpiredProxy(ctx, *a[0].ProxyID, a[1].ProxyID, true)
	require.NoError(t, err)
	fresh, err := r.GetByID(ctx, root)
	require.NoError(t, err)
	require.Equal(t, a[0].ProxyID, fresh.ProxyID)
	require.False(t, fresh.Schedulable)
	_, err = r.SetLogicalAccountSchedulable(ctx, root, true)
	require.NoError(t, err)
	fresh, err = r.GetByID(ctx, root)
	require.NoError(t, err)
	require.False(t, fresh.Schedulable)
}

func TestAccountIPChannelsPostgresMigrationBackfillsOnlyCompatibleIdentities(t *testing.T) {
	r, c, ctx := openAICredentialGroupIntegrationRepo(t)
	compatible := createIPChannelFixture(t, r, c, ctx, "same", 2)
	conflict := createIPChannelFixture(t, r, c, ctx, "conflict", 2)
	_, err := c.Account.UpdateOneID(conflict[1].ID).SetRateMultiplier(2).Save(ctx)
	require.NoError(t, err)
	sql, err := os.ReadFile("../../migrations/251_account_ip_channels.sql")
	require.NoError(t, err)
	_, err = r.sql.ExecContext(ctx, string(sql))
	require.NoError(t, err)
	cs, err := r.GetAccountIPChannels(ctx, []int64{compatible[0].ID, conflict[0].ID})
	require.NoError(t, err)
	require.Len(t, cs[compatible[0].ID], 2)
	require.Empty(t, cs[conflict[0].ID])
	var n int
	require.NoError(t, scanSingleRow(ctx, r.sql, `SELECT count(*) FROM account_ip_merge_conflicts`, nil, &n))
	require.Equal(t, 1, n)
}

func TestAccountIPChannelsPostgresSingleProxyPatchEnrolsAtomically(t *testing.T) {
	r, c, ctx := ipChannelIntegrationRepo(t)
	a := createIPChannelFixture(t, r, c, ctx, "single", 1)
	id := a[0].ID
	off := false
	limit := 8
	require.NoError(t, r.PatchAccountIPChannel(ctx, id, id, service.AccountIPChannelPatch{Schedulable: &off, Concurrency: &limit}))
	cs, err := r.GetAccountIPChannels(ctx, []int64{id})
	require.NoError(t, err)
	require.Len(t, cs[id], 1)
	require.False(t, cs[id][0].Enabled)
	require.NotNil(t, cs[id][0].DisabledAt)
	require.Equal(t, 8, cs[id][0].Account.Concurrency)
}
