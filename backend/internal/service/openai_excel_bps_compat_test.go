package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service/basispoints"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func excelBPSCompatBody(t *testing.T, kind string, stream bool, effort string) []byte {
	t.Helper()
	payload := map[string]any{"model": "gpt-6-astra", "stream": stream}
	if kind == "responses" {
		payload["input"] = "test"
		payload["reasoning"] = map[string]any{"effort": effort}
	} else {
		payload["messages"] = []any{map[string]any{"role": "user", "content": "test"}}
		payload["max_tokens"] = 1024
		if kind == "messages" {
			payload["thinking"] = map[string]any{"type": "adaptive"}
			payload["output_config"] = map[string]any{"effort": effort}
		} else {
			payload["reasoning_effort"] = effort
			payload["stream_options"] = map[string]any{"include_usage": true}
		}
	}
	body, err := json.Marshal(payload)
	require.NoError(t, err)
	return body
}

func excelBPSCompatWire(text string) string {
	content, _ := json.Marshal(text)
	return `data: {"type":"response.completed","response":{"id":"resp_bps_compat","error":null,"status":"completed","model":"gpt-6-astra","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":` + string(content) + `}]}],"usage":` + excelBPSUsageWithCreation + `}}` + "\n\n"
}

func excelBPSCompatUpstream(wire string) *httpUpstreamRecorder {
	return &httpUpstreamRecorder{resp: &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(wire))}}
}

func forwardExcelBPSCompatTest(t *testing.T, svc *OpenAIGatewayService, account *Account, kind string, body []byte, dispatch string) (*OpenAIForwardResult, *httptest.ResponseRecorder, error) {
	t.Helper()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/"+kind, bytes.NewReader(body))
	c.Request.Header.Set("thread-id", t.Name())
	c.Request.Header.Set("x-codex-turn-state", "native-only-must-not-leak")
	var result *OpenAIForwardResult
	var err error
	switch kind {
	case "responses":
		result, err = svc.Forward(c.Request.Context(), c, account, body)
	case "chat/completions":
		result, err = svc.ForwardAsChatCompletions(c.Request.Context(), c, account, body, "", dispatch)
	case "messages":
		result, err = svc.ForwardAsAnthropic(c.Request.Context(), c, account, body, "", dispatch)
	default:
		t.Fatalf("unknown ingress: %s", kind)
	}
	return result, rec, err
}

func excelBPSCompatData(body string) []gjson.Result {
	var result []gjson.Result
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "data: ") && strings.TrimPrefix(line, "data: ") != "[DONE]" {
			result = append(result, gjson.Parse(strings.TrimPrefix(line, "data: ")))
		}
	}
	return result
}

func TestExcelBPSCompatAllIngresses(t *testing.T) {
	for _, kind := range []string{"responses", "chat/completions", "messages"} {
		for _, stream := range []bool{false, true} {
			for _, effort := range []string{"medium", "high", "max"} {
				t.Run(fmt.Sprintf("%s/stream=%t/effort=%s", kind, stream, effort), func(t *testing.T) {
					upstream := excelBPSCompatUpstream(excelBPSCompatWire("audit-ok"))
					result, rec, err := forwardExcelBPSCompatTest(t, openAIClientToolsTestService(upstream), excelAccount(), kind, excelBPSCompatBody(t, kind, stream, effort), "")
					require.NoError(t, err)
					require.Equal(t, 200, rec.Code)
					require.Equal(t, basispoints.ResponsesURL, upstream.lastReq.URL.String())
					require.Len(t, upstream.requests, 1)
					require.Equal(t, "/basispoints/api/responses", result.UpstreamEndpoint)
					require.Equal(t, stream, result.Stream)
					require.Equal(t, "gpt-6-astra", result.Model)
					require.Equal(t, "gpt-6-astra", result.UpstreamModel)
					require.Equal(t, 1000, result.Usage.InputTokens)
					require.Equal(t, 200, result.Usage.CacheCreationInputTokens)
					require.Equal(t, 100, result.Usage.CacheReadInputTokens)
					wantEffort := effort
					if effort == "max" {
						wantEffort = "xhigh"
					}
					require.Equal(t, wantEffort, gjson.GetBytes(upstream.lastBody, "reasoning_effort").String())
					require.Equal(t, wantEffort, *result.ReasoningEffort)
					require.Equal(t, HTTPUpstreamProfileLongStream, HTTPUpstreamProfileFromContext(upstream.lastReq.Context()))
					require.True(t, HTTPUpstreamRedirectsDisabled(upstream.lastReq.Context()))
					require.Empty(t, upstream.lastReq.Header.Get("x-codex-turn-state"))
					require.Contains(t, rec.Body.String(), "audit-ok", "terminal-only text must reach every client")
					if kind == "chat/completions" {
						if stream {
							require.Equal(t, 1, strings.Count(rec.Body.String(), "[DONE]"))
							require.Contains(t, rec.Body.String(), `"finish_reason":"stop"`)
						} else {
							require.Equal(t, "audit-ok", gjson.GetBytes(rec.Body.Bytes(), "choices.0.message.content").String())
						}
					}
					if kind == "messages" {
						if stream {
							require.Equal(t, 1, strings.Count(rec.Body.String(), "event: message_stop"))
						} else {
							require.Equal(t, "message", gjson.GetBytes(rec.Body.Bytes(), "type").String())
							require.Equal(t, "audit-ok", gjson.GetBytes(rec.Body.Bytes(), "content.0.text").String())
						}
					}
				})
			}
		}
	}
}

func TestExcelBPSCompatRoutingMapsModelExactlyOnce(t *testing.T) {
	for _, kind := range []string{"chat/completions", "messages"} {
		for _, tc := range []struct {
			name, request, dispatch string
			models                  []string
			enabled, wantBPS        bool
		}{
			{"account alias", "public-model", "", []string{"gpt-6-astra"}, true, true},
			{"group dispatch", "group-model", "gpt-6-astra", []string{"gpt-6-astra"}, true, true},
			{"unselected model", "public-model", "", []string{"gpt-5.6-sol"}, true, false},
			{"empty scope", "public-model", "", []string{}, true, false},
			{"disabled", "public-model", "", []string{"gpt-6-astra"}, false, false},
		} {
			t.Run(kind+"/"+tc.name, func(t *testing.T) {
				account := excelAccount()
				account.Extra["openai_excel_bps"] = tc.enabled
				account.Extra["openai_excel_bps_models"] = tc.models
				account.Credentials["model_mapping"] = map[string]any{"public-model": "gpt-6-astra", "gpt-6-astra": "must-not-double-map"}
				upstream := excelBPSCompatUpstream(excelBPSCompatWire("ok"))
				body, err := sjson.SetBytes(excelBPSCompatBody(t, kind, false, "high"), "model", tc.request)
				require.NoError(t, err)
				result, rec, err := forwardExcelBPSCompatTest(t, openAIClientToolsTestService(upstream), account, kind, body, tc.dispatch)
				require.NoError(t, err)
				require.Equal(t, tc.request, result.Model)
				require.Equal(t, tc.request, gjson.GetBytes(rec.Body.Bytes(), "model").String())
				require.Equal(t, "gpt-6-astra", result.BillingModel)
				require.Equal(t, "gpt-6-astra", gjson.GetBytes(upstream.lastBody, "model").String())
				wantURL := chatgptCodexAPIURL
				if tc.wantBPS {
					wantURL = basispoints.ResponsesURL
				}
				require.Equal(t, wantURL, upstream.lastReq.URL.String())
			})
		}
	}
}

func TestExcelBPSCompatCacheCreationPresentationDoesNotChangeBilling(t *testing.T) {
	for _, kind := range []string{"chat/completions", "messages"} {
		for _, stream := range []bool{false, true} {
			for _, enabled := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/stream=%t/normalize=%t", kind, stream, enabled), func(t *testing.T) {
					account := excelAccount()
					account.Extra["openai_excel_bps_cache_creation_as_input"] = enabled
					upstream := excelBPSCompatUpstream(excelBPSCompatWire("ok"))
					result, rec, err := forwardExcelBPSCompatTest(t, openAIClientToolsTestService(upstream), account, kind, excelBPSCompatBody(t, kind, stream, "high"), "")
					require.NoError(t, err)
					require.Equal(t, 200, result.Usage.CacheCreationInputTokens)
					usage := gjson.GetBytes(rec.Body.Bytes(), "usage")
					if stream {
						for _, event := range excelBPSCompatData(rec.Body.String()) {
							if event.Get("usage").Exists() {
								usage = event.Get("usage")
							}
						}
					}
					creation := int64(200)
					if enabled {
						creation = 0
					}
					if kind == "chat/completions" {
						require.EqualValues(t, 1000, usage.Get("prompt_tokens").Int())
						require.Equal(t, creation, usage.Get("prompt_tokens_details.cache_creation_tokens").Int())
						require.EqualValues(t, 100, usage.Get("prompt_tokens_details.cached_tokens").Int())
					} else {
						require.Equal(t, creation, usage.Get("cache_creation_input_tokens").Int())
						require.EqualValues(t, 100, usage.Get("cache_read_input_tokens").Int())
						require.Equal(t, int64(900)-creation, usage.Get("input_tokens").Int())
					}
				})
			}
		}
	}
}

func TestExcelBPSCompatErrorsAreNeverSuccessfulOrRetried(t *testing.T) {
	for _, kind := range []string{"chat/completions", "messages"} {
		for _, stream := range []bool{false, true} {
			for _, tc := range []struct {
				name, wire, code string
				status           int
			}{
				{"forbidden", `{"error":{"message":"rejected"}}`, "basispoints_upstream_error", 403},
				{"rate limit", `{"error":{"message":"retry later"}}`, "basispoints_upstream_error", 429},
				{"model access", `{"error":{"code":"basispoints_model_access_changed"}}`, "basispoints_model_access_changed", 403},
				{"truncated", "data: {\"type\":\"response.output_text.delta\",\"delta\":\"partial\"}\n\n", "basispoints_stream_incomplete", 200},
				{"failed", "data: {\"type\":\"response.failed\",\"response\":{\"status\":\"failed\",\"error\":{\"code\":\"test_failure\",\"message\":\"failed\"}}}\n\n", "", 200},
				{"incomplete", "data: {\"type\":\"response.incomplete\",\"response\":{\"status\":\"incomplete\",\"output\":[]}}\n\n", "", 200},
			} {
				t.Run(fmt.Sprintf("%s/stream=%t/%s", kind, stream, tc.name), func(t *testing.T) {
					upstream := excelBPSCompatUpstream(tc.wire)
					upstream.resp.StatusCode = tc.status
					_, rec, err := forwardExcelBPSCompatTest(t, openAIClientToolsTestService(upstream), excelAccount(), kind, excelBPSCompatBody(t, kind, stream, "high"), "")
					require.Error(t, err)
					require.Len(t, upstream.requests, 1)
					require.Contains(t, rec.Body.String(), `"error"`)
					if tc.code != "" {
						require.Contains(t, rec.Body.String(), tc.code)
					}
					require.NotContains(t, rec.Body.String(), `"finish_reason":"stop"`)
					require.NotContains(t, rec.Body.String(), "event: message_stop")
					if tc.status != 200 {
						require.Equal(t, tc.status, rec.Code)
						require.True(t, gjson.ValidBytes(rec.Body.Bytes()))
						if kind == "messages" {
							require.Equal(t, "error", gjson.GetBytes(rec.Body.Bytes(), "type").String())
						}
					}
				})
			}
		}
	}
}

func TestExcelBPSCompatStreamRecoversTextWithoutDuplication(t *testing.T) {
	for _, kind := range []string{"chat/completions", "messages"} {
		t.Run(kind, func(t *testing.T) {
			wire := "data: {\"type\":\"response.output_text.delta\",\"output_index\":0,\"content_index\":0,\"delta\":\"audit-\"}\n\ndata: {\"type\":\"response.output_text.done\",\"output_index\":0,\"content_index\":0,\"text\":\"audit-ok\"}\n\n" + excelBPSCompatWire("audit-ok")
			_, rec, err := forwardExcelBPSCompatTest(t, openAIClientToolsTestService(excelBPSCompatUpstream(wire)), excelAccount(), kind, excelBPSCompatBody(t, kind, true, "high"), "")
			require.NoError(t, err)
			var content strings.Builder
			for _, event := range excelBPSCompatData(rec.Body.String()) {
				if kind == "messages" {
					_, err := content.WriteString(event.Get("delta.text").String())
					require.NoError(t, err)
				} else {
					_, err := content.WriteString(event.Get("choices.0.delta.content").String())
					require.NoError(t, err)
				}
			}
			require.Equal(t, "audit-ok", content.String())
		})
	}
}

func TestExcelBPSCompatResponsesShapeAndNativeFallback(t *testing.T) {
	for _, fallback := range []bool{false, true} {
		t.Run(fmt.Sprintf("fallback=%t", fallback), func(t *testing.T) {
			body := []byte(`{"model":"gpt-6-astra","stream":false,"reasoning":{"effort":"high"},"input":"preserve this input"}`)
			if fallback {
				var err error
				body, err = sjson.SetRawBytes(body, "tools", []byte(`[{"type":"image_generation"}]`))
				require.NoError(t, err)
			}
			upstream := excelBPSCompatUpstream(excelBPSCompatWire("ok"))
			_, rec, err := forwardExcelBPSCompatTest(t, openAIClientToolsTestService(upstream), excelAccount(), "chat/completions", body, "")
			require.NoError(t, err)
			require.Contains(t, string(upstream.lastBody), "preserve this input")
			wantURL := basispoints.ResponsesURL
			if fallback {
				wantURL = chatgptCodexAPIURL
				require.Equal(t, "image_generation", rec.Header().Get("X-Codex2API-Basispoints-Bypass"))
			}
			require.Equal(t, wantURL, upstream.lastReq.URL.String())
		})
	}
}

func TestExcelBPSGroupPelicanUsesBPSAndParsesCompletion(t *testing.T) {
	scheduled, plan, key := groupPelicanFixture()
	upstream := excelBPSCompatUpstream(excelBPSCompatWire("<svg></svg>"))
	gateway := openAIClientToolsTestService(upstream)
	account := excelAccount()
	account.Credentials["model_mapping"] = map[string]any{"public-model": "gpt-6-astra"}
	scheduled.SetGroupGateway(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer "+key.Key, r.Header.Get("Authorization"))
		require.Equal(t, plan.GroupID, ScheduledPelicanGroupID(r.Context()))
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		c, _ := gin.CreateTestContext(w)
		c.Request = r
		_, err = gateway.ForwardAsChatCompletions(r.Context(), c, account, body, "", "")
		require.NoError(t, err)
	}))
	result, err := scheduled.RunGroupPelican(context.Background(), plan)
	require.NoError(t, err)
	require.Equal(t, "success", result.Status)
	require.Equal(t, "<svg></svg>", result.ResponseText)
	require.Equal(t, basispoints.ResponsesURL, upstream.lastReq.URL.String())
	require.Equal(t, "high", gjson.GetBytes(upstream.lastBody, "reasoning_effort").String())
}

func TestExcelBPSCompatToolCallsReplayAcrossTurns(t *testing.T) {
	for _, kind := range []string{"chat/completions", "messages"} {
		for _, stream := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/stream=%t", kind, stream), func(t *testing.T) {
				wire := `data: {"type":"response.completed","response":{"id":"resp_bps_tool","status":"completed","model":"gpt-6-astra","output":[{"type":"function_call","id":"fc_bps_tool","call_id":"call_bps_tool","name":"shell","arguments":"{\"command\":\"pwd\"}"}]}}` + "\n\n"
				upstream := excelBPSCompatUpstream(wire)
				svc := openAIClientToolsTestService(upstream)
				body := excelBPSCompatBody(t, kind, stream, "high")
				tools := `[{"type":"function","function":{"name":"shell","parameters":{"type":"object","properties":{"command":{"type":"string"}}}}}]`
				if kind == "messages" {
					tools = `[{"name":"shell","input_schema":{"type":"object","properties":{"command":{"type":"string"}}}}]`
				}
				body, err := sjson.SetRawBytes(body, "tools", []byte(tools))
				require.NoError(t, err)
				_, rec, err := forwardExcelBPSCompatTest(t, svc, excelAccount(), kind, body, "")
				require.NoError(t, err)
				var callID, name, args string
				if stream {
					for _, event := range excelBPSCompatData(rec.Body.String()) {
						if kind == "messages" {
							if event.Get("content_block.type").String() == "tool_use" {
								callID = event.Get("content_block.id").String()
								name = event.Get("content_block.name").String()
							}
							args += event.Get("delta.partial_json").String()
						} else {
							call := event.Get("choices.0.delta.tool_calls.0")
							if id := call.Get("id").String(); id != "" {
								callID = id
							}
							if value := call.Get("function.name").String(); value != "" {
								name = value
							}
							args += call.Get("function.arguments").String()
						}
					}
				} else if kind == "messages" {
					callID = gjson.GetBytes(rec.Body.Bytes(), "content.0.id").String()
					name = gjson.GetBytes(rec.Body.Bytes(), "content.0.name").String()
					args = gjson.GetBytes(rec.Body.Bytes(), "content.0.input").Raw
				} else {
					call := gjson.GetBytes(rec.Body.Bytes(), "choices.0.message.tool_calls.0")
					callID, name, args = call.Get("id").String(), call.Get("function.name").String(), call.Get("function.arguments").String()
				}
				require.NotEmpty(t, callID)
				require.Equal(t, "shell", name)
				require.JSONEq(t, `{"command":"pwd"}`, args)
				var messages any
				if kind == "messages" {
					messages = []any{
						map[string]any{"role": "user", "content": "test"},
						map[string]any{"role": "assistant", "content": []any{map[string]any{"type": "tool_use", "id": callID, "name": name, "input": json.RawMessage(args)}}},
						map[string]any{"role": "user", "content": []any{map[string]any{"type": "tool_result", "tool_use_id": callID, "content": "/project"}}},
					}
				} else {
					messages = []any{
						map[string]any{"role": "user", "content": "test"},
						map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{"type": "function", "id": callID, "function": map[string]any{"name": name, "arguments": args}}}},
						map[string]any{"role": "tool", "tool_call_id": callID, "content": "/project"},
					}
				}
				body, err = sjson.SetBytes(body, "messages", messages)
				require.NoError(t, err)
				upstream.resp = excelBPSCompatUpstream(excelBPSCompatWire("done")).resp
				_, rec, err = forwardExcelBPSCompatTest(t, svc, excelAccount(), kind, body, "")
				require.NoError(t, err)
				require.Contains(t, rec.Body.String(), "done")
				require.Len(t, upstream.requests, 2)
				require.Equal(t, basispoints.ResponsesURL, upstream.lastReq.URL.String())
				var sawCall, sawOutput bool
				for _, item := range gjson.GetBytes(upstream.lastBody, "input").Array() {
					if item.Get("type").String() == "function_call" {
						require.Equal(t, "run_officejs", item.Get("name").String())
						require.Contains(t, item.Get("arguments").String(), "pwd")
						sawCall = true
					}
					if item.Get("type").String() == "function_call_output" {
						require.Contains(t, item.Get("output").String(), "/project")
						sawOutput = true
					}
				}
				require.True(t, sawCall)
				require.True(t, sawOutput)
			})
		}
	}
}

func TestExcelBPSCompatCancellationClosesUpstream(t *testing.T) {
	for _, kind := range []string{"chat/completions", "messages"} {
		t.Run(kind, func(t *testing.T) {
			reader, writer := io.Pipe()
			defer func() { _ = writer.Close() }()
			upstream := excelBPSCompatUpstream("")
			upstream.resp.Body = reader
			svc := openAIClientToolsTestService(upstream)
			body := excelBPSCompatBody(t, kind, true, "high")
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/"+kind, bytes.NewReader(body))
			ctx, cancel := context.WithCancel(c.Request.Context())
			cancel()
			c.Request = c.Request.WithContext(ctx)
			var result *OpenAIForwardResult
			var err error
			if kind == "messages" {
				result, err = svc.ForwardAsAnthropic(ctx, c, excelAccount(), body, "", "")
			} else {
				result, err = svc.ForwardAsChatCompletions(ctx, c, excelAccount(), body, "", "")
			}
			require.ErrorIs(t, err, context.Canceled)
			require.True(t, result.ClientDisconnect)
			require.Len(t, upstream.requests, 1)
			_, err = writer.Write([]byte("must not block after cancellation"))
			require.Error(t, err)
		})
	}
}

func TestExcelBPSCompatStructuredOutput(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprint(stream), func(t *testing.T) {
			body := excelBPSCompatBody(t, "chat/completions", stream, "high")
			body, err := sjson.SetRawBytes(body, "response_format", []byte(`{"type":"json_schema","json_schema":{"name":"result","strict":true,"schema":{"type":"object","properties":{"ok":{"type":"boolean"}},"required":["ok"],"additionalProperties":false}}}`))
			require.NoError(t, err)
			for _, valid := range []bool{true, false} {
				answer := `{"ok":true}`
				if !valid {
					answer = `{"ok":"wrong-type"}`
				}
				upstream := excelBPSCompatUpstream(excelBPSCompatWire(answer))
				_, rec, err := forwardExcelBPSCompatTest(t, openAIClientToolsTestService(upstream), excelAccount(), "chat/completions", body, "")
				if valid {
					require.NoError(t, err)
					var output string
					if stream {
						for _, event := range excelBPSCompatData(rec.Body.String()) {
							output += event.Get("choices.0.delta.content").String()
						}
					} else {
						output = gjson.GetBytes(rec.Body.Bytes(), "choices.0.message.content").String()
					}
					require.JSONEq(t, answer, output)
				} else {
					require.Error(t, err)
					require.NotContains(t, rec.Body.String(), "wrong-type")
					require.NotContains(t, rec.Body.String(), `"finish_reason":"stop"`)
				}
				require.Equal(t, basispoints.ResponsesURL, upstream.lastReq.URL.String())
			}
		})
	}
}
