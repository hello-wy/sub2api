package admin

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

var consumptionRevenueReportCache = newSnapshotCache(30 * time.Second)

func (h *DashboardHandler) GetConsumptionRevenue(c *gin.Context) {
	s := h.operating(c)
	if s == nil {
		return
	}
	start, end, err := service.ConsumptionRevenueRange(c.Query("start_date"), c.Query("end_date"), time.Now())
	if err != nil {
		operatingError(c, err)
		return
	}
	entry, _, err := consumptionRevenueReportCache.GetOrLoad(start.Format(time.RFC3339)+"/"+end.Format(time.RFC3339), func() (any, error) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
		defer cancel()
		return s.ConsumptionRevenueReport(ctx, start, end)
	})
	if err != nil {
		operatingError(c, err)
		return
	}
	response.Success(c, entry.Payload)
}
