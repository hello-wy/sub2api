package service

import (
	"context"
	"database/sql"
	"sort"
	"time"

	"github.com/shopspring/decimal"
)

type BusinessPositions struct {
	Balance                decimal.Decimal  `json:"balance"`
	Frozen                 decimal.Decimal  `json:"frozen"`
	Debt                   decimal.Decimal  `json:"debt"`
	SubscriptionValue      *decimal.Decimal `json:"subscription_value"`
	KnownSubscriptionValue decimal.Decimal  `json:"known_subscription_value"`
	SuspendedValue         decimal.Decimal  `json:"suspended_value"`
	SuspendedCount         int64            `json:"suspended_count"`
	ActiveSubscriptions    int64            `json:"active_subscriptions"`
	MissingSubscriptions   int64            `json:"missing_subscriptions"`
}
type BusinessCapacity struct {
	GroupID             int64            `json:"group_id"`
	Name                string           `json:"name"`
	Platform            string           `json:"platform"`
	ActiveSubscriptions int64            `json:"active_subscriptions"`
	Available           *decimal.Decimal `json:"available"`
	KnownAvailable      decimal.Decimal  `json:"known_available"`
	Unlimited           int64            `json:"unlimited"`
	SevenDayAverage     decimal.Decimal  `json:"seven_day_average"`
	SevenDayPeak        decimal.Decimal  `json:"seven_day_peak"`
	ExpiringCount       int64            `json:"expiring_count"`
}
type BusinessRewardSummary struct {
	Source string          `json:"source"`
	Amount decimal.Decimal `json:"amount"`
}
type BusinessOverview struct {
	Settings              BusinessSettings        `json:"settings"`
	StartDate             string                  `json:"start_date"`
	EndDate               string                  `json:"end_date"`
	UpdatedAt             time.Time               `json:"updated_at"`
	Summary               BusinessDay             `json:"summary"`
	Daily                 []BusinessDay           `json:"daily"`
	CumulativeProfit      *decimal.Decimal        `json:"cumulative_profit"`
	CumulativeKnownProfit decimal.Decimal         `json:"cumulative_known_profit"`
	Positions             BusinessPositions       `json:"positions"`
	Capacity              []BusinessCapacity      `json:"capacity"`
	Rewards               []BusinessRewardSummary `json:"rewards"`
	PendingTerms          int64                   `json:"pending_terms"`
	ManualReceipts        decimal.Decimal         `json:"manual_receipts"`
}

// Aggregation reads compact daily counters, never requests or upstream prices.
// Every term's cumulative consumption is priced before filtering the requested dates.
const operatingDailySQL = `WITH usage_days AS (
 SELECT day,term_id,SUM(balance_amount) balance,SUM(subscription_units) units
 FROM operating_daily_usage GROUP BY day,term_id
), ordered AS (
 SELECT *,SUM(units) OVER(PARTITION BY term_id ORDER BY day ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW) running FROM usage_days
), components AS (
 SELECT d.day,d.balance,
 CASE WHEN t.amount IS NOT NULL AND t.quota>0 AND t.funding<>'unknown'
 THEN ROUND(t.amount*LEAST((t.baseline_units+d.running)/t.quota,1),8)-ROUND(t.amount*LEAST((t.baseline_units+d.running-d.units)/t.quota,1),8) ELSE 0 END subscription,
 0::numeric other_income,0::numeric manual_cost,0::numeric gift_cost,
 CASE WHEN d.units>0 AND (t.id IS NULL OR t.amount IS NULL OR t.quota IS NULL OR t.funding='unknown') THEN 1 ELSE 0 END missing
 FROM ordered d LEFT JOIN operating_terms t ON t.id=d.term_id
 UNION ALL
 SELECT occurred_on,0,0,CASE WHEN kind IN ('income','income_adjustment') THEN amount ELSE 0 END,
 CASE WHEN kind IN ('cost','cost_adjustment') THEN amount ELSE 0 END,0,0 FROM operating_records
 UNION ALL
 SELECT (occurred_at AT TIME ZONE $1)::date,0,0,0,0,amount,0 FROM operating_rewards
 UNION ALL
 SELECT (starts_at AT TIME ZONE $1)::date,0,0,0,0,CASE WHEN funding='gift' THEN COALESCE(amount,0) ELSE 0 END,
 CASE WHEN funding='unknown' OR (funding='gift' AND amount IS NULL) THEN 1 ELSE 0 END
 FROM operating_terms WHERE NOT opening
)
SELECT day::text,SUM(balance),SUM(subscription),SUM(other_income),SUM(manual_cost),SUM(gift_cost),SUM(missing)
FROM components WHERE day<=$2::date GROUP BY day ORDER BY day`

func (s *BusinessAnalyticsService) Overview(ctx context.Context, startDate, endDate string) (*BusinessOverview, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	cfg, err := readBusinessSettings(ctx, tx)
	if err != nil {
		return nil, err
	}
	zone, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		return nil, err
	}
	now := time.Now().In(zone)
	if startDate == "" {
		startDate = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, zone).Format("2006-01-02")
	}
	if endDate == "" {
		endDate = now.Format("2006-01-02")
	}
	start, err := time.ParseInLocation("2006-01-02", startDate, zone)
	if err != nil {
		return nil, operatingInvalid("开始日期无效")
	}
	end, err := time.ParseInLocation("2006-01-02", endDate, zone)
	if err != nil || end.Before(start) || end.Sub(start) > 365*24*time.Hour {
		return nil, operatingInvalid("请选择不超过 366 天的日期范围")
	}
	if endDate > now.Format("2006-01-02") {
		return nil, operatingInvalid("结束日期不能晚于今天")
	}
	result := &BusinessOverview{Settings: cfg, StartDate: startDate, EndDate: endDate, UpdatedAt: now, Daily: []BusinessDay{}, Capacity: []BusinessCapacity{}, Rewards: []BusinessRewardSummary{}}
	days := map[string]BusinessDay{}
	cumulative := BusinessDay{}
	rows, err := tx.QueryContext(ctx, operatingDailySQL, cfg.Timezone, now.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var d BusinessDay
		if err = rows.Scan(&d.Date, &d.BalanceRevenue, &d.SubscriptionRevenue, &d.OtherRevenue, &d.ManualCost, &d.GiftCost, &d.Missing); err != nil {
			_ = rows.Close()
			return nil, err
		}
		d.finish()
		cumulative.add(d)
		if d.Date >= startDate && d.Date <= endDate {
			days[d.Date] = d
			result.Summary.add(d)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	for day := start; cfg.EnabledAt != nil && !day.After(end); day = day.AddDate(0, 0, 1) {
		if day.Format("2006-01-02") < cfg.EnabledAt.In(zone).Format("2006-01-02") {
			continue
		}
		key := day.Format("2006-01-02")
		d := days[key]
		d.Date = key
		d.finish()
		result.Daily = append(result.Daily, d)
	}
	result.Summary.finish()
	cumulative.finish()
	result.CumulativeKnownProfit = cfg.OpeningRevenue.Add(cfg.OpeningPrivateRevenue).Sub(cfg.OpeningCost).Add(cumulative.KnownProfit)
	if cfg.EnabledAt != nil && cumulative.Missing == 0 {
		n := result.CumulativeKnownProfit
		result.CumulativeProfit = &n
	}
	if cfg.EnabledAt == nil {
		result.Summary.Profit = nil
		for i := range result.Daily {
			result.Daily[i].Profit = nil
		}
	}
	if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM operating_terms WHERE amount IS NULL OR quota IS NULL OR funding='unknown'").Scan(&result.PendingTerms); err != nil {
		return nil, err
	}
	if err = tx.QueryRowContext(ctx, "SELECT COALESCE(SUM(amount),0) FROM operating_records WHERE kind='receipt' AND occurred_on BETWEEN $1::date AND $2::date", startDate, endDate).Scan(&result.ManualReceipts); err != nil {
		return nil, err
	}
	rows, err = tx.QueryContext(ctx, `SELECT source,SUM(amount) FROM operating_rewards WHERE (occurred_at AT TIME ZONE $1)::date BETWEEN $2::date AND $3::date GROUP BY source
 UNION ALL SELECT '赠送订阅',SUM(amount) FROM operating_terms WHERE funding='gift' AND NOT opening AND amount IS NOT NULL AND (starts_at AT TIME ZONE $1)::date BETWEEN $2::date AND $3::date HAVING COUNT(*)>0`, cfg.Timezone, startDate, endDate)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var r BusinessRewardSummary
		if err = rows.Scan(&r.Source, &r.Amount); err != nil {
			_ = rows.Close()
			return nil, err
		}
		result.Rewards = append(result.Rewards, r)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	if err = readBusinessPositions(ctx, tx, &result.Positions); err != nil {
		return nil, err
	}
	result.Capacity, err = readBusinessCapacity(ctx, tx, now, cfg.Timezone)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}
func readBusinessPositions(ctx context.Context, q businessQuery, p *BusinessPositions) error {
	err := q.QueryRowContext(ctx, `SELECT COALESCE(SUM(GREATEST(balance,0)),0),COALESCE(SUM(GREATEST(frozen_balance,0)),0),COALESCE(SUM(GREATEST(-balance,0)),0) FROM users WHERE deleted_at IS NULL`).Scan(&p.Balance, &p.Frozen, &p.Debt)
	if err != nil {
		return err
	}
	rows, err := q.QueryContext(ctx, `WITH consumed AS (SELECT term_id,SUM(subscription_units) used FROM operating_daily_usage GROUP BY term_id)
 SELECT s.status,t.amount,t.quota,t.baseline_units+COALESCE(c.used,0),t.funding
 FROM operating_terms t JOIN user_subscriptions s ON s.id=t.subscription_id
 LEFT JOIN consumed c ON c.term_id=t.id WHERE t.current_term AND s.deleted_at IS NULL AND s.starts_at<=clock_timestamp()
 AND s.expires_at>clock_timestamp() AND s.status IN ('active','suspended')`)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var status, funding string
		var amount, quota *decimal.Decimal
		var used decimal.Decimal
		if err = rows.Scan(&status, &amount, &quota, &used, &funding); err != nil {
			return err
		}
		if status == "suspended" {
			p.SuspendedCount++
		} else {
			p.ActiveSubscriptions++
		}
		if amount == nil || quota == nil || funding == "unknown" {
			p.MissingSubscriptions++
			continue
		}
		left := decimal.Max(amount.Sub(OperatingConsumptionValue(*amount, *quota, used)), decimal.Zero)
		if status == "suspended" {
			p.SuspendedValue = p.SuspendedValue.Add(left)
		} else {
			p.KnownSubscriptionValue = p.KnownSubscriptionValue.Add(left)
		}
	}
	if err = rows.Err(); err != nil {
		return err
	}
	if p.MissingSubscriptions == 0 {
		n := p.KnownSubscriptionValue
		p.SubscriptionValue = &n
	}
	return nil
}

// Follow the existing quota reset semantics, including calendar-day resets and lifetime mode.
func operatingAvailable(sub *UserSubscription, g *Group, now time.Time) *decimal.Decimal {
	var remaining *decimal.Decimal
	take := func(limit *float64, used float64) {
		if limit == nil {
			return
		}
		n := decimal.Max(decimal.NewFromFloat(*limit).Sub(decimal.NewFromFloat(used)), decimal.Zero)
		if remaining == nil || n.LessThan(*remaining) {
			remaining = &n
		}
	}
	if g.UsesSubscriptionLifetimeQuota() && g.HasSubscriptionTotalLimit() {
		take(g.SubscriptionTotalLimitUSD, sub.TotalUsageUSD)
		return remaining
	}
	daily, weekly, monthly := sub.DailyUsageUSD, sub.WeeklyUsageUSD, sub.MonthlyUsageUSD
	if !g.UsesSubscriptionLifetimeQuota() {
		if sub.NeedsDailyResetAt(now) {
			daily = 0
		}
		if sub.NeedsWeeklyResetAt(now) {
			weekly = 0
		}
		if sub.NeedsMonthlyResetAt(now) {
			monthly = 0
		}
	}
	take(g.DailyLimitUSD, daily)
	take(g.WeeklyLimitUSD, weekly)
	take(g.MonthlyLimitUSD, monthly)
	take(g.SubscriptionTotalLimitUSD, sub.TotalUsageUSD)
	return remaining
}
func readBusinessCapacity(ctx context.Context, q businessQuery, now time.Time, zone string) ([]BusinessCapacity, error) {
	groups := map[int64]*BusinessCapacity{}
	rows, err := q.QueryContext(ctx, `SELECT g.id,g.name,g.platform,g.daily_limit_usd,g.weekly_limit_usd,g.monthly_limit_usd,g.subscription_total_limit_usd,g.subscription_quota_reset_mode,
 s.starts_at,s.expires_at,s.daily_window_start,s.weekly_window_start,s.monthly_window_start,s.daily_usage_usd,s.weekly_usage_usd,s.monthly_usage_usd,s.total_usage_usd
 FROM user_subscriptions s JOIN groups g ON g.id=s.group_id WHERE s.deleted_at IS NULL AND s.status='active' AND s.starts_at<=$1 AND s.expires_at>$1`, now)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var g Group
		var sub UserSubscription
		if err = rows.Scan(&g.ID, &g.Name, &g.Platform, &g.DailyLimitUSD, &g.WeeklyLimitUSD, &g.MonthlyLimitUSD, &g.SubscriptionTotalLimitUSD, &g.SubscriptionQuotaResetMode,
			&sub.StartsAt, &sub.ExpiresAt, &sub.DailyWindowStart, &sub.WeeklyWindowStart, &sub.MonthlyWindowStart, &sub.DailyUsageUSD, &sub.WeeklyUsageUSD, &sub.MonthlyUsageUSD, &sub.TotalUsageUSD); err != nil {
			_ = rows.Close()
			return nil, err
		}
		v := groups[g.ID]
		if v == nil {
			v = &BusinessCapacity{GroupID: g.ID, Name: g.Name, Platform: g.Platform}
			groups[g.ID] = v
		}
		v.ActiveSubscriptions++
		if !sub.ExpiresAt.After(now.AddDate(0, 0, 7)) {
			v.ExpiringCount++
		}
		remaining := operatingAvailable(&sub, &g, now)
		if remaining == nil {
			v.Unlimited++
		} else {
			v.KnownAvailable = v.KnownAvailable.Add(*remaining)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	// Seven complete reporting days; today's partial volume does not lower the average.
	rows, err = q.QueryContext(ctx, `SELECT d.group_id,COALESCE(g.name,'未分组'),COALESCE(g.platform,''),SUM(d.used)/7,MAX(d.used) FROM
 (SELECT group_id,day,SUM(balance_amount+subscription_units) used FROM operating_daily_usage
 WHERE day>=($1::timestamptz AT TIME ZONE $2)::date-7 AND day<($1::timestamptz AT TIME ZONE $2)::date GROUP BY group_id,day) d
 LEFT JOIN groups g ON g.id=d.group_id GROUP BY d.group_id,g.name,g.platform`, now, zone)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var v BusinessCapacity
		if err = rows.Scan(&v.GroupID, &v.Name, &v.Platform, &v.SevenDayAverage, &v.SevenDayPeak); err != nil {
			_ = rows.Close()
			return nil, err
		}
		existing := groups[v.GroupID]
		if existing == nil {
			groups[v.GroupID] = &v
		} else {
			existing.SevenDayAverage = v.SevenDayAverage
			existing.SevenDayPeak = v.SevenDayPeak
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	result := []BusinessCapacity{}
	for _, v := range groups {
		if v.Unlimited == 0 {
			n := v.KnownAvailable
			v.Available = &n
		}
		result = append(result, *v)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].GroupID < result[j].GroupID })
	return result, nil
}
