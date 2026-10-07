package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestPrismSettingsSwitchAppliesImmediately(t *testing.T) {
	ctx := context.Background()
	repo := &excelBPSImageSettingsRepo{}
	settings := NewSettingService(repo, &config.Config{})
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Extra: map[string]any{"openai_prism_browser": true}}
	current, err := settings.prismBrowserRuntimeForAccount(ctx, account, "gpt-6-luna")
	require.NoError(t, err)
	require.False(t, current.Enabled)
	update := &SystemSettings{PrismBrowserEnabled: true, PrismBrowserBaseURL: DefaultPrismBrowserBaseURL, PrismBrowserAPIKey: strings.Repeat("a", PrismBrowserAPIKeyMinLength)}
	require.NoError(t, settings.UpdateSettings(ctx, update))
	current, err = settings.prismBrowserRuntimeForAccount(ctx, account, "gpt-6-luna")
	require.NoError(t, err)
	require.True(t, current.Enabled)
	require.Equal(t, update.PrismBrowserAPIKey, current.APIKey)
	require.Equal(t, update.PrismBrowserBaseURL, current.BaseURL)
	disabledAccount := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	current, err = settings.prismBrowserRuntimeForAccount(ctx, disabledAccount, "gpt-6-luna")
	require.NoError(t, err)
	require.False(t, current.Enabled)
	current, err = settings.prismBrowserRuntimeForAccount(ctx, account, "unsupported-model")
	require.NoError(t, err)
	require.False(t, current.Enabled)
	require.NoError(t, settings.UpdateSettings(ctx, &SystemSettings{PrismBrowserBaseURL: DefaultPrismBrowserBaseURL}))
	current, err = settings.prismBrowserRuntimeForAccount(ctx, account, "gpt-6-luna")
	require.NoError(t, err)
	require.False(t, current.Enabled)
	require.Equal(t, update.PrismBrowserAPIKey, current.APIKey)
}

func TestPrismSettingsDeploymentSeedsAndSavedValues(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{}
	cfg.Gateway.PrismBrowser.BaseURL = "http://127.0.0.1:8320/v1"
	cfg.Gateway.PrismBrowser.APIKey = strings.Repeat("b", PrismBrowserAPIKeyMinLength)
	repo := &excelBPSImageSettingsRepo{}
	settings := NewSettingService(repo, cfg)
	current, err := settings.GetPrismBrowserRuntime(ctx)
	require.NoError(t, err)
	require.False(t, current.Enabled)
	require.Equal(t, cfg.Gateway.PrismBrowser.BaseURL, current.BaseURL)
	require.Equal(t, cfg.Gateway.PrismBrowser.APIKey, current.APIKey)
	require.NoError(t, settings.UpdateSettings(ctx, &SystemSettings{PrismBrowserEnabled: true, PrismBrowserBaseURL: DefaultPrismBrowserBaseURL}))
	current, err = settings.GetPrismBrowserRuntime(ctx)
	require.NoError(t, err)
	require.True(t, current.Enabled)
	require.Equal(t, DefaultPrismBrowserBaseURL, current.BaseURL)
	require.Equal(t, cfg.Gateway.PrismBrowser.APIKey, repo.values[SettingKeyPrismBrowserAPIKey])
	stored, err := settings.GetAllSettings(ctx)
	require.NoError(t, err)
	require.True(t, stored.PrismBrowserAPIKeyConfigured)
	require.Equal(t, cfg.Gateway.PrismBrowser.APIKey, stored.PrismBrowserAPIKey)
}

func TestPrismSettingsFailuresAreExposed(t *testing.T) {
	ctx := context.Background()
	repo := &excelBPSImageSettingsRepo{err: errors.New("settings database unavailable")}
	settings := NewSettingService(repo, &config.Config{})
	_, err := settings.GetPrismBrowserRuntime(ctx)
	require.ErrorContains(t, err, "settings database unavailable")
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Extra: map[string]any{"openai_prism_browser": true}}
	_, err = settings.prismBrowserRuntimeForAccount(ctx, account, "gpt-6-luna")
	require.Error(t, err)
	var missing *SettingService
	_, err = missing.prismBrowserRuntimeForAccount(ctx, account, "gpt-6-luna")
	require.ErrorContains(t, err, "Prism settings service is unavailable")
}
