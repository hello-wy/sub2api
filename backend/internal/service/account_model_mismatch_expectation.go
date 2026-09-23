package service

import "context"

type modelMismatchCredentialExpectationKey struct{}
type modelMismatchFixedRouteExpectationKey struct{}
type modelMismatchFixedRouteExpectation struct {
	accountID   int64
	fingerprint string
}

// STATE verifies a complete fixed business route, so its evidence also becomes
// stale when identity, protection or proxy settings change before persistence.
// Ordinary inference snapshots do not opt into this stricter full-route check.
func WithModelMismatchFixedRouteExpectation(ctx context.Context, account *Account) context.Context {
	if account == nil {
		return ctx
	}
	fingerprint := codexTicketFixedProxyFingerprint(account)
	if fingerprint == "" {
		return ctx
	}
	return context.WithValue(ctx, modelMismatchFixedRouteExpectationKey{}, &modelMismatchFixedRouteExpectation{accountID: account.ID, fingerprint: fingerprint})
}

// Private immutable values; never log or serialize credential expectations.
type modelMismatchCredentialExpectation struct {
	accountID           int64
	accessToken         string
	refreshToken        string
	tokenVersion        int64
	chatGPTAccountID    string
	chatGPTUserID       string
	checkFullGeneration bool
}

// WithModelMismatchCredentialExpectation captures the credential generation
// used by a probe. Repositories compare it while holding the account row lock,
// so reauthorization between the response and persistence cannot be quarantined.
func WithModelMismatchCredentialExpectation(ctx context.Context, account *Account) context.Context {
	if account == nil || account.Platform != PlatformOpenAI || account.Type != AccountTypeOAuth || account.IsCredentialShadow() || account.GetCredential("access_token") == "" {
		return ctx
	}
	expected := &modelMismatchCredentialExpectation{accountID: account.ID,
		accessToken: account.GetCredential("access_token"), refreshToken: account.GetCredential("refresh_token"),
		tokenVersion: account.GetCredentialAsInt64("_token_version"), chatGPTAccountID: account.GetCredential("chatgpt_account_id"),
		chatGPTUserID: account.GetCredential("chatgpt_user_id"), checkFullGeneration: true}
	// Inference may have refreshed after its scheduling snapshot. The actual
	// outbound token is authoritative; stale snapshot refresh/version fields
	// must not override the generation sent by the provider/pooled socket.
	if actual, _ := ctx.Value(openAIUpstreamAccessTokenContextKey{}).(string); actual != "" && actual != expected.accessToken {
		expected.accessToken = actual
		expected.checkFullGeneration = false
	}
	return context.WithValue(ctx, modelMismatchCredentialExpectationKey{}, expected)
}

func ModelMismatchCredentialExpectationMatches(ctx context.Context, account *Account) bool {
	if route, _ := ctx.Value(modelMismatchFixedRouteExpectationKey{}).(*modelMismatchFixedRouteExpectation); route != nil {
		if account == nil || account.ID != route.accountID || codexTicketFixedProxyFingerprint(account) != route.fingerprint {
			return false
		}
	}
	expected, _ := ctx.Value(modelMismatchCredentialExpectationKey{}).(*modelMismatchCredentialExpectation)
	if expected == nil {
		return true
	}
	return account != nil && account.ID == expected.accountID && account.Platform == PlatformOpenAI && account.Type == AccountTypeOAuth &&
		account.GetCredential("access_token") == expected.accessToken &&
		account.GetCredential("chatgpt_account_id") == expected.chatGPTAccountID && account.GetCredential("chatgpt_user_id") == expected.chatGPTUserID &&
		(!expected.checkFullGeneration || (account.GetCredential("refresh_token") == expected.refreshToken && account.GetCredentialAsInt64("_token_version") == expected.tokenVersion))
}
