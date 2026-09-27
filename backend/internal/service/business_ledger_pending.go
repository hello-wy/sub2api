package service

import "context"

// Pending stays available when aggregation fails. It reads source evidence and
// the last successful projection, without trying to compute a new overview.
func (s *BusinessLedgerService) Pending(ctx context.Context, before int64) ([]BusinessEvent, error) {
	rows, err := s.db.QueryContext(ctx, `
 SELECT e.id,e.source_key,e.event_type,e.transaction_id,COALESCE(e.user_id,0),e.occurred_at,e.recorded_at,e.payload,COALESCE(e.actor_id,0),COALESCE(e.reverses_id,0)
 FROM business_events e
 WHERE ($1=0 OR e.id<$1) AND e.event_type IN ('opening_unknown','wallet','usage','user_subscriptions','payment_orders')
 AND (
   e.event_type='opening_unknown'
   OR EXISTS(SELECT 1 FROM business_ledger_entries l WHERE l.event_id=e.id AND l.kind IN ('funding_unknown','wallet_adjustment','cost_gap','cash_gap','revenue_gap'))
   OR EXISTS(SELECT 1 FROM business_projection_state p,LATERAL jsonb_each(COALESCE(p.state->'terms','{}')) term WHERE (term.value->>'event_id')=e.id::text AND term.value->>'quality'='unknown')
 )
 AND NOT EXISTS(SELECT 1 FROM business_events a WHERE a.event_type='annotation' AND a.payload->>'source_event_id'=e.id::text
   AND COALESCE((a.payload->'fields'->>'unknown_credits')::numeric,0)=0)
 ORDER BY e.id DESC LIMIT 100`, before)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanBusinessEvents(rows)
}
