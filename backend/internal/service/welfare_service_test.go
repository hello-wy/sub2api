//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/stretchr/testify/require"
)

func TestParseWelfareRewardSettingsUsesDefaultsForInvalidValues(t *testing.T) {
	limit, ratios := parseWelfareRewardSettings("0", `{"bad":true}`)

	require.Equal(t, welfareDefaultRankLimit, limit)
	require.Equal(t, welfareDefaultRewardRatios, ratios)
}

func TestParseWelfareRewardSettingsPadsRatiosToRankLimit(t *testing.T) {
	limit, ratios := parseWelfareRewardSettings("4", `[1,0.5]`)

	require.Equal(t, 4, limit)
	require.Equal(t, []float64{1, 0.5, 0.5, 0.5}, ratios)
}

func TestCreateWelfareRecordSkipsDuplicateRemark(t *testing.T) {
	repo := &welfareRepoStub{existingRemark: true}
	userSvc := &UserService{userRepo: &mockUserRepo{}}
	svc := &WelfareService{welfareRepo: repo, userService: userSvc}

	record, err := svc.CreateWelfareRecord(context.Background(), 7, "u@example.com", 3.5, "2026-06-23 排行榜消费 #1")

	require.NoError(t, err)
	require.Nil(t, record)
	require.Zero(t, repo.createCalls)
}

func TestDistributeRankingRewardsRemarkIncludesSpendAndRank(t *testing.T) {
	repo := &welfareRepoStub{existingRemark: true}
	svc := &WelfareService{welfareRepo: repo}
	day := time.Date(2026, time.June, 25, 0, 0, 0, 0, time.UTC)

	svc.distributeRankingRewards(context.Background(), []usagestats.UserSpendingRankingItem{
		{UserID: 10, Email: "u@example.com", ActualCost: 12.345},
	}, day, 1, []float64{1})

	require.Equal(t, "2026-06-25 消费 $12.35 #1", repo.lastRemark)
}

func TestListWelfareRecordsIncludesSummaryAndTypeFilter(t *testing.T) {
	repo := &welfareRepoStub{}
	filter := WelfareListFilter{BenefitType: WelfareBenefitTypeCheckin}
	svc := &WelfareService{welfareRepo: repo}

	_, summary, _, err := svc.ListWelfareRecords(context.Background(), pagination.PaginationParams{
		Page: 1, PageSize: 20,
	}, filter)

	require.NoError(t, err)
	require.Equal(t, filter, repo.lastFilter)
	require.Equal(t, 5.25, summary.TotalAmount)
	require.Equal(t, 3.25, summary.CheckinAmount)
	require.Equal(t, 2.0, summary.LeaderboardAmount)
}

func TestWelfareServiceLeaderLockSkipsWhenHeldByPeer(t *testing.T) {
	cache := &fakeLeaderLockCache{}
	_, _ = cache.TryAcquireLeaderLock(context.Background(), welfareLeaderLockKey, "peer", welfareLeaderLockTTL)
	svc := &WelfareService{lockCache: cache, instanceID: "welfare"}

	release, ok := svc.tryAcquireLeaderLock(context.Background())

	require.False(t, ok)
	require.Nil(t, release)
	require.Equal(t, "peer", cache.heldBy(welfareLeaderLockKey))
}

func TestWelfareServiceLeaderLockReleasesWhenAcquired(t *testing.T) {
	cache := &fakeLeaderLockCache{}
	svc := &WelfareService{lockCache: cache, instanceID: "welfare"}

	release, ok := svc.tryAcquireLeaderLock(context.Background())

	require.True(t, ok)
	require.NotNil(t, release)
	require.Equal(t, "welfare", cache.heldBy(welfareLeaderLockKey))

	release()

	require.Empty(t, cache.heldBy(welfareLeaderLockKey))
}

type welfareRepoStub struct {
	existingRemark bool
	createCalls    int
	lastRemark     string
	lastFilter     WelfareListFilter
}

func (r *welfareRepoStub) Create(context.Context, int64, string, float64, string) (*WelfareRecord, error) {
	r.createCalls++
	return &WelfareRecord{ID: int64(r.createCalls)}, nil
}

func (r *welfareRepoStub) GetByID(context.Context, int64, string) (*WelfareRecord, error) {
	return nil, nil
}

func (r *welfareRepoStub) MarkRevoked(context.Context, int64, string) (bool, error) {
	return true, nil
}

func (r *welfareRepoStub) List(_ context.Context, _ pagination.PaginationParams, filter WelfareListFilter) ([]WelfareRecord, *WelfareSummary, *pagination.PaginationResult, error) {
	r.lastFilter = filter
	return nil, &WelfareSummary{
		TotalAmount:       5.25,
		CheckinAmount:     3.25,
		LeaderboardAmount: 2.0,
	}, nil, nil
}

func (r *welfareRepoStub) ExistsSuccessByRemarks(_ context.Context, remarks string) (bool, error) {
	r.lastRemark = remarks
	return r.existingRemark, nil
}
