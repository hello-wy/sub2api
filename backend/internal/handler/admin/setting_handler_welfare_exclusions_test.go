package admin

import (
	"context"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type welfareExcludedUserRepo struct{ service.UserRepository }

func (welfareExcludedUserRepo) GetUserAvatar(context.Context, int64) (*service.UserAvatar, error) {
	return nil, nil
}

func (welfareExcludedUserRepo) GetByID(_ context.Context, id int64) (*service.User, error) {
	if id == 404 {
		return nil, service.ErrUserNotFound
	}
	role := service.RoleUser
	if id == 1 {
		role = service.RoleAdmin
	}
	return &service.User{ID: id, Role: role}, nil
}

func TestSettingsWelfareExclusionsRoundTrip(t *testing.T) {
	h, repo := newStepUpSwitchTestHandler(t, map[string]string{})
	h.userService = service.NewUserService(welfareExcludedUserRepo{}, nil, nil, nil)
	rec := doUpdateSettings(t, h, map[string]any{"welfare_excluded_user_ids": []int64{7, 9}}, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "[7,9]", repo.values[service.SettingKeyWelfareExcludedUserIDs])
	require.Equal(t, "[7,9]", gjson.Get(rec.Body.String(), "data.welfare_excluded_user_ids").Raw)

	rec = doUpdateSettings(t, h, map[string]any{"site_name": "keep"}, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "[7,9]", repo.values[service.SettingKeyWelfareExcludedUserIDs])

	rec = doUpdateSettings(t, h, map[string]any{"welfare_excluded_user_ids": []int64{}}, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "[]", repo.values[service.SettingKeyWelfareExcludedUserIDs])
	require.Equal(t, "[]", gjson.Get(rec.Body.String(), "data.welfare_excluded_user_ids").Raw)
}

func TestSettingsWelfareExclusionsRejectInvalidUsers(t *testing.T) {
	h, repo := newStepUpSwitchTestHandler(t, map[string]string{service.SettingKeyWelfareExcludedUserIDs: "[7]"})
	h.userService = service.NewUserService(welfareExcludedUserRepo{}, nil, nil, nil)
	for _, ids := range [][]int64{{0}, {-1}, {7, 7}, {1}, {404}} {
		rec := doUpdateSettings(t, h, map[string]any{"welfare_excluded_user_ids": ids}, nil)
		require.NotEqual(t, http.StatusOK, rec.Code, rec.Body.String())
		require.Equal(t, "[7]", repo.values[service.SettingKeyWelfareExcludedUserIDs])
	}
}

func TestSettingsWelfareVisibilityRoundTrip(t *testing.T) {
	h, repo := newStepUpSwitchTestHandler(t, map[string]string{
		service.SettingKeyWelfareExcludedUserIDs: "[7,9]",
	})
	rec := doUpdateSettings(t, h, map[string]any{"welfare_leaderboard_visible_user_ids": []int64{7}}, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "[7]", repo.values[service.SettingKeyWelfareLeaderboardVisibleUserIDs])
	require.Equal(t, "[7,9]", repo.values[service.SettingKeyWelfareExcludedUserIDs])
	require.Equal(t, "[7]", gjson.Get(rec.Body.String(), "data.welfare_leaderboard_visible_user_ids").Raw)

	rec = doUpdateSettings(t, h, map[string]any{"site_name": "keep"}, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "[7]", repo.values[service.SettingKeyWelfareLeaderboardVisibleUserIDs])

	for _, ids := range [][]int64{{0}, {-1}, {7, 7}, {404}} {
		rec = doUpdateSettings(t, h, map[string]any{"welfare_leaderboard_visible_user_ids": ids}, nil)
		require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
		require.Equal(t, "[7]", repo.values[service.SettingKeyWelfareLeaderboardVisibleUserIDs])
	}

	rec = doUpdateSettings(t, h, map[string]any{"welfare_excluded_user_ids": []int64{9}}, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "[]", repo.values[service.SettingKeyWelfareLeaderboardVisibleUserIDs])
}

func TestSettingsWelfareVisibilityClearKeepsRewardExclusion(t *testing.T) {
	h, repo := newStepUpSwitchTestHandler(t, map[string]string{
		service.SettingKeyWelfareExcludedUserIDs:           "[7]",
		service.SettingKeyWelfareLeaderboardVisibleUserIDs: "[7]",
	})
	rec := doUpdateSettings(t, h, map[string]any{"welfare_leaderboard_visible_user_ids": []int64{}}, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "[]", repo.values[service.SettingKeyWelfareLeaderboardVisibleUserIDs])
	require.Equal(t, "[7]", repo.values[service.SettingKeyWelfareExcludedUserIDs])
}
