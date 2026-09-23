package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func ProvideChannelMonitorV2Handler(svc *service.ChannelMonitorV2Service, keys *service.APIKeyService, groups *service.GroupStatusService) *ChannelMonitorV2Handler {
	h := NewChannelMonitorV2Handler(svc, keys)
	h.groupStatus = groups
	return h
}
func (h *ChannelMonitorV2Handler) GroupStatusService() *service.GroupStatusService {
	if h == nil {
		return nil
	}
	return h.groupStatus
}
func (h *ChannelMonitorV2Handler) GroupStatus(c *gin.Context) {
	if h.groupStatus == nil {
		response.Error(c, 503, "group status unavailable")
		return
	}
	if c.Query("range") == "" {
		q := c.Request.URL.Query()
		q.Set("range", "24h")
		c.Request.URL.RawQuery = q.Encode()
	}
	filter, ok := h.parseFilter(c)
	if !ok {
		return
	}
	if !h.scopeFilter(c, &filter, channelMonitorV2IsAdmin(c)) {
		return
	}
	if filter.Range == "90m" {
		response.BadRequest(c, "supported ranges: 24h, 7d, 15d, 30d")
		return
	}
	result, err := h.groupStatus.Status(c.Request.Context(), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}
func (h *ChannelMonitorV2Handler) ProbeConfigs(c *gin.Context) {
	if !h.requireProbeAdmin(c) {
		return
	}
	items, err := h.groupStatus.ProbeConfigs(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}
func (h *ChannelMonitorV2Handler) SaveProbeConfig(c *gin.Context) {
	if !h.requireProbeAdmin(c) {
		return
	}
	id, ok := probeGroupID(c)
	if !ok {
		return
	}
	var input service.GroupProbeConfig
	if c.ShouldBindJSON(&input) != nil {
		response.BadRequest(c, "invalid probe configuration")
		return
	}
	input.GroupID = id
	if err := service.ValidateGroupProbeConfig(&input); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	subject, _ := middleware.GetAuthSubjectFromContext(c)
	out, err := h.groupStatus.SaveProbeConfig(c.Request.Context(), input, subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, out)
}
func (h *ChannelMonitorV2Handler) RunGroupProbe(c *gin.Context) {
	if !h.requireProbeAdmin(c) {
		return
	}
	id, ok := probeGroupID(c)
	if !ok {
		return
	}
	run, err := h.groupStatus.StartProbe(c.Request.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrGroupProbeBusy):
			response.Error(c, http.StatusConflict, "group probe already running")
		case errors.Is(err, service.ErrGroupProbeBudget):
			response.Error(c, http.StatusTooManyRequests, "daily probe token budget exhausted")
		case errors.Is(err, service.ErrGroupProbeNotConfigured):
			response.BadRequest(c, "save probe configuration first")
		case errors.Is(err, service.ErrChannelMonitorDisabled):
			response.ErrorFrom(c, err)
		default:
			response.ErrorFrom(c, err)
		}
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"code": 0, "message": "success", "data": run})
}
func (h *ChannelMonitorV2Handler) GroupProbeHistory(c *gin.Context) {
	if !h.requireProbeAdmin(c) {
		return
	}
	id, ok := probeGroupID(c)
	if !ok {
		return
	}
	items, err := h.groupStatus.ProbeHistory(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}
func (h *ChannelMonitorV2Handler) requireProbeAdmin(c *gin.Context) bool {
	if !channelMonitorV2IsAdmin(c) {
		response.Error(c, http.StatusForbidden, "administrator required")
		return false
	}
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "administrator required")
		return false
	}
	if h.groupStatus == nil {
		response.Error(c, 503, "group status unavailable")
		return false
	}
	return true
}
func probeGroupID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("group_id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid group_id")
		return 0, false
	}
	return id, true
}
