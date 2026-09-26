package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *scheduledTestPlanRepository) ListByGroupID(ctx context.Context, groupID int64) ([]*service.ScheduledTestPlan, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, COALESCE(account_id, 0), model_id, cron_expression, enabled, max_results, auto_recover, last_run_at, next_run_at, created_at, updated_at, pelican_config, running_until, COALESCE(group_id, 0), COALESCE(api_key_id, 0)
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

func (r *scheduledTestPlanRepository) Trigger(ctx context.Context, id int64, now time.Time) (bool, error) {
	result, err := r.db.ExecContext(ctx, `UPDATE scheduled_test_plans SET next_run_at = $2, updated_at = NOW()
        WHERE id = $1 AND group_id IS NOT NULL AND enabled = true AND (running_until IS NULL OR running_until < $2)`, id, now)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	return n == 1, err
}

func (r *scheduledTestPlanRepository) claimGroupPelican(ctx context.Context, tx *sql.Tx, plan *service.ScheduledTestPlan, now, until, next time.Time) (bool, error) {
	var locked bool
	if err := tx.QueryRowContext(ctx, `SELECT pg_try_advisory_xact_lock(hashtextextended('pelican-group:' || $1::text, 0))`, plan.GroupID).Scan(&locked); err != nil {
		return false, err
	}
	if !locked {
		return false, nil
	}
	result, err := tx.ExecContext(ctx, `UPDATE scheduled_test_plans
        SET running_until = $3, next_run_at = $4
        WHERE id = $1 AND enabled = true AND next_run_at <= $2
        AND (running_until IS NULL OR running_until < $2) AND updated_at = $5
        AND EXISTS (SELECT 1 FROM groups WHERE groups.id = group_id AND deleted_at IS NULL AND status = 'active')
        AND NOT EXISTS (SELECT 1 FROM scheduled_test_plans other WHERE other.group_id = scheduled_test_plans.group_id
            AND other.id <> scheduled_test_plans.id AND other.running_until > $2)`, plan.ID, now, until, next, plan.UpdatedAt)
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
