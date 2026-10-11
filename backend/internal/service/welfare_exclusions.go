package service

import (
	"encoding/json"
	"fmt"
)

const SettingKeyWelfareExcludedUserIDs = "welfare_excluded_user_ids"

// ParseWelfareExcludedUserIDs accepts only positive, unique user IDs.
func ParseWelfareExcludedUserIDs(raw string) ([]int64, error) {
	if raw == "" {
		return []int64{}, nil
	}
	var ids []int64
	if err := json.Unmarshal([]byte(raw), &ids); err != nil {
		return nil, fmt.Errorf("invalid welfare excluded user IDs: %w", err)
	}
	if ids == nil {
		return nil, fmt.Errorf("welfare_excluded_user_ids must be an array")
	}
	if err := ValidateWelfareExcludedUserIDs(ids); err != nil {
		return nil, err
	}
	return ids, nil
}

func ValidateWelfareExcludedUserIDs(ids []int64) error {
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return fmt.Errorf("welfare_excluded_user_ids must contain positive user IDs")
		}
		if _, exists := seen[id]; exists {
			return fmt.Errorf("duplicate welfare excluded user ID: %d", id)
		}
		seen[id] = struct{}{}
	}
	return nil
}

func marshalWelfareExcludedUserIDs(ids []int64) (string, error) {
	if err := ValidateWelfareExcludedUserIDs(ids); err != nil {
		return "", err
	}
	if ids == nil {
		return "[]", nil
	}
	raw, err := json.Marshal(ids)
	return string(raw), err
}
