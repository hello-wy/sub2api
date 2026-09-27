package repository_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/service"
	_ "github.com/lib/pq"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

// BUSINESS_LEDGER_TEST_DSN points to a disposable PostgreSQL server. Each test
// creates its own database and applies the complete production migration chain.
func TestBusinessLedgerPostgresAtomicCaptureAndReplay(t *testing.T) {
	dsn := os.Getenv("BUSINESS_LEDGER_TEST_DSN")
	if dsn == "" {
		t.Skip("set BUSINESS_LEDGER_TEST_DSN for PostgreSQL transaction tests")
	}
	ctx := context.Background()
	admin, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer func() { _ = admin.Close() }()
	dbName := fmt.Sprintf("business_ledger_test_%d", time.Now().UnixNano())
	_, err = admin.ExecContext(ctx, "CREATE DATABASE "+dbName)
	require.NoError(t, err)
	defer func() { _, _ = admin.ExecContext(ctx, "DROP DATABASE "+dbName+" WITH (FORCE)") }()
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	u.Path = "/" + dbName
	db, err := sql.Open("postgres", u.String())
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	require.NoError(t, repository.ApplyMigrations(ctx, db))
	ledger := service.NewBusinessLedgerService(db)
	defer ledger.Stop()
	var userID, accountID int64
	require.NoError(t, db.QueryRowContext(ctx, `INSERT INTO users(email,password_hash,username)VALUES('business@test.local','hash','经营测试')RETURNING id`).Scan(&userID))
	require.NoError(t, db.QueryRowContext(ctx, `INSERT INTO accounts(name,platform,type,credentials)VALUES('supplier','openai','apikey','{}')RETURNING id`).Scan(&accountID))
	pool, err := ledger.CreatePool(ctx, service.BusinessCostPool{Name: "Supplier", Unit: "request", Mode: "postpaid"})
	require.NoError(t, err)
	at := time.Now().Add(-time.Minute).UTC()
	_, err = ledger.CreateBinding(ctx, service.BusinessCostBinding{AccountID: accountID, PoolID: pool.ID, EffectiveAt: at})
	require.NoError(t, err)
	_, err = ledger.CreateRule(ctx, service.BusinessCostRule{PoolID: pool.ID, Model: "*", EffectiveAt: at, Basis: "request", UnitPrice: decimal.NewFromInt(1), CNYPerUnit: decimal.NewFromInt(6), Quality: "contract", Notes: "供应商测试合同"})
	require.NoError(t, err)
	first := service.BusinessRecordInput{IdempotencyKey: "receipt-one", Type: "receipt", UserID: userID, At: time.Now().UTC(), Payload: map[string]any{"amount_cny": "100", "credits": "100", "apply_balance": true, "notes": "线下实际收款"}}
	_, err = ledger.Record(ctx, first, 1)
	require.NoError(t, err)
	_, err = ledger.Record(ctx, first, 1)
	require.NoError(t, err)
	second := first
	second.IdempotencyKey = "receipt-two"
	second.At = time.Now().UTC()
	second.Payload = map[string]any{"amount_cny": "80", "credits": "100", "apply_balance": true, "notes": "会员成交价"}
	_, err = ledger.Record(ctx, second, 1)
	require.NoError(t, err)
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `UPDATE users SET balance=balance+100 WHERE id=$1`, userID)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `INSERT INTO welfare_records(user_id,user_email,amount,remarks,status)VALUES($1,'business@test.local',100,'gift','success')`, userID)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())
	snapshot := map[string]any{"user_id": userID, "account_id": accountID, "api_key_id": 1, "request_id": "financial-test-1", "model": "gpt-test", "created_at": time.Now().UTC(), "actual_cost": "30", "balance_cost": "30", "total_cost": "1", "billing_applied": true}
	raw, err := json.Marshal(snapshot)
	require.NoError(t, err)
	tx, err = db.BeginTx(ctx, nil)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `UPDATE users SET balance=balance-30 WHERE id=$1`, userID)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `SELECT business_emit_usage($1::jsonb)`, string(raw))
	require.NoError(t, err)
	require.NoError(t, tx.Commit())
	var enabled time.Time
	require.NoError(t, db.QueryRowContext(ctx, `SELECT enabled_at FROM business_ledger_config WHERE id=1`).Scan(&enabled))
	report, err := ledger.Overview(ctx, enabled, time.Now().Add(time.Hour))
	require.NoError(t, err)
	require.Equal(t, "180", report.CashIn.String())
	require.Equal(t, "18", report.Revenue.String())
	require.Equal(t, "6", report.UsageCost.String())
	require.Equal(t, "2", report.GiftCost.String())
	require.Equal(t, "12", report.KnownProfit.String())
	require.Nil(t, report.Profit)
	require.Equal(t, "162", report.WalletDeferred.String())
	// Duplicate source emission and projection retries do not duplicate profit.
	_, err = db.ExecContext(ctx, `SELECT business_emit_usage($1::jsonb)`, string(raw))
	require.NoError(t, err)
	again, err := ledger.Overview(ctx, enabled, time.Now().Add(time.Hour))
	require.NoError(t, err)
	require.Equal(t, report.Revenue.String(), again.Revenue.String())
	// An invoice verifies the saved estimate and posts only the actual difference.
	bill := service.BusinessRecordInput{IdempotencyKey: "supplier-bill-1", Type: "reconciliation", At: time.Now().UTC(), Payload: map[string]any{"pool_id": pool.ID, "starts_at": enabled.Format(time.RFC3339Nano), "ends_at": time.Now().UTC().Format(time.RFC3339Nano), "bill_amount_cny": "7", "notes": "供应商月末账单"}}
	_, err = ledger.Record(ctx, bill, 1)
	require.NoError(t, err)
	verified, err := ledger.Overview(ctx, enabled, time.Now().Add(time.Hour))
	require.NoError(t, err)
	require.Equal(t, "7", verified.UsageCost.String())
	require.NotNil(t, verified.Profit)
	require.Equal(t, "11", verified.Profit.String())
	_, err = ledger.Record(ctx, bill, 1)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `UPDATE business_events SET payload='{}' WHERE source_key='manual:receipt-one'`)
	require.Error(t, err)
	_, err = db.ExecContext(ctx, `DELETE FROM business_events WHERE source_key='manual:receipt-one'`)
	require.Error(t, err)
	// Configuration, pending list and trace have independent database paths.
	config, err := ledger.CostConfiguration(ctx)
	require.NoError(t, err)
	require.Len(t, config.Rules, 1)
	_, err = ledger.Pending(ctx, 0)
	require.NoError(t, err)
	var usageID int64
	require.NoError(t, db.QueryRowContext(ctx, `SELECT id FROM business_events WHERE source_key='usage:1:financial-test-1'`).Scan(&usageID))
	trace, err := ledger.Trace(ctx, usageID)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(trace), 2)
	// Rebuilding all derived entries must preserve invoice coverage and amount.
	_, err = ledger.Record(ctx, service.BusinessRecordInput{IdempotencyKey: "verified-usage-source", Type: "annotation", At: time.Now().UTC(), Payload: map[string]any{"source_event_id": usageID, "fields": map[string]any{"actual_supplier_cost_cny": "6"}, "notes": "补录供应商原始扣费凭据"}}, 1)
	require.NoError(t, err)
	replayed, err := ledger.Overview(ctx, enabled, time.Now().Add(time.Hour))
	require.NoError(t, err)
	require.Equal(t, "7", replayed.UsageCost.String())
	require.NotNil(t, replayed.Profit)
	require.Equal(t, "11", replayed.Profit.String())
	// A lower sequence ID can commit AFTER a newer one has been projected.
	late, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	_, err = late.ExecContext(ctx, `SELECT business_event_emit('late-event','adjustment',NULL,NOW(),'"unused"'::jsonb)`)
	require.Error(t, err)
	require.NoError(t, late.Rollback()) // malformed payload cannot partially commit
	late, err = db.BeginTx(ctx, nil)
	require.NoError(t, err)
	_, err = late.ExecContext(ctx, `SELECT business_event_emit('late-event','adjustment',NULL,NOW(),' {"amount_cny":"1","entry_kind":"operating_cost"}'::jsonb)`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `SELECT business_event_emit('early-event','adjustment',NULL,NOW(),' {"amount_cny":"2","entry_kind":"operating_cost"}'::jsonb)`)
	require.NoError(t, err)
	_, err = ledger.Project(ctx)
	require.NoError(t, err)
	require.NoError(t, late.Commit())
	_, err = ledger.Project(ctx)
	require.NoError(t, err)
	var processed int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM business_projection_processed p JOIN business_events e ON e.id=p.event_id WHERE e.source_key IN ('late-event','early-event')`).Scan(&processed))
	require.Equal(t, 2, processed)
	// Run real repository billing concurrently: one wallet lock, one immutable
	// journal event per request, and failed journal writes roll billing back.
	var keyID int64
	require.NoError(t, db.QueryRowContext(ctx, `INSERT INTO api_keys(user_id,key,name)VALUES($1,'business-test-api-key','business')RETURNING id`, userID).Scan(&keyID))
	billing := repository.NewUsageBillingRepository(nil, db)
	var wg sync.WaitGroup
	failures := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			cmd := &service.UsageBillingCommand{RequestID: fmt.Sprintf("concurrent-%d", i), UserID: userID, APIKeyID: keyID, AccountID: accountID, BalanceCost: 1,
				BusinessUsage: map[string]any{"user_id": userID, "api_key_id": keyID, "account_id": accountID, "request_id": fmt.Sprintf("concurrent-%d", i), "model": "gpt-test", "actual_cost": "1", "total_cost": "1", "created_at": time.Now().UTC()}}
			result, err := billing.Apply(ctx, cmd)
			if err == nil && !result.Applied {
				err = fmt.Errorf("first request was not applied")
			}
			if err == nil {
				again, e := billing.Apply(ctx, cmd)
				err = e
				if e == nil && again.Applied {
					err = fmt.Errorf("duplicate request applied twice")
				}
			}
			failures <- err
		}(i)
	}
	wg.Wait()
	close(failures)
	for err := range failures {
		require.NoError(t, err)
	}
	var balance string
	require.NoError(t, db.QueryRowContext(ctx, `SELECT balance::text FROM users WHERE id=$1`, userID).Scan(&balance))
	require.True(t, decimal.RequireFromString(balance).Equal(decimal.NewFromInt(262)))
	_, err = billing.Apply(ctx, &service.UsageBillingCommand{RequestID: "must-rollback", UserID: userID, APIKeyID: keyID, AccountID: accountID, BalanceCost: 1, BusinessUsage: map[string]any{"user_id": userID, "api_key_id": keyID, "request_id": "must-rollback", "created_at": "invalid-timestamp"}})
	require.Error(t, err)
	require.NoError(t, db.QueryRowContext(ctx, `SELECT balance::text FROM users WHERE id=$1`, userID).Scan(&balance))
	require.True(t, decimal.RequireFromString(balance).Equal(decimal.NewFromInt(262)))
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM usage_billing_dedup WHERE request_id='must-rollback'`).Scan(&processed))
	require.Zero(t, processed)
	_, err = db.ExecContext(ctx, `INSERT INTO batch_image_jobs(batch_id,user_id,api_key_id,account_id,provider,model,item_count,success_count,actual_cost)VALUES('ledger-batch',$1,$2,$3,'vertex','gpt-test',1,1,3)`, userID, keyID, accountID)
	require.NoError(t, err)
	_, err = billing.ReserveBatchImageBalance(ctx, &service.BatchImageBalanceHoldCommand{RequestID: service.BatchImageHoldRequestID("ledger-batch"), BatchID: "ledger-batch", UserID: userID, APIKeyID: keyID, HoldAmount: 10})
	require.NoError(t, err)
	_, err = billing.CaptureBatchImageBalance(ctx, &service.BatchImageBalanceHoldCommand{RequestID: service.BatchImageCaptureRequestID("ledger-batch"), BatchID: "ledger-batch", UserID: userID, APIKeyID: keyID, HoldAmount: 10, ActualAmount: 3})
	require.NoError(t, err)
	report, err = ledger.Overview(ctx, enabled, time.Now().Add(time.Hour))
	require.NoError(t, err)
	require.Equal(t, "24.6", report.Revenue.String())
	require.Equal(t, "155.4", report.WalletDeferred.String())
	// Source corrections rebuild the projection; stable entry IDs keep verified
	// invoices attached, and raw source history still survives object deletion.
	_, err = db.ExecContext(ctx, `INSERT INTO usage_logs(user_id,api_key_id,account_id,request_id,model,actual_cost,total_cost)VALUES($1,$2,$3,'financial-test-1','gpt-test',30,1)`, userID, keyID, accountID)
	require.NoError(t, err)
	var logCount int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM usage_logs WHERE user_id=$1`, userID).Scan(&logCount))
	require.Equal(t, 1, logCount)
	_, err = db.ExecContext(ctx, `DELETE FROM usage_logs WHERE user_id=$1`, userID)
	require.NoError(t, err)
	trace, err = ledger.Trace(ctx, usageID)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(trace), 6)
	// Subscription renewals append notes; snapshot the latest complete order line.
	var groupID, subID, oldOrder, newOrder int64
	require.NoError(t, db.QueryRowContext(ctx, `INSERT INTO groups(name,subscription_type)VALUES('ledger subscription','subscription')RETURNING id`).Scan(&groupID))
	for i, amount := range []string{"90", "60"} {
		var orderID int64
		require.NoError(t, db.QueryRowContext(ctx, `INSERT INTO payment_orders(user_id,amount,pay_amount,order_type,status,expires_at,subscription_group_id)VALUES($1,$2,$2,'subscription','COMPLETED',NOW()+INTERVAL '30 days',$3)RETURNING id`, userID, amount, groupID).Scan(&orderID))
		if i == 0 {
			oldOrder = orderID
		} else {
			newOrder = orderID
		}
	}
	notes := fmt.Sprintf("原有备注\npayment order %d\npayment order %d\n", oldOrder, newOrder)
	require.NoError(t, db.QueryRowContext(ctx, `INSERT INTO user_subscriptions(user_id,group_id,starts_at,expires_at,notes)VALUES($1,$2,NOW(),NOW()+INTERVAL '30 days',$3)RETURNING id`, userID, groupID, notes).Scan(&subID))
	var matchedOrder int64
	require.NoError(t, db.QueryRowContext(ctx, `SELECT (payload->'order'->>'id')::bigint FROM business_events WHERE event_type='user_subscriptions' AND payload->>'id'=$1::text ORDER BY id DESC LIMIT 1`, subID).Scan(&matchedOrder))
	require.Equal(t, newOrder, matchedOrder)

	// An older order may finish paying later; fulfillment order is the last
	// appended line, not the largest order ID.
	_, err = db.ExecContext(ctx, `UPDATE user_subscriptions SET term_version=term_version+1, notes=notes || $2 WHERE id=$1`, subID, fmt.Sprintf("payment order %d\n", oldOrder))
	require.NoError(t, err)
	require.NoError(t, db.QueryRowContext(ctx, `SELECT (payload->'order'->>'id')::bigint FROM business_events WHERE event_type='user_subscriptions' AND payload->>'id'=$1::text ORDER BY id DESC LIMIT 1`, subID).Scan(&matchedOrder))
	require.Equal(t, oldOrder, matchedOrder)

	// A matching model/tier/image rule wins over the pool default. Later rules
	// cannot change the financial snapshot already captured for this request.
	variant, err := ledger.CreateRule(ctx, service.BusinessCostRule{PoolID: pool.ID, Model: "gpt-image", ServiceTier: "priority", ImageSize: "2K", EffectiveAt: at, Basis: "image", UnitPrice: decimal.NewFromInt(2), CNYPerUnit: decimal.NewFromInt(6), Quality: "contract", Notes: "图片合同价"})
	require.NoError(t, err)
	variantUsage, err := json.Marshal(map[string]any{"user_id": userID, "api_key_id": keyID, "account_id": accountID, "model": "gpt-image", "service_tier": "priority", "image_size": "2K", "image_count": 2, "request_id": "price-snapshot"})
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `SELECT business_emit_usage($1::jsonb)`, string(variantUsage))
	require.NoError(t, err)
	var ruleID int64
	require.NoError(t, db.QueryRowContext(ctx, `SELECT (payload->'rule'->>'id')::bigint FROM business_events WHERE payload->>'request_id'='price-snapshot'`).Scan(&ruleID))
	require.Equal(t, variant.ID, ruleID)
	originalRuleID := variant.ID
	variant.ID = 0
	variant.EffectiveAt = time.Now()
	variant.UnitPrice = decimal.NewFromInt(10)
	_, err = ledger.CreateRule(ctx, *variant)
	require.NoError(t, err)
	require.NoError(t, db.QueryRowContext(ctx, `SELECT (payload->'rule'->>'id')::bigint FROM business_events WHERE payload->>'request_id'='price-snapshot'`).Scan(&ruleID))
	require.Equal(t, originalRuleID, ruleID)
}
