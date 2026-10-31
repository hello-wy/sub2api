package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

// GetCanvasSettings is registered only on authenticated routes.
func (h *SettingHandler) GetCanvasSettings(c *gin.Context) {
	settings, err := h.settingService.GetCanvasSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	response.Success(c, settings)
}
