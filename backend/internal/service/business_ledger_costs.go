package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/shopspring/decimal"
	"strings"
	"time"
)

type BusinessCostPool struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Supplier string `json:"supplier"`
	Unit     string `json:"unit"`
	Mode     string `json:"mode"`
}
type BusinessCostBinding struct {
	ID          int64     `json:"id"`
	AccountID   int64     `json:"account_id"`
	AccountName string    `json:"account_name"`
	PoolID      int64     `json:"pool_id"`
	EffectiveAt time.Time `json:"effective_at"`
}
type BusinessCostRule struct {
	ServiceTier       string          `json:"service_tier"`
	ImageSize         string          `json:"image_size"`
	VideoResolution   string          `json:"video_resolution"`
	CacheWrite1hPrice decimal.Decimal `json:"cache_write_1h_price"`
	ID                int64           `json:"id"`
	PoolID            int64           `json:"pool_id"`
	Model             string          `json:"model"`
	EffectiveAt       time.Time       `json:"effective_at"`
	Basis             string          `json:"basis"`
	UnitPrice         decimal.Decimal `json:"unit_price"`
	InputPrice        decimal.Decimal `json:"input_price"`
	OutputPrice       decimal.Decimal `json:"output_price"`
	CacheReadPrice    decimal.Decimal `json:"cache_read_price"`
	CacheWritePrice   decimal.Decimal `json:"cache_write_price"`
	CNYPerUnit        decimal.Decimal `json:"cny_per_unit"`
	Quality           string          `json:"quality"`
	Notes             string          `json:"notes"`
}
type BusinessCostConfiguration struct {
	Pools     []BusinessCostPool    `json:"pools"`
	Bindings  []BusinessCostBinding `json:"bindings"`
	Rules     []BusinessCostRule    `json:"rules"`
	EnabledAt time.Time             `json:"enabled_at"`
	Timezone  string                `json:"timezone"`
}

func (s *BusinessLedgerService) CostConfiguration(ctx context.Context) (*BusinessCostConfiguration, error) {
	result := &BusinessCostConfiguration{Pools: []BusinessCostPool{}, Bindings: []BusinessCostBinding{}, Rules: []BusinessCostRule{}}
	if err := s.db.QueryRowContext(ctx, `SELECT enabled_at,reporting_timezone FROM business_ledger_config WHERE id=1`).Scan(&result.EnabledAt, &result.Timezone); err != nil {
		return nil, err
	}
	result.Timezone = businessReportingTimezone(result.Timezone)
	rows, err := s.db.QueryContext(ctx, `SELECT id,name,supplier,unit,mode FROM business_cost_pools WHERE archived_at IS NULL ORDER BY id`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var v BusinessCostPool
		if err = rows.Scan(&v.ID, &v.Name, &v.Supplier, &v.Unit, &v.Mode); err != nil {
			_ = rows.Close()
			return nil, err
		}
		result.Pools = append(result.Pools, v)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return nil, err
	}
	rows, err = s.db.QueryContext(ctx, `SELECT b.id,b.account_id,COALESCE(a.name,''),b.pool_id,b.effective_at FROM business_cost_bindings b LEFT JOIN accounts a ON a.id=b.account_id ORDER BY b.effective_at DESC,b.id DESC`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var v BusinessCostBinding
		if err = rows.Scan(&v.ID, &v.AccountID, &v.AccountName, &v.PoolID, &v.EffectiveAt); err != nil {
			_ = rows.Close()
			return nil, err
		}
		result.Bindings = append(result.Bindings, v)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return nil, err
	}
	rows, err = s.db.QueryContext(ctx, `SELECT id,pool_id,model,effective_at,basis,unit_price,input_price,output_price,cache_read_price,cache_write_price,cny_per_unit,quality,notes,service_tier,image_size,video_resolution,cache_write_1h_price FROM business_cost_rules ORDER BY effective_at DESC,id DESC`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var v BusinessCostRule
		if err = rows.Scan(&v.ID, &v.PoolID, &v.Model, &v.EffectiveAt, &v.Basis, &v.UnitPrice, &v.InputPrice, &v.OutputPrice, &v.CacheReadPrice, &v.CacheWritePrice, &v.CNYPerUnit, &v.Quality, &v.Notes, &v.ServiceTier, &v.ImageSize, &v.VideoResolution, &v.CacheWrite1hPrice); err != nil {
			_ = rows.Close()
			return nil, err
		}
		result.Rules = append(result.Rules, v)
	}
	err = rows.Err()
	_ = rows.Close()
	return result, err
}
func (s *BusinessLedgerService) CreatePool(ctx context.Context, v BusinessCostPool) (*BusinessCostPool, error) {
	v.Name = strings.TrimSpace(v.Name)
	v.Unit = strings.TrimSpace(v.Unit)
	if v.Name == "" || len(v.Name) > 200 || v.Unit == "" || len(v.Unit) > 64 {
		return nil, fmt.Errorf("请输入成本池名称与上游计价单位")
	}
	if v.Mode != "prepaid" && v.Mode != "postpaid" && v.Mode != "fixed" {
		return nil, fmt.Errorf("无效的成本方式")
	}
	err := s.db.QueryRowContext(ctx, `INSERT INTO business_cost_pools(name,supplier,unit,mode)VALUES($1,$2,$3,$4)RETURNING id`, v.Name, v.Supplier, v.Unit, v.Mode).Scan(&v.ID)
	return &v, err
}
func (s *BusinessLedgerService) CreateBinding(ctx context.Context, v BusinessCostBinding) (*BusinessCostBinding, error) {
	if v.AccountID <= 0 || v.PoolID <= 0 || v.EffectiveAt.IsZero() {
		return nil, fmt.Errorf("请选择账号、成本池和生效时间")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(252254)`); err != nil {
		return nil, err
	}
	err = tx.QueryRowContext(ctx, `INSERT INTO business_cost_bindings(account_id,pool_id,effective_at)SELECT a.id,$2,$3 FROM accounts a WHERE a.id=$1 AND a.deleted_at IS NULL RETURNING id`, v.AccountID, v.PoolID, v.EffectiveAt).Scan(&v.ID)
	if err != nil {
		return nil, err
	}
	return &v, tx.Commit()
}
func (s *BusinessLedgerService) CreateRule(ctx context.Context, v BusinessCostRule) (*BusinessCostRule, error) {
	if v.PoolID <= 0 || v.EffectiveAt.IsZero() {
		return nil, fmt.Errorf("请选择成本池和生效时间")
	}
	if v.Model == "" {
		v.Model = "*"
	}
	if len(v.Model) > 200 {
		return nil, fmt.Errorf("模型名称过长")
	}
	switch v.Basis {
	case "account_stats", "tokens", "request", "image", "video_second":
	default:
		return nil, fmt.Errorf("无效的计价依据")
	}
	if strings.TrimSpace(v.Notes) == "" {
		return nil, fmt.Errorf("请输入价格凭据说明")
	}
	if v.Quality != "estimated" && v.Quality != "contract" {
		return nil, fmt.Errorf("请选择价格来源")
	}
	for _, price := range []decimal.Decimal{v.UnitPrice, v.InputPrice, v.OutputPrice, v.CacheReadPrice, v.CacheWritePrice, v.CacheWrite1hPrice, v.CNYPerUnit} {
		if price.IsNegative() || price.GreaterThan(decimal.New(1, 12)) {
			return nil, fmt.Errorf("单价必须是合理的非负金额")
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(252254)`); err != nil {
		return nil, err
	}
	err = tx.QueryRowContext(ctx, `INSERT INTO business_cost_rules(pool_id,model,effective_at,basis,unit_price,input_price,output_price,cache_read_price,cache_write_price,cny_per_unit,quality,notes,service_tier,image_size,video_resolution,cache_write_1h_price)VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)RETURNING id`, v.PoolID, v.Model, v.EffectiveAt, v.Basis, v.UnitPrice.String(), v.InputPrice.String(), v.OutputPrice.String(), v.CacheReadPrice.String(), v.CacheWritePrice.String(), v.CNYPerUnit.String(), v.Quality, v.Notes, v.ServiceTier, v.ImageSize, v.VideoResolution, v.CacheWrite1hPrice.String()).Scan(&v.ID)
	if err != nil {
		return nil, err
	}
	return &v, tx.Commit()
}

// Trace is intentionally independent of disposable usage_logs and entity FKs.
func (s *BusinessLedgerService) Trace(ctx context.Context, id int64) ([]BusinessEvent, error) {
	rows, err := s.db.QueryContext(ctx, `WITH roots AS (
 SELECT $1::bigint id
 UNION SELECT ids.value::bigint FROM business_ledger_entries l,
 LATERAL jsonb_array_elements_text(CASE WHEN jsonb_typeof(l.payload->'funding_event_ids')='array' THEN l.payload->'funding_event_ids' ELSE '[]'::jsonb END) ids(value) WHERE l.event_id=$1
 UNION SELECT ids.value::bigint FROM business_projection_state p,LATERAL jsonb_each(COALESCE(p.state->'terms','{}')) t,
 LATERAL jsonb_array_elements_text(CASE WHEN jsonb_typeof(t.value->'funding_event_ids')='array' THEN t.value->'funding_event_ids' ELSE '[]'::jsonb END) ids(value) WHERE t.value->>'event_id'=$1::text
 ) SELECT e.id,e.source_key,e.event_type,e.transaction_id,COALESCE(e.user_id,0),e.occurred_at,e.recorded_at,e.payload,COALESCE(e.actor_id,0),COALESCE(e.reverses_id,0)
 FROM business_events e WHERE e.transaction_id IN (SELECT transaction_id FROM business_events WHERE id IN(SELECT id FROM roots))
 OR e.payload->>'source_event_id' IN(SELECT id::text FROM roots) OR e.reverses_id IN(SELECT id FROM roots)
 OR (e.event_type='payment_orders' AND e.payload->>'id' IN(SELECT payload->'order'->>'id' FROM business_events WHERE id IN(SELECT id FROM roots)))
 ORDER BY e.id`, id)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanBusinessEvents(rows)
}

type BusinessBatchBinding struct {
	AccountIDs  []int64   `json:"account_ids"`
	PoolID      int64     `json:"pool_id"`
	EffectiveAt time.Time `json:"effective_at"`
}

func (s *BusinessLedgerService) CreateBindings(ctx context.Context, v BusinessBatchBinding) (int, error) {
	if len(v.AccountIDs) == 0 || len(v.AccountIDs) > 1000 || v.PoolID <= 0 || v.EffectiveAt.IsZero() {
		return 0, fmt.Errorf("请选择 1–1000 个账号、成本池和生效时间")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(252254)`); err != nil {
		return 0, err
	}
	seen := map[int64]bool{}
	for _, id := range v.AccountIDs {
		if id <= 0 {
			return 0, fmt.Errorf("账号编号无效")
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		var pool int64
		err = tx.QueryRowContext(ctx, `INSERT INTO business_cost_bindings(account_id,pool_id,effective_at) SELECT id,$2,$3 FROM accounts WHERE id=$1 AND deleted_at IS NULL
 ON CONFLICT(account_id,effective_at) DO NOTHING RETURNING pool_id`, id, v.PoolID, v.EffectiveAt).Scan(&pool)
		if errors.Is(err, sql.ErrNoRows) {
			err = tx.QueryRowContext(ctx, `SELECT pool_id FROM business_cost_bindings WHERE account_id=$1 AND effective_at=$2`, id, v.EffectiveAt).Scan(&pool)
		}
		if err != nil {
			return 0, fmt.Errorf("账号 #%d 不存在或无法绑定: %w", id, err)
		}
		if pool != v.PoolID {
			return 0, fmt.Errorf("账号 #%d 在该生效时间已有其他成本池绑定", id)
		}
	}
	return len(seen), tx.Commit()
}

// PostgreSQL requires an IANA name; Go's uninitialized "Local" is not one.
func businessReportingTimezone(configured string) string {
	if zone := timezone.Location().String(); zone != "Local" {
		return zone
	}
	if configured != "" && configured != "Local" {
		return configured
	}
	return "UTC"
}
