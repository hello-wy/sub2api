package admin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestSettingsPrismRoundTripAndSecretPreservation(t *testing.T) {
	h, repo := newStepUpSwitchTestHandler(t, map[string]string{})
	key := strings.Repeat("a", service.PrismBrowserAPIKeyMinLength)
	rec := doUpdateSettings(t, h, map[string]any{
		"prism_browser_enabled": true, "prism_browser_base_url": " http://127.0.0.1:8319/v1/ ", "prism_browser_api_key": key,
	}, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.True(t, gjson.Get(rec.Body.String(), "data.prism_browser_enabled").Bool())
	require.True(t, gjson.Get(rec.Body.String(), "data.prism_browser_api_key_configured").Bool())
	require.False(t, gjson.Get(rec.Body.String(), "data.prism_browser_api_key").Exists())
	require.NotContains(t, rec.Body.String(), key)
	require.Equal(t, key, repo.values[service.SettingKeyPrismBrowserAPIKey])
	rec = doUpdateSettings(t, h, map[string]any{"site_name": "preserve Prism", "prism_browser_api_key": " "}, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "true", repo.values[service.SettingKeyPrismBrowserEnabled])
	require.Equal(t, key, repo.values[service.SettingKeyPrismBrowserAPIKey])
	rec = httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/settings", nil)
	h.GetSettings(c)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.True(t, gjson.Get(rec.Body.String(), "data.prism_browser_enabled").Bool())
	require.Equal(t, service.DefaultPrismBrowserBaseURL, gjson.Get(rec.Body.String(), "data.prism_browser_base_url").String())
	require.NotContains(t, rec.Body.String(), key)
	rec = doUpdateSettings(t, h, map[string]any{"prism_browser_enabled": false}, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "false", repo.values[service.SettingKeyPrismBrowserEnabled])
	require.Equal(t, key, repo.values[service.SettingKeyPrismBrowserAPIKey])
	rec = doUpdateSettings(t, h, map[string]any{"prism_browser_enabled": true}, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

func TestSettingsPrismInvalidUpdatesAreAtomic(t *testing.T) {
	key := strings.Repeat("a", service.PrismBrowserAPIKeyMinLength)
	h, repo := newStepUpSwitchTestHandler(t, map[string]string{
		service.SettingKeyPrismBrowserEnabled: "true", service.SettingKeyPrismBrowserBaseURL: service.DefaultPrismBrowserBaseURL,
		service.SettingKeyPrismBrowserAPIKey: key,
	})
	for _, payload := range []map[string]any{
		{"prism_browser_base_url": "https://external.example/v1"},
		{"prism_browser_base_url": "http://127.0.0.1:8319/v1?token=secret"},
		{"prism_browser_api_key": "short"},
		{"prism_browser_api_key": key + "\ninvalid"},
	} {
		rec := doUpdateSettings(t, h, payload, nil)
		require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
		require.Equal(t, key, repo.values[service.SettingKeyPrismBrowserAPIKey])
		require.Equal(t, service.DefaultPrismBrowserBaseURL, repo.values[service.SettingKeyPrismBrowserBaseURL])
		require.Equal(t, "true", repo.values[service.SettingKeyPrismBrowserEnabled])
		require.NotContains(t, rec.Body.String(), key)
	}
}

func TestSettingsPrismEnableRequiresAdapterKey(t *testing.T) {
	h, _ := newStepUpSwitchTestHandler(t, map[string]string{})
	rec := doUpdateSettings(t, h, map[string]any{"prism_browser_enabled": true}, nil)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "INVALID_PRISM_BROWSER_API_KEY")
}
