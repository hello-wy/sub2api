//go:build unit

package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpenAIForwardQuarantinesBeforeUsageSubmission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, stream := range []bool{false, true} {
		t.Run(map[bool]string{false: "json", true: "sse"}[stream], func(t *testing.T) {
			account := rawChatCompletionsTestAccount()
			account.ID = 9721
			defer ClearPendingAccountModelMismatch(account.ID)
			repo := &modelMismatchMarkerStub{}
			responseBody := `{"id":"resp_mismatch","object":"response","status":"completed","model":"gpt-5.6-luna","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]}],"usage":{"input_tokens":10,"output_tokens":5}}`
			contentType := "application/json"
			if stream {
				responseBody = "event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":" + responseBody + "}\n\n"
				contentType = "text/event-stream"
			}
			upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{contentType}}, Body: io.NopCloser(strings.NewReader(responseBody))}}
			svc := &OpenAIGatewayService{accountRepo: repo, cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}
			body := []byte(`{"model":"gpt-6-astra","input":"hello","stream":` + map[bool]string{false: "false", true: "true"}[stream] + `}`)
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(string(body)))
			result, err := svc.Forward(context.Background(), c, account, body)
			require.NoError(t, err)
			require.NotNil(t, result)
			require.Equal(t, 1, repo.calls, "must quarantine synchronously without RecordUsage")
			require.Equal(t, "gpt-6-astra", repo.expected)
			require.Equal(t, "gpt-5.6-luna", repo.actual)
			require.Equal(t, 5, result.Usage.OutputTokens)
			require.Equal(t, http.StatusOK, rec.Code, "already served output and accounting remain intact")
		})
	}
}

func TestOpenAIChatForwardQuarantinesRawUpstreamModel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	account := rawChatCompletionsTestAccount()
	account.ID = 9722
	account.Credentials["api_protocol"] = "chat_completions"
	account.Extra = map[string]any{openai_compat.ExtraKeyResponsesSupported: false}
	defer ClearPendingAccountModelMismatch(account.ID)
	repo := &modelMismatchMarkerStub{}
	upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"id":"chat_model","object":"chat.completion","model":"gpt-5.6-luna","choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5}}`))}}
	svc := &OpenAIGatewayService{accountRepo: repo, cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}
	body := []byte(`{"model":"gpt-6-astra","messages":[{"role":"user","content":"hello"}]}`)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(string(body)))
	result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 1, repo.calls)
	require.Equal(t, "gpt-5.6-luna", repo.actual)
}

func TestOpenAIForwardObservationKeepsServingAndRetainsUsageMismatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, stream := range []bool{false, true} {
		t.Run(map[bool]string{false: "json", true: "sse"}[stream], func(t *testing.T) {
			account := rawChatCompletionsTestAccount()
			account.ID = 9842
			defer ClearPendingAccountModelMismatch(account.ID)
			repo := &mismatchPolicyStub{}
			responseBody := `{"id":"resp_mismatch","object":"response","status":"completed","model":"gpt-5.6-luna","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]}],"usage":{"input_tokens":10,"output_tokens":5}}`
			contentType := "application/json"
			if stream {
				responseBody = "event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":" + responseBody + "}\n\n"
				contentType = "text/event-stream"
			}
			upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{contentType}}, Body: io.NopCloser(strings.NewReader(responseBody))}}
			svc := &OpenAIGatewayService{accountRepo: repo, cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}
			body := []byte(`{"model":"gpt-6-astra","input":"hello","stream":` + map[bool]string{false: "false", true: "true"}[stream] + `}`)
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(string(body)))
			result, err := svc.Forward(context.Background(), c, account, body)
			require.NoError(t, err)
			require.Equal(t, 1, repo.calls)
			require.False(t, hasPendingAccountModelMismatch(account))
			require.Equal(t, "gpt-5.6-luna", result.UpstreamResponseModel)
			require.Equal(t, 5, result.Usage.OutputTokens)
			require.Equal(t, http.StatusOK, rec.Code)
		})
	}
}

func TestGatewayForwardAuditsOtherModelDifferencesWithoutQuarantine(t *testing.T) {
	gin.SetMode(gin.TestMode)
	account := newAnthropicAPIKeyAccountForTest()
	account.ID = 9723
	defer ClearPendingAccountModelMismatch(account.ID)
	repo := &modelMismatchMarkerStub{}
	upstream := &anthropicHTTPUpstreamRecorder{resp: &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"id":"msg_model","type":"message","role":"assistant","model":"claude-haiku-4-5","content":[{"type":"text","text":"hello"}],"stop_reason":"end_turn","usage":{"input_tokens":10,"output_tokens":5}}`))}}
	svc := newForwardPartialUsageServiceForTest(upstream)
	svc.accountRepo = repo
	body := []byte(`{"model":"claude-sonnet-4-6","max_tokens":20,"messages":[{"role":"user","content":"hello"}]}`)
	parsed, err := ParseGatewayRequest(NewRequestBodyRef(body), PlatformAnthropic)
	require.NoError(t, err)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(string(body)))
	result, err := svc.Forward(context.Background(), c, account, parsed)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Zero(t, repo.calls)
	require.Equal(t, "claude-haiku-4-5", result.UpstreamResponseModel)
	require.False(t, hasPendingAccountModelMismatch(account))
}

func TestModelMismatchQuarantineAndBillingFailuresAreIndependent(t *testing.T) {
	for _, gateway := range []bool{false, true} {
		t.Run(map[bool]string{false: "openai", true: "gateway"}[gateway], func(t *testing.T) {
			defer ClearPendingAccountModelMismatch(9724)
			marker := &modelMismatchMarkerStub{err: errors.New("quarantine write failed")}
			usage := &openAIRecordUsageLogRepoStub{}
			billingErr := errors.New("billing failed")
			billing := &openAIRecordUsageBillingRepoStub{err: billingErr}
			account := &Account{ID: 9724, Type: AccountTypeAPIKey}
			var err error
			if gateway {
				svc := newGatewayRecordUsageServiceWithBillingRepoForTest(usage, billing, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{})
				svc.accountRepo = marker
				err = svc.RecordUsage(context.Background(), &RecordUsageInput{Result: &ForwardResult{RequestID: "billing-model", Model: "claude-sonnet-4", UpstreamModel: "gpt-6-astra", UpstreamResponseModel: "gpt-5.6-luna", Usage: ClaudeUsage{InputTokens: 10, OutputTokens: 5}, Duration: time.Second}, APIKey: &APIKey{ID: 1}, User: &User{ID: 2}, Account: account})
			} else {
				svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(usage, billing, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
				svc.accountRepo = marker
				err = svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{Result: &OpenAIForwardResult{RequestID: "billing-model", Model: "gpt-5.1", UpstreamModel: "gpt-6-astra", UpstreamResponseModel: "gpt-5.6-luna", Usage: OpenAIUsage{InputTokens: 10, OutputTokens: 5}, Duration: time.Second}, APIKey: &APIKey{ID: 1}, User: &User{ID: 2}, Account: account})
			}
			require.ErrorIs(t, err, billingErr)
			require.Equal(t, 1, marker.calls, "billing failure must not suppress quarantine")
			require.Equal(t, 1, billing.calls, "quarantine failure must not suppress billing")
			require.Equal(t, 1, usage.calls)
			require.True(t, *usage.lastLog.UpstreamModelMismatch)
		})
	}
}
