package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

// A bounded, single batch writer keeps telemetry off the request's critical
// path. Backpressure drops telemetry with an explicit diagnostic; it never
// stalls a user response or invents a successful outcome.
func (s *GroupStatusService) outcomeLoop() {
	defer close(s.outcomeDone)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	batch := make([]GroupRequestOutcome, 0, 100)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		err := s.writeOutcomeBatch(ctx, batch)
		cancel()
		if err != nil {
			logger.LegacyPrintf("group_status", "completion batch unavailable: %v", err)
		}
		batch = batch[:0]
	}
	for {
		select {
		case outcome, ok := <-s.outcomes:
			if !ok {
				flush()
				return
			}
			batch = append(batch, outcome)
			if len(batch) >= 100 {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}
func (s *GroupStatusService) writeOutcomeBatch(ctx context.Context, batch []GroupRequestOutcome) error {
	if len(batch) == 0 {
		return nil
	}
	// Coalesce duplicate completion writes before INSERT ... ON CONFLICT: one
	// statement cannot update the same PostgreSQL row twice.
	type key struct {
		id          string
		user, group int64
	}
	unique := map[key]GroupRequestOutcome{}
	for _, o := range batch {
		unique[key{o.RequestID, o.UserID, o.GroupID}] = o
	}
	args := make([]any, 0, len(unique)*9)
	values := make([]string, 0, len(unique))
	for _, o := range unique {
		n := len(args)
		values = append(values, fmt.Sprintf("($%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d)", n+1, n+2, n+3, n+4, n+5, n+6, n+7, n+8, n+9))
		args = append(args, o.RequestID, o.GroupID, o.UserID, o.Platform, o.Model, o.Success, o.ErrorCategory, o.CompletedAt, o.GatewayRequestID)
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO channel_monitor_request_outcomes(request_id,group_id,user_id,platform,model,success,error_category,completed_at,gateway_request_id) VALUES `+strings.Join(values, ",")+` ON CONFLICT(request_id,user_id,group_id) DO UPDATE SET success=EXCLUDED.success,error_category=EXCLUDED.error_category,completed_at=EXCLUDED.completed_at,gateway_request_id=EXCLUDED.gateway_request_id`, args...)
	return err
}
