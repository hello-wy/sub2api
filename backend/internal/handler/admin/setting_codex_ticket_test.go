package admin

import (
	"context"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type codexTicketSettingHandlerRepo struct {
	service.SettingRepository
	values map[string]string
}

func (r *codexTicketSettingHandlerRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := map[string]string{}
	for _, key := range keys {
		if value, exists := r.values[key]; exists {
			out[key] = value
		}
	}
	return out, nil
}
func (r *codexTicketSettingHandlerRepo) SetMultiple(_ context.Context, updates map[string]string) error {
	if r.values == nil {
		r.values = map[string]string{}
	}
	for key, value := range updates {
		r.values[key] = value
	}
	return nil
}

func TestCodexTicketSettingHandlerWriteOnlyContract(t *testing.T) {
	r := &codexTicketSettingHandlerRepo{}
	h := &SettingHandler{settingService: service.NewSettingService(r, nil)}
	ctx, rec := codexTicketControlContext(http.MethodPut, `{"enabled":true,"harvest_proxy_url":"http://secret-user-{sid}:secret-password@pool.example.test:8080"}`, "")
	h.UpdateCodexTicketSettings(ctx)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"harvest_proxy_configured":true`)
	require.NotContains(t, rec.Body.String(), "secret-user")
	require.NotContains(t, rec.Body.String(), "secret-password")
	ctx, rec = codexTicketControlContext(http.MethodGet, "", "")
	h.GetCodexTicketSettings(ctx)
	require.Equal(t, http.StatusOK, rec.Code)
	require.NotContains(t, rec.Body.String(), "secret-password")
	ctx, rec = codexTicketControlContext(http.MethodPut, `{"enabled":false,"clear_proxy":true}`, "")
	h.UpdateCodexTicketSettings(ctx)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"harvest_proxy_configured":false`)
}

func TestCodexTicketSettingHandlerRequiresEnabled(t *testing.T) {
	for _, body := range []string{`{}`, `{"enabled":null}`, `{"enabled":"secret-password"}`} {
		ctx, rec := codexTicketControlContext(http.MethodPut, body, "")
		(&SettingHandler{}).UpdateCodexTicketSettings(ctx)
		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.NotContains(t, rec.Body.String(), "secret-password")
	}
}

func TestCodexTicketSettingRequestAuditDoesNotRetainProxyCredentials(t *testing.T) {
	raw := []byte(`{"enabled":true,"harvest_proxy_url":"http://secret-user:secret-password@pool.example.test:8080"}`)
	redacted := service.RedactAuditBody(raw, "application/json")
	require.NotContains(t, redacted, "secret-user")
	require.NotContains(t, redacted, "secret-password")
	require.Contains(t, redacted, `"enabled":true`)
}
