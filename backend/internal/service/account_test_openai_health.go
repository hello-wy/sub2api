package service

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/tidwall/gjson"
)

// Ordinary probes use the same refresh/cache path as inference. Intelligent
// observations deliberately keep their read-only credential snapshot.
func (s *AccountTestService) openAIAccountTestToken(ctx context.Context, account *Account) (string, error) {
	if openAIOAuthReauthorizationRequired(account) {
		return "", errors.New("OAuth credentials require reauthorization before testing")
	}
	if account.IsOpenAIAgentIdentity() {
		return "", nil
	}
	if intelligentContext(ctx) == nil && s.openaiGatewayService != nil {
		token, _, err := s.openaiGatewayService.GetAccessToken(ctx, account)
		return token, err
	}
	token := account.GetOpenAIAccessToken()
	if token == "" {
		return "", errors.New("no access token available")
	}
	return token, nil
}

func openAIOAuthReauthorizationRequired(account *Account) bool {
	return account != nil && account.Platform == PlatformOpenAI && account.Type == AccountTypeOAuth &&
		account.Status == StatusError && (IsOAuthRefreshCooldown(account.ErrorMessage) ||
		strings.HasPrefix(account.ErrorMessage, "OAuth refresh rejected;") ||
		strings.HasPrefix(account.ErrorMessage, "Token revoked (401):"))
}

// Capture the final outbound header, after administrator header overrides.
// Health CAS must never use a scheduler snapshot or a later refreshed token.
func openAIAccountTestAuthContext(req *http.Request) context.Context {
	ctx := req.Context()
	if value := strings.TrimSpace(req.Header.Get("Authorization")); len(value) > 7 && strings.EqualFold(value[:7], "Bearer ") {
		ctx = WithOpenAIUpstreamAccessToken(ctx, strings.TrimSpace(value[7:]))
	}
	return ctx
}

func (s *AccountTestService) recordOpenAIAccountTest401(ctx context.Context, account *Account, headers http.Header, body []byte) {
	if s == nil || account == nil || intelligentContext(ctx) != nil {
		return
	}
	if s.openaiGatewayService != nil && s.openaiGatewayService.rateLimitService != nil {
		s.openaiGatewayService.handleOpenAIAccountUpstreamError(ctx, account, http.StatusUnauthorized, headers, body)
		return
	}
	if s.accountRepo == nil {
		return
	}
	limits := &RateLimitService{accountRepo: s.accountRepo, cfg: s.cfg}
	if handled, _ := limits.handleOpenAIOAuth401(ctx, account, extractUpstreamErrorCode(body), ""); handled {
		return
	}
	stateCtx, cancel := openAIAccountStateContext(ctx)
	defer cancel()
	_ = s.accountRepo.SetError(stateCtx, account.ID, fmt.Sprintf("Authentication failed (401): %s", sanitizeUpstreamErrorMessage(extractUpstreamErrorMessage(body))))
}

func (s *AccountTestService) observeOpenAIAccountTest401(ctx context.Context, account *Account, headers http.Header, payload []byte) {
	if openAIStreamCredentialAuthFailure(payload) && openAIStreamFailureStatus(payload, "") == http.StatusUnauthorized {
		s.recordOpenAIAccountTest401(ctx, account, headers, payload)
	}
}

// Compact/image probes already buffer their response. Inspect structured JSON
// or SSE errors without changing bytes consumed by their result parsers.
func (s *AccountTestService) observeOpenAIAccountTestBody401(ctx context.Context, account *Account, headers http.Header, body []byte) {
	if gjson.ValidBytes(body) {
		s.observeOpenAIAccountTest401(ctx, account, headers, body)
		return
	}
	scanner := bufio.NewScanner(bytes.NewReader(body))
	scanner.Buffer(make([]byte, 4096), maxAccountTestSSELine)
	for scanner.Scan() {
		payload, ok := accountTestSSEData(scanner.Text())
		if ok && openAIStreamCredentialAuthFailure([]byte(payload)) && openAIStreamFailureStatus([]byte(payload), "") == http.StatusUnauthorized {
			s.recordOpenAIAccountTest401(ctx, account, headers, []byte(payload))
			return
		}
	}
}
