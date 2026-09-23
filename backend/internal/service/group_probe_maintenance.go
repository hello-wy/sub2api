package service

import (
	"context"
	"database/sql"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

const groupMaintenanceBatchLimit = 32

func (s *GroupStatusService) probeMaintenance() {
	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()
	queries := []string{
		`UPDATE group_probe_runs SET status='failed',error_code='worker_interrupted',finished_at=now() WHERE id IN (SELECT id FROM group_probe_runs WHERE status='running' AND lease_until<now() LIMIT 1000)`,
		`DELETE FROM group_probe_runs WHERE id IN (SELECT id FROM group_probe_runs WHERE status<>'running' AND checked_at<now()-INTERVAL '90 days' LIMIT 1000)`,
		`DELETE FROM channel_monitor_request_outcomes WHERE (request_id,user_id,group_id) IN (SELECT request_id,user_id,group_id FROM channel_monitor_request_outcomes WHERE completed_at<now()-INTERVAL '90 days' LIMIT 1000)`,
	}
	// Round-robin bounded batches prevent one large backlog from starving
	// lease recovery or the other retention table. The deadline also bounds
	// database work on a loaded instance.
	done := make([]bool, len(queries))
	for batch := 0; batch < groupMaintenanceBatchLimit; batch++ {
		allDone := true
		for i, query := range queries {
			if done[i] {
				continue
			}
			result, err := s.db.ExecContext(ctx, query)
			if err != nil {
				logger.LegacyPrintf("group_status", "status maintenance interrupted batch=%d table=%d: %v", batch, i, err)
				return
			}
			n, err := result.RowsAffected()
			if err != nil {
				logger.LegacyPrintf("group_status", "status maintenance count unavailable: %v", err)
				return
			}
			done[i] = n < 1000
			allDone = allDone && done[i]
		}
		if allDone {
			return
		}
	}
	var oldest sql.NullTime
	if err := s.db.QueryRowContext(ctx, `SELECT MIN(completed_at) FROM channel_monitor_request_outcomes`).Scan(&oldest); err == nil && oldest.Valid && time.Since(oldest.Time) > 90*24*time.Hour {
		logger.LegacyPrintf("group_status", "completion retention backlog remains after %d batches; oldest=%s", groupMaintenanceBatchLimit, oldest.Time.UTC().Format(time.RFC3339))
	}
}
