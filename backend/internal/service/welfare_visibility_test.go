package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type welfareVisibilitySettingsStub struct {
	SettingRepository
	values map[string]string
}

func (s welfareVisibilitySettingsStub) GetAll(context.Context) (map[string]string, error) {
	return s.values, nil
}

func TestWelfareVisibilitySettingsValidation(t *testing.T) {
	for _, raw := range []string{"null", "{}", "[0]", "[-1]", "[7,7]", `["7"]`, "[9]"} {
		svc := NewSettingService(welfareVisibilitySettingsStub{values: map[string]string{
			SettingKeyWelfareExcludedUserIDs:           "[7]",
			SettingKeyWelfareLeaderboardVisibleUserIDs: raw,
		}}, &config.Config{})
		_, err := svc.GetAllSettings(context.Background())
		require.Error(t, err, raw)
	}
	for _, raw := range []string{"", "[]", "[7]"} {
		svc := NewSettingService(welfareVisibilitySettingsStub{values: map[string]string{
			SettingKeyWelfareExcludedUserIDs:           "[7]",
			SettingKeyWelfareLeaderboardVisibleUserIDs: raw,
		}}, &config.Config{})
		settings, err := svc.GetAllSettings(context.Background())
		require.NoError(t, err)
		require.Equal(t, []int64{7}, settings.WelfareExcludedUserIDs)
		require.NotNil(t, settings.WelfareLeaderboardVisibleUserIDs)
	}
}
