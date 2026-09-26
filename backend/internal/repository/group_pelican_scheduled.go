package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *scheduledTestPlanRepository) ListByGroupID(ctx context.Context, groupID int64) ([]*service.ScheduledTestPlan, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, COALESCE(account_id, 0), model_id, cron_expression, enabled, max_results, auto_recover, last_run_at, next_run_at, created_at, updated_at, pelican_config, running_until, COALESCE(group_id, 0), COALESCE(api_key_id, 0), execution_state
        FROM scheduled_test_plans WHERE group_id = $1 ORDER BY created_at DESC, id DESC`, groupID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanPlans(rows)
}

func (r *scheduledTestPlanRepository) ListGroupTestKeys(ctx context.Context, groupID int64) ([]*service.GroupTestKey, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT k.id, k.name, u.email FROM api_keys k JOIN users u ON u.id = k.user_id
        WHERE k.group_id = $1 AND k.deleted_at IS NULL AND k.status = 'active'
        AND (k.expires_at IS NULL OR k.expires_at > NOW()) AND u.deleted_at IS NULL AND u.status = 'active'
        ORDER BY k.id DESC LIMIT 500`, groupID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	keys := make([]*service.GroupTestKey, 0)
	for rows.Next() {
		key := &service.GroupTestKey{}
		if err := rows.Scan(&key.ID, &key.Name, &key.UserEmail); err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, rows.Err()
}

func (r *scheduledTestPlanRepository) claimGroupPelican(ctx context.Context, tx *sql.Tx, plan *service.ScheduledTestPlan, now, until, next time.Time, immediate bool) (bool, error) {
	var locked bool
	if err := tx.QueryRowContext(ctx, `SELECT pg_try_advisory_xact_lock(hashtextextended('pelican-group:' || $1::text, 0))`, plan.GroupID).Scan(&locked); err != nil {
		return false, err
	}
	if !locked {
		return false, nil
	}
	result, err := tx.ExecContext(ctx, `UPDATE scheduled_test_plans
        SET running_until = $3, next_run_at = $4,
        execution_state = jsonb_build_object('status', 'running', 'phase', 'waiting', 'attempt', 1, 'max_attempts', 3,
          'total', (pelican_config->>'parallel_count')::int, 'completed', 0, 'succeeded', 0, 'failed', 0, 'started_at', $2::timestamptz)
        WHERE id = $1 AND enabled = true AND ($6 OR next_run_at <= $2)
        AND (running_until IS NULL OR running_until < $2) AND updated_at = $5
        AND EXISTS (SELECT 1 FROM groups WHERE groups.id = group_id AND deleted_at IS NULL AND status = 'active')
        AND NOT EXISTS (SELECT 1 FROM scheduled_test_plans other WHERE other.group_id = scheduled_test_plans.group_id
            AND other.id <> scheduled_test_plans.id AND other.running_until > $2)`, plan.ID, now, until, next, plan.UpdatedAt, immediate)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return n == 1, nil
}

// Lease fencing prevents an old worker from overwriting a newer execution.
func (r *scheduledTestPlanRepository) UpdatePelicanExecution(ctx context.Context, id int64, until time.Time, state *service.ScheduledTestExecution) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	result, err := r.db.ExecContext(ctx, `UPDATE scheduled_test_plans SET execution_state = CASE
 WHEN execution_state->>'cancel_requested' = 'true' THEN
   ($3::jsonb - 'phase' - 'retry_at') || jsonb_build_object('cancel_requested', true,
     'status', CASE WHEN $3::jsonb->>'finished_at' IS NOT NULL THEN 'interrupted' ELSE 'cancelling' END,
     'last_error', 'Test interrupted by administrator')
 ELSE $3::jsonb END
 WHERE id = $1 AND running_until = $2 AND running_until > NOW()`, id, until, string(data))
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("test execution lease expired or changed")
	}
	return nil
}

// Persist cancellation without releasing the lease while upstream requests are
// still unwinding. A progress write cannot remove this request.
func (r *scheduledTestPlanRepository) RequestPelicanCancellation(ctx context.Context, id int64, until time.Time) (bool, error) {
	result, err := r.db.ExecContext(ctx, `UPDATE scheduled_test_plans
 SET execution_state = (COALESCE(execution_state, '{}'::jsonb) - 'phase' - 'retry_at')
   || jsonb_build_object('cancel_requested', true, 'status', 'cancelling')
 WHERE id = $1 AND group_id IS NOT NULL AND running_until = $2 AND running_until > NOW()
   AND COALESCE(execution_state->>'status', 'running') IN ('running', 'retrying', 'cancelling')`, id, until)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	return n == 1, err
}
