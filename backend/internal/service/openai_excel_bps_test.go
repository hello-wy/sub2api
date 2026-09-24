package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func excelAccount() *Account {
	return &Account{ID: 300, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 10,
		Credentials: map[string]any{"access_token": "test-token", "chatgpt_account_id": "test-account"}, Extra: map[string]any{"openai_excel_bps": true, "openai_passthrough": true}}
}
func TestExcelBPSForwardContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprint(stream), func(t *testing.T) {
			wire := "event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_excel\",\"status\":\"completed\",\"model\":\"gpt-5.6-sol\",\"output\":[{\"type\":\"message\",\"role\":\"assistant\",\"content\":[{\"type\":\"output_text\",\"text\":\"21\"}]}],\"usage\":{\"input_tokens\":10,\"output_tokens\":2}}}\n\n"
			upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(wire))}}
			svc := openAIClientToolsTestService(upstream)
			body := []byte(fmt.Sprintf(`{"model":"gpt-5.6-sol","stream":%v,"reasoning":{"effort":"max"},"input":"test","tools":[{"type":"custom","name":"apply_patch"}]}`, stream))
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest("POST", "/v1/responses", bytes.NewReader(body))
			c.Request.Header.Set("x-codex-turn-state", "must-not-leak")
			result, err := svc.Forward(context.Background(), c, excelAccount(), body)
			require.NoError(t, err)
			require.NotNil(t, result)
			require.Equal(t, "bps.openai.com", upstream.lastReq.URL.Host)
			require.Equal(t, "/basispoints/api/responses", upstream.lastReq.URL.Path)
			require.Equal(t, "Bearer test-token", upstream.lastReq.Header.Get("Authorization"))
			require.Empty(t, upstream.lastReq.Header.Get("x-codex-turn-state"))
			require.Equal(t, HTTPUpstreamProfileLongStream, HTTPUpstreamProfileFromContext(upstream.lastReq.Context()))
			require.True(t, HTTPUpstreamRedirectsDisabled(upstream.lastReq.Context()))
			require.False(t, gjson.GetBytes(upstream.lastBody, "tools").Exists())
			require.Equal(t, "xhigh", gjson.GetBytes(upstream.lastBody, "reasoning_effort").String())
			require.Equal(t, "xhigh", *result.ReasoningEffort)
			require.Equal(t, 10, result.Usage.InputTokens)
			require.Contains(t, rec.Body.String(), "21")
		})
	}
}
func TestExcelBPSModelDeniedDoesNotFailover(t *testing.T) {
	upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: 403, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"error":{"code":"basispoints_model_access_changed","message":"SECRET_UPSTREAM"}}`))}}
	svc := openAIClientToolsTestService(upstream)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	_, err := svc.Forward(context.Background(), c, excelAccount(), []byte(`{"model":"gpt-5.6-sol","input":"x"}`))
	require.Error(t, err)
	var failover *UpstreamFailoverError
	require.NotErrorAs(t, err, &failover)
	require.Equal(t, 403, rec.Code)
	require.Contains(t, rec.Body.String(), "basispoints_model_access_changed")
	require.NotContains(t, rec.Body.String(), "SECRET_UPSTREAM")
}
func TestExcelBPSThreadScopeSeparatesParallelChildren(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	c.Request.Header.Set("session_id", "shared-root")
	first, _ := resolveOpenAIWSExecutionScope(c, []byte(`{"client_metadata":{"x-codex-turn-metadata":"{\"thread_id\":\"child-A\"}"}}`), 1)
	second, _ := resolveOpenAIWSExecutionScope(c, []byte(`{"client_metadata":{"x-codex-turn-metadata":"{\"thread_id\":\"child-B\"}"}}`), 1)
	require.NotEmpty(t, first)
	require.NotEqual(t, first, second)
}
