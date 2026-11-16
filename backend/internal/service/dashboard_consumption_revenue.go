package service

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

// DashboardConsumptionRevenue is a consumption-value estimate, not cash receipts.
// Wallets use each user's historical paid recharge average (including recharge
// bonuses); independently granted credits are not a traced cash funding ledger.
type DashboardConsumptionRevenue struct {
	BalanceCNY      decimal.Decimal `json:"balance_cny"`
	SubscriptionCNY decimal.Decimal `json:"subscription_cny"`
	TotalCNY        decimal.Decimal `json:"total_cny"`
	UnpricedCredits decimal.Decimal `json:"unpriced_credits"`
}

// Read original orders even when operating statistics have not been initialized.
// Only explicit subscription order notes establish a purchase link. Missing
// receipts/currency/quota remain unpriced instead of silently assuming $1=¥1.
const dashboardConsumptionRevenueSQL = `WITH day_usage AS MATERIALIZED (
 SELECT user_id,subscription_id,billing_type,created_at,actual_cost AS credits
 FROM usage_logs WHERE created_at >= $1 AND created_at < $2 AND actual_cost > 0
), day_subscriptions AS MATERIALIZED (
 SELECT DISTINCT subscription_id FROM day_usage WHERE subscription_id IS NOT NULL
), paid_orders AS MATERIALIZED (
 SELECT id,user_id,order_type,payment_type,subscription_group_id,subscription_days,
 UPPER(COALESCE(NULLIF(provider_snapshot->>'currency',''),'CNY')) AS currency,
 COALESCE(completed_at,paid_at) AS fulfilled_at,
 GREATEST(amount-refund_amount,0) AS net_units,
 pay_amount*GREATEST(amount-refund_amount,0)/NULLIF(amount,0) AS net_paid
 FROM payment_orders WHERE status IN ('COMPLETED','REFUNDED') AND amount>0
 AND COALESCE(completed_at,paid_at)<$2
), wallet_orders AS MATERIALIZED (
 SELECT user_id,fulfilled_at,net_units,net_paid FROM paid_orders
 WHERE order_type='balance' AND payment_type<>'balance' AND currency IN ('CNY','RMB')
), wallet_rates AS (
 SELECT user_id,SUM(net_paid)/NULLIF(SUM(net_units),0) AS rate FROM wallet_orders GROUP BY user_id
), wallet_values AS (
 SELECT SUM(u.credits) AS credits,w.rate FROM day_usage u
 LEFT JOIN wallet_rates w ON w.user_id=u.user_id
 WHERE u.subscription_id IS NULL AND u.billing_type=0 GROUP BY u.user_id,w.rate
), purchases AS MATERIALIZED (
 SELECT DISTINCT s.id AS subscription_id,s.user_id,o.id AS order_id,o.fulfilled_at,
 o.fulfilled_at+o.subscription_days*interval '1 day' AS ends_at,
 CASE WHEN o.payment_type='balance' THEN o.net_paid*(
   SELECT SUM(w.net_paid)/NULLIF(SUM(w.net_units),0) FROM wallet_orders w
   WHERE w.user_id=s.user_id AND w.fulfilled_at<=o.fulfilled_at
 ) WHEN o.currency IN ('CNY','RMB') THEN o.net_paid END AS amount,
 CASE WHEN g.subscription_quota_reset_mode='until_subscription_expires'
   THEN LEAST(NULLIF(g.subscription_total_limit_usd,0),NULLIF(g.daily_limit_usd,0),NULLIF(g.weekly_limit_usd,0),NULLIF(g.monthly_limit_usd,0))
   ELSE NULLIF(g.subscription_total_limit_usd,0) END AS quota
 FROM day_subscriptions d JOIN user_subscriptions s ON s.id=d.subscription_id
 JOIN groups g ON g.id=s.group_id
 CROSS JOIN LATERAL regexp_split_to_table(COALESCE(s.notes,''),E'\\r?\\n') n(line)
 JOIN paid_orders o ON btrim(n.line)='payment order '||o.id
 AND o.user_id=s.user_id AND o.subscription_group_id=s.group_id AND o.order_type='subscription'
 WHERE o.subscription_days>0
), subscription_orders AS MATERIALIZED (
 SELECT *,LEAST(ends_at,LEAD(fulfilled_at) OVER (PARTITION BY subscription_id ORDER BY fulfilled_at,order_id)) AS active_until
 FROM purchases
), subscription_usage AS (
 SELECT u.user_id,u.subscription_id,o.order_id,SUM(u.credits) AS credits
 FROM day_usage u LEFT JOIN subscription_orders o ON o.subscription_id=u.subscription_id
 AND o.user_id=u.user_id AND u.created_at>=o.fulfilled_at AND u.created_at<o.active_until
 WHERE u.subscription_id IS NOT NULL OR u.billing_type<>0
 GROUP BY u.user_id,u.subscription_id,o.order_id
), order_values AS MATERIALIZED (
 SELECT o.*,COALESCE(p.used,0) AS previous FROM subscription_orders o
 LEFT JOIN LATERAL (
 SELECT SUM(actual_cost) AS used FROM usage_logs l
 WHERE l.user_id=o.user_id AND l.subscription_id=o.subscription_id
 AND l.created_at>=o.fulfilled_at AND l.created_at<LEAST($1,o.active_until) AND l.actual_cost>0
 AND o.quota>0 AND o.amount IS NOT NULL
 ) p ON true
 WHERE o.active_until>$1 AND o.fulfilled_at<$2
), subscription_values AS (
 SELECT u.credits,o.amount,o.quota,o.previous
 FROM subscription_usage u LEFT JOIN order_values o ON o.subscription_id=u.subscription_id AND o.order_id=u.order_id
), components AS (
 SELECT COALESCE(credits*rate,0) AS balance,0::numeric AS subscription,
 CASE WHEN rate IS NULL THEN credits ELSE 0 END AS unpriced FROM wallet_values
 UNION ALL
 SELECT 0,CASE WHEN quota>0 AND amount IS NOT NULL THEN
 ROUND(amount*LEAST((previous+credits)/quota,1),8)-ROUND(amount*LEAST(previous/quota,1),8) ELSE 0 END,
 CASE WHEN quota IS NULL OR quota<=0 OR amount IS NULL THEN credits ELSE 0 END FROM subscription_values
)
SELECT COALESCE(SUM(balance),0),COALESCE(SUM(subscription),0),COALESCE(SUM(unpriced),0) FROM components`

func (s *BusinessAnalyticsService) DashboardConsumptionRevenue(ctx context.Context, start, end time.Time) (*DashboardConsumptionRevenue, error) {
	var result DashboardConsumptionRevenue
	err := s.db.QueryRowContext(ctx, dashboardConsumptionRevenueSQL, start, end).Scan(&result.BalanceCNY, &result.SubscriptionCNY, &result.UnpricedCredits)
	if err != nil {
		return nil, err
	}
	result.BalanceCNY = result.BalanceCNY.Round(8)
	result.SubscriptionCNY = result.SubscriptionCNY.Round(8)
	result.TotalCNY = result.BalanceCNY.Add(result.SubscriptionCNY)
	return &result, nil
}
