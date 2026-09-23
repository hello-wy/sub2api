package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"net/http"
)

func (h *IntelligentTestHandler) Delete(c *gin.Context) {
	actor, ok := intelligentAdminActor(c)
	if !ok {
		return
	}
	id, ok := IntelligentTestParamID(c, "id")
	if !ok {
		return
	}
	count, err := h.svc.DeleteRecords(c.Request.Context(), actor, []int64{id})
	if !response.ErrorFrom(c, err) {
		response.Success(c, gin.H{"deleted": count})
	}
}

func (h *IntelligentTestHandler) DeleteBatch(c *gin.Context) {
	actor, ok := intelligentAdminActor(c)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
	var req struct {
		IDs []int64 `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "无效的删除请求")
		return
	}
	count, err := h.svc.DeleteRecords(c.Request.Context(), actor, req.IDs)
	if !response.ErrorFrom(c, err) {
		response.Success(c, gin.H{"deleted": count})
	}
}
