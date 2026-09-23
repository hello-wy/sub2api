package admin

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type codexTicketControlStub struct {
	input     service.CodexAccountTicketUpdate
	accountID int64
	calls     int
	err       error
}

func (s *codexTicketControlStub) GetCodexAccountTicketStatus(_ context.Context, id int64) (*service.CodexAccountTicketStatus, error) {
	s.accountID = id
	s.calls++
	return &service.CodexAccountTicketStatus{TicketPlan: "pro", TargetLength: 292}, s.err
}
func (s *codexTicketControlStub) ConfigureCodexAccountTicket(ctx context.Context, id int64, input service.CodexAccountTicketUpdate) (*service.CodexAccountTicketStatus, error) {
	s.input = input
	return s.GetCodexAccountTicketStatus(ctx, id)
}
func (s *codexTicketControlStub) HarvestCodexAccountTicket(ctx context.Context, id int64) (*service.CodexAccountTicketStatus, error) {
	return s.GetCodexAccountTicketStatus(ctx, id)
}

func codexTicketControlContext(method, body, id string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, "/test", strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Params = gin.Params{{Key: "id", Value: id}}
	return ctx, recorder
}

func TestCodexAccountTicketControlsContract(t *testing.T) {
	s := &codexTicketControlStub{}
	h := &AccountHandler{codexAccountTickets: s}
	ctx, rec := codexTicketControlContext(http.MethodGet, "", "41")
	h.GetCodexAccountTicket(ctx)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"ticket_plan":"pro"`)
	require.Equal(t, int64(41), s.accountID)
	ctx, rec = codexTicketControlContext(http.MethodPut, `{"enabled":true,"ticket_plan":"team","model":"gpt-6-astra"}`, "42")
	h.UpdateCodexAccountTicket(ctx)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, int64(42), s.accountID)
	require.Equal(t, service.CodexAccountTicketUpdate{Enabled: true, TicketPlan: "team", Model: "gpt-6-astra"}, s.input)
	ctx, rec = codexTicketControlContext(http.MethodPost, "", "42")
	h.HarvestCodexAccountTicket(ctx)
	require.Equal(t, http.StatusAccepted, rec.Code)
}

func TestCodexAccountTicketControlsRejectInvalidInputs(t *testing.T) {
	for _, id := range []string{"", "no", "0", "-1", "9223372036854775808"} {
		ctx, rec := codexTicketControlContext(http.MethodGet, "", id)
		(&AccountHandler{}).GetCodexAccountTicket(ctx)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	}
	s := &codexTicketControlStub{}
	for _, body := range []string{`{}`, `{"enabled":null}`, `{"enabled":"private-secret"}`, `{"enabled":true,"proxy_url":` + `"` + strings.Repeat("x", 17000) + `"}`} {
		ctx, rec := codexTicketControlContext(http.MethodPut, body, "1")
		(&AccountHandler{codexAccountTickets: s}).UpdateCodexAccountTicket(ctx)
		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.NotContains(t, rec.Body.String(), "private-secret")
	}
	require.Zero(t, s.calls)
	ctx, rec := codexTicketControlContext(http.MethodGet, "", "1")
	(&AccountHandler{}).GetCodexAccountTicket(ctx)
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestCodexAccountTicketControlsHideInternalErrors(t *testing.T) {
	ctx, rec := codexTicketControlContext(http.MethodGet, "", "1")
	(&AccountHandler{codexAccountTickets: &codexTicketControlStub{err: errors.New("http://secret-user:secret-password@pool.invalid")}}).GetCodexAccountTicket(ctx)
	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.NotContains(t, rec.Body.String(), "secret-user")
	require.NotContains(t, rec.Body.String(), "secret-password")
}
