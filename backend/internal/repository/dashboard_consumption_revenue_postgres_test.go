package repository_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

// Exercise the report against real order, subscription and usage schemas without
// initializing the operating ledger or changing any user balances.
func TestDashboardConsumptionRevenuePostgres(t *testing.T) {
	db := operatingDB(t)
	svc := service.NewBusinessAnalyticsService(db)
	day := time.Date(2026, 11, 16, 0, 0, 0, 0, mustOperatingZone())
	account := operatingID(t, db, `INSERT INTO accounts(name,platform,type,credentials) VALUES('revenue-test','openai','apikey','{}') RETURNING id`)
	sequence := 0
	user := func(t *testing.T) (int64, int64) {
		u := operatingUser(t, db, "100")
		k := operatingID(t, db, `INSERT INTO api_keys(user_id,key,name) VALUES($1,$2,'revenue-test') RETURNING id`, u, fmt.Sprintf("revenue-key-%d", u))
		return u, k
	}
	order := func(t *testing.T, u int64, kind, payment, amount, paid, refund, currency string, group any, at time.Time) int64 {
		sequence++
		return operatingID(t, db, `INSERT INTO payment_orders(user_id,user_email,user_name,amount,pay_amount,refund_amount,recharge_code,out_trade_no,payment_type,order_type,subscription_group_id,subscription_days,status,completed_at,paid_at,expires_at,provider_snapshot)
   VALUES($1,'test@local','test',$2,$3,$4,$5,$5,$6,$7,$8,30,'COMPLETED',$9,$9,$9::timestamptz+interval '30 days',jsonb_build_object('currency',$10::text)) RETURNING id`, u, amount, paid, refund, fmt.Sprintf("revenue-order-%d", sequence), payment, kind, group, at, currency)
	}
	subscription := func(t *testing.T, u int64, quota string) (int64, int64) {
		sequence++
		g := operatingID(t, db, `INSERT INTO groups(name,platform,subscription_type,subscription_total_limit_usd,subscription_quota_reset_mode) VALUES($1,'openai','subscription',$2,'until_subscription_expires') RETURNING id`, fmt.Sprintf("revenue-group-%d", sequence), quota)
		notes := ""

		s := operatingID(t, db, `INSERT INTO user_subscriptions(user_id,group_id,starts_at,expires_at,notes) VALUES($1,$2,$3,$3::timestamptz+interval '30 days',$4) RETURNING id`, u, g, day.Add(-24*time.Hour), notes)
		return g, s
	}
	usage := func(t *testing.T, u, k int64, sub any, billing int, credits string, at time.Time) {
		sequence++
		operatingExec(t, db, `INSERT INTO usage_logs(user_id,api_key_id,account_id,request_id,model,subscription_id,billing_type,actual_cost,created_at) VALUES($1,$2,$3,$4,'gpt-test',$5,$6,$7,$8)`, u, k, account, fmt.Sprintf("revenue-use-%d", sequence), sub, billing, credits, at)
	}
	check := func(t *testing.T, balance, sub, unpriced string) {
		r, err := svc.DashboardConsumptionRevenue(context.Background(), day, day.AddDate(0, 0, 1))
		require.NoError(t, err)
		require.Equal(t, balance, r.BalanceCNY.String())
		require.Equal(t, sub, r.SubscriptionCNY.String())
		require.Equal(t, unpriced, r.UnpricedCredits.String())
		require.True(t, r.TotalCNY.Equal(r.BalanceCNY.Add(r.SubscriptionCNY)))
		detail, err := svc.ConsumptionRevenueReport(context.Background(), day, day.AddDate(0, 0, 1))
		require.NoError(t, err)
		require.True(t, r.TotalCNY.Equal(detail.Summary.TotalCNY))
		require.True(t, r.UnpricedCredits.Equal(detail.Summary.UnpricedCredits))
		var requests int64
		var actual string
		require.NoError(t, db.QueryRow("SELECT COUNT(*),COALESCE(SUM(actual_cost),0)::text FROM usage_logs WHERE created_at >= $1 AND created_at < $2 AND actual_cost>0", day, day.AddDate(0, 0, 1)).Scan(&requests, &actual))
		require.Equal(t, requests, detail.Requests)
		require.True(t, detail.ActualCredits.Equal(decimal.RequireFromString(actual)))

	}
	t.Run("empty", func(t *testing.T) { check(t, "0", "0", "0") })
	t.Run("wallet weighted net receipts includes recharge bonuses and refunds", func(t *testing.T) {
		u, k := user(t)
		order(t, u, "balance", "alipay", "100", "80", "0", "CNY", nil, day.Add(-48*time.Hour))
		refunded := order(t, u, "balance", "alipay", "100", "60", "50", "RMB", nil, day.Add(-24*time.Hour))
		operatingExec(t, db, "UPDATE payment_orders SET status='REFUNDED' WHERE id=$1", refunded)
		order(t, u, "balance", "stripe", "100", "100", "0", "USD", nil, day.Add(-time.Hour))
		future := order(t, u, "balance", "alipay", "100", "100", "0", "CNY", nil, day.Add(48*time.Hour))
		pending := order(t, u, "balance", "alipay", "100", "100", "0", "CNY", nil, day.Add(-time.Hour))
		operatingExec(t, db, "UPDATE payment_orders SET status='PAID' WHERE id=$1", pending)
		require.Positive(t, future)
		usage(t, u, k, nil, 0, "30", day)                  // midnight inclusive: (80+30)/(100+50)*30=22
		usage(t, u, k, nil, 0, "99", day.AddDate(0, 0, 1)) // next midnight exclusive
		check(t, "22", "0", "0")
	})
	t.Run("subscription discount prorated and capped across days", func(t *testing.T) {
		u, k := user(t)
		g, s := subscription(t, u, "160")
		o := order(t, u, "subscription", "alipay", "120", "115.2", "0", "CNY", g, day.Add(-24*time.Hour))
		operatingExec(t, db, "UPDATE user_subscriptions SET notes=$1 WHERE id=$2", fmt.Sprintf("payment order %d\r\npayment order %d", o, o), s)
		usage(t, u, k, s, 1, "150", day.Add(-time.Hour))
		usage(t, u, k, s, 1, "20", day.Add(time.Hour))
		check(t, "22", "7.2", "0") // only remaining 10/160 of the discounted receipt
	})
	t.Run("same day renewal starts a new cap without overlapping orders", func(t *testing.T) {
		u, k := user(t)
		g, s := subscription(t, u, "100")
		a := order(t, u, "subscription", "alipay", "100", "100", "0", "CNY", g, day.Add(-24*time.Hour))
		b := order(t, u, "subscription", "alipay", "100", "60", "0", "CNY", g, day.Add(12*time.Hour))
		operatingExec(t, db, "UPDATE user_subscriptions SET notes=$1 WHERE id=$2", fmt.Sprintf("payment order %d\npayment order %d", a, b), s)
		usage(t, u, k, s, 1, "90", day.Add(-time.Hour))
		usage(t, u, k, s, 1, "20", day.Add(time.Hour))    // old term: capped to 10 CNY
		usage(t, u, k, s, 1, "10", day.Add(13*time.Hour)) // new term: 6 CNY
		check(t, "22", "23.2", "0")
	})
	t.Run("balance purchased subscription uses original cash conversion", func(t *testing.T) {
		u, k := user(t)
		g, s := subscription(t, u, "200")
		order(t, u, "balance", "alipay", "100", "80", "0", "CNY", nil, day.Add(-48*time.Hour))
		o := order(t, u, "subscription", "balance", "50", "50", "0", "CNY", g, day.Add(-24*time.Hour))
		operatingExec(t, db, "UPDATE user_subscriptions SET notes=$1 WHERE id=$2", fmt.Sprintf("payment order %d", o), s)
		usage(t, u, k, s, 1, "20", day.Add(time.Hour))
		usage(t, u, k, nil, 0, "10", day.Add(time.Hour))
		check(t, "30", "27.2", "0") // subscription: 50*0.8*20/200=4
	})
	t.Run("unknown funding quota and deleted subscription are not treated as cash", func(t *testing.T) {
		u, k := user(t)
		g, s := subscription(t, u, "0")
		o := order(t, u, "subscription", "alipay", "100", "100", "0", "CNY", g, day.Add(-24*time.Hour))
		operatingExec(t, db, "UPDATE user_subscriptions SET notes=$1 WHERE id=$2", fmt.Sprintf("payment order %d", o), s)
		usage(t, u, k, s, 1, "2", day.Add(time.Hour))   // no period quota
		usage(t, u, k, nil, 0, "3", day.Add(time.Hour)) // no wallet receipts
		usage(t, u, k, nil, 1, "4", day.Add(time.Hour)) // deleted subscription ID
		_, unknown := subscription(t, u, "100")
		usage(t, u, k, unknown, 1, "5", day.Add(time.Hour)) // no matching purchase
		check(t, "30", "27.2", "14")
	})
	t.Run("lifetime quota uses tightest limit and missing currency defaults to CNY", func(t *testing.T) {
		u, k := user(t)
		g, s := subscription(t, u, "200")
		operatingExec(t, db, "UPDATE groups SET monthly_limit_usd=100 WHERE id=$1", g)
		o := order(t, u, "subscription", "alipay", "50", "50", "0", "CNY", g, day.Add(-24*time.Hour))
		operatingExec(t, db, "UPDATE payment_orders SET provider_snapshot='{}' WHERE id=$1", o)
		operatingExec(t, db, "UPDATE user_subscriptions SET notes=$1 WHERE id=$2", fmt.Sprintf("payment order %d", o), s)
		usage(t, u, k, s, 1, "10", day.Add(time.Hour))
		check(t, "30", "32.2", "14")
	})
	t.Run("expired receipts and nonmatching note fragments stay unknown", func(t *testing.T) {
		u, k := user(t)
		g, s := subscription(t, u, "100")
		o := order(t, u, "subscription", "alipay", "50", "50", "0", "CNY", g, day.Add(-31*24*time.Hour))
		operatingExec(t, db, "UPDATE user_subscriptions SET notes=$1 WHERE id=$2", fmt.Sprintf("payment order %d", o), s)
		usage(t, u, k, s, 1, "6", day.Add(time.Hour))
		other, otherKey := user(t)
		g2, s2 := subscription(t, other, "100")
		o2 := order(t, other, "subscription", "alipay", "50", "50", "0", "CNY", g2, day.Add(-24*time.Hour))
		operatingExec(t, db, "UPDATE user_subscriptions SET notes=$1 WHERE id=$2", fmt.Sprintf("prefix payment order %d suffix\npayment order %d0", o2, o2), s2)
		usage(t, other, otherKey, s2, 1, "7", day.Add(time.Hour))
		check(t, "30", "32.2", "27")
	})

}
