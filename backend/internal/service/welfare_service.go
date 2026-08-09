package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
)

const (
	welfareJobName                = "welfare_reward"
	welfareLeaderLockKey          = "welfare:reward:leader"
	welfareLeaderLockTTL          = 30 * time.Minute
	welfareCronStopTimeout        = 3 * time.Second
	welfareAdvisoryLockID   int64 = 694208311321144028 // Unique advisory lock ID
	welfareDefaultRankLimit       = 3
)

var welfareDefaultRewardRatios = []float64{1.0, 0.5, 0.2}

var welfareCronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

type WelfareService struct {
	welfareRepo  WelfareRepository
	userService  *UserService
	dashboardSvc *DashboardService
	settingRepo  SettingRepository
	lockCache    LeaderLockCache
	db           *sql.DB
	entClient    *dbent.Client
	cfg          *config.Config

	instanceID string
	mu         sync.Mutex
	cron       *cron.Cron
	started    bool
	stopped    bool

	warnNoRedisOnce sync.Once
}

func NewWelfareService(
	welfareRepo WelfareRepository,
	userService *UserService,
	dashboardSvc *DashboardService,
	settingRepo SettingRepository,
	lockCache LeaderLockCache,
	db *sql.DB,
	entClient *dbent.Client,
	cfg *config.Config,
) *WelfareService {
	return &WelfareService{
		welfareRepo:  welfareRepo,
		userService:  userService,
		dashboardSvc: dashboardSvc,
		settingRepo:  settingRepo,
		lockCache:    lockCache,
		db:           db,
		entClient:    entClient,
		cfg:          cfg,
		instanceID:   uuid.NewString(),
	}
}

// RunRewardJob executes the daily welfare rewards calculation and distribution.
func (s *WelfareService) RunRewardJob(ctx context.Context) {
	if s == nil || s.db == nil || s.dashboardSvc == nil || s.welfareRepo == nil {
		return
	}

	// Tries to acquire leader lock so only one node runs the rewards job.
	release, ok := s.tryAcquireLeaderLock(ctx)
	if !ok {
		return
	}
	if release != nil {
		defer release()
	}

	startedAt := time.Now()
	slog.Info("[WelfareService] starting daily reward job execution")

	rankLimit, ratios := s.loadRewardSettings(ctx)
	today := timezone.Today()
	endTime := timezone.Now()
	ranking, err := s.dashboardSvc.GetUserSpendingRanking(ctx, today, endTime, rankLimit)
	if err != nil {
		slog.Error("[WelfareService] failed to fetch daily user spending ranking", "error", err)
		return
	}

	count := s.distributeRankingRewards(ctx, ranking.Ranking, today, rankLimit, ratios)
	slog.Info("[WelfareService] daily reward job finished", "distributed_count", count, "duration", time.Since(startedAt))
}

func (s *WelfareService) distributeRankingRewards(ctx context.Context, ranking []usagestats.UserSpendingRankingItem, day time.Time, rankLimit int, ratios []float64) int {
	distributedCount := 0
	todayStr := day.Format("2006-01-02")
	for i, item := range ranking {
		if i >= rankLimit || i >= len(ratios) {
			break
		}
		ratio := ratios[i]
		if item.ActualCost <= 0 || ratio <= 0 {
			continue
		}
		rewardAmount := item.ActualCost * ratio
		if rewardAmount <= 0 {
			continue
		}
		remarks := fmt.Sprintf("%s 消费 $%.2f #%d", todayStr, item.ActualCost, i+1)
		_, err := s.CreateWelfareRecord(ctx, item.UserID, item.Email, rewardAmount, remarks)
		if err != nil {
			slog.Error("[WelfareService] failed to distribute welfare reward", "user_id", item.UserID, "email", item.Email, "amount", rewardAmount, "error", err)
		} else {
			distributedCount++
			slog.Info("[WelfareService] distributed welfare reward successfully", "user_id", item.UserID, "email", item.Email, "amount", rewardAmount, "rank", i+1)
		}
	}
	return distributedCount
}

// CreateWelfareRecord creates a welfare reward record, increases the user's balance, and invalidates caches.
func (s *WelfareService) CreateWelfareRecord(ctx context.Context, userID int64, email string, amount float64, remarks string) (*WelfareRecord, error) {
	exists, err := s.welfareRepo.ExistsSuccessByRemarks(ctx, remarks)
	if err != nil {
		return nil, fmt.Errorf("check duplicate welfare record: %w", err)
	}
	if exists {
		return nil, nil
	}
	return s.createWelfareRecordTx(ctx, userID, email, amount, remarks)
}

func (s *WelfareService) createWelfareRecordTx(ctx context.Context, userID int64, email string, amount float64, remarks string) (*WelfareRecord, error) {
	if s.entClient == nil {
		return nil, errors.New("ent client is required to create welfare record")
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin welfare transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)

	record, err := s.createWelfareRecordInTx(txCtx, userID, email, amount, remarks)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit welfare transaction: %w", err)
	}
	return record, nil
}

func (s *WelfareService) createWelfareRecordInTx(ctx context.Context, userID int64, email string, amount float64, remarks string) (*WelfareRecord, error) {
	exists, err := s.welfareRepo.ExistsSuccessByRemarks(ctx, remarks)
	if err != nil {
		return nil, fmt.Errorf("check duplicate welfare record in transaction: %w", err)
	}
	if exists {
		return nil, nil
	}
	if err := s.userService.UpdateBalance(ctx, userID, amount); err != nil {
		return nil, fmt.Errorf("update user balance: %w", err)
	}
	record, err := s.welfareRepo.Create(ctx, userID, email, amount, remarks)
	if err != nil {
		return nil, fmt.Errorf("create welfare record: %w", err)
	}
	if err := recordTypedBalanceAdjustment(ctx, s.userService.redeemCodeRepo, userID, amount, AdjustmentTypeUsageRebate, balanceAdjustmentNote("用量返利", timezone.Now(), remarks)); err != nil {
		return nil, err
	}
	return record, nil
}

// RevokeWelfareRecord revokes a welfare record, deducts the amount from the user's balance, and updates the status to "revoked".
func (s *WelfareService) RevokeWelfareRecord(ctx context.Context, recordID int64, benefitType string) error {
	normalizedType := normalizeWelfareBenefitType(benefitType)
	record, err := s.welfareRepo.GetByID(ctx, recordID, normalizedType)
	if err != nil {
		return fmt.Errorf("failed to find welfare record: %w", err)
	}

	if record.Status == WelfareStatusRevoked {
		return errors.New("welfare reward is already revoked")
	}
	if err := s.revokeWelfareRecordTx(ctx, record); err != nil {
		return err
	}

	slog.Info("[WelfareService] revoked welfare record successfully", "record_id", recordID, "user_id", record.UserID, "amount", record.Amount)
	return nil
}

func (s *WelfareService) revokeWelfareRecordTx(ctx context.Context, record *WelfareRecord) error {
	if s.entClient == nil {
		return errors.New("ent client is required to revoke welfare record")
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin welfare revoke transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)

	if err := s.userService.UpdateBalance(txCtx, record.UserID, -record.Amount); err != nil {
		return fmt.Errorf("deduct user balance for revoke: %w", err)
	}
	updated, err := s.welfareRepo.MarkRevoked(txCtx, record.ID, record.Type)
	if err != nil {
		return fmt.Errorf("mark welfare record revoked: %w", err)
	}
	if !updated {
		return errors.New("welfare reward is already revoked")
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit welfare revoke transaction: %w", err)
	}
	return nil
}

func normalizeWelfareBenefitType(benefitType string) string {
	if benefitType == WelfareBenefitTypeCheckin {
		return WelfareBenefitTypeCheckin
	}
	if benefitType == WelfareBenefitTypeLottery {
		return WelfareBenefitTypeLottery
	}
	return WelfareBenefitTypeLeaderboard
}

// ListWelfareRecords lists all welfare records with paginated parameters and optional filters.
func (s *WelfareService) ListWelfareRecords(ctx context.Context, params pagination.PaginationParams, filter WelfareListFilter) ([]WelfareRecord, *WelfareSummary, *pagination.PaginationResult, error) {
	return s.welfareRepo.List(ctx, params, filter)
}
