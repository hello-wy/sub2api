package admin

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *SettingHandler) GetNewAccountCodexTicketDefaults(c *gin.Context) {
	value, err := h.settingService.GetNewAccountCodexTicketDefaults(c.Request.Context())
	if !codexTicketControlError(c, err) {
		response.Success(c, value)
	}
}

func (h *SettingHandler) UpdateNewAccountCodexTicketDefaults(c *gin.Context) {
	var req struct {
		Enabled    *bool  `json:"enabled"`
		TicketPlan string `json:"ticket_plan"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4096)
	if err := c.ShouldBindJSON(&req); err != nil || req.Enabled == nil {
		response.BadRequest(c, "Invalid new-account STATE defaults; enabled and ticket_plan are required")
		return
	}
	value, err := h.settingService.UpdateNewAccountCodexTicketDefaults(c.Request.Context(), service.NewAccountCodexTicketDefaults{Enabled: *req.Enabled, TicketPlan: req.TicketPlan})
	if !codexTicketControlError(c, err) {
		response.Success(c, value)
	}
}
