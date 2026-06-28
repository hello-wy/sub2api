package service

import (
	"context"
	cryptorand "crypto/rand"
	"fmt"
	"io"
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

const (
	dailyCheckinBaseRewardMin   = 1.0
	dailyCheckinBaseRewardMax   = 3.0
	dailyCheckinRewardCents     = 100
	dailyCheckinCycleDays       = 30
	dailyCheckinBonus3Days      = 3.0
	dailyCheckinBonus7Days      = 6.0
	dailyCheckinBonus14Days     = 12.0
	dailyCheckinBonus30Days     = 24.0
	dailyCheckinThreshold3Days  = 3
	dailyCheckinThreshold7Days  = 7
	dailyCheckinThreshold14Days = 14
	dailyCheckinThreshold30Days = 30
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
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type DailyCheckinRule struct {
	Threshold int     `json:"threshold"`
	Bonus     float64 `json:"bonus"`
}

var dailyCheckinRules = []DailyCheckinRule{
	{Threshold: dailyCheckinThreshold3Days, Bonus: dailyCheckinBonus3Days},
	{Threshold: dailyCheckinThreshold7Days, Bonus: dailyCheckinBonus7Days},
	{Threshold: dailyCheckinThreshold14Days, Bonus: dailyCheckinBonus14Days},
	{Threshold: dailyCheckinThreshold30Days, Bonus: dailyCheckinBonus30Days},
}

type DailyCheckinSummary struct {
	Timezone       string               `json:"timezone"`
	Today          string               `json:"today"`
	QQBound        bool                 `json:"qq_bound"`
	WechatBound    bool                 `json:"wechat_bound"`
	CanCheckIn     bool                 `json:"can_check_in"`
	CheckedInToday bool                 `json:"checked_in_today"`
	StreakDays     int                  `json:"streak_days"`
	ThisMonthCount int                  `json:"this_month_count"`
	TotalReward    float64              `json:"total_reward"`
	BaseReward     float64              `json:"base_reward"`
	BaseRewardMin  float64              `json:"base_reward_min"`
	BaseRewardMax  float64              `json:"base_reward_max"`
	BonusReward    float64              `json:"bonus_reward"`
	TodayReward    float64              `json:"today_reward"`
	TodayRewardMin float64              `json:"today_reward_min"`
	TodayRewardMax float64              `json:"today_reward_max"`
	Balance        float64              `json:"balance"`
	RecentRecords  []DailyCheckinRecord `json:"recent_records"`
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

	var result *DailyCheckinResult
	run := func(txCtx context.Context) error {
		var err error
		result, err = s.checkInDailyInTx(txCtx, repo, userID, tz)
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

func (s *UserService) checkInDailyInTx(ctx context.Context, repo dailyCheckinRepository, userID int64, userTZ string) (*DailyCheckinResult, error) {
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
	baseReward := randomDailyCheckinBaseReward()
	bonusReward := computeDailyCheckinBonus(streakDays)
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

	summary := DailyCheckinSummary{
		Timezone:       userTZ,
		Today:          today,
		QQBound:        true,
		WechatBound:    false,
		CanCheckIn:     false,
		CheckedInToday: true,
		StreakDays:     streakDays,
		ThisMonthCount: countCheckinsThisMonth(recent, today, record),
		TotalReward:    sumCheckinRewards(recent, record),
		BaseReward:     baseReward,
		BaseRewardMin:  dailyCheckinBaseRewardMin,
		BaseRewardMax:  dailyCheckinBaseRewardMax,
		BonusReward:    bonusReward,
		TodayReward:    totalReward,
		TodayRewardMin: dailyCheckinBaseRewardMin + bonusReward,
		TodayRewardMax: dailyCheckinBaseRewardMax + bonusReward,
		Balance:        user.Balance + totalReward,
		RecentRecords:  prependRecord(recent, record, 7),
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
	baseReward := dailyCheckinBaseRewardMin
	bonusReward := computeDailyCheckinBonus(streakDays)
	totalReward := baseReward + bonusReward

	summary := DailyCheckinSummary{
		Timezone:       tz,
		Today:          today,
		QQBound:        qqBound,
		WechatBound:    false,
		CanCheckIn:     qqBound && !checkedInToday,
		CheckedInToday: checkedInToday,
		StreakDays:     streakDays,
		ThisMonthCount: countCheckinsThisMonth(recent, today, nil),
		TotalReward:    sumCheckinRewards(recent, nil),
		BaseReward:     baseReward,
		BaseRewardMin:  dailyCheckinBaseRewardMin,
		BaseRewardMax:  dailyCheckinBaseRewardMax,
		BonusReward:    bonusReward,
		TodayReward:    totalReward,
		TodayRewardMin: dailyCheckinBaseRewardMin + bonusReward,
		TodayRewardMax: dailyCheckinBaseRewardMax + bonusReward,
		Balance:        user.Balance,
		RecentRecords:  prependRecord(recent, nil, 7),
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

func randomDailyCheckinBaseReward() float64 {
	return randomDailyCheckinBaseRewardFromReader(cryptorand.Reader)
}

func randomDailyCheckinBaseRewardFromReader(reader io.Reader) float64 {
	min := int64(dailyCheckinBaseRewardMin * dailyCheckinRewardCents)
	max := int64(dailyCheckinBaseRewardMax * dailyCheckinRewardCents)
	span := max - min + 1
	if span <= 1 {
		return float64(min) / dailyCheckinRewardCents
	}
	n, err := cryptorand.Int(reader, big.NewInt(span))
	if err != nil {
		return dailyCheckinBaseRewardMin
	}
	return float64(min+n.Int64()) / dailyCheckinRewardCents
}

func computeDailyCheckinBonus(streakDays int) float64 {
	bonus := 0.0
	for _, rule := range dailyCheckinRules {
		if streakDays >= rule.Threshold {
			bonus += rule.Bonus
		}
	}
	return bonus
}

func computeCheckinStreak(records []DailyCheckinRecord, today string, userTZ string) int {
	streak := 0
	expected := previousDateKey(today)
	for _, record := range records {
		if localDateKey(record.CheckinDate, userTZ) != expected {
			break
		}
		streak++
		expected = previousDateKey(expected)
	}
	if streak == 0 {
		return 1
	}
	return streak + 1
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
