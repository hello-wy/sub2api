package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// The actual registered gateway executes this request, including group allow-
// lists, composite routes, model mapping, scheduler, concurrency and adapters.
// The private context capability is supplied by GroupStatusService after DB
// budget admission; there is no network-facing bypass endpoint.
func NewGroupProbeGatewayRunner(gateway http.Handler) service.GroupProbeRunner {
	return func(ctx context.Context, e service.GroupProbeExecution) service.GroupProbeResult {
		if e.Key == nil || service.GroupProbeKeyFromContext(ctx) != e.Key {
			return service.GroupProbeResult{ErrorCode: "internal_error"}
		}
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		payload := map[string]any{"model": e.Config.Model, "messages": []any{map[string]any{"role": "user", "content": "Reply with exactly OK."}}, "stream": true, "max_completion_tokens": e.Config.MaxOutputTokens}
		if e.Config.ReasoningEffort != "" {
			payload["reasoning_effort"] = e.Config.ReasoningEffort
		}
		body, _ := json.Marshal(payload)
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "http://group-probe.internal/v1/chat/completions", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "Internal-Group-Probe")
		req.RemoteAddr = "127.0.0.1:0"
		writer := &groupProbeWriter{header: http.Header{}, cancel: cancel}
		started := time.Now()
		gateway.ServeHTTP(writer, req)
		result := service.GroupProbeResult{LatencyMs: time.Since(started).Milliseconds()}
		if writer.overflow {
			result.ErrorCode = "output_limit"
			return result
		}
		if ctx.Err() != nil {
			result.ErrorCode = "timeout"
			return result
		}
		if writer.code >= 400 {
			switch writer.code {
			case 401, 403:
				result.ErrorCode = "authentication"
			case 429:
				result.ErrorCode = "capacity"
			case 400, 404:
				result.ErrorCode = "request_rejected"
			default:
				result.ErrorCode = "upstream_error"
			}
			return result
		}
		complete, failed, hasOutput := false, false, false
		for _, line := range strings.Split(string(writer.body), "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "data:") {
				continue
			}
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data == "[DONE]" {
				complete = true
				continue
			}
			if !gjson.Valid(data) {
				continue
			}
			v := gjson.Parse(data)
			if v.Get("error").Exists() || v.Get("type").String() == "error" || v.Get("type").String() == "response.failed" {
				failed = true
			}
			for _, choice := range v.Get("choices").Array() {
				if choice.Get("delta.content").String() != "" || choice.Get("message.content").String() != "" {
					hasOutput = true
				}
				if finish := choice.Get("finish_reason"); finish.Exists() && finish.Type != gjson.Null {
					complete = true
				}
			}
			switch v.Get("type").String() {
			case "response.completed", "message_stop":
				complete = true
			case "response.output_text.delta":
				hasOutput = hasOutput || v.Get("delta").String() != ""
			case "content_block_delta":
				hasOutput = hasOutput || v.Get("delta.text").String() != ""
			}
		}
		if failed {
			result.ErrorCode = "stream_error"
		} else if !complete {
			result.ErrorCode = "incomplete_response"
		} else if !hasOutput {
			result.ErrorCode = "empty_response"
		} else {
			result.Success = true
		}
		return result
	}
}

type groupProbeWriter struct {
	header   http.Header
	body     []byte
	code     int
	overflow bool
	cancel   context.CancelFunc
}

func (w *groupProbeWriter) Header() http.Header { return w.header }
func (w *groupProbeWriter) WriteHeader(code int) {
	if w.code == 0 {
		w.code = code
	}
}
func (w *groupProbeWriter) Flush() {
	if w.code == 0 {
		w.code = 200
	}
}
func (w *groupProbeWriter) Write(p []byte) (int, error) {
	if w.code == 0 {
		w.code = 200
	}
	if len(w.body)+len(p) > groupCompletionBufferLimit {
		w.overflow = true
		w.cancel()
		return 0, io.ErrShortBuffer
	}
	if w.body == nil {
		w.body = make([]byte, 0, groupCompletionBufferLimit)
	}
	w.body = append(w.body, p...)
	return len(p), nil
}

const groupOutcomeBillingIDKey = "group_outcome_billing_id"

// The billing key remains untouched. Only monitoring links the HTTP terminal
// outcome to this request's durable money-event key (audio/search, for example).
func bindGroupOutcomeBillingID(c *gin.Context, upstreamID string) {
	if c != nil && c.Request != nil {
		c.Set(groupOutcomeBillingIDKey, service.GroupOutcomeBillingRequestID(c.Request.Context(), upstreamID))
	}
}

func recordGroupMonitorOutcome(c *gin.Context, w *opsCaptureWriter, completion *groupCompletionWriter, svc *service.GroupStatusService) {
	if svc == nil || c.Request == nil || service.IsGroupProbe(c.Request.Context()) || c.Request.Method != http.MethodPost {
		return
	}
	if !groupMonitorCompletedEndpoint(c.Request.URL.Path) {
		return
	}
	if _, rejected := middleware.GetIngressRejectReason(c); rejected {
		return
	}
	key := getOpsAPIKey(c)
	if key == nil || key.GroupID == nil || key.Group == nil {
		return
	}
	requestID, _ := c.Request.Context().Value(ctxkey.RequestID).(string)
	if requestID == "" {
		requestID = c.Writer.Header().Get("X-Request-Id")
	}
	if clientID, _ := c.Request.Context().Value(ctxkey.ClientRequestID).(string); clientID != "" {
		requestID = "client:" + clientID
	} else if requestID != "" {
		requestID = "local:" + requestID
	}
	if requestID == "" {
		return
	}
	gatewayRequestID := requestID
	if billingID := c.GetString(groupOutcomeBillingIDKey); billingID != "" {
		requestID = billingID
	}
	parsed := parseOpsErrorResponse(w.capturedBytes())
	if terminal, ok := w.capturedTerminalError(); ok {
		parsed = terminal
	}
	status := c.Writer.Status()
	if completion != nil && completion.failed && status < 400 {
		status = 502
		parsed.ErrorType = "stream_read_error"
		parsed.Message = "upstream stream error"
	}
	if status < 400 && parsed.StreamFailure {
		status = inferStreamFailureStatus(c, parsed)
	}
	for _, streamError := range service.GetOpsStreamErrors(c) {
		if streamError.CountTowardsSLA {
			if streamError.IntendedStatus >= 400 {
				status = streamError.IntendedStatus
			} else {
				status = 502
			}
			parsed.Message = streamError.Message
		}
	}
	if status < 400 {
		switch {
		case errors.Is(c.Request.Context().Err(), context.Canceled):
			status = 499
			parsed.ErrorType = "client_cancelled"
		case errors.Is(c.Request.Context().Err(), context.DeadlineExceeded):
			status = 504
			parsed.ErrorType = "timeout"
		case completion != nil && completion.sse && !completion.complete:
			status = 502
			parsed.ErrorType = "stream_read_error"
			parsed.Message = "missing terminal event"
		}
	}
	if service.GetOpsCyberPolicy(c) != nil {
		status = 400
		parsed.ErrorType = "cyber_policy"
	}
	success := status >= 200 && status < 400
	category := ""
	if !success {
		phase, business, owner, source := classifyOpsErrorLog(c, parsed.ErrorType, parsed.Message, parsed.Code, status)
		category = service.ClassifyChannelMonitorV2Error(service.ChannelMonitorV2ErrorInput{ErrorType: parsed.ErrorType, ErrorOwner: owner, ErrorSource: source, Message: parsed.Message, StatusCode: status})
		if business && phase != "routing" && !hasOpsUpstreamErrorContext(c) {
			category = "invalid_request"
		}
	}
	outcome := service.GroupRequestOutcome{RequestID: requestID, GatewayRequestID: gatewayRequestID, GroupID: *key.GroupID, UserID: key.UserID, Platform: resolveOpsPlatform(c.Request.Context(), key, key.Group.Platform), Model: c.GetString(opsModelKey), Success: success, ErrorCategory: category, CompletedAt: time.Now().UTC()}
	if outcome.Model == "" {
		outcome.Model = "unknown"
	}
	if err := svc.RecordOutcome(c.Request.Context(), outcome); err != nil {
		logger.LegacyPrintf("group_status", "failed to persist request completion: %v", err)
	}
}

func groupMonitorCompletedEndpoint(path string) bool {
	for _, suffix := range []string{"/chat/completions", "/responses", "/messages", "/embeddings", "/images/generations", "/images/edits", "/tts", "/stt", "/web_search", "/x_search"} {
		if strings.HasSuffix(path, suffix) {
			return true
		}
	}
	return strings.HasSuffix(path, ":generateContent") || strings.HasSuffix(path, ":streamGenerateContent")
}
