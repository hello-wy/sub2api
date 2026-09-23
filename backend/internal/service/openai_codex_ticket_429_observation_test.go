//go:build unit

package service

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCodexTicket429DoesNotDelayManualOrScheduledAcquisition(t *testing.T) {
	previous := Context429Enforcement(context.Background())
	Set429EnforcementEnabled(false)
	t.Cleanup(func() { Set429EnforcementEnabled(previous) })
	a := stateTicketTestAccount(8121)
	expires := time.Now().Add(time.Hour)
	a.RateLimitResetAt = &expires // A previously recorded upstream reset is informational.
	old := stateTicketVerified(a, time.Now().Add(-20*time.Minute))
	a.Extra[openAICodexTicketExtraKey(old.Model)] = old
	s, repo := stateTicketTestService(t, a)
	var probes atomic.Int32
	s.openaiCodexTicketProbe = func(context.Context, *Account, string, string, string, string, time.Duration) (string, int, error) {
		probes.Add(1)
		return "", 429, nil
	}
	require.Nil(t, codexTicketHarvestRetryAfter(context.Background(), a))
	for i := 0; i < 2; i++ {
		waitStateTicketJob(t, s.startCodexAccountTicketJob(context.Background(), a.ID, true))
	}
	require.EqualValues(t, 2, probes.Load())
	status, err := s.GetCodexAccountTicketStatus(context.Background(), a.ID)
	require.NoError(t, err)
	require.Nil(t, status.RetryAfter)
	require.Contains(t, status.LastError, "429")
	require.True(t, status.TicketUsable)
	live, err := repo.GetByID(context.Background(), a.ID)
	require.NoError(t, err)
	require.Equal(t, old.State, s.lookupOpenAICodexTicket(live, old.Model).State)
}
