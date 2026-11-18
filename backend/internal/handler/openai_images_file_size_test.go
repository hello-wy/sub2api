//go:build unit

package handler

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type imagesFileSizeUpstream struct {
	service.HTTPUpstream
	accounts []int64
}

func (u *imagesFileSizeUpstream) Do(_ *http.Request, _ string, accountID int64, _ int) (*http.Response, error) {
	u.accounts = append(u.accounts, accountID)
	return &http.Response{StatusCode: 500, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"error":{"message":"file \"request_file:image[]\" exceeds the 20971520 byte limit"}}`))}, nil
}

func TestOpenAIImagesFileSize_NoRetryFailoverBillingOrCooldown(t *testing.T) {
	gin.SetMode(gin.TestMode)
	accounts := []service.Account{}
	for _, id := range []int64{1, 2} {
		accounts = append(accounts, service.Account{ID: id, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true, Priority: int(id), Credentials: map[string]any{"api_key": "test-only"}})
	}
	cfg := &config.Config{RunMode: config.RunModeSimple}
	upstream := &imagesFileSizeUpstream{}
	gateway := service.NewOpenAIGatewayService(openAIImagesFailoverAccountRepo{accounts: accounts}, nil, nil, nil, nil, nil, nil, cfg, nil, nil, nil, nil, nil, upstream, nil, nil, nil, nil, nil, nil, nil, nil)
	billing := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(billing.Stop)
	h := NewOpenAIGatewayHandler(gateway, service.NewConcurrencyService(nil), billing, service.NewAPIKeyService(nil, nil, nil, nil, nil, nil, cfg), nil, nil, nil, nil, cfg)
	h.maxAccountSwitches = 10
	// With no usage repository configured, reaching the billing path would fail.
	// Repeated requests also reveal any model cooldown accidentally accumulated.
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "gemini-3-pro-image"))
	require.NoError(t, writer.WriteField("prompt", "test"))
	part, err := writer.CreateFormFile("image[]", "reference.png")
	require.NoError(t, err)
	_, err = part.Write([]byte("test"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	for i := 0; i < 4; i++ {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest("POST", "/v1/images/edits", bytes.NewReader(body.Bytes())).WithContext(context.Background())
		c.Request.Header.Set("Content-Type", writer.FormDataContentType())
		groupID := int64(11)
		c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{ID: 1037, GroupID: &groupID, Group: &service.Group{ID: groupID, AllowImageGeneration: true}, User: &service.User{ID: 698}})
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 698})
		h.Images(c)
		require.Equal(t, 413, rec.Code, rec.Body.String())
		require.Equal(t, service.OpenAIImagesFileTooLargeCode, gjson.GetBytes(rec.Body.Bytes(), "error.code").String())
		require.Len(t, upstream.accounts, i+1)
	}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest("POST", "/v1/images/edits", bytes.NewReader(body.Bytes()))
	c.Request.Body = http.MaxBytesReader(rec, c.Request.Body, 10)
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{ID: 1037, User: &service.User{ID: 698}})
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 698})
	h.Images(c)
	require.Equal(t, 413, rec.Code)
	require.Contains(t, rec.Body.String(), "invalid_request_error")
	require.Equal(t, []int64{1, 1, 1, 1}, upstream.accounts)
}

func TestOpenAIImagesFileSize_OpsClassifiesInputAndRetainsUpstream(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	service.SetOpsUpstreamError(c, 500, "file exceeds upstream limit", "")
	phase, limited, owner, _ := classifyOpsErrorLog(c, "invalid_request_error", "参考图片超过当前上游限制", service.OpenAIImagesFileTooLargeCode, 413)
	require.Equal(t, "request", phase)
	require.True(t, limited, "input limits must not be counted as upstream availability failures")
	require.Equal(t, "client", owner)
	require.Equal(t, 500, c.GetInt(service.OpsUpstreamStatusCodeKey))
}
