package admin

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// GetLotteryPrizePool GET /api/v1/admin/settings/lottery
func (h *SettingHandler) GetLotteryPrizePool(c *gin.Context) {
	if h.lotteryService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Lottery service is unavailable")
		return
	}
	config, err := h.lotteryService.GetPrizePoolConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, config)
}

// UpdateLotteryPrizePool PUT /api/v1/admin/settings/lottery
func (h *SettingHandler) UpdateLotteryPrizePool(c *gin.Context) {
	if h.lotteryService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Lottery service is unavailable")
		return
	}
	var config service.LotteryPrizePoolConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		response.BadRequest(c, "Invalid lottery prize pool: "+err.Error())
		return
	}
	updated, err := h.lotteryService.UpdatePrizePoolConfig(c.Request.Context(), config)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, updated)
}
