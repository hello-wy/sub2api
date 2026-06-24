package admin

import (
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type WelfareHandler struct {
	welfareSvc *service.WelfareService
}

func NewWelfareHandler(welfareSvc *service.WelfareService) *WelfareHandler {
	return &WelfareHandler{welfareSvc: welfareSvc}
}

// ListWelfareRecords handles listing welfare records with pagination and email filter
// GET /api/v1/admin/welfare-records
func (h *WelfareHandler) ListWelfareRecords(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	searchEmail := strings.TrimSpace(c.Query("email"))

	records, pagResult, err := h.welfareSvc.ListWelfareRecords(c.Request.Context(), pagination.PaginationParams{
		Page:     page,
		PageSize: pageSize,
	}, searchEmail)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Paginated(c, records, int64(pagResult.Total), page, pageSize)
}

// RevokeWelfareRecord handles revoking a welfare record
// POST /api/v1/admin/welfare-records/:id/revoke
func (h *WelfareHandler) RevokeWelfareRecord(c *gin.Context) {
	recordID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid record ID")
		return
	}

	err = h.welfareSvc.RevokeWelfareRecord(c.Request.Context(), recordID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{
		"message": "Welfare record revoked successfully",
	})
}
