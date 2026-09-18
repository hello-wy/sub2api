package service

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const codexGatewayHistoryPrefix = "codex_gateway_history_"

func (s *adminServiceImpl) normalizeAccountCodexGateway(account *Account) error {
	raw, exists := account.Extra[codexBaseURLExtraKey]
	if !exists {
		return nil
	}
	value, ok := raw.(string)
	if !ok {
		return infraerrors.BadRequest("INVALID_CODEX_GATEWAY", "codex_base_url must be a string")
	}
	if strings.TrimSpace(value) == "" {
		delete(account.Extra, codexBaseURLExtraKey)
		return nil
	}
	if !account.IsOpenAIOAuthLike() || account.IsShadow() {
		return infraerrors.BadRequest("INVALID_CODEX_GATEWAY_ACCOUNT", "Codex gateways can only be configured on OpenAI OAuth/setup-token accounts; shadows inherit their parent's gateway")
	}
	normalized, err := normalizeCodexBaseURL(value, s.cfg)
	if err != nil {
		return infraerrors.BadRequest("INVALID_CODEX_GATEWAY", err.Error())
	}
	if normalized == "" {
		delete(account.Extra, codexBaseURLExtraKey)
	} else {
		account.Extra[codexBaseURLExtraKey] = normalized
	}
	return nil
}

// One setting per normalized URL makes concurrent account saves an idempotent
// upsert, without a read/modify/write race or a new database table. History
// survives switching an account back to the official endpoint or deleting it.
func (s *adminServiceImpl) rememberCodexGateway(ctx context.Context, account *Account) {
	base := account.GetExtraString(codexBaseURLExtraKey)
	if base == "" || !account.IsOpenAIOAuthLike() || account.IsShadow() || s.settingService == nil || s.settingService.settingRepo == nil {
		return
	}
	key := fmt.Sprintf("%s%x", codexGatewayHistoryPrefix, sha256.Sum256([]byte(base)))
	if err := s.settingService.settingRepo.Set(ctx, key, base); err != nil {
		// The account is already saved; do not report its creation as failed.
		slog.Warn("remember_codex_gateway_failed", "account_id", account.ID, "error", err)
	}
}

func (s *adminServiceImpl) ListCodexGateways(ctx context.Context) ([]string, error) {
	gateways := make([]string, 0)
	if s.settingService == nil || s.settingService.settingRepo == nil {
		return gateways, nil
	}
	settings, err := s.settingService.settingRepo.GetAll(ctx)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	for key, value := range settings {
		if strings.HasPrefix(key, codexGatewayHistoryPrefix) && value != "" && !seen[value] {
			gateways = append(gateways, value)
			seen[value] = true
		}
	}
	sort.Strings(gateways)
	return gateways, nil
}
