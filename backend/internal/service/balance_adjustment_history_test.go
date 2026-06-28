//go:build unit

package service

import (
	"context"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

func TestCheckInDailyRecordsBalanceHistory(t *testing.T) {
	repo := &mockUserRepo{
		getByIDUser: &User{ID: 42, Balance: 10},
		hasQQ:       true,
	}
	redeemRepo := &balanceAdjustmentRedeemRepoStub{}
	svc := NewUserService(repo, nil, nil, nil)
	svc.SetRedeemCodeRepository(redeemRepo)

	result, err := svc.CheckInDaily(context.Background(), 42, "Asia/Shanghai")

	require.NoError(t, err)
	require.Len(t, redeemRepo.created, 1)
	created := redeemRepo.created[0]
	require.Equal(t, AdjustmentTypeDailyCheckin, created.Type)
	require.Equal(t, StatusUsed, created.Status)
	require.Equal(t, int64(42), *created.UsedBy)
	require.Equal(t, result.Record.TotalReward, created.Value)
	require.Contains(t, created.Notes, "每日签到")
	require.Contains(t, created.Notes, "/")
}

func TestWelfareRewardRecordsBalanceHistory(t *testing.T) {
	userSvc := NewUserService(&mockUserRepo{}, nil, nil, nil)
	redeemRepo := &balanceAdjustmentRedeemRepoStub{}
	userSvc.SetRedeemCodeRepository(redeemRepo)
	welfareRepo := &welfareRepoStub{}
	svc := &WelfareService{welfareRepo: welfareRepo, userService: userSvc}

	record, err := svc.createWelfareRecordInTx(context.Background(), 10, "u@example.com", 2.5, "2026-06-25 消费 $12.50 #1")

	require.NoError(t, err)
	require.NotNil(t, record)
	require.Len(t, redeemRepo.created, 1)
	created := redeemRepo.created[0]
	require.Equal(t, AdjustmentTypeUsageRebate, created.Type)
	require.Equal(t, StatusUsed, created.Status)
	require.Equal(t, int64(10), *created.UsedBy)
	require.Equal(t, 2.5, created.Value)
	require.True(t, strings.Contains(created.Notes, "用量返利"))
	require.True(t, strings.Contains(created.Notes, "2026-06-25 消费 $12.50 #1"))
}

type balanceAdjustmentRedeemRepoStub struct {
	created []RedeemCode
}

func (s *balanceAdjustmentRedeemRepoStub) Create(_ context.Context, code *RedeemCode) error {
	if code == nil {
		return nil
	}
	cloned := *code
	s.created = append(s.created, cloned)
	return nil
}

func (s *balanceAdjustmentRedeemRepoStub) CreateBatch(context.Context, []RedeemCode) error {
	panic("unexpected CreateBatch call")
}

func (s *balanceAdjustmentRedeemRepoStub) GetByID(context.Context, int64) (*RedeemCode, error) {
	panic("unexpected GetByID call")
}

func (s *balanceAdjustmentRedeemRepoStub) GetByCode(context.Context, string) (*RedeemCode, error) {
	panic("unexpected GetByCode call")
}

func (s *balanceAdjustmentRedeemRepoStub) Update(context.Context, *RedeemCode) error {
	panic("unexpected Update call")
}

func (s *balanceAdjustmentRedeemRepoStub) BatchUpdate(context.Context, []int64, RedeemCodeBatchUpdateFields) (int64, error) {
	panic("unexpected BatchUpdate call")
}

func (s *balanceAdjustmentRedeemRepoStub) Delete(context.Context, int64) error {
	panic("unexpected Delete call")
}

func (s *balanceAdjustmentRedeemRepoStub) Use(context.Context, int64, int64) error {
	panic("unexpected Use call")
}

func (s *balanceAdjustmentRedeemRepoStub) List(context.Context, pagination.PaginationParams) ([]RedeemCode, *pagination.PaginationResult, error) {
	panic("unexpected List call")
}

func (s *balanceAdjustmentRedeemRepoStub) ListWithFilters(context.Context, pagination.PaginationParams, string, string, string) ([]RedeemCode, *pagination.PaginationResult, error) {
	panic("unexpected ListWithFilters call")
}

func (s *balanceAdjustmentRedeemRepoStub) ListByUser(context.Context, int64, int) ([]RedeemCode, error) {
	panic("unexpected ListByUser call")
}

func (s *balanceAdjustmentRedeemRepoStub) ListByUserPaginated(context.Context, int64, pagination.PaginationParams, string) ([]RedeemCode, *pagination.PaginationResult, error) {
	panic("unexpected ListByUserPaginated call")
}

func (s *balanceAdjustmentRedeemRepoStub) SumPositiveBalanceByUser(context.Context, int64) (float64, error) {
	panic("unexpected SumPositiveBalanceByUser call")
}
