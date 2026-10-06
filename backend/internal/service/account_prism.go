package service

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

const PrismBrowserModelsKey = "openai_prism_browser_models"

var prismBrowserModels = [...]string{"gpt-6.1-sol", "gpt-5.6-sol", "gpt-5.6-terra", "gpt-6-luna"}

// PrismBrowserSupportedModels returns the adapter contract. Account access is
// still determined by the upstream Prism session at request time.
func PrismBrowserSupportedModels() []string {
	return append([]string(nil), prismBrowserModels[:]...)
}

func prismBrowserResponsesURL(baseURL string) string {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		return ""
	}
	if strings.HasSuffix(base, "/responses") {
		return base
	}
	return base + "/responses"
}

func accountUsesPrismBrowser(account *Account, cfg *config.Config) bool {
	if account == nil || cfg == nil || !cfg.Gateway.PrismBrowser.Enabled || account.Platform != PlatformOpenAI || account.Type != AccountTypeOAuth {
		return false
	}
	if value, ok := account.Extra["openai_prism_browser"]; !ok || value != true {
		return false
	}
	return true
}

// IsPrismBrowserEnabledForModel applies account mapping before the explicit
// Prism scope. An absent scope preserves legacy enabled accounts; an explicit
// empty or malformed scope never widens routing.
func (a *Account) IsPrismBrowserEnabledForModel(requestedModel string) bool {
	if a == nil || a.Platform != PlatformOpenAI || a.Type != AccountTypeOAuth {
		return false
	}
	if value, ok := a.Extra["openai_prism_browser"]; !ok || value != true {
		return false
	}
	upstream := strings.TrimSpace(a.GetMappedModel(strings.TrimSpace(requestedModel)))
	if !isPrismBrowserModel(upstream) {
		return false
	}
	raw, configured := a.Extra[PrismBrowserModelsKey]
	if !configured {
		return true
	}
	switch models := raw.(type) {
	case []string:
		for _, model := range models {
			if strings.TrimSpace(model) == upstream {
				return true
			}
		}
	case []any:
		for _, value := range models {
			if model, ok := value.(string); ok && strings.TrimSpace(model) == upstream {
				return true
			}
		}
	}
	return false
}

func isPrismBrowserModel(model string) bool {
	for _, supported := range prismBrowserModels {
		if model == supported {
			return true
		}
	}
	return false
}
