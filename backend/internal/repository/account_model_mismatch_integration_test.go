//go:build unit

package repository

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type mismatchSnapshotRecorder struct {
	service.SchedulerCache
	accounts map[int64]*service.Account
}

func (s *mismatchSnapshotRecorder) SetAccount(_ context.Context, account *service.Account) error {
	s.accounts[account.ID] = account
	return nil
}

func TestAccountModelMismatchPostgresQuarantinesLogicalFamilyAndExplicitResume(t *testing.T) {
	r, c, ctx := ipChannelIntegrationRepo(t)
	a := createIPChannelFixture(t, r, c, ctx, "mismatch-family", 3)
	root, err := r.JoinAccountIPChannels(ctx, []int64{a[0].ID, a[1].ID, a[2].ID})
	require.NoError(t, err)
	off := false
	require.NoError(t, r.PatchAccountIPChannel(ctx, root, a[2].ID, service.AccountIPChannelPatch{Schedulable: &off}))
	stale, err := r.GetByID(ctx, a[1].ID)
	require.NoError(t, err)
	cache := &mismatchSnapshotRecorder{accounts: map[int64]*service.Account{}}
	r.schedulerCache = cache
	require.NoError(t, r.MarkAccountModelMismatch(ctx, a[1].ID, "gpt-6-astra", "gpt-5.6-luna", "req-test"))
	for _, member := range a {
		require.NotNil(t, cache.accounts[member.ID], "sticky sessions must observe every channel's quarantine")
		require.True(t, cache.accounts[member.ID].HasModelMismatch())
		require.False(t, cache.accounts[member.ID].Schedulable)
		row, err := r.GetByID(ctx, member.ID)
		require.NoError(t, err)
		require.True(t, row.HasModelMismatch())
		require.False(t, row.Schedulable)
		require.False(t, row.IsSchedulable())
		marker := row.Extra[service.AccountModelMismatchExtraKey].(map[string]any)
		require.Equal(t, "gpt-6-astra", marker["expected_model"])
		require.Equal(t, float64(a[1].ID), marker["source_account_id"])
	}
	members, err := r.GetAccountIPChannels(ctx, []int64{root})
	require.NoError(t, err)
	require.False(t, members[root][0].LogicalEnabled)
	require.True(t, members[root][0].Enabled)
	require.False(t, members[root][2].Enabled)
	// A request's stale full snapshot, extra merge, generic auto recovery and
	// batch edit cannot erase quarantine or re-enable traffic.
	stale.Name = "edited while quarantined"
	require.NoError(t, r.Update(ctx, stale))
	require.NoError(t, r.UpdateExtra(ctx, a[1].ID, map[string]any{"model_mismatch": nil, "test": "retained"}))
	require.NoError(t, r.SetSchedulable(ctx, a[1].ID, true))
	_, err = r.BulkUpdate(ctx, []int64{a[1].ID}, service.AccountBulkUpdate{Extra: map[string]any{"model_mismatch": nil}})
	require.NoError(t, err)
	on := true
	_, err = r.BulkUpdate(ctx, []int64{a[1].ID}, service.AccountBulkUpdate{Schedulable: &on})
	require.ErrorContains(t, err, "降智")
	row, err := r.GetByID(ctx, a[1].ID)
	require.NoError(t, err)
	require.True(t, row.HasModelMismatch())
	require.False(t, row.Schedulable)
	p := pagination.PaginationParams{Page: 1, PageSize: 20}
	rows, total, err := r.ListLogicalAccounts(ctx, p, "", "", "model_mismatch", "", 0, "")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.EqualValues(t, 1, total.Total)
	for _, status := range []string{"active", "rate_limited", "temp_unschedulable", "unschedulable", "error", "inactive"} {
		rows, _, err := r.ListLogicalAccounts(ctx, p, "", "", status, "", 0, "")
		require.NoError(t, err)
		require.Empty(t, rows, status)
	}
	handled, err := r.ResumeAccountModelMismatch(ctx, root)
	require.True(t, handled)
	require.ErrorContains(t, err, "确认")
	confirmed := service.WithModelMismatchResumeConfirmation(ctx)
	handled, err = r.ResumeAccountModelMismatch(confirmed, a[1].ID)
	require.True(t, handled)
	require.ErrorContains(t, err, "逻辑账号")
	handled, err = r.ResumeAccountModelMismatch(confirmed, root)
	require.True(t, handled)
	require.NoError(t, err)
	for i, member := range a {
		require.False(t, cache.accounts[member.ID].HasModelMismatch())
		require.Equal(t, i != 2, cache.accounts[member.ID].Schedulable)
		row, err := r.GetByID(ctx, member.ID)
		require.NoError(t, err)
		require.False(t, row.HasModelMismatch())
		require.Equal(t, i != 2, row.Schedulable, "manually paused IP must remain paused")
	}
}

func TestAccountModelMismatchPostgresRetiredRootAndNewChannelStayBlocked(t *testing.T) {
	r, c, ctx := ipChannelIntegrationRepo(t)
	a := createIPChannelFixture(t, r, c, ctx, "mismatch-retired", 2)
	root, err := r.JoinAccountIPChannels(ctx, []int64{a[0].ID, a[1].ID})
	require.NoError(t, err)
	_, err = r.sql.ExecContext(ctx, `UPDATE account_ip_channels SET enabled=false,retired_at=NOW(),disabled_at=NOW() WHERE account_id=$1`, root)
	require.NoError(t, err)
	require.NoError(t, r.MarkAccountModelMismatch(ctx, a[1].ID, "gpt-6-astra", "gpt-5.6-luna", "req-retired"))
	proxy, err := c.Proxy.Create().SetName("added after mismatch").SetProtocol("http").SetHost("192.0.2.20").SetPort(8080).SetStatus(service.StatusActive).Save(ctx)
	require.NoError(t, err)
	require.NoError(t, r.AddAccountIPChannels(ctx, root, []int64{proxy.ID}, nil, nil))
	members, err := r.GetAccountIPChannels(ctx, []int64{root})
	require.NoError(t, err)
	require.Len(t, members[root], 2)
	for _, member := range members[root] {
		require.True(t, member.Account.HasModelMismatch())
		require.False(t, member.Account.Schedulable)
	}
	_, err = r.ResumeAccountModelMismatch(service.WithModelMismatchResumeConfirmation(ctx), root)
	require.NoError(t, err)
	retired, err := r.GetByID(ctx, root)
	require.NoError(t, err)
	require.False(t, retired.HasModelMismatch())
	require.False(t, retired.Schedulable)
}

func TestAccountModelMismatchPostgresAtomicOutboxAndConcurrentEdits(t *testing.T) {
	r, c, ctx := ipChannelIntegrationRepo(t)
	a := createIPChannelFixture(t, r, c, ctx, "mismatch-concurrent", 1)[0]
	var wg sync.WaitGroup
	errCh := make(chan error, 9)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errCh <- r.UpdateExtra(ctx, a.ID, map[string]any{"model_mismatch": nil, "test_update": time.Now().UnixNano()})
		}()
	}
	errCh <- r.MarkAccountModelMismatch(ctx, a.ID, "gpt-6-astra", "gpt-5.6-luna", "req-race")
	wg.Wait()
	close(errCh)
	for err := range errCh {
		require.NoError(t, err)
	}
	row, err := r.GetByID(ctx, a.ID)
	require.NoError(t, err)
	require.True(t, row.HasModelMismatch())
	var events int
	rows, err := r.sql.QueryContext(ctx, `SELECT count(*) FROM scheduler_outbox WHERE event_type=$1 AND payload->'account_ids' @> $2::jsonb`, service.SchedulerOutboxEventAccountBulkChanged, "["+itoa(int(a.ID))+"]")
	require.NoError(t, err)
	require.True(t, rows.Next())
	require.NoError(t, rows.Scan(&events))
	require.NoError(t, rows.Close())
	require.Greater(t, events, 0)
	_, err = r.sql.ExecContext(ctx, `DROP TABLE scheduler_outbox`)
	require.NoError(t, err)
	_, err = r.ResumeAccountModelMismatch(service.WithModelMismatchResumeConfirmation(ctx), a.ID)
	require.Error(t, err)
	row, err = r.GetByID(ctx, a.ID)
	require.NoError(t, err)
	require.True(t, row.HasModelMismatch(), "outbox failure must roll back resume")
	require.False(t, row.Schedulable)
}

func TestAccountModelMismatchPostgresBackupIgnoresInvalidMarkerAndPropagatesValidChannel(t *testing.T) {
	for _, tc := range []struct {
		name          string
		rootMarker    any
		channelMarker any
		quarantined   bool
	}{
		{name: "empty_object", rootMarker: map[string]any{}},
		{name: "string", rootMarker: "invalid legacy state"},
		{name: "array", rootMarker: []any{"invalid"}},
		{name: "null"},
		{name: "valid_channel_replaces_invalid_root", rootMarker: map[string]any{}, channelMarker: map[string]any{"expected_model": "gpt-6-astra", "actual_model": "gpt-5.6-luna"}, quarantined: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, c, ctx := ipChannelIntegrationRepo(t)
			accounts := createIPChannelFixture(t, r, c, ctx, "restore-marker", 2)
			root := *accounts[0]
			root.ID = 0
			root.Extra = copyJSONMap(root.Extra)
			root.Extra[service.AccountModelMismatchExtraKey] = tc.rootMarker
			channels := []service.AccountIPChannelBackup{
				{ProxyID: *accounts[0].ProxyID, IsRoot: true, Enabled: true, Status: service.StatusActive, Extra: map[string]any{service.AccountModelMismatchExtraKey: tc.rootMarker}},
				{ProxyID: *accounts[1].ProxyID, Enabled: true, Status: service.StatusActive, Extra: map[string]any{service.AccountModelMismatchExtraKey: tc.channelMarker}},
			}
			require.NoError(t, r.RestoreAccountIPChannels(ctx, &root, channels, true, false))
			members, err := r.GetAccountIPChannels(ctx, []int64{root.ID})
			require.NoError(t, err)
			require.Len(t, members[root.ID], 2)
			for _, member := range members[root.ID] {
				require.Equal(t, !tc.quarantined, member.LogicalEnabled)
				require.Equal(t, !tc.quarantined, member.Account.Schedulable)
				require.Equal(t, tc.quarantined, member.Account.HasModelMismatch())
			}
		})
	}
}

func TestAccountModelMismatchPostgresDisabledRecordsWithoutChangingScheduling(t *testing.T) {
	r, c, ctx := ipChannelIntegrationRepo(t)
	a := createIPChannelFixture(t, r, c, ctx, "mismatch-observe", 3)
	root, err := r.JoinAccountIPChannels(ctx, []int64{a[0].ID, a[1].ID, a[2].ID})
	require.NoError(t, err)
	off := false
	require.NoError(t, r.PatchAccountIPChannel(ctx, root, a[2].ID, service.AccountIPChannelPatch{Schedulable: &off}))
	settings := service.NewSettingService(NewSettingRepository(c), nil)
	_, err = settings.UpdateModelMismatchSettings(ctx, service.ModelMismatchSettings{AutoQuarantineEnabled: false})
	require.NoError(t, err)
	gateCalls := 0
	gate := func([]int64) { gateCalls++ }
	quarantined, err := r.RecordAccountModelMismatch(ctx, a[1].ID, "gpt-6-astra", "gpt-5.6-luna", "observe-1", gate)
	require.NoError(t, err)
	require.False(t, quarantined)
	require.Zero(t, gateCalls, "observation persistence/cache publication must not register a gate")
	for i, member := range a {
		row, err := r.GetByID(ctx, member.ID)
		require.NoError(t, err)
		require.True(t, row.HasModelMismatch())
		require.False(t, row.IsModelMismatchQuarantined())
		require.Equal(t, i != 2, row.Schedulable)
		require.Equal(t, i != 2, row.IsSchedulable())
		// Full edits and background state writes must not convert observations
		// back into quarantine or erase their display evidence.
		require.NoError(t, r.Update(ctx, row))
		require.NoError(t, r.UpdateExtra(ctx, row.ID, map[string]any{"model_mismatch": nil}))
	}
	members, err := r.GetAccountIPChannels(ctx, []int64{root})
	require.NoError(t, err)
	for _, member := range members[root] {
		require.True(t, member.LogicalEnabled)
	}
	active, _, err := r.ListLogicalAccounts(ctx, pagination.PaginationParams{Page: 1, PageSize: 20}, "", "", "active", "", 0, "")
	require.NoError(t, err)
	require.Len(t, active, 1, "observed active accounts must remain in the active filter")
	detected, _, err := r.ListLogicalAccounts(ctx, pagination.PaginationParams{Page: 1, PageSize: 20}, "", "", "model_mismatch", "", 0, "")
	require.NoError(t, err)
	require.Len(t, detected, 1)
	routing, err := r.ListSchedulable(ctx)
	require.NoError(t, err)
	require.Len(t, routing, 2)
	// Ordinary pause/resume remains usable for observation-only accounts.
	on := true
	_, err = r.BulkUpdate(ctx, []int64{a[0].ID}, service.AccountBulkUpdate{Schedulable: &on})
	require.NoError(t, err)
	_, err = r.SetLogicalAccountSchedulable(ctx, root, false)
	require.NoError(t, err)
	_, err = r.SetLogicalAccountSchedulable(ctx, root, true)
	require.NoError(t, err)
	// Turning protection on does not rewrite history. The next detection must
	// promote observations into quarantine for the complete logical family.
	_, err = settings.UpdateModelMismatchSettings(ctx, service.ModelMismatchSettings{AutoQuarantineEnabled: true})
	require.NoError(t, err)
	quarantined, err = r.RecordAccountModelMismatch(ctx, a[1].ID, "gpt-6-astra", "gpt-5.6-luna", "quarantine-2", gate)
	require.NoError(t, err)
	require.True(t, quarantined)
	require.Positive(t, gateCalls)
	_, err = settings.UpdateModelMismatchSettings(ctx, service.ModelMismatchSettings{AutoQuarantineEnabled: false})
	require.NoError(t, err)
	quarantined, err = r.RecordAccountModelMismatch(ctx, a[1].ID, "gpt-6-astra", "other-model", "observe-3")
	require.NoError(t, err)
	require.False(t, quarantined, "unrelated differences must not create quarantine; existing valid quarantine remains unchanged")
	for _, member := range a {
		row, err := r.GetByID(ctx, member.ID)
		require.NoError(t, err)
		require.True(t, row.IsModelMismatchQuarantined())
		require.False(t, row.Schedulable)
		require.Equal(t, "quarantine-2", row.Extra[service.AccountModelMismatchExtraKey].(map[string]any)["request_id"])
	}
	_, err = r.ResumeAccountModelMismatch(service.WithModelMismatchResumeConfirmation(ctx), root)
	require.NoError(t, err)
	paused, err := r.GetByID(ctx, a[2].ID)
	require.NoError(t, err)
	require.False(t, paused.Schedulable, "individually paused IP remains paused after explicit recovery")
}
