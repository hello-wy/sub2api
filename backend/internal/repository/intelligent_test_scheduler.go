package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

// Cooldowns are checked when a schedule becomes due and again by the runner.
// Permanent policy changes also prevent a queued scheduled test from starting.
const intelligentScheduledRunnableSQL = `NOT q.scheduled OR (
EXISTS(SELECT 1 FROM intelligent_test_schedules sc WHERE sc.test_type=q.test_type AND sc.enabled AND q.account_id=ANY(sc.account_ids))
AND EXISTS(SELECT 1 FROM users u WHERE u.id=q.requested_by AND u.role IN ('admin','super_admin') AND u.status='active' AND u.deleted_at IS NULL)
AND EXISTS(SELECT 1 FROM accounts a WHERE a.id=q.account_id AND a.deleted_at IS NULL AND a.status='active' AND a.schedulable
AND (NOT a.auto_pause_on_expired OR a.expires_at IS NULL OR a.expires_at>NOW())))`

func cancelInvalidScheduledIntelligentTests(ctx context.Context, tx *sql.Tx) error {
	// Bound maintenance work; Claim uses the same predicate so anything not yet
	// cancelled is still ineligible to run during this pass.
	_, err := tx.ExecContext(ctx, `WITH invalid AS (
SELECT q.id FROM account_tests q WHERE q.scheduled AND q.status='queued' AND NOT (`+intelligentScheduledRunnableSQL+`)
ORDER BY q.id LIMIT 100 FOR UPDATE SKIP LOCKED)
UPDATE account_tests t SET status='cancelled',finished_at=NOW(),error_message='定时测试已停用、账号已移除或执行授权已失效',queue_reason=''
FROM invalid WHERE t.id=invalid.id`)
	return err
}

func scanIntelligentSchedule(row intelligentScanner) (*service.IntelligentTestSchedule, error) {
	schedule := &service.IntelligentTestSchedule{AccountIDs: []int64{}}
	err := row.Scan(&schedule.Enabled, &schedule.IntervalMinutes, pq.Array(&schedule.AccountIDs), &schedule.NextRunAt, &schedule.LastRunAt, &schedule.LastError)
	return schedule, err
}

func updateIntelligentSchedule(ctx context.Context, tx *sql.Tx, actor int64, setting *service.IntelligentTestSetting) error {
	schedule := setting.Schedule
	if schedule != nil {
		if schedule.Enabled {
			var validActor int64
			err := tx.QueryRowContext(ctx, `SELECT id FROM users WHERE id=$1 AND role IN ('admin','super_admin') AND status='active' AND deleted_at IS NULL FOR SHARE`, actor).Scan(&validActor)
			if errors.Is(err, sql.ErrNoRows) {
				return service.ErrIntelligentTestForbidden
			}
			if err != nil {
				return err
			}
			var existing int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM accounts WHERE id=ANY($1) AND deleted_at IS NULL`, pq.Array(schedule.AccountIDs)).Scan(&existing); err != nil {
				return err
			}
			if existing != len(schedule.AccountIDs) {
				return infraerrors.BadRequest("INTELLIGENT_TEST_ACCOUNT_MISSING", "定时测试所选账号不存在或已删除")
			}
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO intelligent_test_schedules(test_type,enabled,interval_minutes,account_ids,requested_by,next_run_at)
VALUES($1,$2,$3,$4,$5,CASE WHEN $2 THEN NOW()+make_interval(mins=>$3) ELSE NULL END)
ON CONFLICT(test_type) DO UPDATE SET enabled=EXCLUDED.enabled,interval_minutes=EXCLUDED.interval_minutes,account_ids=EXCLUDED.account_ids,requested_by=EXCLUDED.requested_by,
next_run_at=CASE WHEN EXCLUDED.enabled THEN CASE WHEN intelligent_test_schedules.enabled
AND intelligent_test_schedules.interval_minutes=EXCLUDED.interval_minutes AND intelligent_test_schedules.account_ids=EXCLUDED.account_ids
THEN COALESCE(intelligent_test_schedules.next_run_at,EXCLUDED.next_run_at) ELSE EXCLUDED.next_run_at END ELSE NULL END,
last_error='',updated_at=NOW()`, setting.TestType, setting.Enabled && schedule.Enabled, schedule.IntervalMinutes, pq.Array(schedule.AccountIDs), actor)
		if err != nil {
			return err
		}
	} else if !setting.Enabled {
		if _, err := tx.ExecContext(ctx, `UPDATE intelligent_test_schedules SET enabled=false,next_run_at=NULL,updated_at=NOW() WHERE test_type=$1`, setting.TestType); err != nil {
			return err
		}
	}
	actual, err := scanIntelligentSchedule(tx.QueryRowContext(ctx, `SELECT enabled,interval_minutes,account_ids,next_run_at,last_run_at,last_error FROM intelligent_test_schedules WHERE test_type=$1`, setting.TestType))
	if errors.Is(err, sql.ErrNoRows) {
		actual = &service.IntelligentTestSchedule{IntervalMinutes: 60, AccountIDs: []int64{}}
	} else if err != nil {
		return err
	}
	setting.Schedule = actual
	return cancelInvalidScheduledIntelligentTests(ctx, tx)
}

func (r *intelligentTestRepository) EnqueueDueSchedules(ctx context.Context, limit int) (int, error) {
	if limit < 1 || limit > 10 {
		limit = 10
	}
	processed := 0
	for processed < limit {
		done, err := r.enqueueDueIntelligentSchedule(ctx)
		if err != nil || !done {
			return processed, err
		}
		processed++
	}
	return processed, nil
}

func (r *intelligentTestRepository) enqueueDueIntelligentSchedule(ctx context.Context) (bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	// Use the manual queue's lock before any setting row locks. A busy replica
	// can simply let the next tick handle this schedule.
	var acquired bool
	if err := tx.QueryRowContext(ctx, `SELECT pg_try_advisory_xact_lock(247000)`).Scan(&acquired); err != nil || !acquired {
		return false, err
	}
	var kind string
	var interval int
	var actor int64
	var selected []int64
	err = tx.QueryRowContext(ctx, `SELECT sc.test_type,sc.interval_minutes,sc.account_ids,sc.requested_by
FROM intelligent_test_schedules sc JOIN test_settings s ON s.test_type=sc.test_type
WHERE sc.enabled AND s.enabled AND sc.next_run_at<=NOW()
ORDER BY sc.next_run_at,sc.test_type LIMIT 1 FOR UPDATE OF sc SKIP LOCKED`).Scan(&kind, &interval, pq.Array(&selected), &actor)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var validActor int64
	err = tx.QueryRowContext(ctx, `SELECT id FROM users WHERE id=$1 AND role IN ('admin','super_admin') AND status='active' AND deleted_at IS NULL FOR SHARE`, actor).Scan(&validActor)
	if errors.Is(err, sql.ErrNoRows) {
		_, err = tx.ExecContext(ctx, `UPDATE intelligent_test_schedules SET enabled=false,next_run_at=NULL,last_run_at=NOW(),last_error='定时测试管理员已失效，请由有效管理员重新启用',updated_at=NOW() WHERE test_type=$1`, kind)
		if err != nil {
			return false, err
		}
		return true, tx.Commit()
	}
	if err != nil {
		return false, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT a.id FROM accounts a WHERE a.id=ANY($1)
AND a.deleted_at IS NULL AND a.status='active' AND a.schedulable
AND (NOT a.auto_pause_on_expired OR a.expires_at IS NULL OR a.expires_at>NOW())
AND (a.rate_limit_reset_at IS NULL OR a.rate_limit_reset_at<=NOW())
AND (a.overload_until IS NULL OR a.overload_until<=NOW())
AND (a.temp_unschedulable_until IS NULL OR a.temp_unschedulable_until<=NOW())
AND NOT EXISTS(SELECT 1 FROM account_tests q WHERE q.account_id=a.id AND q.test_type=$2 AND q.status IN ('queued','running'))
ORDER BY a.id LIMIT 100 FOR SHARE OF a`, pq.Array(selected), kind)
	if err != nil {
		return false, err
	}
	eligible := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return false, err
		}
		eligible = append(eligible, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return false, err
	}
	lastError := ""
	skipped := len(selected) - len(eligible)
	if skipped > 0 {
		lastError = fmt.Sprintf("本轮跳过 %d 个已有任务、停用或冷却中的账号", skipped)
	}
	var ids []int64
	if len(eligible) > 0 {
		if _, err := tx.ExecContext(ctx, `SAVEPOINT scheduled_enqueue`); err != nil {
			return false, err
		}
		out, err := r.enqueueIntelligentTestsTx(ctx, tx, actor, service.IntelligentTestEnqueue{
			AccountIDs: eligible, TestTypes: []string{kind}, IdempotencyKey: "schedule_" + uuid.NewString(),
		})
		if err != nil {
			if infraerrors.Reason(err) != "INTELLIGENT_TEST_QUEUE_FULL" {
				return false, err
			}
			if _, err := tx.ExecContext(ctx, `ROLLBACK TO SAVEPOINT scheduled_enqueue`); err != nil {
				return false, err
			}
			lastError = "测试队列已满，本轮已跳过，将在下一个定时周期重新检查"
		} else {
			for _, record := range out.Records {
				ids = append(ids, record.ID)
			}
			if _, err := tx.ExecContext(ctx, `UPDATE account_tests SET scheduled=true WHERE id=ANY($1)`, pq.Array(ids)); err != nil {
				return false, err
			}
		}
		if _, err := tx.ExecContext(ctx, `RELEASE SAVEPOINT scheduled_enqueue`); err != nil {
			return false, err
		}
	}
	// Advance from NOW(), never from the old deadline. Commit with enqueue so
	// other replicas/restarts cannot duplicate or replay this interval.
	if _, err := tx.ExecContext(ctx, `UPDATE intelligent_test_schedules SET next_run_at=NOW()+make_interval(mins=>$2),last_run_at=NOW(),last_error=$3,updated_at=NOW() WHERE test_type=$1`, kind, interval, lastError); err != nil {
		return false, err
	}
	if err := insertIntelligentAudit(ctx, tx, actor, "admin.intelligent_tests.schedule", map[string]any{
		"test_type": kind, "account_ids": selected, "record_ids": ids, "created_count": len(ids), "skipped_count": skipped, "reason": lastError,
	}); err != nil {
		return false, err
	}
	return true, tx.Commit()
}
