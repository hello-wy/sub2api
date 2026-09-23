package service

import (
	"context"
	"log/slog"
	"sort"
	"time"
)

const (
	IntelligentTestScheduleMinMinutes  = 5
	IntelligentTestScheduleMaxMinutes  = 7 * 24 * 60
	IntelligentTestScheduleMaxAccounts = 100
)

func validateIntelligentTestSchedule(setting *IntelligentTestSetting) error {
	schedule := setting.Schedule
	if schedule == nil {
		return nil // Older clients can save other settings without changing a schedule.
	}
	schedule.NextRunAt, schedule.LastRunAt, schedule.LastError = nil, nil, ""
	if !setting.Enabled {
		schedule.Enabled = false
	}
	if !schedule.Enabled && schedule.IntervalMinutes == 0 {
		schedule.IntervalMinutes = 60
	}
	if schedule.IntervalMinutes < IntelligentTestScheduleMinMinutes || schedule.IntervalMinutes > IntelligentTestScheduleMaxMinutes {
		return intelligentTestBad("定时测试间隔必须为 5–10080 分钟")
	}
	if len(schedule.AccountIDs) > IntelligentTestScheduleMaxAccounts || (schedule.Enabled && len(schedule.AccountIDs) == 0) {
		return intelligentTestBad("启用定时测试需要明确选择 1–100 个账号")
	}
	seen := make(map[int64]bool, len(schedule.AccountIDs))
	for _, id := range schedule.AccountIDs {
		if id <= 0 || seen[id] {
			return intelligentTestBad("定时测试账号 ID 必须为正整数且不能重复")
		}
		seen[id] = true
	}
	// Keep comparison independent of the order of selections in the browser.
	schedule.AccountIDs = append([]int64{}, schedule.AccountIDs...)
	sort.Slice(schedule.AccountIDs, func(i, j int) bool { return schedule.AccountIDs[i] < schedule.AccountIDs[j] })
	return nil
}

func (s *IntelligentTestService) runScheduleLoop(ctx context.Context, repo IntelligentTestScheduleRepository) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		// The repository advances each due schedule from the current time, so a
		// restart performs at most one run instead of replaying missed intervals.
		if ctx.Err() != nil {
			return
		}
		batchCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		_, err := repo.EnqueueDueSchedules(batchCtx, 10)
		cancel()
		if err != nil && ctx.Err() == nil {
			slog.Error("intelligent test schedule enqueue failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
