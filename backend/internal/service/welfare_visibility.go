package service

import "fmt"

const SettingKeyWelfareLeaderboardVisibleUserIDs = "welfare_leaderboard_visible_user_ids"

func ParseWelfareVisibleUserIDs(raw string) ([]int64, error) {
	ids, err := ParseWelfareExcludedUserIDs(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid welfare leaderboard visibility: %w", err)
	}
	return ids, nil
}

func marshalWelfareVisibleUserIDs(ids []int64) (string, error) {
	raw, err := marshalWelfareExcludedUserIDs(ids)
	if err != nil {
		return "", fmt.Errorf("invalid welfare leaderboard visibility: %w", err)
	}
	return raw, nil
}

func ValidateWelfareVisibility(excluded, visible []int64) error {
	if err := ValidateWelfareExcludedUserIDs(visible); err != nil {
		return fmt.Errorf("invalid welfare leaderboard visibility: %w", err)
	}
	excludedSet := make(map[int64]struct{}, len(excluded))
	for _, id := range excluded {
		excludedSet[id] = struct{}{}
	}
	for _, id := range visible {
		if _, ok := excludedSet[id]; !ok {
			return fmt.Errorf("leaderboard visibility user #%d must be in the welfare exclusion list", id)
		}
	}
	return nil
}
