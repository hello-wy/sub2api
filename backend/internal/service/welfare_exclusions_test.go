package service

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/stretchr/testify/require"
)

func TestWelfareExclusionsInvalidPersistedSettingFails(t *testing.T) {
	for _, raw := range []string{"null", "{}", "[0]", "[-1]", "[7,7]", `["7"]`, "invalid"} {
		_, err := ParseWelfareExcludedUserIDs(raw)
		require.Error(t, err, raw)
		svc := NewSettingService(ticketRebateSettingStubWithGetAll{raw: raw}, &config.Config{})
		_, err = svc.GetAllSettings(context.Background())
		require.Error(t, err, raw)
	}
}

type ticketRebateSettingStubWithGetAll struct {
	SettingRepository
	raw string
}

func (s ticketRebateSettingStubWithGetAll) GetAll(context.Context) (map[string]string, error) {
	return map[string]string{SettingKeyWelfareExcludedUserIDs: s.raw}, nil
}

func TestWelfareExclusionsMissingAndEmptySettings(t *testing.T) {
	for _, raw := range []string{"", "[]"} {
		ids, err := ParseWelfareExcludedUserIDs(raw)
		require.NoError(t, err)
		require.Equal(t, []int64{}, ids)
	}
}

func TestDailyRewardCandidatesApplyExclusionsBeforeSettlement(t *testing.T) {
	svc, mock := newTicketSettlementTestService(t)
	day := time.Date(2026, time.October, 11, 0, 0, 0, 0, time.UTC)
	end := day.Add(23 * time.Hour)
	mock.ExpectQuery("SELECT u.user_id, us.email, SUM.*welfare_excluded_user_ids").
		WithArgs(day, end).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "email", "sum"}).AddRow(9, "user@example.com", "20"))
	ranking := []usagestats.UserSpendingRankingItem{{UserID: 9, ActualCost: 20}}
	candidates, err := svc.dailyRewardCandidates(context.Background(), day, end, ranking, []float64{0.5})
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	require.Equal(t, int64(9), candidates[0].userID)
	require.Equal(t, 10.0, candidates[0].amount)
	require.NoError(t, mock.ExpectationsWereMet())
}
