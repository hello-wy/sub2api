package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCodexTicketV2PublicationAndRevocationPreserveLegacyTraffic(t *testing.T) {
	ctx := context.Background()
	account := stateTicketTestAccount(8220)
	legacy := stateTicketVerified(account, time.Now().Add(-time.Minute))
	legacyKey := openAICodexTicketExtraKeyPrefix + legacy.Model
	account.Extra[legacyKey] = legacy
	s, repo := stateTicketTestService(t, account)
	require.Equal(t, "codex_turn_ticket:v2:gpt-6-astra", openAICodexTicketExtraKey(legacy.Model))
	// Even a blob shaped like a v2 proof under the legacy key is not trusted.
	require.Nil(t, s.lookupOpenAICodexTicket(account, legacy.Model))
	require.False(t, s.CodexAccountTicketStatusFromAccount(ctx, account).TicketUsable)
	current := stateTicketVerified(account, time.Now())
	require.NoError(t, s.publishCodexTicket(ctx, current, s.openAICodexTicketHarvestProxyURLContext(ctx)))
	live, err := repo.GetByID(ctx, account.ID)
	require.NoError(t, err)
	require.Equal(t, legacy, live.Extra[legacyKey])
	require.True(t, receiptForCodexTicket(current).matches(s.lookupOpenAICodexTicket(live, current.Model)))
	// Revoke only new-version evidence; do not start a real acquisition job.
	s.cfg.Gateway.OpenAICodexTicket.HarvestProxyURL = ""
	require.True(t, s.invalidateCodexTicketFromResponse(receiptForCodexTicket(current), "state_312"))
	live, err = repo.GetByID(ctx, account.ID)
	require.NoError(t, err)
	require.Equal(t, legacy, live.Extra[legacyKey])
	require.Nil(t, live.Extra[openAICodexTicketExtraKey(current.Model)])
	require.Nil(t, s.lookupOpenAICodexTicket(live, current.Model))
	// An explicit operator opt-out deliberately clears both generations.
	_, err = s.ConfigureCodexAccountTicket(ctx, account.ID, CodexAccountTicketUpdate{Enabled: false})
	require.NoError(t, err)
	live, err = repo.GetByID(ctx, account.ID)
	require.NoError(t, err)
	require.Nil(t, live.Extra[legacyKey])
	require.Nil(t, live.Extra[openAICodexTicketExtraKey(current.Model)])
}

func TestCodexTicketBothVersionsStayPrivateAndManaged(t *testing.T) {
	legacyKey, currentKey := "codex_turn_ticket:gpt-6-astra", openAICodexTicketExtraKey("gpt-6-astra")
	stored := map[string]any{legacyKey: "legacy-private", currentKey: "current-private", "ordinary": "visible"}
	for _, key := range []string{legacyKey, currentKey} {
		require.True(t, IsOpenAICodexTicketExtraKey(key))
		require.True(t, IsOpenAICodexTicketPrivateExtraKey(key))
	}
	require.Equal(t, map[string]any{"ordinary": "visible"}, RedactOpenAICodexTicketExtra(stored))
	merged := MergeOpenAICodexTicketExtra(map[string]any{legacyKey: "forged", currentKey: "forged", "ordinary": "edited"}, stored)
	require.Equal(t, "legacy-private", merged[legacyKey])
	require.Equal(t, "current-private", merged[currentKey])
	require.Equal(t, "edited", merged["ordinary"])
	imported := MergeOpenAICodexTicketExtra(stored, nil)
	require.NotContains(t, imported, legacyKey)
	require.NotContains(t, imported, currentKey)
}
