package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/shopspring/decimal"
)

type dailyRewardCandidate struct {
	userID      int64
	email       string
	consumption decimal.Decimal
	rank        int
	amount      float64
}

func (s *WelfareService) settleDailyRewards(ctx context.Context, day, end time.Time, ranking []usagestats.UserSpendingRankingItem, ratios []float64) (int, error) {
	enabled, rules, err := s.loadTicketRebateSettings(ctx)
	if err != nil {
		return 0, err
	}
	candidates, err := s.dailyRewardCandidates(ctx, day, end, ranking, ratios)
	if err != nil {
		return 0, err
	}
	count := 0
	failed := 0
	var firstError error
	for _, candidate := range candidates {
		if err := s.settleDailyReward(ctx, day, candidate, enabled, rules); err != nil {
			slog.Error("[WelfareService] user settlement failed", "user_id", candidate.userID, "date", day.Format(time.DateOnly), "error", err)
			failed++
			if firstError == nil {
				firstError = err
			}
			continue
		}
		count++
	}
	if failed > 0 {
		return count, fmt.Errorf("%d daily settlements failed: %w", failed, firstError)
	}
	return count, nil
}

func (s *WelfareService) loadTicketRebateSettings(ctx context.Context) (bool, []TicketRebateRule, error) {
	enabled, err := s.settingRepo.GetValue(ctx, SettingKeyTicketRebateEnabled)
	if errors.Is(err, ErrSettingNotFound) {
		return false, nil, nil
	}
	if err != nil {
		return false, nil, fmt.Errorf("load ticket rebate switch: %w", err)
	}
	if enabled == "false" {
		return false, nil, nil
	}
	if enabled != "true" {
		return false, nil, fmt.Errorf("invalid ticket rebate switch: %q", enabled)
	}
	raw, err := s.settingRepo.GetValue(ctx, SettingKeyTicketRebateRules)
	if err != nil && !errors.Is(err, ErrSettingNotFound) {
		return false, nil, fmt.Errorf("load ticket rebate rules: %w", err)
	}
	rules, err := parseTicketRebateRules(raw)
	return true, rules, err
}

func (s *WelfareService) dailyRewardCandidates(ctx context.Context, day, end time.Time, ranking []usagestats.UserSpendingRankingItem, ratios []float64) ([]dailyRewardCandidate, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT u.user_id, us.email, SUM(u.actual_cost)::text
FROM usage_logs u JOIN users us ON us.id = u.user_id
WHERE u.created_at >= $1 AND u.created_at < $2 AND us.role <> 'admin'
GROUP BY u.user_id, us.email HAVING SUM(u.actual_cost) > 0`, day, end)
	if err != nil {
		return nil, fmt.Errorf("load daily consumption: %w", err)
	}
	defer func() { _ = rows.Close() }()
	byUser := make(map[int64]dailyRewardCandidate, len(ranking))
	for i, item := range ranking {
		if i < len(ratios) && ratios[i] > 0 {
			byUser[item.UserID] = dailyRewardCandidate{rank: i + 1, amount: item.ActualCost * ratios[i]}
		}
	}
	var candidates []dailyRewardCandidate
	for rows.Next() {
		var userID int64
		var email, raw string
		if err := rows.Scan(&userID, &email, &raw); err != nil {
			return nil, err
		}
		consumption, err := decimal.NewFromString(raw)
		if err != nil {
			return nil, err
		}
		candidate := byUser[userID]
		candidate.userID, candidate.email, candidate.consumption = userID, email, consumption
		candidates = append(candidates, candidate)
	}
	return candidates, rows.Err()
}

func (s *WelfareService) settleDailyReward(ctx context.Context, day time.Time, candidate dailyRewardCandidate, enabled bool, rules []TicketRebateRule) error {
	count, matched := 0, []TicketRebateRule{}
	if enabled {
		count, matched = CalculateTicketRebate(candidate.consumption, rules)
	}
	snapshot, err := json.Marshal(matched)
	if err != nil {
		return err
	}
	err = s.settleDailyRewardTx(ctx, day, candidate, count, snapshot)
	if err != nil {
		s.recordFailedDailySettlement(ctx, day, candidate, count, snapshot)
	}
	return err
}

func (s *WelfareService) settleDailyRewardTx(ctx context.Context, day time.Time, candidate dailyRewardCandidate, count int, snapshot []byte) error {
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()
	legacyAmount, alreadyPaid, err := existingDailyAmountRebate(txCtx, client, day, candidate.userID)
	if err != nil {
		return err
	}
	if alreadyPaid {
		candidate.amount = legacyAmount
	}
	settlementID, claimed, err := claimDailySettlement(txCtx, client, day, candidate, count, snapshot)
	if err != nil || !claimed {
		return err
	}
	if err := creditDailyRebateTickets(txCtx, client, day, candidate.userID, count); err != nil {
		return err
	}
	if !alreadyPaid {
		if err := s.creditDailyAmountRebate(txCtx, day, candidate); err != nil {
			return err
		}
	}
	if _, err := client.ExecContext(txCtx, `UPDATE daily_ticket_rebates SET status = 'success', updated_at = NOW() WHERE id = $1`, settlementID); err != nil {
		return err
	}
	return tx.Commit()
}

func existingDailyAmountRebate(ctx context.Context, client *dbent.Client, day time.Time, userID int64) (float64, bool, error) {
	var amount float64
	var count int
	err := scanOne(ctx, client, `SELECT COALESCE(SUM(amount), 0), COUNT(*) FROM welfare_records
WHERE user_id = $1 AND status = 'success' AND remarks LIKE $2`,
		[]any{userID, day.Format(time.DateOnly) + " 消费 $%"}, &amount, &count)
	return amount, count > 0, err
}

func claimDailySettlement(ctx context.Context, client *dbent.Client, day time.Time, candidate dailyRewardCandidate, count int, snapshot []byte) (int64, bool, error) {
	var settlementID int64
	err := scanOne(ctx, client, `INSERT INTO daily_ticket_rebates
(user_id, settlement_date, daily_consumption, ticket_count, matched_rules, amount_rebate, status)
VALUES ($1, $2, $3, $4, $5, $6, 'pending')
ON CONFLICT (user_id, settlement_date) DO UPDATE SET daily_consumption = EXCLUDED.daily_consumption,
ticket_count = EXCLUDED.ticket_count, matched_rules = EXCLUDED.matched_rules,
amount_rebate = EXCLUDED.amount_rebate, status = 'pending', updated_at = NOW()
WHERE daily_ticket_rebates.status = 'failed' RETURNING id`,
		[]any{candidate.userID, day.Format(time.DateOnly), candidate.consumption.String(), count, string(snapshot), candidate.amount}, &settlementID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("claim daily settlement: %w", err)
	}
	return settlementID, true, nil
}

func (s *WelfareService) creditDailyAmountRebate(ctx context.Context, day time.Time, candidate dailyRewardCandidate) error {
	if candidate.amount <= 0 {
		return nil
	}
	remarks := fmt.Sprintf("%s 消费 $%.2f #%d", day.Format(time.DateOnly), candidate.consumption.InexactFloat64(), candidate.rank)
	_, err := s.createWelfareRecordInTx(ctx, candidate.userID, candidate.email, candidate.amount, remarks)
	return err
}

func creditDailyRebateTickets(ctx context.Context, client *dbent.Client, day time.Time, userID int64, count int) error {
	if count == 0 {
		return nil
	}
	if _, err := client.ExecContext(ctx, `INSERT INTO lottery_user_states (user_id) VALUES ($1) ON CONFLICT DO NOTHING`, userID); err != nil {
		return err
	}
	var debt int
	if err := scanOne(ctx, client, `SELECT ticket_debt FROM lottery_user_states WHERE user_id = $1 FOR UPDATE`, []any{userID}, &debt); err != nil {
		return err
	}
	remaining := count
	if debt < remaining {
		remaining -= debt
		debt = 0
	} else {
		debt -= remaining
		remaining = 0
	}
	ref := fmt.Sprintf("%d:%s", userID, day.Format(time.DateOnly))
	if _, err := client.ExecContext(ctx, `INSERT INTO lottery_ticket_ledger
(user_id, delta, remaining, source_type, source_ref, business_date)
VALUES ($1, $2, $3, 'daily_rebate', $4, $5)`, userID, count, remaining, ref, day.Format(time.DateOnly)); err != nil {
		return err
	}
	_, err := client.ExecContext(ctx, `UPDATE lottery_user_states SET ticket_debt = $2,
available_tickets = (SELECT COALESCE(SUM(remaining), 0) FROM lottery_ticket_ledger
WHERE user_id = $1 AND remaining > 0 AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at > NOW())),
updated_at = NOW(), version = version + 1 WHERE user_id = $1`, userID, debt)
	return err
}

func (s *WelfareService) recordFailedDailySettlement(ctx context.Context, day time.Time, candidate dailyRewardCandidate, count int, snapshot []byte) {
	_, err := s.db.ExecContext(ctx, `INSERT INTO daily_ticket_rebates
(user_id, settlement_date, daily_consumption, ticket_count, matched_rules, amount_rebate, status)
VALUES ($1, $2, $3, $4, $5, $6, 'failed')
ON CONFLICT (user_id, settlement_date) DO UPDATE SET status = 'failed', updated_at = NOW()
WHERE daily_ticket_rebates.status = 'failed'`,
		candidate.userID, day.Format(time.DateOnly), candidate.consumption.String(), count, string(snapshot), candidate.amount)
	if err != nil {
		slog.Error("[WelfareService] failed to record settlement failure", "user_id", candidate.userID, "error", err)
	}
}
