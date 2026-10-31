//go:build unit

package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type canvasAdminSettingsRepo struct {
	service.SettingRepository
	values map[string]string
}

func (r *canvasAdminSettingsRepo) GetMultiple(context.Context, []string) (map[string]string, error) {
	return r.values, nil
}
func (r *canvasAdminSettingsRepo) Set(_ context.Context, key, value string) error {
	r.values[key] = value
	return nil
}

type canvasAdminUserRepo struct {
	service.UserRepository
	user *service.User
}

func (r canvasAdminUserRepo) GetByID(context.Context, int64) (*service.User, error) {
	return r.user, nil
}
func (r canvasAdminUserRepo) GetUserAvatar(context.Context, int64) (*service.UserAvatar, error) {
	return nil, nil
}

func TestCanvasSettingsAdminAuthorization(t *testing.T) {
	for _, role := range []string{service.RoleUser, service.RoleAdmin} {
		t.Run(role, func(t *testing.T) {
			cfg := &config.Config{JWT: config.JWTConfig{Secret: "canvas-test-only", ExpireHour: 1}}
			auth := service.NewAuthService(nil, nil, nil, nil, cfg, nil, nil, nil, nil, nil, nil, nil, nil)
			user := &service.User{ID: 7, Role: role, Status: service.StatusActive}
			users := service.NewUserService(canvasAdminUserRepo{user: user}, nil, nil, nil)
			repo := &canvasAdminSettingsRepo{values: map[string]string{}}
			settings := service.NewSettingService(repo, cfg)
			handler := &SettingHandler{settingService: settings}
			router := gin.New()
			router.PUT("/api/v1/admin/settings/canvas", gin.HandlerFunc(middleware.NewAdminAuthMiddleware(auth, users, nil, nil)), handler.UpdateCanvasSettings)
			token, err := auth.GenerateToken(context.Background(), user)
			require.NoError(t, err)
			request := httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings/canvas", strings.NewReader(`{"upstream_retry_count":3,"enabled":true,"url":"https://canvas.example","api_base_url":"https://gateway.example","effective_api_base_url":"https://forged.example"}`))
			request.Header.Set("Authorization", "Bearer "+token)
			request.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)
			if role == service.RoleUser {
				require.Equal(t, http.StatusForbidden, recorder.Code)
				require.Empty(t, repo.values)
			} else {
				require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
				require.NotContains(t, recorder.Body.String(), "api_base_url")
				require.Contains(t, recorder.Body.String(), `"upstream_retry_count":3`)
				require.NotContains(t, repo.values[service.SettingKeyCanvas], "forged.example")
			}
		})
	}
}

func TestCanvasRetryCountValidation(t *testing.T) {
	for _, tc := range []struct {
		value  string
		status int
	}{
		{`0`, 200}, {`5`, 200}, {`-1`, 400}, {`6`, 400}, {`1.5`, 400}, {`"3"`, 400},
	} {
		t.Run(tc.value, func(t *testing.T) {
			repo := &canvasAdminSettingsRepo{values: map[string]string{}}
			handler := &SettingHandler{settingService: service.NewSettingService(repo, &config.Config{})}
			router := gin.New()
			router.PUT("/canvas", handler.UpdateCanvasSettings)
			request := httptest.NewRequest(http.MethodPut, "/canvas", strings.NewReader(`{"upstream_retry_count":`+tc.value+`}`))
			request.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)
			require.Equal(t, tc.status, recorder.Code, recorder.Body.String())
			if tc.status != 200 {
				require.Empty(t, repo.values)
			}
		})
	}
}
