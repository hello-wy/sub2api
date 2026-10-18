package admin

import (
	"database/sql"
	"fmt"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"testing"
)

func TestOperatingErrorClassification(t *testing.T) {
	for _, tc := range []struct {
		name    string
		err     error
		status  int
		message string
	}{
		{"validation", &service.OperatingValidationError{Message: "日期无效"}, 400, "日期无效"},
		{"missing", fmt.Errorf("wrapped: %w", sql.ErrNoRows), 404, "记录不存在"},
		{"database", fmt.Errorf("SELECT secret FROM operating_records failed"), 500, "经营统计处理失败"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("GET", "/", nil)
			operatingError(c, tc.err)
			require.Equal(t, tc.status, w.Code)
			require.Contains(t, w.Body.String(), tc.message)
			require.NotContains(t, w.Body.String(), "secret")
		})
	}
}
