package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestIntelligentReasoningSelectedEffortReachesHTTPProtocol(t *testing.T) {
	for _, protocol := range []string{APIProtocolResponses, APIProtocolChatCompletions} {
		for _, effort := range []string{"", "none", "minimal", "low", "medium", "high", "xhigh"} {
			t.Run(protocol+"/"+effort, func(t *testing.T) {
				var received map[string]any
				svc, repo := intelligentRunnerFixture(t, func(w http.ResponseWriter, r *http.Request) {
					require.NoError(t, json.NewDecoder(r.Body).Decode(&received))
					if protocol == APIProtocolResponses {
						require.Equal(t, "/v1/responses", r.URL.Path)
						fmt.Fprint(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"ANSWER: 12\"}\n\ndata: {\"type\":\"response.completed\"}\n\n")
					} else {
						require.Equal(t, "/v1/chat/completions", r.URL.Path)
						fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"ANSWER: 12\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
					}
				})
				if protocol == APIProtocolChatCompletions {
					repo.account.Extra[openai_compat.ExtraKeyResponsesMode] = string(openai_compat.ResponsesSupportModeForceChatCompletions)
				}
				record := &IntelligentTestRecord{AccountID: 81, Input: "Solve the configured question", ConfigSnapshot: &IntelligentTestConfig{Model: "test-reasoning-model", ReasoningEffort: effort}}
				require.NoError(t, svc.RunIntelligentTest(context.Background(), record))
				if effort == "" {
					require.NotContains(t, received, "reasoning")
					require.NotContains(t, received, "reasoning_effort")
					require.Empty(t, record.ConfigSnapshot.Execution.SentReasoningEffort)
				} else if protocol == APIProtocolResponses {
					reasoning, ok := received["reasoning"].(map[string]any)
					require.True(t, ok)
					require.Equal(t, effort, reasoning["effort"])
					require.NotContains(t, received, "reasoning_effort")
				} else {
					require.Equal(t, effort, received["reasoning_effort"])
					require.NotContains(t, received, "reasoning")
				}
				require.Equal(t, effort, record.ConfigSnapshot.Execution.RequestedReasoningEffort)
				require.Equal(t, effort, record.ConfigSnapshot.Execution.SentReasoningEffort)
				if effort != "" {
					require.Equal(t, protocol, record.ConfigSnapshot.Execution.ReasoningProtocol)
				}
				require.Zero(t, repo.writes)
			})
		}
	}
}

func TestIntelligentReasoningCNAdaptiveAndGrokUseNativeFields(t *testing.T) {
	for _, platform := range []string{PlatformKimi, PlatformGrok} {
		t.Run(platform, func(t *testing.T) {
			var received map[string]any
			svc, repo := intelligentRunnerFixture(t, func(w http.ResponseWriter, r *http.Request) {
				require.NoError(t, json.NewDecoder(r.Body).Decode(&received))
				if platform == PlatformGrok {
					fmt.Fprint(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"ANSWER: 12\"}\n\ndata: {\"type\":\"response.completed\"}\n\n")
				} else {
					fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"ANSWER: 12\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
				}
			})
			repo.account.Platform = platform
			repo.account.Credentials["api_protocol"] = APIProtocolAdaptive
			record := &IntelligentTestRecord{AccountID: 81, Input: "Solve this question", ConfigSnapshot: &IntelligentTestConfig{Model: "grok-4.6", ReasoningEffort: "xhigh"}}
			require.NoError(t, svc.RunIntelligentTest(context.Background(), record))
			if platform == PlatformGrok {
				reasoning, ok := received["reasoning"].(map[string]any)
				require.True(t, ok)
				require.Equal(t, "xhigh", reasoning["effort"])
			} else {
				require.Equal(t, "xhigh", received["reasoning_effort"])
			}
		})
	}
}

func TestIntelligentReasoningProtectionDoesNotOverrideSelectedEffort(t *testing.T) {
	var received []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		received = append(received, body)
		fmt.Fprint(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"ANSWER: 12\"}\n\ndata: {\"type\":\"response.completed\"}\n\n")
	}))
	defer server.Close()
	target, err := url.Parse(server.URL)
	require.NoError(t, err)
	transport := &intelligentRedirectUpstream{target: target, client: server.Client()}
	account := &Account{ID: 813, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Concurrency: 4,
		Credentials: map[string]any{"access_token": "local-fixture-token", "expires_at": time.Now().Add(time.Hour).Format(time.RFC3339)}}
	admin := &adminServiceImpl{accountRepo: &upstreamBillingProbeAccountRepo{accounts: map[int64]*Account{account.ID: account}}}
	account, err = NewAntiDegradeService(admin).ApplyAntiDegradeMode(context.Background(), account.ID, AntiDegradeMode1)
	require.NoError(t, err)
	repo := &intelligentRunnerRepo{account: account}
	svc := NewAccountTestService(repo, nil, nil, nil, nil, transport, &config.Config{}, nil)
	for i, effort := range []string{"none", "minimal", "low", "high", "xhigh"} {
		record := &IntelligentTestRecord{AccountID: account.ID, Input: "draw a pelican", ConfigSnapshot: &IntelligentTestConfig{Model: "gpt-5.3-codex", ReasoningEffort: effort}}
		require.NoError(t, svc.RunIntelligentTest(context.Background(), record))
		reasoning, ok := received[i]["reasoning"].(map[string]any)
		require.True(t, ok)
		require.Equal(t, effort, reasoning["effort"])
		require.Equal(t, effort, record.ConfigSnapshot.Execution.SentReasoningEffort)
		require.True(t, record.AntiDegradation)
	}
	// Shared adapters must not inject a remembered selection into a later
	// ordinary connectivity probe on the same service/account.
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/test", nil)
	require.NoError(t, svc.TestAccountConnection(c, account.ID, "gpt-5.3-codex", "", AccountTestModeDefault))
	require.NotContains(t, received[len(received)-1], "reasoning")
	require.NotContains(t, received[len(received)-1], "reasoning_effort")
	require.Zero(t, repo.writes)
}

func TestIntelligentReasoningUnsupportedRejectedBeforeRequest(t *testing.T) {
	for _, tc := range []struct{ platform, protocol, model, effort string }{
		{PlatformAnthropic, "", "claude-sonnet", "high"},
		{PlatformGemini, "", "gemini-pro", "low"},
		{PlatformAntigravity, "", "gemini-pro", "medium"},
		{PlatformKimi, APIProtocolAnthropic, "kimi", "high"},
		{PlatformGrok, "", "grok-composer", "high"},
		{PlatformGrok, "", "grok-4.3", "minimal"},
		{PlatformGrok, "", "grok-4.3", "xhigh"},
		{PlatformOpenAI, "", "test-model", "ultra"},
	} {
		t.Run(tc.platform+tc.protocol+tc.effort, func(t *testing.T) {
			requests := 0
			svc, repo := intelligentRunnerFixture(t, func(http.ResponseWriter, *http.Request) { requests++ })
			repo.account.Platform = tc.platform
			repo.account.Credentials["api_protocol"] = tc.protocol
			record := &IntelligentTestRecord{AccountID: 81, Input: "draw", ConfigSnapshot: &IntelligentTestConfig{Model: tc.model, ReasoningEffort: tc.effort}}
			require.Error(t, svc.RunIntelligentTest(context.Background(), record))
			require.Equal(t, "request_error", record.Status)
			require.Zero(t, requests)
		})
	}
}

func TestScheduledIntelligentRunnerRechecksAccountAvailability(t *testing.T) {
	for _, state := range []string{"disabled", "unschedulable", "expired"} {
		t.Run(state, func(t *testing.T) {
			requests := 0
			svc, repo := intelligentRunnerFixture(t, func(http.ResponseWriter, *http.Request) { requests++ })
			repo.account.Status = StatusActive
			repo.account.Schedulable = true
			switch state {
			case "disabled":
				repo.account.Status = StatusDisabled
			case "unschedulable":
				repo.account.Schedulable = false
			case "expired":
				expired := time.Now().Add(-time.Hour)
				repo.account.AutoPauseOnExpired = true
				repo.account.ExpiresAt = &expired
			}
			record := &IntelligentTestRecord{AccountID: 81, Scheduled: true, Input: "draw", ConfigSnapshot: &IntelligentTestConfig{Model: "test-model"}}
			require.Error(t, svc.RunIntelligentTest(context.Background(), record))
			require.Equal(t, "account_error", record.Status)
			require.Zero(t, requests)
		})
	}
}

func TestScheduledIntelligentRunnerRejectsLocalQuotaWithoutChangingManualProbe(t *testing.T) {
	for _, quota := range []string{"total", "daily", "weekly"} {
		t.Run(quota, func(t *testing.T) {
			requests := 0
			svc, repo := intelligentRunnerFixture(t, func(w http.ResponseWriter, r *http.Request) {
				requests++
				fmt.Fprint(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"ANSWER: 12\"}\n\ndata: {\"type\":\"response.completed\"}\n\n")
			})
			repo.account.Status = StatusActive
			repo.account.Schedulable = true
			prefix := "quota"
			if quota != "total" {
				prefix += "_" + quota
				repo.account.Extra[prefix+"_start"] = time.Now().Add(-time.Minute).Format(time.RFC3339)
			}
			repo.account.Extra[prefix+"_limit"] = float64(1)
			repo.account.Extra[prefix+"_used"] = float64(1)
			require.True(t, repo.account.IsQuotaExceeded())
			record := &IntelligentTestRecord{AccountID: 81, Scheduled: true, Input: "draw", ConfigSnapshot: &IntelligentTestConfig{Model: "test-model"}}
			require.ErrorContains(t, svc.RunIntelligentTest(context.Background(), record), "本地配额已耗尽")
			require.Equal(t, "account_error", record.Status)
			require.Zero(t, requests, "an automatic test must honor local spending limits before any upstream request")

			manual := &IntelligentTestRecord{AccountID: 81, Input: "draw", ConfigSnapshot: &IntelligentTestConfig{Model: "test-model"}}
			require.NoError(t, svc.RunIntelligentTest(context.Background(), manual))
			require.Equal(t, 1, requests, "an explicit administrative probe retains its existing behavior")
			require.True(t, repo.account.IsQuotaExceeded())
			require.Zero(t, repo.writes)
		})
	}
}
