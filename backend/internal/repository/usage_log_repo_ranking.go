package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
)

const defaultUserSpendingRankingLimit = 12

const userSpendingRankingQuery = `
	WITH user_spend AS (
		SELECT
			u.user_id,
			COALESCE(us.email, '') as email,
			COALESCE(SUM(u.actual_cost), 0) as actual_cost,
			COUNT(*) as requests,
			COALESCE(SUM(u.input_tokens + u.output_tokens + u.cache_creation_tokens + u.cache_read_tokens), 0) as tokens
		FROM usage_logs u
		LEFT JOIN users us ON u.user_id = us.id
		WHERE u.created_at >= $1 AND u.created_at < $2
		  AND COALESCE(us.role, 'user') <> 'admin'
		GROUP BY u.user_id, us.email
	),
	ranked AS (
		SELECT
			user_id,
			email,
			actual_cost,
			requests,
			tokens,
			COALESCE(SUM(actual_cost) OVER (), 0) as total_actual_cost,
			COALESCE(SUM(requests) OVER (), 0) as total_requests,
			COALESCE(SUM(tokens) OVER (), 0) as total_tokens
		FROM user_spend
		ORDER BY actual_cost DESC, tokens DESC, user_id ASC
		LIMIT $3
	)
	SELECT
		user_id,
		email,
		actual_cost,
		requests,
		tokens,
		total_actual_cost,
		total_requests,
		total_tokens
	FROM ranked
	ORDER BY actual_cost DESC, tokens DESC, user_id ASC
`

const currentUserRankingQuery = `
	WITH user_spend AS (
		SELECT
			u.user_id,
			COALESCE(us.email, '') as email,
			COALESCE(SUM(u.actual_cost), 0) as actual_cost,
			COUNT(u.user_id) as requests,
			COALESCE(SUM(u.input_tokens + u.output_tokens + u.cache_creation_tokens + u.cache_read_tokens), 0) as tokens
		FROM usage_logs u
		LEFT JOIN users us ON u.user_id = us.id
		WHERE u.created_at >= $1 AND u.created_at < $2
		  AND COALESCE(us.role, 'user') <> 'admin'
		GROUP BY u.user_id, us.email
	),
	all_possible_users AS (
		SELECT user_id, email, actual_cost, requests, tokens FROM user_spend
		UNION
		SELECT
			$3::bigint as user_id,
			COALESCE((SELECT email FROM users WHERE id = $3), '') as email,
			0::numeric as actual_cost,
			0::bigint as requests,
			0::bigint as tokens
		WHERE NOT EXISTS (SELECT 1 FROM user_spend WHERE user_id = $3)
		  AND NOT EXISTS (SELECT 1 FROM users WHERE id = $3 AND role = 'admin')
	),
	ranked AS (
		SELECT
			user_id,
			email,
			actual_cost,
			requests,
			tokens,
			ROW_NUMBER() OVER (ORDER BY actual_cost DESC, tokens DESC, user_id ASC) as rank
		FROM all_possible_users
	)
	SELECT rank, user_id, email, actual_cost, requests, tokens
	FROM ranked
	WHERE user_id = $3
`

type userSpendingTotals struct {
	actualCost float64
	requests   int64
	tokens     int64
}

// GetUserSpendingRanking returns user spending ranking aggregated within the time range.
func (r *usageLogRepository) GetUserSpendingRanking(ctx context.Context, startTime, endTime time.Time, limit int) (result *UserSpendingRankingResponse, err error) {
	if limit <= 0 {
		limit = defaultUserSpendingRankingLimit
	}
	rows, err := r.sql.QueryContext(ctx, userSpendingRankingQuery, startTime, endTime, limit)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err, result = closeErr, nil
		}
	}()

	ranking, totals, err := scanUserSpendingRankingRows(rows)
	if err != nil {
		return nil, err
	}
	currentUser, err := r.resolveCurrentUserRanking(ctx, startTime, endTime, ranking)
	if err != nil {
		return nil, err
	}
	return &UserSpendingRankingResponse{
		Ranking: ranking, TotalActualCost: totals.actualCost, TotalRequests: totals.requests,
		TotalTokens: totals.tokens, UserRanking: currentUser,
	}, nil
}

func scanUserSpendingRankingRows(rows *sql.Rows) ([]UserSpendingRankingItem, userSpendingTotals, error) {
	ranking := make([]UserSpendingRankingItem, 0)
	totals := userSpendingTotals{}
	for rows.Next() {
		var row UserSpendingRankingItem
		if err := rows.Scan(&row.UserID, &row.Email, &row.ActualCost, &row.Requests, &row.Tokens, &totals.actualCost, &totals.requests, &totals.tokens); err != nil {
			return nil, userSpendingTotals{}, err
		}
		row.Rank = int64(len(ranking) + 1)
		ranking = append(ranking, row)
	}
	if err := rows.Err(); err != nil {
		return nil, userSpendingTotals{}, err
	}
	return ranking, totals, nil
}

func (r *usageLogRepository) resolveCurrentUserRanking(ctx context.Context, startTime, endTime time.Time, ranking []UserSpendingRankingItem) (*UserSpendingRankingItem, error) {
	userID, ok := ctx.Value(usagestats.ContextKeyRankingUserID).(int64)
	if !ok || userID <= 0 {
		return nil, nil
	}
	for index := range ranking {
		if ranking[index].UserID == userID {
			return &ranking[index], nil
		}
	}
	return r.loadCurrentUserRanking(ctx, startTime, endTime, userID)
}

func (r *usageLogRepository) loadCurrentUserRanking(ctx context.Context, startTime, endTime time.Time, userID int64) (*UserSpendingRankingItem, error) {
	var row UserSpendingRankingItem
	err := scanSingleRow(ctx, r.sql, currentUserRankingQuery, []any{startTime, endTime, userID}, &row.Rank, &row.UserID, &row.Email, &row.ActualCost, &row.Requests, &row.Tokens)
	if err == nil {
		return &row, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return nil, err
}
