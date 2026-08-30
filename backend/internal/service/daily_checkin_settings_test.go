package service

import "testing"

func TestDailyCheckinRewardMinimum(t *testing.T) {
	defaults := defaultDailyCheckinSettings()
	if defaults.RewardMin != dailyCheckinRewardMinimum {
		t.Fatalf("default reward minimum = %v, want %v", defaults.RewardMin, dailyCheckinRewardMinimum)
	}
	if defaults.RewardRanges[0].Min != dailyCheckinRewardMinimum {
		t.Fatalf("default first reward range minimum = %v, want %v", defaults.RewardRanges[0].Min, dailyCheckinRewardMinimum)
	}
	if defaults.CycleDays != 30 {
		t.Fatalf("default cycle days = %d, want 30", defaults.CycleDays)
	}

	settings, err := ParseDailyCheckinSettings(map[string]string{
		SettingKeyDailyCheckinRewardMin:    "0",
		SettingKeyDailyCheckinRewardMax:    "3",
		SettingKeyDailyCheckinRewardRanges: `[{"min":0,"max":1,"probability":1}]`,
	})
	if err != nil {
		t.Fatalf("legacy zero minimum should be normalized: %v", err)
	}
	if settings.RewardMin != dailyCheckinRewardMinimum || settings.RewardRanges[0].Min != dailyCheckinRewardMinimum {
		t.Fatalf("normalized settings = %+v, want minimum %v", settings, dailyCheckinRewardMinimum)
	}
}

func TestDailyCheckinSettingsRejectRuleBeyondCycle(t *testing.T) {
	_, err := ParseDailyCheckinSettings(map[string]string{
		SettingKeyDailyCheckinRewardMin:    "0.01",
		SettingKeyDailyCheckinRewardMax:    "3",
		SettingKeyDailyCheckinRewardRanges: `[{"min":0.01,"max":3,"probability":1}]`,
		SettingKeyDailyCheckinStreakRules:  `[{"threshold":31,"bonus":1}]`,
		SettingKeyDailyCheckinCycleDays:    "30",
	})
	if err == nil {
		t.Fatal("expected rule beyond cycle to be rejected")
	}
}
