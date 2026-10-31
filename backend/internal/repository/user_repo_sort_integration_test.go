//go:build integration

package repository

import (
	"fmt"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (s *UserRepoSuite) numericSortAttribute() *dbent.UserAttributeDefinition {
	s.T().Helper()
	def, err := s.client.UserAttributeDefinition.Create().
		SetKey(fmt.Sprintf("numeric_sort_%d", time.Now().UnixNano())).
		SetName("测试数字").SetType("number").Save(s.ctx)
	s.Require().NoError(err)
	s.T().Cleanup(func() { _ = s.client.UserAttributeDefinition.DeleteOneID(def.ID).Exec(s.ctx) })
	return def
}

func (s *UserRepoSuite) TestListWithFilters_NumericAttributeSortAcrossPages() {
	def := s.numericSortAttribute()
	values := []string{"10", "2", "-2.5", "0", "1.25", "2", "9007199254740993", "9007199254740992", "", "broken", "NaN", "Infinity", "-Infinity", "1e1000000", " \t2.50 ", "1e1"}
	ids := make([]int64, 0, len(values)+1)
	for i, value := range values {
		u := s.mustCreateUser(&service.User{Email: fmt.Sprintf("numeric-sort-%02d@example.com", i)})
		ids = append(ids, u.ID)
		_, err := s.client.UserAttributeValue.Create().SetUserID(u.ID).SetAttributeID(def.ID).SetValue(value).Save(s.ctx)
		s.Require().NoError(err)
	}
	ids = append(ids, s.mustCreateUser(&service.User{Email: "numeric-sort-missing@example.com"}).ID)
	// 筛选外的用户不能混入全局排序或总数。
	other := s.mustCreateUser(&service.User{Email: "unrelated@example.com"})
	_, err := s.client.UserAttributeValue.Create().SetUserID(other.ID).SetAttributeID(def.ID).SetValue("1e99").Save(s.ctx)
	s.Require().NoError(err)
	for _, tc := range []struct {
		order   string
		indices []int
	}{
		{"asc", []int{2, 3, 4, 1, 5, 14, 0, 15, 7, 6, 8, 9, 10, 11, 12, 13, 16}},
		{"desc", []int{6, 7, 15, 0, 14, 5, 1, 4, 3, 2, 16, 13, 12, 11, 10, 9, 8}},
	} {
		s.Run(tc.order, func() {
			var got []int64
			for page := 1; len(got) < len(ids); page++ {
				users, result, err := s.repo.ListWithFilters(s.ctx, pagination.PaginationParams{
					Page: page, PageSize: 3, SortBy: fmt.Sprintf("attr_%d", def.ID), SortOrder: tc.order,
				}, service.UserListFilters{Search: "numeric-sort-"})
				s.Require().NoError(err)
				s.Require().Equal(int64(len(ids)), result.Total)
				s.Require().NotEmpty(users)
				for _, u := range users {
					got = append(got, u.ID)
				}
			}
			want := make([]int64, 0, len(ids))
			for _, index := range tc.indices {
				want = append(want, ids[index])
			}
			s.Require().Equal(want, got)
		})
	}
	users, _, err := s.repo.ListWithFilters(s.ctx, pagination.PaginationParams{
		Page: 1, PageSize: 10, SortBy: fmt.Sprintf("attr_%d", def.ID), SortOrder: "asc",
	}, service.UserListFilters{Attributes: map[int64]string{def.ID: "1.25"}})
	s.Require().NoError(err)
	s.Require().Len(users, 1)
	s.Require().Equal(ids[4], users[0].ID)
}

func (s *UserRepoSuite) TestListWithFilters_NumericAttributeSortFallback() {
	first := s.mustCreateUser(&service.User{Email: "fallback-first@example.com"})
	last := s.mustCreateUser(&service.User{Email: "fallback-last@example.com"})
	def := s.numericSortAttribute()
	key := fmt.Sprintf("attr_%d", def.ID)
	check := func(sortBy string) {
		s.T().Helper()
		users, _, err := s.repo.ListWithFilters(s.ctx, pagination.PaginationParams{
			Page: 1, PageSize: 10, SortBy: sortBy, SortOrder: "asc",
		}, service.UserListFilters{})
		s.Require().NoError(err)
		s.Require().Len(users, 2)
		s.Require().Equal([]int64{last.ID, first.ID}, []int64{users[0].ID, users[1].ID})
	}
	for _, invalid := range []string{"attr_0", "attr_-1", "attr_9223372036854775808", "attr_999999999", "attr_1; DROP TABLE users"} {
		check(invalid)
	}
	_, err := s.client.UserAttributeDefinition.UpdateOneID(def.ID).SetEnabled(false).Save(s.ctx)
	s.Require().NoError(err)
	check(key)
	_, err = s.client.UserAttributeDefinition.UpdateOneID(def.ID).SetEnabled(true).SetType("text").Save(s.ctx)
	s.Require().NoError(err)
	check(key)
	_, err = s.client.UserAttributeDefinition.UpdateOneID(def.ID).SetType("number").SetDeletedAt(time.Now()).Save(s.ctx)
	s.Require().NoError(err)
	check(key)
}

func (s *UserRepoSuite) TestListWithFilters_NumericAttributeSortUsesLotteryLedger() {
	def := s.numericSortAttribute()
	_, err := s.client.UserAttributeDefinition.UpdateOneID(def.ID).SetName("剩余抽奖次数").Save(s.ctx)
	s.Require().NoError(err)
	zero := s.mustCreateUser(&service.User{Email: "tickets-zero@example.com"})
	low := s.mustCreateUser(&service.User{Email: "tickets-low@example.com"})
	high := s.mustCreateUser(&service.User{Email: "tickets-high@example.com"})
	same := s.mustCreateUser(&service.User{Email: "tickets-same@example.com"})
	for i, tc := range []struct {
		userID               int64
		remaining            int
		expiresAt, revokedAt any
	}{
		{low.ID, 2, nil, nil}, {high.ID, 6, nil, nil}, {high.ID, 4, time.Now().Add(time.Hour), nil},
		{same.ID, 2, nil, nil}, {zero.ID, 100, time.Now().Add(-time.Hour), nil},
		{zero.ID, 100, nil, time.Now()}, {zero.ID, 0, nil, nil},
	} {
		_, err := integrationDB.ExecContext(s.ctx, `INSERT INTO lottery_ticket_ledger
			(user_id, delta, remaining, source_type, source_ref, expires_at, revoked_at)
			VALUES ($1, $2, $2, 'admin', $3, $4, $5)`, tc.userID, tc.remaining, fmt.Sprintf("sort-%d-%d", def.ID, i), tc.expiresAt, tc.revokedAt)
		s.Require().NoError(err)
	}
	// 属性表和旧汇总故意与账本相反，确保不误用缓存/手工属性。
	_, err = s.client.UserAttributeValue.Create().SetUserID(zero.ID).SetAttributeID(def.ID).SetValue("9999").Save(s.ctx)
	s.Require().NoError(err)
	_, err = integrationDB.ExecContext(s.ctx, "INSERT INTO lottery_user_states(user_id, available_tickets) VALUES ($1, 9999)", zero.ID)
	s.Require().NoError(err)
	standard, err := NewUserAttributeDefinitionRepository(s.client).GetByKey(s.ctx, service.LotteryAvailableTicketsAttributeKey)
	s.Require().NoError(err)
	for _, attributeID := range []int64{def.ID, standard.ID} {
		for _, tc := range []struct {
			order string
			want  []int64
		}{
			{"asc", []int64{zero.ID, low.ID, same.ID, high.ID}},
			{"desc", []int64{high.ID, same.ID, low.ID, zero.ID}},
		} {
			users, _, err := s.repo.ListWithFilters(s.ctx, pagination.PaginationParams{
				Page: 1, PageSize: 10, SortBy: fmt.Sprintf("attr_%d", attributeID), SortOrder: tc.order,
			}, service.UserListFilters{})
			s.Require().NoError(err)
			var got []int64
			for _, u := range users {
				got = append(got, u.ID)
			}
			s.Require().Equal(tc.want, got)
		}
	}
}

func (s *UserRepoSuite) mustInsertUsageLog(userID int64, createdAt time.Time) {
	s.T().Helper()

	account := mustCreateAccount(s.T(), s.client, &service.Account{Name: "usage-log-account"})
	apiKey := mustCreateApiKey(s.T(), s.client, &service.APIKey{UserID: userID})

	_, err := integrationDB.ExecContext(
		s.ctx,
		`INSERT INTO usage_logs (user_id, api_key_id, account_id, model, input_tokens, output_tokens, total_cost, actual_cost, created_at)
		 VALUES ($1, $2, $3, 'gpt-test', 1, 1, 0.01, 0.01, $4)`,
		userID,
		apiKey.ID,
		account.ID,
		createdAt.UTC(),
	)
	s.Require().NoError(err)
}

func (s *UserRepoSuite) TestListWithFilters_SortByEmailAsc() {
	s.mustCreateUser(&service.User{Email: "z-last@example.com", Username: "z-user"})
	s.mustCreateUser(&service.User{Email: "a-first@example.com", Username: "a-user"})

	users, _, err := s.repo.ListWithFilters(s.ctx, pagination.PaginationParams{
		Page:      1,
		PageSize:  10,
		SortBy:    "email",
		SortOrder: "asc",
	}, service.UserListFilters{})
	s.Require().NoError(err)
	s.Require().Len(users, 2)
	s.Require().Equal("a-first@example.com", users[0].Email)
	s.Require().Equal("z-last@example.com", users[1].Email)
}

func (s *UserRepoSuite) TestList_DefaultSortByNewestFirst() {
	first := s.mustCreateUser(&service.User{Email: "first@example.com"})
	second := s.mustCreateUser(&service.User{Email: "second@example.com"})

	users, _, err := s.repo.List(s.ctx, pagination.PaginationParams{Page: 1, PageSize: 10})
	s.Require().NoError(err)
	s.Require().Len(users, 2)
	s.Require().Equal(second.ID, users[0].ID)
	s.Require().Equal(first.ID, users[1].ID)
}

func (s *UserRepoSuite) TestCreateAndRead_PreservesSignupSourceAndActivityTimestamps() {
	lastLoginAt := time.Now().Add(-2 * time.Hour).UTC().Truncate(time.Microsecond)
	lastActiveAt := time.Now().Add(-30 * time.Minute).UTC().Truncate(time.Microsecond)

	created := s.mustCreateUser(&service.User{
		Email:        "identity-meta@example.com",
		SignupSource: "linuxdo",
		LastLoginAt:  &lastLoginAt,
		LastActiveAt: &lastActiveAt,
	})

	got, err := s.repo.GetByID(s.ctx, created.ID)
	s.Require().NoError(err)
	s.Require().Equal("linuxdo", got.SignupSource)
	s.Require().NotNil(got.LastLoginAt)
	s.Require().NotNil(got.LastActiveAt)
	s.Require().True(got.LastLoginAt.Equal(lastLoginAt))
	s.Require().True(got.LastActiveAt.Equal(lastActiveAt))
}

func (s *UserRepoSuite) TestUpdate_PersistsSignupSourceAndActivityTimestamps() {
	created := s.mustCreateUser(&service.User{Email: "identity-update@example.com"})
	lastLoginAt := time.Now().Add(-90 * time.Minute).UTC().Truncate(time.Microsecond)
	lastActiveAt := time.Now().Add(-15 * time.Minute).UTC().Truncate(time.Microsecond)

	created.SignupSource = "oidc"
	created.LastLoginAt = &lastLoginAt
	created.LastActiveAt = &lastActiveAt

	s.Require().NoError(s.repo.Update(s.ctx, created, service.UserUpdateFields{SignupSource: true, LastLoginAt: true, LastActiveAt: true}))

	got, err := s.repo.GetByID(s.ctx, created.ID)
	s.Require().NoError(err)
	s.Require().Equal("oidc", got.SignupSource)
	s.Require().NotNil(got.LastLoginAt)
	s.Require().NotNil(got.LastActiveAt)
	s.Require().True(got.LastLoginAt.Equal(lastLoginAt))
	s.Require().True(got.LastActiveAt.Equal(lastActiveAt))
}

func (s *UserRepoSuite) TestListWithFilters_SortByLastActiveAtAsc() {
	earlier := time.Now().Add(-3 * time.Hour).UTC().Truncate(time.Microsecond)
	later := time.Now().Add(-45 * time.Minute).UTC().Truncate(time.Microsecond)

	s.mustCreateUser(&service.User{Email: "nil-active@example.com"})
	s.mustCreateUser(&service.User{Email: "later-active@example.com", LastActiveAt: &later})
	s.mustCreateUser(&service.User{Email: "earlier-active@example.com", LastActiveAt: &earlier})

	users, _, err := s.repo.ListWithFilters(s.ctx, pagination.PaginationParams{
		Page:      1,
		PageSize:  10,
		SortBy:    "last_active_at",
		SortOrder: "asc",
	}, service.UserListFilters{})
	s.Require().NoError(err)
	s.Require().Len(users, 3)
	s.Require().Equal("earlier-active@example.com", users[0].Email)
	s.Require().Equal("later-active@example.com", users[1].Email)
	s.Require().Equal("nil-active@example.com", users[2].Email)
}

func (s *UserRepoSuite) TestGetLatestUsedAtByUserIDs_UsesUsageLogs() {
	older := time.Now().Add(-4 * time.Hour).UTC().Truncate(time.Second)
	newer := time.Now().Add(-90 * time.Minute).UTC().Truncate(time.Second)

	userWithUsage := s.mustCreateUser(&service.User{Email: "usage-source@example.com"})
	userWithoutUsage := s.mustCreateUser(&service.User{Email: "usage-missing@example.com"})
	s.mustInsertUsageLog(userWithUsage.ID, older)
	s.mustInsertUsageLog(userWithUsage.ID, newer)

	got, err := s.repo.GetLatestUsedAtByUserIDs(s.ctx, []int64{userWithUsage.ID, userWithoutUsage.ID})
	s.Require().NoError(err)
	s.Require().Contains(got, userWithUsage.ID)
	s.Require().NotContains(got, userWithoutUsage.ID)
	s.Require().NotNil(got[userWithUsage.ID])
	s.Require().True(got[userWithUsage.ID].Equal(newer))
}

func (s *UserRepoSuite) TestListWithFilters_SortByLastUsedAtDesc_UsesUsageLogsNotLastActiveAt() {
	lastUsedOlder := time.Now().Add(-6 * time.Hour).UTC().Truncate(time.Second)
	lastUsedNewer := time.Now().Add(-2 * time.Hour).UTC().Truncate(time.Second)
	lastActiveVeryRecent := time.Now().Add(-10 * time.Minute).UTC().Truncate(time.Second)

	nilUsage := s.mustCreateUser(&service.User{Email: "nil-last-used@example.com"})
	wrongSource := s.mustCreateUser(&service.User{
		Email:        "active-not-usage@example.com",
		LastActiveAt: &lastActiveVeryRecent,
	})
	rightSource := s.mustCreateUser(&service.User{Email: "usage-wins@example.com"})

	s.mustInsertUsageLog(wrongSource.ID, lastUsedOlder)
	s.mustInsertUsageLog(rightSource.ID, lastUsedNewer)

	users, _, err := s.repo.ListWithFilters(s.ctx, pagination.PaginationParams{
		Page:      1,
		PageSize:  10,
		SortBy:    "last_used_at",
		SortOrder: "desc",
	}, service.UserListFilters{})
	s.Require().NoError(err)
	s.Require().Len(users, 3)
	s.Require().Equal(rightSource.ID, users[0].ID)
	s.Require().Equal(wrongSource.ID, users[1].ID)
	s.Require().Equal(nilUsage.ID, users[2].ID)
}

func TestUserRepoSortSuiteSmoke(_ *testing.T) {}
