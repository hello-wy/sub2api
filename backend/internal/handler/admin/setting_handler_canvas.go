package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *SettingHandler) UpdateCanvasSettings(c *gin.Context) {
	settings := service.CanvasSettings{UpstreamRetryCount: 5}
	if err := c.ShouldBindJSON(&settings); err != nil {
		response.BadRequest(c, "Invalid canvas settings")
		return
	}
	if err := h.settingService.UpdateCanvasSettings(c.Request.Context(), settings); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	saved, err := h.settingService.GetCanvasSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, saved)
}
