package service

import (
	"context"
	"encoding/json"
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

// BPS can accept HTTP and then reject inference inside SSE. The production
// failures contained two lifecycle events followed by this nested error.
const excelBPSStreamPreamble = "data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_rejected\",\"status\":\"in_progress\",\"output\":[]}}\n\n" +
	"data: {\"type\":\"response.in_progress\",\"response\":{\"id\":\"resp_rejected\",\"status\":\"in_progress\",\"output\":[]}}\n\n"

const excelBPSStream429 = "event: error\ndata: {\"type\":\"error\",\"error\":{\"code\":\"rate_limit_exceeded\",\"type\":\"tokens\",\"message\":\"PRIVATE_UPSTREAM TPM exhausted\",\"headers\":{\"retry-after\":\"31\",\"retry-after-ms\":\"203\"}}}\n\n"

func TestExcelBPSStream429BeforeOutput(t *testing.T) {
	for _, stream := range []bool{false, true} {
		for _, preamble := range []string{"", excelBPSStreamPreamble} {
			t.Run(fmt.Sprintf("stream=%t/preamble=%t", stream, preamble != ""), func(t *testing.T) {
				upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: http.StatusOK,
					Header: http.Header{"Content-Type": {"text/event-stream"}, "X-Codex-Primary-Used-Percent": {"100"}},
					Body:   io.NopCloser(strings.NewReader(preamble + excelBPSStream429)),
				}}
				svc := openAIClientToolsTestService(upstream)
				repo := &excelBPSQuotaRepo{writes: make(chan excelBPSQuotaWrite, 4)}
				svc.accountRepo = repo
				account := excelAccount()
				rec := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(rec)
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
				result, err := svc.Forward(context.Background(), c, account, []byte(fmt.Sprintf(
					"{\"model\":\"gpt-6-sol\",\"stream\":%t,\"input\":\"test\"}", stream)))
				requireExcelBPSRateLimitFailover(t, err, c)
				require.Nil(t, result, "rejected attempts must not produce zero-token usage records")
				require.Empty(t, rec.Body.String())
				require.False(t, c.Writer.Written())
				require.InDelta(t, 31, excelBPSCooldownRemaining(t, svc, account.ID).Seconds(), 2)
				requireNoExcelBPSQuotaWrite(t, repo)
				require.False(t, svc.isOpenAIAccountRuntimeBlocked(account))
			})
		}
	}
}

func TestExcelBPSStream429AfterOutputDoesNotReplay(t *testing.T) {
	for _, prewritten := range []bool{false, true} {
		t.Run(fmt.Sprint(prewritten), func(t *testing.T) {
			wire := excelBPSStreamPreamble
			if !prewritten {
				wire += "data: {\"type\":\"response.output_text.delta\",\"delta\":\"partial answer\"}\n\n"
			}
			upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(wire + excelBPSStream429))}}
			svc := openAIClientToolsTestService(upstream)
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
			if prewritten {
				_, _ = c.Writer.WriteString(": keepalive\n\n")
				c.Writer.Flush()
			}
			account := excelAccount()
			result, err := svc.Forward(context.Background(), c, account, []byte("{\"model\":\"gpt-6-sol\",\"stream\":true,\"input\":\"test\"}"))
			require.Error(t, err)
			var failover *UpstreamFailoverError
			require.NotErrorAs(t, err, &failover)
			require.NotNil(t, result)
			require.Len(t, upstream.requests, 1)
			require.Equal(t, 1, strings.Count(rec.Body.String(), "event: response.failed\n"))
			require.NotContains(t, rec.Body.String(), "event: error\n")
			require.NotContains(t, rec.Body.String(), "PRIVATE_UPSTREAM")
			require.True(t, IsResponseCommitted(c))
			require.True(t, svc.isExcelBPSCoolingDown(account, "gpt-6-sol"))
			if !prewritten {
				require.Contains(t, rec.Body.String(), "partial answer")
			}
		})
	}
}

func TestExcelBPSProtocolFailureRetainsMeasuredUsage(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprint(stream), func(t *testing.T) {
			code, err := json.Marshal(map[string]any{"name": "exec", "arguments": "PRIVATE_INVALID_INPUT"})
			require.NoError(t, err)
			arguments, err := json.Marshal(map[string]any{"code": string(code), "summary": "Run tool"})
			require.NoError(t, err)
			payload, err := json.Marshal(map[string]any{
				"type": "response.completed", "response": map[string]any{
					"id": "resp_protocol_usage", "model": "gpt-6-sol", "status": "completed",
					"usage":  json.RawMessage(excelBPSUsageWithCreation),
					"output": []any{map[string]any{"type": "function_call", "name": "run_officejs", "id": "fc_native", "call_id": "call_native", "arguments": string(arguments)}},
				},
			})
			require.NoError(t, err)
			upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("data: " + string(payload) + "\n\n"))}}
			svc := openAIClientToolsTestService(upstream)
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
			result, err := svc.Forward(context.Background(), c, excelAccount(), []byte(fmt.Sprintf(
				"{\"model\":\"gpt-6-sol\",\"stream\":%t,\"input\":\"test\",\"tools\":[{\"type\":\"custom\",\"name\":\"exec\"}]}", stream)))
			require.Error(t, err)
			require.NotNil(t, result)
			require.Equal(t, 1000, result.Usage.InputTokens)
			require.Equal(t, 50, result.Usage.OutputTokens)
			require.Equal(t, 100, result.Usage.CacheReadInputTokens)
			require.Equal(t, 200, result.Usage.CacheCreationInputTokens)
			require.Equal(t, "resp_protocol_usage", result.ResponseID)
			require.Equal(t, "gpt-6-sol", result.UpstreamResponseModel)
			require.Equal(t, "response.failed", result.UpstreamTerminalEvent)
			require.NotContains(t, rec.Body.String(), "PRIVATE_INVALID_INPUT")
			require.NotContains(t, rec.Body.String(), "run_officejs")
			if stream {
				require.Equal(t, 1, strings.Count(rec.Body.String(), "event: response.failed\n"))
				for _, line := range strings.Split(rec.Body.String(), "\n") {
					if strings.HasPrefix(line, "data: ") {
						payload := strings.TrimPrefix(line, "data: ")
						require.Equal(t, "basispoints_protocol_error", gjson.Get(payload, "response.error.code").String())
						require.EqualValues(t, 1000, gjson.Get(payload, "response.usage.input_tokens").Int())
					}
				}
			}
		})
	}
}
