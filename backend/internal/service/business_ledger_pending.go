package service

import "context"

// Kept for compatibility. Manual source work is never a list of API calls.
func (s *BusinessLedgerService) Pending(ctx context.Context, before int64) ([]BusinessEvent, error) {
	return s.PendingSources(ctx, 0, before)
}
func (s *BusinessLedgerService) PendingSources(ctx context.Context, userID, before int64) ([]BusinessEvent, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,source_key,event_type,transaction_id,COALESCE(user_id,0),occurred_at,recorded_at,payload,COALESCE(actor_id,0),COALESCE(reverses_id,0)
 FROM business_unresolved_sources WHERE ($1=0 OR id<$1) AND ($2=0 OR user_id=$2) ORDER BY id DESC LIMIT 100`, before, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanBusinessEvents(rows)
}
