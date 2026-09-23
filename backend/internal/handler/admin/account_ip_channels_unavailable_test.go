package admin

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type unavailableIPChannelAdmin struct {
	*stubAdminService
	service.AccountIPChannelAdmin
}

func (s *unavailableIPChannelAdmin) GetAccountIPChannels(context.Context, []int64) (map[int64][]service.AccountIPChannel, error) {
	return nil, errors.New("channel metadata database temporarily unavailable")
}

func TestAccountIPChannelsUnavailableReadFailsClosedAndWriteRemainsCommitted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &unavailableIPChannelAdmin{stubAdminService: newStubAdminService()}
	svc.getAccountResult = &service.Account{ID: 51, Name: "logical-account", Platform: service.PlatformOpenAI, Type: service.AccountTypeOAuth, Status: service.StatusActive, Schedulable: true}
	h := NewAccountHandler(svc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	router := gin.New()
	router.GET("/accounts/:id", h.GetByID)
	router.PUT("/accounts/:id", h.Update)

	// Metadata errors cannot turn a logical-account detail response into an
	// apparently ordinary account whose IP/concurrency controls can be edited.
	read := httptest.NewRecorder()
	router.ServeHTTP(read, httptest.NewRequest(http.MethodGet, "/accounts/51", nil))
	require.Equal(t, http.StatusServiceUnavailable, read.Code)
	require.Contains(t, read.Body.String(), "IP_CHANNEL_METADATA_UNAVAILABLE")
	require.NotContains(t, read.Body.String(), "logical-account")
	require.Zero(t, svc.updateAccountCalls)

	// After a successful mutation, failed response enrichment is distinct from
	// a failed write. Clients must refresh metadata, not repeat the mutation.
	write := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/accounts/51", strings.NewReader(`{"name":"saved-name"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(write, req)
	require.Equal(t, http.StatusOK, write.Code)
	require.Equal(t, 1, svc.updateAccountCalls)
	require.Equal(t, "saved-name", svc.lastUpdateAccountInput.Name)
	var payload struct {
		Data struct {
			ID          int64  `json:"id"`
			Name        string `json:"name"`
			Unavailable bool   `json:"ip_channels_unavailable"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(write.Body.Bytes(), &payload))
	require.EqualValues(t, 51, payload.Data.ID)
	require.Equal(t, "saved-name", payload.Data.Name)
	require.True(t, payload.Data.Unavailable)
	require.True(t, h.buildAccountResponseWithRuntime(context.Background(), svc.getAccountResult).IPChannelsUnavailable)
}
