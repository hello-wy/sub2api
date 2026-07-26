package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type dailyResetTrackingUserSubRepo struct {
	userSubRepoNoop

	resetDailyCalled bool
}

func (r *dailyResetTrackingUserSubRepo) ResetDailyUsage(context.Context, int64, *time.Time, time.Time) error {
	r.resetDailyCalled = true
	return nil
}

func TestAssignOrExtendSubscription_ExpiredDailyCardStartsNewOneTimeQuota(t *testing.T) {
	groupRepo := &subscriptionGroupRepoStub{
		group: &Group{ID: 1, SubscriptionType: SubscriptionTypeSubscription},
	}
	subRepo := newSubscriptionUserSubRepoStub()
	oldStart := time.Now().AddDate(0, 0, -3)
	oldWindowStart := startOfDay(oldStart)
	subRepo.seed(&UserSubscription{
		ID:                 100,
		UserID:             200,
		GroupID:            1,
		StartsAt:           oldStart,
		ExpiresAt:          oldStart.AddDate(0, 0, 1),
		Status:             SubscriptionStatusExpired,
		TermVersion:        3,
		DailyWindowStart:   &oldWindowStart,
		WeeklyWindowStart:  &oldWindowStart,
		MonthlyWindowStart: &oldWindowStart,
		DailyUsageUSD:      10,
		WeeklyUsageUSD:     20,
		MonthlyUsageUSD:    30,
		TotalUsageUSD:      40,
		Notes:              "old",
	})
	svc := NewSubscriptionService(groupRepo, subRepo, nil, nil, nil)

	renewed, reused, err := svc.AssignOrExtendSubscription(context.Background(), &AssignSubscriptionInput{
		UserID:       200,
		GroupID:      1,
		ValidityDays: 1,
		Notes:        "new",
	})

	require.NoError(t, err)
	require.True(t, reused)
	require.True(t, renewed.HasOneTimeDailyQuota(), "过期后重新购买 1 日卡仍应被识别为一次性日额度")
	require.Equal(t, SubscriptionStatusActive, renewed.Status)
	require.Equal(t, int64(4), renewed.TermVersion)
	require.True(t, renewed.StartsAt.After(oldStart), "重新购买过期订阅时应重置当前周期 StartsAt")
	require.False(t, renewed.ExpiresAt.After(renewed.StartsAt.AddDate(0, 0, 1)))
	require.NotNil(t, renewed.DailyWindowStart)
	require.Equal(t, startOfDay(renewed.StartsAt), *renewed.DailyWindowStart)
	require.Equal(t, 0.0, renewed.DailyUsageUSD)
	require.Equal(t, 0.0, renewed.WeeklyUsageUSD)
	require.Equal(t, 0.0, renewed.MonthlyUsageUSD)
	require.Equal(t, 0.0, renewed.TotalUsageUSD)
	require.Equal(t, "old\nnew", renewed.Notes)
}

func TestAssignOrExtendSubscription_ActiveSubscriptionIsOverwrittenFromNow(t *testing.T) {
	groupRepo := &subscriptionGroupRepoStub{
		group: &Group{ID: 1, SubscriptionType: SubscriptionTypeSubscription},
	}
	subRepo := newSubscriptionUserSubRepoStub()
	oldStart := time.Now().AddDate(0, 0, -10)
	oldExpiresAt := time.Now().AddDate(0, 0, 20)
	oldWindowStart := startOfDay(oldStart)
	subRepo.seed(&UserSubscription{
		ID:                 102,
		UserID:             202,
		GroupID:            1,
		StartsAt:           oldStart,
		ExpiresAt:          oldExpiresAt,
		Status:             SubscriptionStatusActive,
		TermVersion:        7,
		DailyWindowStart:   &oldWindowStart,
		WeeklyWindowStart:  &oldWindowStart,
		MonthlyWindowStart: &oldWindowStart,
		DailyUsageUSD:      10,
		WeeklyUsageUSD:     20,
		MonthlyUsageUSD:    30,
		TotalUsageUSD:      40,
	})
	svc := NewSubscriptionService(groupRepo, subRepo, nil, nil, nil)

	before := time.Now()
	overwritten, reused, err := svc.AssignOrExtendSubscription(context.Background(), &AssignSubscriptionInput{
		UserID:       202,
		GroupID:      1,
		ValidityDays: 7,
	})
	after := time.Now()

	require.NoError(t, err)
	require.True(t, reused)
	require.WithinDuration(t, before, overwritten.StartsAt, after.Sub(before)+time.Second)
	require.WithinDuration(t, overwritten.StartsAt.AddDate(0, 0, 7), overwritten.ExpiresAt, time.Second)
	require.True(t, overwritten.ExpiresAt.Before(oldExpiresAt), "新周期不得叠加旧订阅剩余时间")
	require.Equal(t, SubscriptionStatusActive, overwritten.Status)
	require.Equal(t, int64(8), overwritten.TermVersion)
	require.Zero(t, overwritten.DailyUsageUSD)
	require.Zero(t, overwritten.WeeklyUsageUSD)
	require.Zero(t, overwritten.MonthlyUsageUSD)
	require.Zero(t, overwritten.TotalUsageUSD)
}

func TestExtendSubscription_ExpiredSubscriptionStartsFreshTermAndClearsUsage(t *testing.T) {
	subRepo := newSubscriptionUserSubRepoStub()
	subRepo.seed(&UserSubscription{
		ID:              103,
		UserID:          203,
		GroupID:         1,
		StartsAt:        time.Now().AddDate(0, 0, -10),
		ExpiresAt:       time.Now().AddDate(0, 0, -2),
		Status:          SubscriptionStatusExpired,
		TermVersion:     11,
		DailyUsageUSD:   10,
		WeeklyUsageUSD:  20,
		MonthlyUsageUSD: 30,
		TotalUsageUSD:   40,
	})
	svc := NewSubscriptionService(groupRepoNoop{}, subRepo, nil, nil, nil)

	before := time.Now()
	renewed, err := svc.ExtendSubscription(context.Background(), 103, 5)
	after := time.Now()

	require.NoError(t, err)
	require.WithinDuration(t, before, renewed.StartsAt, after.Sub(before)+time.Second)
	require.WithinDuration(t, renewed.StartsAt.AddDate(0, 0, 5), renewed.ExpiresAt, time.Second)
	require.Equal(t, SubscriptionStatusActive, renewed.Status)
	require.Equal(t, int64(12), renewed.TermVersion)
	require.Zero(t, renewed.DailyUsageUSD)
	require.Zero(t, renewed.WeeklyUsageUSD)
	require.Zero(t, renewed.MonthlyUsageUSD)
	require.Zero(t, renewed.TotalUsageUSD)
}

func TestAssignOrExtendSubscription_ExpiredSubscriptionAppendsMatchingNotes(t *testing.T) {
	groupRepo := &subscriptionGroupRepoStub{
		group: &Group{ID: 1, SubscriptionType: SubscriptionTypeSubscription},
	}
	subRepo := newSubscriptionUserSubRepoStub()
	oldStart := time.Now().AddDate(0, 0, -3)
	subRepo.seed(&UserSubscription{
		ID:        101,
		UserID:    201,
		GroupID:   1,
		StartsAt:  oldStart,
		ExpiresAt: oldStart.AddDate(0, 0, 1),
		Status:    SubscriptionStatusExpired,
		Notes:     "same",
	})
	svc := NewSubscriptionService(groupRepo, subRepo, nil, nil, nil)

	renewed, reused, err := svc.AssignOrExtendSubscription(context.Background(), &AssignSubscriptionInput{
		UserID:       201,
		GroupID:      1,
		ValidityDays: 1,
		Notes:        "same",
	})

	require.NoError(t, err)
	require.True(t, reused)
	require.Equal(t, "same\nsame", renewed.Notes)
}

func TestUserSubscriptionNeedsDailyReset_DailyCardKeepsOneTimeQuota(t *testing.T) {
	start := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	dailyWindowStart := time.Date(2026, 5, 18, 0, 0, 0, 0, time.UTC)
	sub := &UserSubscription{
		StartsAt:         start,
		ExpiresAt:        start.Add(24 * time.Hour),
		DailyWindowStart: &dailyWindowStart,
		DailyUsageUSD:    10,
	}

	require.True(t, sub.HasOneTimeDailyQuota())
	require.False(t, sub.NeedsDailyResetAt(dailyWindowStart.Add(25*time.Hour)), "日卡应作为一次性配额，跨 0 点后不再刷新日额度")
}

func TestUserSubscriptionNeedsDailyReset_MultiDaySubscriptionStillRefreshes(t *testing.T) {
	start := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	dailyWindowStart := time.Date(2026, 5, 18, 0, 0, 0, 0, time.UTC)
	sub := &UserSubscription{
		StartsAt:         start,
		ExpiresAt:        start.AddDate(0, 0, 2),
		DailyWindowStart: &dailyWindowStart,
	}

	require.False(t, sub.HasOneTimeDailyQuota())
	require.True(t, sub.NeedsDailyResetAt(dailyWindowStart.Add(24*time.Hour)), "多日订阅仍应按 24 小时日窗口刷新")
}

func TestUserSubscriptionDailyResetTime_DailyCardReturnsExpiry(t *testing.T) {
	start := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	dailyWindowStart := time.Date(2026, 5, 18, 0, 0, 0, 0, time.UTC)
	expiresAt := start.Add(24 * time.Hour)
	sub := &UserSubscription{
		StartsAt:         start,
		ExpiresAt:        expiresAt,
		DailyWindowStart: &dailyWindowStart,
	}

	resetAt := sub.DailyResetTime()
	require.NotNil(t, resetAt)
	require.Equal(t, expiresAt, *resetAt, "日卡展示的日额度结束时间应为订阅过期时间")
}

func TestCheckAndResetWindows_DailyCardDoesNotResetDailyUsage(t *testing.T) {
	now := time.Now()
	startsAt := now.Add(-23 * time.Hour)
	dailyWindowStart := now.Add(-25 * time.Hour)
	repo := &dailyResetTrackingUserSubRepo{}
	svc := NewSubscriptionService(groupRepoNoop{}, repo, nil, nil, nil)
	sub := &UserSubscription{
		ID:               1,
		UserID:           10,
		GroupID:          20,
		StartsAt:         startsAt,
		ExpiresAt:        startsAt.Add(24 * time.Hour),
		DailyUsageUSD:    10,
		DailyWindowStart: &dailyWindowStart,
	}

	err := svc.CheckAndResetWindows(context.Background(), sub)

	require.NoError(t, err)
	require.False(t, repo.resetDailyCalled, "日卡作为一次性配额，过了 24 小时日窗口也不应重置 daily usage")
	require.Equal(t, 10.0, sub.DailyUsageUSD)
}

func TestCheckAndResetWindows_MultiDaySubscriptionStillResetsDailyUsage(t *testing.T) {
	now := time.Now()
	startsAt := now.Add(-48 * time.Hour)
	dailyWindowStart := now.Add(-25 * time.Hour)
	repo := &dailyResetTrackingUserSubRepo{}
	svc := NewSubscriptionService(groupRepoNoop{}, repo, nil, nil, nil)
	sub := &UserSubscription{
		ID:               1,
		UserID:           10,
		GroupID:          20,
		StartsAt:         startsAt,
		ExpiresAt:        startsAt.AddDate(0, 0, 2),
		DailyUsageUSD:    10,
		DailyWindowStart: &dailyWindowStart,
	}

	err := svc.CheckAndResetWindows(context.Background(), sub)

	require.NoError(t, err)
	require.True(t, repo.resetDailyCalled, "多日订阅仍应重置过期 daily window")
	require.Equal(t, 0.0, sub.DailyUsageUSD)
}

func TestCheckAndResetWindows_SubscriptionLifetimeQuotaDoesNotReset(t *testing.T) {
	now := time.Now()
	startsAt := now.Add(-48 * time.Hour)
	dailyWindowStart := now.Add(-25 * time.Hour)
	repo := &dailyResetTrackingUserSubRepo{}
	svc := NewSubscriptionService(groupRepoNoop{}, repo, nil, nil, nil)
	sub := &UserSubscription{
		ID:               1,
		UserID:           10,
		GroupID:          20,
		StartsAt:         startsAt,
		ExpiresAt:        now.AddDate(0, 0, 2),
		DailyWindowStart: &dailyWindowStart,
		DailyUsageUSD:    10,
	}
	group := &Group{
		SubscriptionType:           SubscriptionTypeSubscription,
		SubscriptionQuotaResetMode: SubscriptionQuotaResetModeUntilSubscriptionExpires,
	}

	err := svc.CheckAndResetWindows(context.Background(), sub, group)

	require.NoError(t, err)
	require.False(t, repo.resetDailyCalled, "套餐总额度模式在到期前不得重置日额度")
	require.Equal(t, 10.0, sub.DailyUsageUSD)
}

func TestValidateAndCheckLimits_SubscriptionLifetimeQuotaDoesNotRestoreUsage(t *testing.T) {
	now := time.Now()
	totalLimit := 10.0
	dailyWindowStart := now.Add(-25 * time.Hour)
	sub := &UserSubscription{
		Status:           SubscriptionStatusActive,
		StartsAt:         now.Add(-48 * time.Hour),
		ExpiresAt:        now.AddDate(0, 0, 2),
		DailyWindowStart: &dailyWindowStart,
		DailyUsageUSD:    999, // 总额度模式下不读取历史日用量。
		TotalUsageUSD:    totalLimit + 0.01,
	}
	group := &Group{
		SubscriptionType:           SubscriptionTypeSubscription,
		SubscriptionQuotaResetMode: SubscriptionQuotaResetModeUntilSubscriptionExpires,
		SubscriptionTotalLimitUSD:  &totalLimit,
	}
	svc := NewSubscriptionService(groupRepoNoop{}, userSubRepoNoop{}, nil, nil, nil)

	needsMaintenance, err := svc.ValidateAndCheckLimits(sub, group)

	require.False(t, needsMaintenance, "套餐总额度模式不应因窗口到期触发重置")
	require.True(t, errors.Is(err, ErrSubscriptionTotalLimitExceeded))
	require.Equal(t, totalLimit+0.01, sub.TotalUsageUSD)
}

func TestValidateAndCheckLimits_DailyCardDoesNotAllowSecondQuotaAfterMidnight(t *testing.T) {
	start := time.Now().Add(-23 * time.Hour)
	dailyWindowStart := time.Now().Add(-25 * time.Hour)
	dailyLimit := 10.0
	sub := &UserSubscription{
		Status:           SubscriptionStatusActive,
		StartsAt:         start,
		ExpiresAt:        start.Add(24 * time.Hour),
		DailyWindowStart: &dailyWindowStart,
		DailyUsageUSD:    dailyLimit + 0.01,
	}
	group := &Group{
		SubscriptionType: SubscriptionTypeSubscription,
		DailyLimitUSD:    &dailyLimit,
	}
	svc := NewSubscriptionService(groupRepoNoop{}, userSubRepoNoop{}, nil, nil, nil)

	needsMaintenance, err := svc.ValidateAndCheckLimits(sub, group)

	require.False(t, needsMaintenance, "日卡跨过日窗口后不应触发 daily reset 维护")
	require.True(t, errors.Is(err, ErrDailyLimitExceeded))
	require.Equal(t, dailyLimit+0.01, sub.DailyUsageUSD, "热路径不应清零日卡已用额度")
}

func TestValidateAndCheckLimits_RollingZeroLimitDeniesUsage(t *testing.T) {
	zero := 0.0
	now := time.Now()
	tests := []struct {
		name    string
		apply   func(*Group)
		wantErr error
	}{
		{
			name: "daily",
			apply: func(group *Group) {
				group.DailyLimitUSD = &zero
			},
			wantErr: ErrDailyLimitExceeded,
		},
		{
			name: "weekly",
			apply: func(group *Group) {
				group.WeeklyLimitUSD = &zero
			},
			wantErr: ErrWeeklyLimitExceeded,
		},
		{
			name: "monthly",
			apply: func(group *Group) {
				group.MonthlyLimitUSD = &zero
			},
			wantErr: ErrMonthlyLimitExceeded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			windowStart := now
			sub := &UserSubscription{
				Status:             SubscriptionStatusActive,
				ExpiresAt:          now.Add(time.Hour),
				DailyWindowStart:   &windowStart,
				WeeklyWindowStart:  &windowStart,
				MonthlyWindowStart: &windowStart,
			}
			group := &Group{
				SubscriptionType:           SubscriptionTypeSubscription,
				SubscriptionQuotaResetMode: SubscriptionQuotaResetModeRolling,
			}
			tt.apply(group)
			svc := NewSubscriptionService(groupRepoNoop{}, userSubRepoNoop{}, nil, nil, nil)

			needsMaintenance, err := svc.ValidateAndCheckLimits(sub, group)

			require.False(t, needsMaintenance)
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}
