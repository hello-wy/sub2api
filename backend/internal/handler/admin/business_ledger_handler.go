package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"log/slog"
	"strconv"
	"time"
)

func (h *DashboardHandler) businessLedger(c *gin.Context) *service.BusinessLedgerService {
	if h.businessService == nil {
		response.Error(c, 503, "经营账服务暂不可用")
		return nil
	}
	return h.businessService.Ledger()
}
func (h *DashboardHandler) GetBusinessLedgerOverview(c *gin.Context) {
	s := h.businessLedger(c)
	if s == nil {
		return
	}
	config, err := s.CostConfiguration(c.Request.Context())
	if err != nil {
		response.Error(c, 500, "无法读取经营账配置")
		return
	}
	zone, err := time.LoadLocation(config.Timezone)
	if err != nil {
		response.Error(c, 500, "报表时区无效")
		return
	}
	now := time.Now().In(zone)
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, zone)
	end := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, zone)
	if v := c.Query("start_date"); v != "" {
		start, err = time.ParseInLocation("2006-01-02", v, zone)
		if err != nil {
			response.BadRequest(c, "开始日期无效")
			return
		}
	}
	if v := c.Query("end_date"); v != "" {
		end, err = time.ParseInLocation("2006-01-02", v, zone)
		if err != nil {
			response.BadRequest(c, "结束日期无效")
			return
		}
		end = end.AddDate(0, 0, 1)
	}
	data, err := s.Overview(c.Request.Context(), start, end)
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "business ledger overview failed", "error", err)
		response.Error(c, 500, "经营汇总失败，台账仍可独立使用")
		return
	}
	response.Success(c, data)
}
func (h *DashboardHandler) ListBusinessLedgerRecords(c *gin.Context) {
	s := h.businessLedger(c)
	if s == nil {
		return
	}
	before, _ := strconv.ParseInt(c.Query("before"), 10, 64)
	limit, _ := strconv.Atoi(c.Query("limit"))
	data, err := s.Records(c.Request.Context(), before, limit)
	if err != nil {
		response.Error(c, 500, "台账读取失败")
		return
	}
	response.Success(c, data)
}

func (h *DashboardHandler) ListBusinessLedgerPending(c *gin.Context) {
	s := h.businessLedger(c)
	if s == nil {
		return
	}
	before, _ := strconv.ParseInt(c.Query("before"), 10, 64)
	data, err := s.Pending(c.Request.Context(), before)
	if err != nil {
		response.Error(c, 500, "待核对列表读取失败")
		return
	}
	response.Success(c, data)
}
func (h *DashboardHandler) CreateBusinessLedgerRecord(c *gin.Context) {
	s := h.businessLedger(c)
	if s == nil {
		return
	}
	var input service.BusinessRecordInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "台账参数无效")
		return
	}
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Error(c, 401, "需要管理员身份")
		return
	}
	event, err := s.Record(c.Request.Context(), input, subject.UserID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Created(c, event)
}
func (h *DashboardHandler) GetBusinessCostConfiguration(c *gin.Context) {
	s := h.businessLedger(c)
	if s == nil {
		return
	}
	data, err := s.CostConfiguration(c.Request.Context())
	if err != nil {
		response.Error(c, 500, "成本配置读取失败")
		return
	}
	response.Success(c, data)
}
func (h *DashboardHandler) CreateBusinessCostPool(c *gin.Context) {
	s := h.businessLedger(c)
	if s == nil {
		return
	}
	var input service.BusinessCostPool
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "成本池参数无效")
		return
	}
	data, err := s.CreatePool(c.Request.Context(), input)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Created(c, data)
}
func (h *DashboardHandler) CreateBusinessCostBinding(c *gin.Context) {
	s := h.businessLedger(c)
	if s == nil {
		return
	}
	var input service.BusinessCostBinding
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "绑定参数无效")
		return
	}
	data, err := s.CreateBinding(c.Request.Context(), input)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Created(c, data)
}
func (h *DashboardHandler) CreateBusinessCostRule(c *gin.Context) {
	s := h.businessLedger(c)
	if s == nil {
		return
	}
	var input service.BusinessCostRule
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "价格规则参数无效")
		return
	}
	data, err := s.CreateRule(c.Request.Context(), input)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Created(c, data)
}
func (h *DashboardHandler) SearchBusinessEntities(c *gin.Context) {
	s := h.businessLedger(c)
	if s == nil {
		return
	}
	data, err := s.Lookups(c.Request.Context(), c.Query("kind"), c.Query("q"))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, data)
}
func (h *DashboardHandler) GetBusinessEventTrace(c *gin.Context) {
	s := h.businessLedger(c)
	if s == nil {
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "事件编号无效")
		return
	}
	data, err := s.Trace(c.Request.Context(), id)
	if err != nil {
		response.Error(c, 500, "凭据读取失败")
		return
	}
	response.Success(c, data)
}

func (h *DashboardHandler) RejectLegacyBusinessWrite(c *gin.Context) {
	response.Error(c, 409, "旧经营估算已只读，请在人民币经营账中登记成本或修正凭据")
}
