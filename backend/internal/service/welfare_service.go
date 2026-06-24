package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
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

type WelfareRecord struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	UserEmail string    `json:"user_email"`
	Amount    float64   `json:"amount"`
	Remarks   string    `json:"remarks"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type WelfareRepository interface {
	Create(ctx context.Context, userID int64, email string, amount float64, remarks string) (*WelfareRecord, error)
	GetByID(ctx context.Context, id int64) (*WelfareRecord, error)
	MarkRevoked(ctx context.Context, id int64) (bool, error)
	ExistsSuccessByRemarks(ctx context.Context, remarks string) (bool, error)
	List(ctx context.Context, params pagination.PaginationParams, searchEmail string) ([]WelfareRecord, *pagination.PaginationResult, error)
}

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

// Start starts the daily 23:55 cron job.
func (s *WelfareService) Start() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started || s.stopped {
		return
	}
	s.started = true

	// Schedule for 23:55 every day
	schedule := "55 23 * * *"
	loc := timezone.Location()

	c := cron.New(cron.WithParser(welfareCronParser), cron.WithLocation(loc))
	if _, err := c.AddFunc(schedule, func() { s.RunRewardJob(context.Background()) }); err != nil {
		slog.Error("[WelfareService] failed to schedule cron job", "error", err)
		return
	}
	c.Start()
	s.cron = c
	slog.Info("[WelfareService] scheduled reward job successfully", "schedule", schedule, "timezone", loc.String())
}

// Stop stops the cron job.
func (s *WelfareService) Stop() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return
	}
	s.stopped = true
	if s.cron != nil {
		ctx := s.cron.Stop()
		select {
		case <-ctx.Done():
		case <-time.After(welfareCronStopTimeout):
			slog.Warn("[WelfareService] cron stop timed out")
		}
		s.cron = nil
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
		remarks := fmt.Sprintf("%s 排行榜消费 #%d", todayStr, i+1)
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

func (s *WelfareService) loadRewardSettings(ctx context.Context) (int, []float64) {
	limitRaw, _ := s.settingRepo.GetValue(ctx, SettingKeyWelfareLeaderboardRankLimit)
	ratiosRaw, _ := s.settingRepo.GetValue(ctx, SettingKeyWelfareLeaderboardRewardRatios)
	return parseWelfareRewardSettings(limitRaw, ratiosRaw)
}

func parseWelfareRewardSettings(limitRaw string, ratiosRaw string) (int, []float64) {
	limit := welfareDefaultRankLimit
	if parsed, err := strconv.Atoi(limitRaw); err == nil && parsed >= 1 {
		limit = parsed
	}
	ratios := append([]float64(nil), welfareDefaultRewardRatios...)
	if parsed := parseWelfareRatios(ratiosRaw); len(parsed) > 0 {
		ratios = parsed
	}
	return limit, resizeWelfareRatios(limit, ratios)
}

func parseWelfareRatios(raw string) []float64 {
	var parsed []float64
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil
	}
	out := make([]float64, 0, len(parsed))
	for _, ratio := range parsed {
		if ratio >= 0 {
			out = append(out, ratio)
		}
	}
	return out
}

func resizeWelfareRatios(limit int, ratios []float64) []float64 {
	if len(ratios) >= limit {
		return append([]float64(nil), ratios[:limit]...)
	}
	out := append([]float64(nil), ratios...)
	for len(out) < limit {
		out = append(out, out[len(out)-1])
	}
	return out
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
	return record, nil
}

// RevokeWelfareRecord revokes a welfare record, deducts the amount from the user's balance, and updates the status to "revoked".
func (s *WelfareService) RevokeWelfareRecord(ctx context.Context, recordID int64) error {
	record, err := s.welfareRepo.GetByID(ctx, recordID)
	if err != nil {
		return fmt.Errorf("failed to find welfare record: %w", err)
	}

	if record.Status == "revoked" {
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
	updated, err := s.welfareRepo.MarkRevoked(txCtx, record.ID)
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

// ListWelfareRecords lists all welfare records with paginated parameters and optional email filter.
func (s *WelfareService) ListWelfareRecords(ctx context.Context, params pagination.PaginationParams, searchEmail string) ([]WelfareRecord, *pagination.PaginationResult, error) {
	return s.welfareRepo.List(ctx, params, searchEmail)
}

func (s *WelfareService) tryAcquireLeaderLock(ctx context.Context) (func(), bool) {
	if s == nil {
		return nil, false
	}
	if s.cfg != nil && s.cfg.RunMode == config.RunModeSimple {
		return nil, true
	}

	key := welfareLeaderLockKey
	ttl := welfareLeaderLockTTL

	if s.lockCache != nil {
		ok, err := s.lockCache.TryAcquireLeaderLock(ctx, key, s.instanceID, ttl)
		if err == nil {
			if !ok {
				return nil, false
			}
			return func() {
				_ = s.lockCache.ReleaseLeaderLock(context.Background(), key, s.instanceID)
			}, true
		}
		s.warnNoRedisOnce.Do(func() {
			logger.LegacyPrintf("service.welfare", "[WelfareService] leader lock failed; falling back to DB advisory lock: %v", err)
		})
	}

	return tryAcquireDBAdvisoryLock(ctx, s.db, welfareAdvisoryLockID)
}
