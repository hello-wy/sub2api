package service

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/lib/pq"
	"github.com/shopspring/decimal"
)

type BusinessIssue struct {
	ObjectTypes   []string        `json:"object_types"`
	PeriodStart   *time.Time      `json:"period_start_at,omitempty"`
	PeriodEnd     *time.Time      `json:"period_end_at,omitempty"`
	Key           string          `json:"key"`
	Kind          string          `json:"kind"`
	AccountID     int64           `json:"account_id"`
	PoolID        int64           `json:"pool_id"`
	UserID        int64           `json:"user_id"`
	Name          string          `json:"name"`
	Model         string          `json:"model"`
	Period        string          `json:"period"`
	AffectedCount int64           `json:"affected_count"`
	SourceCount   int64           `json:"source_count"`
	Amount        decimal.Decimal `json:"known_amount_cny"`
	FirstAt       time.Time       `json:"first_at"`
	LastAt        time.Time       `json:"last_at"`
}
type BusinessIssueSummary struct {
	Start            time.Time       `json:"starts_at"`
	End              time.Time       `json:"ends_at"`
	Items            []BusinessIssue `json:"items"`
	Total            int64           `json:"total"`
	ProcessingCount  int64           `json:"processing_count"`
	UpdatedAt        time.Time       `json:"updated_at"`
	Revision         int64           `json:"revision"`
	CalculationError bool            `json:"calculation_error"`
}

// Group before paginating. Thousands of requests caused by one missing binding
// are one task. Source tasks include opening balances outside the report period.
const businessIssuesSQL = `WITH cost AS (
 SELECT l.*,COALESCE((l.payload->>'pool_id')::bigint,(l.payload->'pool'->>'id')::bigint,(SELECT (e.payload->>'pool_id')::bigint FROM business_events e WHERE e.id=l.event_id),0) AS pool_id,
 CASE WHEN invoice_stale THEN 'invoice_changed'
 WHEN kind='usage_cost' THEN 'invoice_needed'
 WHEN payload->>'reason' IN ('上游采购额度不足或期初未录入','已补录实际成本，但采购额度或期初仍不足，待核对','采购剩余额度不足，待核对') THEN 'procurement'
 WHEN payload->'pool' IS NULL OR payload->'pool'='null'::jsonb THEN 'cost_binding'
 WHEN payload->'rule' IS NULL OR payload->'rule'='null'::jsonb THEN 'cost_rule'
 ELSE 'cost_other' END AS issue_kind
 FROM business_effective_cost_entries l
 WHERE occurred_at >= $1 AND occurred_at < $2 AND NOT invoice_verified
 AND (kind='cost_gap' OR quality='estimated' OR invoice_stale)
), raw AS (
 SELECT issue_kind AS kind,
 CASE WHEN issue_kind IN ('cost_binding','cost_other') THEN COALESCE(account_id,0) ELSE 0 END AS account_id,
 pool_id,0::bigint AS user_id,
 CASE WHEN issue_kind IN ('cost_binding','cost_other') THEN COALESCE(NULLIF(payload->>'account_name',''),'账号 #'||COALESCE(account_id,0))
 ELSE COALESCE(NULLIF(payload->'pool'->>'name',''),NULLIF(payload->>'pool_name',''),'成本池 #'||pool_id) END AS name,
 CASE WHEN issue_kind='cost_rule' THEN COALESCE(NULLIF((SELECT e.payload->>'upstream_model' FROM business_events e WHERE e.id=cost.event_id),''),model) ELSE '' END AS model,
 to_char(occurred_at AT TIME ZONE $3,'YYYY-MM') AS period,
 event_id,0::bigint AS source_id,COALESCE(amount_cny,0) AS amount,occurred_at,
 CASE WHEN issue_kind IN ('cost_binding','cost_other') THEN COALESCE(NULLIF((SELECT e.payload->>'account_type' FROM business_events e WHERE e.id=cost.event_id),''),'account') ELSE 'cost_pool' END AS object_type
 FROM cost
 UNION ALL
 SELECT 'funding_source',0,0,COALESCE(user_id,0),COALESCE(NULLIF(payload->>'user_name',''),'用户 #'||COALESCE(user_id,0)),
 '', '',0,id,0,occurred_at,CASE WHEN event_type='user_subscriptions' THEN 'user_subscription' ELSE 'user_balance' END FROM business_unresolved_sources
 UNION ALL
 SELECT 'funding_source',0,0,COALESCE(user_id,0),'用户 #'||COALESCE(user_id,0),'','',event_id,0,0,occurred_at,
 CASE WHEN (SELECT e.event_type FROM business_events e WHERE e.id=event_id)='user_subscriptions' THEN 'user_subscription' ELSE 'user_balance' END
 FROM business_ledger_entries WHERE kind='revenue_gap' AND occurred_at >= $1 AND occurred_at < $2
 UNION ALL
 SELECT 'fixed_cost',COALESCE(l.account_id,0),COALESCE((l.payload->'pool'->>'id')::bigint,0),0,
 COALESCE(NULLIF(l.payload->>'account_name',''),'账号 #'||l.account_id),'',to_char(l.occurred_at AT TIME ZONE $3,'YYYY-MM'),l.event_id,0,0,l.occurred_at,
 COALESCE(NULLIF((SELECT e.payload->>'account_type' FROM business_events e WHERE e.id=l.event_id),''),'account')
 FROM business_ledger_entries l WHERE l.kind='usage_weight' AND l.payload->>'requires_fixed_cost'='true'
 AND l.occurred_at >= $1 AND l.occurred_at < $2
 AND NOT EXISTS (SELECT 1 FROM business_projection_state s,LATERAL jsonb_array_elements(COALESCE(s.state->'expenses','[]'::jsonb)) x
 WHERE (x->'event'->'payload'->>'account_id')::bigint=l.account_id
 AND (x->>'starts_at')::timestamptz<=l.occurred_at AND (x->>'ends_at')::timestamptz>l.occurred_at)
), grouped AS (
 SELECT kind,account_id,pool_id,user_id,MAX(name) AS name,model,period,
 COUNT(DISTINCT event_id) FILTER(WHERE event_id>0) AS affected_count,
 COUNT(DISTINCT source_id) FILTER(WHERE source_id>0) AS source_count,
 SUM(amount)::text AS amount,MIN(occurred_at) AS first_at,MAX(occurred_at) AS last_at,ARRAY_AGG(DISTINCT object_type ORDER BY object_type) AS object_types
 FROM raw GROUP BY kind,account_id,pool_id,user_id,model,period
) `

func (s *BusinessLedgerService) Issues(ctx context.Context, start, end time.Time, offset int) (*BusinessIssueSummary, error) {
	if !end.After(start) || end.Sub(start) > 366*24*time.Hour || offset < 0 {
		return nil, fmt.Errorf("请选择不超过 366 天的有效区间")
	}
	s.Start()
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	out := &BusinessIssueSummary{Items: []BusinessIssue{}, Start: start, End: end}
	err = tx.QueryRowContext(ctx, `SELECT revision,updated_at,last_error<>'', (SELECT COUNT(*) FROM business_events e LEFT JOIN business_projection_processed p ON p.event_id=e.id WHERE p.event_id IS NULL) FROM business_projection_state WHERE id=1`).Scan(&out.Revision, &out.UpdatedAt, &out.CalculationError, &out.ProcessingCount)
	if err != nil {
		return nil, err
	}
	var configuredZone string
	if err = tx.QueryRowContext(ctx, `SELECT reporting_timezone FROM business_ledger_config WHERE id=1`).Scan(&configuredZone); err != nil {
		return nil, err
	}
	zone := businessReportingTimezone(configuredZone)
	location, err := time.LoadLocation(zone)
	if err != nil {
		return nil, err
	}
	args := []any{start, end, zone}
	if err = tx.QueryRowContext(ctx, businessIssuesSQL+`SELECT COUNT(*) FROM grouped`, args...).Scan(&out.Total); err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, businessIssuesSQL+`SELECT kind,account_id,pool_id,user_id,name,model,period,affected_count,source_count,amount,first_at,last_at,object_types FROM grouped ORDER BY kind,pool_id,account_id,user_id,model,period LIMIT 100 OFFSET $4`, append(args, offset)...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var v BusinessIssue
		if err = rows.Scan(&v.Kind, &v.AccountID, &v.PoolID, &v.UserID, &v.Name, &v.Model, &v.Period, &v.AffectedCount, &v.SourceCount, &v.Amount, &v.FirstAt, &v.LastAt, pq.Array(&v.ObjectTypes)); err != nil {
			_ = rows.Close()
			return nil, err
		}
		v.Key = fmt.Sprintf("%s:%d:%d:%d:%s:%s", v.Kind, v.AccountID, v.PoolID, v.UserID, v.Model, v.Period)
		if v.Period != "" {
			left, parseErr := time.ParseInLocation("2006-01", v.Period, location)
			if parseErr != nil {
				_ = rows.Close()
				return nil, parseErr
			}
			right := left.AddDate(0, 1, 0)
			v.PeriodStart = &left
			v.PeriodEnd = &right
		}
		out.Items = append(out.Items, v)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return nil, err
	}
	return out, tx.Commit()
}
