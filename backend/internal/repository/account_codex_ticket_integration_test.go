//go:build unit

package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestCodexTicketPostgresManagedWritesAndChannelIsolation(t *testing.T) {
	r, c, ctx := ipChannelIntegrationRepo(t)
	accounts := createIPChannelFixture(t, r, c, ctx, "state-family", 2)
	root, err := r.JoinAccountIPChannels(ctx, []int64{accounts[0].ID, accounts[1].ID})
	require.NoError(t, err)
	id := accounts[0].ID
	stale, err := r.GetByID(ctx, id)
	require.NoError(t, err)
	const key = "codex_turn_ticket:gpt-6-astra"
	const v2Key = "codex_turn_ticket:v2:gpt-6-astra"
	updates := map[string]any{
		"codex_ticket_config": map[string]any{"enabled": true, "revision": "current"},
		key:                   map[string]any{"state": "private-state", "captured_at": "newer"},
		v2Key:                 map[string]any{"state": "private-v2-state", "captured_at": "verified-current"},
	}
	require.NoError(t, r.MutateOpenAICodexTicketExtra(ctx, id, func(a *service.Account) (map[string]any, error) {
		require.NotNil(t, a.Proxy, "validation must see the actual fixed proxy")
		return updates, nil
	}))
	// A stale full edit, generic partial edit and bulk edit cannot erase or
	// forge a verified ticket. They can still change unrelated metadata.
	stale.Name = "renamed"
	require.NoError(t, r.Update(ctx, stale))
	require.NoError(t, r.UpdateExtra(ctx, id, map[string]any{key: nil, v2Key: nil, "codex_ticket_config": nil, "ordinary": true}))
	_, err = r.BulkUpdate(ctx, []int64{id}, service.AccountBulkUpdate{Extra: map[string]any{key: "forged", v2Key: "forged-v2", "codex_turn_ticket:other": "forged", "codex_turn_ticket:v2:other": "forged"}})
	require.NoError(t, err)
	a, err := r.GetByID(ctx, id)
	require.NoError(t, err)
	require.Equal(t, updates[key], a.Extra[key])
	require.Equal(t, updates[v2Key], a.Extra[v2Key])
	require.Equal(t, updates["codex_ticket_config"], a.Extra["codex_ticket_config"])
	require.Equal(t, true, a.Extra["ordinary"])
	require.NotContains(t, a.Extra, "codex_turn_ticket:other")
	require.NotContains(t, a.Extra, "codex_turn_ticket:v2:other")
	sibling, err := r.GetByID(ctx, accounts[1].ID)
	require.NoError(t, err)
	require.NotContains(t, sibling.Extra, key)
	require.NotContains(t, sibling.Extra, v2Key)
	// Publishing STATE never resumes an existing whole-family quarantine.
	require.NoError(t, r.MarkAccountModelMismatch(ctx, id, "gpt-6-astra", "gpt-5.6-luna", "test-request"))
	require.NoError(t, r.MutateOpenAICodexTicketExtra(ctx, id, func(_ *service.Account) (map[string]any, error) { return updates, nil }))
	channels, err := r.GetAccountIPChannels(ctx, []int64{root})
	require.NoError(t, err)
	for _, ch := range channels[root] {
		require.False(t, ch.LogicalEnabled)
		require.False(t, ch.Account.Schedulable)
		require.True(t, ch.Account.HasModelMismatch())
	}
	// Even the dedicated callback cannot mutate unrelated fields.
	err = r.MutateOpenAICodexTicketExtra(ctx, id, func(_ *service.Account) (map[string]any, error) {
		return map[string]any{"model_mismatch": nil}, nil
	})
	require.Error(t, err)
}

func TestCodexTicketPostgresMutationsSerializeAndRollback(t *testing.T) {
	r, c, ctx := ipChannelIntegrationRepo(t)
	a := createIPChannelFixture(t, r, c, ctx, "state-concurrent", 1)[0]
	entered, release := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- r.MutateOpenAICodexTicketExtra(ctx, a.ID, func(_ *service.Account) (map[string]any, error) {
			close(entered)
			<-release
			return map[string]any{"codex_ticket_config": map[string]any{"revision": "new"}}, nil
		})
	}()
	<-entered
	other := make(chan error, 1)
	go func() {
		other <- r.MutateOpenAICodexTicketExtra(ctx, a.ID, func(current *service.Account) (map[string]any, error) {
			if current.Extra["codex_ticket_config"].(map[string]any)["revision"] != "new" {
				return nil, errors.New("mutation observed a stale snapshot")
			}
			return map[string]any{"codex_ticket_watchdog": map[string]any{"trigger_count": 1}}, nil
		})
	}()
	close(release)
	require.NoError(t, <-done)
	require.NoError(t, <-other)
	cancelled, cancel := context.WithTimeout(ctx, time.Second)
	cancel()
	require.Error(t, r.MutateOpenAICodexTicketExtra(cancelled, a.ID, func(_ *service.Account) (map[string]any, error) {
		return map[string]any{"codex_ticket_config": nil}, nil
	}))
	current, err := r.GetByID(ctx, a.ID)
	require.NoError(t, err)
	require.Equal(t, "new", current.Extra["codex_ticket_config"].(map[string]any)["revision"])
}
