package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"strconv"
)

// GetCanvasSettings is registered only on authenticated routes.
func (h *SettingHandler) GetCanvasSettings(c *gin.Context) {
	settings, err := h.settingService.GetCanvasSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, settings)
}
func (h *APIKeyHandler) CanvasSession(settings *service.SettingService) gin.HandlerFunc {
	return func(c *gin.Context) {
		subject, ok := middleware.GetAuthSubjectFromContext(c)
		if !ok {
			response.Unauthorized(c, "User not authenticated")
			return
		}
		config, err := settings.GetCanvasSettings(c.Request.Context())
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		if !config.Enabled {
			response.Forbidden(c, "无限画布已关闭")
			return
		}
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			response.BadRequest(c, "Invalid key ID")
			return
		}
		key, err := h.apiKeyService.GetByID(c.Request.Context(), id)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		if key.UserID != subject.UserID {
			response.NotFound(c, "API key not found")
			return
		}
		if !key.IsActive() || key.IsExpired() || key.GroupID == nil || (key.Quota > 0 && key.QuotaUsed >= key.Quota) {
			response.Forbidden(c, "密钥不可用，请检查分组、状态、有效期和配额")
			return
		}
		c.Header("Cache-Control", "no-store")
		response.Success(c, gin.H{
			"user":   gin.H{"id": subject.UserID, "role": c.GetString(string(middleware.ContextKeyUserRole))},
			"apiKey": key.Key, "keyId": key.ID, "keyName": key.Name, "canvasUrl": config.URL,
			"baseUrl":            config.EffectiveAPIBaseURL,
			"upstreamRetryCount": config.UpstreamRetryCount,
		})
	}
}
