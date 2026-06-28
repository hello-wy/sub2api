//go:build unit

package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRequireAdminAPIKeyAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		authMethod string
		wantStatus int
	}{
		{name: "admin api key allowed", authMethod: "admin_api_key", wantStatus: http.StatusOK},
		{name: "jwt rejected", authMethod: "jwt", wantStatus: http.StatusUnauthorized},
		{name: "missing rejected", authMethod: "", wantStatus: http.StatusUnauthorized},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				if tc.authMethod != "" {
					c.Set("auth_method", tc.authMethod)
				}
				c.Next()
			})
			router.POST("/admin/users/:id/checkin", RequireAdminAPIKeyAuth(), func(c *gin.Context) {
				c.Status(http.StatusOK)
			})

			recorder := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/admin/users/31/checkin", nil)
			router.ServeHTTP(recorder, req)

			require.Equal(t, tc.wantStatus, recorder.Code)
			if tc.wantStatus == http.StatusUnauthorized {
				require.Contains(t, recorder.Body.String(), "ADMIN_API_KEY_REQUIRED")
			}
		})
	}
}
