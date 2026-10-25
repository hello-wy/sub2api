//go:build unit

package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type channelMonitorOrderRepoStub struct {
	service.ChannelMonitorRepository
	ids []int64
	err error
}

func (r *channelMonitorOrderRepoStub) UpdateSortOrder(_ context.Context, ids []int64) error {
	r.ids = append([]int64(nil), ids...)
	return r.err
}

func TestChannelMonitorSortOrderHandler(t *testing.T) {
	for _, tt := range []struct {
		name, body string
		repoErr    error
		status     int
	}{
		{name: "valid", body: `{"monitor_ids":[42,7,8]}`, status: http.StatusOK},
		{name: "duplicate", body: `{"monitor_ids":[42,42]}`, status: http.StatusBadRequest},
		{name: "empty", body: `{"monitor_ids":[]}`, status: http.StatusBadRequest},
		{name: "invalid ID", body: `{"monitor_ids":[0]}`, status: http.StatusBadRequest},
		{name: "fractional ID", body: `{"monitor_ids":[1.5]}`, status: http.StatusBadRequest},
		{name: "deleted monitor", body: `{"monitor_ids":[42,7,8]}`, repoErr: service.ErrChannelMonitorNotFound, status: http.StatusNotFound},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo := &channelMonitorOrderRepoStub{err: tt.repoErr}
			h := NewChannelMonitorHandler(service.NewChannelMonitorService(repo, nil))
			router := gin.New()
			router.PUT("/channel-monitors/sort-order", h.UpdateSortOrder)
			req := httptest.NewRequest(http.MethodPut, "/channel-monitors/sort-order", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)
			require.Equal(t, tt.status, recorder.Code, recorder.Body.String())
			if tt.status == http.StatusBadRequest {
				require.Nil(t, repo.ids)
			} else {
				require.Equal(t, []int64{42, 7, 8}, repo.ids)
			}
		})
	}
}
