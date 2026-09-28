//go:build unit

package handler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func excelBPSSSE429(partial bool) *http.Response {
	wire := "data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_rejected\",\"output\":[]}}\n\n" +
		"data: {\"type\":\"response.in_progress\",\"response\":{\"id\":\"resp_rejected\",\"output\":[]}}\n\n"
	if partial {
		wire += "data: {\"type\":\"response.output_text.delta\",\"delta\":\"partial answer\"}\n\n"
	}
	wire += "event: error\ndata: {\"type\":\"error\",\"error\":{\"code\":\"rate_limit_exceeded\",\"type\":\"tokens\",\"message\":\"PRIVATE_UPSTREAM TPM exhausted\",\"headers\":{\"retry-after\":\"30\"}}}\n\n"
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(wire))}
}

func TestOpenAIResponsesExcelBPSStream429Failover(t *testing.T) {
	for _, stream := range []bool{false, true} {
		for _, allLimited := range []bool{false, true} {
			for _, loadBatch := range []bool{false, true} {
				t.Run(fmt.Sprintf("stream=%t/all_limited=%t/load_batch=%t", stream, allLimited, loadBatch), func(t *testing.T) {
					upstream := &excelBPSFailoverUpstream{answer: func(call int) *http.Response {
						if call == 0 || allLimited {
							return excelBPSSSE429(false)
						}
						return excelBPSCompleted()
					}}
					handler := newExcelBPSFailoverTestHandler(t, upstream, loadBatch)
					c, rec := newExcelBPSFailoverTestContext(context.Background(), stream)
					handler.Responses(c)
					calls := upstream.calls()
					require.Len(t, calls, 2)
					require.NotEqual(t, calls[0], calls[1])
					require.NotContains(t, rec.Body.String(), "PRIVATE_UPSTREAM")
					require.NotContains(t, rec.Body.String(), "resp_rejected")
					if allLimited {
						require.Equal(t, http.StatusTooManyRequests, rec.Code)
						require.Equal(t, "basispoints_rate_limited", gjson.Get(rec.Body.String(), "error.code").String())
						require.Equal(t, "30", rec.Header().Get("Retry-After"))
					} else {
						require.Equal(t, http.StatusOK, rec.Code)
						require.Contains(t, rec.Body.String(), "done")
					}
					c, rec = newExcelBPSFailoverTestContext(context.Background(), stream)
					handler.Responses(c)
					if allLimited {
						require.Len(t, upstream.calls(), 2, "cooling accounts must not be tried again")
						require.Equal(t, http.StatusTooManyRequests, rec.Code)
					} else {
						require.Equal(t, []int64{calls[0], calls[1], calls[1]}, upstream.calls())
					}
				})
			}
		}
	}
}

func TestOpenAIResponsesExcelBPSStream429AfterText(t *testing.T) {
	upstream := &excelBPSFailoverUpstream{answer: func(int) *http.Response { return excelBPSSSE429(true) }}
	handler := newExcelBPSFailoverTestHandler(t, upstream, false)
	c, rec := newExcelBPSFailoverTestContext(context.Background(), true)
	handler.Responses(c)
	require.Len(t, upstream.calls(), 1)
	require.Contains(t, rec.Body.String(), "partial answer")
	require.Equal(t, 1, strings.Count(rec.Body.String(), "event: response.failed\n"))
	require.NotContains(t, rec.Body.String(), "event: error\n")
}

func TestOpenAICompatibleExcelBPSStream429(t *testing.T) {
	for _, path := range []string{"/v1/chat/completions", "/v1/messages"} {
		for _, stream := range []bool{false, true} {
			for _, allLimited := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/stream=%t/all_limited=%t", path, stream, allLimited), func(t *testing.T) {
					upstream := &excelBPSFailoverUpstream{answer: func(call int) *http.Response {
						if allLimited || call == 0 {
							return excelBPSSSE429(false)
						}
						return excelBPSCompleted()
					}}
					handler := newExcelBPSFailoverTestHandler(t, upstream, true)
					c, rec := newExcelBPSFailoverTestContext(context.Background(), stream)
					body := fmt.Sprintf("{\"model\":\"gpt-6-astra\",\"stream\":%t,\"max_tokens\":64,\"messages\":[{\"role\":\"user\",\"content\":\"hello\"}]}", stream)
					c.Request = httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
					c.Request.Header.Set("Content-Type", "application/json")
					if path == "/v1/messages" {
						handler.Messages(c)
					} else {
						handler.ChatCompletions(c)
					}
					calls := upstream.calls()
					require.Len(t, calls, 2)
					require.NotEqual(t, calls[0], calls[1])
					if allLimited {
						require.Equal(t, http.StatusTooManyRequests, rec.Code)
					} else {
						require.Equal(t, http.StatusOK, rec.Code)
						require.Contains(t, rec.Body.String(), "done")
					}
					require.NotContains(t, rec.Body.String(), "PRIVATE_UPSTREAM")
					require.NotContains(t, rec.Body.String(), "resp_rejected")
				})
			}
		}
	}
}
