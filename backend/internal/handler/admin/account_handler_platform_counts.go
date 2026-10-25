package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

// GetPlatformCounts returns provider totals independently of pagination and account filters.
// GET /api/v1/admin/accounts/platform-counts
func (h *AccountHandler) GetPlatformCounts(c *gin.Context) {
	counts, err := h.adminService.GetAccountPlatformCounts(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, counts)
}
