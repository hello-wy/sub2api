//go:build unit

package repository

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestOpenAIHealthPostgresSharesCooldownAndProtectsTokenGeneration(t *testing.T) {
	r, c, ctx := openAICredentialGroupIntegrationRepo(t)
	a := createGroupTestAccount(t, ctx, c, "group-a", "rt-1", "at-1")
	b := createGroupTestAccount(t, ctx, c, "group-a", "rt-1", "at-1")
	other := createGroupTestAccount(t, ctx, c, "group-b", "rt-other", "at-other")
	_, err := c.Account.UpdateOneID(b.ID).SetSchedulable(false).SetExtra(map[string]any{"model_mismatch": map[string]any{"expected_model": "gpt-6-astra", "actual_model": "gpt-5.6-luna", "quarantined": true}}).Save(ctx)
	require.NoError(t, err)
	until := time.Now().Add(time.Hour)
	changed, err := r.ApplyOpenAIOAuthHealth(ctx, a, service.OpenAIOAuthHealthMutation{RateLimitUntil: &until})
	require.NoError(t, err)
	require.Len(t, changed, 2)
	shorter := time.Now().Add(5 * time.Second)
	_, err = r.ApplyOpenAIOAuthHealth(ctx, b, service.OpenAIOAuthHealthMutation{RateLimitUntil: &shorter})
	require.NoError(t, err)
	read, err := r.GetByID(ctx, a.ID)
	require.NoError(t, err)
	require.WithinDuration(t, until, *read.RateLimitResetAt, time.Millisecond)
	readOther, err := r.GetByID(ctx, other.ID)
	require.NoError(t, err)
	require.Nil(t, readOther.RateLimitResetAt)

	authUntil := time.Now().Add(10 * time.Minute)
	changed, err = r.ApplyOpenAIOAuthHealth(ctx, a, service.OpenAIOAuthHealthMutation{AuthCooldownUntil: &authUntil, Reason: "OAuth 401: refresh required"})
	require.NoError(t, err)
	require.Len(t, changed, 2)
	read, err = r.GetByID(ctx, b.ID)
	require.NoError(t, err)
	require.Equal(t, "rt-1", read.GetCredential("refresh_token"))
	require.True(t, read.GetCredentialAsTime("expires_at").Before(time.Now()))
	require.False(t, read.Schedulable)
	require.True(t, read.IsModelMismatchQuarantined())
	// Simulate successful coordinated rotation, then a delayed rejection of at-1.
	_, err = r.RefreshOpenAIOAuthCredentials(ctx, a, func(_ context.Context, _ *service.Account) (map[string]any, error) {
		return map[string]any{"access_token": "at-2", "refresh_token": "rt-2", "expires_at": time.Now().Add(time.Hour).Format(time.RFC3339), service.OpenAIOAuthCredentialGroupKey: "group-a"}, nil
	})
	require.NoError(t, err)
	changed, err = r.ApplyOpenAIOAuthHealth(ctx, a, service.OpenAIOAuthHealthMutation{Permanent: true, Reason: "revoked"})
	require.NoError(t, err)
	require.Empty(t, changed)
	fresh, err := r.GetByID(ctx, a.ID)
	require.NoError(t, err)
	changed, err = r.ClearOpenAIOAuthRefreshCooldown(ctx, fresh)
	require.NoError(t, err)
	require.Len(t, changed, 2)
	read, err = r.GetByID(ctx, b.ID)
	require.NoError(t, err)
	require.Nil(t, read.TempUnschedulableUntil)
	require.False(t, read.Schedulable)
	require.True(t, read.IsModelMismatchQuarantined())
	require.WithinDuration(t, until, *read.RateLimitResetAt, time.Millisecond)
	changed, err = r.ApplyOpenAIOAuthHealth(ctx, fresh, service.OpenAIOAuthHealthMutation{Permanent: true, MatchRefreshToken: true, Reason: "reauthorization required"})
	require.NoError(t, err)
	require.Len(t, changed, 2)
	for _, member := range changed {
		require.Equal(t, service.StatusError, member.Status)
		require.False(t, member.Schedulable)
	}
}

func TestOpenAIHealthPostgresLate401WaitsForConcurrentRotation(t *testing.T) {
	r, c, ctx := openAICredentialGroupIntegrationRepo(t)
	a := createGroupTestAccount(t, ctx, c, "concurrent-health", "old-rt", "old-at")
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { close(release) }) })
	rotation := make(chan error, 1)
	go func() {
		_, err := r.RefreshOpenAIOAuthCredentials(ctx, a, func(context.Context, *service.Account) (map[string]any, error) {
			close(entered)
			<-release
			return map[string]any{"access_token": "new-at", "refresh_token": "new-rt", service.OpenAIOAuthCredentialGroupKey: "concurrent-health"}, nil
		})
		rotation <- err
	}()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("rotation did not start")
	}
	type result struct {
		accounts []*service.Account
		err      error
	}
	failure := make(chan result, 1)
	go func() {
		changed, err := r.ApplyOpenAIOAuthHealth(ctx, a, service.OpenAIOAuthHealthMutation{Permanent: true, Reason: "revoked old token"})
		failure <- result{changed, err}
	}()
	once.Do(func() { close(release) })
	require.NoError(t, <-rotation)
	got := <-failure
	require.NoError(t, got.err)
	require.Empty(t, got.accounts)
	fresh, err := r.GetByID(ctx, a.ID)
	require.NoError(t, err)
	require.Equal(t, "new-at", fresh.GetCredential("access_token"))
	require.Equal(t, service.StatusActive, fresh.Status)
}

func TestOpenAIHealthPostgresPausedAccountsRemainRefreshCandidates(t *testing.T) {
	r, c, ctx := openAICredentialGroupIntegrationRepo(t)
	a := createGroupTestAccount(t, ctx, c, "paused-group", "rt", "at")
	_, err := c.Account.UpdateOneID(a.ID).SetStatus(service.StatusActive).SetSchedulable(false).Save(ctx)
	require.NoError(t, err)
	page, err := r.ListOAuthRefreshCandidatePage(ctx, service.OAuthRefreshPageOptions{Platforms: []string{service.PlatformOpenAI}, Limit: 20, ActiveOnly: true, RequireRefreshToken: true})
	require.NoError(t, err)
	require.Len(t, page.Accounts, 1)
	require.False(t, page.Accounts[0].Schedulable)
}

func TestOpenAIHealthPostgresUsesCurrentRefreshTokenAndLateJoinInheritsCooldown(t *testing.T) {
	r, c, ctx := openAICredentialGroupIntegrationRepo(t)
	a := createGroupTestAccount(t, ctx, c, "late-health", "current-rt", "current-at")
	stale := *a
	stale.Credentials = map[string]any{"access_token": "current-at"}
	until := time.Now().Add(10 * time.Minute)
	members, err := r.ApplyOpenAIOAuthHealth(ctx, &stale, service.OpenAIOAuthHealthMutation{AuthCooldownUntil: &until, Reason: "OAuth 401: refresh required"})
	require.NoError(t, err)
	require.Len(t, members, 1)
	require.Equal(t, service.StatusActive, members[0].Status, "old request snapshot missing refresh token must not quarantine a currently refreshable account")
	require.Equal(t, "current-rt", members[0].GetCredential("refresh_token"))
	quotaUntil := time.Now().Add(time.Hour)
	_, err = r.ApplyOpenAIOAuthHealth(ctx, a, service.OpenAIOAuthHealthMutation{RateLimitUntil: &quotaUntil})
	require.NoError(t, err)
	late := &service.Account{Name: "new-ip", Platform: service.PlatformOpenAI, Type: service.AccountTypeOAuth, Status: service.StatusActive, Schedulable: true, Credentials: map[string]any{"access_token": "current-at", "refresh_token": "current-rt", service.OpenAIOAuthCredentialGroupKey: "late-health"}}
	require.NoError(t, r.Create(ctx, late))
	read, err := r.GetByID(ctx, late.ID)
	require.NoError(t, err)
	require.WithinDuration(t, quotaUntil, *read.RateLimitResetAt, time.Millisecond)
	require.WithinDuration(t, until, *read.TempUnschedulableUntil, time.Millisecond)

	unrefreshable := createGroupTestAccount(t, ctx, c, "no-rt", "", "at")
	members, err = r.ApplyOpenAIOAuthHealth(ctx, unrefreshable, service.OpenAIOAuthHealthMutation{AuthCooldownUntil: &until, Reason: "OAuth 401: refresh required"})
	require.NoError(t, err)
	require.Len(t, members, 1)
	require.Equal(t, service.StatusError, members[0].Status)
	require.Contains(t, members[0].ErrorMessage, "reauthorization required")
}

func TestOpenAIHealthPostgresStaleEditorsCannotClearHealth(t *testing.T) {
	for _, reimport := range []bool{false, true} {
		t.Run(fmt.Sprintf("reimport=%t", reimport), func(t *testing.T) {
			r, c, ctx := ipChannelIntegrationRepo(t)
			a := createGroupTestAccount(t, ctx, c, "edit-health", "edit-rt", "edit-at")
			b := createGroupTestAccount(t, ctx, c, "edit-health", "edit-rt", "edit-at")
			old, err := r.GetByID(ctx, a.ID)
			require.NoError(t, err)
			until := time.Now().Add(time.Hour)
			_, err = r.ApplyOpenAIOAuthHealth(ctx, a, service.OpenAIOAuthHealthMutation{RateLimitUntil: &until})
			require.NoError(t, err)
			updateCtx := ctx
			if reimport {
				old.Credentials, err = r.RegisterOpenAIOAuthCredentialGroup(ctx, []int64{a.ID}, old.Credentials)
				require.NoError(t, err)
				updateCtx = service.WithOpenAIRegisteredCredentialSnapshot(ctx)
			}
			old.Name = "renamed after rate limit"
			require.NoError(t, r.Update(updateCtx, old))
			for _, id := range []int64{a.ID, b.ID} {
				fresh, err := r.GetByID(ctx, id)
				require.NoError(t, err)
				require.NotNil(t, fresh.RateLimitResetAt)
				require.WithinDuration(t, until, *fresh.RateLimitResetAt, time.Millisecond)
			}
			old, err = r.GetByID(ctx, a.ID)
			require.NoError(t, err)
			_, err = r.ApplyOpenAIOAuthHealth(ctx, old, service.OpenAIOAuthHealthMutation{Permanent: true, Reason: "OAuth 401: token revoked; reauthorization required"})
			require.NoError(t, err)
			if reimport {
				old.Credentials, err = r.RegisterOpenAIOAuthCredentialGroup(ctx, []int64{a.ID}, old.Credentials)
				require.NoError(t, err)
			}
			old.Name = "renamed after revoked token"
			require.NoError(t, r.Update(updateCtx, old))
			for _, id := range []int64{a.ID, b.ID} {
				fresh, err := r.GetByID(ctx, id)
				require.NoError(t, err)
				require.Equal(t, service.StatusError, fresh.Status)
				require.False(t, fresh.Schedulable)
				require.Contains(t, fresh.ErrorMessage, "reauthorization required")
			}

			// Reauthorization plus explicit recovery remains possible. Generic
			// edits must also not restore the old failure after it was cleared.
			beforeRecovery, err := r.GetByID(ctx, a.ID)
			require.NoError(t, err)
			newCredentials, err := r.RegisterOpenAIOAuthCredentialGroup(ctx, []int64{a.ID}, map[string]any{"access_token": "reauthorized-at", "refresh_token": "reauthorized-rt"})
			require.NoError(t, err)
			for _, id := range []int64{a.ID, b.ID} {
				require.NoError(t, r.ClearError(ctx, id))
				require.NoError(t, r.ClearRateLimit(ctx, id))
				require.NoError(t, r.SetSchedulable(ctx, id, true))
			}
			beforeRecovery.Credentials = newCredentials
			beforeRecovery.Name = "edited after explicit recovery"
			require.NoError(t, r.Update(service.WithOpenAIRegisteredCredentialSnapshot(ctx), beforeRecovery))
			fresh, err := r.GetByID(ctx, a.ID)
			require.NoError(t, err)
			require.Equal(t, service.StatusActive, fresh.Status)
			require.True(t, fresh.Schedulable)
			require.Empty(t, fresh.ErrorMessage)
			require.Nil(t, fresh.RateLimitResetAt)
			require.Equal(t, "reauthorized-at", fresh.GetCredential("access_token"))
		})
	}
}

func TestOpenAIHealthPostgresStandaloneEditorKeeps401RefreshMarker(t *testing.T) {
	r, c, ctx := openAICredentialGroupIntegrationRepo(t)
	a := createGroupTestAccount(t, ctx, c, "", "standalone-rt", "standalone-at")
	old, err := r.GetByID(ctx, a.ID)
	require.NoError(t, err)
	old.Credentials["expires_at"] = time.Now().Add(time.Hour).Format(time.RFC3339)
	until := time.Now().Add(10 * time.Minute)
	_, err = r.ApplyOpenAIOAuthHealth(ctx, a, service.OpenAIOAuthHealthMutation{AuthCooldownUntil: &until, Reason: "OAuth 401: refresh required"})
	require.NoError(t, err)
	old.Name = "changed only name"
	require.NoError(t, r.Update(ctx, old))
	fresh, err := r.GetByID(ctx, a.ID)
	require.NoError(t, err)
	require.NotNil(t, fresh.GetCredentialAsTime("expires_at"))
	require.True(t, fresh.GetCredentialAsTime("expires_at").Before(time.Now()))
	require.NotNil(t, fresh.TempUnschedulableUntil)
	require.WithinDuration(t, until, *fresh.TempUnschedulableUntil, time.Millisecond)
}
