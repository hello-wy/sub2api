package service

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	SettingKeyOpenAICodexTicketEnabled         = "openai_codex_ticket_enabled"
	SettingKeyOpenAICodexTicketHarvestProxyURL = "openai_codex_ticket_harvest_proxy_url"
	codexTicketSettingsCacheTTL                = 5 * time.Second
	codexTicketSettingsErrorTTL                = time.Second
	codexTicketSettingsDBTimeout               = 2 * time.Second
)

// The endpoint is intentionally separate from SystemSettings: the proxy URL
// is write-only and must never appear in the generic settings response.
type CodexTicketSettings struct {
	Enabled                bool   `json:"enabled"`
	HarvestProxyConfigured bool   `json:"harvest_proxy_configured"`
	HarvestProxyDisplay    string `json:"harvest_proxy_display"`
}

type CodexTicketSettingsUpdate struct {
	Enabled         bool   `json:"enabled"`
	HarvestProxyURL string `json:"harvest_proxy_url"`
	ClearProxy      bool   `json:"clear_proxy"`
}

type cachedCodexTicketSettings struct {
	enabled       bool
	enabledStored bool
	proxyURL      string
	expiresAt     time.Time
	failed        bool
}

func (s *SettingService) readCodexTicketSettings(ctx context.Context) (*cachedCodexTicketSettings, error) {
	if s == nil || s.settingRepo == nil {
		return nil, errors.New("STATE settings service unavailable")
	}
	values, err := s.settingRepo.GetMultiple(ctx, []string{SettingKeyOpenAICodexTicketEnabled, SettingKeyOpenAICodexTicketHarvestProxyURL})
	if err != nil {
		// Repository errors can include SQL arguments; do not propagate secrets.
		return nil, errors.New("unable to read STATE settings")
	}
	enabledValue, exists := values[SettingKeyOpenAICodexTicketEnabled]
	proxyURL := strings.TrimSpace(values[SettingKeyOpenAICodexTicketHarvestProxyURL])
	if ValidateOpenAICodexTicketHarvestProxyURL(proxyURL) != nil || IsMaskedProxyURL(proxyURL) {
		proxyURL = ""
	}
	return &cachedCodexTicketSettings{enabled: enabledValue == "true", enabledStored: exists, proxyURL: proxyURL, expiresAt: time.Now().Add(codexTicketSettingsCacheTTL)}, nil
}

func (s *SettingService) codexTicketSettingsRuntime(ctx context.Context) cachedCodexTicketSettings {
	if s == nil || s.settingRepo == nil {
		return cachedCodexTicketSettings{failed: true}
	}
	s.codexTicketSettingsMu.Lock()
	defer s.codexTicketSettingsMu.Unlock()
	if cached := s.codexTicketSettingsCache; cached != nil && time.Now().Before(cached.expiresAt) {
		return *cached
	}
	if ctx == nil {
		ctx = context.Background()
	}
	dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), codexTicketSettingsDBTimeout)
	defer cancel()
	cached, err := s.readCodexTicketSettings(dbCtx)
	if err != nil {
		// Never serve stale enabled configuration after a failed refresh.
		cached = &cachedCodexTicketSettings{failed: true, expiresAt: time.Now().Add(codexTicketSettingsErrorTTL)}
	}
	s.codexTicketSettingsCache = cached
	return *cached
}

func (s *SettingService) GetOpenAICodexTicketEnabled(ctx context.Context, fallback bool) bool {
	settings := s.codexTicketSettingsRuntime(ctx)
	if settings.failed {
		return false
	}
	if !settings.enabledStored {
		return fallback
	}
	return settings.enabled && settings.proxyURL != ""
}

func (s *SettingService) GetOpenAICodexTicketHarvestProxyURL(ctx context.Context) string {
	return s.codexTicketSettingsRuntime(ctx).proxyURL
}

// GetOpenAICodexTicketRuntime distinguishes an explicit global disable from a
// settings failure. Opted-in accounts must keep their ticket gate on errors.
func (s *SettingService) GetOpenAICodexTicketRuntime(ctx context.Context, fallback bool) (bool, string, error) {
	settings := s.codexTicketSettingsRuntime(ctx)
	if settings.failed {
		return false, "", errors.New("STATE settings unavailable")
	}
	enabled := settings.enabled
	if !settings.enabledStored {
		enabled = fallback
	}
	if enabled && settings.proxyURL == "" {
		return true, "", errors.New("STATE harvest proxy unavailable")
	}
	return enabled, settings.proxyURL, nil
}

func codexTicketSettingsPublic(settings *cachedCodexTicketSettings) *CodexTicketSettings {
	return &CodexTicketSettings{Enabled: settings.enabled, HarvestProxyConfigured: settings.proxyURL != "", HarvestProxyDisplay: MaskProxyURL(settings.proxyURL)}
}

func (s *SettingService) GetCodexTicketSettings(ctx context.Context) (*CodexTicketSettings, error) {
	if s == nil {
		return nil, errors.New("STATE settings service unavailable")
	}
	s.codexTicketSettingsMu.Lock()
	defer s.codexTicketSettingsMu.Unlock()
	settings, err := s.readCodexTicketSettings(ctx)
	if err != nil {
		s.codexTicketSettingsCache = nil
		return nil, err
	}
	s.codexTicketSettingsCache = settings
	return codexTicketSettingsPublic(settings), nil
}

func (s *SettingService) UpdateCodexTicketSettings(ctx context.Context, input CodexTicketSettingsUpdate) (*CodexTicketSettings, error) {
	if s == nil || s.settingRepo == nil {
		return nil, errors.New("STATE settings service unavailable")
	}
	proxyURL := strings.TrimSpace(input.HarvestProxyURL)
	if len(proxyURL) > 4096 || (input.ClearProxy && proxyURL != "") {
		return nil, infraerrors.BadRequest("CODEX_TICKET_PROXY_INVALID", "Provide one proxy URL or clear_proxy, not both; URL must be at most 4096 bytes")
	}
	if proxyURL != "" && (ValidateOpenAICodexTicketHarvestProxyURL(proxyURL) != nil || IsMaskedProxyURL(proxyURL)) {
		return nil, infraerrors.BadRequest("CODEX_TICKET_PROXY_INVALID", "Invalid STATE harvest proxy URL; provide the original HTTP(S) or SOCKS5(h) URL")
	}
	s.codexTicketSettingsMu.Lock()
	defer s.codexTicketSettingsMu.Unlock()
	settings, err := s.readCodexTicketSettings(ctx)
	if err != nil {
		s.codexTicketSettingsCache = nil
		return nil, err
	}
	updates := map[string]string{SettingKeyOpenAICodexTicketEnabled: strconv.FormatBool(input.Enabled)}
	if input.ClearProxy {
		settings.proxyURL = ""
		updates[SettingKeyOpenAICodexTicketHarvestProxyURL] = ""
	} else if proxyURL != "" {
		settings.proxyURL = proxyURL
		updates[SettingKeyOpenAICodexTicketHarvestProxyURL] = proxyURL
	}
	if input.Enabled && settings.proxyURL == "" {
		return nil, infraerrors.BadRequest("CODEX_TICKET_PROXY_REQUIRED", "Configure a valid STATE harvest proxy before enabling")
	}
	if err := s.settingRepo.SetMultiple(ctx, updates); err != nil {
		s.codexTicketSettingsCache = nil
		return nil, errors.New("unable to save STATE settings")
	}
	settings.enabled, settings.enabledStored = input.Enabled, true
	settings.expiresAt = time.Now().Add(codexTicketSettingsCacheTTL)
	s.codexTicketSettingsCache = settings
	return codexTicketSettingsPublic(settings), nil
}

func (s *SettingService) InvalidateOpenAICodexTicketEnabledCache() {
	if s == nil {
		return
	}
	s.codexTicketSettingsMu.Lock()
	s.codexTicketSettingsCache = nil
	s.codexTicketSettingsMu.Unlock()
}
