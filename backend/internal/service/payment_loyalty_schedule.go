package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"entgo.io/ent/dialect"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/robfig/cron/v3"
)

const paymentLoyaltyWeeklyResetSchedule = "0 0 * * 1"

var paymentLoyaltyCronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

// StartLoyaltyWeeklyResetScheduler clears weekly loyalty points at Monday 00:00
// in the configured server timezone. A startup cleanup covers downtime windows.
func (s *PaymentService) StartLoyaltyWeeklyResetScheduler() {
	if s == nil {
		return
	}
	s.loyaltyMu.Lock()
	defer s.loyaltyMu.Unlock()
	if s.loyaltyCron != nil || s.loyaltyStopped {
		return
	}

	loc := timezone.Location()
	c := cron.New(cron.WithParser(paymentLoyaltyCronParser), cron.WithLocation(loc))
	if _, err := c.AddFunc(paymentLoyaltyWeeklyResetSchedule, func() {
		s.runWeeklyLoyaltyPointsReset(context.Background(), timezone.Now())
	}); err != nil {
		slog.Error("[PaymentService] failed to schedule weekly loyalty points reset", "error", err)
		return
	}
	s.loyaltyCron = c
	c.Start()
	slog.Info("[PaymentService] scheduled weekly loyalty points reset", "schedule", paymentLoyaltyWeeklyResetSchedule, "timezone", loc.String())

	go s.runWeeklyLoyaltyPointsReset(context.Background(), timezone.Now())
}

func (s *PaymentService) StopLoyaltyWeeklyResetScheduler() {
	if s == nil {
		return
	}
	s.loyaltyMu.Lock()
	defer s.loyaltyMu.Unlock()
	s.loyaltyStopped = true
	if s.loyaltyCron == nil {
		return
	}
	ctx := s.loyaltyCron.Stop()
	select {
	case <-ctx.Done():
	case <-time.After(10 * time.Second):
		slog.Warn("[PaymentService] weekly loyalty points reset cron stop timed out")
	}
	s.loyaltyCron = nil
}

func (s *PaymentService) runWeeklyLoyaltyPointsReset(ctx context.Context, now time.Time) {
	updated, err := s.resetExpiredWeeklyLoyaltyPoints(ctx, now)
	if err != nil {
		slog.Error("[PaymentService] failed to reset weekly loyalty points", "error", err)
		return
	}
	if updated > 0 {
		slog.Info("[PaymentService] reset weekly loyalty points", "count", updated)
	}
}

func (s *PaymentService) resetExpiredWeeklyLoyaltyPoints(ctx context.Context, now time.Time) (int64, error) {
	if s == nil || s.entClient == nil {
		return 0, nil
	}
	defs, configured, err := s.ensureLoyaltyAttributeDefinitions(ctx)
	if err != nil {
		return 0, fmt.Errorf("ensure loyalty attribute definitions: %w", err)
	}
	if !configured || defs[LoyaltyWeeklyPointsAttributeKey] == nil {
		return 0, nil
	}
	weekStart := timezone.StartOfWeek(now)
	return s.resetWeeklyLoyaltyPointsBefore(ctx, defs[LoyaltyWeeklyPointsAttributeKey].ID, weekStart)
}

func (s *PaymentService) resetWeeklyLoyaltyPointsBefore(ctx context.Context, attributeID int64, weekStart time.Time) (int64, error) {
	client := paymentServiceClientFromContext(ctx, s.entClient)
	if client == nil {
		return 0, nil
	}
	query, args := buildResetWeeklyLoyaltyPointsQuery(client, attributeID, weekStart)
	result, err := client.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return 0, nil
	}
	return updated, nil
}

func buildResetWeeklyLoyaltyPointsQuery(client *dbent.Client, attributeID int64, weekStart time.Time) (string, []any) {
	if client.Driver() != nil && client.Driver().Dialect() == dialect.SQLite {
		return `
UPDATE user_attribute_values
SET value = '0', updated_at = CURRENT_TIMESTAMP
WHERE attribute_id = ?
  AND updated_at < ?
  AND COALESCE(NULLIF(value, ''), '0') != '0'`,
			[]any{attributeID, weekStart}
	}
	return `
UPDATE user_attribute_values
SET value = '0', updated_at = NOW()
WHERE attribute_id = $1
  AND updated_at < $2::timestamptz
  AND COALESCE(NULLIF(value, ''), '0') != '0'`,
		[]any{attributeID, weekStart}
}
