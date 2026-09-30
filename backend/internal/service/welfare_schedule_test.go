package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/stretchr/testify/require"
)

func TestValidateWelfareRewardTime(t *testing.T) {
	for _, value := range []string{"00:00", "23:55", "12:30", "23:59"} {
		parsed, err := ValidateWelfareRewardTime(value)
		require.NoError(t, err)
		require.Equal(t, value, parsed)
	}
	for _, value := range []string{"", "0:05", "24:00", "12:60", "12:30:00"} {
		_, err := ValidateWelfareRewardTime(value)
		require.Error(t, err, value)
	}
}

func TestWelfareRewardTimeChangesWithoutRestart(t *testing.T) {
	settings := ticketRebateSettingStub{values: map[string]string{}}
	svc := &WelfareService{settingRepo: settings}
	now := time.Date(2026, time.September, 30, 23, 55, 0, 0, timezone.Location())
	due, err := svc.scheduledRewardDue(context.Background(), now)
	require.NoError(t, err)
	require.True(t, due)

	settings.values[SettingKeyWelfareRewardTime] = "22:30"
	due, err = svc.scheduledRewardDue(context.Background(), now)
	require.NoError(t, err)
	require.False(t, due)
	due, err = svc.scheduledRewardDue(context.Background(), now.Add(-85*time.Minute))
	require.NoError(t, err)
	require.True(t, due)
}

func TestDailyRewardWindowUsesCurrentDay(t *testing.T) {
	for _, hourAndMinute := range [][2]int{{23, 55}, {0, 5}} {
		now := time.Date(2026, time.September, 30, hourAndMinute[0], hourAndMinute[1], 0, 0, timezone.Location())
		day, end := dailyRewardWindow(now)
		require.Equal(t, time.Date(2026, time.September, 30, 0, 0, 0, 0, timezone.Location()), day)
		require.Equal(t, now, end)
	}
}
