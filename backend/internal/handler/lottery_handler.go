package handler

import (
	"context"
	"net/http"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

func (h *UserHandler) lotterySubject(c *gin.Context) (int64, bool) {
	if h == nil || h.lotteryService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Lottery service is unavailable")
		return 0, false
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return 0, false
	}
	return subject.UserID, true
}

// GetLotteryStatus GET /api/v1/lottery/status
func (h *UserHandler) GetLotteryStatus(c *gin.Context) {
	userID, ok := h.lotterySubject(c)
	if !ok {
		return
	}
	result, err := h.lotteryService.GetStatus(c.Request.Context(), userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

// GetLotteryPrizePool GET /api/v1/lottery/prizes
func (h *UserHandler) GetLotteryPrizePool(c *gin.Context) {
	if _, ok := h.lotterySubject(c); !ok {
		return
	}
	result, err := h.lotteryService.GetPrizePoolConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

// PurchaseLotteryTicket POST /api/v1/lottery/tickets/purchase
func (h *UserHandler) PurchaseLotteryTicket(c *gin.Context) {
	userID, ok := h.lotterySubject(c)
	if !ok {
		return
	}
	executeUserIdempotentJSON(c, "lottery.ticket.purchase", struct{}{}, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		return h.lotteryService.PurchaseTicket(ctx, userID, c.GetHeader("Idempotency-Key"))
	})
}

// DrawLottery POST /api/v1/lottery/draw
func (h *UserHandler) DrawLottery(c *gin.Context) {
	userID, ok := h.lotterySubject(c)
	if !ok {
		return
	}
	executeUserIdempotentJSON(c, "lottery.draw", struct{}{}, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		return h.lotteryService.Draw(ctx, userID, c.GetHeader("Idempotency-Key"))
	})
}

// ListLotteryDraws GET /api/v1/lottery/draws
func (h *UserHandler) ListLotteryDraws(c *gin.Context) {
	userID, ok := h.lotterySubject(c)
	if !ok {
		return
	}
	limit := 20
	if raw := c.Query("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = min(parsed, 100)
		}
	}
	items, err := h.lotteryService.ListDraws(c.Request.Context(), userID, limit)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}

// ListRecentLotteryWinners GET /api/v1/lottery/recent-winners
func (h *UserHandler) ListRecentLotteryWinners(c *gin.Context) {
	if _, ok := h.lotterySubject(c); !ok {
		return
	}
	limit := 30
	if raw := c.Query("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = min(parsed, 100)
		}
	}
	items, err := h.lotteryService.ListRecentWinners(c.Request.Context(), limit)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}

// ListLotteryBalanceTransactions GET /api/v1/lottery/balance-transactions
func (h *UserHandler) ListLotteryBalanceTransactions(c *gin.Context) {
	userID, ok := h.lotterySubject(c)
	if !ok {
		return
	}
	limit := 25
	if raw := c.Query("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = min(parsed, 200)
		}
	}
	items, err := h.lotteryService.ListBalanceTransactions(c.Request.Context(), userID, limit)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}
