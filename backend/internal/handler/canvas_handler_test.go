//go:build unit

package handler

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type canvasSettingRepo struct {
	service.SettingRepository
	enabled bool
}

func (r canvasSettingRepo) GetMultiple(context.Context, []string) (map[string]string, error) {
	data, _ := json.Marshal(service.CanvasSettings{Enabled: r.enabled, URL: "https://canvas.example"})
	return map[string]string{service.SettingKeyCanvas: string(data)}, nil
}
func TestCanvasSettingsResponse(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		handler := &SettingHandler{settingService: service.NewSettingService(canvasSettingRepo{enabled: enabled}, nil)}
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest("GET", "/canvas/settings", nil)
		handler.GetCanvasSettings(c)
		require.Equal(t, 200, recorder.Code, recorder.Body.String())
		require.Contains(t, recorder.Body.String(), "https://canvas.example")
		require.NotContains(t, recorder.Body.String(), "apiKey")
		require.NotContains(t, recorder.Body.String(), "baseUrl")
		require.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
	}
}
