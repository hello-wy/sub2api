package service

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"
)

type BusinessRepairInput struct {
	Start          time.Time `json:"starts_at"`
	End            time.Time `json:"ends_at"`
	AccountIDs     []int64   `json:"account_ids"`
	PoolID         int64     `json:"pool_id"`
	Model          string    `json:"model"`
	ThroughID      int64     `json:"through_event_id"`
	Fingerprint    string    `json:"fingerprint"`
	IdempotencyKey string    `json:"idempotency_key"`
	Notes          string    `json:"notes"`
}
type BusinessRepairPreview struct {
	ThroughID      int64  `json:"through_event_id"`
	Fingerprint    string `json:"fingerprint"`
	Repairable     int64  `json:"repairable_count"`
	MissingBinding int64  `json:"missing_binding_count"`
	MissingRule    int64  `json:"missing_rule_count"`
	Protected      int64  `json:"protected_count"`
}
type BusinessRepairJob struct {
	ID        int64     `json:"id"`
	Count     int64     `json:"count"`
	Status    string    `json:"status"`
	Notes     string    `json:"notes"`
	CreatedAt time.Time `json:"created_at"`
}

// A repair fills absent snapshots only, using the rules effective at usage time.
// Actual supplier costs and invoice-covered records never enter the repair set.
const businessRepairSQL = `WITH source AS (
 SELECT e.id,e.user_id,e.payload || COALESCE(a.fields,'{}'::jsonb) AS data,e.occurred_at,
 EXISTS (SELECT 1 FROM business_effective_cost_entries l WHERE l.event_id=e.id AND (l.invoice_verified OR l.invoice_stale)) AS invoice_protected
 FROM business_events e LEFT JOIN business_applied_annotations a ON a.source_id=e.id::text
 WHERE e.event_type='usage' AND e.occurred_at >= $1 AND e.occurred_at < $2 AND e.id<=$3
 AND (cardinality($4::bigint[])=0 OR (e.payload->>'account_id')::bigint=ANY($4::bigint[]))
 AND ($6='' OR COALESCE(NULLIF(e.payload->>'upstream_model',''),e.payload->>'model')=$6)
), pools AS (
 SELECT s.*,COALESCE(NULLIF(data->'pool','null'::jsonb),b.pool) AS resolved_pool
 FROM source s LEFT JOIN LATERAL (
 SELECT to_jsonb(p) AS pool FROM business_cost_bindings b JOIN business_cost_pools p ON p.id=b.pool_id
 WHERE b.account_id=(s.data->>'account_id')::bigint AND b.effective_at<=s.occurred_at
 ORDER BY b.effective_at DESC,b.id DESC LIMIT 1
 ) b ON true
), priced AS (
 SELECT p.*,COALESCE(NULLIF(data->'rule','null'::jsonb),r.rule) AS resolved_rule
 FROM pools p LEFT JOIN LATERAL (
 SELECT to_jsonb(r) AS rule FROM business_cost_rules r WHERE r.pool_id=(p.resolved_pool->>'id')::bigint
 AND r.effective_at<=p.occurred_at AND r.model IN ('*',COALESCE(NULLIF(p.data->>'upstream_model',''),p.data->>'model'))
 AND r.service_tier IN ('',COALESCE(p.data->>'service_tier','')) AND r.image_size IN ('',COALESCE(p.data->>'image_size',''))
 AND r.video_resolution IN ('',COALESCE(p.data->>'video_resolution',''))
 ORDER BY (r.model<>'*') DESC,((r.service_tier<>'')::int+(r.image_size<>'')::int+(r.video_resolution<>'')::int) DESC,r.effective_at DESC,r.id DESC LIMIT 1
 ) r ON true WHERE ($5=0 OR (p.resolved_pool->>'id')::bigint=$5)
), candidates AS (
 SELECT id,user_id,
 CASE WHEN data->>'actual_supplier_cost_cny' IS NOT NULL OR invoice_protected THEN 'protected'
 WHEN resolved_pool IS NULL THEN 'missing_binding'
 WHEN resolved_pool->>'mode'<>'fixed' AND resolved_rule IS NULL THEN 'missing_rule'
 ELSE 'repairable' END AS status,
 (CASE WHEN NULLIF(data->'pool','null'::jsonb) IS NULL THEN jsonb_build_object('pool',resolved_pool) ELSE '{}'::jsonb END ||
 CASE WHEN NULLIF(data->'rule','null'::jsonb) IS NULL AND resolved_rule IS NOT NULL THEN jsonb_build_object('rule',resolved_rule) ELSE '{}'::jsonb END) AS fields
 FROM priced WHERE NULLIF(data->'pool','null'::jsonb) IS NULL
 OR (data->'pool'->>'mode'<>'fixed' AND NULLIF(data->'rule','null'::jsonb) IS NULL)
) `

func validateBusinessRepair(v BusinessRepairInput) error {
	if v.Start.IsZero() || !v.End.After(v.Start) || v.End.Sub(v.Start) > 366*24*time.Hour || len(v.AccountIDs) > 1000 || v.PoolID < 0 || len(v.Model) > 200 || v.ThroughID < 0 {
		return fmt.Errorf("请选择不超过 366 天的补算范围，账号最多 1000 个")
	}
	for _, id := range v.AccountIDs {
		if id <= 0 {
			return fmt.Errorf("账号编号无效")
		}
	}
	return nil
}
func businessRepairArgs(v BusinessRepairInput) []any {
	ids := v.AccountIDs
	if ids == nil {
		ids = []int64{}
	}
	return []any{v.Start, v.End, v.ThroughID, pq.Array(ids), v.PoolID, v.Model}
}
func businessRepairPreview(ctx context.Context, tx *sql.Tx, v BusinessRepairInput) (*BusinessRepairPreview, error) {
	var calculating bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM business_events e LEFT JOIN business_projection_processed p ON p.event_id=e.id WHERE p.event_id IS NULL AND e.event_type IN ('annotation','reconciliation','reversal','cost_repair_batch'))`).Scan(&calculating); err != nil {
		return nil, err
	}
	if calculating {
		return nil, fmt.Errorf("账务修订正在计算，请完成后再预览或提交补算")
	}
	rows, err := tx.QueryContext(ctx, businessRepairSQL+`SELECT id,status,fields::text FROM candidates ORDER BY id`, businessRepairArgs(v)...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := &BusinessRepairPreview{ThroughID: v.ThroughID}
	hash := sha256.New()
	_, _ = fmt.Fprintf(hash, "%s|%s|%d|%v|%d|%s\n", v.Start.UTC().Format(time.RFC3339Nano), v.End.UTC().Format(time.RFC3339Nano), v.ThroughID, v.AccountIDs, v.PoolID, v.Model)
	for rows.Next() {
		var id int64
		var status, fields string
		if err = rows.Scan(&id, &status, &fields); err != nil {
			return nil, err
		}
		_, _ = fmt.Fprintf(hash, "%d|%s|%s\n", id, status, fields)
		switch status {
		case "repairable":
			out.Repairable++
		case "missing_binding":
			out.MissingBinding++
		case "missing_rule":
			out.MissingRule++
		case "protected":
			out.Protected++
		}
	}
	out.Fingerprint = hex.EncodeToString(hash.Sum(nil))
	return out, rows.Err()
}
func (s *BusinessLedgerService) PreviewCostRepair(ctx context.Context, v BusinessRepairInput) (*BusinessRepairPreview, error) {
	if err := validateBusinessRepair(v); err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	// Always establish a new cutoff on preview, excluding subsequent live traffic.
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(id),0) FROM business_events`).Scan(&v.ThroughID); err != nil {
		return nil, err
	}
	out, err := businessRepairPreview(ctx, tx, v)
	if err != nil {
		return nil, err
	}
	return out, tx.Commit()
}
func (s *BusinessLedgerService) QueueCostRepair(ctx context.Context, v BusinessRepairInput, actor int64) (*BusinessRepairJob, error) {
	if err := validateBusinessRepair(v); err != nil {
		return nil, err
	}
	if len(v.IdempotencyKey) < 8 || len(v.IdempotencyKey) > 128 || len(v.Fingerprint) != 64 || strings.TrimSpace(v.Notes) == "" || len(v.Notes) > 2000 {
		return nil, fmt.Errorf("请先预览补算范围，并填写凭据说明")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	// Record() uses the same lock for financial corrections. No invoice or
	// annotation can slip between revalidation and the single batch insert.
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(252254)`); err != nil {
		return nil, err
	}
	key := "cost-repair:" + v.IdempotencyKey
	raw, _ := json.Marshal(v)
	var existing int64
	var same bool
	err = tx.QueryRowContext(ctx, `SELECT id,payload->'request'=$2::jsonb FROM business_events WHERE source_key=$1`, key, string(raw)).Scan(&existing, &same)
	if err == nil {
		if !same {
			return nil, fmt.Errorf("幂等标识已用于其他补算")
		}
		_ = tx.Rollback()
		return s.CostRepairJob(ctx, existing)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	preview, err := businessRepairPreview(ctx, tx, v)
	if err != nil {
		return nil, err
	}
	if preview.Fingerprint != v.Fingerprint {
		return nil, fmt.Errorf("成本配置或核对状态已变化，请重新预览")
	}
	if preview.Repairable == 0 {
		return nil, fmt.Errorf("当前没有可补算的缺失记录，请先完善有效期内的账号绑定和价格")
	}
	payload, _ := json.Marshal(map[string]any{"request": v, "count": preview.Repairable, "notes": v.Notes})
	var id int64
	err = tx.QueryRowContext(ctx, `INSERT INTO business_events(source_key,event_type,occurred_at,payload,actor_id)VALUES($1,'cost_repair_batch',clock_timestamp(),$2,$3)RETURNING id`, key, string(payload), actor).Scan(&id)
	if err != nil {
		return nil, err
	}
	args := append(businessRepairArgs(v), id, actor, v.Notes)
	result, err := tx.ExecContext(ctx, businessRepairSQL+`INSERT INTO business_events(source_key,event_type,user_id,occurred_at,payload,actor_id)
 SELECT 'cost-repair-item:'||$7::bigint||':'||id,'annotation',user_id,clock_timestamp(),
 jsonb_build_object('source_event_id',id,'fields',fields,'repair_batch_id',$7::bigint,'notes',$9::text),$8 FROM candidates WHERE status='repairable'`, args...)
	if err != nil {
		return nil, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if count != preview.Repairable {
		return nil, fmt.Errorf("补算范围发生变化，请重新预览")
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	s.Wake()
	return s.CostRepairJob(ctx, id)
}
func (s *BusinessLedgerService) CostRepairJob(ctx context.Context, id int64) (*BusinessRepairJob, error) {
	var v BusinessRepairJob
	err := s.db.QueryRowContext(ctx, `SELECT e.id,(e.payload->>'count')::bigint,CASE WHEN p.event_id IS NOT NULL THEN 'completed' WHEN s.last_error<>'' THEN 'retrying' ELSE 'queued' END,e.payload->>'notes',e.recorded_at
 FROM business_events e CROSS JOIN business_projection_state s LEFT JOIN business_projection_processed p ON p.event_id=e.id
 WHERE e.id=$1 AND e.event_type='cost_repair_batch' AND s.id=1`, id).Scan(&v.ID, &v.Count, &v.Status, &v.Notes, &v.CreatedAt)
	return &v, err
}
func (s *BusinessLedgerService) CostRepairJobs(ctx context.Context) ([]BusinessRepairJob, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT e.id,(e.payload->>'count')::bigint,CASE WHEN p.event_id IS NOT NULL THEN 'completed' WHEN s.last_error<>'' THEN 'retrying' ELSE 'queued' END,e.payload->>'notes',e.recorded_at
 FROM business_events e CROSS JOIN business_projection_state s LEFT JOIN business_projection_processed p ON p.event_id=e.id
 WHERE e.event_type='cost_repair_batch' AND s.id=1 ORDER BY e.id DESC LIMIT 20`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []BusinessRepairJob{}
	for rows.Next() {
		var v BusinessRepairJob
		if err = rows.Scan(&v.ID, &v.Count, &v.Status, &v.Notes, &v.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
