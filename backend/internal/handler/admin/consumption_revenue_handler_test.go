package admin

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestConsumptionRevenueHandler(t *testing.T) {
	consumptionRevenueReportCache = newSnapshotCache(time.Minute)
	t.Cleanup(func() { consumptionRevenueReportCache = newSnapshotCache(30 * time.Second) })
	h := NewDashboardHandler(nil, nil)
	router := gin.New()
	router.GET("/business/consumption-revenue", h.GetConsumptionRevenue)
	read := func(query string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest("GET", "/business/consumption-revenue"+query, nil))
		return w
	}
	require.Equal(t, 503, read("").Code)
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { mock.ExpectClose(); require.NoError(t, db.Close()) })
	h.SetBusinessAnalyticsService(service.NewBusinessAnalyticsService(db))
	require.Equal(t, 400, read("?start_date=invalid").Code)
	mock.ExpectQuery("WITH day_usage").WillReturnRows(sqlmock.NewRows([]string{"source", "group_id", "name", "reason", "requests", "credits", "cny", "unpriced"}).AddRow("balance", nil, "", "priced", 2, "10", "8", "0").AddRow("subscription", 49, "测试套餐", "missing_period_quota", 1, "1.49", "0", "1.49"))
	w := read("?start_date=2026-01-01&end_date=2026-01-01")
	require.Equal(t, 200, w.Code)
	require.Contains(t, w.Body.String(), `"total_cny":"8"`)
	require.Contains(t, w.Body.String(), `"actual_credits":"11.49"`)
	require.Contains(t, w.Body.String(), `"reason":"missing_period_quota"`)
	require.Equal(t, 200, read("?start_date=2026-01-01&end_date=2026-01-01").Code)
	require.NoError(t, mock.ExpectationsWereMet())
}
