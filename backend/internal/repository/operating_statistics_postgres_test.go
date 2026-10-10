package repository_test

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	_ "github.com/lib/pq"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func operatingDB(t *testing.T) *sql.DB {
	return operatingDBBefore(t, nil)
}
func operatingDBBefore(t *testing.T, before func(*sql.DB)) *sql.DB {
	t.Helper()
	dsn := os.Getenv("OPERATING_STATISTICS_TEST_DSN")
	if dsn == "" {
		if os.Getenv("OPERATING_REQUIRE_POSTGRES") == "1" {
			t.Fatal("Operating integration job requires OPERATING_STATISTICS_TEST_DSN")
		}
		t.Skip("set OPERATING_STATISTICS_TEST_DSN to an isolated PostgreSQL server")
	}
	admin, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	name := fmt.Sprintf("operating_test_%d", time.Now().UnixNano())
	_, err = admin.Exec("CREATE DATABASE " + name)
	require.NoError(t, err)
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	u.Path = "/" + name
	db, err := sql.Open("postgres", u.String())
	require.NoError(t, err)
	t.Cleanup(func() { db.Close(); admin.Exec("DROP DATABASE " + name + " WITH (FORCE)"); admin.Close() })
	if before != nil {
		before(db)
	}
	require.NoError(t, repository.ApplyMigrations(context.Background(), db))
	return db
}
func operatingExec(t *testing.T, db *sql.DB, q string, args ...any) {
	t.Helper()
	_, err := db.Exec(q, args...)
	require.NoError(t, err)
}
func operatingID(t *testing.T, db *sql.DB, q string, args ...any) int64 {
	t.Helper()
	var id int64
	require.NoError(t, db.QueryRow(q, args...).Scan(&id))
	return id
}
func operatingUser(t *testing.T, db *sql.DB, balance string) int64 {
	return operatingID(t, db, "INSERT INTO users(email,password_hash,username,balance) VALUES($1,'hash','经营测试',$2) RETURNING id", fmt.Sprintf("operating-%d@test.local", time.Now().UnixNano()), balance)
}
func operatingInit(t *testing.T, svc *service.BusinessAnalyticsService) {
	t.Helper()
	_, err := svc.Initialize(context.Background(), service.BusinessSettings{Timezone: "Asia/Shanghai", OpeningRevenue: decimal.NewFromInt(200), OpeningPrivateRevenue: decimal.NewFromInt(50), OpeningCost: decimal.NewFromInt(100)}, 1)
	require.NoError(t, err)
}
func operatingOverview(t *testing.T, svc *service.BusinessAnalyticsService) *service.BusinessOverview {
	t.Helper()
	r, err := svc.Overview(context.Background(), "", "")
	require.NoError(t, err)
	return r
}
func TestOperatingStatisticsPostgres(t *testing.T) {
	db := operatingDB(t)
	ctx := context.Background()
	svc := service.NewBusinessAnalyticsService(db)
	var legacy int
	require.NoError(t, db.QueryRow("SELECT count(*) FROM information_schema.tables WHERE table_schema='public' AND table_name LIKE 'business_%'").Scan(&legacy))
	require.Positive(t, legacy)
	user := operatingUser(t, db, "1000")
	group := operatingID(t, db, "INSERT INTO groups(name,platform,subscription_type,subscription_total_limit_usd) VALUES('套餐','openai','subscription',1000) RETURNING id")
	plan := operatingID(t, db, "INSERT INTO subscription_plans(group_id,name,price,currency) VALUES($1,'100元套餐',100,'CNY') RETURNING id", group)
	order := operatingID(t, db, `INSERT INTO payment_orders(user_id,user_email,user_name,amount,pay_amount,recharge_code,out_trade_no,payment_type,order_type,plan_id,subscription_group_id,status,expires_at)
 VALUES($1,'test@local','test',100,100,'subscription-test','operating-order','alipay','subscription',$2,$3,'COMPLETED',now()+interval '30 days') RETURNING id`, user, plan, group)
	sub := operatingID(t, db, `INSERT INTO user_subscriptions(user_id,group_id,starts_at,expires_at,total_usage_usd,notes) VALUES($1,$2,now()-interval '1 day',now()+interval '29 days',400,$3) RETURNING id`, user, group, fmt.Sprintf("payment order %d", order))
	key := operatingID(t, db, "INSERT INTO api_keys(user_id,key,name,group_id) VALUES($1,'operating-key','test',$2) RETURNING id", user, group)
	operatingInit(t, svc)
	_, err := svc.Initialize(ctx, service.BusinessSettings{Timezone: "UTC"}, 1)
	require.ErrorContains(t, err, "已启用")
	var unchanged string
	require.NoError(t, db.QueryRow("SELECT balance::text FROM users WHERE id=$1", user).Scan(&unchanged))
	require.Equal(t, "1000.00000000", unchanged)
	initial := operatingOverview(t, svc)
	require.Equal(t, "150", initial.CumulativeProfit.String())
	require.Zero(t, initial.PendingTerms)
	require.True(t, initial.Positions.KnownSubscriptionValue.Equal(decimal.NewFromInt(60)))
	repo := repository.NewUsageBillingRepository(nil, db)
	cmd := &service.UsageBillingCommand{RequestID: "subscription-one", APIKeyID: key, UserID: user, SubscriptionID: &sub, SubscriptionTermVersion: 1, SubscriptionCost: 100}
	applied, err := repo.Apply(ctx, cmd)
	require.NoError(t, err)
	require.True(t, applied.Applied)
	again, err := repo.Apply(ctx, cmd)
	require.NoError(t, err)
	require.False(t, again.Applied)
	r := operatingOverview(t, svc)
	require.Equal(t, "10", r.Summary.SubscriptionRevenue.String())
	require.Equal(t, "50", r.Positions.KnownSubscriptionValue.String())
	_, err = repo.Apply(ctx, &service.UsageBillingCommand{RequestID: "balance-one", APIKeyID: key, UserID: user, BalanceCost: 20})
	require.NoError(t, err)
	// Gift sources are distinct; recording evidence twice cannot double the grant.
	operatingExec(t, db, "INSERT INTO redeem_codes(code,type,value,status,used_by,used_at) VALUES('rebate-test','usage_rebate',5,'used',$1,now())", user)
	operatingExec(t, db, "UPDATE redeem_codes SET notes='evidence updated' WHERE code='rebate-test'")
	operatingExec(t, db, "INSERT INTO balance_transactions(user_id,transaction_type,amount,source_type,source_id) VALUES($1,'lottery_reward',7,'lottery_draw','1')", user)
	operatingExec(t, db, "INSERT INTO user_affiliate_ledger(user_id,action,amount) VALUES($1,'transfer',6)", user)
	promo := operatingID(t, db, "INSERT INTO promo_codes(code,bonus_amount) VALUES('operating-promo',4) RETURNING id")
	operatingExec(t, db, "INSERT INTO promo_code_usages(promo_code_id,user_id,bonus_amount) VALUES($1,$2,4)", promo, user)
	operatingUser(t, db, "3")
	now := time.Now().In(mustOperatingZone()).Format("2006-01-02")
	cost := service.BusinessRecordInput{SourceKey: "cost-record-1", Kind: "cost", Amount: decimal.NewFromInt(10), Date: now, Category: "账号采购", Notes: "测试成本"}
	costRecord, err := svc.Record(ctx, cost, 1)
	require.NoError(t, err)
	duplicate, err := svc.Record(ctx, cost, 1)
	require.NoError(t, err)
	require.Equal(t, costRecord.ID, duplicate.ID)
	changed := cost
	changed.Amount = decimal.NewFromInt(100)
	_, err = svc.Record(ctx, changed, 1)
	require.ErrorContains(t, err, "不同内容")
	_, err = svc.Record(ctx, service.BusinessRecordInput{SourceKey: "income-record-1", Kind: "income", Amount: decimal.NewFromInt(8), Date: now, Category: "独立服务"}, 1)
	require.NoError(t, err)
	_, err = svc.Record(ctx, service.BusinessRecordInput{SourceKey: "receipt-record-1", Kind: "receipt", Amount: decimal.NewFromInt(1000), Date: now, Category: "线下预收款"}, 1)
	require.NoError(t, err)
	r = operatingOverview(t, svc)
	require.Equal(t, "20", r.Summary.BalanceRevenue.String())
	require.Equal(t, "25", r.Summary.GiftCost.String())
	require.Equal(t, "3", r.Summary.Profit.String())
	require.Equal(t, "153", r.CumulativeProfit.String())
	require.Equal(t, "1000", r.ManualReceipts.String())
	// Current price changes cannot rewrite a purchased term's frozen conversion.
	operatingExec(t, db, "UPDATE subscription_plans SET price=200 WHERE id=$1", plan)
	operatingExec(t, db, "UPDATE groups SET subscription_total_limit_usd=2000 WHERE id=$1", group)
	require.Equal(t, "10", operatingOverview(t, svc).Summary.SubscriptionRevenue.String())
	// Manual correction remains auditable and is idempotent.
	require.NoError(t, svc.ReverseRecord(ctx, costRecord.ID, 1, "重复费用"))
	require.NoError(t, svc.ReverseRecord(ctx, costRecord.ID, 1, "重复费用"))
	require.Equal(t, "13", operatingOverview(t, svc).Summary.Profit.String())
	records, err := svc.Records(ctx, 0)
	require.NoError(t, err)
	require.Len(t, records, 4)
	// Final capture is revenue; the held and released amounts are not.
	hold := &service.BatchImageBalanceHoldCommand{BatchID: "operating-batch", RequestID: service.BatchImageHoldRequestID("operating-batch"), APIKeyID: key, UserID: user, HoldAmount: 10}
	_, err = repo.ReserveBatchImageBalance(ctx, hold)
	require.NoError(t, err)
	capture := *hold
	capture.RequestID = service.BatchImageCaptureRequestID("operating-batch")
	capture.ActualAmount = 7
	_, err = repo.CaptureBatchImageBalance(ctx, &capture)
	require.NoError(t, err)
	_, err = repo.CaptureBatchImageBalance(ctx, &capture)
	require.NoError(t, err)
	require.Equal(t, "27", operatingOverview(t, svc).Summary.BalanceRevenue.String())
	// Fully rolled-back transactions leave no statistics.
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	_, err = tx.Exec("SELECT operating_record_usage('rollback', $1, $2, NULL, 0, 99, 0, 0)", user, group)
	require.NoError(t, err)
	require.NoError(t, tx.Rollback())
	require.Equal(t, "27", operatingOverview(t, svc).Summary.BalanceRevenue.String())
	// Historical reports are independent of usage-log cleanup.
	operatingExec(t, db, "DELETE FROM usage_logs")
	require.Equal(t, "20", operatingOverview(t, svc).Summary.Profit.String())
}
func mustOperatingZone() *time.Location { v, _ := time.LoadLocation("Asia/Shanghai"); return v }

func TestOperatingStatisticsUnknownGiftAndTermCap(t *testing.T) {
	db := operatingDB(t)
	svc := service.NewBusinessAnalyticsService(db)
	ctx := context.Background()
	user := operatingUser(t, db, "0")
	operatingInit(t, svc)
	group := operatingID(t, db, "INSERT INTO groups(name,platform,subscription_type) VALUES('赠送组','openai','subscription') RETURNING id")
	sub := operatingID(t, db, "INSERT INTO user_subscriptions(user_id,group_id,starts_at,expires_at) VALUES($1,$2,now(),now()+interval '30 days') RETURNING id", user, group)
	operatingExec(t, db, "SELECT operating_record_usage('gift-use',$1,$2,$3,1,0,100,0)", user, group, sub)
	r := operatingOverview(t, svc)
	require.Nil(t, r.Summary.Profit)
	require.Nil(t, r.Positions.SubscriptionValue)
	terms, err := svc.Terms(ctx, 0)
	require.NoError(t, err)
	require.Len(t, terms, 1)
	require.NoError(t, svc.ConfigureTerm(ctx, terms[0].ID, 1, service.BusinessTermInput{Amount: decimal.NewFromInt(100), Quota: decimal.NewFromInt(1000), Funding: "gift", Notes: "赠送100元套餐"}))
	r = operatingOverview(t, svc)
	require.Equal(t, "10", r.Summary.SubscriptionRevenue.String())
	require.Equal(t, "100", r.Summary.GiftCost.String())
	require.Equal(t, "-90", r.Summary.Profit.String())
	require.Equal(t, "90", r.Positions.SubscriptionValue.String())
	operatingExec(t, db, "SELECT operating_record_usage('gift-use-more',$1,$2,$3,1,0,2000,0)", user, group, sub)
	r = operatingOverview(t, svc)
	require.Equal(t, "100", r.Summary.SubscriptionRevenue.String())
	require.Equal(t, "0", r.Summary.Profit.String())
	require.Error(t, svc.ConfigureTerm(ctx, terms[0].ID, 1, service.BusinessTermInput{Amount: decimal.NewFromInt(200), Quota: decimal.NewFromInt(1000), Funding: "gift", Notes: "不能改历史"}))
	operatingExec(t, db, "UPDATE user_subscriptions SET expires_at=now()-interval '1 second' WHERE id=$1", sub)
	r = operatingOverview(t, svc)
	require.Zero(t, r.Positions.ActiveSubscriptions)
	require.Equal(t, "100", r.Summary.SubscriptionRevenue.String())
}

func TestOperatingUpgradePreservesLegacyArchive(t *testing.T) {
	var user, countBefore int64
	db := operatingDBBefore(t, func(db *sql.DB) {
		migration, err := migrations.FS.ReadFile("255_operating_statistics.sql")
		require.NoError(t, err)
		operatingExec(t, db, "CREATE TABLE schema_migrations(filename TEXT PRIMARY KEY,checksum TEXT NOT NULL,applied_at TIMESTAMPTZ DEFAULT now())")
		operatingExec(t, db, "INSERT INTO schema_migrations(filename,checksum) VALUES('255_operating_statistics.sql',$1)", fmt.Sprintf("%x", sha256.Sum256([]byte(strings.TrimSpace(string(migration))))))
		require.NoError(t, repository.ApplyMigrations(context.Background(), db))
		user = operatingUser(t, db, "123.45")
		require.NoError(t, db.QueryRow("SELECT count(*) FROM business_events").Scan(&countBefore))
		require.Positive(t, countBefore)
		operatingExec(t, db, "DELETE FROM schema_migrations WHERE filename='255_operating_statistics.sql'")
	})
	var count int64
	require.NoError(t, db.QueryRow("SELECT count(*) FROM business_events").Scan(&count))
	require.Equal(t, countBefore, count)
	operatingExec(t, db, "UPDATE users SET balance=balance+1 WHERE id=$1", user)
	require.NoError(t, db.QueryRow("SELECT count(*) FROM business_events").Scan(&count))
	require.Equal(t, countBefore, count)
	var balance string
	require.NoError(t, db.QueryRow("SELECT balance::text FROM users WHERE id=$1", user).Scan(&balance))
	require.Equal(t, "124.45000000", balance)
	var enabled *time.Time
	require.NoError(t, db.QueryRow("SELECT enabled_at FROM operating_settings WHERE id=1").Scan(&enabled))
	require.Nil(t, enabled)
	svc := service.NewBusinessAnalyticsService(db)
	operatingInit(t, svc)
	now := time.Now().In(mustOperatingZone()).Format("2006-01-02")
	_, err := svc.Record(context.Background(), service.BusinessRecordInput{SourceKey: "migration-record", Kind: "cost", Amount: decimal.NewFromInt(4), Date: now, Category: "采购"}, 1)
	require.NoError(t, err)
	require.NoError(t, repository.ApplyMigrations(context.Background(), db))
	require.Equal(t, "4", operatingOverview(t, svc).Summary.ManualCost.String())
	require.NoError(t, db.QueryRow("SELECT count(*) FROM business_events").Scan(&count))
	require.Equal(t, countBefore, count)
	if os.Getenv("OPERATING_ARCHIVE_TOOLS") == "1" {
		var database string
		require.NoError(t, db.QueryRow("SELECT current_database()").Scan(&database))
		u, err := url.Parse(os.Getenv("OPERATING_STATISTICS_TEST_DSN"))
		require.NoError(t, err)
		u.Path = "/" + database
		password, _ := u.User.Password()
		env := append(os.Environ(), "PGDATABASE="+database, "PGHOST="+u.Hostname(), "PGPORT="+u.Port(), "PGUSER="+u.User.Username(), "PGPASSWORD="+password, "PGSSLMODE=disable")
		destination := filepath.Join(t.TempDir(), "archive")
		cmd := exec.Command("python3", "../../../scripts/export-operating-legacy.py", destination)
		cmd.Env = env
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, string(output))
		raw, err := os.ReadFile(filepath.Join(destination, "manifest.json"))
		require.NoError(t, err)
		var manifest struct {
			Tables  map[string]int64 `json:"tables"`
			Cutover string           `json:"cutover_at"`
		}
		require.NoError(t, json.Unmarshal(raw, &manifest))
		require.Equal(t, countBefore, manifest.Tables["business_events"])
		require.NotEmpty(t, manifest.Cutover)
		// Restore into this isolated test database with its matching external schema.
		restore := exec.Command("pg_restore", "--clean", "--if-exists", "--no-owner", "--no-acl", "--exit-on-error", "--single-transaction", "--dbname="+u.String(), filepath.Join(destination, "legacy.dump"))
		output, err = restore.CombinedOutput()
		require.NoError(t, err, string(output))
		require.NoError(t, db.QueryRow("SELECT count(*) FROM business_events").Scan(&count))
		require.Equal(t, countBefore, count)
	}
}

func TestOperatingRechargeBonusAndCalendarQuota(t *testing.T) {
	db := operatingDB(t)
	svc := service.NewBusinessAnalyticsService(db)
	user := operatingUser(t, db, "0")
	operatingInit(t, svc)
	operatingExec(t, db, "UPDATE operating_settings SET quota_timezone='Asia/Shanghai'")
	operatingExec(t, db, `INSERT INTO payment_orders(user_id,user_email,user_name,amount,pay_amount,recharge_code,out_trade_no,payment_type,order_type,status,expires_at)
 VALUES($1,'test@local','test',110,100,'bonus-test','bonus-test','alipay','balance','COMPLETED',now()+interval '1 day')`, user)
	operatingExec(t, db, "INSERT INTO redeem_codes(code,type,value,status,used_by,used_at) VALUES('bonus-test','balance',110,'used',$1,now())", user)
	operatingExec(t, db, "UPDATE redeem_codes SET notes='retry evidence' WHERE code='bonus-test'")
	require.Equal(t, "10", operatingOverview(t, svc).Summary.GiftCost.String())
	require.Zero(t, operatingOverview(t, svc).Summary.BalanceRevenue.Sign())
	group := operatingID(t, db, "INSERT INTO groups(name,platform,subscription_type,daily_limit_usd) VALUES('日额度','openai','subscription',100) RETURNING id")
	sub := operatingID(t, db, `INSERT INTO user_subscriptions(user_id,group_id,starts_at,expires_at) VALUES($1,$2,'2026-09-01 12:00:00+08','2026-09-03 12:00:00+08') RETURNING id`, user, group)
	var quota string
	require.NoError(t, db.QueryRow("SELECT quota::text FROM operating_terms WHERE subscription_id=$1", sub).Scan(&quota))
	require.Equal(t, "300.00000000", quota)
	operatingExec(t, db, "UPDATE groups SET subscription_quota_reset_mode='until_subscription_expires',subscription_total_limit_usd=1000,weekly_limit_usd=500 WHERE id=$1", group)
	operatingExec(t, db, "UPDATE user_subscriptions SET term_version=term_version+1 WHERE id=$1", sub)
	require.NoError(t, db.QueryRow("SELECT quota::text FROM operating_terms WHERE subscription_id=$1 AND current_term", sub).Scan(&quota))
	require.Equal(t, "100.00000000", quota)
}

// Each settlement races the cutover using the same wallet locks as production.
func TestOperatingConcurrentCutoverAndRollback(t *testing.T) {
	db := operatingDB(t)
	ctx := context.Background()
	svc := service.NewBusinessAnalyticsService(db)
	user := operatingUser(t, db, "1000")
	key := operatingID(t, db, "INSERT INTO api_keys(user_id,key,name) VALUES($1,'cutover-key','test') RETURNING id", user)
	repo := repository.NewUsageBillingRepository(nil, db)
	start := make(chan struct{})
	errs := make(chan error, 21)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, err := repo.Apply(ctx, &service.UsageBillingCommand{RequestID: fmt.Sprintf("race-%d", i), APIKeyID: key, UserID: user, BalanceCost: 1})
			errs <- err
		}(i)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		_, err := svc.Initialize(ctx, service.BusinessSettings{Timezone: "Asia/Shanghai"}, 1)
		errs <- err
	}()
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	var balance string
	require.NoError(t, db.QueryRow("SELECT balance::text FROM users WHERE id=$1", user).Scan(&balance))
	require.Equal(t, "980.00000000", balance)
	var recorded int64
	require.NoError(t, db.QueryRow("SELECT count(*) FROM operating_usage").Scan(&recorded))
	require.LessOrEqual(t, recorded, int64(20))
	require.Equal(t, decimal.NewFromInt(recorded).String(), operatingOverview(t, svc).Summary.BalanceRevenue.String())
	// A post-cutover retry is ignored even when the first attempt preceded initialization.
	_, err := repo.Apply(ctx, &service.UsageBillingCommand{RequestID: "race-0", APIKeyID: key, UserID: user, BalanceCost: 1})
	require.NoError(t, err)
	require.Equal(t, decimal.NewFromInt(recorded).String(), operatingOverview(t, svc).Summary.BalanceRevenue.String())
	// Force statistics insertion failure and verify the wallet deduction rolls back.
	operatingExec(t, db, `CREATE FUNCTION reject_operating_test() RETURNS TRIGGER LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'test rollback'; END $$;
 CREATE TRIGGER reject_operating_test BEFORE INSERT ON operating_usage FOR EACH ROW EXECUTE FUNCTION reject_operating_test()`)
	_, err = repo.Apply(ctx, &service.UsageBillingCommand{RequestID: "must-rollback", APIKeyID: key, UserID: user, BalanceCost: 7})
	require.Error(t, err)
	require.NoError(t, db.QueryRow("SELECT balance::text FROM users WHERE id=$1", user).Scan(&balance))
	require.Equal(t, "980.00000000", balance)
	hold := &service.BatchImageBalanceHoldCommand{BatchID: "rollback-batch", RequestID: service.BatchImageHoldRequestID("rollback-batch"), APIKeyID: key, UserID: user, HoldAmount: 10}
	_, err = repo.ReserveBatchImageBalance(ctx, hold)
	require.NoError(t, err)
	capture := *hold
	capture.RequestID = service.BatchImageCaptureRequestID(hold.BatchID)
	capture.ActualAmount = 7
	_, err = repo.CaptureBatchImageBalance(ctx, &capture)
	require.Error(t, err)
	var frozen string
	require.NoError(t, db.QueryRow("SELECT balance::text,frozen_balance::text FROM users WHERE id=$1", user).Scan(&balance, &frozen))
	require.Equal(t, "970.00000000", balance)
	require.Equal(t, "10.00000000", frozen)
}

func TestOperatingStaleSubscriptionAndReportingDays(t *testing.T) {
	db := operatingDB(t)
	ctx := context.Background()
	svc := service.NewBusinessAnalyticsService(db)
	user := operatingUser(t, db, "100")
	g := operatingID(t, db, "INSERT INTO groups(name,platform,subscription_type,subscription_total_limit_usd) VALUES('calendar','openai','subscription',1000) RETURNING id")
	sub := operatingID(t, db, "INSERT INTO user_subscriptions(user_id,group_id,starts_at,expires_at,term_version) VALUES($1,$2,now(),now()+interval '30 days',2) RETURNING id", user, g)
	key := operatingID(t, db, "INSERT INTO api_keys(user_id,key,name,group_id) VALUES($1,'stale-key','test',$2) RETURNING id", user, g)
	_, err := svc.Initialize(ctx, service.BusinessSettings{Timezone: "America/New_York"}, 1)
	require.NoError(t, err)
	repo := repository.NewUsageBillingRepository(nil, db)
	result, err := repo.Apply(ctx, &service.UsageBillingCommand{RequestID: "old-term", APIKeyID: key, UserID: user, SubscriptionID: &sub, SubscriptionTermVersion: 1, SubscriptionCost: 5})
	require.NoError(t, err)
	require.True(t, result.SubscriptionTermStale)
	var count int
	require.NoError(t, db.QueryRow("SELECT count(*) FROM operating_usage").Scan(&count))
	require.Zero(t, count)
	// UTC instants on either side of local midnight and the spring DST transition.
	operatingExec(t, db, "UPDATE operating_settings SET enabled_at='2026-03-01T00:00:00Z'")
	operatingExec(t, db, `INSERT INTO operating_rewards(source_key,amount,source,occurred_at) VALUES
 ('before-midnight',1,'test','2026-03-08T04:59:59Z'),('after-midnight',2,'test','2026-03-08T05:00:00Z'),('after-dst',3,'test','2026-03-08T07:00:00Z')`)
	r, err := svc.Overview(ctx, "2026-03-07", "2026-03-08")
	require.NoError(t, err)
	require.Len(t, r.Daily, 2)
	require.Equal(t, "1", r.Daily[0].GiftCost.String())
	require.Equal(t, "5", r.Daily[1].GiftCost.String())
}
