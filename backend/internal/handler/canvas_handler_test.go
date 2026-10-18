//go:build unit

package handler

import (
	"context"
	"encoding/json"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"testing"
	"time"
)

type canvasKeyRepo struct {
	service.APIKeyRepository
	key *service.APIKey
}

func (r canvasKeyRepo) GetByID(context.Context, int64) (*service.APIKey, error) { return r.key, nil }

type canvasSettingRepo struct {
	service.SettingRepository
	enabled bool
}

func (r canvasSettingRepo) GetMultiple(context.Context, []string) (map[string]string, error) {
	data, _ := json.Marshal(service.CanvasSettings{Enabled: r.enabled, URL: "https://canvas.example", APIBaseURL: "https://gateway.example"})
	return map[string]string{service.SettingKeyCanvas: string(data)}, nil
}
func TestCanvasSessionAuthorization(t *testing.T) {
	group := int64(1)
	expired := time.Now().Add(-time.Hour)
	for _, tc := range []struct {
		name                   string
		authenticated, enabled bool
		owner                  int64
		group                  *int64
		status                 string
		expires                *time.Time
		quota, used            float64
		want                   int
	}{
		{name: "unauthenticated", want: 401},
		{name: "disabled", authenticated: true, want: 403},
		{name: "other user", authenticated: true, enabled: true, owner: 8, want: 404},
		{name: "ungrouped", authenticated: true, enabled: true, owner: 7, status: "active", want: 403},
		{name: "inactive", authenticated: true, enabled: true, owner: 7, group: &group, status: "disabled", want: 403},
		{name: "expired", authenticated: true, enabled: true, owner: 7, group: &group, status: "active", expires: &expired, want: 403},
		{name: "quota exhausted", authenticated: true, enabled: true, owner: 7, group: &group, status: "active", quota: 1, used: 1, want: 403},
		{name: "own active key", authenticated: true, enabled: true, owner: 7, group: &group, status: "active", want: 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			key := &service.APIKey{ID: 12, UserID: tc.owner, Key: "gateway-only", GroupID: tc.group, Status: tc.status, ExpiresAt: tc.expires, Quota: tc.quota, QuotaUsed: tc.used}
			handler := NewAPIKeyHandler(service.NewAPIKeyService(canvasKeyRepo{key: key}, nil, nil, nil, nil, nil, nil))
			settings := service.NewSettingService(canvasSettingRepo{enabled: tc.enabled}, nil)
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest("GET", "/canvas/session/12?baseUrl=https://forged.example", nil)
			c.Params = gin.Params{{Key: "id", Value: "12"}}
			if tc.authenticated {
				c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 7})
				c.Set(string(middleware.ContextKeyUserRole), "user")
			}
			handler.CanvasSession(settings)(c)
			require.Equal(t, tc.want, recorder.Code, recorder.Body.String())
			if tc.want == 200 {
				require.Contains(t, recorder.Body.String(), "gateway-only")
				require.Contains(t, recorder.Body.String(), "https://gateway.example")
				require.NotContains(t, recorder.Body.String(), "forged.example")
				require.Contains(t, recorder.Body.String(), "\"role\":\"user\"")
				require.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
			} else {
				require.NotContains(t, recorder.Body.String(), "gateway-only")
			}
		})
	}
}
