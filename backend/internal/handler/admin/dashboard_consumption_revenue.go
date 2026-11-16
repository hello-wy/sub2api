package admin

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

var dashboardConsumptionRevenueCache = newSnapshotCache(30 * time.Second)

func (h *DashboardHandler) todayConsumptionRevenue(ctx context.Context) *service.DashboardConsumptionRevenue {
	if h.businessService == nil {
		return nil
	}
	start := timezone.Today()
	value, _, err := dashboardConsumptionRevenueCache.GetOrLoad(start.Format(time.RFC3339), func() (any, error) {
		queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		return h.businessService.DashboardConsumptionRevenue(queryCtx, start, start.AddDate(0, 0, 1))
	})
	if err != nil {
		logger.LegacyPrintf("handler.dashboard", "[Dashboard] 消费折合人民币统计失败: %v", err)
		return nil
	}
	result, _ := value.Payload.(*service.DashboardConsumptionRevenue)
	return result
}
