package admin

import "github.com/Wei-Shaw/sub2api/internal/service"

func resolveWelfareVisibility(req UpdateSettingsRequest, previous *service.SystemSettings, omitted service.OmittedSettingKeys) ([]int64, error) {
	excluded := req.WelfareExcludedUserIDs
	if _, ok := omitted[service.SettingKeyWelfareExcludedUserIDs]; ok {
		excluded = previous.WelfareExcludedUserIDs
	}
	if _, ok := omitted[service.SettingKeyWelfareLeaderboardVisibleUserIDs]; !ok {
		return req.WelfareLeaderboardVisibleUserIDs, service.ValidateWelfareVisibility(excluded, req.WelfareLeaderboardVisibleUserIDs)
	}
	// Removing an exclusion also removes its saved visibility preference.
	visible := make([]int64, 0, len(previous.WelfareLeaderboardVisibleUserIDs))
	excludedSet := make(map[int64]struct{}, len(excluded))
	for _, id := range excluded {
		excludedSet[id] = struct{}{}
	}
	for _, id := range previous.WelfareLeaderboardVisibleUserIDs {
		if _, ok := excludedSet[id]; ok {
			visible = append(visible, id)
		}
	}
	return visible, nil
}
