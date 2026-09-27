package service

import (
	"context"
	"fmt"
	"github.com/shopspring/decimal"
	"time"
)

// prepareBill fixes the list of covered cost entries and the booked comparison
// amount. A subsequent invoice/price edit cannot silently change this comparison.
func (s *BusinessLedgerService) prepareBill(ctx context.Context, input *BusinessRecordInput) error {
	if input.Type != "reconciliation" || input.Payload["bill_amount_cny"] == nil {
		return nil
	}
	if _, err := s.Project(ctx); err != nil {
		return fmt.Errorf("请先完成入账再核对账单: %w", err)
	}
	start := bTime(input.Payload, "starts_at", time.Time{})
	end := bTime(input.Payload, "ends_at", time.Time{})
	pool := bInt(input.Payload, "pool_id")
	amount, err := decimal.NewFromString(bString(input.Payload, "bill_amount_cny"))
	if err != nil || amount.IsNegative() || pool <= 0 || start.IsZero() || !end.After(start) || end.Sub(start) > 366*24*time.Hour {
		return fmt.Errorf("账单需要成本池、有效期间和非负人民币金额")
	}
	var pending int
	if err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM business_events e LEFT JOIN business_projection_processed p ON p.event_id=e.id WHERE p.event_id IS NULL`).Scan(&pending); err != nil {
		return err
	}
	if pending > 0 {
		return fmt.Errorf("还有待入账记录，请稍后核对账单")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT l.id,l.event_id,l.amount_cny::text FROM business_ledger_entries l JOIN business_events e ON e.id=l.event_id
 WHERE l.occurred_at>=$1 AND l.occurred_at<$2 AND l.kind IN ('usage_cost','cost_gap') AND e.event_type='usage'
 AND COALESCE(l.payload->>'pool_id',l.payload->'pool'->>'id')=$3::text ORDER BY l.id`, start, end, pool)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	ids := []string{}
	amounts := map[string]any{}
	eventIDs := []int64{}
	booked := decimal.Zero
	for rows.Next() {
		var id string
		var eventID int64
		var raw *string
		if err = rows.Scan(&id, &eventID, &raw); err != nil {
			return err
		}
		ids = append(ids, id)
		eventIDs = append(eventIDs, eventID)
		amounts[id] = raw
		if raw != nil {
			v, err := decimal.NewFromString(*raw)
			if err != nil {
				return err
			}
			booked = booked.Add(v)
		}
	}
	if err = rows.Err(); err != nil {
		return err
	}
	input.Payload["amount_cny"] = amount.Sub(booked).String()
	input.Payload["booked_amount_cny"] = booked.String()
	input.Payload["covered_entry_ids"] = ids
	input.Payload["covered_entry_amounts"] = amounts
	input.Payload["covered_event_ids"] = eventIDs
	input.Payload["entry_kind"] = "usage_cost"
	return nil
}
