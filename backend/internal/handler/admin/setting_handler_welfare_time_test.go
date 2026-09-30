package admin

import (
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestSettingsWelfareRewardTimeRoundTrip(t *testing.T) {
	h, repo := newStepUpSwitchTestHandler(t, map[string]string{})
	rec := doUpdateSettings(t, h, map[string]any{"welfare_reward_time": "07:30"}, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "07:30", repo.values[service.SettingKeyWelfareRewardTime])
	require.Equal(t, "07:30", gjson.Get(rec.Body.String(), "data.welfare_reward_time").String())

	rec = doUpdateSettings(t, h, map[string]any{"site_name": "kept"}, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "07:30", repo.values[service.SettingKeyWelfareRewardTime])

	for _, invalid := range []string{"", "24:00", "7:30"} {
		rec = doUpdateSettings(t, h, map[string]any{"welfare_reward_time": invalid}, nil)
		require.Equal(t, http.StatusBadRequest, rec.Code, invalid)
		require.Equal(t, "07:30", repo.values[service.SettingKeyWelfareRewardTime])
	}
}
