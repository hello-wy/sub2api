package repository_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func businessWorkflowDB(t *testing.T) (*sql.DB, *service.BusinessLedgerService, time.Time, int64, int64) {
	t.Helper()
	dsn := os.Getenv("BUSINESS_LEDGER_TEST_DSN")
	if dsn == "" {
		t.Skip("set BUSINESS_LEDGER_TEST_DSN for isolated PostgreSQL tests")
	}
	admin, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	name := fmt.Sprintf("business_workflow_test_%d", time.Now().UnixNano())
	_, err = admin.Exec("CREATE DATABASE " + name)
	require.NoError(t, err)
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	u.Path = "/" + name
	db, err := sql.Open("postgres", u.String())
	require.NoError(t, err)
	ledger := service.NewBusinessLedgerService(db)
	t.Cleanup(func() {
		ledger.Stop()
		_ = db.Close()
		_, _ = admin.Exec("DROP DATABASE " + name + " WITH (FORCE)")
		_ = admin.Close()
	})
	require.NoError(t, repository.ApplyMigrations(context.Background(), db))
	var at time.Time
	var user, account int64
	require.NoError(t, db.QueryRow(`SELECT enabled_at FROM business_ledger_config WHERE id=1`).Scan(&at))
	require.NoError(t, db.QueryRow(`INSERT INTO users(email,password_hash,username)VALUES('workflow@test.local','hash','Workflow')RETURNING id`).Scan(&user))
	require.NoError(t, db.QueryRow(`INSERT INTO accounts(name,platform,type,credentials)VALUES('shared supplier','openai','apikey','{}')RETURNING id`).Scan(&account))
	return db, ledger, at, user, account
}
func emitWorkflowUsage(t *testing.T, db *sql.DB, user, account int64, at time.Time, n int) {
	t.Helper()
	_, err := db.Exec(`SELECT business_emit_usage(jsonb_build_object('user_id',$1::bigint,'api_key_id',999,'request_id','workflow-'||i,'account_id',$2::bigint,'created_at',$3::timestamptz,'model','gpt','upstream_model','supplier-gpt','actual_cost','0','total_cost','1','billing_type',0)) FROM generate_series(1,$4::integer) i`, user, account, at, n)
	require.NoError(t, err)
}
func workflowProject(t *testing.T, ledger *service.BusinessLedgerService, db *sql.DB) {
	t.Helper()
	ctx := context.Background()
	require.Eventually(t, func() bool {
		_, err := ledger.Project(ctx)
		if err != nil {
			return false
		}
		var n int
		err = db.QueryRow(`SELECT COUNT(*) FROM business_events e LEFT JOIN business_projection_processed p ON p.event_id=e.id WHERE p.event_id IS NULL`).Scan(&n)
		return err == nil && n == 0
	}, 10*time.Second, 20*time.Millisecond)
}
func TestBusinessLedgerGroupedRepairAndInvoiceWorkflow(t *testing.T) {
	db, ledger, at, user, account := businessWorkflowDB(t)
	ctx := context.Background()
	end := at.Add(time.Hour)
	emitWorkflowUsage(t, db, user, account, at, 205)
	workflowProject(t, ledger, db)
	summary, err := ledger.Issues(ctx, at, end, 0)
	require.NoError(t, err)
	require.EqualValues(t, 1, summary.Total)
	require.Equal(t, "cost_binding", summary.Items[0].Kind)
	require.EqualValues(t, 205, summary.Items[0].AffectedCount)
	require.NotNil(t, summary.Items[0].PeriodStart)
	require.Equal(t, 1, summary.Items[0].PeriodStart.Day())
	require.Equal(t, 0, summary.Items[0].PeriodStart.Hour())
	sources, err := ledger.Pending(ctx, 0)
	require.NoError(t, err)
	require.Empty(t, sources, "consuming requests must not become funding forms")
	input := service.BusinessRepairInput{Start: at, End: end, AccountIDs: []int64{account}}
	preview, err := ledger.PreviewCostRepair(ctx, input)
	require.NoError(t, err)
	require.EqualValues(t, 205, preview.MissingBinding)
	pool, err := ledger.CreatePool(ctx, service.BusinessCostPool{Name: "shared", Mode: "postpaid", Unit: "CNY"})
	require.NoError(t, err)
	_, err = ledger.CreateBindings(ctx, service.BusinessBatchBinding{AccountIDs: []int64{account, account}, PoolID: pool.ID, EffectiveAt: at.Add(-time.Minute)})
	require.NoError(t, err)
	preview, err = ledger.PreviewCostRepair(ctx, input)
	require.NoError(t, err)
	require.EqualValues(t, 205, preview.MissingRule)
	_, err = ledger.CreateRule(ctx, service.BusinessCostRule{PoolID: pool.ID, Model: "supplier-gpt", EffectiveAt: at.Add(-time.Minute), Basis: "request", UnitPrice: decimal.NewFromInt(1), CNYPerUnit: decimal.NewFromInt(2), Quality: "contract", Notes: "supplier contract"})
	require.NoError(t, err)
	preview, err = ledger.PreviewCostRepair(ctx, input)
	require.NoError(t, err)
	require.EqualValues(t, 205, preview.Repairable)
	input.ThroughID = preview.ThroughID
	input.Fingerprint = preview.Fingerprint
	input.Notes = "historical contract evidence"
	input.IdempotencyKey = "workflow-batch-1"
	var beforeRevision int64
	require.NoError(t, db.QueryRow(`SELECT revision FROM business_projection_state WHERE id=1`).Scan(&beforeRevision))
	job, err := ledger.QueueCostRepair(ctx, input, 1)
	require.NoError(t, err)
	require.EqualValues(t, 205, job.Count)
	again, err := ledger.QueueCostRepair(ctx, input, 1)
	require.NoError(t, err)
	require.Equal(t, job.ID, again.ID)
	workflowProject(t, ledger, db)
	status, err := ledger.CostRepairJob(ctx, job.ID)
	require.NoError(t, err)
	require.Equal(t, "completed", status.Status)
	var afterRevision int64
	require.NoError(t, db.QueryRow(`SELECT revision FROM business_projection_state WHERE id=1`).Scan(&afterRevision))
	require.Equal(t, beforeRevision+1, afterRevision, "all corrections share one rebuild")
	records, err := ledger.Records(ctx, 0, 100)
	require.NoError(t, err)
	require.Len(t, records, 1)
	require.Equal(t, "cost_repair_batch", records[0].Type)
	var immutable bool
	require.NoError(t, db.QueryRow(`SELECT bool_and(payload->'pool'='null'::jsonb) FROM business_events WHERE event_type='usage'`).Scan(&immutable))
	require.True(t, immutable)
	report, err := ledger.Overview(ctx, at, end)
	require.NoError(t, err)
	require.Equal(t, "410", report.UsageCost.String())
	summary, err = ledger.Issues(ctx, at, end, 0)
	require.NoError(t, err)
	require.EqualValues(t, 1, summary.Total)
	require.Equal(t, "invoice_needed", summary.Items[0].Kind)
	require.EqualValues(t, 205, summary.Items[0].AffectedCount)
	preview, err = ledger.PreviewCostRepair(ctx, service.BusinessRepairInput{Start: at, End: end})
	require.NoError(t, err)
	require.Zero(t, preview.Repairable)
	// The complete invoice closes the grouped task, and only its difference is booked.
	bill := service.BusinessRecordInput{IdempotencyKey: "workflow-bill-1", Type: "reconciliation", At: time.Now().UTC(), Payload: map[string]any{"pool_id": pool.ID, "starts_at": at.Format(time.RFC3339Nano), "ends_at": end.Format(time.RFC3339Nano), "bill_amount_cny": "400", "notes": "complete supplier invoice"}}
	billEvent, err := ledger.Record(ctx, bill, 1)
	require.NoError(t, err)
	workflowProject(t, ledger, db)
	summary, err = ledger.Issues(ctx, at, end, 0)
	require.NoError(t, err)
	require.Zero(t, summary.Total)
	report, err = ledger.Overview(ctx, at, end)
	require.NoError(t, err)
	require.Equal(t, "400", report.UsageCost.String())
	// A later actual-cost correction reopens the invoice task instead of hiding it.
	var usageID int64
	require.NoError(t, db.QueryRow(`SELECT MIN(id) FROM business_events WHERE event_type='usage'`).Scan(&usageID))
	_, err = ledger.Record(ctx, service.BusinessRecordInput{IdempotencyKey: "workflow-correction", Type: "annotation", At: time.Now().UTC(), Payload: map[string]any{"source_event_id": usageID, "fields": map[string]any{"actual_supplier_cost_cny": "3"}, "notes": "actual charge"}}, 1)
	require.NoError(t, err)
	workflowProject(t, ledger, db)
	summary, err = ledger.Issues(ctx, at, end, 0)
	require.NoError(t, err)
	require.EqualValues(t, 1, summary.Total)
	require.Equal(t, "invoice_changed", summary.Items[0].Kind)
	report, err = ledger.Overview(ctx, at, end)
	require.NoError(t, err)
	require.Nil(t, report.Profit)
	_, err = ledger.Record(ctx, service.BusinessRecordInput{IdempotencyKey: "workflow-reverse", Type: "reversal", At: time.Now().UTC(), Payload: map[string]any{"reverses_id": billEvent.ID, "notes": "replace obsolete invoice"}}, 1)
	require.NoError(t, err)
	workflowProject(t, ledger, db)
	summary, err = ledger.Issues(ctx, at, end, 0)
	require.NoError(t, err)
	require.Equal(t, "invoice_needed", summary.Items[0].Kind)
}

func TestBusinessLedgerRepairProtectsConfirmedAndStalePreview(t *testing.T) {
	db, ledger, at, user, account := businessWorkflowDB(t)
	ctx := context.Background()
	end := at.Add(time.Hour)
	emitWorkflowUsage(t, db, user, account, at, 2)
	workflowProject(t, ledger, db)
	var usageID int64
	require.NoError(t, db.QueryRow(`SELECT MIN(id) FROM business_events WHERE event_type='usage'`).Scan(&usageID))
	_, err := ledger.Record(ctx, service.BusinessRecordInput{IdempotencyKey: "actual-cost-first", Type: "annotation", At: time.Now().UTC(), Payload: map[string]any{"source_event_id": usageID, "fields": map[string]any{"actual_supplier_cost_cny": "7"}, "notes": "actual cost"}}, 1)
	require.NoError(t, err)
	workflowProject(t, ledger, db)
	pool, err := ledger.CreatePool(ctx, service.BusinessCostPool{Name: "fixed account", Mode: "fixed", Unit: "CNY"})
	require.NoError(t, err)
	_, err = ledger.CreateBindings(ctx, service.BusinessBatchBinding{AccountIDs: []int64{account}, PoolID: pool.ID, EffectiveAt: at.Add(-time.Hour)})
	require.NoError(t, err)
	in := service.BusinessRepairInput{Start: at, End: end}
	preview, err := ledger.PreviewCostRepair(ctx, in)
	require.NoError(t, err)
	require.EqualValues(t, 1, preview.Repairable)
	require.EqualValues(t, 1, preview.Protected)
	in.ThroughID = preview.ThroughID
	in.Fingerprint = preview.Fingerprint
	in.Notes = "batch test"
	in.IdempotencyKey = "protected-batch"
	other, err := ledger.CreatePool(ctx, service.BusinessCostPool{Name: "new contract", Mode: "postpaid", Unit: "CNY"})
	require.NoError(t, err)
	_, err = ledger.CreateBinding(ctx, service.BusinessCostBinding{AccountID: account, PoolID: other.ID, EffectiveAt: at.Add(-time.Minute)})
	require.NoError(t, err)
	_, err = ledger.QueueCostRepair(ctx, in, 1)
	require.ErrorContains(t, err, "重新预览")
	var count int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM business_events WHERE event_type='cost_repair_batch'`).Scan(&count))
	require.Zero(t, count)
	// Incomplete bulk account selection rolls back all bindings.
	_, err = ledger.CreateBindings(ctx, service.BusinessBatchBinding{AccountIDs: []int64{account, 999999}, PoolID: pool.ID, EffectiveAt: at})
	require.Error(t, err)
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM business_cost_bindings WHERE effective_at=$1`, at).Scan(&count))
	require.Zero(t, count)
	report, err := ledger.Overview(ctx, at, end)
	require.NoError(t, err)
	require.Equal(t, "7", report.UsageCost.String())
}

func TestBusinessLedgerSourcesAndCalculationAreSeparate(t *testing.T) {
	db, ledger, at, user, account := businessWorkflowDB(t)
	ctx := context.Background()
	end := at.Add(time.Hour)
	var opening int64
	require.NoError(t, db.QueryRow(`INSERT INTO business_events(source_key,event_type,user_id,occurred_at,payload)VALUES('test-opening','opening_unknown',$1,$2,'{"credits":"100"}')RETURNING id`, user, at).Scan(&opening))
	workflowProject(t, ledger, db)
	summary, err := ledger.Issues(ctx, at, end, 0)
	require.NoError(t, err)
	require.EqualValues(t, 1, summary.Total)
	require.EqualValues(t, 1, summary.Items[0].SourceCount)
	_, err = ledger.Record(ctx, service.BusinessRecordInput{IdempotencyKey: "partial-opening", Type: "annotation", At: time.Now().UTC(), Payload: map[string]any{"source_event_id": opening, "fields": map[string]any{"paid_credits": "60", "gift_credits": "10", "unknown_credits": "30", "amount_cny": "48"}, "notes": "partial source"}}, 1)
	require.NoError(t, err)
	workflowProject(t, ledger, db)
	sources, err := ledger.PendingSources(ctx, user, 0)
	require.NoError(t, err)
	require.Len(t, sources, 1)
	require.Equal(t, "30", sources[0].Payload["unknown_credits"])
	_, err = ledger.Record(ctx, service.BusinessRecordInput{IdempotencyKey: "complete-opening", Type: "annotation", At: time.Now().UTC(), Payload: map[string]any{"source_event_id": opening, "fields": map[string]any{"paid_credits": "90", "gift_credits": "10", "unknown_credits": "0", "amount_cny": "72"}, "notes": "all receipts matched"}}, 1)
	require.NoError(t, err)
	workflowProject(t, ledger, db)
	sources, err = ledger.PendingSources(ctx, user, 0)
	require.NoError(t, err)
	require.Empty(t, sources)
	emitWorkflowUsage(t, db, user, account, at, 2)
	summary, err = ledger.Issues(ctx, at, end, 0)
	require.NoError(t, err)
	require.EqualValues(t, 2, summary.ProcessingCount)
	require.Zero(t, summary.Total, "unprocessed events are not manual tasks")
	// Fixed account fee issues are grouped independently of price rules.
	pool, err := ledger.CreatePool(ctx, service.BusinessCostPool{Name: "fixed", Mode: "fixed", Unit: "CNY"})
	require.NoError(t, err)
	_, err = ledger.CreateBinding(ctx, service.BusinessCostBinding{AccountID: account, PoolID: pool.ID, EffectiveAt: at})
	require.NoError(t, err)
	raw, _ := json.Marshal(map[string]any{"user_id": user, "account_id": account, "api_key_id": 999, "request_id": "fixed-cost", "model": "gpt", "created_at": at, "actual_cost": "0"})
	_, err = db.Exec(`SELECT business_emit_usage($1::jsonb)`, string(raw))
	require.NoError(t, err)
	workflowProject(t, ledger, db)
	summary, err = ledger.Issues(ctx, at, end, 0)
	require.NoError(t, err)
	found := false
	for _, i := range summary.Items {
		if i.Kind == "fixed_cost" {
			found = true
			require.EqualValues(t, 1, i.AffectedCount)
		}
	}
	require.True(t, found)
}

func TestBusinessLedgerInvoiceProtectsMissingSnapshot(t *testing.T) {
	db, ledger, at, user, account := businessWorkflowDB(t)
	ctx := context.Background()
	end := at.Add(time.Hour)
	pool, err := ledger.CreatePool(ctx, service.BusinessCostPool{Name: "supplier", Mode: "postpaid", Unit: "CNY"})
	require.NoError(t, err)
	_, err = ledger.CreateBinding(ctx, service.BusinessCostBinding{AccountID: account, PoolID: pool.ID, EffectiveAt: at})
	require.NoError(t, err)
	emitWorkflowUsage(t, db, user, account, at, 3)
	workflowProject(t, ledger, db)
	summary, err := ledger.Issues(ctx, at, end, 0)
	require.NoError(t, err)
	require.EqualValues(t, 1, summary.Total)
	require.Equal(t, "cost_rule", summary.Items[0].Kind)
	require.Equal(t, "supplier-gpt", summary.Items[0].Model)
	_, err = ledger.Record(ctx, service.BusinessRecordInput{IdempotencyKey: "missing-price-bill", Type: "reconciliation", At: time.Now().UTC(), Payload: map[string]any{"pool_id": pool.ID, "starts_at": at.Format(time.RFC3339Nano), "ends_at": end.Format(time.RFC3339Nano), "bill_amount_cny": "9", "notes": "supplier actual invoice"}}, 1)
	require.NoError(t, err)
	workflowProject(t, ledger, db)
	summary, err = ledger.Issues(ctx, at, end, 0)
	require.NoError(t, err)
	require.Zero(t, summary.Total)
	_, err = ledger.CreateRule(ctx, service.BusinessCostRule{PoolID: pool.ID, Model: "*", EffectiveAt: at, Basis: "request", UnitPrice: decimal.NewFromInt(1), CNYPerUnit: decimal.NewFromInt(7), Quality: "contract", Notes: "backdated contract"})
	require.NoError(t, err)
	preview, err := ledger.PreviewCostRepair(ctx, service.BusinessRepairInput{Start: at, End: end})
	require.NoError(t, err)
	require.EqualValues(t, 3, preview.Protected)
	require.Zero(t, preview.Repairable)
	report, err := ledger.Overview(ctx, at, end)
	require.NoError(t, err)
	require.Equal(t, "9", report.UsageCost.String())
	require.NotNil(t, report.Profit)
}

func TestBusinessLedgerLargeIssueAggregation(t *testing.T) {
	db, ledger, at, user, account := businessWorkflowDB(t)
	emitWorkflowUsage(t, db, user, account, at, 5000)
	workflowProject(t, ledger, db)
	summary, err := ledger.Issues(context.Background(), at, at.Add(time.Hour), 0)
	require.NoError(t, err)
	require.EqualValues(t, 1, summary.Total)
	require.EqualValues(t, 5000, summary.Items[0].AffectedCount)
}

func TestBusinessLedgerWorkerDrainsBacklog(t *testing.T) {
	db, ledger, at, _, _ := businessWorkflowDB(t)
	// Distinct transaction identities exercise more than two projector pages.
	_, err := db.Exec(`INSERT INTO business_events(source_key,event_type,transaction_id,occurred_at,payload)
 SELECT 'backlog-'||i,'adjustment',-i,$1,'{"amount_cny":"1","entry_kind":"operating_cost"}'::jsonb FROM generate_series(1,1001) i`, at)
	require.NoError(t, err)
	ledger.Wake()
	require.Eventually(t, func() bool {
		var count int
		err := db.QueryRow(`SELECT COUNT(*) FROM business_projection_processed`).Scan(&count)
		return err == nil && count == 1001
	}, 10*time.Second, 25*time.Millisecond)
}
