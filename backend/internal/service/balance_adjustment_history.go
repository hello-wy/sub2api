package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
)

const balanceAdjustmentTimeLayout = "2006/01/02 15:04:05"

func recordBalanceAdjustment(ctx context.Context, repo RedeemCodeRepository, userID int64, amount float64, note string) error {
	return recordTypedBalanceAdjustment(ctx, repo, userID, amount, AdjustmentTypeAdminBalance, note)
}

func recordTypedBalanceAdjustment(ctx context.Context, repo RedeemCodeRepository, userID int64, amount float64, recordType string, note string) error {
	if repo == nil || amount == 0 {
		return nil
	}

	code, err := GenerateRedeemCode()
	if err != nil {
		return fmt.Errorf("generate adjustment redeem code: %w", err)
	}

	now := timezone.Now()
	usedBy := userID
	record := &RedeemCode{
		Code:   code,
		Type:   recordType,
		Value:  amount,
		Status: StatusUsed,
		UsedBy: &usedBy,
		UsedAt: &now,
		Notes:  note,
	}
	if err := repo.Create(ctx, record); err != nil {
		return fmt.Errorf("create balance adjustment redeem code: %w", err)
	}
	return nil
}

func balanceAdjustmentNote(kind string, at time.Time, details ...string) string {
	parts := []string{strings.TrimSpace(kind), at.In(timezone.Location()).Format(balanceAdjustmentTimeLayout)}
	for _, detail := range details {
		detail = strings.TrimSpace(detail)
		if detail != "" {
			parts = append(parts, detail)
		}
	}
	return strings.Join(parts, " ")
}
