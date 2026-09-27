package service

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/shopspring/decimal"
)

func (s *BusinessAnalyticsService) Ledger() *BusinessLedgerService {
	if s == nil {
		return nil
	}
	s.ledgerOnce.Do(func() { s.ledger = NewBusinessLedgerService(s.db) })
	return s.ledger
}

type BusinessLedgerService struct {
	db            *sql.DB
	once          sync.Once
	stop          chan struct{}
	balanceCache  *BillingCacheService
	stopOnce      sync.Once
	done          chan struct{}
	workerContext context.Context
	cancelWorker  context.CancelFunc
}

func NewBusinessLedgerService(db *sql.DB) *BusinessLedgerService {
	ctx, cancel := context.WithCancel(context.Background())
	return &BusinessLedgerService{db: db, stop: make(chan struct{}), done: make(chan struct{}), workerContext: ctx, cancelWorker: cancel}
}
func (s *BusinessLedgerService) SetBalanceCache(cache *BillingCacheService) { s.balanceCache = cache }

func (s *BusinessLedgerService) Start() {
	if s == nil || s.db == nil {
		return
	}
	s.once.Do(func() {
		go func() {
			defer close(s.done)
			ticker := time.NewTicker(15 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-s.stop:
					return
				case <-ticker.C:
					ctx, cancel := context.WithTimeout(s.workerContext, 12*time.Second)
					_, err := s.Project(ctx)
					cancel()
					if err != nil {
						ctx, cancel = context.WithTimeout(context.Background(), time.Second)
						_, _ = s.db.ExecContext(ctx, `UPDATE business_projection_state SET last_error=$1 WHERE id=1`, err.Error())
						cancel()
					}
				}
			}
		}()
	})
}
func (s *BusinessLedgerService) Stop() {
	if s == nil || s.db == nil {
		return
	}
	s.Start()
	s.stopOnce.Do(func() { close(s.stop); s.cancelWorker() })
	<-s.done
}

// Project selects unprocessed IDs, not id > cursor: sequence allocation is not
// commit order, and a late committing transaction must never be skipped.
func (s *BusinessLedgerService) Project(ctx context.Context) (int, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	var locked bool
	if err = tx.QueryRowContext(ctx, `SELECT pg_try_advisory_xact_lock(252252)`).Scan(&locked); err != nil {
		return 0, err
	}
	if !locked {
		return 0, nil
	}
	var raw []byte
	if err = tx.QueryRowContext(ctx, `SELECT state FROM business_projection_state WHERE id=1 FOR UPDATE`).Scan(&raw); err != nil {
		return 0, err
	}
	state := NewBusinessProjection()
	if len(raw) > 2 {
		if err = json.Unmarshal(raw, state); err != nil {
			return 0, err
		}
	}
	rows, err := tx.QueryContext(ctx, `WITH pending AS (
 SELECT e.transaction_id,MIN(e.id) AS first_id FROM business_events e
 LEFT JOIN business_projection_processed p ON p.event_id=e.id WHERE p.event_id IS NULL
 GROUP BY e.transaction_id ORDER BY MIN(e.id) LIMIT 500
 ) SELECT e.id,e.source_key,e.event_type,e.transaction_id,COALESCE(e.user_id,0),e.occurred_at,e.recorded_at,e.payload,COALESCE(e.actor_id,0),COALESCE(e.reverses_id,0)
 FROM business_events e JOIN pending ON pending.transaction_id=e.transaction_id
 LEFT JOIN business_projection_processed p ON p.event_id=e.id WHERE p.event_id IS NULL ORDER BY pending.first_id,e.id`)
	if err != nil {
		return 0, err
	}
	events, err := scanBusinessEvents(rows)
	_ = rows.Close()
	if err != nil {
		return 0, err
	}
	if len(events) == 0 {
		return 0, nil
	}
	// A valuation/annotation intentionally requests an auditable rebuild. Source
	// journal rows are immutable; only derived projections are regenerated.
	rebuild := false
	var lastProcessedID int64
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(event_id),0) FROM business_projection_processed`).Scan(&lastProcessedID); err != nil {
		return 0, err
	}
	for _, e := range events {
		// A late commit may precede a procurement already used by the
		// projection. Replay in business order to preserve moving-average cost.
		if e.ID < lastProcessedID || e.Type == "annotation" || e.Type == "reversal" || e.Type == "purchase" || e.Type == "opening_pool" {
			rebuild = true
		}
	}
	if rebuild {
		rows, err = tx.QueryContext(ctx, `SELECT id,source_key,event_type,transaction_id,COALESCE(user_id,0),occurred_at,recorded_at,payload,COALESCE(actor_id,0),COALESCE(reverses_id,0) FROM business_events ORDER BY id`)
		if err != nil {
			return 0, err
		}
		events, err = scanBusinessEvents(rows)
		_ = rows.Close()
		if err != nil {
			return 0, err
		}
		state = NewBusinessProjection()
		if _, err = tx.ExecContext(ctx, `DELETE FROM business_ledger_entries; DELETE FROM business_projection_processed`); err != nil {
			return 0, err
		}
	}
	projected := businessApplyAnnotations(events)
	groups := map[int64][]BusinessEvent{}
	order := []int64{}
	for _, e := range projected {
		if _, ok := groups[e.TransactionID]; !ok {
			order = append(order, e.TransactionID)
		}
		groups[e.TransactionID] = append(groups[e.TransactionID], e)
	}
	// Wallet row locking establishes the order for simultaneous user changes.
	sort.SliceStable(order, func(i, j int) bool {
		a, b := groups[order[i]], groups[order[j]]
		at, bt := businessTransactionTime(a), businessTransactionTime(b)
		if !at.Equal(bt) {
			return at.Before(bt)
		}
		return businessTransactionOrder(a) < businessTransactionOrder(b)
	})
	for _, id := range order {
		state.ApplyTransaction(groups[id])
	}
	for _, entry := range state.Entries {
		detail, _ := json.Marshal(entry.Detail)
		var amount any
		if entry.Amount != nil {
			amount = entry.Amount.String()
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO business_ledger_entries(id,event_id,occurred_at,kind,user_id,account_id,group_id,model,plan_id,amount_cny,credits,quality,payload) VALUES($1,$2,$3,$4,NULLIF($5,0),NULLIF($6,0),NULLIF($7,0),$8,NULLIF($9,0),$10,$11,$12,$13) ON CONFLICT(id) DO NOTHING`, entry.ID, entry.EventID, entry.At, entry.Kind, entry.UserID, entry.AccountID, entry.GroupID, entry.Model, entry.PlanID, amount, entry.Credits.String(), entry.Quality, string(detail))
		if err != nil {
			return 0, err
		}
	}
	for _, e := range events {
		if _, err = tx.ExecContext(ctx, `INSERT INTO business_projection_processed(event_id)VALUES($1) ON CONFLICT DO NOTHING`, e.ID); err != nil {
			return 0, err
		}
	}
	encoded, err := json.Marshal(state)
	if err != nil {
		return 0, err
	}
	_, err = tx.ExecContext(ctx, `UPDATE business_projection_state SET state=$1,revision=revision+1,event_count=(SELECT COUNT(*) FROM business_projection_processed),updated_at=clock_timestamp(),last_error='' WHERE id=1`, string(encoded))
	if err != nil {
		return 0, err
	}
	return len(events), tx.Commit()
}

func businessTransactionTime(events []BusinessEvent) time.Time {
	for _, e := range events {
		if e.Type == "wallet" || e.Type == "signup_wallet" {
			return e.OccurredAt
		}
	}
	return events[0].OccurredAt
}

func businessTransactionOrder(events []BusinessEvent) int64 {
	for _, e := range events {
		if e.Type == "wallet" || e.Type == "signup_wallet" {
			return e.ID
		}
	}
	return events[0].ID
}
func businessApplyAnnotations(events []BusinessEvent) []BusinessEvent {
	annotations := map[int64]map[string]any{}
	reversed := map[int64]bool{}
	orders := map[int64]map[string]any{}
	for _, e := range events {
		if e.Type == "annotation" {
			target := bInt(e.Payload, "source_event_id")
			if annotations[target] == nil {
				annotations[target] = map[string]any{}
			}
			for k, v := range bMap(e.Payload, "fields") {
				annotations[target][k] = v
			}
		}
		if e.Type == "reversal" {
			reversed[e.ReversesID] = true
		}
	}
	for _, e := range events {
		if e.Type == "payment_orders" && annotations[e.ID] != nil {
			orders[bInt(e.Payload, "id")] = annotations[e.ID]
		}
	}
	result := make([]BusinessEvent, 0, len(events))
	for _, e := range events {
		if reversed[e.ID] {
			continue
		}
		if e.Type == "annotation" || e.Type == "reversal" {
			continue
		}
		if fields := annotations[e.ID]; fields != nil {
			for k, v := range fields {
				e.Payload[k] = v
			}
			e.Payload["resolved"] = true
		}
		if order := bMap(e.Payload, "order"); order != nil {
			for k, v := range orders[bInt(order, "id")] {
				order[k] = v
			}
		}
		if e.Type == "payment_orders" {
			for k, v := range orders[bInt(e.Payload, "id")] {
				e.Payload[k] = v
			}
		}
		result = append(result, e)
	}
	return result
}
func scanBusinessEvents(rows *sql.Rows) ([]BusinessEvent, error) {
	result := []BusinessEvent{}
	for rows.Next() {
		var e BusinessEvent
		var raw []byte
		if err := rows.Scan(&e.ID, &e.SourceKey, &e.Type, &e.TransactionID, &e.UserID, &e.OccurredAt, &e.RecordedAt, &raw, &e.ActorID, &e.ReversesID); err != nil {
			return nil, err
		}
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		if err := decoder.Decode(&e.Payload); err != nil {
			return nil, err
		}
		result = append(result, e)
	}
	return result, rows.Err()
}

func (s *BusinessLedgerService) Records(ctx context.Context, before int64, limit int) ([]BusinessEvent, error) {
	if limit < 1 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,source_key,event_type,transaction_id,COALESCE(user_id,0),occurred_at,recorded_at,payload,COALESCE(actor_id,0),COALESCE(reverses_id,0) FROM business_events WHERE ($1=0 OR id<$1) AND event_type IN ('receipt','purchase','expense','opening_pool','annotation','reconciliation','adjustment','reversal','supplier_refund','supplier_loss','payment_orders','expense_stop') ORDER BY id DESC LIMIT $2`, before, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanBusinessEvents(rows)
}

func (s *BusinessLedgerService) Record(ctx context.Context, input BusinessRecordInput, actor int64) (*BusinessEvent, error) {
	input.At = input.At.Truncate(time.Microsecond)
	payload := map[string]any{}
	for k, v := range input.Payload {
		payload[k] = v
	}
	input.Payload = payload
	if err := s.prepareBill(ctx, &input); err != nil {
		return nil, err
	}
	if err := validateBusinessRecord(input); err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	// Replay-safe writes: a retry cannot credit a user twice, and a reused key
	// with a different payload is rejected instead of silently changing a receipt.
	key := "manual:" + input.IdempotencyKey
	for _, entity := range []struct{ Key, Name, Table string }{{"account_id", "account_name", "accounts"}, {"group_id", "group_name", "groups"}, {"plan_id", "plan_name", "subscription_plans"}, {"pool_id", "pool_name", "business_cost_pools"}} {
		delete(input.Payload, entity.Name)
		if id := bInt(input.Payload, entity.Key); id > 0 {
			var name string
			if err = tx.QueryRowContext(ctx, "SELECT name FROM "+entity.Table+" WHERE id=$1", id).Scan(&name); err != nil {
				return nil, fmt.Errorf("关联对象不存在: %s", entity.Key)
			}
			input.Payload[entity.Name] = name
		}
	}
	if input.UserID > 0 {
		var name string
		if err = tx.QueryRowContext(ctx, "SELECT username FROM users WHERE id=$1", input.UserID).Scan(&name); err != nil {
			if !errors.Is(err, sql.ErrNoRows) || (input.Type != "annotation" && input.Type != "reversal") {
				return nil, fmt.Errorf("关联用户不存在")
			}
			name = fmt.Sprintf("已删除用户 #%d", input.UserID)
		}
		input.Payload["user_name"] = name
	}
	raw, err := json.Marshal(input.Payload)
	if err != nil {
		return nil, err
	}
	var id int64
	err = tx.QueryRowContext(ctx, `INSERT INTO business_events(source_key,event_type,user_id,occurred_at,payload,actor_id,reverses_id)VALUES($1,$2,NULLIF($3,0),$4,$5,$6,NULLIF($7,0))ON CONFLICT(source_key)DO NOTHING RETURNING id`, key, input.Type, input.UserID, input.At, string(raw), actor, bInt(input.Payload, "reverses_id")).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		var storedType string
		var storedPayload []byte
		var storedUser int64
		var storedAt time.Time
		err = tx.QueryRowContext(ctx, `SELECT id,event_type,payload,COALESCE(user_id,0),occurred_at FROM business_events WHERE source_key=$1`, key).Scan(&id, &storedType, &storedPayload, &storedUser, &storedAt)
		if err != nil {
			return nil, err
		}
		var stored map[string]any
		_ = json.Unmarshal(storedPayload, &stored)
		var incoming map[string]any
		_ = json.Unmarshal(raw, &incoming)
		for _, k := range []string{"account_name", "group_name", "plan_name", "pool_name", "user_name"} {
			delete(stored, k)
			delete(incoming, k)
		}
		if input.Type == "reconciliation" && input.Payload["bill_amount_cny"] != nil {
			// Server-derived coverage is not part of the administrator's request.
			for _, k := range []string{"amount_cny", "booked_amount_cny", "covered_entry_ids", "covered_entry_amounts", "covered_event_ids"} {
				delete(stored, k)
				delete(incoming, k)
			}
		}
		a, _ := json.Marshal(stored)
		b, _ := json.Marshal(incoming)
		if storedType != input.Type || storedUser != input.UserID || !storedAt.Equal(input.At) || !bytes.Equal(a, b) {
			return nil, fmt.Errorf("幂等标识已用于其他记录")
		}
	} else if err != nil {
		return nil, err
	} else {
		if input.Type == "receipt" && bBool(input.Payload, "apply_balance") {
			result, err := tx.ExecContext(ctx, `UPDATE users SET balance=balance+$1,updated_at=NOW() WHERE id=$2 AND deleted_at IS NULL`, bDecimal(input.Payload, "credits").String(), input.UserID)
			if err != nil {
				return nil, err
			}
			count, _ := result.RowsAffected()
			if count != 1 {
				return nil, fmt.Errorf("用户不存在")
			}
		}
		if input.Type == "reconciliation" && input.Payload["bill_amount_cny"] != nil {
			poolID := bInt(input.Payload, "pool_id")
			if _, err = tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(252253,$1::integer)", poolID); err != nil {
				return nil, err
			}
			var overlap bool
			err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM business_events e WHERE e.event_type='reconciliation' AND e.id<>$1 AND e.payload->>'pool_id'=$2::text
              AND e.payload ? 'bill_amount_cny' AND (e.payload->>'starts_at')::timestamptz<$4 AND (e.payload->>'ends_at')::timestamptz>$3
              AND NOT EXISTS(SELECT 1 FROM business_events r WHERE r.event_type='reversal' AND r.reverses_id=e.id))`, id, poolID, bTime(input.Payload, "starts_at", input.At), bTime(input.Payload, "ends_at", input.At)).Scan(&overlap)
			if err != nil {
				return nil, err
			}
			if overlap {
				return nil, fmt.Errorf("该成本池已有重叠期间账单，请先冲销原核对记录")
			}
		}
		if poolID := bInt(input.Payload, "pool_id"); poolID > 0 {
			var exists bool
			if err = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM business_cost_pools WHERE id=$1)", poolID).Scan(&exists); err != nil {
				return nil, err
			}
			if !exists {
				return nil, fmt.Errorf("成本池不存在")
			}
		}
		if input.Type == "annotation" {
			var sourceType string
			var payload []byte
			if err = tx.QueryRowContext(ctx, `SELECT event_type,payload FROM business_events WHERE id=$1`, bInt(input.Payload, "source_event_id")).Scan(&sourceType, &payload); err != nil {
				return nil, fmt.Errorf("原始事件不存在")
			}
			var source map[string]any
			_ = json.Unmarshal(payload, &source)
			fields := bMap(input.Payload, "fields")
			switch sourceType {
			case "opening_unknown", "wallet", "user_subscriptions", "usage", "payment_orders":
			default:
				return nil, fmt.Errorf("该凭据不能修改来源估值，请冲销后重新登记")
			}
			if sourceType == "wallet" && bDecimal(source, "after").LessThan(bDecimal(source, "before")) && bString(fields, "classification") != "usage" {
				return nil, fmt.Errorf("余额减少只能补录为消费；退款请使用退款流程")
			}
			if sourceType == "opening_unknown" {
				if fields["paid_credits"] == nil || fields["gift_credits"] == nil || fields["unknown_credits"] == nil || fields["amount_cny"] == nil {
					return nil, fmt.Errorf("期初补录必须包含全部来源组成与人民币价值")
				}
				total := bDecimal(fields, "paid_credits").Add(bDecimal(fields, "gift_credits")).Add(bDecimal(fields, "unknown_credits"))
				if !total.Equal(bDecimal(source, "credits")) {
					return nil, fmt.Errorf("付费、赠送与未知额度之和必须等于期初额度")
				}
			}
		}
		if input.Type == "expense_stop" {
			target := bInt(input.Payload, "source_event_id")
			var typ string
			var data []byte
			if err = tx.QueryRowContext(ctx, "SELECT event_type,payload FROM business_events WHERE id=$1 FOR UPDATE", target).Scan(&typ, &data); err != nil {
				return nil, fmt.Errorf("原费用不存在")
			}
			var original map[string]any
			_ = json.Unmarshal(data, &original)
			if typ != "expense" || bBool(original, "cash_only") || !bTime(original, "ends_at", input.At).After(input.At) {
				return nil, fmt.Errorf("只能终止尚未摊销完成的服务期费用")
			}
			var stopped bool
			if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM business_events e WHERE e.event_type='expense_stop' AND e.payload->>'source_event_id'=$1::text AND e.id<>$2 AND NOT EXISTS(SELECT 1 FROM business_events r WHERE r.reverses_id=e.id))`, target, id).Scan(&stopped); err != nil {
				return nil, err
			}
			if stopped {
				return nil, fmt.Errorf("该费用已终止")
			}
		}
		if input.Type == "reversal" {
			var typ string
			var targetPayload []byte
			if err = tx.QueryRowContext(ctx, `SELECT event_type,payload FROM business_events WHERE id=$1 FOR UPDATE`, bInt(input.Payload, "reverses_id")).Scan(&typ, &targetPayload); err != nil {
				return nil, fmt.Errorf("原始记录不存在")
			}
			var target map[string]any
			_ = json.Unmarshal(targetPayload, &target)
			if typ != "purchase" && typ != "expense" && typ != "reconciliation" && typ != "adjustment" && typ != "expense_stop" {
				return nil, fmt.Errorf("该记录需通过退款或资金来源更正，不能直接作废")
			}
			var exists bool
			if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM business_events WHERE event_type='reversal' AND reverses_id=$1 AND id<>$2)`, bInt(input.Payload, "reverses_id"), id).Scan(&exists); err != nil {
				return nil, err
			}
			if exists {
				return nil, fmt.Errorf("记录已经冲销")
			}
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	if input.Type == "receipt" && bBool(input.Payload, "apply_balance") && s.balanceCache != nil {
		cacheCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_ = s.balanceCache.InvalidateUserBalance(cacheCtx, input.UserID)
		cancel()
	}
	s.Start()
	return &BusinessEvent{ID: id, SourceKey: key, Type: input.Type, UserID: input.UserID, OccurredAt: input.At, Payload: input.Payload, ActorID: actor}, nil
}

var businessDecimalPattern = regexp.MustCompile(`^-?[0-9]+(?:\.[0-9]{1,8})?$`)

func validateBusinessRecord(input BusinessRecordInput) error {
	if len(input.IdempotencyKey) < 8 || len(input.IdempotencyKey) > 128 {
		return fmt.Errorf("需要 8 至 128 字符的幂等标识")
	}
	if input.At.IsZero() || input.At.After(time.Now().Add(time.Minute)) {
		return fmt.Errorf("请输入有效的发生时间")
	}
	if input.Payload == nil {
		return fmt.Errorf("记录内容不能为空")
	}
	switch input.Type {
	case "receipt", "purchase", "expense", "opening_pool", "reconciliation", "adjustment", "supplier_refund", "supplier_loss":
		amount, ok := bMoney(input.Payload)
		if !ok || input.Payload["amount_cny"] == nil {
			return fmt.Errorf("必须录入实际人民币金额")
		}
		if amount.IsNegative() && input.Type != "adjustment" && input.Type != "reconciliation" {
			return fmt.Errorf("金额不能为负数")
		}
		if len(bString(input.Payload, "notes")) == 0 {
			return fmt.Errorf("请输入凭据或说明")
		}
		if input.Type == "purchase" || input.Type == "opening_pool" || input.Type == "supplier_loss" || input.Type == "supplier_refund" {
			if bInt(input.Payload, "pool_id") <= 0 || !bDecimal(input.Payload, "credits").IsPositive() {
				return fmt.Errorf("请选择成本池并填写正数上游额度")
			}
		}
		if input.Type == "receipt" && bBool(input.Payload, "apply_balance") {
			if !amount.IsPositive() {
				return fmt.Errorf("付费充值需要正数实际收款，赠送请使用赠送入口")
			}
			if input.UserID <= 0 || !bDecimal(input.Payload, "credits").IsPositive() {
				return fmt.Errorf("余额充值需要用户与正数到账额度")
			}
		}
		if input.Type == "expense" {
			a := bTime(input.Payload, "starts_at", input.At)
			b := bTime(input.Payload, "ends_at", a)
			if b.Before(a) {
				return fmt.Errorf("成本结束时间必须晚于开始时间")
			}
		}
		if kind := bString(input.Payload, "entry_kind"); kind != "" && kind != "usage_cost" && kind != "operating_cost" && kind != "refund_revenue" {
			return fmt.Errorf("不支持的调整类别")
		}
	case "annotation":
		if bInt(input.Payload, "source_event_id") <= 0 || len(bMap(input.Payload, "fields")) == 0 || bString(input.Payload, "notes") == "" {
			return fmt.Errorf("补录需要原始事件、字段与凭据说明")
		}
		allowed := map[string]bool{"amount_cny": true, "fx_rate": true, "paid_credits": true, "gift_credits": true, "unknown_credits": true, "classification": true, "gift_share": true, "actual_supplier_cost_cny": true}
		for k, v := range bMap(input.Payload, "fields") {
			if !allowed[k] {
				return fmt.Errorf("不可更改原始字段 %s", k)
			}
			if k == "classification" && v != "paid" && v != "gift" && v != "usage" {
				return fmt.Errorf("来源分类无效")
			}
			if k == "gift_share" && bDecimal(bMap(input.Payload, "fields"), k).GreaterThan(decimal.NewFromInt(1)) {
				return fmt.Errorf("赠送比例不能大于 1")
			}
			if k != "classification" {
				if len(fmt.Sprint(v)) > 32 || !businessDecimalPattern.MatchString(fmt.Sprint(v)) {
					return fmt.Errorf("%s 必须为最多 8 位小数的金额", k)
				}
				value, err := decimal.NewFromString(fmt.Sprint(v))
				if err != nil || value.IsNegative() {
					return fmt.Errorf("%s 必须是非负十进制数", k)
				}
			}
		}
	case "expense_stop":
		if bInt(input.Payload, "source_event_id") <= 0 || bString(input.Payload, "notes") == "" {
			return fmt.Errorf("终止费用需要原始凭据和原因")
		}
	case "reversal":
		if bInt(input.Payload, "reverses_id") <= 0 || bString(input.Payload, "notes") == "" {
			return fmt.Errorf("冲销需要原记录与原因")
		}
	default:
		return fmt.Errorf("不支持的台账类型")
	}
	for _, key := range []string{"starts_at", "ends_at"} {
		if input.Payload[key] != nil {
			if _, err := time.Parse(time.RFC3339Nano, bString(input.Payload, key)); err != nil {
				return fmt.Errorf("%s 必须是带时区的有效时间", key)
			}
		}
	}
	if len(bString(input.Payload, "notes")) > 4000 {
		return fmt.Errorf("凭据说明过长")
	}
	for _, key := range []string{"amount_cny", "credits", "fx_rate", "bill_amount_cny", "original_amount"} {
		if v, exists := input.Payload[key]; exists {
			if len(fmt.Sprint(v)) > 32 || !businessDecimalPattern.MatchString(fmt.Sprint(v)) {
				return fmt.Errorf("%s 必须为最多 8 位小数的金额", key)
			}
			d, err := decimal.NewFromString(fmt.Sprint(v))
			if err != nil || d.Abs().GreaterThan(decimal.New(1, 15)) {
				return fmt.Errorf("%s 不是有效金额", key)
			}
		}
	}
	return nil
}

type BusinessLookup struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"`
}

func (s *BusinessLedgerService) Lookups(ctx context.Context, kind, q string) ([]BusinessLookup, error) {
	sources := map[string]string{"accounts": "SELECT id,name,type AS kind FROM accounts WHERE deleted_at IS NULL", "groups": "SELECT id,name,subscription_type AS kind FROM groups WHERE deleted_at IS NULL", "users": "SELECT id,username || ' · ' || email AS name,role AS kind FROM users WHERE deleted_at IS NULL", "plans": "SELECT id,name,'plan' AS kind FROM subscription_plans"}
	query, ok := sources[kind]
	if !ok {
		return nil, fmt.Errorf("无效的查询类型")
	}
	rows, err := s.db.QueryContext(ctx, "SELECT id,name,kind FROM ("+query+") t WHERE ($1='' OR name ILIKE '%'||$1||'%' OR id::text=$1) ORDER BY id DESC LIMIT 100", strings.TrimSpace(q))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	result := []BusinessLookup{}
	for rows.Next() {
		var v BusinessLookup
		if err = rows.Scan(&v.ID, &v.Name, &v.Kind); err != nil {
			return nil, err
		}
		result = append(result, v)
	}
	return result, rows.Err()
}
