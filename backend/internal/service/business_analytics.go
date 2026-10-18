package service

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"strings"
	"sync"
	"time"

	"github.com/shopspring/decimal"
)

// BusinessAnalyticsService turns existing usage, payment, balance, subscription,
// and account-pool data into an explicitly labelled operational estimate. API key
// accounts contribute variable cost using their score cost and CNY conversion;
// OAuth and other fixed-cost accounts contribute only through the manual ledger.
type BusinessAnalyticsService struct {
	db         *sql.DB
	ledgerOnce sync.Once
	ledger     *BusinessLedgerService
}

func NewBusinessAnalyticsService(db *sql.DB) *BusinessAnalyticsService {
	return &BusinessAnalyticsService{db: db}
}

type OperatingValidationError struct{ Message string }

func (e *OperatingValidationError) Error() string { return e.Message }
func operatingInvalid(format string, args ...any) error {
	return &OperatingValidationError{Message: fmt.Sprintf(format, args...)}
}

type BusinessSettings struct {
	EnabledAt             *time.Time      `json:"enabled_at"`
	Timezone              string          `json:"timezone"`
	OpeningRevenue        decimal.Decimal `json:"opening_revenue"`
	OpeningPrivateRevenue decimal.Decimal `json:"opening_private_revenue"`
	OpeningCost           decimal.Decimal `json:"opening_cost"`
}
type BusinessRecord struct {
	ID         int64           `json:"id"`
	SourceKey  string          `json:"source_key"`
	Kind       string          `json:"kind"`
	Amount     decimal.Decimal `json:"amount"`
	Date       string          `json:"date"`
	Category   string          `json:"category"`
	Notes      string          `json:"notes"`
	ReversesID *int64          `json:"reverses_id"`
	Reversed   bool            `json:"reversed"`
	CreatedAt  time.Time       `json:"created_at"`
}
type BusinessRecordInput struct {
	SourceKey string          `json:"source_key"`
	Kind      string          `json:"kind"`
	Amount    decimal.Decimal `json:"amount"`
	Date      string          `json:"date"`
	Category  string          `json:"category"`
	Notes     string          `json:"notes"`
}
type BusinessDay struct {
	Date                string           `json:"date"`
	BalanceRevenue      decimal.Decimal  `json:"balance_revenue"`
	SubscriptionRevenue decimal.Decimal  `json:"subscription_revenue"`
	OtherRevenue        decimal.Decimal  `json:"other_revenue"`
	ManualCost          decimal.Decimal  `json:"manual_cost"`
	GiftCost            decimal.Decimal  `json:"gift_cost"`
	Revenue             decimal.Decimal  `json:"revenue"`
	Cost                decimal.Decimal  `json:"cost"`
	KnownProfit         decimal.Decimal  `json:"known_profit"`
	Profit              *decimal.Decimal `json:"profit"`
	Missing             int64            `json:"missing"`
}

func (d *BusinessDay) finish() {
	d.Revenue = d.BalanceRevenue.Add(d.SubscriptionRevenue).Add(d.OtherRevenue)
	d.Cost = d.ManualCost.Add(d.GiftCost)
	d.KnownProfit = d.Revenue.Sub(d.Cost)
	d.Profit = nil
	if d.Missing == 0 {
		n := d.KnownProfit
		d.Profit = &n
	}
}
func (d *BusinessDay) add(v BusinessDay) {
	d.BalanceRevenue = d.BalanceRevenue.Add(v.BalanceRevenue)
	d.SubscriptionRevenue = d.SubscriptionRevenue.Add(v.SubscriptionRevenue)
	d.OtherRevenue = d.OtherRevenue.Add(v.OtherRevenue)
	d.ManualCost = d.ManualCost.Add(v.ManualCost)
	d.GiftCost = d.GiftCost.Add(v.GiftCost)
	d.Missing += v.Missing
}

// OperatingConsumptionValue is capped per purchased term, never per calendar window.
func OperatingConsumptionValue(amount, quota, used decimal.Decimal) decimal.Decimal {
	if !quota.IsPositive() || !amount.IsPositive() || !used.IsPositive() {
		return decimal.Zero
	}
	return amount.Mul(decimal.Min(used, quota)).Div(quota).Round(8)
}
func OperatingConsumptionDelta(amount, quota, baseline, previous, used decimal.Decimal) decimal.Decimal {
	return OperatingConsumptionValue(amount, quota, baseline.Add(previous).Add(used)).Sub(OperatingConsumptionValue(amount, quota, baseline.Add(previous)))
}

type businessQuery interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func readBusinessSettings(ctx context.Context, q businessQuery) (BusinessSettings, error) {
	var c BusinessSettings
	err := q.QueryRowContext(ctx, "SELECT enabled_at,timezone,opening_revenue,opening_private_revenue,opening_cost FROM operating_settings WHERE id=1").Scan(&c.EnabledAt, &c.Timezone, &c.OpeningRevenue, &c.OpeningPrivateRevenue, &c.OpeningCost)
	return c, err
}
func (s *BusinessAnalyticsService) Settings(ctx context.Context) (BusinessSettings, error) {
	return readBusinessSettings(ctx, s.db)
}
func validOperatingMoney(v decimal.Decimal, negative bool) bool {
	return (negative || !v.IsNegative()) && v.Abs().LessThan(decimal.NewFromInt(1_000_000_000_000)) && v.Equal(v.Round(8))
}
func (s *BusinessAnalyticsService) Initialize(ctx context.Context, in BusinessSettings, actor int64) (BusinessSettings, error) {
	if !validOperatingMoney(in.OpeningRevenue, false) || !validOperatingMoney(in.OpeningPrivateRevenue, false) || !validOperatingMoney(in.OpeningCost, false) {
		return BusinessSettings{}, operatingInvalid("期初金额需非负、小于一万亿元，最多 8 位小数")
	}
	if _, err := time.LoadLocation(in.Timezone); err != nil || strings.TrimSpace(in.Timezone) == "" {
		return BusinessSettings{}, operatingInvalid("请选择有效报表时区")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return BusinessSettings{}, err
	}
	defer func() { _ = tx.Rollback() }()
	// Lock in billing order before choosing the cutoff. In-flight settlements cannot straddle the baseline.
	if _, err = tx.ExecContext(ctx, "LOCK TABLE users, user_subscriptions IN SHARE ROW EXCLUSIVE MODE"); err != nil {
		return BusinessSettings{}, err
	}
	var enabled *time.Time
	if err = tx.QueryRowContext(ctx, "SELECT enabled_at FROM operating_settings WHERE id=1 FOR UPDATE").Scan(&enabled); err != nil {
		return BusinessSettings{}, err
	}
	if enabled != nil {
		return BusinessSettings{}, operatingInvalid("期初已启用，不能覆盖历史基线；请使用调整记录更正")
	}
	_, err = tx.ExecContext(ctx, `UPDATE operating_settings SET enabled_at=clock_timestamp(),timezone=$1,opening_revenue=$2,opening_private_revenue=$3,opening_cost=$4,actor_id=$5,quota_timezone=$6 WHERE id=1`, in.Timezone, in.OpeningRevenue, in.OpeningPrivateRevenue, in.OpeningCost, actor, timezone.Location().String())
	if err != nil {
		return BusinessSettings{}, err
	}
	if _, err = tx.ExecContext(ctx, "SELECT operating_snapshot_term(id,true) FROM user_subscriptions WHERE deleted_at IS NULL AND expires_at>clock_timestamp()"); err != nil {
		return BusinessSettings{}, err
	}
	c, err := readBusinessSettings(ctx, tx)
	if err != nil {
		return c, err
	}
	err = tx.Commit()
	return c, err
}
func (s *BusinessAnalyticsService) Record(ctx context.Context, in BusinessRecordInput, actor int64) (BusinessRecord, error) {
	var out BusinessRecord
	in.SourceKey = strings.TrimSpace(in.SourceKey)
	in.Category = strings.TrimSpace(in.Category)
	in.Notes = strings.TrimSpace(in.Notes)
	if len(in.SourceKey) < 8 || len(in.SourceKey) > 100 || strings.HasPrefix(in.SourceKey, "reverse:") {
		return out, operatingInvalid("有效的唯一凭据编号为必填")
	}
	signed := in.Kind == "income_adjustment" || in.Kind == "cost_adjustment"
	if in.Kind != "cost" && in.Kind != "income" && in.Kind != "receipt" && !signed {
		return out, operatingInvalid("不支持的记账类型")
	}
	if !validOperatingMoney(in.Amount, signed) || in.Amount.IsZero() {
		return out, operatingInvalid("金额需非零、最多 8 位小数；普通收入和成本必须为正")
	}
	if len(in.Notes) > 2000 || len(in.Category) > 100 || in.Category == "" {
		return out, operatingInvalid("请填写类别，备注不能超过 2000 字节")
	}
	c, err := s.Settings(ctx)
	if err != nil {
		return out, err
	}
	if c.EnabledAt == nil {
		return out, operatingInvalid("请先设置期初")
	}
	zone, _ := time.LoadLocation(c.Timezone)
	date, err := time.ParseInLocation("2006-01-02", in.Date, zone)
	if err != nil || date.Format("2006-01-02") < c.EnabledAt.In(zone).Format("2006-01-02") || date.After(time.Now().In(zone)) {
		return out, operatingInvalid("日期必须在启用日和今天之间")
	}
	// The conflict branch never changes the previous record. Compare its payload after reading it.
	err = s.db.QueryRowContext(ctx, `INSERT INTO operating_records(source_key,kind,amount,occurred_on,category,notes,actor_id) VALUES($1,$2,$3,$4,$5,$6,$7)
 ON CONFLICT(source_key) DO UPDATE SET source_key=EXCLUDED.source_key
 RETURNING id,source_key,kind,amount,occurred_on::text,category,notes,reverses_id,created_at`, in.SourceKey, in.Kind, in.Amount, in.Date, in.Category, in.Notes, actor).Scan(&out.ID, &out.SourceKey, &out.Kind, &out.Amount, &out.Date, &out.Category, &out.Notes, &out.ReversesID, &out.CreatedAt)
	if err == nil && (out.Kind != in.Kind || !out.Amount.Equal(in.Amount) || out.Date != in.Date || out.Category != in.Category || out.Notes != in.Notes) {
		return out, operatingInvalid("同一凭据编号不能登记不同内容")
	}
	return out, err
}
func (s *BusinessAnalyticsService) Records(ctx context.Context, before int64) ([]BusinessRecord, error) {
	result := []BusinessRecord{}
	rows, err := s.db.QueryContext(ctx, `SELECT r.id,r.source_key,r.kind,r.amount,r.occurred_on::text,r.category,r.notes,r.reverses_id,r.created_at,EXISTS(SELECT 1 FROM operating_records x WHERE x.reverses_id=r.id)
 FROM operating_records r WHERE ($1=0 OR r.id<$1) ORDER BY r.id DESC LIMIT 50`, before)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var r BusinessRecord
		if err = rows.Scan(&r.ID, &r.SourceKey, &r.Kind, &r.Amount, &r.Date, &r.Category, &r.Notes, &r.ReversesID, &r.CreatedAt, &r.Reversed); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}
func (s *BusinessAnalyticsService) ReverseRecord(ctx context.Context, id, actor int64, reason string) error {
	reason = strings.TrimSpace(reason)
	if reason == "" || len(reason) > 2000 {
		return operatingInvalid("请填写冲销原因，最多 2000 字节")
	}
	result, err := s.db.ExecContext(ctx, `INSERT INTO operating_records(source_key,kind,amount,occurred_on,category,notes,actor_id,reverses_id)
 SELECT 'reverse:'||id,kind,-amount,occurred_on,category,$3,$2,id FROM operating_records WHERE id=$1 AND reverses_id IS NULL
 ON CONFLICT(reverses_id) DO NOTHING`, id, actor, reason)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		var exists bool
		if err = s.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM operating_records WHERE reverses_id=$1)", id).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return sql.ErrNoRows
		}
	}
	return nil
}

type BusinessTerm struct {
	ID        int64            `json:"id"`
	UserID    int64            `json:"user_id"`
	GroupName string           `json:"group_name"`
	PlanName  string           `json:"plan_name"`
	Funding   string           `json:"funding"`
	Amount    *decimal.Decimal `json:"amount"`
	Quota     *decimal.Decimal `json:"quota"`
	Baseline  decimal.Decimal  `json:"baseline_units"`
	Used      decimal.Decimal  `json:"used_units"`
	StartsAt  time.Time        `json:"starts_at"`
	EndsAt    time.Time        `json:"ends_at"`
	Opening   bool             `json:"opening"`
}
type BusinessTermInput struct {
	Amount  decimal.Decimal `json:"amount"`
	Quota   decimal.Decimal `json:"quota"`
	Funding string          `json:"funding"`
	Notes   string          `json:"notes"`
}

func (s *BusinessAnalyticsService) Terms(ctx context.Context, before int64) ([]BusinessTerm, error) {
	result := []BusinessTerm{}
	rows, err := s.db.QueryContext(ctx, `SELECT t.id,t.user_id,t.group_name,t.plan_name,t.funding,t.amount,t.quota,t.baseline_units,
 t.baseline_units+COALESCE((SELECT SUM(subscription_units) FROM operating_daily_usage d WHERE d.term_id=t.id),0),t.starts_at,t.ends_at,t.opening
 FROM operating_terms t WHERE ($1=0 OR t.id<$1) AND (t.amount IS NULL OR t.quota IS NULL OR t.funding='unknown') ORDER BY t.id DESC LIMIT 50`, before)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var t BusinessTerm
		if err = rows.Scan(&t.ID, &t.UserID, &t.GroupName, &t.PlanName, &t.Funding, &t.Amount, &t.Quota, &t.Baseline, &t.Used, &t.StartsAt, &t.EndsAt, &t.Opening); err != nil {
			return nil, err
		}
		result = append(result, t)
	}
	return result, rows.Err()
}
func (s *BusinessAnalyticsService) ConfigureTerm(ctx context.Context, id, actor int64, in BusinessTermInput) error {
	if !validOperatingMoney(in.Amount, false) || !validOperatingMoney(in.Quota, false) || !in.Quota.IsPositive() || (in.Funding != "gift" && in.Funding != "paid") || strings.TrimSpace(in.Notes) == "" || len(in.Notes) > 2000 {
		return operatingInvalid("请填写有效计价金额、正的本期总额度、资金类型及依据")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var amount, quota *decimal.Decimal
	var funding string
	err = tx.QueryRowContext(ctx, "SELECT amount,quota,funding FROM operating_terms WHERE id=$1 FOR UPDATE", id).Scan(&amount, &quota, &funding)
	if err != nil {
		return err
	}
	if amount != nil && quota != nil && funding != "unknown" {
		return operatingInvalid("已确定的权益期换算基准不可覆盖")
	}
	if amount != nil && !amount.Equal(in.Amount) {
		return operatingInvalid("已冻结的成交金额不能被覆盖")
	}
	if quota != nil && !quota.Equal(in.Quota) {
		return operatingInvalid("已冻结的额度不能被覆盖")
	}
	if funding == "paid" && in.Funding != "paid" {
		return operatingInvalid("支付订单不能改为赠送")
	}
	_, err = tx.ExecContext(ctx, "UPDATE operating_terms SET amount=$2,quota=$3,funding=$4,notes=$5,configured_by=$6 WHERE id=$1", id, in.Amount, in.Quota, in.Funding, in.Notes, actor)
	if err != nil {
		return err
	}
	return tx.Commit()
}
