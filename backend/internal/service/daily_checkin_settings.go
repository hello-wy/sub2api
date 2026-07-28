package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
)

const (
	dailyCheckinRewardCents        = 100
	dailyCheckinProbabilityScale   = 1_000_000
	dailyCheckinProbabilityEpsilon = 0.0000001
)

type DailyCheckinRewardRange struct {
	Min         float64 `json:"min"`
	Max         float64 `json:"max"`
	Probability float64 `json:"probability"`
}

type DailyCheckinSettings struct {
	RewardMin    float64                   `json:"reward_min"`
	RewardMax    float64                   `json:"reward_max"`
	RewardRanges []DailyCheckinRewardRange `json:"reward_ranges"`
	StreakRules  []DailyCheckinRule        `json:"streak_rules"`
}

func defaultDailyCheckinSettings() DailyCheckinSettings {
	return DailyCheckinSettings{
		RewardMin: 0,
		RewardMax: 3,
		RewardRanges: []DailyCheckinRewardRange{
			{Min: 0, Max: 1, Probability: 0.5},
			{Min: 1, Max: 2, Probability: 0.4},
			{Min: 2, Max: 2.5, Probability: 0.0999},
			{Min: 2.5, Max: 3, Probability: 0.0001},
		},
		StreakRules: []DailyCheckinRule{
			{Threshold: 3, Bonus: 3},
			{Threshold: 7, Bonus: 6},
			{Threshold: 14, Bonus: 12},
			{Threshold: 30, Bonus: 24},
		},
	}
}

func (s *UserService) dailyCheckinSettings(ctx context.Context) (DailyCheckinSettings, error) {
	if s == nil || s.settingRepo == nil {
		return defaultDailyCheckinSettings(), nil
	}

	values, err := s.settingRepo.GetMultiple(ctx, []string{
		SettingKeyDailyCheckinRewardMin,
		SettingKeyDailyCheckinRewardMax,
		SettingKeyDailyCheckinRewardRanges,
		SettingKeyDailyCheckinStreakRules,
	})
	if err != nil {
		return DailyCheckinSettings{}, fmt.Errorf("load daily checkin settings: %w", err)
	}
	return ParseDailyCheckinSettings(values)
}

func ParseDailyCheckinSettings(values map[string]string) (DailyCheckinSettings, error) {
	if len(values) == 0 {
		return defaultDailyCheckinSettings(), nil
	}

	defaults := defaultDailyCheckinSettings()
	settings := DailyCheckinSettings{RewardMin: defaults.RewardMin, RewardMax: defaults.RewardMax}
	if raw := strings.TrimSpace(values[SettingKeyDailyCheckinRewardMin]); raw != "" {
		if _, err := fmt.Sscan(raw, &settings.RewardMin); err != nil {
			return DailyCheckinSettings{}, fmt.Errorf("parse daily checkin reward minimum: %w", err)
		}
	}
	if raw := strings.TrimSpace(values[SettingKeyDailyCheckinRewardMax]); raw != "" {
		if _, err := fmt.Sscan(raw, &settings.RewardMax); err != nil {
			return DailyCheckinSettings{}, fmt.Errorf("parse daily checkin reward maximum: %w", err)
		}
	}
	if raw := strings.TrimSpace(values[SettingKeyDailyCheckinRewardRanges]); raw != "" {
		if err := json.Unmarshal([]byte(raw), &settings.RewardRanges); err != nil {
			return DailyCheckinSettings{}, fmt.Errorf("parse daily checkin reward ranges: %w", err)
		}
	} else {
		settings.RewardRanges = defaults.RewardRanges
	}
	if raw := strings.TrimSpace(values[SettingKeyDailyCheckinStreakRules]); raw != "" {
		if err := json.Unmarshal([]byte(raw), &settings.StreakRules); err != nil {
			return DailyCheckinSettings{}, fmt.Errorf("parse daily checkin streak rules: %w", err)
		}
	} else {
		settings.StreakRules = defaults.StreakRules
	}
	if err := ValidateDailyCheckinSettings(settings); err != nil {
		return DailyCheckinSettings{}, err
	}
	return settings, nil
}

func ValidateDailyCheckinSettings(settings DailyCheckinSettings) error {
	if !isFiniteNonNegative(settings.RewardMin) || !isFiniteNonNegative(settings.RewardMax) || settings.RewardMax < settings.RewardMin {
		return fmt.Errorf("daily checkin reward range must be finite, non-negative, and have max >= min")
	}
	if len(settings.RewardRanges) == 0 {
		return fmt.Errorf("daily checkin reward ranges must not be empty")
	}
	if len(settings.RewardRanges) > 100 || len(settings.StreakRules) > 100 {
		return fmt.Errorf("daily checkin settings support at most 100 reward ranges and streak rules")
	}
	if err := validateDailyCheckinRewardRanges(settings); err != nil {
		return err
	}
	return validateDailyCheckinStreakRules(settings.StreakRules)
}

func validateDailyCheckinRewardRanges(settings DailyCheckinSettings) error {
	total := 0.0
	for _, rewardRange := range settings.RewardRanges {
		if !isFiniteNonNegative(rewardRange.Min) || !isFiniteNonNegative(rewardRange.Max) || rewardRange.Max < rewardRange.Min {
			return fmt.Errorf("daily checkin reward range bounds must be finite, non-negative, and max >= min")
		}
		if rewardRange.Min < settings.RewardMin || rewardRange.Max > settings.RewardMax {
			return fmt.Errorf("daily checkin reward ranges must stay within the configured reward minimum and maximum")
		}
		if !isFiniteNonNegative(rewardRange.Probability) {
			return fmt.Errorf("daily checkin reward probabilities must be finite and non-negative")
		}
		total += rewardRange.Probability
	}
	if math.Abs(total-1) > dailyCheckinProbabilityEpsilon {
		return fmt.Errorf("daily checkin reward probabilities must total 1")
	}
	return nil
}

func validateDailyCheckinStreakRules(rules []DailyCheckinRule) error {
	thresholds := make(map[int]struct{}, len(rules))
	for _, rule := range rules {
		if rule.Threshold < 1 || !isFiniteNonNegative(rule.Bonus) {
			return fmt.Errorf("daily checkin streak rules require a positive day count and non-negative reward")
		}
		if _, exists := thresholds[rule.Threshold]; exists {
			return fmt.Errorf("daily checkin streak rules must not repeat a day count")
		}
		thresholds[rule.Threshold] = struct{}{}
	}
	return nil
}

func isFiniteNonNegative(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0
}

func sortedDailyCheckinRules(rules []DailyCheckinRule) []DailyCheckinRule {
	out := append([]DailyCheckinRule(nil), rules...)
	sort.Slice(out, func(i, j int) bool { return out[i].Threshold < out[j].Threshold })
	return out
}

func formatDailyCheckinJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return "[]"
	}
	return string(data)
}
