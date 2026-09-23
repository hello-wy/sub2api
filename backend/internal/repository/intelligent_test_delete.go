package repository

import (
	"context"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/lib/pq"
)

func (r *intelligentTestRepository) DeleteRecords(ctx context.Context, actor int64, ids []int64) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT id,status FROM account_tests WHERE id=ANY($1) ORDER BY id FOR UPDATE`, pq.Array(ids))
	if err != nil {
		return 0, err
	}
	removed := make([]int64, 0, len(ids))
	active := false
	for rows.Next() {
		var id int64
		var status string
		if err := rows.Scan(&id, &status); err != nil {
			rows.Close()
			return 0, err
		}
		active = active || status == "queued" || status == "running"
		removed = append(removed, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	if active {
		return 0, infraerrors.Conflict("INTELLIGENT_TEST_DELETE_ACTIVE", "排队或运行中的记录不能删除；排队任务请先取消，运行中的任务请等待结束")
	}
	if len(removed) == 0 {
		return 0, tx.Commit()
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM account_tests WHERE id=ANY($1)`, pq.Array(removed)); err != nil {
		return 0, err
	}
	// Retain request keys and their original record IDs. Replaying a request after
	// history deletion must never enqueue new, potentially billable model calls.
	if err = insertIntelligentAudit(ctx, tx, actor, "admin.intelligent_tests.delete", map[string]any{"record_ids": removed, "count": len(removed)}); err != nil {
		return 0, err
	}
	return int64(len(removed)), tx.Commit()
}
