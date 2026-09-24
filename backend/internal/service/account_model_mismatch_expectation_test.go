package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type mismatchCredentialCASRepo struct {
	modelMismatchMarkerStub
	current *Account
	failure error
}

func (r *mismatchCredentialCASRepo) RecordAccountModelMismatch(ctx context.Context, id int64, expected, actual, requestID string, gates ...func([]int64)) (bool, error) {
	if !ModelMismatchCredentialExpectationMatches(ctx, r.current) {
		return false, nil
	}
	for _, gate := range gates {
		gate([]int64{id})
	}
	if r.failure != nil {
		return false, r.failure
	}
	return true, r.MarkAccountModelMismatch(ctx, id, expected, actual, requestID)
}

func TestModelMismatchCredentialExpectationIsImmutableAndChecksIdentity(t *testing.T) {
	account := &Account{ID: 9880, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{
		"access_token": "old-at", "refresh_token": "old-rt", "_token_version": int64(1), "chatgpt_account_id": "workspace-a", "chatgpt_user_id": "user-a",
	}}
	ctx := WithModelMismatchCredentialExpectation(context.Background(), account)
	require.True(t, ModelMismatchCredentialExpectationMatches(ctx, account))
	for key, value := range map[string]any{"access_token": "new-at", "refresh_token": "new-rt", "_token_version": int64(2), "chatgpt_account_id": "workspace-b", "chatgpt_user_id": "user-b"} {
		fresh := *account
		fresh.Credentials = shallowCopyMap(account.Credentials)
		fresh.Credentials[key] = value
		require.False(t, ModelMismatchCredentialExpectationMatches(ctx, &fresh), key)
	}
	account.Credentials["access_token"] = "new-at"
	require.False(t, ModelMismatchCredentialExpectationMatches(ctx, account), "captured values must not share the account's mutable map")
	require.True(t, ModelMismatchCredentialExpectationMatches(context.Background(), account), "direct repository callers remain compatible")
}

func TestModelMismatchPendingRetryKeepsCredentialCASAndClearsStaleGate(t *testing.T) {
	account := &Account{ID: 9881, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"access_token": "old-at", "refresh_token": "old-rt"}}
	defer ClearPendingAccountModelMismatch(account.ID)
	repo := &mismatchCredentialCASRepo{current: account, failure: errors.New("temporary write failure")}
	require.False(t, quarantineAccountModelMismatch(context.Background(), repo, account, "gpt-6-astra", "gpt-5.6-luna", "old-probe"))
	value, exists := pendingAccountModelMismatches.Load(account.ID)
	require.True(t, exists)
	state, ok := value.(*pendingAccountModelMismatch)
	require.True(t, ok)
	state.checkedAt = time.Time{}
	fresh := *account
	fresh.Credentials = map[string]any{"access_token": "new-at", "refresh_token": "new-rt"}
	repo.current, repo.failure = &fresh, nil
	require.False(t, hasPendingAccountModelMismatch(&fresh), "a known CAS miss removes only the old pending generation")
	require.Zero(t, repo.calls, "the old probe must not write evidence for newly authorized credentials")
	_, exists = pendingAccountModelMismatches.Load(account.ID)
	require.False(t, exists)
}

func TestModelMismatchFixedRouteExpectationRejectsIdentityAndProxyChanges(t *testing.T) {
	account := stateTicketTestAccount(9882)
	for _, change := range []string{"identity", "protection", "proxy"} {
		t.Run(change, func(t *testing.T) {
			ctx := WithModelMismatchFixedRouteExpectation(WithModelMismatchCredentialExpectation(context.Background(), account), account)
			require.True(t, ModelMismatchCredentialExpectationMatches(ctx, account))
			fresh := cloneStateTicketAccount(account)
			switch change {
			case "identity":
				fresh.Extra["openai_device_id"] = "replacement-device"
			case "protection":
				PrepareNewAccountProtection(fresh)
			case "proxy":
				fresh.Proxy.Host = "replacement.fixed.invalid"
			}
			require.False(t, ModelMismatchCredentialExpectationMatches(ctx, fresh))
			require.True(t, ModelMismatchCredentialExpectationMatches(WithModelMismatchCredentialExpectation(context.Background(), account), fresh), "ordinary credential-only inference keeps its existing semantics")
		})
	}
}

func TestModelMismatchPendingRetryKeepsFixedRouteExpectation(t *testing.T) {
	account := stateTicketTestAccount(9883)
	defer ClearPendingAccountModelMismatch(account.ID)
	repo := &mismatchCredentialCASRepo{current: account, failure: errors.New("temporary write failure")}
	ctx := WithModelMismatchFixedRouteExpectation(context.Background(), account)
	require.False(t, quarantineAccountModelMismatch(ctx, repo, account, "gpt-6-astra", "gpt-5.6-luna", "old-fixed-probe"))
	value, exists := pendingAccountModelMismatches.Load(account.ID)
	require.True(t, exists)
	state, ok := value.(*pendingAccountModelMismatch)
	require.True(t, ok)
	state.checkedAt = time.Time{}
	fresh := cloneStateTicketAccount(account)
	fresh.Proxy.Host = "replacement.fixed.invalid"
	repo.current, repo.failure = fresh, nil
	require.False(t, hasPendingAccountModelMismatch(fresh))
	require.Zero(t, repo.calls)
	_, exists = pendingAccountModelMismatches.Load(account.ID)
	require.False(t, exists)
}
