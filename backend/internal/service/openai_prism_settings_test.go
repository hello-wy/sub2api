package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrismForwardUsesSavedAddressAndRotatedKey(t *testing.T) {
	ctx := context.Background()
	repo := &excelBPSImageSettingsRepo{}
	cfg := &config.Config{}
	cfg.Gateway.PrismBrowser.BaseURL = "http://127.0.0.1:1/v1"
	cfg.Gateway.PrismBrowser.APIKey = "deployment-key-must-not-be-used"
	settings := NewSettingService(repo, cfg)
	gateway := &OpenAIGatewayService{cfg: cfg, settingService: settings}
	account := &Account{ID: 42, Platform: PlatformOpenAI, Type: AccountTypeOAuth,
		Extra: map[string]any{"openai_prism_browser": true}, Credentials: map[string]any{"access_token": "test-oauth-token"}}
	for _, character := range []string{"a", "b"} {
		key := strings.Repeat(character, PrismBrowserAPIKeyMinLength)
		adapter := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/v1/responses", r.URL.Path)
			assert.Equal(t, "Bearer "+key, r.Header.Get("Authorization"))
			assert.Equal(t, "test-oauth-token", r.Header.Get("X-Prism-OAuth-Token"))
			assert.Equal(t, "42", r.Header.Get("X-Prism-Account-ID"))
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"resp_test","status":"completed","output":[{"content":[{"text":"test response"}]}]}`))
		}))
		t.Cleanup(adapter.Close)
		require.NoError(t, settings.UpdateSettings(ctx, &SystemSettings{
			PrismBrowserEnabled: true, PrismBrowserBaseURL: adapter.URL + "/v1", PrismBrowserAPIKey: key,
		}))
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		body := []byte(`{"model":"gpt-6-luna","input":"hello","stream":false}`)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(string(body)))
		result, err := gateway.Forward(ctx, c, account, body)
		require.NoError(t, err)
		require.Equal(t, "resp_test", result.ResponseID)
		require.Equal(t, http.StatusOK, recorder.Code)
	}
}
