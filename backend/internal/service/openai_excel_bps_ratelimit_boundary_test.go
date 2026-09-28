package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestExcelBPS429AfterClientOutputDoesNotFailover(t *testing.T) {
	for _, mode := range []string{"text", "headers", "compact heartbeat", "committed marker"} {
		t.Run(mode, func(t *testing.T) {
			upstream := &httpUpstreamRecorder{resp: &http.Response{
				StatusCode: http.StatusTooManyRequests,
				Header:     http.Header{"Retry-After": {"45"}},
				Body:       io.NopCloser(strings.NewReader("PRIVATE_UPSTREAM")),
			}}
			svc := openAIClientToolsTestService(upstream)
			account := excelAccount()
			repo := &excelBPSQuotaRepo{writes: make(chan excelBPSQuotaWrite, 4)}
			svc.accountRepo = repo
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
			c.Header("Content-Type", "text/event-stream")
			switch mode {
			case "text":
				_, _ = c.Writer.WriteString("data: {\"type\":\"response.output_text.delta\",\"delta\":\"partial\"}\n\n")
			case "headers":
				c.Writer.Flush()
			case "compact heartbeat":
				c.Request.URL.Path = "/v1/responses/compact"
				stop := startOpenAISSEKeepalive(c, time.Hour)
				defer stop()
				value, _ := c.Get(openAICompactSSEKeepaliveKey)
				require.True(t, value.(*openAICompactSSEKeepalive).beat())
			case "committed marker":
				MarkResponseCommitted(c)
			}
			result, err := svc.Forward(context.Background(), c, account, []byte("{\"model\":\"gpt-6-astra\",\"stream\":true,\"input\":\"test\"}"))
			require.Error(t, err)
			var failover *UpstreamFailoverError
			require.NotErrorAs(t, err, &failover, "even heartbeat-only output forbids BPS replay")
			require.Nil(t, result)
			require.Len(t, upstream.requests, 1)
			require.True(t, IsResponseCommitted(c))
			require.Contains(t, rec.Body.String(), string(ExcelBPSRateLimitedReason))
			if mode != "committed marker" {
				require.Contains(t, rec.Body.String(), "response.failed")
				require.Equal(t, http.StatusOK, rec.Code)
			}
			require.NotContains(t, rec.Body.String(), "PRIVATE_UPSTREAM")
			require.InDelta(t, 45, excelBPSCooldownRemaining(t, svc, account.ID).Seconds(), 2)
			requireNoExcelBPSQuotaWrite(t, repo)
			require.False(t, svc.isOpenAIAccountRuntimeBlocked(account))
		})
	}
}

func TestExcelBPS429StopsPendingCompactHeartbeatBeforeFailover(t *testing.T) {
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusTooManyRequests, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("{}")),
	}}
	svc := openAIClientToolsTestService(upstream)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses/compact", nil)
	stop := startOpenAISSEKeepalive(c, time.Hour)
	defer stop()
	value, _ := c.Get(openAICompactSSEKeepaliveKey)
	heartbeat := value.(*openAICompactSSEKeepalive)
	_, err := svc.Forward(context.Background(), c, excelAccount(), []byte("{\"model\":\"gpt-6-astra\",\"input\":\"test\"}"))
	requireExcelBPSRateLimitFailover(t, err, c)
	require.False(t, heartbeat.beat(), "a stopped heartbeat cannot write while switching accounts")
	require.False(t, c.Writer.Written())
}
