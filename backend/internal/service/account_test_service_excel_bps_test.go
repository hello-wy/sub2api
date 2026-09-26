package service

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service/basispoints"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestBuildExcelBPSAccountTestBodyUsesResponsesContract(t *testing.T) {
	raw, err := buildExcelBPSAccountTestBody("gpt-6-astra", "糖果题", "")
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	if body["model"] != "gpt-6-astra" || body["stream"] != true || body["store"] != false {
		t.Fatalf("unexpected body: %#v", body)
	}
	input, _ := body["input"].([]any)
	item, _ := input[0].(map[string]any)
	contentItems, _ := item["content"].([]any)
	content, _ := contentItems[0].(map[string]any)
	if content["text"] != "糖果题" {
		t.Fatalf("prompt was not preserved: %#v", content)
	}
	reasoning, ok := body["reasoning"].(map[string]any)
	if !ok || reasoning["effort"] != "medium" {
		t.Fatalf("missing reasoning effort: %#v", body["reasoning"])
	}
}

func TestExcelBPSAccountOnlyUsesOAuth(t *testing.T) {
	oauth := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Extra: map[string]any{"openai_excel_bps": true}}
	apiKey := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Extra: map[string]any{"openai_excel_bps": true}}
	if !oauth.IsExcelBPSEnabled() || apiKey.IsExcelBPSEnabled() {
		t.Fatal("Excel BPS gate must be OAuth-only")
	}
}

func TestPelicanExcelBPSReasoningAndPrompt(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, effort := range []string{"low", "medium", "high"} {
		t.Run(effort, func(t *testing.T) {
			account := excelAccount()
			account.Credentials["model_mapping"] = map[string]any{
				"iq-alias": "gpt-6-astra", "gpt-6-astra": "gpt-6-sol",
			}
			account.Extra["openai_excel_bps_models"] = []string{"gpt-6-astra"}
			upstream := &httpUpstreamRecorder{resp: &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": {"text/event-stream"}},
				Body:       io.NopCloser(strings.NewReader("data: {\"type\":\"response.output_text.delta\",\"delta\":\"21\"}\n\ndata: {\"type\":\"response.completed\",\"response\":{\"id\":\"iq-result\",\"status\":\"completed\",\"output\":[]}}\n\n")),
			}}
			svc := &AccountTestService{
				accountRepo:          &stubOpenAIAccountRepo{accounts: []Account{*account}},
				httpUpstream:         upstream,
				openaiGatewayService: openAIClientToolsTestService(upstream),
			}
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/300/pelican-test", nil)
			require.NoError(t, svc.TestPelicanAccountConnection(c, account.ID, "iq-alias", CandyPrompt, effort))
			require.NotNil(t, upstream.lastReq)
			require.Equal(t, basispoints.ResponsesURL, upstream.lastReq.URL.String())
			require.Equal(t, "gpt-6-astra", gjson.GetBytes(upstream.lastBody, "model").String())
			require.Equal(t, effort, gjson.GetBytes(upstream.lastBody, "reasoning_effort").String())
			require.Contains(t, string(upstream.lastBody), "圆形 7 9 8")
			require.Contains(t, rec.Body.String(), "\"type\":\"test_complete\",\"success\":true")
		})
	}
}

// Account tests must apply the same single mapping as a user request and must
// preserve the existing native compaction probe even when BPS is enabled.
func TestExcelBPSAccountTestRouting(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const textWire = "event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"hello\"}\n\n" +
		"event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_probe\",\"status\":\"completed\",\"model\":\"gpt-6-astra\",\"output\":[{\"type\":\"message\",\"role\":\"assistant\",\"content\":[{\"type\":\"output_text\",\"text\":\"hello\"}]}]}}\n\n"
	for _, tc := range []struct {
		name, model, mode, url, upstreamModel, wire string
		scoped                                      bool
	}{
		{"mapped model uses BPS once", "test-alias", AccountTestModeDefault, basispoints.ResponsesURL, "gpt-6-astra", textWire, true},
		{"unselected model keeps native test", "gpt-6-luna", AccountTestModeDefault, chatgptCodexAPIURL, "gpt-6-luna", textWire, true},
		{"compact retains native probe", "test-alias", AccountTestModeCompact, chatgptCodexAPIURL, "gpt-6-astra", compactProbeSSESuccessBody, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			account := excelAccount()
			account.Credentials["model_mapping"] = map[string]any{
				"test-alias": "gpt-6-astra", "gpt-6-astra": "gpt-6-sol",
			}
			if tc.scoped {
				account.Extra["openai_excel_bps_models"] = []string{"gpt-6-astra"}
			}
			upstream := &httpUpstreamRecorder{resp: &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": {"text/event-stream"}},
				Body:       io.NopCloser(strings.NewReader(tc.wire)),
			}}
			svc := &AccountTestService{
				httpUpstream:         upstream,
				openaiGatewayService: openAIClientToolsTestService(upstream),
			}
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/300/test", nil)
			require.NoError(t, svc.testOpenAIAccountConnection(c, account, tc.model, "hello", tc.mode))
			require.NotNil(t, upstream.lastReq)
			require.Equal(t, tc.url, upstream.lastReq.URL.String())
			require.Equal(t, tc.upstreamModel, gjson.GetBytes(upstream.lastBody, "model").String())
			require.Contains(t, rec.Body.String(), "\"type\":\"test_complete\",\"success\":true")
			if tc.mode == AccountTestModeCompact {
				require.Contains(t, string(upstream.lastBody), "\"type\":\"compaction_trigger\"")
			}
		})
	}
}
