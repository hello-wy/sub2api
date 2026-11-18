package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

const oversizedImageError = `{"error":{"message":"file \"request_file:image[]\" exceeds the 20971520 byte limit (request id: provider-test)","type":"server_error"}}`

func TestOpenAIImagesFileSizeClassification(t *testing.T) {
	for _, tt := range []struct {
		name, body, param, limit string
		match                    bool
	}{
		{"production", oversizedImageError, "image[]", "20.00 MiB", true},
		{"different limit", `{"error":{"message":"file \"image[0]\" exceeds the 10485760 byte limit"}}`, "image[0]", "10.00 MiB", true},
		{"mask", `{"error":{"message":"file \"request_file:mask\" exceeds the 1048576 byte limit"}}`, "mask", "1.00 MiB", true},
		{"code", `{"error":{"code":"image_file_too_large","param":"image"}}`, "image", "", true},
		{"image code", `{"error":{"code":"image_too_large"}}`, "", "", true},
		{"file code with image", `{"error":{"code":"file_too_large","param":"image[]"}}`, "image[]", "", true},
		{"generic file", `{"error":{"code":"file_too_large","param":"document"}}`, "", "", false},
		{"capacity", `{"error":{"message":"request limit exceeded"}}`, "", "", false},
		{"rate", `{"error":{"code":"rate_limit_exceeded","message":"Too many pending requests"}}`, "", "", false},
		{"server", `{"error":{"message":"server unavailable"}}`, "", "", false},
		{"other file", `{"error":{"message":"file \"document\" exceeds the 20971520 byte limit"}}`, "", "", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := openAIImagesFileSizeError([]byte(tt.body))
			if !tt.match {
				require.Nil(t, err)
				return
			}
			require.NotNil(t, err)
			require.Equal(t, 413, err.StatusCode)
			require.Equal(t, "invalid_request_error", err.ErrorType)
			require.Equal(t, OpenAIImagesFileTooLargeCode, err.Code)
			require.Equal(t, tt.param, err.Param)
			require.Contains(t, err.Message, tt.limit)
			require.False(t, IsOpenAIImagesTransientUpstreamFailure(err))
			require.False(t, IsOpenAIImagesRetryableUpstreamError(err))
		})
	}
}

func TestOpenAIImagesMultipartPreservesLargeFiles(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "gemini-3-pro-image"))
	payload := bytes.Repeat([]byte("a"), (20<<20)+7)
	copy(payload[len(payload)-7:], "the-end")
	for _, field := range []string{"image", "image[]", "mask"} {
		part, err := writer.CreateFormFile(field, "reference.png")
		require.NoError(t, err)
		_, err = part.Write(payload)
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	parsed := &OpenAIImagesRequest{}
	require.NoError(t, parseOpenAIImagesMultipartRequest(body.Bytes(), writer.FormDataContentType(), parsed))
	require.Len(t, parsed.Uploads, 2)
	for _, upload := range parsed.Uploads {
		require.Equal(t, payload, upload.Data)
	}
	require.Equal(t, payload, parsed.MaskUpload.Data)
}

func TestOpenAIImagesFileSizeForwardPreservesHealthAndDiagnostics(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, mode := range []string{"json", "json_keepalive", "sse", "oauth"} {
		t.Run(mode, func(t *testing.T) {
			body := []byte(`{"model":"gemini-3-pro-image","prompt":"test"}`)
			if mode == "oauth" {
				body = []byte(`{"model":"gpt-image-1","prompt":"test"}`)
			}
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest("POST", "/v1/images/generations", bytes.NewReader(body))
			c.Request.Header.Set("Content-Type", "application/json")
			if mode == "json_keepalive" {
				stop := StartOpenAIImagesJSONKeepalive(c, time.Hour)
				defer stop()
				require.True(t, openAIImagesJSONKeepaliveFromContext(c).beat())
			} else if mode == "sse" {
				c.Header("Content-Type", "text/event-stream")
				_, _ = c.Writer.WriteString(": ping\n\n")
				c.Writer.Flush()
			}
			upstream := &httpUpstreamRecorder{resp: &http.Response{
				StatusCode: 500, Header: http.Header{"X-Request-Id": []string{"provider-test"}},
				Body: io.NopCloser(strings.NewReader(oversizedImageError)),
			}}
			svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
			account := &Account{ID: 918, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test-only"}}
			if mode == "oauth" {
				account.Type = AccountTypeOAuth
				account.Credentials["access_token"] = "test-only"
			}
			before := svc.recordOpenAIAccountModelTransientFailure(account, "gemini-3-pro-image", time.Now())
			before = svc.recordOpenAIAccountModelTransientFailure(account, "gemini-3-pro-image", time.Now())
			parsed, err := svc.ParseOpenAIImagesRequest(c, body)
			require.NoError(t, err)
			result, err := svc.ForwardImages(context.Background(), c, account, body, parsed, "")
			require.Nil(t, result)
			var inputErr *OpenAIImagesUpstreamError
			require.ErrorAs(t, err, &inputErr)
			require.Equal(t, OpenAIImagesFileTooLargeCode, inputErr.Code)
			require.Equal(t, "provider-test", inputErr.UpstreamRequestID)
			require.False(t, IsOpenAIImagesTransientUpstreamFailure(err))
			after := svc.recordOpenAIAccountModelTransientFailure(account, "gemini-3-pro-image", time.Now())
			require.Equal(t, before.FailureStreak+1, after.FailureStreak, "input rejection must neither increment nor clear health failures")
			if mode == "json" || mode == "oauth" {
				require.Equal(t, 413, rec.Code)
			} else {
				require.Equal(t, 200, rec.Code)
			}
			if mode == "sse" {
				require.Contains(t, rec.Body.String(), "event: error\n")
				require.Contains(t, rec.Body.String(), OpenAIImagesFileTooLargeCode)
			} else {
				require.Equal(t, OpenAIImagesFileTooLargeCode, gjson.GetBytes(rec.Body.Bytes(), "error.code").String())
			}
			require.Equal(t, 500, c.GetInt(OpsUpstreamStatusCodeKey))
			require.Contains(t, c.GetString(OpsUpstreamErrorMessageKey), "20971520")
			if mode == "json_keepalive" || mode == "sse" {
				require.Equal(t, OpenAIImagesFileTooLargeCode, GetOpsStreamErrors(c)[0].Code)
			}
		})
	}
}

func TestImageTaskFileSizeFailureRetainsCode(t *testing.T) {
	svc := NewImageTaskServiceWithOptions(&imageTaskMemoryStore{}, time.Hour, time.Minute)
	owner := ImageTaskOwner{UserID: 1, APIKeyID: 2}
	task, err := svc.Create(context.Background(), owner)
	require.NoError(t, err)
	inputErr := openAIImagesFileSizeError([]byte(oversizedImageError))
	require.NoError(t, svc.Fail(context.Background(), task.ID, 413, []byte(fmt.Sprintf(`{"type":%q,"code":%q,"message":%q}`, inputErr.ErrorType, inputErr.Code, inputErr.Message))))
	got, err := svc.Get(context.Background(), owner, task.ID)
	require.NoError(t, err)
	require.Equal(t, ImageTaskStatusFailed, got.Status)
	require.Equal(t, 413, got.HTTPStatus)
	require.Equal(t, OpenAIImagesFileTooLargeCode, gjson.GetBytes(got.Error, "code").String())
}
