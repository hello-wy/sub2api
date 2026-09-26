package middleware

import (
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"net/http"
)

// Prevent a cached or concurrently rebound key from testing a different entry group.
func ScheduledPelicanGroupTarget() gin.HandlerFunc {
	return func(c *gin.Context) {
		expected := service.ScheduledPelicanGroupID(c.Request.Context())
		if expected > 0 {
			key, ok := GetAPIKeyFromContext(c)
			if !ok || key == nil || key.GroupID == nil || *key.GroupID != expected {
				c.AbortWithStatusJSON(http.StatusConflict, gin.H{"error": gin.H{"message": "test API Key group changed; update the scheduled plan"}})
				return
			}
		}
		c.Next()
	}
}
