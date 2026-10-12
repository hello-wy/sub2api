//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
)

func (s *UsageLogRepoSuite) TestWelfareExcludedUsersRanking() {
	const rankLimit = 1
	day := time.Date(2026, time.October, 11, 0, 0, 0, 0, time.UTC)
	excluded, eligible := s.createWelfareRankingUsers(day)
	raw, err := json.Marshal([]int64{excluded.ID})
	s.Require().NoError(err)
	_, err = s.tx.ExecContext(s.ctx, `UPDATE settings SET value = $1 WHERE key = $2`, string(raw), service.SettingKeyWelfareExcludedUserIDs)
	s.Require().NoError(err)
	ctx := context.WithValue(s.ctx, usagestats.ContextKeyRankingUserID, excluded.ID)
	result, err := s.repo.GetUserSpendingRanking(ctx, day, day.AddDate(0, 0, 1), rankLimit)
	s.Require().NoError(err)
	s.Require().Len(result.Ranking, rankLimit)
	s.Equal(eligible.ID, result.Ranking[0].UserID)
	s.Equal(int64(1), result.Ranking[0].Rank)
	s.Equal(20.0, result.TotalActualCost)
	s.Nil(result.UserRanking)
	_, err = s.tx.ExecContext(s.ctx, `UPDATE settings SET value = $1 WHERE key = $2`, string(raw), service.SettingKeyWelfareLeaderboardVisibleUserIDs)
	s.Require().NoError(err)
	result, err = s.repo.GetUserSpendingRanking(ctx, day, day.AddDate(0, 0, 1), rankLimit)
	s.Require().NoError(err)
	s.Equal(excluded.ID, result.Ranking[0].UserID)
	s.Equal(120.0, result.TotalActualCost)
	s.Require().NotNil(result.UserRanking)
	s.Equal(int64(1), result.UserRanking.Rank)

	s.assertWelfareRewardRecipient(day, eligible.ID)

	_, err = s.tx.ExecContext(s.ctx, `UPDATE settings SET value = '[]' WHERE key = $1`, service.SettingKeyWelfareLeaderboardVisibleUserIDs)
	s.Require().NoError(err)
	result, err = s.repo.GetUserSpendingRanking(ctx, day, day.AddDate(0, 0, 1), rankLimit)
	s.Require().NoError(err)
	s.Equal(eligible.ID, result.Ranking[0].UserID)
	s.Nil(result.UserRanking)
}

func (s *UsageLogRepoSuite) createWelfareRankingUsers(day time.Time) (*service.User, *service.User) {
	excluded := mustCreateUser(s.T(), s.client, &service.User{Email: "excluded@example.com"})
	eligible := mustCreateUser(s.T(), s.client, &service.User{Email: "eligible@example.com"})
	admin := mustCreateUser(s.T(), s.client, &service.User{Email: "admin@example.com", Role: service.RoleAdmin})
	account := mustCreateAccount(s.T(), s.client, &service.Account{Name: "exclusion-test"})
	for _, entry := range []struct {
		user *service.User
		cost float64
	}{{excluded, 100}, {eligible, 20}, {admin, 200}} {
		key := mustCreateApiKey(s.T(), s.client, &service.APIKey{UserID: entry.user.ID, Key: entry.user.Email, Name: "exclusion"})
		s.createUsageLog(entry.user, key, account, 10, 10, entry.cost, day.Add(time.Hour))
	}
	return excluded, eligible
}

func (s *UsageLogRepoSuite) assertWelfareRewardRecipient(day time.Time, eligibleID int64) {
	var rewardUserID int64
	var email, cost string
	err := scanSingleRow(s.ctx, s.tx, usagestats.WelfareRewardCandidatesSQL+` LIMIT 1`,
		[]any{day, day.AddDate(0, 0, 1)}, &rewardUserID, &email, &cost)
	s.Require().NoError(err)
	s.Equal(eligibleID, rewardUserID)
	s.NotEmpty(cost)
}

func (s *UsageLogRepoSuite) TestWelfareExcludedUserWithoutSpendingHasNoRank() {
	user := mustCreateUser(s.T(), s.client, &service.User{Email: "no-spending@example.com"})
	raw, err := json.Marshal([]int64{user.ID})
	s.Require().NoError(err)
	_, err = s.tx.ExecContext(s.ctx, `UPDATE settings SET value = $1 WHERE key = $2`, string(raw), service.SettingKeyWelfareExcludedUserIDs)
	s.Require().NoError(err)
	day := time.Date(2026, time.October, 11, 0, 0, 0, 0, time.UTC)
	ctx := context.WithValue(s.ctx, usagestats.ContextKeyRankingUserID, user.ID)
	result, err := s.repo.GetUserSpendingRanking(ctx, day, day.AddDate(0, 0, 1), 1)
	s.Require().NoError(err)
	s.Nil(result.UserRanking)
	_, err = s.tx.ExecContext(s.ctx, `UPDATE settings SET value = $1 WHERE key = $2`, string(raw), service.SettingKeyWelfareLeaderboardVisibleUserIDs)
	s.Require().NoError(err)
	result, err = s.repo.GetUserSpendingRanking(ctx, day, day.AddDate(0, 0, 1), 1)
	s.Require().NoError(err)
	s.Require().NotNil(result.UserRanking)
	s.Equal(user.ID, result.UserRanking.UserID)
	s.Zero(result.UserRanking.ActualCost)
}

func (s *UsageLogRepoSuite) TestWelfareExclusionsMigrationPreservesLegacyUser() {
	user := mustCreateUser(s.T(), s.client, &service.User{Email: "2281137167@qq.com"})
	_, err := s.tx.ExecContext(s.ctx, `DELETE FROM settings WHERE key = $1`, service.SettingKeyWelfareExcludedUserIDs)
	s.Require().NoError(err)
	sql, err := migrations.FS.ReadFile("255_welfare_excluded_users.sql")
	s.Require().NoError(err)
	_, err = s.tx.ExecContext(s.ctx, string(sql))
	s.Require().NoError(err)
	var raw string
	err = scanSingleRow(s.ctx, s.tx, `SELECT value FROM settings WHERE key = $1`, []any{service.SettingKeyWelfareExcludedUserIDs}, &raw)
	s.Require().NoError(err)
	ids, err := service.ParseWelfareExcludedUserIDs(raw)
	s.Require().NoError(err)
	s.Equal([]int64{user.ID}, ids)
	_, err = s.tx.ExecContext(s.ctx, `UPDATE settings SET value = '[]' WHERE key = $1`, service.SettingKeyWelfareExcludedUserIDs)
	s.Require().NoError(err)
	_, err = s.tx.ExecContext(s.ctx, string(sql))
	s.Require().NoError(err)
	err = scanSingleRow(s.ctx, s.tx, `SELECT value FROM settings WHERE key = $1`, []any{service.SettingKeyWelfareExcludedUserIDs}, &raw)
	s.Require().NoError(err)
	s.Equal("[]", raw)
}
