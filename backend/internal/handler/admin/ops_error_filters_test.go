package admin

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type opsErrorFilterRepo struct {
	service.OpsRepository
	filter *service.OpsErrorLogFilter
}

func (r *opsErrorFilterRepo) ListErrorLogs(_ context.Context, filter *service.OpsErrorLogFilter) (*service.OpsErrorLogList, error) {
	r.filter = filter
	return &service.OpsErrorLogList{Errors: []*service.OpsErrorLog{}, Page: filter.Page, PageSize: filter.PageSize}, nil
}

func TestOpsSplitErrorFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, upstream := range []bool{false, true} {
		t.Run(map[bool]string{false: "request", true: "upstream"}[upstream], func(t *testing.T) {
			repo := &opsErrorFilterRepo{}
			handler := NewOpsHandler(service.NewOpsService(repo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil))
			router := gin.New()
			if upstream {
				router.GET("/errors", handler.ListUpstreamErrors)
			} else {
				router.GET("/errors", handler.ListRequestErrors)
			}
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest("GET", "/errors?user_id=7&account_id=839&group_id=3&api_key_id=4&phase=upstream&error_owner=provider&status_codes_other=1&q=request_timeout&model=gpt-6&view=all", nil))
			require.Equal(t, 200, w.Code, w.Body.String())
			require.NotNil(t, repo.filter)
			require.EqualValues(t, 7, *repo.filter.UserID)
			require.EqualValues(t, 839, *repo.filter.AccountID)
			require.EqualValues(t, 3, *repo.filter.GroupID)
			require.EqualValues(t, 4, *repo.filter.APIKeyID)
			require.Equal(t, "upstream", repo.filter.Phase)
			require.Equal(t, "provider", repo.filter.Owner)
			require.Equal(t, "request_timeout", repo.filter.Query)
			require.Equal(t, "gpt-6", repo.filter.Model)
			require.True(t, repo.filter.StatusCodesOther)
			require.Equal(t, upstream, repo.filter.IncludeRecoveredUpstream)
			if upstream {
				require.Equal(t, []string{"upstream", "account_auth"}, repo.filter.ErrorPhasesAny)
			}
			for _, query := range []string{"user_id=abc", "user_id=0", "user_id=-1", "api_key_id=bad", "status_codes_other=bad"} {
				repo.filter = nil
				w = httptest.NewRecorder()
				router.ServeHTTP(w, httptest.NewRequest("GET", "/errors?"+query, nil))
				require.Equal(t, 400, w.Code, query)
				require.Nil(t, repo.filter)
			}
			if upstream {
				for _, query := range []string{"phase=internal", "error_owner=client"} {
					w = httptest.NewRecorder()
					router.ServeHTTP(w, httptest.NewRequest("GET", "/errors?"+query, nil))
					require.Equal(t, 400, w.Code, query)
				}
				w = httptest.NewRecorder()
				router.ServeHTTP(w, httptest.NewRequest("GET", "/errors?phase=account_auth", nil))
				require.Equal(t, 200, w.Code)
				require.Equal(t, "account_auth", repo.filter.Phase)
			}
		})
	}
}
