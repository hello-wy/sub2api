package admin

import (
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
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
	filter, ok := parseWelfareListFilter(c)
	if !ok {
		return
	}

	records, summary, pagResult, err := h.welfareSvc.ListWelfareRecords(c.Request.Context(), pagination.PaginationParams{
		Page:     page,
		PageSize: pageSize,
	}, filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, welfareListResponse{
		PaginatedData: response.PaginatedData{
			Items:    records,
			Total:    int64(pagResult.Total),
			Page:     page,
			PageSize: pageSize,
			Pages:    pagResult.Pages,
		},
		Summary: summary,
	})
}

type welfareListResponse struct {
	response.PaginatedData
	Summary *service.WelfareSummary `json:"summary"`
}

func parseWelfareListFilter(c *gin.Context) (service.WelfareListFilter, bool) {
	userTZ := c.Query("timezone")
	startTime, ok := parseWelfareDate(c, "start_date", userTZ)
	if !ok {
		return service.WelfareListFilter{}, false
	}
	endTime, ok := parseWelfareDate(c, "end_date", userTZ)
	if !ok {
		return service.WelfareListFilter{}, false
	}
	if !endTime.IsZero() {
		endTime = endTime.AddDate(0, 0, 1)
	}
	return service.WelfareListFilter{
		SearchEmail: strings.TrimSpace(c.Query("email")),
		StartTime:   startTime,
		EndTime:     endTime,
		BenefitType: strings.TrimSpace(c.Query("type")),
		Status:      strings.TrimSpace(c.Query("status")),
	}, true
}

func parseWelfareDate(c *gin.Context, key string, userTZ string) (time.Time, bool) {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return time.Time{}, true
	}
	parsed, err := timezone.ParseInUserLocation("2006-01-02", raw, userTZ)
	if err != nil {
		response.BadRequest(c, "Invalid "+key)
		return time.Time{}, false
	}
	return parsed, true
}

// RevokeWelfareRecord handles revoking a welfare record
// POST /api/v1/admin/welfare-records/:id/revoke
func (h *WelfareHandler) RevokeWelfareRecord(c *gin.Context) {
	recordID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid record ID")
		return
	}

	benefitType := strings.TrimSpace(c.Query("type"))
	err = h.welfareSvc.RevokeWelfareRecord(c.Request.Context(), recordID, benefitType)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{
		"message": "Welfare record revoked successfully",
	})
}
