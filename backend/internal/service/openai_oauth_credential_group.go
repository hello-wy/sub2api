package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

const OpenAIOAuthCredentialGroupKey = "_oauth_credential_group"
const OpenAIOAuthSourceFingerprintKey = "_oauth_source_fingerprint"
const OpenAIOAuthSourceFingerprintsKey = "_oauth_source_fingerprints"

// Registration is deliberately optional so repository substitutes for unrelated
// platforms do not need to implement an OAuth mutation transaction.
type OpenAIOAuthCredentialGroupRegistrar interface {
	RegisterOpenAIOAuthCredentialGroup(context.Context, []int64, map[string]any) (map[string]any, error)
}

type OpenAIOAuthCredentialCoordinator interface {
	RefreshOpenAIOAuthCredentials(context.Context, *Account, func(context.Context, *Account) (map[string]any, error)) (*Account, error)
}

// Health mutations are serialized with credential rotation. Authentication
// failures apply only to the rejected generation; quota belongs to the whole
// registered upstream identity, irrespective of which fixed IP saw the 429.
type OpenAIOAuthHealthMutation struct {
	Reason            string
	Permanent         bool
	MatchRefreshToken bool
	AuthCooldownUntil *time.Time
	RateLimitUntil    *time.Time
}

type openAIUpstreamAccessTokenContextKey struct{}

// Carries only the token actually sent on this attempt; never log this value.
func WithOpenAIUpstreamAccessToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, openAIUpstreamAccessTokenContextKey{}, token)
}

type OpenAIOAuthHealthRepository interface {
	ApplyOpenAIOAuthHealth(context.Context, *Account, OpenAIOAuthHealthMutation) ([]*Account, error)
	ClearOpenAIOAuthRefreshCooldown(context.Context, *Account) ([]*Account, error)
}

func IsOAuthRefreshCooldown(reason string) bool {
	return strings.HasPrefix(reason, "OAuth 401:") ||
		strings.HasPrefix(reason, "Authentication failed (401):") ||
		strings.HasPrefix(reason, "token refresh retry exhausted:")
}

type openAIOAuthCoordinatorContextKey struct{}

type openAIRegisteredCredentialSnapshotKey struct{}

// WithOpenAIRegisteredCredentialSnapshot marks credentials produced by the
// group registrar/coordinator, never an arbitrary admin credential edit.
// Their token fields may have rotated again before an outer account write.
func WithOpenAIRegisteredCredentialSnapshot(ctx context.Context) context.Context {
	return context.WithValue(ctx, openAIRegisteredCredentialSnapshotKey{}, true)
}

func IsOpenAIRegisteredCredentialSnapshot(ctx context.Context) bool {
	marked, _ := ctx.Value(openAIRegisteredCredentialSnapshotKey{}).(bool)
	return marked
}

func withOpenAIOAuthCoordinator(ctx context.Context) context.Context {
	return context.WithValue(ctx, openAIOAuthCoordinatorContextKey{}, true)
}

func WithOpenAIRefreshCoordinator(ctx context.Context) context.Context {
	return withOpenAIOAuthCoordinator(ctx)
}

func inOpenAIOAuthCoordinator(ctx context.Context) bool {
	v, _ := ctx.Value(openAIOAuthCoordinatorContextKey{}).(bool)
	return v
}

func (s *adminServiceImpl) RegisterOpenAIOAuthCredentialGroup(ctx context.Context, ids []int64, credentials map[string]any) (map[string]any, error) {
	repo, ok := s.accountRepo.(OpenAIOAuthCredentialGroupRegistrar)
	if !ok {
		return nil, errors.New("OpenAI credential group registration is unavailable")
	}
	return repo.RegisterOpenAIOAuthCredentialGroup(ctx, ids, credentials)
}

// Only provider credentials are shared; proxy, model mapping, limits and every
// scheduler field remain properties of the individual account record.
var OpenAIOAuthSharedCredentialKeys = []string{
	"access_token", "refresh_token", "id_token", "expires_at", "expires_in", "token_type", "scope", "client_id",
	"email", "chatgpt_account_id", "chatgpt_user_id", "organization_id", "plan_type", "subscription_expires_at",
	"auth_mode", "_token_version", OpenAIOAuthCredentialGroupKey, OpenAIOAuthSourceFingerprintKey, OpenAIOAuthSourceFingerprintsKey,
}

func OpenAIOAuthCredentialPatch(credentials map[string]any) map[string]any {
	patch := make(map[string]any)
	for _, key := range OpenAIOAuthSharedCredentialKeys {
		if value, ok := credentials[key]; ok {
			patch[key] = value
		}
	}
	return patch
}

func OpenAIOAuthSourceFingerprint(credentials map[string]any) string {
	account := &Account{Credentials: credentials}
	value := "refresh:" + strings.TrimSpace(account.GetCredential("refresh_token"))
	if value == "refresh:" {
		value = "access:" + strings.TrimSpace(account.GetCredential("access_token"))
	}
	hash := sha256.Sum256([]byte(value))
	return hex.EncodeToString(hash[:])
}

func openAITokenInfoFromAccount(account *Account) *OpenAITokenInfo {
	info := &OpenAITokenInfo{AccessToken: account.GetCredential("access_token"), RefreshToken: account.GetCredential("refresh_token"),
		IDToken: account.GetCredential("id_token"), ClientID: account.GetCredential("client_id"), Email: account.GetCredential("email"),
		ChatGPTAccountID: account.GetCredential("chatgpt_account_id"), ChatGPTUserID: account.GetCredential("chatgpt_user_id"),
		OrganizationID: account.GetCredential("organization_id"), PlanType: account.GetCredential("plan_type"),
		SubscriptionExpiresAt: account.GetCredential("subscription_expires_at"), AuthMode: account.GetCredential("auth_mode")}
	if expiry := account.GetCredentialAsTime("expires_at"); expiry != nil {
		info.ExpiresAt = expiry.Unix()
		info.ExpiresIn = int64(time.Until(*expiry).Seconds())
	}
	return info
}

// Explicit upstream codes take precedence over generic error types/messages.
// Do not infer revoked credentials from free-form text that may echo a prompt.
func isOpenAIAuthenticationErrorCode(code string) bool {
	switch strings.ToLower(strings.TrimSpace(code)) {
	case "invalid_api_key", "api_key_disabled", "unauthorized", "authentication_error",
		"invalid_token", "access_token_invalid", "token_expired", "access_token_expired",
		"token_revoked", "token_invalidated", "invalid_grant", "invalid_credentials", "credential_invalid":
		return true
	default:
		return false
	}
}
