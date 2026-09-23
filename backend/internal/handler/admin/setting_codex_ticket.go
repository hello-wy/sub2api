package admin

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *SettingHandler) GetCodexTicketSettings(c *gin.Context) {
	settings, err := h.settingService.GetCodexTicketSettings(c.Request.Context())
	if !codexTicketControlError(c, err) {
		response.Success(c, settings)
	}
}

func (h *SettingHandler) UpdateCodexTicketSettings(c *gin.Context) {
	var req struct {
		Enabled         *bool  `json:"enabled"`
		HarvestProxyURL string `json:"harvest_proxy_url"`
		ClearProxy      bool   `json:"clear_proxy"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16*1024)
	if err := c.ShouldBindJSON(&req); err != nil || req.Enabled == nil {
		response.BadRequest(c, "Invalid STATE settings; enabled is required")
		return
	}
	settings, err := h.settingService.UpdateCodexTicketSettings(c.Request.Context(), service.CodexTicketSettingsUpdate{Enabled: *req.Enabled, HarvestProxyURL: req.HarvestProxyURL, ClearProxy: req.ClearProxy})
	if !codexTicketControlError(c, err) {
		response.Success(c, settings)
	}
}
