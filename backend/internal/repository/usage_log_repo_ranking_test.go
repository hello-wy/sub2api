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

	ctx := context.WithValue(context.Background(), usagestats.ContextKeyRankingUserID, userID)
	got, err := repo.GetUserSpendingRanking(ctx, start, end, 12)

	require.NoError(t, err)
	require.Equal(t, &usagestats.UserSpendingRankingItem{
		UserID: userID, Email: "viewer@example.com", ActualCost: 1.5, Requests: 2, Tokens: 100, Rank: 7,
	}, got.UserRanking)
	require.NoError(t, mock.ExpectationsWereMet())
}
