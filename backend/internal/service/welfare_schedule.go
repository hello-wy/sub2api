package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/robfig/cron/v3"
)

const DefaultWelfareRewardTime = "23:55"

func ValidateWelfareRewardTime(value string) (string, error) {
	parsed, err := time.Parse("15:04", value)
	if err != nil || parsed.Format("15:04") != value {
		return "", fmt.Errorf("welfare_reward_time must use HH:mm (00:00-23:59)")
	}
	return value, nil
}

// Start checks the editable daily settlement time every minute.
func (s *WelfareService) Start() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started || s.stopped {
		return
	}
	s.started = true

	schedule := "* * * * *"
	loc := timezone.Location()
	c := newWelfareCron(loc)
	if _, err := c.AddFunc(schedule, func() { s.runScheduledRewardJob(context.Background()) }); err != nil {
		slog.Error("[WelfareService] failed to schedule cron job", "error", err)
		return
	}
	c.Start()
	s.cron = c
	slog.Info("[WelfareService] scheduled reward job successfully", "schedule", schedule, "timezone", loc.String())
}

func (s *WelfareService) runScheduledRewardJob(ctx context.Context) {
	due, err := s.scheduledRewardDue(ctx, timezone.Now())
	if err != nil {
		slog.Error("[WelfareService] failed to read reward time", "error", err)
		return
	}
	if due {
		s.RunRewardJob(ctx)
	}
}

func (s *WelfareService) scheduledRewardDue(ctx context.Context, now time.Time) (bool, error) {
	raw, err := s.settingRepo.GetValue(ctx, SettingKeyWelfareRewardTime)
	if errors.Is(err, ErrSettingNotFound) {
		raw = DefaultWelfareRewardTime
	} else if err != nil {
		return false, err
	}
	if raw == "" {
		raw = DefaultWelfareRewardTime
	}
	setting, err := ValidateWelfareRewardTime(raw)
	if err != nil {
		return false, err
	}
	return now.In(timezone.Location()).Format("15:04") == setting, nil
}

func newWelfareCron(loc *time.Location) *cron.Cron {
	return cron.New(cron.WithParser(welfareCronParser), cron.WithLocation(loc))
}

// Stop stops the cron job.
func (s *WelfareService) Stop() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return
	}
	s.stopped = true
	if s.cron == nil {
		return
	}
	ctx := s.cron.Stop()
	select {
	case <-ctx.Done():
	case <-time.After(welfareCronStopTimeout):
		slog.Warn("[WelfareService] cron stop timed out")
	}
	s.cron = nil
}

func (s *WelfareService) tryAcquireLeaderLock(ctx context.Context) (func(), bool) {
	if s == nil {
		return nil, false
	}
	if s.cfg != nil && s.cfg.RunMode == config.RunModeSimple {
		return nil, true
	}
	if s.lockCache == nil {
		return tryAcquireDBAdvisoryLock(ctx, s.db, welfareAdvisoryLockID)
	}
	release, ok, err := s.tryAcquireCacheLeaderLock(ctx)
	if err == nil {
		return release, ok
	}
	s.warnNoRedisOnce.Do(func() {
		logger.LegacyPrintf("service.welfare", "[WelfareService] leader lock failed; falling back to DB advisory lock: %v", err)
	})
	return tryAcquireDBAdvisoryLock(ctx, s.db, welfareAdvisoryLockID)
}

func (s *WelfareService) tryAcquireCacheLeaderLock(ctx context.Context) (func(), bool, error) {
	ok, err := s.lockCache.TryAcquireLeaderLock(ctx, welfareLeaderLockKey, s.instanceID, welfareLeaderLockTTL)
	if err != nil || !ok {
		return nil, ok, err
	}
	return func() {
		_ = s.lockCache.ReleaseLeaderLock(context.Background(), welfareLeaderLockKey, s.instanceID)
	}, true, nil
}
