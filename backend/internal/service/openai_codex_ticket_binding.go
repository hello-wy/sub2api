package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const codexTicketVerificationScope = "fixed_business_route"
const codexTicketExpiryKind = "local_cache_ttl"

// These digests bind verification to one authorization generation. They never
// expose bearer/refresh credentials through the administrative status DTO.
func codexTicketDigest(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func codexTicketCredentialFingerprint(account *Account) string {
	if account == nil || account.GetOpenAIAccessToken() == "" {
		return ""
	}
	values := map[string]any{"binding_version": 1, "type": account.Type}
	for _, key := range []string{"access_token", "refresh_token", "_token_version", OpenAIOAuthCredentialGroupKey, "client_id", "chatgpt_account_id", "chatgpt_user_id", "auth_mode"} {
		values[key] = account.Credentials[key]
	}
	return codexTicketDigest(values)
}

func codexTicketIdentityFingerprint(account *Account) string {
	if account == nil {
		return ""
	}
	values := map[string]any{"binding_version": 1, "namespace": codexAccountIdentityNamespace(account), "canonical_ua": CodexCanonicalUserAgent(), "identity_enforcement": codexIdentityEnforcement.Load()}
	for _, key := range []string{AntiDegradationExtraKey, AntiDegradeMarkerExtraKey, ProtectionScopeExtraKey, codexFingerprintModeExtraKey, codexFingerprintSeedExtraKey, "openai_device_id", "enable_tls_fingerprint", "tls_fingerprint_builtin", "tls_fingerprint_profile_id", "request_integrity_mode"} {
		values[key] = account.Extra[key]
	}
	for _, key := range []string{"user_agent", "device_id", "installation_id", "chatgpt_account_id", "chatgpt_user_id"} {
		values["credential_"+key] = account.Credentials[key]
	}
	values["configured_ua"] = account.GetOpenAIUserAgent()
	if account.Proxy != nil && account.ProxyID != nil {
		values["fixed_route"] = []any{*account.ProxyID, account.Proxy.URL()}
	}
	return codexTicketDigest(values)
}

func (s *OpenAIGatewayService) codexTicketRuntimeFingerprint() string {
	forceCLI, tlsEnabled := false, true
	if s != nil && s.cfg != nil {
		forceCLI, tlsEnabled = s.cfg.Gateway.ForceCodexCLI, s.cfg.Gateway.TLSFingerprint.Enabled
	}
	return codexTicketDigest([]any{1, forceCLI, tlsEnabled})
}

func codexTicketAuthenticationBlocked(account *Account, now time.Time) bool {
	return account == nil || (account.Status == StatusError && IsOAuthRefreshCooldown(account.ErrorMessage)) ||
		(account.TempUnschedulableUntil != nil && account.TempUnschedulableUntil.After(now) && IsOAuthRefreshCooldown(account.TempUnschedulableReason))
}

// Only this private context value skips recursive ticket injection while the
// candidate is being verified; ordinary forwarding cannot set it from input.
type codexTicketProbeContextKey struct{}
type codexTicketProbeEvidenceKey struct{}
type codexTicketProbeEvidence struct {
	model string
}

// Reuse the business request builder and its body/header identity helpers.
// A new diagnostic conversation is appropriate; device/session convergence is
// applied exactly as it is for a new ordinary request under the chosen policy.
func (s *OpenAIGatewayService) buildCodexTicketProbeRequest(ctx context.Context, account *Account, token, model, injectedState string) (*http.Request, error) {
	if err := ValidateAccountProtectionConfiguration(account); err != nil {
		return nil, err
	}
	session := uuid.NewString()
	payload := map[string]any{"model": model, "store": false, "stream": true, "instructions": "Reply with exactly: pong", "input": []any{map[string]any{"role": "user", "content": []any{map[string]any{"type": "input_text", "text": "ping"}}}}, "prompt_cache_key": session}
	original, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	source, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://localhost/v1/responses", bytes.NewReader(original))
	if err != nil {
		return nil, err
	}
	source.Header.Set("session-id", session)
	source.Header.Set("OpenAI-Beta", "responses=experimental")
	c := &gin.Context{Request: source}
	stageMode1Request(c, account, original)
	if _, err := s.prepareCodexAccountIdentitySource(ctx, c, account); err != nil {
		return nil, err
	}
	applyCodexClientMetadata(payload, account)
	applyCodexAccountIdentityClientMetadataMap(payload, account, 0)
	ids := resolveCodexFingerprintIDsFromRequest(account, source.Header)
	applyCodexFingerprintClientMetadata(payload, ids)
	stageCodexFingerprintIDs(c, ids)
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := s.buildUpstreamRequest(context.WithValue(ctx, codexTicketProbeContextKey{}, true), c, account, body, token, true, session, true)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(WithHTTPUpstreamRedirectsDisabled(req.Context()))
	req.Close = true
	if injectedState != "" {
		req.Header.Set(openAICodexTurnStateHeader, injectedState)
	}
	return req, nil
}
