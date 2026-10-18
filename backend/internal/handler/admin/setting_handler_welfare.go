package admin

import (
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// Existing IDs may belong to deleted users; validate only newly added entries.
func (h *SettingHandler) validateNewWelfareExclusions(c *gin.Context, ids, previous []int64) bool {
	existing := make(map[int64]struct{}, len(previous))
	for _, id := range previous {
		existing[id] = struct{}{}
	}
	for _, id := range ids {
		if _, ok := existing[id]; ok {
			continue
		}
		if h.userService == nil {
			response.InternalError(c, "User service is unavailable")
			return false
		}
		user, err := h.userService.GetByID(c.Request.Context(), id)
		if err != nil {
			response.ErrorFrom(c, err)
			return false
		}
		if user.Role != service.RoleUser {
			response.BadRequest(c, fmt.Sprintf("User #%d is already excluded as an administrator", id))
			return false
		}
	}
	return true
}
