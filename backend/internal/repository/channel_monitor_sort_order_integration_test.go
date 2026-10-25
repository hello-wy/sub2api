//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestChannelMonitorSortOrderPreservesHistory(t *testing.T) {
	ctx := context.Background()
	repo := NewChannelMonitorRepository(integrationEntClient, integrationDB)
	const prefix = "sort-order-history-test-"
	monitors := make([]*service.ChannelMonitor, 0, 3)
	checkedAt := time.Now().UTC().Add(-24 * time.Hour).Truncate(time.Microsecond)
	for i, order := range []int{30, 10, 10} {
		monitor := &service.ChannelMonitor{
			Name: prefix + string(rune('a'+i)), Provider: service.MonitorProviderOpenAI,
			Endpoint: "https://api.openai.com", APIKey: "encrypted", PrimaryModel: "gpt-test",
			Enabled: true, IntervalSeconds: 60, SortOrder: order,
		}
		require.NoError(t, repo.Create(ctx, monitor))
		t.Cleanup(func() { require.NoError(t, repo.Delete(ctx, monitor.ID)) })
		monitors = append(monitors, monitor)
		require.NoError(t, repo.InsertHistoryBatch(ctx, []*service.ChannelMonitorHistoryRow{{
			MonitorID: monitor.ID, Model: monitor.PrimaryModel, Status: service.MonitorStatusOperational,
			Message: monitor.Name, CheckedAt: checkedAt,
		}}))
		require.NoError(t, repo.MarkChecked(ctx, monitor.ID, checkedAt))
	}
	_, err := repo.UpsertDailyRollupsFor(ctx, checkedAt)
	require.NoError(t, err)
	assertOrder := func(want []int64) {
		t.Helper()
		rows, total, err := repo.List(ctx, service.ChannelMonitorListParams{Search: prefix, PageSize: 100})
		require.NoError(t, err)
		require.EqualValues(t, 3, total)
		ids := make([]int64, 0, len(rows))
		for _, row := range rows {
			ids = append(ids, row.ID)
		}
		require.Equal(t, want, ids)
		rows, err = repo.ListEnabled(ctx)
		require.NoError(t, err)
		ids = ids[:0]
		for _, row := range rows {
			for _, monitor := range monitors {
				if row.ID == monitor.ID {
					ids = append(ids, row.ID)
				}
			}
		}
		require.Equal(t, want, ids)
	}
	assertOrder([]int64{monitors[1].ID, monitors[2].ID, monitors[0].ID})
	// Compare every field of the original history and rollup rows, including IDs.
	snapshot := func() string {
		t.Helper()
		var data string
		err := integrationDB.QueryRowContext(ctx, `SELECT jsonb_build_object(
   'history', (SELECT jsonb_agg(to_jsonb(h) ORDER BY h.id)
    FROM channel_monitor_histories h JOIN channel_monitors m ON m.id=h.monitor_id WHERE m.name LIKE $1),
   'rollups', (SELECT jsonb_agg(to_jsonb(r) ORDER BY r.id)
    FROM channel_monitor_daily_rollups r JOIN channel_monitors m ON m.id=r.monitor_id WHERE m.name LIKE $1)
  )::text`, prefix+"%").Scan(&data)
		require.NoError(t, err)
		return data
	}
	before := snapshot()
	require.NotContains(t, before, `"rollups": null`)
	loaded, err := repo.GetByID(ctx, monitors[0].ID)
	require.NoError(t, err)
	createdAt := loaded.CreatedAt
	loaded.SortOrder = 0
	require.NoError(t, repo.Update(ctx, loaded))
	assertOrder([]int64{monitors[0].ID, monitors[1].ID, monitors[2].ID})
	require.JSONEq(t, before, snapshot())
	// Drag sorting only changes order fields, and a stale/deleted ID rolls the
	// entire batch back without partially reordering any surviving monitor.
	require.NoError(t, repo.UpdateSortOrder(ctx, []int64{monitors[2].ID, monitors[0].ID, monitors[1].ID}))
	assertOrder([]int64{monitors[2].ID, monitors[0].ID, monitors[1].ID})
	require.JSONEq(t, before, snapshot())
	require.ErrorIs(t, repo.UpdateSortOrder(ctx, []int64{monitors[1].ID, 1 << 62}), service.ErrChannelMonitorNotFound)
	assertOrder([]int64{monitors[2].ID, monitors[0].ID, monitors[1].ID})
	require.JSONEq(t, before, snapshot())
	reloaded, err := repo.GetByID(ctx, monitors[0].ID)
	require.NoError(t, err)
	require.Equal(t, createdAt, reloaded.CreatedAt)
	require.Equal(t, &checkedAt, reloaded.LastCheckedAt)
	require.Equal(t, 10, reloaded.SortOrder)
}
