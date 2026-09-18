package service

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

const codexBaseURLExtraKey = "codex_base_url"
const codexBackendPath = "/backend-api/codex"

// normalizeCodexBaseURL accepts a base endpoint, never credentials or query
// parameters. Reuse the deployment's outbound URL policy for custom gateways.
func normalizeCodexBaseURL(raw string, cfg *config.Config) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	u, err := url.Parse(raw)
	if err != nil || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return "", fmt.Errorf("Codex gateway must be a base URL without credentials, query parameters or fragment")
	}
	if u.RawPath != "" || strings.ContainsAny(u.Path, "\\") {
		return "", fmt.Errorf("invalid Codex gateway path")
	}
	for _, part := range strings.Split(u.Path, "/") {
		if part == "." || part == ".." {
			return "", fmt.Errorf("invalid Codex gateway path")
		}
	}
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	u.Path = strings.TrimRight(u.Path, "/")
	if strings.HasSuffix(u.Path, "/responses") || strings.HasSuffix(u.Path, "/responses/compact") {
		return "", fmt.Errorf("Codex gateway must be a base URL; remove /responses or /responses/compact")
	}
	normalized, err := (&OpenAIGatewayService{cfg: cfg}).validateOutboundURL(u.String())
	if err != nil {
		return "", fmt.Errorf("invalid Codex gateway: %w", err)
	}
	if normalized == "https://chatgpt.com"+codexBackendPath {
		return "", nil
	}
	return normalized, nil
}

// Only the Codex inference namespace is redirected. OAuth refresh, account
// settings and usage APIs deliberately retain their original destinations.
func rewriteCodexEndpoint(account *Account, cfg *config.Config, original string) (string, error) {
	if !account.IsOpenAIOAuthLike() {
		return original, nil
	}
	u, err := url.Parse(original)
	if err != nil {
		return "", err
	}
	if u.Host != "chatgpt.com" || (u.Path != codexBackendPath && !strings.HasPrefix(u.Path, codexBackendPath+"/")) {
		return original, nil
	}
	base, err := normalizeCodexBaseURL(account.GetExtraString(codexBaseURLExtraKey), cfg)
	if err != nil || base == "" {
		return original, err
	}
	target, _ := url.Parse(base)
	if u.Scheme == "wss" || u.Scheme == "ws" {
		if target.Scheme == "https" {
			target.Scheme = "wss"
		} else {
			target.Scheme = "ws"
		}
	}
	target.Path += strings.TrimPrefix(u.Path, codexBackendPath)
	target.RawQuery = u.RawQuery
	return target.String(), nil
}

func resolveCodexEndpoint(ctx context.Context, repo AccountRepository, cfg *config.Config, account *Account, original string) (string, error) {
	if !account.IsOpenAIOAuthLike() {
		return original, nil
	}
	credentialAccount, err := resolveCredentialAccount(ctx, repo, account)
	if err != nil {
		return "", err
	}
	return rewriteCodexEndpoint(credentialAccount, cfg, original)
}

// Applied immediately before transport, including callers that replace the
// request URL after constructing authentication (for example image requests).
func applyCodexRequestEndpoint(req *http.Request, repo AccountRepository, cfg *config.Config, account *Account) error {
	target, err := resolveCodexEndpoint(req.Context(), repo, cfg, account, req.URL.String())
	if err != nil {
		return err
	}
	if target != req.URL.String() {
		req.URL, err = url.Parse(target)
		req.Host = req.URL.Host
		req.Header.Del("Host")
	}
	return err
}

func (s *OpenAIGatewayService) buildOpenAIResponsesWSURLForContext(ctx context.Context, account *Account) (string, error) {
	credentialAccount, err := resolveCredentialAccount(ctx, s.accountRepo, account)
	if err != nil {
		return "", err
	}
	return s.buildOpenAIResponsesWSURL(credentialAccount)
}
