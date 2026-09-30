package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/stretchr/testify/require"
)

func TestUsageLogRepositoryGetUserSpendingRankingReturnsCurrentUserOutsideTopLimit(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	userID := int64(17)

	mock.ExpectQuery("WITH user_spend AS \\(").
		WithArgs(start, end, 12).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "email", "actual_cost", "requests", "tokens", "total_actual_cost", "total_requests", "total_tokens"}).
			AddRow(int64(2), "top@example.com", 12.5, int64(9), int64(900), 12.5, int64(9), int64(900)))
	mock.ExpectQuery("ROW_NUMBER\\(\\) OVER").
		WithArgs(start, end, userID).
		WillReturnRows(sqlmock.NewRows([]string{"rank", "user_id", "email", "actual_cost", "requests", "tokens"}).
			AddRow(int64(7), userID, "viewer@example.com", 1.5, int64(2), int64(100)))
	mock.ExpectQuery("FROM daily_ticket_rebates").WithArgs("2025-01-01", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "amount_rebate", "ticket_count", "matched_rules"}))

	ctx := context.WithValue(context.Background(), usagestats.ContextKeyRankingUserID, userID)
	got, err := repo.GetUserSpendingRanking(ctx, start, end, 12)

	require.NoError(t, err)
	require.Equal(t, &usagestats.UserSpendingRankingItem{
		UserID: userID, Email: "viewer@example.com", ActualCost: 1.5, Requests: 2, Tokens: 100, Rank: 7,
	}, got.UserRanking)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageLogRepositoryRankingUsesStoredRebateSnapshot(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}
	day := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	mock.ExpectQuery("WITH user_spend AS").WithArgs(day, day.Add(24*time.Hour), 12).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "email", "actual_cost", "requests", "tokens", "total_actual_cost", "total_requests", "total_tokens"}).
			AddRow(int64(7), "viewer@example.com", 10, int64(1), int64(10), 10, int64(1), int64(10)))
	mock.ExpectQuery("FROM daily_ticket_rebates").WithArgs("2025-01-01", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "amount_rebate", "ticket_count", "matched_rules"}).
			AddRow(int64(7), 2.5, 2, []byte(`[{"amount_threshold":5,"ticket_count":1},{"amount_threshold":10,"ticket_count":1}]`)))

	result, err := repo.GetUserSpendingRanking(context.Background(), day, day.Add(24*time.Hour), 12)
	require.NoError(t, err)
	require.Equal(t, 2, *result.Ranking[0].TicketCount)
	require.Equal(t, 2.5, *result.Ranking[0].AmountRebate)
	require.JSONEq(t, `[{"amount_threshold":5,"ticket_count":1},{"amount_threshold":10,"ticket_count":1}]`, string(result.Ranking[0].MatchedRules))
	require.NoError(t, mock.ExpectationsWereMet())
}
