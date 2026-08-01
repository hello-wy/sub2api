package service

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/lib/pq"
)

// BusinessAnalyticsService turns existing usage, payment, balance, subscription,
// and account-pool data into an explicitly labelled operational estimate. API key
// accounts contribute variable cost using their score cost and CNY conversion;
// OAuth and other fixed-cost accounts contribute only through the manual ledger.
type BusinessAnalyticsService struct {
	db *sql.DB
}

func NewBusinessAnalyticsService(db *sql.DB) *BusinessAnalyticsService {
	return &BusinessAnalyticsService{db: db}
}

type BusinessAnalyticsOverview struct {
	StartAt              time.Time                 `json:"start_at"`
	EndAt                time.Time                 `json:"end_at"`
	Settings             BusinessAnalyticsSettings `json:"settings"`
	UsageCreditsUSD      float64                   `json:"usage_credits_usd"`
	UsageRevenueCNY      float64                   `json:"usage_revenue_cny"`
	APIKeyUsageCostUSD   float64                   `json:"api_key_usage_cost_usd"`
	APIKeyUsageCostCNY   float64                   `json:"api_key_usage_cost_cny"`
	UnpricedAPIKeyUSD    float64                   `json:"unpriced_api_key_usage_cost_usd"`
	ProfitComplete       bool                      `json:"profit_complete"`
	GrossProfitCNY       float64                   `json:"gross_profit_cny"`
	GrossMargin          float64                   `json:"gross_margin"`
	WelfareGrantedUSD    float64                   `json:"welfare_granted_usd"`
	WelfareCostCNY       float64                   `json:"welfare_cost_cny"`
	AccountCostCNY       float64                   `json:"account_cost_cny"`
	OperatingProfitCNY   float64                   `json:"operating_profit_cny"`
	OperatingMargin      float64                   `json:"operating_margin"`
	Cumulative           BusinessProfitSummary     `json:"cumulative"`
	CostLedgerConfigured bool                      `json:"cost_ledger_configured"`
	CashReceipts         []BusinessCurrencyAmount  `json:"cash_receipts"`
	Liabilities          BusinessLiabilitySummary  `json:"liabilities"`
	Daily                []BusinessDailyAnalytics  `json:"daily"`
	Groups               []BusinessGroupAnalytics  `json:"groups"`
	SnapshotCapturedAt   *time.Time                `json:"snapshot_captured_at,omitempty"`
}

type BusinessProfitSummary struct {
	StartAt            time.Time `json:"start_at"`
	EndAt              time.Time `json:"end_at"`
	UsageRevenueCNY    float64   `json:"usage_revenue_cny"`
	APIKeyUsageCostCNY float64   `json:"api_key_usage_cost_cny"`
	WelfareCostCNY     float64   `json:"welfare_cost_cny"`
	AccountCostCNY     float64   `json:"account_cost_cny"`
	TotalCostCNY       float64   `json:"total_cost_cny"`
	OperatingProfitCNY float64   `json:"operating_profit_cny"`
	OperatingMargin    float64   `json:"operating_margin"`
	UnpricedAPIKeyUSD  float64   `json:"unpriced_api_key_usage_cost_usd"`
	ProfitComplete     bool      `json:"profit_complete"`
}

type BusinessAnalyticsSettings struct {
	BalanceCreditsPerCNY float64 `json:"balance_credits_per_cny"`
}

type BusinessCurrencyAmount struct {
	Currency string  `json:"currency"`
	Amount   float64 `json:"amount"`
}

type BusinessLiabilitySummary struct {
	BalanceCreditsUSD            float64 `json:"balance_credits_usd"`
	BalanceFaceValueCNY          float64 `json:"balance_face_value_cny"`
	BalanceEstimatedCostCNY      float64 `json:"balance_estimated_cost_cny"`
	ActiveSubscriptions          int64   `json:"active_subscriptions"`
	SubscriptionCommitmentUSD    float64 `json:"subscription_commitment_usd"`
	SubscriptionEstimatedCostCNY float64 `json:"subscription_estimated_cost_cny"`
	UnlimitedSubscriptions       int64   `json:"unlimited_subscriptions"`
}

type BusinessGroupAnalytics struct {
	GroupID                    int64   `json:"group_id"`
	GroupName                  string  `json:"group_name"`
	EffectiveRateMultiplier    float64 `json:"effective_rate_multiplier"`
	UsageCreditsUSD            float64 `json:"usage_credits_usd"`
	UsageRevenueCNY            float64 `json:"usage_revenue_cny"`
	APIKeyUsageCostUSD         float64 `json:"api_key_usage_cost_usd"`
	APIKeyUsageCostCNY         float64 `json:"api_key_usage_cost_cny"`
	UnpricedAPIKeyUSD          float64 `json:"unpriced_api_key_usage_cost_usd"`
	ProfitComplete             bool    `json:"profit_complete"`
	GrossProfitCNY             float64 `json:"gross_profit_cny"`
	GrossMargin                float64 `json:"gross_margin"`
	AllocatedWelfareCostCNY    float64 `json:"allocated_welfare_cost_cny"`
	AllocatedAccountCostCNY    float64 `json:"allocated_account_cost_cny"`
	OperatingProfitCNY         float64 `json:"operating_profit_cny"`
	ForecastP50DailyCostUSD    float64 `json:"forecast_p50_daily_cost_usd"`
	ForecastP95DailyCostUSD    float64 `json:"forecast_p95_daily_cost_usd"`
	ObservedCapacityPerAccount float64 `json:"observed_capacity_per_account"`
	SchedulableAccounts        int64   `json:"schedulable_accounts"`
	ConcurrencyMax             int64   `json:"concurrency_max"`
	RequiredAccounts           int64   `json:"required_accounts"`
	AdditionalAccounts         int64   `json:"additional_accounts"`
}

type BusinessDailyAnalytics struct {
	Date               string  `json:"date"`
	UsageRevenueCNY    float64 `json:"usage_revenue_cny"`
	APIKeyUsageCostCNY float64 `json:"api_key_usage_cost_cny"`
	UnpricedAPIKeyUSD  float64 `json:"unpriced_api_key_usage_cost_usd"`
	ProfitComplete     bool    `json:"profit_complete"`
	WelfareGrantedUSD  float64 `json:"welfare_granted_usd"`
	WelfareCostCNY     float64 `json:"welfare_cost_cny"`
	AccountCostCNY     float64 `json:"account_cost_cny"`
	OperatingProfitCNY float64 `json:"operating_profit_cny"`
}

type BusinessAccountCost struct {
	ID        int64     `json:"id"`
	AccountID *int64    `json:"account_id,omitempty"`
	GroupID   *int64    `json:"group_id,omitempty"`
	CostType  string    `json:"cost_type"`
	Amount    float64   `json:"amount"`
	Currency  string    `json:"currency"`
	FXRate    float64   `json:"fx_rate"`
	StartsAt  time.Time `json:"starts_at"`
	EndsAt    time.Time `json:"ends_at"`
	Notes     string    `json:"notes"`
	CreatedAt time.Time `json:"created_at"`
}

type BusinessAccountCostInput struct {
	AccountID *int64    `json:"account_id"`
	GroupID   *int64    `json:"group_id"`
	CostType  string    `json:"cost_type"`
	Amount    float64   `json:"amount"`
	Currency  string    `json:"currency"`
	FXRate    float64   `json:"fx_rate"`
	StartsAt  time.Time `json:"starts_at"`
	EndsAt    time.Time `json:"ends_at"`
	Notes     string    `json:"notes"`
}

type BusinessAPIKeyAccount struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Platform string `json:"platform"`
}

type BusinessAPIKeyCostRate struct {
	ID            int64     `json:"id"`
	AccountID     int64     `json:"account_id"`
	CreditsPerCNY float64   `json:"credits_per_cny"`
	Notes         string    `json:"notes"`
	CreatedAt     time.Time `json:"created_at"`
}

type BusinessAPIKeyCostRateInput struct {
	AccountID     int64   `json:"account_id"`
	CreditsPerCNY float64 `json:"credits_per_cny"`
	Notes         string  `json:"notes"`
}

type BusinessAPIKeyCostRateConfig struct {
	Accounts []BusinessAPIKeyAccount  `json:"accounts"`
	Rates    []BusinessAPIKeyCostRate `json:"rates"`
}

type businessDailyGroup struct {
	groupID         int64
	groupName       string
	day             time.Time
	baseCredits     float64
	usageCredits    float64
	apiKeyUsageCost float64
	apiKeyCostCNY   float64
	unpricedAPIKey  float64
	capacityUsage   float64
	accountCount    int64
}

type businessGroupAccumulator struct {
	groupID             int64
	groupName           string
	baseCredits         float64
	usageCredits        float64
	apiKeyUsageCost     float64
	apiKeyCostCNY       float64
	unpricedAPIKey      float64
	dailyCapacityUsage  []float64
	dailyAccountCounts  []int64
	accountCostCNY      float64
	schedulableAccounts int64
	concurrencyMax      int64
	snapshotCapacity    float64
}

type businessDailyAccumulator struct {
	date               string
	usageCredits       float64
	apiKeyUsageCostUSD float64
	apiKeyUsageCostCNY float64
	unpricedAPIKeyUSD  float64
	welfareGrantedUSD  float64
	accountCostCNY     float64
}

func (s *BusinessAnalyticsService) GetOverview(ctx context.Context, start, end time.Time) (*BusinessAnalyticsOverview, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("business analytics database is unavailable")
	}
	if !end.After(start) {
		return nil, fmt.Errorf("end time must be after start time")
	}

	settings, err := s.GetSettings(ctx)
	if err != nil {
		return nil, err
	}
	groups, daily, err := s.loadGroupUsage(ctx, start, end)
	if err != nil {
		return nil, err
	}
	groupCosts, unallocatedAccountCost, totalAccountCost, configured, err := s.loadAccruedCosts(ctx, start, end, daily)
	if err != nil {
		return nil, err
	}
	for id, cost := range groupCosts {
		if g, ok := groups[id]; ok {
			g.accountCostCNY += cost
		} else {
			// A group with no usage in the selected period cannot receive a
			// revenue-based row. Keep its cost in the unallocated pool so group
			// profit still reconciles with the overview whenever revenue exists.
			unallocatedAccountCost += cost
		}
	}
	welfareGrantedUSD, err := s.loadWelfare(ctx, start, end, daily)
	if err != nil {
		return nil, err
	}
	if err := s.loadPoolCapacity(ctx, groups); err != nil {
		return nil, err
	}
	if err := s.loadSnapshotCapacity(ctx, groups); err != nil {
		return nil, err
	}

	overview := &BusinessAnalyticsOverview{
		StartAt:              start,
		EndAt:                end,
		Settings:             *settings,
		WelfareGrantedUSD:    welfareGrantedUSD,
		AccountCostCNY:       totalAccountCost,
		CostLedgerConfigured: configured,
	}
	for _, g := range groups {
		overview.UsageCreditsUSD += g.usageCredits
		overview.APIKeyUsageCostUSD += g.apiKeyUsageCost
		overview.APIKeyUsageCostCNY += g.apiKeyCostCNY
		overview.UnpricedAPIKeyUSD += g.unpricedAPIKey
	}
	overview.ProfitComplete = overview.UnpricedAPIKeyUSD <= 1e-12
	overview.UsageRevenueCNY = creditsToCNY(overview.UsageCreditsUSD, settings.BalanceCreditsPerCNY)
	overview.WelfareCostCNY = creditsToCNY(overview.WelfareGrantedUSD, settings.BalanceCreditsPerCNY)
	overview.GrossProfitCNY = overview.UsageRevenueCNY - overview.APIKeyUsageCostCNY
	overview.GrossMargin = percentage(overview.GrossProfitCNY, overview.UsageRevenueCNY)
	overview.OperatingProfitCNY = overview.GrossProfitCNY - overview.WelfareCostCNY - overview.AccountCostCNY
	overview.OperatingMargin = percentage(overview.OperatingProfitCNY, overview.UsageRevenueCNY)
	if overview.Cumulative, err = s.loadCumulativeProfit(ctx, end, *settings); err != nil {
		return nil, err
	}

	if overview.CashReceipts, err = s.loadCashReceipts(ctx, start, end); err != nil {
		return nil, err
	}
	if overview.Liabilities, err = s.loadLiabilities(ctx, overview.APIKeyUsageCostCNY, overview.UsageCreditsUSD, *settings); err != nil {
		return nil, err
	}
	if overview.SnapshotCapturedAt, err = s.latestSnapshotTime(ctx); err != nil {
		return nil, err
	}

	items := make([]BusinessGroupAnalytics, 0, len(groups))
	for _, g := range groups {
		p50 := mean(g.dailyCapacityUsage)
		p95 := percentile(g.dailyCapacityUsage, 0.95)
		capacity := meanPerAccount(g.dailyCapacityUsage, g.dailyAccountCounts)
		if g.snapshotCapacity > 0 {
			// A persisted 24-hour snapshot remains usable after raw usage-log
			// retention expires. Prefer the smaller value as the safer baseline.
			if capacity == 0 || g.snapshotCapacity < capacity {
				capacity = g.snapshotCapacity
			}
		}
		required := int64(0)
		if p95 > 0 && capacity > 0 {
			// 75% is the operating headroom: use conservative observed capacity
			// rather than assuming every account can be exhausted continuously.
			required = int64(math.Ceil(p95 / (capacity * 0.75)))
		}
		additional := required - g.schedulableAccounts
		if additional < 0 {
			additional = 0
		}
		revenueCNY := creditsToCNY(g.usageCredits, settings.BalanceCreditsPerCNY)
		revenueShare := share(revenueCNY, overview.UsageRevenueCNY)
		allocatedWelfare := overview.WelfareCostCNY * revenueShare
		allocatedAccountCost := g.accountCostCNY + unallocatedAccountCost*revenueShare
		grossProfit := revenueCNY - g.apiKeyCostCNY
		items = append(items, BusinessGroupAnalytics{
			GroupID:                    g.groupID,
			GroupName:                  g.groupName,
			EffectiveRateMultiplier:    share(g.usageCredits, g.baseCredits),
			UsageCreditsUSD:            g.usageCredits,
			UsageRevenueCNY:            revenueCNY,
			APIKeyUsageCostUSD:         g.apiKeyUsageCost,
			APIKeyUsageCostCNY:         g.apiKeyCostCNY,
			UnpricedAPIKeyUSD:          g.unpricedAPIKey,
			ProfitComplete:             g.unpricedAPIKey <= 1e-12,
			GrossProfitCNY:             grossProfit,
			GrossMargin:                percentage(grossProfit, revenueCNY),
			AllocatedWelfareCostCNY:    allocatedWelfare,
			AllocatedAccountCostCNY:    allocatedAccountCost,
			OperatingProfitCNY:         grossProfit - allocatedWelfare - allocatedAccountCost,
			ForecastP50DailyCostUSD:    p50,
			ForecastP95DailyCostUSD:    p95,
			ObservedCapacityPerAccount: capacity,
			SchedulableAccounts:        g.schedulableAccounts,
			ConcurrencyMax:             g.concurrencyMax,
			RequiredAccounts:           required,
			AdditionalAccounts:         additional,
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].UsageRevenueCNY > items[j].UsageRevenueCNY })
	overview.Groups = items
	overview.Daily = buildDailyAnalytics(daily, *settings)
	return overview, nil
}

func (s *BusinessAnalyticsService) loadCumulativeProfit(ctx context.Context, end time.Time, settings BusinessAnalyticsSettings) (BusinessProfitSummary, error) {
	const query = `
		WITH activity_start AS (
		    SELECT LEAST(
		        COALESCE((SELECT MIN(ul.created_at) FROM usage_logs ul JOIN users u ON u.id = ul.user_id WHERE ul.group_id > 0 AND ul.created_at < $1 AND u.role <> 'admin'), $1),
		        COALESCE((SELECT MIN(wr.created_at) FROM welfare_records wr JOIN users u ON u.id = wr.user_id WHERE wr.status = 'success' AND wr.created_at < $1 AND u.role <> 'admin'), $1),
		        COALESCE((SELECT MIN(dc.created_at) FROM daily_checkin_records dc JOIN users u ON u.id = dc.user_id WHERE dc.status = 'success' AND dc.created_at < $1 AND u.role <> 'admin'), $1),
		        COALESCE((SELECT MIN(starts_at) FROM business_account_costs WHERE starts_at < $1), $1)
		    ) AS starts_at
		), usage_totals AS (
		    SELECT COALESCE(SUM(ul.actual_cost), 0) AS usage_credits,
		           COALESCE(SUM(CASE WHEN a.type = 'apikey' AND cost_rate.credits_per_cny IS NOT NULL
		               THEN COALESCE(ul.account_stats_cost, ul.total_cost) / cost_rate.credits_per_cny
		               ELSE 0 END), 0) AS api_key_cost_cny,
		           COALESCE(SUM(CASE WHEN a.type = 'apikey' AND cost_rate.credits_per_cny IS NULL
		               THEN COALESCE(ul.account_stats_cost, ul.total_cost)
		               ELSE 0 END), 0) AS unpriced_api_key
		    FROM usage_logs ul
		    JOIN users u ON u.id = ul.user_id
		    LEFT JOIN accounts a ON a.id = ul.account_id
		    LEFT JOIN business_api_key_cost_rates cost_rate ON cost_rate.account_id = ul.account_id
		    WHERE ul.created_at < $1 AND ul.group_id > 0 AND u.role <> 'admin'
		), welfare_totals AS (
		    SELECT COALESCE(SUM(amount), 0) AS welfare_credits
		    FROM (
		        SELECT wr.amount FROM welfare_records wr JOIN users u ON u.id = wr.user_id
		        WHERE wr.status = 'success' AND wr.created_at < $1 AND u.role <> 'admin'
		        UNION ALL
		        SELECT dc.total_reward AS amount FROM daily_checkin_records dc JOIN users u ON u.id = dc.user_id
		        WHERE dc.status = 'success' AND dc.created_at < $1 AND u.role <> 'admin'
		    ) welfare
		), fixed_cost_totals AS (
		    SELECT COALESCE(SUM(
		        amount * fx_rate
		        * EXTRACT(EPOCH FROM (LEAST(ends_at, $1) - starts_at))
		        / NULLIF(EXTRACT(EPOCH FROM (ends_at - starts_at)), 0)
		    ), 0) AS fixed_cost_cny
		    FROM business_account_costs
		    WHERE starts_at < $1
		)
		SELECT activity_start.starts_at, usage_totals.usage_credits,
		       usage_totals.api_key_cost_cny, usage_totals.unpriced_api_key,
		       welfare_totals.welfare_credits, fixed_cost_totals.fixed_cost_cny
		FROM activity_start, usage_totals, welfare_totals, fixed_cost_totals`

	var start time.Time
	var usageCredits, apiKeyCostCNY, unpricedAPIKey, welfareCredits, accountCostCNY float64
	if err := s.db.QueryRowContext(ctx, query, end).Scan(
		&start, &usageCredits, &apiKeyCostCNY, &unpricedAPIKey, &welfareCredits, &accountCostCNY,
	); err != nil {
		return BusinessProfitSummary{}, fmt.Errorf("query cumulative business profit: %w", err)
	}
	return buildProfitSummary(start, end, usageCredits, apiKeyCostCNY, unpricedAPIKey, welfareCredits, accountCostCNY, settings), nil
}

func buildProfitSummary(start, end time.Time, usageCredits, apiKeyCostCNY, unpricedAPIKey, welfareCredits, accountCostCNY float64, settings BusinessAnalyticsSettings) BusinessProfitSummary {
	revenue := creditsToCNY(usageCredits, settings.BalanceCreditsPerCNY)
	welfareCost := creditsToCNY(welfareCredits, settings.BalanceCreditsPerCNY)
	totalCost := apiKeyCostCNY + welfareCost + accountCostCNY
	profit := revenue - totalCost
	return BusinessProfitSummary{
		StartAt:            start,
		EndAt:              end,
		UsageRevenueCNY:    revenue,
		APIKeyUsageCostCNY: apiKeyCostCNY,
		WelfareCostCNY:     welfareCost,
		AccountCostCNY:     accountCostCNY,
		TotalCostCNY:       totalCost,
		OperatingProfitCNY: profit,
		OperatingMargin:    percentage(profit, revenue),
		UnpricedAPIKeyUSD:  unpricedAPIKey,
		ProfitComplete:     unpricedAPIKey <= 1e-12,
	}
}

func (s *BusinessAnalyticsService) loadGroupUsage(ctx context.Context, start, end time.Time) (map[int64]*businessGroupAccumulator, map[string]*businessDailyAccumulator, error) {
	reportingZone := start.Location().String()
	const query = `
		SELECT ul.group_id, COALESCE(g.name, ''), (ul.created_at AT TIME ZONE $3)::date,
		       COALESCE(SUM(ul.total_cost), 0),
		       COALESCE(SUM(ul.actual_cost), 0),
		       COALESCE(SUM(CASE WHEN a.type = 'apikey'
		           THEN COALESCE(ul.account_stats_cost, ul.total_cost)
		           ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN a.type = 'apikey' AND cost_rate.credits_per_cny IS NOT NULL
		           THEN COALESCE(ul.account_stats_cost, ul.total_cost) / cost_rate.credits_per_cny
		           ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN a.type = 'apikey' AND cost_rate.credits_per_cny IS NULL
		           THEN COALESCE(ul.account_stats_cost, ul.total_cost)
		           ELSE 0 END), 0),
		       COALESCE(SUM(COALESCE(ul.account_stats_cost, ul.total_cost)), 0),
		       COUNT(DISTINCT NULLIF(ul.account_id, 0))
		FROM usage_logs ul
		JOIN users u ON u.id = ul.user_id
		LEFT JOIN groups g ON g.id = ul.group_id
		LEFT JOIN accounts a ON a.id = ul.account_id
		LEFT JOIN business_api_key_cost_rates cost_rate ON cost_rate.account_id = ul.account_id
		WHERE ul.created_at >= $1 AND ul.created_at < $2 AND ul.group_id > 0 AND u.role <> 'admin'
		GROUP BY ul.group_id, g.name, (ul.created_at AT TIME ZONE $3)::date`
	rows, err := s.db.QueryContext(ctx, query, start, end, reportingZone)
	if err != nil {
		return nil, nil, fmt.Errorf("query group usage: %w", err)
	}
	defer func() { _ = rows.Close() }()
	groups := make(map[int64]*businessGroupAccumulator)
	daily := make(map[string]*businessDailyAccumulator)
	for rows.Next() {
		var row businessDailyGroup
		if err := rows.Scan(&row.groupID, &row.groupName, &row.day, &row.baseCredits, &row.usageCredits, &row.apiKeyUsageCost, &row.apiKeyCostCNY, &row.unpricedAPIKey, &row.capacityUsage, &row.accountCount); err != nil {
			return nil, nil, err
		}
		g := groups[row.groupID]
		if g == nil {
			g = &businessGroupAccumulator{groupID: row.groupID, groupName: row.groupName}
			groups[row.groupID] = g
		}
		g.baseCredits += row.baseCredits
		g.usageCredits += row.usageCredits
		g.apiKeyUsageCost += row.apiKeyUsageCost
		g.apiKeyCostCNY += row.apiKeyCostCNY
		g.unpricedAPIKey += row.unpricedAPIKey
		g.dailyCapacityUsage = append(g.dailyCapacityUsage, row.capacityUsage)
		g.dailyAccountCounts = append(g.dailyAccountCounts, row.accountCount)
		key := row.day.Format("2006-01-02")
		day := ensureDaily(daily, key)
		day.usageCredits += row.usageCredits
		day.apiKeyUsageCostUSD += row.apiKeyUsageCost
		day.apiKeyUsageCostCNY += row.apiKeyCostCNY
		day.unpricedAPIKeyUSD += row.unpricedAPIKey
	}
	return groups, daily, rows.Err()
}

func (s *BusinessAnalyticsService) loadAccruedCosts(ctx context.Context, start, end time.Time, daily map[string]*businessDailyAccumulator) (map[int64]float64, float64, float64, bool, error) {
	const query = `SELECT group_id, amount * fx_rate, starts_at, ends_at FROM business_account_costs WHERE starts_at < $2 AND ends_at > $1`
	rows, err := s.db.QueryContext(ctx, query, start, end)
	if err != nil {
		return nil, 0, 0, false, fmt.Errorf("query account costs: %w", err)
	}
	defer func() { _ = rows.Close() }()
	byGroup := make(map[int64]float64)
	var unallocated, total float64
	configured := false
	for rows.Next() {
		var groupID sql.NullInt64
		var amount float64
		var startsAt, endsAt time.Time
		if err := rows.Scan(&groupID, &amount, &startsAt, &endsAt); err != nil {
			return nil, 0, 0, false, err
		}
		overlap := overlapDuration(start, end, startsAt, endsAt)
		period := endsAt.Sub(startsAt)
		if overlap <= 0 || period <= 0 {
			continue
		}
		accrued := amount * overlap.Seconds() / period.Seconds()
		total += accrued
		configured = true
		addAccruedCostByDay(daily, start, end, startsAt, endsAt, amount)
		if groupID.Valid {
			byGroup[groupID.Int64] += accrued
		} else {
			unallocated += accrued
		}
	}
	return byGroup, unallocated, total, configured, rows.Err()
}

func (s *BusinessAnalyticsService) loadWelfare(ctx context.Context, start, end time.Time, daily map[string]*businessDailyAccumulator) (float64, error) {
	reportingZone := start.Location().String()
	const query = `
		SELECT (created_at AT TIME ZONE $3)::date, COALESCE(SUM(amount), 0)
		FROM (
			SELECT wr.created_at, wr.amount FROM welfare_records wr JOIN users u ON u.id = wr.user_id
			WHERE wr.status = 'success' AND u.role <> 'admin'
			UNION ALL
			SELECT dc.created_at, dc.total_reward AS amount FROM daily_checkin_records dc JOIN users u ON u.id = dc.user_id
			WHERE dc.status = 'success' AND u.role <> 'admin'
		) welfare
		WHERE created_at >= $1 AND created_at < $2
		GROUP BY (created_at AT TIME ZONE $3)::date
		ORDER BY 1`
	rows, err := s.db.QueryContext(ctx, query, start, end, reportingZone)
	if err != nil {
		return 0, fmt.Errorf("query welfare grants: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var total float64
	for rows.Next() {
		var day time.Time
		var amount float64
		if err := rows.Scan(&day, &amount); err != nil {
			return 0, err
		}
		total += amount
		ensureDaily(daily, day.Format("2006-01-02")).welfareGrantedUSD += amount
	}
	return total, rows.Err()
}

func (s *BusinessAnalyticsService) GetSettings(ctx context.Context) (*BusinessAnalyticsSettings, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("business analytics database is unavailable")
	}
	var raw string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = $1`, SettingBalanceRechargeMult).Scan(&raw)
	if err != nil {
		if err == sql.ErrNoRows {
			return &BusinessAnalyticsSettings{BalanceCreditsPerCNY: defaultBalanceRechargeMultiplier}, nil
		}
		return nil, fmt.Errorf("query business settings: %w", err)
	}
	value, _ := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	return &BusinessAnalyticsSettings{
		BalanceCreditsPerCNY: normalizeBalanceRechargeMultiplier(value),
	}, nil
}

func (s *BusinessAnalyticsService) loadPoolCapacity(ctx context.Context, groups map[int64]*businessGroupAccumulator) error {
	if len(groups) == 0 {
		return nil
	}
	const query = `
		SELECT ag.group_id, COUNT(DISTINCT a.id), COALESCE(SUM(a.concurrency), 0)
		FROM account_groups ag
		JOIN accounts a ON a.id = ag.account_id
		WHERE a.status = 'active' AND a.schedulable = TRUE AND ag.group_id = ANY($1)
		GROUP BY ag.group_id`
	ids := make([]int64, 0, len(groups))
	for id := range groups {
		ids = append(ids, id)
	}
	rows, err := s.db.QueryContext(ctx, query, pq.Array(ids))
	if err != nil {
		return fmt.Errorf("query pool capacity: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id, accounts, concurrency int64
		if err := rows.Scan(&id, &accounts, &concurrency); err != nil {
			return err
		}
		if g := groups[id]; g != nil {
			g.schedulableAccounts, g.concurrencyMax = accounts, concurrency
		}
	}
	return rows.Err()
}

func (s *BusinessAnalyticsService) loadSnapshotCapacity(ctx context.Context, groups map[int64]*businessGroupAccumulator) error {
	if len(groups) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(groups))
	for id := range groups {
		ids = append(ids, id)
	}
	const query = `
		WITH latest AS (
			SELECT group_id, MAX(captured_at) AS captured_at
			FROM business_account_capacity_snapshots
			WHERE group_id = ANY($1)
			GROUP BY group_id
		)
		SELECT s.group_id, COALESCE(AVG(s.observed_account_cost), 0)
		FROM business_account_capacity_snapshots s
		JOIN latest l ON l.group_id = s.group_id AND l.captured_at = s.captured_at
		GROUP BY s.group_id`
	rows, err := s.db.QueryContext(ctx, query, pq.Array(ids))
	if err != nil {
		return fmt.Errorf("query capacity snapshots: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var groupID int64
		var capacity float64
		if err := rows.Scan(&groupID, &capacity); err != nil {
			return err
		}
		if g := groups[groupID]; g != nil {
			g.snapshotCapacity = capacity
		}
	}
	return rows.Err()
}

func (s *BusinessAnalyticsService) loadCashReceipts(ctx context.Context, start, end time.Time) ([]BusinessCurrencyAmount, error) {
	const query = `
		SELECT UPPER(COALESCE(NULLIF(provider_snapshot->>'currency', ''), 'CNY')),
		       COALESCE(SUM(pay_amount - CASE WHEN amount > 0 THEN pay_amount * refund_amount / amount ELSE 0 END), 0)
		FROM payment_orders
		WHERE paid_at >= $1 AND paid_at < $2
		  AND status IN ('COMPLETED', 'PAID', 'RECHARGING')
		  AND payment_type <> 'balance'
		GROUP BY 1 ORDER BY 1`
	rows, err := s.db.QueryContext(ctx, query, start, end)
	if err != nil {
		return nil, fmt.Errorf("query cash receipts: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items := make([]BusinessCurrencyAmount, 0)
	for rows.Next() {
		var item BusinessCurrencyAmount
		if err := rows.Scan(&item.Currency, &item.Amount); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *BusinessAnalyticsService) loadLiabilities(ctx context.Context, apiKeyUsageCostCNY, usageCreditsUSD float64, settings BusinessAnalyticsSettings) (BusinessLiabilitySummary, error) {
	var out BusinessLiabilitySummary
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(balance + frozen_balance), 0) FROM users WHERE deleted_at IS NULL AND role <> 'admin'`).Scan(&out.BalanceCreditsUSD); err != nil {
		return out, err
	}
	out.BalanceFaceValueCNY = creditsToCNY(out.BalanceCreditsUSD, settings.BalanceCreditsPerCNY)
	ratio := 0.0
	if usageCreditsUSD > 0 {
		ratio = math.Max(0, apiKeyUsageCostCNY/usageCreditsUSD)
	}
	out.BalanceEstimatedCostCNY = out.BalanceCreditsUSD * ratio
	rows, err := s.db.QueryContext(ctx, `
		SELECT us.expires_at, us.total_usage_usd, g.subscription_quota_reset_mode,
		       g.subscription_total_limit_usd, g.daily_limit_usd, g.weekly_limit_usd, g.monthly_limit_usd
		FROM user_subscriptions us
		JOIN users u ON u.id = us.user_id
		JOIN groups g ON g.id = us.group_id
		WHERE us.status = 'active' AND us.expires_at > NOW() AND u.role <> 'admin'`)
	if err != nil {
		return out, err
	}
	defer func() { _ = rows.Close() }()
	now := time.Now()
	for rows.Next() {
		var expiresAt time.Time
		var used float64
		var mode string
		var total, daily, weekly, monthly sql.NullFloat64
		if err := rows.Scan(&expiresAt, &used, &mode, &total, &daily, &weekly, &monthly); err != nil {
			return out, err
		}
		out.ActiveSubscriptions++
		if mode == "until_subscription_expires" && total.Valid {
			out.SubscriptionCommitmentUSD += math.Max(0, total.Float64-used)
			continue
		}
		days := int(math.Ceil(math.Max(0, expiresAt.Sub(now).Hours()) / 24))
		candidates := make([]float64, 0, 3)
		if daily.Valid {
			candidates = append(candidates, daily.Float64*float64(days))
		}
		if weekly.Valid {
			candidates = append(candidates, weekly.Float64*float64(int(math.Ceil(float64(days)/7))))
		}
		if monthly.Valid {
			candidates = append(candidates, monthly.Float64*float64(int(math.Ceil(float64(days)/30))))
		}
		if len(candidates) == 0 {
			out.UnlimitedSubscriptions++
			continue
		}
		out.SubscriptionCommitmentUSD += minFloat(candidates)
	}
	out.SubscriptionEstimatedCostCNY = out.SubscriptionCommitmentUSD * ratio
	return out, rows.Err()
}

func (s *BusinessAnalyticsService) latestSnapshotTime(ctx context.Context) (*time.Time, error) {
	var value sql.NullTime
	if err := s.db.QueryRowContext(ctx, `SELECT MAX(captured_at) FROM business_account_capacity_snapshots`).Scan(&value); err != nil {
		return nil, err
	}
	if !value.Valid {
		return nil, nil
	}
	return &value.Time, nil
}

func (s *BusinessAnalyticsService) CaptureCapacitySnapshot(ctx context.Context) (*time.Time, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("business analytics database is unavailable")
	}
	captured := time.Now().UTC().Truncate(time.Hour)
	const query = `
		INSERT INTO business_account_capacity_snapshots (account_id, group_id, captured_at, observed_requests, observed_account_cost, concurrency_max, source)
		SELECT ul.account_id, ul.group_id, $1,
		       COUNT(*), COALESCE(SUM(COALESCE(ul.account_stats_cost, ul.total_cost)), 0),
		       COALESCE(MAX(a.concurrency), 0), 'usage_window'
		FROM usage_logs ul
		JOIN users u ON u.id = ul.user_id
		JOIN accounts a ON a.id = ul.account_id
		WHERE ul.created_at >= $2 AND ul.created_at < $1 AND ul.account_id > 0 AND ul.group_id > 0 AND u.role <> 'admin'
		GROUP BY ul.account_id, ul.group_id
		ON CONFLICT (account_id, group_id, captured_at) DO UPDATE
		SET observed_requests = EXCLUDED.observed_requests, observed_account_cost = EXCLUDED.observed_account_cost,
		    concurrency_max = EXCLUDED.concurrency_max`
	if _, err := s.db.ExecContext(ctx, query, captured, captured.Add(-24*time.Hour)); err != nil {
		return nil, fmt.Errorf("capture capacity snapshot: %w", err)
	}
	return &captured, nil
}

func (s *BusinessAnalyticsService) GetAPIKeyCostRateConfig(ctx context.Context) (*BusinessAPIKeyCostRateConfig, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("business analytics database is unavailable")
	}
	config := &BusinessAPIKeyCostRateConfig{
		Accounts: make([]BusinessAPIKeyAccount, 0),
		Rates:    make([]BusinessAPIKeyCostRate, 0),
	}
	accountRows, err := s.db.QueryContext(ctx, `
		SELECT id, name, platform
		FROM accounts
		WHERE type = 'apikey' AND deleted_at IS NULL
		ORDER BY name, id`)
	if err != nil {
		return nil, fmt.Errorf("query API key accounts: %w", err)
	}
	for accountRows.Next() {
		var account BusinessAPIKeyAccount
		if err := accountRows.Scan(&account.ID, &account.Name, &account.Platform); err != nil {
			_ = accountRows.Close()
			return nil, err
		}
		config.Accounts = append(config.Accounts, account)
	}
	if err := accountRows.Close(); err != nil {
		return nil, err
	}
	if err := accountRows.Err(); err != nil {
		return nil, err
	}

	rateRows, err := s.db.QueryContext(ctx, `
		SELECT id, account_id, credits_per_cny, notes, created_at
		FROM business_api_key_cost_rates
		ORDER BY id DESC`)
	if err != nil {
		return nil, fmt.Errorf("query API key cost rates: %w", err)
	}
	defer func() { _ = rateRows.Close() }()
	for rateRows.Next() {
		var rate BusinessAPIKeyCostRate
		if err := rateRows.Scan(&rate.ID, &rate.AccountID, &rate.CreditsPerCNY, &rate.Notes, &rate.CreatedAt); err != nil {
			return nil, err
		}
		config.Rates = append(config.Rates, rate)
	}
	return config, rateRows.Err()
}

func (s *BusinessAnalyticsService) CreateAPIKeyCostRate(ctx context.Context, input BusinessAPIKeyCostRateInput) (*BusinessAPIKeyCostRate, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("business analytics database is unavailable")
	}
	if input.AccountID <= 0 {
		return nil, fmt.Errorf("account ID must be greater than zero")
	}
	if math.IsNaN(input.CreditsPerCNY) || math.IsInf(input.CreditsPerCNY, 0) || input.CreditsPerCNY <= 0 {
		return nil, fmt.Errorf("credits per CNY rate must be greater than zero")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var accountType string
	if err := tx.QueryRowContext(ctx, `SELECT type FROM accounts WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`, input.AccountID).Scan(&accountType); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("account not found")
		}
		return nil, err
	}
	if accountType != "apikey" {
		return nil, fmt.Errorf("cost rate can only be configured for API key accounts")
	}

	item := &BusinessAPIKeyCostRate{}
	err = tx.QueryRowContext(ctx, `
		INSERT INTO business_api_key_cost_rates (account_id, credits_per_cny, notes)
		VALUES ($1, $2, $3)
		ON CONFLICT (account_id) DO UPDATE
		SET credits_per_cny = EXCLUDED.credits_per_cny,
		    notes = EXCLUDED.notes,
		    updated_at = NOW()
		RETURNING id, account_id, credits_per_cny, notes, created_at`,
		input.AccountID, input.CreditsPerCNY, strings.TrimSpace(input.Notes),
	).Scan(&item.ID, &item.AccountID, &item.CreditsPerCNY, &item.Notes, &item.CreatedAt)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *BusinessAnalyticsService) DeleteAPIKeyCostRate(ctx context.Context, id int64) error {
	if id <= 0 {
		return fmt.Errorf("invalid API key cost rate ID")
	}
	result, err := s.db.ExecContext(ctx, `DELETE FROM business_api_key_cost_rates WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return fmt.Errorf("API key cost rate not found")
	}
	return nil
}

func (s *BusinessAnalyticsService) ListAccountCosts(ctx context.Context, limit int) ([]BusinessAccountCost, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, account_id, group_id, cost_type, amount, currency, fx_rate, starts_at, ends_at, notes, created_at FROM business_account_costs ORDER BY starts_at DESC, id DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]BusinessAccountCost, 0)
	for rows.Next() {
		var item BusinessAccountCost
		var accountID, groupID sql.NullInt64
		if err := rows.Scan(&item.ID, &accountID, &groupID, &item.CostType, &item.Amount, &item.Currency, &item.FXRate, &item.StartsAt, &item.EndsAt, &item.Notes, &item.CreatedAt); err != nil {
			return nil, err
		}
		if accountID.Valid {
			value := accountID.Int64
			item.AccountID = &value
		}
		if groupID.Valid {
			value := groupID.Int64
			item.GroupID = &value
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *BusinessAnalyticsService) CreateAccountCost(ctx context.Context, input BusinessAccountCostInput) (*BusinessAccountCost, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("business analytics database is unavailable")
	}
	if math.IsNaN(input.Amount) || math.IsInf(input.Amount, 0) || input.Amount <= 0 {
		return nil, fmt.Errorf("amount must be greater than zero")
	}
	if input.FXRate == 0 {
		input.FXRate = 1
	}
	if math.IsNaN(input.FXRate) || math.IsInf(input.FXRate, 0) || input.FXRate <= 0 {
		return nil, fmt.Errorf("fx rate must be greater than zero")
	}
	if !input.EndsAt.After(input.StartsAt) {
		return nil, fmt.Errorf("end time must be after start time")
	}
	if input.AccountID != nil {
		var accountType string
		err := s.db.QueryRowContext(ctx, `SELECT type FROM accounts WHERE id = $1 AND deleted_at IS NULL`, *input.AccountID).Scan(&accountType)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, fmt.Errorf("account not found")
			}
			return nil, fmt.Errorf("query account type: %w", err)
		}
		if accountType == "apikey" {
			return nil, fmt.Errorf("API key account cost is calculated automatically from usage and cannot be entered again")
		}
	}
	input.CostType = strings.TrimSpace(input.CostType)
	if input.CostType == "" {
		input.CostType = "renewal"
	}
	input.Currency = strings.ToUpper(strings.TrimSpace(input.Currency))
	if input.Currency == "" {
		input.Currency = "CNY"
	}
	if len(input.Currency) > 12 || len(input.CostType) > 32 {
		return nil, fmt.Errorf("cost type or currency is too long")
	}
	item := &BusinessAccountCost{}
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO business_account_costs (account_id, group_id, cost_type, amount, currency, fx_rate, starts_at, ends_at, notes)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING id, account_id, group_id, cost_type, amount, currency, fx_rate, starts_at, ends_at, notes, created_at`,
		input.AccountID, input.GroupID, input.CostType, input.Amount, input.Currency, input.FXRate, input.StartsAt, input.EndsAt, strings.TrimSpace(input.Notes),
	).Scan(&item.ID, &item.AccountID, &item.GroupID, &item.CostType, &item.Amount, &item.Currency, &item.FXRate, &item.StartsAt, &item.EndsAt, &item.Notes, &item.CreatedAt)
	if err != nil {
		return nil, err
	}
	return item, nil
}

func (s *BusinessAnalyticsService) DeleteAccountCost(ctx context.Context, id int64) error {
	if id <= 0 {
		return fmt.Errorf("invalid account cost id")
	}
	result, err := s.db.ExecContext(ctx, `DELETE FROM business_account_costs WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return fmt.Errorf("account cost not found")
	}
	return nil
}

func overlapDuration(start, end, left, right time.Time) time.Duration {
	if left.Before(start) {
		left = start
	}
	if right.After(end) {
		right = end
	}
	if !right.After(left) {
		return 0
	}
	return right.Sub(left)
}
func percentage(value, total float64) float64 {
	if total == 0 {
		return 0
	}
	return value / total * 100
}
func mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var sum float64
	for _, value := range values {
		sum += value
	}
	return sum / float64(len(values))
}
func meanPerAccount(costs []float64, accounts []int64) float64 {
	var total float64
	var n int64
	for i, cost := range costs {
		if i < len(accounts) && accounts[i] > 0 {
			total += cost
			n += accounts[i]
		}
	}
	if n == 0 {
		return 0
	}
	return total / float64(n)
}
func percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	index := int(math.Ceil(p*float64(len(sorted)))) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	return sorted[index]
}
func minFloat(values []float64) float64 {
	result := values[0]
	for _, value := range values[1:] {
		if value < result {
			result = value
		}
	}
	return result
}

func creditsToCNY(credits, creditsPerCNY float64) float64 {
	return credits / normalizeBalanceRechargeMultiplier(creditsPerCNY)
}

func share(value, total float64) float64 {
	if total <= 0 {
		return 0
	}
	return value / total
}

func ensureDaily(items map[string]*businessDailyAccumulator, date string) *businessDailyAccumulator {
	item := items[date]
	if item == nil {
		item = &businessDailyAccumulator{date: date}
		items[date] = item
	}
	return item
}

func addAccruedCostByDay(items map[string]*businessDailyAccumulator, reportStart, reportEnd, costStart, costEnd time.Time, amountCNY float64) {
	period := costEnd.Sub(costStart)
	if period <= 0 {
		return
	}
	location := reportStart.Location()
	year, month, day := reportStart.In(location).Date()
	cursor := time.Date(year, month, day, 0, 0, 0, 0, location)
	for cursor.Before(reportEnd) {
		next := cursor.AddDate(0, 0, 1)
		overlap := overlapDuration(cursor, next, businessMaxTime(reportStart, costStart), businessMinTime(reportEnd, costEnd))
		if overlap > 0 {
			ensureDaily(items, cursor.Format("2006-01-02")).accountCostCNY += amountCNY * overlap.Seconds() / period.Seconds()
		}
		cursor = next
	}
}

func buildDailyAnalytics(items map[string]*businessDailyAccumulator, settings BusinessAnalyticsSettings) []BusinessDailyAnalytics {
	dates := make([]string, 0, len(items))
	for date := range items {
		dates = append(dates, date)
	}
	sort.Strings(dates)
	out := make([]BusinessDailyAnalytics, 0, len(dates))
	for _, date := range dates {
		item := items[date]
		revenue := creditsToCNY(item.usageCredits, settings.BalanceCreditsPerCNY)
		apiKeyUsageCost := item.apiKeyUsageCostCNY
		welfare := creditsToCNY(item.welfareGrantedUSD, settings.BalanceCreditsPerCNY)
		out = append(out, BusinessDailyAnalytics{
			Date:               item.date,
			UsageRevenueCNY:    revenue,
			APIKeyUsageCostCNY: apiKeyUsageCost,
			UnpricedAPIKeyUSD:  item.unpricedAPIKeyUSD,
			ProfitComplete:     item.unpricedAPIKeyUSD <= 1e-12,
			WelfareGrantedUSD:  item.welfareGrantedUSD,
			WelfareCostCNY:     welfare,
			AccountCostCNY:     item.accountCostCNY,
			OperatingProfitCNY: revenue - apiKeyUsageCost - welfare - item.accountCostCNY,
		})
	}
	return out
}

func businessMinTime(left, right time.Time) time.Time {
	if left.Before(right) {
		return left
	}
	return right
}

func businessMaxTime(left, right time.Time) time.Time {
	if left.After(right) {
		return left
	}
	return right
}
