package service

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	SettingKeyPrismBrowserEnabled = "prism_browser_enabled"
	SettingKeyPrismBrowserBaseURL = "prism_browser_base_url"
	SettingKeyPrismBrowserAPIKey  = "prism_browser_api_key"
	DefaultPrismBrowserBaseURL    = "http://127.0.0.1:8319/v1"
	PrismBrowserAPIKeyMinLength   = 32
)

type PrismBrowserRuntime struct {
	Enabled bool
	BaseURL string
	APIKey  string `json:"-"`
}

// Deployment configuration seeds missing address/key settings. The database
// switch is authoritative and defaults to disabled, including existing installs.
func (s *SettingService) parsePrismBrowserSettings(values map[string]string) PrismBrowserRuntime {
	baseURL, hasBaseURL := values[SettingKeyPrismBrowserBaseURL]
	key, hasKey := values[SettingKeyPrismBrowserAPIKey]
	if s != nil && s.cfg != nil {
		if !hasBaseURL {
			baseURL = s.cfg.Gateway.PrismBrowser.BaseURL
		}
		if !hasKey {
			key = s.cfg.Gateway.PrismBrowser.APIKey
		}
	}
	if !hasBaseURL && strings.TrimSpace(baseURL) == "" {
		baseURL = DefaultPrismBrowserBaseURL
	}
	return PrismBrowserRuntime{
		Enabled: values[SettingKeyPrismBrowserEnabled] == "true",
		BaseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"), APIKey: strings.TrimSpace(key),
	}
}

func validatePrismBrowserSettings(runtime PrismBrowserRuntime) error {
	if runtime.Enabled || runtime.BaseURL != "" {
		if _, err := prismBrowserAdapterURL(runtime.BaseURL); err != nil {
			return infraerrors.BadRequest("INVALID_PRISM_BROWSER_BASE_URL", err.Error())
		}
	}
	if runtime.Enabled || runtime.APIKey != "" {
		if utf8.RuneCountInString(runtime.APIKey) < PrismBrowserAPIKeyMinLength || strings.ContainsAny(runtime.APIKey, "\r\n") {
			return infraerrors.BadRequest("INVALID_PRISM_BROWSER_API_KEY", "Prism adapter key must contain at least 32 characters and no line breaks")
		}
	}
	return nil
}

// Read per request so administrator saves apply without restart or stale caches.
func (s *SettingService) GetPrismBrowserRuntime(ctx context.Context) (PrismBrowserRuntime, error) {
	runtime, err := s.readPrismBrowserSettings(ctx)
	if err != nil {
		return PrismBrowserRuntime{}, err
	}
	if runtime.Enabled {
		if err := validatePrismBrowserSettings(runtime); err != nil {
			return PrismBrowserRuntime{}, err
		}
	}
	return runtime, nil
}

func (s *SettingService) readPrismBrowserSettings(ctx context.Context) (PrismBrowserRuntime, error) {
	if s == nil || s.settingRepo == nil {
		return PrismBrowserRuntime{}, infraerrors.ServiceUnavailable("PRISM_SETTINGS_UNAVAILABLE", "Prism settings service is unavailable")
	}
	dbCtx, cancel := context.WithTimeout(ctx, gatewayForwardingDBTimeout)
	defer cancel()
	values, err := s.settingRepo.GetMultiple(dbCtx, []string{
		SettingKeyPrismBrowserEnabled, SettingKeyPrismBrowserBaseURL, SettingKeyPrismBrowserAPIKey,
	})
	if err != nil {
		return PrismBrowserRuntime{}, fmt.Errorf("read Prism settings: %w", err)
	}
	return s.parsePrismBrowserSettings(values), nil
}

func (s *SettingService) prismBrowserRuntimeForAccount(ctx context.Context, account *Account, model string) (PrismBrowserRuntime, error) {
	if !account.IsPrismBrowserEnabledForModel(model) {
		return PrismBrowserRuntime{}, nil
	}
	return s.GetPrismBrowserRuntime(ctx)
}

func (s *SettingService) prismBrowserSettingsForUpdate(ctx context.Context, settings *SystemSettings) (PrismBrowserRuntime, error) {
	runtime := PrismBrowserRuntime{
		Enabled: settings.PrismBrowserEnabled,
		BaseURL: strings.TrimRight(strings.TrimSpace(settings.PrismBrowserBaseURL), "/"),
		APIKey:  strings.TrimSpace(settings.PrismBrowserAPIKey),
	}
	if runtime.Enabled && runtime.APIKey == "" {
		current, err := s.readPrismBrowserSettings(ctx)
		if err != nil {
			return PrismBrowserRuntime{}, err
		}
		runtime.APIKey = current.APIKey
	}
	return runtime, validatePrismBrowserSettings(runtime)
}
