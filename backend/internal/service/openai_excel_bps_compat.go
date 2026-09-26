package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/Wei-Shaw/sub2api/internal/service/basispoints"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

type excelBPSCompatOptions struct {
	model, billingModel, promptCacheKey string
	stream, anthropic, includeUsage     bool
}

// Use the resolved upstream model, not the client alias. Inspect both shapes:
// typed compat conversion can omit attributes of native hosted tools.
func useExcelBPSForCompat(c *gin.Context, account *Account, model string, bodies ...[]byte) bool {
	if !account.isExcelBPSUpstreamModelEnabled(model) {
		return false
	}
	for _, body := range bodies {
		if reason := basispoints.NativeFallbackReason(body); reason != "" {
			c.Header("X-Codex2API-Upstream", "codex")
			c.Header("X-Codex2API-Basispoints-Bypass", reason)
			return false
		}
	}
	return true
}

// Adapt only the downstream representation. The Responses BPS path remains
// authoritative for preparation, replay, images, usage, cancellation and
// account state. In particular, raw billing usage is collected before this
// writer sees the optionally normalized downstream usage. No request is retried.
func (s *OpenAIGatewayService) forwardExcelBPSCompat(ctx context.Context, c *gin.Context, account *Account, original, body []byte, start time.Time, options excelBPSCompatOptions) (*OpenAIForwardResult, error) {
	var err error
	body, err = sjson.SetBytes(body, "stream", options.stream)
	if err != nil {
		return nil, err
	}
	// Compat structs omit these routing fields. Preserve explicit client scope
	// before the shared BPS path isolates it by account and API key.
	for _, key := range []string{"client_metadata", "metadata", "prompt_cache_key"} {
		if value := gjson.GetBytes(original, key); value.Exists() {
			body, err = sjson.SetRawBytes(body, key, []byte(value.Raw))
			if err != nil {
				return nil, err
			}
		}
	}
	if options.promptCacheKey != "" && !gjson.GetBytes(body, "prompt_cache_key").Exists() {
		body, err = sjson.SetBytes(body, "prompt_cache_key", options.promptCacheKey)
		if err != nil {
			return nil, err
		}
	}
	originalWriter := c.Writer
	writer := &excelBPSCompatWriter{
		ResponseWriter: originalWriter, options: options,
		chat:      apicompat.NewResponsesEventToChatState(),
		messages:  apicompat.NewResponsesEventToAnthropicState(),
		textParts: make(map[[2]int]string),
	}
	writer.chat.Model, writer.chat.IncludeUsage = options.model, options.includeUsage
	writer.messages.Model = options.model
	c.Writer = writer
	defer func() { c.Writer = originalWriter }()
	result, err := s.forwardExcelBPSMapped(ctx, c, account, body, start, options.model, options.billingModel)
	if result != nil {
		result.BillingModel = options.billingModel
	}
	return result, err
}

type excelBPSCompatWriter struct {
	gin.ResponseWriter
	options   excelBPSCompatOptions
	chat      *apicompat.ResponsesEventToChatState
	messages  *apicompat.ResponsesEventToAnthropicState
	textParts map[[2]int]string
	pending   string
	finished  bool
}

func (w *excelBPSCompatWriter) WriteString(value string) (int, error) {
	return w.Write([]byte(value))
}

func (w *excelBPSCompatWriter) Write(data []byte) (int, error) {
	if !w.options.stream || !strings.HasPrefix(w.Header().Get("Content-Type"), "text/event-stream") {
		var output any
		if problem := gjson.GetBytes(data, "error"); problem.Exists() && problem.Type != gjson.Null {
			if !w.options.anthropic {
				return w.ResponseWriter.Write(data)
			}
			detail, _ := gjson.GetBytes(data, "error").Value().(map[string]any)
			if detail == nil {
				detail = map[string]any{"message": "Excel BPS request failed"}
			}
			if detail["type"] == nil || detail["type"] == "" {
				detail["type"] = "api_error"
			}
			output = map[string]any{"type": "error", "error": detail}
		} else {
			var response apicompat.ResponsesResponse
			if err := json.Unmarshal(data, &response); err != nil {
				return 0, err
			}
			if w.options.anthropic {
				output = apicompat.ResponsesToAnthropic(&response, w.options.model)
			} else {
				output = apicompat.ResponsesToChatCompletions(&response, w.options.model)
			}
		}
		encoded, err := json.Marshal(output)
		if err != nil {
			return 0, err
		}
		w.Header().Del("Content-Length")
		_, err = w.ResponseWriter.Write(encoded)
		return len(data), err
	}
	w.pending += string(data)
	for {
		line, rest, ok := strings.Cut(w.pending, "\n")
		if !ok {
			break
		}
		w.pending = rest
		if err := w.writeLine(strings.TrimSuffix(line, "\r")); err != nil {
			return 0, err
		}
	}
	return len(data), nil
}

func (w *excelBPSCompatWriter) writeLine(line string) error {
	if w.finished {
		return nil
	}
	if strings.HasPrefix(line, ":") {
		_, err := w.ResponseWriter.WriteString(line + "\n\n")
		return err
	}
	if !strings.HasPrefix(line, "data: ") {
		return nil
	}
	data := []byte(strings.TrimPrefix(line, "data: "))
	var event apicompat.ResponsesStreamEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return err
	}
	if event.Type == "error" || event.Type == "response.failed" || event.Type == "response.incomplete" {
		w.finished = true
		code, message := "basispoints_protocol_error", "Excel BPS did not complete the response"
		for _, prefix := range []string{"response.error.", "error.", ""} {
			if value := gjson.GetBytes(data, prefix+"code").String(); value != "" {
				code = value
			}
			if value := gjson.GetBytes(data, prefix+"message").String(); value != "" {
				message = value
			}
		}
		value := map[string]any{"error": map[string]string{"type": "api_error", "code": code, "message": message}}
		if w.options.anthropic {
			value["type"] = "error"
		}
		raw, err := json.Marshal(value)
		if err != nil {
			return err
		}
		wire := "data: " + string(raw) + "\n\n"
		if w.options.anthropic {
			wire = "event: error\n" + wire
		} else {
			wire += "data: [DONE]\n\n"
		}
		_, err = w.ResponseWriter.WriteString(wire)
		return err
	}
	if w.options.anthropic {
		for _, converted := range apicompat.ResponsesEventToAnthropicEvents(&event, w.messages) {
			wire, err := apicompat.ResponsesAnthropicEventToSSE(converted)
			if err != nil {
				return err
			}
			if _, err := w.ResponseWriter.WriteString(wire); err != nil {
				return err
			}
		}
	} else {
		if !w.chat.SentRole && event.Type != "response.created" {
			if err := w.writeChatEvent(&apicompat.ResponsesStreamEvent{Type: "response.created", Response: event.Response}); err != nil {
				return err
			}
		}
		if event.Type == "response.output_text.delta" {
			w.textParts[[2]int{event.OutputIndex, event.ContentIndex}] += event.Delta
		}
		// Some BPS responses only carry final text in output_text.done or in
		// the terminal snapshot. Recover each missing suffix exactly once.
		if event.Type == "response.output_text.done" {
			if err := w.recoverChatText(event.OutputIndex, event.ContentIndex, event.Text); err != nil {
				return err
			}
		}
		if event.Type == "response.completed" && event.Response != nil {
			for i, item := range event.Response.Output {
				if item.Type != "message" {
					continue
				}
				for j, part := range item.Content {
					if part.Type == "output_text" {
						if err := w.recoverChatText(i, j, part.Text); err != nil {
							return err
						}
					}
				}
			}
		}
		if err := w.writeChatEvent(&event); err != nil {
			return err
		}
	}
	if event.Type == "response.completed" {
		w.finished = true
		if !w.options.anthropic {
			_, err := w.ResponseWriter.WriteString("data: [DONE]\n\n")
			return err
		}
	}
	return nil
}

func (w *excelBPSCompatWriter) writeChatEvent(event *apicompat.ResponsesStreamEvent) error {
	for _, chunk := range apicompat.ResponsesEventToChatChunks(event, w.chat) {
		wire, err := apicompat.ChatChunkToSSE(chunk)
		if err != nil {
			return err
		}
		if _, err := w.ResponseWriter.WriteString(wire); err != nil {
			return err
		}
	}
	return nil
}

func (w *excelBPSCompatWriter) recoverChatText(outputIndex, contentIndex int, text string) error {
	key := [2]int{outputIndex, contentIndex}
	previous := w.textParts[key]
	if !strings.HasPrefix(text, previous) {
		return fmt.Errorf("excel BPS terminal text does not match streamed prefix")
	}
	w.textParts[key] = text
	return w.writeChatEvent(&apicompat.ResponsesStreamEvent{Type: "response.output_text.delta", Delta: strings.TrimPrefix(text, previous)})
}
