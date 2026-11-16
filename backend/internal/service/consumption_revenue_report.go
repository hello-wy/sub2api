package service

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/shopspring/decimal"
)

type ConsumptionRevenueComponent struct {
	Source          string          `json:"source"`
	GroupID         *int64          `json:"group_id"`
	GroupName       string          `json:"group_name"`
	Reason          string          `json:"reason"`
	Requests        int64           `json:"requests"`
	Credits         decimal.Decimal `json:"credits"`
	RevenueCNY      decimal.Decimal `json:"revenue_cny"`
	UnpricedCredits decimal.Decimal `json:"unpriced_credits"`
}

type ConsumptionRevenueReport struct {
	StartDate     string                        `json:"start_date"`
	EndDate       string                        `json:"end_date"`
	Timezone      string                        `json:"timezone"`
	UpdatedAt     time.Time                     `json:"updated_at"`
	Summary       DashboardConsumptionRevenue   `json:"summary"`
	ActualCredits decimal.Decimal               `json:"actual_credits"`
	Requests      int64                         `json:"requests"`
	Components    []ConsumptionRevenueComponent `json:"components"`
}

const consumptionRevenueComponentsSQL = consumptionRevenueCTE + `
SELECT c.source,c.group_id,COALESCE(g.name,''),c.reason,SUM(c.requests),SUM(c.credits),
 ROUND(SUM(c.balance+c.subscription),8),SUM(c.unpriced)
FROM components c LEFT JOIN groups g ON g.id=c.group_id
GROUP BY c.source,c.group_id,g.name,c.reason ORDER BY c.source,c.group_id NULLS FIRST,c.reason`

// ConsumptionRevenueRange uses the same site timezone as the dashboard, even
// when the optional operating ledger has a different accounting timezone.
func ConsumptionRevenueRange(startDate, endDate string, now time.Time) (time.Time, time.Time, error) {
	now = now.In(timezone.Location())
	if startDate == "" {
		startDate = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
	}
	if endDate == "" {
		endDate = now.Format("2006-01-02")
	}
	start, err := time.ParseInLocation("2006-01-02", startDate, now.Location())
	if err != nil {
		return time.Time{}, time.Time{}, operatingInvalid("开始日期无效")
	}
	end, err := time.ParseInLocation("2006-01-02", endDate, now.Location())
	if err != nil || end.Before(start) || end.After(now) || end.After(start.AddDate(1, 0, 0)) {
		return time.Time{}, time.Time{}, operatingInvalid("请选择不超过一年且不晚于今天的日期范围")
	}
	return start, end.AddDate(0, 0, 1), nil
}

func (s *BusinessAnalyticsService) ConsumptionRevenueReport(ctx context.Context, start, end time.Time) (*ConsumptionRevenueReport, error) {
	rows, err := s.db.QueryContext(ctx, consumptionRevenueComponentsSQL, start, end)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	result := &ConsumptionRevenueReport{StartDate: start.Format("2006-01-02"), EndDate: end.AddDate(0, 0, -1).Format("2006-01-02"), Timezone: start.Location().String(), UpdatedAt: time.Now(), Components: []ConsumptionRevenueComponent{}}
	for rows.Next() {
		var row ConsumptionRevenueComponent
		if err = rows.Scan(&row.Source, &row.GroupID, &row.GroupName, &row.Reason, &row.Requests, &row.Credits, &row.RevenueCNY, &row.UnpricedCredits); err != nil {
			return nil, err
		}
		result.Components = append(result.Components, row)
		result.Requests += row.Requests
		result.ActualCredits = result.ActualCredits.Add(row.Credits)
		result.Summary.UnpricedCredits = result.Summary.UnpricedCredits.Add(row.UnpricedCredits)
		if row.Source == "balance" {
			result.Summary.BalanceCNY = result.Summary.BalanceCNY.Add(row.RevenueCNY)
		} else {
			result.Summary.SubscriptionCNY = result.Summary.SubscriptionCNY.Add(row.RevenueCNY)
		}
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	result.Summary.TotalCNY = result.Summary.BalanceCNY.Add(result.Summary.SubscriptionCNY)
	return result, nil
}
