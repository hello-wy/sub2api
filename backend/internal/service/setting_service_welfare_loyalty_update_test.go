//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestSettingService_UpdateSettings_PersistsWelfareAndLoyaltySettings(t *testing.T) {
	repo := &settingUpdateRepoStub{}
	svc := NewSettingService(repo, &config.Config{})

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		WelfareLeaderboardRankLimit:    5,
		WelfareLeaderboardRewardRatios: "[1.0,0.75,0.5,0.25,0.1]",
		LoyaltyWeeklyRules:             `[{"level":"Weekly","points":30,"discount":5}]`,
		LoyaltyPermanentRules:          `[{"level":"Permanent","points":300,"discount":15}]`,
		DailyCheckinRewardMin:          0,
		DailyCheckinRewardMax:          3,
		DailyCheckinRewardRanges:       `[{"min":0,"max":1,"probability":0.5},{"min":1,"max":3,"probability":0.5}]`,
		DailyCheckinStreakRules:        `[{"threshold":2,"bonus":1.5}]`,
	})

	require.NoError(t, err)
	require.Equal(t, "5", repo.updates[SettingKeyWelfareLeaderboardRankLimit])
	require.Equal(t, "[1.0,0.75,0.5,0.25,0.1]", repo.updates[SettingKeyWelfareLeaderboardRewardRatios])
	require.Equal(t, `[{"scope":"weekly","level":"Weekly","points":30,"discount":5}]`, repo.updates[SettingKeyLoyaltyWeeklyRules])
	require.Equal(t, `[{"scope":"permanent","level":"Permanent","points":300,"discount":15}]`, repo.updates[SettingKeyLoyaltyPermanentRules])
	require.Equal(t, "0.01", repo.updates[SettingKeyDailyCheckinRewardMin])
	require.Equal(t, "3", repo.updates[SettingKeyDailyCheckinRewardMax])
	require.Equal(t, `[{"min":0.01,"max":1,"probability":0.5},{"min":1,"max":3,"probability":0.5}]`, repo.updates[SettingKeyDailyCheckinRewardRanges])
	require.Equal(t, `[{"threshold":2,"bonus":1.5}]`, repo.updates[SettingKeyDailyCheckinStreakRules])
}
