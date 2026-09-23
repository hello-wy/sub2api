package admin

import (
	"context"
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type codexAccountTicketBatchManager interface {
	BatchCodexAccountTickets(context.Context, service.CodexAccountTicketBatchUpdate) (*service.CodexAccountTicketBatchResult, error)
}

func (h *AccountHandler) BatchCodexAccountTickets(c *gin.Context) {
	manager, ok := h.codexAccountTickets.(codexAccountTicketBatchManager)
	if !ok {
		response.Error(c, http.StatusServiceUnavailable, "Account STATE batch service unavailable")
		return
	}
	var req struct {
		AccountIDs []int64 `json:"account_ids"`
		Enabled    *bool   `json:"enabled"`
		TicketPlan string  `json:"ticket_plan"`
		Model      string  `json:"model"`
		Harvest    bool    `json:"harvest"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16*1024)
	if err := c.ShouldBindJSON(&req); err != nil || req.Enabled == nil {
		response.BadRequest(c, "Invalid STATE batch settings; enabled is required")
		return
	}
	result, err := manager.BatchCodexAccountTickets(c.Request.Context(), service.CodexAccountTicketBatchUpdate{
		AccountIDs: req.AccountIDs, Enabled: *req.Enabled, TicketPlan: req.TicketPlan, Model: req.Model, Harvest: req.Harvest,
	})
	if !codexTicketControlError(c, err) {
		response.Success(c, result)
	}
}
