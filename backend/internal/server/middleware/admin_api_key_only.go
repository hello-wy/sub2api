package middleware

import "github.com/gin-gonic/gin"

// RequireAdminAPIKeyAuth allows only requests authenticated with the admin API key.
// It must run after AdminAuthMiddleware, which sets auth_method to admin_api_key or jwt.
func RequireAdminAPIKeyAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if method, _ := c.Get("auth_method"); method == "admin_api_key" {
			c.Next()
			return
		}

		AbortWithError(c, 401, "ADMIN_API_KEY_REQUIRED", "Admin API key is required")
	}
}
