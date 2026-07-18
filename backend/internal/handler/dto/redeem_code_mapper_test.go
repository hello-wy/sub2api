package dto

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestRedeemCodeFromServiceShowsRewardNotes(t *testing.T) {
	t.Parallel()

	for _, codeType := range []string{service.AdjustmentTypeDailyCheckin, service.AdjustmentTypeUsageRebate, service.AdjustmentTypeSubscriptionPay} {
		dto := RedeemCodeFromService(&service.RedeemCode{
			Type:  codeType,
			Notes: "每日签到 2026/06/28 13:02:25",
		})

		require.NotNil(t, dto.Notes)
		require.Equal(t, "每日签到 2026/06/28 13:02:25", *dto.Notes)
	}
}

func TestRedeemCodeFromServiceHidesRegularRedeemNotes(t *testing.T) {
	t.Parallel()

	dto := RedeemCodeFromService(&service.RedeemCode{
		Type:  service.RedeemTypeBalance,
		Notes: "internal note",
	})

	require.Nil(t, dto.Notes)
}
