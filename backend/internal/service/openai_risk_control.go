package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	// OpenAIRiskControlExtraKey stores only the latest turn-state observation.
	// The opaque x-codex-turn-state value itself must never be persisted.
	OpenAIRiskControlExtraKey = "openai_risk_control"

	OpenAIRiskControlStatusNormal    = "normal"
	OpenAIRiskControlStatusSuspected = "suspected"
	OpenAIRiskControlStatusAbnormal  = "abnormal"
	OpenAIRiskControlStatusMissing   = "missing"

	openAIRiskControlProbeModel = "gpt-5.6-luna"

	openAIRiskControlAutoProbeTimeout = 30 * time.Second
	openAIRiskControlErrorThreshold   = 3
	openAIRiskControlErrorWindow      = 10 * time.Minute
	openAIRiskControlAutoCooldown     = 30 * time.Minute
)

var ErrOpenAIRiskControlUnsupported = infraerrors.BadRequest(
	"OPENAI_RISK_CONTROL_UNSUPPORTED",
	"risk control check only supports non-shadow OpenAI OAuth accounts",
)

type OpenAIRiskControlSnapshot struct {
	Status      string    `json:"status"`
	Suspected   bool      `json:"suspected"`
	StateLength *int      `json:"state_length,omitempty"`
	HTTPStatus  int       `json:"http_status"`
	CheckedAt   time.Time `json:"checked_at"`
	Reason      string    `json:"reason"`
}

const (
	openAIRiskControlTriggerAccountCreated = "account_created"
	openAIRiskControlTriggerRepeatedErrors = "repeated_429_529"
)

type openAIRiskControlErrorWindowState struct {
	startedAt     time.Time
	count         int
	cooldownUntil time.Time
}

// OpenAIRiskControlAutoProber is the narrow dependency used by account
// creation and upstream error handling. Automatic checks are best-effort and
// never alter the account's normal scheduling status.
type OpenAIRiskControlAutoProber interface {
	ScheduleOpenAIRiskControlProbe(accountID int64, trigger string) bool
	ObserveOpenAIRiskControlError(account *Account, statusCode int) bool
}

// OpenAIRiskControlService owns automatic scheduling and repeated-error
// aggregation. Its probe implementation is shared with the manual endpoint.
type OpenAIRiskControlService struct {
	probeService *AccountTestService

	mu           sync.Mutex
	inFlight     map[int64]struct{}
	errorWindows map[int64]openAIRiskControlErrorWindowState
	now          func() time.Time
	probe        func(context.Context, int64) (*OpenAIRiskControlSnapshot, error)
}

func NewOpenAIRiskControlService(
	accountRepo AccountRepository,
	httpUpstream HTTPUpstream,
	tlsFPProfileService *TLSFingerprintProfileService,
) *OpenAIRiskControlService {
	probeService := &AccountTestService{
		accountRepo:         accountRepo,
		httpUpstream:        httpUpstream,
		tlsFPProfileService: tlsFPProfileService,
	}
	service := &OpenAIRiskControlService{
		probeService: probeService,
		inFlight:     make(map[int64]struct{}),
		errorWindows: make(map[int64]openAIRiskControlErrorWindowState),
		now:          time.Now,
	}
	service.probe = probeService.probeOpenAIRiskControl
	return service
}

func (s *OpenAIRiskControlService) SetPluginManager(pluginManager *PluginManager) {
	if s != nil && s.probeService != nil {
		s.probeService.SetPluginManager(pluginManager)
	}
}

// ProbeOpenAIRiskControl performs a synchronous check. This path is used by
// the manual admin action and intentionally bypasses automatic cooldowns.
func (s *OpenAIRiskControlService) ProbeOpenAIRiskControl(ctx context.Context, accountID int64) (*OpenAIRiskControlSnapshot, error) {
	if s == nil || s.probe == nil {
		return nil, fmt.Errorf("OpenAI risk control probe is unavailable")
	}
	return s.probe(ctx, accountID)
}

// ScheduleOpenAIRiskControlProbe starts a bounded best-effort check and
// coalesces concurrent automatic triggers for the same account.
func (s *OpenAIRiskControlService) ScheduleOpenAIRiskControlProbe(accountID int64, trigger string) bool {
	if s == nil || s.probe == nil || accountID <= 0 {
		return false
	}

	s.mu.Lock()
	if s.inFlight == nil {
		s.inFlight = make(map[int64]struct{})
	}
	if _, exists := s.inFlight[accountID]; exists {
		s.mu.Unlock()
		return false
	}
	s.inFlight[accountID] = struct{}{}
	s.mu.Unlock()

	go func() {
		defer func() {
			s.mu.Lock()
			delete(s.inFlight, accountID)
			s.mu.Unlock()
			if recovered := recover(); recovered != nil {
				slog.Error("openai_risk_control_auto_probe_panic", "account_id", accountID, "trigger", trigger, "recover", recovered)
			}
		}()

		ctx, cancel := context.WithTimeout(context.Background(), openAIRiskControlAutoProbeTimeout)
		defer cancel()
		snapshot, err := s.probe(ctx, accountID)
		if err != nil {
			slog.Warn("openai_risk_control_auto_probe_failed", "account_id", accountID, "trigger", trigger, "error", err)
			return
		}
		if snapshot == nil {
			slog.Warn("openai_risk_control_auto_probe_failed", "account_id", accountID, "trigger", trigger, "error", "empty probe result")
			return
		}
		attrs := []any{
			"account_id", accountID,
			"trigger", trigger,
			"status", snapshot.Status,
			"http_status", snapshot.HTTPStatus,
		}
		if snapshot.StateLength != nil {
			attrs = append(attrs, "state_length", *snapshot.StateLength)
		}
		slog.Info("openai_risk_control_auto_probe_completed", attrs...)
	}()
	return true
}

// ObserveOpenAIRiskControlError counts combined 429/529 responses in a
// per-account window. The third response schedules one check and starts a
// cooldown to avoid turning an upstream incident into probe traffic.
func (s *OpenAIRiskControlService) ObserveOpenAIRiskControlError(account *Account, statusCode int) bool {
	if s == nil || !isOpenAIRiskControlEligible(account) || (statusCode != http.StatusTooManyRequests && statusCode != 529) {
		return false
	}

	now := time.Now()
	if s.now != nil {
		now = s.now()
	}
	s.mu.Lock()
	if s.errorWindows == nil {
		s.errorWindows = make(map[int64]openAIRiskControlErrorWindowState)
	}
	state := s.errorWindows[account.ID]
	if now.Before(state.cooldownUntil) {
		s.mu.Unlock()
		return false
	}
	if state.startedAt.IsZero() || now.Sub(state.startedAt) >= openAIRiskControlErrorWindow {
		state.startedAt = now
		state.count = 0
	}
	state.count++
	if state.count < openAIRiskControlErrorThreshold {
		s.errorWindows[account.ID] = state
		s.mu.Unlock()
		return false
	}
	state.startedAt = time.Time{}
	state.count = 0
	state.cooldownUntil = now.Add(openAIRiskControlAutoCooldown)
	s.errorWindows[account.ID] = state
	s.mu.Unlock()

	return s.ScheduleOpenAIRiskControlProbe(account.ID, openAIRiskControlTriggerRepeatedErrors)
}

func isOpenAIRiskControlEligible(account *Account) bool {
	return account != nil && account.Platform == PlatformOpenAI && account.Type == AccountTypeOAuth && !account.IsCredentialShadow()
}

func classifyOpenAIRiskControlState(value string) (status string, length *int, suspected bool, reason string) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return OpenAIRiskControlStatusMissing, nil, false, "turn_state_missing"
	}

	n := len(trimmed)
	switch n {
	case 292, 332:
		return OpenAIRiskControlStatusNormal, &n, false, "turn_state_normal_length"
	case 312:
		return OpenAIRiskControlStatusSuspected, &n, true, "turn_state_length_312"
	default:
		return OpenAIRiskControlStatusAbnormal, &n, false, "turn_state_abnormal_length"
	}
}

// ProbeOpenAIRiskControl sends one minimal request to the official Codex
// Responses endpoint, reads only the turn-state response header, then closes
// the stream. It persists the length and classification, never the raw state.
func (s *AccountTestService) ProbeOpenAIRiskControl(ctx context.Context, accountID int64) (*OpenAIRiskControlSnapshot, error) {
	if s != nil && s.openaiRiskControlService != nil {
		return s.openaiRiskControlService.ProbeOpenAIRiskControl(ctx, accountID)
	}
	return s.probeOpenAIRiskControl(ctx, accountID)
}

func (s *AccountTestService) probeOpenAIRiskControl(ctx context.Context, accountID int64) (*OpenAIRiskControlSnapshot, error) {
	if s == nil || s.accountRepo == nil || s.httpUpstream == nil {
		return nil, fmt.Errorf("OpenAI risk control probe is unavailable")
	}

	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if !isOpenAIRiskControlEligible(account) {
		return nil, ErrOpenAIRiskControlUnsupported
	}

	authToken := strings.TrimSpace(account.GetOpenAIAccessToken())
	if authToken == "" {
		return nil, infraerrors.BadRequest("OPENAI_ACCESS_TOKEN_MISSING", "OpenAI OAuth access token is missing")
	}

	payload, err := json.Marshal(map[string]any{
		"model":        openAIRiskControlProbeModel,
		"store":        false,
		"stream":       true,
		"instructions": "Reply with exactly: pong",
		"input": []map[string]any{{
			"role": "user",
			"content": []map[string]any{{
				"type": "input_text",
				"text": "ping",
			}},
		}},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal OpenAI risk control probe: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, chatgptCodexAPIURL, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build OpenAI risk control probe: %w", err)
	}
	req = req.WithContext(WithHTTPUpstreamProfile(req.Context(), HTTPUpstreamProfileOpenAI))
	req.Host = "chatgpt.com"
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+authToken)
	req.Header.Set("OpenAI-Beta", "responses=experimental")
	applyOpenAICodexProbeHeaders(req.Header)
	setOpenAIChatGPTAccountHeaders(req.Header, account)
	enforceCodexIdentityHeadersWithUA(req.Header, account.GetOpenAIUserAgent())
	account.ApplyHeaderOverrides(req.Header)

	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	resp, err := s.doOpenAIOfficialProbeUpstream(req, proxyURL, account)
	if err != nil {
		return nil, fmt.Errorf("request OpenAI risk control probe: %w", err)
	}
	if resp == nil {
		return nil, fmt.Errorf("request OpenAI risk control probe: upstream returned no response")
	}
	if resp.Body != nil {
		defer func() { _ = resp.Body.Close() }()
	}

	status, stateLength, suspected, reason := classifyOpenAIRiskControlState(resp.Header.Get(openAICodexTurnStateHeader))
	snapshot := &OpenAIRiskControlSnapshot{
		Status:      status,
		Suspected:   suspected,
		StateLength: stateLength,
		HTTPStatus:  resp.StatusCode,
		CheckedAt:   time.Now().UTC(),
		Reason:      reason,
	}
	if err := s.accountRepo.UpdateExtra(ctx, account.ID, map[string]any{
		OpenAIRiskControlExtraKey: snapshot,
	}); err != nil {
		return nil, fmt.Errorf("persist OpenAI risk control result: %w", err)
	}
	return snapshot, nil
}

var _ OpenAIRiskControlAutoProber = (*OpenAIRiskControlService)(nil)
