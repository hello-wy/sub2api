package service

import (
	"context"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
)

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
