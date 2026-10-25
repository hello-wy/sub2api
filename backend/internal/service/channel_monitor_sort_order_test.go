//go:build unit

package service

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

type monitorOrderRepoStub struct {
	ChannelMonitorRepository
	ids []int64
}

func (r *monitorOrderRepoStub) UpdateSortOrder(_ context.Context, ids []int64) error {
	r.ids = append([]int64(nil), ids...)
	return nil
}

func TestMonitorSortOrderBatchValidation(t *testing.T) {
	repo := &monitorOrderRepoStub{}
	svc := NewChannelMonitorService(repo, nil)
	for _, ids := range [][]int64{nil, {}, {1, 1}, {0}, {-1}, make([]int64, 10001)} {
		require.ErrorIs(t, svc.UpdateSortOrder(context.Background(), ids), ErrChannelMonitorInvalidOrder)
		require.Nil(t, repo.ids)
	}
	require.NoError(t, svc.UpdateSortOrder(context.Background(), []int64{42, 7, 8}))
	require.Equal(t, []int64{42, 7, 8}, repo.ids)
}

func TestMonitorSortOrderValidation(t *testing.T) {
	for _, order := range []int{0, 10, 2147483647} {
		require.NoError(t, validateMonitorSortOrder(order))
	}
	for _, order := range []int{-1, 2147483648} {
		require.ErrorIs(t, validateCreateParams(ChannelMonitorCreateParams{SortOrder: order}), ErrChannelMonitorInvalidSortOrder)
		monitor := &ChannelMonitor{SortOrder: 20}
		require.ErrorIs(t, applyMonitorUpdate(monitor, ChannelMonitorUpdateParams{SortOrder: &order}), ErrChannelMonitorInvalidSortOrder)
		require.Equal(t, 20, monitor.SortOrder)
	}
}

func TestMonitorSortOrderUpdatePreservesIdentityAndState(t *testing.T) {
	checkedAt := time.Now().UTC()
	original := ChannelMonitor{
		ID: 42, Name: "Plus", Provider: MonitorProviderOpenAI, Endpoint: "https://api.openai.com",
		APIMode: MonitorAPIModeChatCompletions, APIKey: "encrypted", PrimaryModel: "gpt-test", Enabled: true,
		SortOrder: 20, LastCheckedAt: &checkedAt, CreatedAt: checkedAt, UpdatedAt: checkedAt,
	}
	monitor := original
	require.NoError(t, applyMonitorUpdate(&monitor, ChannelMonitorUpdateParams{}))
	require.Equal(t, original, monitor)
	zero := 0
	require.NoError(t, applyMonitorUpdate(&monitor, ChannelMonitorUpdateParams{SortOrder: &zero}))
	original.SortOrder = 0
	require.Equal(t, original, monitor)
}
