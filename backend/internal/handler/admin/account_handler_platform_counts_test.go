package admin

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type platformCountsAdminService struct {
	service.AdminService
	counts []service.AccountPlatformCount
	err    error
}

func (s *platformCountsAdminService) GetAccountPlatformCounts(context.Context) ([]service.AccountPlatformCount, error) {
	return s.counts, s.err
}
func TestAccountHandlerPlatformCounts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, failed := range []bool{false, true} {
		t.Run(map[bool]string{false: "global counts ignore list filters", true: "failure returns no misleading totals"}[failed], func(t *testing.T) {
			svc := &platformCountsAdminService{counts: []service.AccountPlatformCount{{Platform: "deepseek", Count: 2}, {Platform: "openai", Count: 45}}}
			if failed {
				svc.err = errors.New("database unavailable")
			}
			handler := &AccountHandler{adminService: svc}
			router := gin.New()
			router.GET("/accounts/platform-counts", handler.GetPlatformCounts)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/accounts/platform-counts?platform=openai&status=active&page=2", nil))
			if failed {
				require.Equal(t, http.StatusInternalServerError, rec.Code)
				return
			}
			require.Equal(t, http.StatusOK, rec.Code)
			var payload struct {
				Data []service.AccountPlatformCount `json:"data"`
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
			require.Equal(t, svc.counts, payload.Data)
			require.NotContains(t, rec.Body.String(), "credentials")
		})
	}
}
