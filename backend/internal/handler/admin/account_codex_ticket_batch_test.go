package admin

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type codexTicketBatchControlStub struct {
	codexTicketControlStub
	batch service.CodexAccountTicketBatchUpdate
}

func (s *codexTicketBatchControlStub) BatchCodexAccountTickets(_ context.Context, input service.CodexAccountTicketBatchUpdate) (*service.CodexAccountTicketBatchResult, error) {
	s.calls++
	s.batch = input
	return &service.CodexAccountTicketBatchResult{SelectedAccounts: 1, TotalChannels: 2, Saved: 2, Harvesting: 1, Waiting: 1, Results: []service.CodexAccountTicketBatchItem{{AccountID: 42, LogicalAccountID: 41, Saved: true, Outcome: "waiting", Message: "waiting for a slot"}}}, s.err
}

func TestCodexAccountTicketBatchControlsContract(t *testing.T) {
	s := &codexTicketBatchControlStub{}
	ctx, rec := codexTicketControlContext(http.MethodPost, `{"account_ids":[41],"enabled":true,"ticket_plan":"team","model":"gpt-6-astra","harvest":true}`, "")
	(&AccountHandler{codexAccountTickets: s}).BatchCodexAccountTickets(ctx)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, service.CodexAccountTicketBatchUpdate{AccountIDs: []int64{41}, Enabled: true, TicketPlan: "team", Model: "gpt-6-astra", Harvest: true}, s.batch)
	require.Contains(t, rec.Body.String(), `"outcome":"waiting"`)
	require.Contains(t, rec.Body.String(), `"logical_account_id":41`)
	require.NotContains(t, rec.Body.String(), `"success":true`)
}

func TestCodexAccountTicketBatchControlsRejectInvalidBody(t *testing.T) {
	s := &codexTicketBatchControlStub{}
	for _, body := range []string{`{}`, `{"enabled":null}`, `{"enabled":"private-secret"}`, `{"enabled":true,"account_ids":["secret"]}`, `{"enabled":true,"model":"` + strings.Repeat("x", 17000) + `"}`} {
		ctx, rec := codexTicketControlContext(http.MethodPost, body, "")
		(&AccountHandler{codexAccountTickets: s}).BatchCodexAccountTickets(ctx)
		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.NotContains(t, rec.Body.String(), "private-secret")
	}
	require.Zero(t, s.calls)
	ctx, rec := codexTicketControlContext(http.MethodPost, `{"enabled":true}`, "")
	(&AccountHandler{codexAccountTickets: &codexTicketControlStub{}}).BatchCodexAccountTickets(ctx)
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestCodexAccountTicketBatchControlsHideServiceErrors(t *testing.T) {
	s := &codexTicketBatchControlStub{}
	s.err = errors.New("http://user:private-password@pool.invalid")
	ctx, rec := codexTicketControlContext(http.MethodPost, `{"account_ids":[41],"enabled":false}`, "")
	(&AccountHandler{codexAccountTickets: s}).BatchCodexAccountTickets(ctx)
	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.NotContains(t, rec.Body.String(), "private-password")
}
