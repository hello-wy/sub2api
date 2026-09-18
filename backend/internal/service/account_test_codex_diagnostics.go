package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/gin-gonic/gin"
)

const AccountTestModeGateway = "gateway"
const codexProbeTimeout = 45 * time.Second

type codexProbeContextKey struct{}

// TargetURL is observed at the transport boundary, never inferred from the UI.
// Empty means no request was sent. URLs exclude credentials and query strings.
type codexCapabilityResult struct {
	Capability string `json:"capability"`
	Status     string `json:"status"`
	TargetURL  string `json:"target_url,omitempty"`
	Source     string `json:"source,omitempty"`
	HTTPStatus int    `json:"http_status,omitempty"`
	DurationMS int64  `json:"duration_ms,omitempty"`
	Code       string `json:"code,omitempty"`
}

type codexProbeObservation struct {
	result    *codexCapabilityResult
	completed bool
	emit      func()
}

func codexObservation(ctx context.Context) *codexProbeObservation {
	observation, _ := ctx.Value(codexProbeContextKey{}).(*codexProbeObservation)
	return observation
}

func safeCodexDiagnosticURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	parsed.User, parsed.RawQuery, parsed.Fragment = nil, "", ""
	parsed.ForceQuery = false
	return parsed.String()
}

func (s *AccountTestService) testCodexGateway(c *gin.Context, account *Account, model string) error {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("X-Accel-Buffering", "no")
	c.Writer.Flush()
	if !account.IsOpenAIOAuthLike() {
		return s.sendErrorAndEnd(c, "Codex gateway diagnostics require an OpenAI OAuth account")
	}
	if model == "" {
		model = openai.DefaultTestModel
	}
	if isOpenAIImageModel(account.GetMappedModel(model)) {
		return s.sendErrorAndEnd(c, "Codex gateway diagnostics require a text model")
	}
	originalRequest := c.Request
	defer func() { c.Request = originalRequest }()
	allPassed := true
	for _, capability := range []string{"http", "websocket", "compact"} {
		result := codexCapabilityResult{Capability: capability, Status: "pending"}
		s.sendEvent(c, TestEvent{Type: "capability_result", Data: result})
	}
	for _, capability := range []string{"http", "websocket", "compact"} {
		result := codexCapabilityResult{Capability: capability, Status: "running"}
		emit := func() { s.sendEvent(c, TestEvent{Type: "capability_result", Data: result}) }
		// Only capability_result events pass through while a probe runs; its
		// terminal event is captured until the whole suite has completed.
		observation := &codexProbeObservation{result: &result, emit: emit}
		ctx, cancel := context.WithTimeout(originalRequest.Context(), codexProbeTimeout)
		ctx = context.WithValue(ctx, codexProbeContextKey{}, observation)
		c.Request = originalRequest.WithContext(ctx)
		started := time.Now()
		credentialAccount, err := resolveCredentialAccount(ctx, s.accountRepo, account)
		if err == nil {
			var base string
			base, err = normalizeCodexBaseURL(credentialAccount.GetExtraString(codexBaseURLExtraKey), s.cfg)
			if err == nil {
				result.Source = "official"
				if base != "" {
					result.Source = "account"
					if account.IsCredentialShadow() {
						result.Source = "parent"
					}
				}
			}
		}
		observation.emit()
		switch {
		case ctx.Err() != nil:
			err = ctx.Err()
		case err != nil:
			result.Code = "configuration_error"
		case s.pluginManager != nil && s.pluginManager.ShouldRouteOpenAIOAuth(account):
			// The plugin owns its internal destination and may bridge WS to HTTP.
			// Do not bypass it or claim to have observed its actual transport URL.
			result.Status, result.Code = "skipped", "plugin_transport"
		case capability == "websocket":
			err = s.probeCodexWebSocket(c, account, credentialAccount, model, observation)
		default:
			mode := AccountTestModeDefault
			if capability == "compact" {
				mode = AccountTestModeCompact
			}
			err = s.testOpenAIAccountConnection(c, account, model, "", mode)
		}
		if result.Status != "skipped" {
			result.Status = "passed"
			if err != nil || !observation.completed || ctx.Err() != nil {
				result.Status = "failed"
				if result.Code == "" {
					result.Code = "probe_failed"
				}
				if result.HTTPStatus >= 400 {
					result.Code = "http_error"
				}
				if errors.Is(ctx.Err(), context.DeadlineExceeded) {
					result.Code = "timeout"
				}
				if originalRequest.Context().Err() != nil {
					result.Status, result.Code = "cancelled", "cancelled"
				}
			}
		}
		cancel()
		result.DurationMS = time.Since(started).Milliseconds()
		c.Request = originalRequest
		emit()
		if result.Status != "passed" {
			allPassed = false
		}
	}
	if !allPassed {
		s.sendEvent(c, TestEvent{Type: "test_complete", Success: false, Error: "Codex gateway diagnostics did not pass all capabilities"})
		return errors.New("codex gateway diagnostics did not pass all capabilities")
	}
	s.sendEvent(c, TestEvent{Type: "test_complete", Success: true})
	return nil
}

func (s *AccountTestService) probeCodexWebSocket(c *gin.Context, account, credentialAccount *Account, model string, observation *codexProbeObservation) error {
	ctx := c.Request.Context()
	gateway := s.openaiGatewayService
	if gateway == nil {
		gateway = &OpenAIGatewayService{accountRepo: s.accountRepo, cfg: s.cfg}
	}
	target, err := gateway.buildOpenAIResponsesWSURLForContext(ctx, account)
	if err != nil {
		observation.result.Code = "configuration_error"
		return err
	}
	token := credentialAccount.GetOpenAIAccessToken()
	if token == "" && !credentialAccount.IsOpenAIAgentIdentity() {
		observation.result.Code = "authentication_error"
		return errors.New("no access token")
	}
	model = normalizeOpenAIModelForUpstream(credentialAccount, account.GetMappedModel(model))
	headers, _, err := gateway.buildOpenAIWSHeaders(ctx, c, account, token,
		OpenAIWSProtocolDecision{Transport: OpenAIUpstreamTransportResponsesWebsocketV2}, true, "", "", "", model, "")
	if err != nil {
		return err
	}
	if credentialAccount.IsOpenAIAgentIdentity() {
		auth, authErr := buildAgentIdentityAuthenticationHeaders(ctx, s.accountRepo, s.agentIdentityWS, &s.agentIdentityTaskMu, credentialAccount)
		if authErr != nil {
			observation.result.Code = "authentication_error"
			return authErr
		}
		for key, values := range auth {
			headers[key] = values
		}
	}
	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	dialer := s.codexTestWSDialer
	if dialer == nil {
		dialer = gateway.getOpenAIWSPassthroughDialer()
	}
	observation.result.TargetURL = safeCodexDiagnosticURL(target)
	observation.emit()
	conn, status, _, err := dialer.Dial(ctx, target, headers, proxyURL)
	observation.result.HTTPStatus = status
	if err != nil {
		observation.result.Code = "websocket_handshake_failed"
		return err
	}
	defer func() {
		if closer, ok := conn.(openAIWSForceCloser); ok {
			_ = closer.CloseNow()
		} else {
			_ = conn.Close()
		}
	}()
	if status == 0 {
		observation.result.HTTPStatus = http.StatusSwitchingProtocols
	}
	observation.emit()
	payload := gateway.buildOpenAIWSCreatePayload(createOpenAITestPayload(model, true), account)
	if err := conn.WriteJSON(ctx, payload); err != nil {
		return err
	}
	for {
		message, err := conn.ReadMessage(ctx)
		if err != nil {
			observation.result.Code = "incomplete_response"
			return err
		}
		var event struct {
			Type     string `json:"type"`
			Response struct {
				Status string `json:"status"`
			} `json:"response"`
		}
		if err := json.Unmarshal(message, &event); err != nil {
			return err
		}
		switch event.Type {
		case "response.completed", "response.done":
			if event.Response.Status != "" && event.Response.Status != "completed" {
				observation.result.Code = "response_failed"
				return errors.New("response not completed")
			}
			observation.completed = true
			return nil
		case "error", "response.failed", "response.incomplete":
			observation.result.Code = "response_failed"
			return errors.New("websocket response failed")
		}
	}
}

// Compaction may arrive as SSE or as a unary response from a compatible relay.
func codexProbeResponseCompleted(body []byte) bool {
	var unary struct {
		Output json.RawMessage `json:"output"`
		Status string          `json:"status"`
	}
	if json.Unmarshal(body, &unary) == nil && len(unary.Output) > 0 {
		return unary.Status == "" || unary.Status == "completed"
	}
	completed := false
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		var event struct {
			Type     string `json:"type"`
			Response struct {
				Status string `json:"status"`
			} `json:"response"`
		}
		if json.Unmarshal([]byte(strings.TrimSpace(strings.TrimPrefix(line, "data:"))), &event) != nil {
			continue
		}
		switch event.Type {
		case "error", "response.failed", "response.incomplete":
			return false
		case "response.completed", "response.done":
			if event.Response.Status != "" && event.Response.Status != "completed" {
				return false
			}
			completed = true
		}
	}
	return completed
}

// The standard WS dialer may follow redirects during its HTTP handshake.
// Observe the response request so the result shows the actual final hop.
func observeCodexWebSocketHandshake(ctx context.Context, response *http.Response) {
	observation := codexObservation(ctx)
	if observation == nil || response == nil {
		return
	}
	observation.result.HTTPStatus = response.StatusCode
	if response.Request != nil && response.Request.URL != nil {
		target := *response.Request.URL
		if target.Scheme == "https" {
			target.Scheme = "wss"
		}
		if target.Scheme == "http" {
			target.Scheme = "ws"
		}
		observation.result.TargetURL = safeCodexDiagnosticURL(target.String())
	}
	observation.emit()
}
