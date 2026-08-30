package service

import (
	"context"
	cryptorand "crypto/rand"
	"fmt"
	"io"
	"math"
	"math/big"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
)

var (
	ErrDailyCheckinQQRequired = infraerrors.Forbidden(
		"DAILY_CHECKIN_QQ_REQUIRED",
		"QQ account binding is required for daily check-in",
	)
	ErrDailyCheckinAlreadyDone = infraerrors.Conflict(
		"DAILY_CHECKIN_ALREADY_DONE",
		"today's check-in has already been completed",
	)
	ErrDailyCheckinUnavailable = infraerrors.BadRequest(
		"DAILY_CHECKIN_UNAVAILABLE",
		"daily check-in service is unavailable",
	)
)

type DailyCheckinRecord struct {
	ID          int64     `json:"id"`
	UserID      int64     `json:"user_id"`
	CheckinDate time.Time `json:"checkin_date"`
	Timezone    string    `json:"timezone"`
	BaseReward  float64   `json:"base_reward"`
	BonusReward float64   `json:"bonus_reward"`
	TotalReward float64   `json:"total_reward"`
	StreakDays  int       `json:"streak_days"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type DailyCheckinRule struct {
	Threshold int     `json:"threshold"`
	Bonus     float64 `json:"bonus"`
}

type DailyCheckinSummary struct {
	Timezone          string                    `json:"timezone"`
	Today             string                    `json:"today"`
	QQBound           bool                      `json:"qq_bound"`
	WechatBound       bool                      `json:"wechat_bound"`
	CanCheckIn        bool                      `json:"can_check_in"`
	CheckedInToday    bool                      `json:"checked_in_today"`
	StreakDays        int                       `json:"streak_days"`
	ThisMonthCount    int                       `json:"this_month_count"`
	TotalReward       float64                   `json:"total_reward"`
	BaseReward        float64                   `json:"base_reward"`
	BaseRewardMin     float64                   `json:"base_reward_min"`
	BaseRewardMax     float64                   `json:"base_reward_max"`
	BonusReward       float64                   `json:"bonus_reward"`
	TodayReward       float64                   `json:"today_reward"`
	TodayRewardMin    float64                   `json:"today_reward_min"`
	TodayRewardMax    float64                   `json:"today_reward_max"`
	RewardRanges      []DailyCheckinRewardRange `json:"reward_ranges"`
	RewardRules       []DailyCheckinRule        `json:"reward_rules"`
	RewardCycleNumber int                       `json:"reward_cycle_number"`
	RewardCycleDays   int                       `json:"reward_cycle_days"`
	RewardCycleDay    int                       `json:"reward_cycle_day"`
	Balance           float64                   `json:"balance"`
	RecentRecords     []DailyCheckinRecord      `json:"recent_records"`
}

type DailyCheckinStatus struct {
	Summary DailyCheckinSummary `json:"summary"`
	Balance float64             `json:"balance"`
}

type DailyCheckinResult struct {
	Record  DailyCheckinRecord  `json:"record"`
	Summary DailyCheckinSummary `json:"summary"`
	Balance float64             `json:"balance"`
}

type DailyCheckinHistoryPage struct {
	Items []DailyCheckinRecord `json:"items"`
	Total int64                `json:"total"`
	Page  int                  `json:"page"`
	Pages int                  `json:"pages"`
}

type dailyCheckinTxRunner interface {
	WithDailyCheckinTx(ctx context.Context, fn func(txCtx context.Context) error) error
}

type dailyCheckinRepository interface {
	HasUserQQ(ctx context.Context, userID int64) (bool, error)
	GetByID(ctx context.Context, id int64) (*User, error)
	UpdateBalance(ctx context.Context, id int64, amount float64) error
	ListRecentDailyCheckinRecords(ctx context.Context, userID int64, limit int) ([]DailyCheckinRecord, error)
	ListDailyCheckinRecords(ctx context.Context, userID int64, page, pageSize int) ([]DailyCheckinRecord, int64, error)
	GetDailyCheckinRecordByDate(ctx context.Context, userID int64, checkinDate time.Time) (*DailyCheckinRecord, error)
	CreateDailyCheckinRecord(ctx context.Context, record *DailyCheckinRecord) error
}

func (s *UserService) CheckInDaily(ctx context.Context, userID int64, userTZ string) (*DailyCheckinResult, error) {
	if s == nil || s.userRepo == nil {
		return nil, ErrDailyCheckinUnavailable
	}

	repo, ok := s.userRepo.(dailyCheckinRepository)
	if !ok {
		return nil, ErrDailyCheckinUnavailable
	}

	tz := strings.TrimSpace(userTZ)
	if tz == "" {
		tz = timezone.Name()
	}
	settings, err := s.dailyCheckinSettings(ctx)
	if err != nil {
		return nil, err
	}

	var result *DailyCheckinResult
	run := func(txCtx context.Context) error {
		var err error
		result, err = s.checkInDailyInTx(txCtx, repo, userID, tz, settings)
		return err
	}

	if txRunner, ok := s.userRepo.(dailyCheckinTxRunner); ok {
		if err := txRunner.WithDailyCheckinTx(ctx, run); err != nil {
			return nil, err
		}
		return result, nil
	}

	if err := run(ctx); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *UserService) checkInDailyInTx(ctx context.Context, repo dailyCheckinRepository, userID int64, userTZ string, settings DailyCheckinSettings) (*DailyCheckinResult, error) {
	user, err := repo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	qqBound, err := repo.HasUserQQ(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("check user qq binding: %w", err)
	}
	if !qqBound {
		return nil, ErrDailyCheckinQQRequired
	}

	now := timezone.NowInUserLocation(userTZ)
	today := localDateKey(now, userTZ)
	checkinDate := timezone.StartOfDayInUserLocation(now, userTZ)

	recent, err := repo.ListRecentDailyCheckinRecords(ctx, userID, 40)
	if err != nil {
		return nil, fmt.Errorf("list recent checkin records: %w", err)
	}

	if len(recent) > 0 && localDateKey(recent[0].CheckinDate, userTZ) == today {
		return nil, ErrDailyCheckinAlreadyDone
	}

	streakDays := computeCheckinStreak(recent, today, userTZ)
	baseReward, err := randomDailyCheckinBaseReward(settings)
	if err != nil {
		return nil, err
	}
	bonusReward := computeDailyCheckinBonus(streakDays, settings.CycleDays, settings.StreakRules)
	totalReward := baseReward + bonusReward

	record := &DailyCheckinRecord{
		UserID:      userID,
		CheckinDate: checkinDate,
		Timezone:    userTZ,
		BaseReward:  baseReward,
		BonusReward: bonusReward,
		TotalReward: totalReward,
		StreakDays:  streakDays,
	}

	if err := repo.UpdateBalance(ctx, userID, totalReward); err != nil {
		return nil, fmt.Errorf("update balance: %w", err)
	}
	if err := repo.CreateDailyCheckinRecord(ctx, record); err != nil {
		return nil, err
	}
	if err := recordTypedBalanceAdjustment(ctx, s.redeemCodeRepo, userID, totalReward, AdjustmentTypeDailyCheckin, balanceAdjustmentNote("每日签到", timezone.NowInUserLocation(userTZ))); err != nil {
		return nil, err
	}

	summary := DailyCheckinSummary{
		Timezone:          userTZ,
		Today:             today,
		QQBound:           true,
		WechatBound:       false,
		CanCheckIn:        false,
		CheckedInToday:    true,
		StreakDays:        streakDays,
		ThisMonthCount:    countCheckinsThisMonth(recent, today, record),
		TotalReward:       sumCheckinRewards(recent, record),
		BaseReward:        baseReward,
		BaseRewardMin:     settings.RewardMin,
		BaseRewardMax:     settings.RewardMax,
		BonusReward:       bonusReward,
		TodayReward:       totalReward,
		TodayRewardMin:    settings.RewardMin + bonusReward,
		TodayRewardMax:    settings.RewardMax + bonusReward,
		RewardRanges:      append([]DailyCheckinRewardRange(nil), settings.RewardRanges...),
		RewardRules:       sortedDailyCheckinRules(settings.StreakRules),
		RewardCycleNumber: dailyCheckinCycleNumber(streakDays, settings.CycleDays),
		RewardCycleDays:   settings.CycleDays,
		RewardCycleDay:    dailyCheckinCycleDay(streakDays, settings.CycleDays),
		Balance:           user.Balance + totalReward,
		RecentRecords:     prependRecord(recent, record, 7),
	}

	return &DailyCheckinResult{
		Record:  *record,
		Summary: summary,
		Balance: summary.Balance,
	}, nil
}

func (s *UserService) GetDailyCheckinStatus(ctx context.Context, userID int64, userTZ string) (*DailyCheckinStatus, error) {
	if s == nil || s.userRepo == nil {
		return nil, ErrDailyCheckinUnavailable
	}

	repo, ok := s.userRepo.(dailyCheckinRepository)
	if !ok {
		return nil, ErrDailyCheckinUnavailable
	}

	tz := strings.TrimSpace(userTZ)
	if tz == "" {
		tz = timezone.Name()
	}
	settings, err := s.dailyCheckinSettings(ctx)
	if err != nil {
		return nil, err
	}

	user, err := repo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}

	now := timezone.NowInUserLocation(tz)
	today := localDateKey(now, tz)

	recent, err := repo.ListRecentDailyCheckinRecords(ctx, userID, 40)
	if err != nil {
		return nil, fmt.Errorf("list recent checkin records: %w", err)
	}

	qqBound, err := repo.HasUserQQ(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("check user qq binding: %w", err)
	}
	checkedInToday := len(recent) > 0 && localDateKey(recent[0].CheckinDate, tz) == today
	streakDays := computeCheckinStreak(recent, today, tz)
	baseReward := settings.RewardMin
	bonusReward := computeDailyCheckinBonus(streakDays, settings.CycleDays, settings.StreakRules)
	totalReward := baseReward + bonusReward

	summary := DailyCheckinSummary{
		Timezone:          tz,
		Today:             today,
		QQBound:           qqBound,
		WechatBound:       false,
		CanCheckIn:        qqBound && !checkedInToday,
		CheckedInToday:    checkedInToday,
		StreakDays:        streakDays,
		ThisMonthCount:    countCheckinsThisMonth(recent, today, nil),
		TotalReward:       sumCheckinRewards(recent, nil),
		BaseReward:        baseReward,
		BaseRewardMin:     settings.RewardMin,
		BaseRewardMax:     settings.RewardMax,
		BonusReward:       bonusReward,
		TodayReward:       totalReward,
		TodayRewardMin:    settings.RewardMin + bonusReward,
		TodayRewardMax:    settings.RewardMax + bonusReward,
		RewardRanges:      append([]DailyCheckinRewardRange(nil), settings.RewardRanges...),
		RewardRules:       sortedDailyCheckinRules(settings.StreakRules),
		RewardCycleNumber: dailyCheckinCycleNumber(streakDays, settings.CycleDays),
		RewardCycleDays:   settings.CycleDays,
		RewardCycleDay:    dailyCheckinCycleDay(streakDays, settings.CycleDays),
		Balance:           user.Balance,
		RecentRecords:     prependRecord(recent, nil, 7),
	}

	return &DailyCheckinStatus{
		Summary: summary,
		Balance: user.Balance,
	}, nil
}

func (s *UserService) ListDailyCheckinHistory(ctx context.Context, userID int64, page, pageSize int, userTZ string) ([]DailyCheckinRecord, *pagination.PaginationResult, error) {
	if s == nil || s.userRepo == nil {
		return nil, nil, ErrDailyCheckinUnavailable
	}

	repo, ok := s.userRepo.(dailyCheckinRepository)
	if !ok {
		return nil, nil, ErrDailyCheckinUnavailable
	}

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	records, total, err := repo.ListDailyCheckinRecords(ctx, userID, page, pageSize)
	if err != nil {
		return nil, nil, err
	}
	pag := &pagination.PaginationResult{Total: total, Page: page, PageSize: pageSize}
	pag.Pages = int((total + int64(pageSize) - 1) / int64(pageSize))
	if pag.Pages < 1 {
		pag.Pages = 1
	}
	_ = userTZ
	return records, pag, nil
}

func randomDailyCheckinBaseReward(settings DailyCheckinSettings) (float64, error) {
	return randomDailyCheckinBaseRewardFromReader(cryptorand.Reader, settings)
}

func randomDailyCheckinBaseRewardFromReader(reader io.Reader, settings DailyCheckinSettings) (float64, error) {
	draw, err := cryptorand.Int(reader, big.NewInt(dailyCheckinProbabilityScale))
	if err != nil {
		return 0, fmt.Errorf("draw daily checkin reward probability: %w", err)
	}
	rewardRange := selectDailyCheckinRewardRange(draw.Int64(), settings.RewardRanges)
	return randomDailyCheckinRangeReward(reader, rewardRange, settings.RewardMax)
}

func selectDailyCheckinRewardRange(draw int64, ranges []DailyCheckinRewardRange) DailyCheckinRewardRange {
	threshold := int64(0)
	for index, rewardRange := range ranges {
		threshold += int64(math.Round(rewardRange.Probability * dailyCheckinProbabilityScale))
		if draw < threshold || index == len(ranges)-1 {
			return rewardRange
		}
	}
	return ranges[len(ranges)-1]
}

func randomDailyCheckinRangeReward(reader io.Reader, rewardRange DailyCheckinRewardRange, overallMax float64) (float64, error) {
	min := int64(math.Ceil(rewardRange.Min * dailyCheckinRewardCents))
	max := int64(math.Ceil(rewardRange.Max*dailyCheckinRewardCents)) - 1
	if rewardRange.Max == overallMax {
		max = int64(math.Floor(rewardRange.Max * dailyCheckinRewardCents))
	}
	if max < min {
		max = min
	}
	span := max - min + 1
	draw, err := cryptorand.Int(reader, big.NewInt(span))
	if err != nil {
		return 0, fmt.Errorf("draw daily checkin reward amount: %w", err)
	}
	return float64(min+draw.Int64()) / dailyCheckinRewardCents, nil
}

func computeDailyCheckinBonus(streakDays, cycleDays int, rules []DailyCheckinRule) float64 {
	cycleDay := dailyCheckinCycleDay(streakDays, cycleDays)
	if cycleDay == 0 {
		return 0
	}
	for _, rule := range rules {
		if cycleDay == rule.Threshold {
			return rule.Bonus
		}
	}
	return 0
}

func dailyCheckinCycleDay(streakDays, cycleDays int) int {
	if streakDays < 1 || cycleDays < 1 {
		return 0
	}
	return (streakDays-1)%cycleDays + 1
}

func dailyCheckinCycleNumber(streakDays, cycleDays int) int {
	if streakDays < 1 || cycleDays < 1 {
		return 0
	}
	return (streakDays-1)/cycleDays + 1
}

func computeCheckinStreak(records []DailyCheckinRecord, today string, userTZ string) int {
	if len(records) == 0 {
		return 1
	}

	latest := records[0]
	latestDate := localDateKey(latest.CheckinDate, userTZ)
	if latest.StreakDays < 1 {
		latest.StreakDays = 1
	}
	if latestDate == today {
		return latest.StreakDays
	}
	if latestDate == previousDateKey(today) {
		return latest.StreakDays + 1
	}
	return 1
}

func countCheckinsThisMonth(records []DailyCheckinRecord, today string, current *DailyCheckinRecord) int {
	month := monthKey(today)
	total := 0
	for _, record := range records {
		if monthKey(localDateKey(record.CheckinDate, record.Timezone)) == month {
			total++
		}
	}
	if current != nil && monthKey(localDateKey(current.CheckinDate, current.Timezone)) == month {
		total++
	}
	return total
}

func sumCheckinRewards(records []DailyCheckinRecord, current *DailyCheckinRecord) float64 {
	total := 0.0
	for _, record := range records {
		total += record.TotalReward
	}
	if current != nil {
		total += current.TotalReward
	}
	return total
}

func prependRecord(records []DailyCheckinRecord, current *DailyCheckinRecord, limit int) []DailyCheckinRecord {
	out := make([]DailyCheckinRecord, 0, min(limit, len(records)+1))
	if current != nil {
		out = append(out, *current)
	}
	for _, record := range records {
		if len(out) >= limit {
			break
		}
		out = append(out, record)
	}
	return out
}

func localDateKey(t time.Time, userTZ string) string {
	loc := timezone.Location()
	if userTZ != "" {
		if userLoc, err := time.LoadLocation(userTZ); err == nil {
			loc = userLoc
		}
	}
	return t.In(loc).Format("2006-01-02")
}

func previousDateKey(dateKey string) string {
	t, err := time.Parse("2006-01-02", dateKey)
	if err != nil {
		return dateKey
	}
	return t.AddDate(0, 0, -1).Format("2006-01-02")
}

func monthKey(dateKey string) string {
	if len(dateKey) < 7 {
		return dateKey
	}
	return dateKey[:7]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
