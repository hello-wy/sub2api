package admin

import (
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestNewAccountCodexTicketDefaultsHandlerContract(t *testing.T) {
	repo := &codexTicketSettingHandlerRepo{}
	h := &SettingHandler{settingService: service.NewSettingService(repo, nil)}
	ctx, rec := codexTicketControlContext(http.MethodGet, "", "")
	h.GetNewAccountCodexTicketDefaults(ctx)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"enabled":false`)
	require.Contains(t, rec.Body.String(), `"ticket_plan":"pro"`)
	ctx, rec = codexTicketControlContext(http.MethodPut, `{"enabled":true,"ticket_plan":"team","model":"untrusted"}`, "")
	h.UpdateNewAccountCodexTicketDefaults(ctx)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"enabled":true`)
	require.Contains(t, rec.Body.String(), `"ticket_plan":"team"`)
	require.Contains(t, rec.Body.String(), `"model":"gpt-6-astra"`)
	ctx, rec = codexTicketControlContext(http.MethodGet, "", "")
	h.GetNewAccountCodexTicketDefaults(ctx)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"ticket_plan":"team"`)
	for _, body := range []string{`{}`, `{"enabled":null,"ticket_plan":"pro"}`, `{"enabled":true}`, `{"enabled":true,"ticket_plan":"312"}`} {
		ctx, rec = codexTicketControlContext(http.MethodPut, body, "")
		h.UpdateNewAccountCodexTicketDefaults(ctx)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	}
	require.Len(t, repo.values, 1)
}
