package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCodexAccountTicketSummaryUsesSnapshotAndRedactsTicket(t *testing.T) {
	account := stateTicketTestAccount(7001)
	ticket := stateTicketVerified(account, time.Now())
	account.Extra[openAICodexTicketExtraKey(ticket.Model)] = ticket
	s, _ := stateTicketTestService(t, account)
	s.accountRepo = nil // list rendering must not query once per IP channel
	status := s.CodexAccountTicketStatusFromAccount(context.Background(), account)
	require.NotNil(t, status)
	require.True(t, status.TicketUsable)
	require.Equal(t, ticket.Length, status.ActualLength)
	require.NotNil(t, status.ExpiresAt)
	encoded, err := json.Marshal(status)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), ticket.State)

	s.cfg.Gateway.OpenAICodexTicket.Enabled = false
	status = s.CodexAccountTicketStatusFromAccount(context.Background(), account)
	require.Equal(t, "global_disabled", status.State)
	require.Zero(t, status.ActualLength, "target length must never masquerade as an acquired ticket")
	require.False(t, status.TicketUsable)
}
