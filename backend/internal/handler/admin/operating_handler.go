package admin

import (
	"database/sql"
	"errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"log/slog"
	"strconv"
)

func (h *DashboardHandler) operating(c *gin.Context) *service.BusinessAnalyticsService {
	if h.businessService == nil {
		response.Error(c, 503, "经营统计暂不可用")
		return nil
	}
	return h.businessService
}
func operatingActor(c *gin.Context) (int64, bool) {
	s, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Error(c, 401, "需要管理员身份")
		return 0, false
	}
	return s.UserID, true
}
func operatingID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		response.BadRequest(c, "记录编号无效")
		return 0, false
	}
	return id, true
}
func operatingError(c *gin.Context, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		response.NotFound(c, "记录不存在")
	} else {
		var invalid *service.OperatingValidationError
		if errors.As(err, &invalid) {
			response.BadRequest(c, invalid.Error())
			return
		}
		slog.ErrorContext(c.Request.Context(), "operating statistics failed", "error", err)
		response.Error(c, 500, "经营统计处理失败，请稍后重试")
	}
}
func (h *DashboardHandler) GetOperatingOverview(c *gin.Context) {
	s := h.operating(c)
	if s == nil {
		return
	}
	v, err := s.Overview(c.Request.Context(), c.Query("start_date"), c.Query("end_date"))
	if err != nil {
		operatingError(c, err)
		return
	}
	response.Success(c, v)
}
func (h *DashboardHandler) GetOperatingSettings(c *gin.Context) {
	s := h.operating(c)
	if s == nil {
		return
	}
	v, err := s.Settings(c.Request.Context())
	if err != nil {
		operatingError(c, err)
		return
	}
	response.Success(c, v)
}
func (h *DashboardHandler) InitializeOperating(c *gin.Context) {
	s := h.operating(c)
	if s == nil {
		return
	}
	actor, ok := operatingActor(c)
	if !ok {
		return
	}
	var in service.BusinessSettings
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, "期初参数无效")
		return
	}
	v, err := s.Initialize(c.Request.Context(), in, actor)
	if err != nil {
		operatingError(c, err)
		return
	}
	response.Success(c, v)
}
func (h *DashboardHandler) GetOperatingRecords(c *gin.Context) {
	s := h.operating(c)
	if s == nil {
		return
	}
	before, err := strconv.ParseInt(c.DefaultQuery("before", "0"), 10, 64)
	if err != nil || before < 0 {
		response.BadRequest(c, "分页参数无效")
		return
	}
	v, err := s.Records(c.Request.Context(), before)
	if err != nil {
		operatingError(c, err)
		return
	}
	response.Success(c, v)
}
func (h *DashboardHandler) CreateOperatingRecord(c *gin.Context) {
	s := h.operating(c)
	if s == nil {
		return
	}
	actor, ok := operatingActor(c)
	if !ok {
		return
	}
	var in service.BusinessRecordInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, "记账参数无效")
		return
	}
	v, err := s.Record(c.Request.Context(), in, actor)
	if err != nil {
		operatingError(c, err)
		return
	}
	response.Created(c, v)
}
func (h *DashboardHandler) ReverseOperatingRecord(c *gin.Context) {
	s := h.operating(c)
	if s == nil {
		return
	}
	actor, ok := operatingActor(c)
	if !ok {
		return
	}
	id, ok := operatingID(c)
	if !ok {
		return
	}
	var in struct{ Reason string }
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, "请填写冲销原因")
		return
	}
	if err := s.ReverseRecord(c.Request.Context(), id, actor, in.Reason); err != nil {
		operatingError(c, err)
		return
	}
	response.Success(c, gin.H{"reversed": true})
}
func (h *DashboardHandler) GetOperatingTerms(c *gin.Context) {
	s := h.operating(c)
	if s == nil {
		return
	}
	before, err := strconv.ParseInt(c.DefaultQuery("before", "0"), 10, 64)
	if err != nil || before < 0 {
		response.BadRequest(c, "分页参数无效")
		return
	}
	v, err := s.Terms(c.Request.Context(), before)
	if err != nil {
		operatingError(c, err)
		return
	}
	response.Success(c, v)
}
func (h *DashboardHandler) ConfigureOperatingTerm(c *gin.Context) {
	s := h.operating(c)
	if s == nil {
		return
	}
	actor, ok := operatingActor(c)
	if !ok {
		return
	}
	id, ok := operatingID(c)
	if !ok {
		return
	}
	var in service.BusinessTermInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, "订阅基准无效")
		return
	}
	if err := s.ConfigureTerm(c.Request.Context(), id, actor, in); err != nil {
		operatingError(c, err)
		return
	}
	response.Success(c, gin.H{"configured": true})
}
