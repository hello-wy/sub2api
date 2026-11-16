package admin

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestDashboardConsumptionRevenueCacheAndFailure(t *testing.T) {
	dashboardConsumptionRevenueCache = newSnapshotCache(30 * time.Second)
	t.Cleanup(func() { dashboardConsumptionRevenueCache = newSnapshotCache(30 * time.Second) })
	h := NewDashboardHandler(nil, nil)
	require.Nil(t, h.todayConsumptionRevenue(context.Background()))
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	h.SetBusinessAnalyticsService(service.NewBusinessAnalyticsService(db))
	start := timezone.Today()
	mock.ExpectQuery("WITH day_usage").WithArgs(start, start.AddDate(0, 0, 1)).WillReturnError(errors.New("database unavailable"))
	require.Nil(t, h.todayConsumptionRevenue(context.Background()))
	// Failure must not cache a fake zero or poison the next successful response.
	mock.ExpectQuery("WITH day_usage").WithArgs(start, start.AddDate(0, 0, 1)).WillReturnRows(sqlmock.NewRows([]string{"balance", "subscription", "unpriced"}).AddRow("12.5", "7.5", "2"))
	value := h.todayConsumptionRevenue(context.Background())
	require.NotNil(t, value)
	require.Equal(t, "20", value.TotalCNY.String())
	require.Same(t, value, h.todayConsumptionRevenue(context.Background()))
	encoded, err := json.Marshal(dashboardSnapshotV2Stats{TodayConsumptionRevenue: value})
	require.NoError(t, err)
	require.Contains(t, string(encoded), `"today_consumption_revenue":{"balance_cny":"12.5","subscription_cny":"7.5","total_cny":"20","unpriced_credits":"2"}`)
	require.NoError(t, mock.ExpectationsWereMet())
}
