package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

const (
	openAICodexTicketExtraKeyPrefix  = "codex_turn_ticket:"
	openAICodexTicketStatePrefix     = "gAAAAA"
	openAICodexTicketDefaultModel    = "gpt-6-astra"
	openAICodexTicketDefaultSolModel = "gpt-5.6-sol"
)

// ErrOpenAICodexTicketUnavailable indicates an opted-in account has no verified
// STATE for its configured model. Other accounts and models are unaffected.
var ErrOpenAICodexTicketUnavailable = errors.New("STATE codex turn-state ticket unavailable")

type openAICodexTicket struct {
	AccountID             int64     `json:"account_id"`
	Model                 string    `json:"model"`
	State                 string    `json:"state"`
	Length                int       `json:"length"`
	CapturedAt            time.Time `json:"captured_at"`
	ExpiresAt             time.Time `json:"expires_at"`
	Attempts              int       `json:"attempts"`
	Verified              bool      `json:"verified"`
	ConfigRevision        string    `json:"config_revision"`
	FixedProxyFingerprint string    `json:"fixed_proxy_fingerprint"`
	CredentialFingerprint string    `json:"credential_fingerprint"`
	IdentityFingerprint   string    `json:"identity_fingerprint"`
	RuntimeFingerprint    string    `json:"runtime_fingerprint"`
	VerifiedAt            time.Time `json:"verified_at"`
	VerifiedModel         string    `json:"verified_model"`
}

func openAICodexTicketKey(accountID int64, model string) string {
	return fmt.Sprintf("%d\x00%s", accountID, strings.TrimSpace(model))
}

func openAICodexTicketExtraKey(model string) string {
	// Keep the legacy private/managed parent prefix so an old instance still
	// redacts and preserves this value during a rolling deployment. Exact-key
	// reads are versioned: neither instance may trust or overwrite the other's
	// verification evidence while its business requests are draining.
	return openAICodexTicketExtraKeyPrefix + "v2:" + strings.TrimSpace(model)
}

func normalizeOpenAICodexTicketModel(model string) string {
	return strings.TrimSpace(model)
}

func extractOpenAICodexTicketModel(body []byte) string {
	return normalizeOpenAICodexTicketModel(gjson.GetBytes(body, "model").String())
}

func (s *OpenAIGatewayService) openAICodexTicketConfig() config.OpenAICodexTicketConfig {
	cfg := config.OpenAICodexTicketConfig{}
	if s != nil && s.cfg != nil {
		cfg = s.cfg.Gateway.OpenAICodexTicket
	}
	if cfg.TTLSeconds <= 0 || cfg.TTLSeconds > 3600 {
		cfg.TTLSeconds = 3600
	}
	if cfg.RefreshBeforeSeconds <= 0 {
		cfg.RefreshBeforeSeconds = 600
	}
	if cfg.RefreshBeforeSeconds >= cfg.TTLSeconds {
		cfg.RefreshBeforeSeconds = cfg.TTLSeconds / 6
	}
	if cfg.HarvestProbeIntervalSeconds < 5 {
		cfg.HarvestProbeIntervalSeconds = 30
	}
	if cfg.HarvestAttemptTimeoutSeconds <= 0 || cfg.HarvestAttemptTimeoutSeconds > 60 {
		cfg.HarvestAttemptTimeoutSeconds = 25
	}
	if cfg.MaxConcurrentHarvests <= 0 || cfg.MaxConcurrentHarvests > 8 {
		cfg.MaxConcurrentHarvests = 2
	}
	return cfg
}

func (s *OpenAIGatewayService) openAICodexTicketGatedModel(model string) bool {
	model = normalizeOpenAICodexTicketModel(model)
	if model == "" || !s.openAICodexTicketEnabled() {
		return false
	}
	for _, item := range []string{openAICodexTicketDefaultModel, openAICodexTicketDefaultSolModel} {
		if normalizeOpenAICodexTicketModel(item) == model {
			return true
		}
	}
	return false
}

// OpenAICodexTicketStatus 是给管理端看的门票摘要，不含 state blob。
type OpenAICodexTicketStatus struct {
	Model            string     `json:"model"`
	Length           int        `json:"length,omitempty"`
	Ready            bool       `json:"ready"`
	RemainingSeconds int64      `json:"remaining_seconds"`
	Blocked          bool       `json:"blocked"`
	ExpiresAt        *time.Time `json:"expires_at,omitempty"`
	ExpiryKind       string     `json:"expiry_kind"`
}

func OpenAICodexTicketStatuses(account *Account, cfg config.OpenAICodexTicketConfig, now time.Time) []OpenAICodexTicketStatus {
	ac := codexAccountTicketConfigOf(account)
	if !cfg.Enabled || !isOpenAICodexTicketAccount(account) || !ac.Enabled {
		return nil
	}
	status := OpenAICodexTicketStatus{Model: ac.Model, ExpiryKind: codexTicketExpiryKind}
	ticket := parseOpenAICodexTicketFromAny(account.ID, ac.Model, account.Extra[openAICodexTicketExtraKey(ac.Model)])
	if ticket.validFor(account, ac, now) {
		status.Ready = true
		status.Length = ticket.Length
		status.RemainingSeconds = int64(ticket.ExpiresAt.Sub(now) / time.Second)
		expiry := ticket.ExpiresAt
		status.ExpiresAt = &expiry
	}
	status.Blocked = !status.Ready
	return []OpenAICodexTicketStatus{status}
}

func (s *OpenAIGatewayService) openAICodexTicketEnabled() bool {
	return s.openAICodexTicketEnabledContext(context.Background())
}

func (s *OpenAIGatewayService) openAICodexTicketEnabledContext(ctx context.Context) bool {
	if s == nil {
		return false
	}
	fallback := s.cfg != nil && s.cfg.Gateway.OpenAICodexTicket.Enabled
	if s.settingService != nil {
		return s.settingService.GetOpenAICodexTicketEnabled(ctx, fallback)
	}
	return fallback
}

func (s *OpenAIGatewayService) openAICodexTicketRuntimeEnabled(ctx context.Context) (bool, error) {
	if s == nil {
		return false, nil
	}
	fallback := s.cfg != nil && s.cfg.Gateway.OpenAICodexTicket.Enabled
	if s.settingService != nil {
		enabled, _, err := s.settingService.GetOpenAICodexTicketRuntime(ctx, fallback)
		return enabled, err
	}
	return fallback, nil
}

func (s *OpenAIGatewayService) openAICodexTicketHarvestProxyURL() string {
	return s.openAICodexTicketHarvestProxyURLContext(context.Background())
}

func (s *OpenAIGatewayService) openAICodexTicketHarvestProxyURLContext(ctx context.Context) string {
	if s.settingService != nil {
		return s.settingService.GetOpenAICodexTicketHarvestProxyURL(ctx)
	}
	return strings.TrimSpace(s.openAICodexTicketConfig().HarvestProxyURL)
}

// Length alone is not proof of the returned model; validFor also enforces the manual plan.
func (t *openAICodexTicket) valid(now time.Time, _ int) bool {
	return t != nil && t.Verified && validCodexTicketState(t.State) && t.Length == len(t.State) &&
		t.AccountID > 0 && t.Model != "" && t.ConfigRevision != "" && t.FixedProxyFingerprint != "" &&
		!t.CapturedAt.IsZero() && !t.CapturedAt.After(now.Add(time.Minute)) && now.Before(t.ExpiresAt) &&
		t.ExpiresAt.After(t.CapturedAt) && t.ExpiresAt.Sub(t.CapturedAt) <= time.Hour
}
func (t *openAICodexTicket) validFor(account *Account, ac codexAccountTicketConfig, now time.Time) bool {
	return account != nil && account.Status == StatusActive && ac.Enabled && !codexTicketAuthenticationBlocked(account, now) && t.valid(now, 0) && t.Length == codexTicketTargetLength(ac.TicketPlan) && t.AccountID == account.ID && t.Model == ac.Model &&
		t.ConfigRevision == ac.Revision && t.FixedProxyFingerprint == codexTicketFixedProxyFingerprint(account) &&
		t.CredentialFingerprint != "" && t.CredentialFingerprint == codexTicketCredentialFingerprint(account) &&
		t.IdentityFingerprint != "" && t.IdentityFingerprint == codexTicketIdentityFingerprint(account) &&
		!t.VerifiedAt.IsZero() && !t.VerifiedAt.After(now.Add(time.Minute)) && upstreamModelsMatchForAudit(t.Model, t.VerifiedModel)
}
func validCodexTicketState(state string) bool {
	if len(state) < 32 || len(state) > 8192 || !strings.HasPrefix(state, openAICodexTicketStatePrefix) {
		return false
	}
	for _, c := range state {
		if (c < 'A' || c > 'Z') && (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '_' && c != '-' && c != '=' {
			return false
		}
	}
	return true
}

func (t *openAICodexTicket) needsRefresh(now time.Time, refreshBefore time.Duration) bool {
	if t == nil || t.ExpiresAt.IsZero() {
		return true
	}
	return !t.ExpiresAt.After(now.Add(refreshBefore))
}

func (s *OpenAIGatewayService) lookupOpenAICodexTicket(account *Account, model string) *openAICodexTicket {
	if s == nil || account == nil || account.ID <= 0 {
		return nil
	}
	ac := codexAccountTicketConfigOf(account)
	if !ac.Enabled || ac.Model != normalizeOpenAICodexTicketModel(model) {
		return nil
	}
	now := time.Now()
	key := openAICodexTicketKey(account.ID, model)
	var memory *openAICodexTicket
	if raw, ok := s.openaiCodexTickets.Load(key); ok {
		memory, _ = raw.(*openAICodexTicket)
	}
	persisted := parseOpenAICodexTicketFromAny(account.ID, model, account.Extra[openAICodexTicketExtraKey(model)])
	if memory != nil && memory.RuntimeFingerprint != s.codexTicketRuntimeFingerprint() {
		memory = nil
	}
	if persisted != nil && persisted.RuntimeFingerprint != s.codexTicketRuntimeFingerprint() {
		persisted = nil
	}
	// A fresh database read must override process cache after another instance
	// revoked or replaced a ticket. Never resurrect memory-only persisted state.
	if s.accountRepo != nil && memory != nil && !receiptForCodexTicket(memory).matches(persisted) {
		memory = nil
	}
	if s.codexTicketRejectedByWatchdog(memory) {
		memory = nil
	}
	if s.codexTicketRejectedByWatchdog(persisted) {
		persisted = nil
	}
	if persisted.validFor(account, ac, now) && (!memory.validFor(account, ac, now) || persisted.CapturedAt.After(memory.CapturedAt)) {
		s.openaiCodexTickets.Store(key, persisted)
		return persisted
	}
	if memory.validFor(account, ac, now) {
		return memory
	}
	s.openaiCodexTickets.Delete(key)
	return nil
}

func parseOpenAICodexTicketFromAny(accountID int64, model string, raw any) *openAICodexTicket {
	if raw == nil {
		return nil
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var ticket openAICodexTicket
	if err := json.Unmarshal(b, &ticket); err != nil {
		return nil
	}
	if ticket.AccountID != accountID || ticket.Model != model {
		return nil
	}
	ticket.State = strings.TrimSpace(ticket.State)
	if ticket.Length == 0 {
		ticket.Length = len(ticket.State)
	}
	if ticket.State == "" {
		return nil
	}
	return &ticket
}

// applyOpenAICodexTicket 在出站请求上覆盖 x-codex-turn-state。
// 请求路径只注入已捕获的有效门票，不现场打票；无票则返回
// ErrOpenAICodexTicketUnavailable。打票由后台 harvester 完成。
func (s *OpenAIGatewayService) applyOpenAICodexTicket(ctx context.Context, account *Account, model string, h http.Header) error {
	_, err := s.applyOpenAICodexTicketWithReceipt(ctx, account, model, h)
	return err
}

func (s *OpenAIGatewayService) applyOpenAICodexTicketWithReceipt(ctx context.Context, account *Account, model string, h http.Header) (*codexTicketReceipt, error) {
	if ctx.Value(codexTicketProbeContextKey{}) == true {
		return nil, nil
	}
	if s == nil || h == nil || !isOpenAICodexTicketAccount(account) {
		return nil, nil
	}
	enabled, settingsErr := s.openAICodexTicketRuntimeEnabled(ctx)
	if settingsErr == nil && !enabled && !codexAccountTicketConfigOf(account).RequireVerified {
		return nil, nil
	}
	liveCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	live, err := s.codexTicketLiveAccount(liveCtx, account)
	if err != nil {
		return nil, ErrOpenAICodexTicketUnavailable
	}
	ac := codexAccountTicketConfigOf(live)
	if !isOpenAICodexTicketAccount(live) || !ac.Enabled || (!ac.RequireVerified && ac.Model != normalizeOpenAICodexTicketModel(model)) {
		return nil, nil
	}
	if settingsErr != nil || (!enabled && ac.RequireVerified) {
		return nil, ErrOpenAICodexTicketUnavailable
	}
	if !codexAccountTicketEligible(live) {
		return nil, ErrOpenAICodexTicketUnavailable
	}
	// A scheduler snapshot with a different business proxy must be reselected.
	if codexTicketFixedProxyFingerprint(account) != codexTicketFixedProxyFingerprint(live) {
		return nil, ErrOpenAICodexTicketUnavailable
	}
	if authorization := strings.TrimSpace(h.Get("Authorization")); strings.HasPrefix(authorization, "Bearer ") && strings.TrimSpace(strings.TrimPrefix(authorization, "Bearer ")) != live.GetOpenAIAccessToken() {
		return nil, ErrOpenAICodexTicketUnavailable
	}
	ticket := s.lookupOpenAICodexTicket(live, ac.Model)
	if ticket.validFor(live, ac, time.Now()) {
		if ac.Model != normalizeOpenAICodexTicketModel(model) {
			return nil, nil // Admission is verified, but a different model must never receive this STATE.
		}
		h.Set(openAICodexTurnStateHeader, ticket.State)
		receipt := receiptForCodexTicket(ticket)
		return &receipt, nil
	}
	return nil, ErrOpenAICodexTicketUnavailable
}

// openAICodexTicketOutboundModel 预测本请求真正出站的模型名，也就是
// applyOpenAICodexTicket 注入时读到的 body.model。
//
// 调度门控与注入必须按同一个模型名判定门票。普通请求下二者同源：Forward 的
// upstreamModel 与本函数都走 resolveOpenAIAccountUpstreamModelForRequest，且
// Forward 会把 body.model 改写成该值后才注入。但 /responses/compact 例外——
// Forward 会把出站模型进一步改写为 compact 映射或 gateway.openai_compact_model
// （默认非空），此时若门控仍按客户端原始模型判定，就会把「实际出站是非门控
// 模型、根本不需要票」的 compact 请求整片误拦成不可调度。
func (s *OpenAIGatewayService) openAICodexTicketOutboundModel(account *Account, requestedModel string, requireCompact bool) string {
	model := strings.TrimSpace(requestedModel)
	if account == nil || model == "" {
		return model
	}
	if !account.IsOpenAI() {
		return canonicalOpenAIAccountSchedulingModel(account, model)
	}
	_, upstreamModel := resolveOpenAIForwardMappedModels(account, model, requireCompact)
	if requireCompact {
		// 与 Forward 同序：compact 兜底模型优先于普通/compact 映射结果。
		if compactModel := strings.TrimSpace(s.resolveOpenAICompactFallbackModel(account, model)); compactModel != "" {
			upstreamModel = compactModel
		}
	}
	if upstreamModel = strings.TrimSpace(upstreamModel); upstreamModel != "" {
		return upstreamModel
	}
	return model
}

// outboundModel 必须是真正会发给上游的模型名（openAICodexTicketOutboundModel），
// 不是客户端原始模型：注入侧读的是出站 body.model，两侧口径必须一致。
func (s *OpenAIGatewayService) openAICodexTicketBlocksAccount(ctx context.Context, account *Account, outboundModel string) bool {
	if s == nil || !isOpenAICodexTicketAccount(account) {
		return false
	}
	ac := codexAccountTicketConfigOf(account)
	if !ac.Enabled || (!ac.RequireVerified && ac.Model != normalizeOpenAICodexTicketModel(outboundModel)) {
		return false
	}
	enabled, err := s.openAICodexTicketRuntimeEnabled(ctx)
	if err != nil {
		return true
	}
	if !enabled {
		return ac.RequireVerified
	}
	// Scheduler caches deliberately exclude raw STATE. Only opted-in target
	// models perform this authoritative read; ordinary accounts keep the fast path.
	live, err := s.codexTicketSelectionAccount(ctx, account)
	if err != nil {
		return true
	}
	liveConfig := codexAccountTicketConfigOf(live)
	if !liveConfig.Enabled || (!liveConfig.RequireVerified && liveConfig.Model != normalizeOpenAICodexTicketModel(outboundModel)) {
		return false
	}
	return !codexAccountTicketEligible(live) || !s.lookupOpenAICodexTicket(live, liveConfig.Model).validFor(live, liveConfig, time.Now())
}

func (s *OpenAIGatewayService) fireOpenAICodexTicketProbe(ctx context.Context, account *Account, token, model, proxyURL string, attemptTimeout time.Duration) (state string, status int, err error) {
	return s.fireCodexAccountTicketProbe(ctx, account, token, model, proxyURL, "", attemptTimeout)
}

func (s *OpenAIGatewayService) fireCodexAccountTicketProbe(ctx context.Context, account *Account, token, model, proxyURL, injectedState string, attemptTimeout time.Duration) (state string, status int, err error) {
	if s.openaiCodexTicketProbe != nil {
		state, status, err := s.openaiCodexTicketProbe(ctx, account, token, model, proxyURL, injectedState, attemptTimeout)
		if evidence, _ := ctx.Value(codexTicketProbeEvidenceKey{}).(*codexTicketProbeEvidence); evidence != nil && err == nil && status == http.StatusOK {
			evidence.model = model // The injected test probe implements the completed-model contract.
		}
		return state, status, err
	}
	attemptCtx, cancel := context.WithTimeout(ctx, attemptTimeout)
	defer cancel()

	req, err := s.buildCodexTicketProbeRequest(attemptCtx, account, token, model, injectedState)
	if err != nil {
		return "", 0, err
	}

	// Synthetic probes must use the dedicated no-reuse transport even when the
	// production account is bound to a plugin. This also avoids reading pluginManager
	// while handlers are still wiring it during gateway construction.
	var resp *http.Response
	if injectedState == "" {
		var client *http.Client
		client, err = newCodexTicketChainedClient(proxyURL, s.openAICodexTicketConfig().HarvestDialProxyURL)
		if err == nil {
			defer client.CloseIdleConnections()
			resp, err = s.doCodexTicketSyntheticProbe(req, proxyURL, account, client)
		}
	} else if s.httpUpstream != nil {
		// Verification uses the exact fixed route and normal transport identity.
		resp, err = s.doOpenAIUpstream(req, proxyURL, account)
	} else {
		err = errors.New("ticket transport unavailable")
	}
	if err != nil {
		return "", 0, err
	}
	if resp == nil {
		return "", 0, errors.New("nil upstream response")
	}
	return s.validateCodexTicketProbeResponse(ctx, account, token, model, injectedState, resp)
}

func (s *OpenAIGatewayService) validateCodexTicketProbeResponse(ctx context.Context, account *Account, token, model, injectedState string, resp *http.Response) (string, int, error) {
	headerStateLength := len(extractOpenAICodexTurnState(resp.Header))
	diagnosticError := func(err error) error {
		return &codexTicketProbeDiagnosticError{cause: err, stateLength: headerStateLength}
	}
	// A completed response with the requested actual model is required.
	defer func() {
		if resp.Body != nil {
			_ = resp.Body.Close()
		}
	}()
	if resp.StatusCode != http.StatusOK {
		if injectedState != "" && s.rateLimitService != nil && (resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusTooManyRequests) {
			// Synthetic probes consume the same credentials and quota as normal
			// traffic. A rejected token must enter shared health maintenance too.
			// Bound error reads and never return/log a raw response or token.
			var body []byte
			if resp.Body != nil {
				body, _ = io.ReadAll(io.LimitReader(resp.Body, 64*1024))
			}
			stateCtx, stateCancel := openAIAccountStateContext(WithOpenAIUpstreamAccessToken(ctx, token))
			s.rateLimitService.HandleUpstreamError(stateCtx, account, resp.StatusCode, resp.Header, body)
			stateCancel()
		}
		return "", resp.StatusCode, diagnosticError(errors.New("upstream request was rejected"))
	}
	evidence, _ := ctx.Value(codexTicketProbeEvidenceKey{}).(*codexTicketProbeEvidence)
	if err := validateCodexTicketCompletedModelEvidence(resp.Body, model, evidence); err != nil {
		var mismatch *codexTicketModelMismatchError
		if injectedState != "" && errors.As(err, &mismatch) {
			// A dynamic candidate is not evidence about the fixed business route.
			// Neither may a late response quarantine newly authorized credentials.
			live, liveErr := s.codexTicketLiveAccount(ctx, account)
			if liveErr == nil && codexTicketFixedProxyFingerprint(live) == codexTicketFixedProxyFingerprint(account) {
				quarantineAccountModelMismatch(WithModelMismatchFixedRouteExpectation(ctx, account), s.accountRepo, account, model, mismatch.actual, resp.Header.Get("x-request-id"))
			}
		}
		var rejected *codexTicketStreamError
		if errors.As(err, &rejected) {
			if injectedState != "" && s.rateLimitService != nil && (rejected.status == http.StatusUnauthorized || rejected.status == http.StatusTooManyRequests) {
				stateCtx, stateCancel := openAIAccountStateContext(WithOpenAIUpstreamAccessToken(ctx, token))
				s.rateLimitService.HandleUpstreamError(stateCtx, account, rejected.status, resp.Header, rejected.payload)
				stateCancel()
			}
			return "", rejected.status, diagnosticError(err)
		}
		return "", resp.StatusCode, diagnosticError(err)
	}
	return extractOpenAICodexTurnState(resp.Header), resp.StatusCode, nil
}

func (s *OpenAIGatewayService) doCodexTicketSyntheticProbe(req *http.Request, _ string, account *Account, client *http.Client) (*http.Response, error) {
	req = WithAccountTrafficRequest(req, account)
	return accountTrafficController(s.httpUpstream).DoHTTP(req, client.Do)
}

func (s *OpenAIGatewayService) StartOpenAICodexTicketHarvester() {
	if s == nil {
		return
	}
	s.openaiCodexTicketLifecycleMu.Lock()
	defer s.openaiCodexTicketLifecycleMu.Unlock()
	if s.openaiCodexTicketStopped || s.openaiCodexTicketDone != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	s.openaiCodexTicketCancel = cancel
	s.openaiCodexTicketContext = ctx
	s.openaiCodexTicketDone = done
	go func() {
		defer close(done)
		s.openAICodexTicketHarvestLoop(ctx)
	}()
	logger.L().Info("openai_codex_ticket harvester started",
		zap.Int("ttl_seconds", s.openAICodexTicketConfig().TTLSeconds),
		zap.String("scope", "account_opt_in"),
		zap.Bool("completed_model_and_fixed_proxy_validation", true),
	)
}

func (s *OpenAIGatewayService) StopOpenAICodexTicketHarvester() {
	if s == nil {
		return
	}
	s.openaiCodexTicketLifecycleMu.Lock()
	s.openaiCodexTicketStopped = true
	cancel, done := s.openaiCodexTicketCancel, s.openaiCodexTicketDone
	s.openaiCodexTicketLifecycleMu.Unlock()
	if cancel != nil {
		cancel()
	}
	s.openaiCodexAccountMu.Lock()
	s.openaiCodexAccountStopping = true
	s.cancelCodexTicketJobsLocked()
	s.openaiCodexAccountMu.Unlock()
	if done != nil {
		<-done
	}
	s.openaiCodexAccountWG.Wait()
}

func (s *OpenAIGatewayService) openAICodexTicketHarvestLoop(ctx context.Context) {
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			s.refreshOpenAICodexTickets(ctx)
			timer.Reset(time.Duration(s.openAICodexTicketConfig().HarvestProbeIntervalSeconds) * time.Second)
		}
	}
}

// The ticker only launches bounded jobs. A failed job cools down for five minutes.
func (s *OpenAIGatewayService) refreshOpenAICodexTickets(ctx context.Context) {
	if s == nil || s.accountRepo == nil || ctx.Err() != nil {
		return
	}
	s.openaiCodexWatchdogRevoked.Range(func(key, value any) bool {
		if expiry, ok := value.(time.Time); ok && time.Now().After(expiry) {
			s.openaiCodexWatchdogRevoked.CompareAndDelete(key, value)
			s.openaiCodexWatchdogPending.Delete(key)
			s.openaiCodexWatchdogRetry.Delete(key)
		}
		return true
	})
	s.retryCodexTicketInvalidations(ctx)
	if !s.openAICodexTicketEnabledContext(ctx) {
		s.cancelCodexTicketJobs()
		return
	}
	accounts, err := s.accountRepo.ListByPlatform(ctx, PlatformOpenAI)
	if err != nil {
		return
	}
	for i := range accounts {
		account := &accounts[i]
		ac := codexAccountTicketConfigOf(account)
		if !codexAccountTicketEligible(account) || !ac.Enabled {
			continue
		}
		ticket := s.lookupOpenAICodexTicket(account, ac.Model)
		if ticket.validFor(account, ac, time.Now()) && !ticket.needsRefresh(time.Now(), time.Duration(s.openAICodexTicketConfig().RefreshBeforeSeconds)*time.Second) {
			continue
		}
		s.startCodexAccountTicketJob(ctx, account.ID, false)
	}
}

// Compatibility helper for tests and internal callers: one bounded, opted-in account job.
func (s *OpenAIGatewayService) probeOnceOpenAICodexTicket(ctx context.Context, account *Account, model string) {
	if account == nil || codexAccountTicketConfigOf(account).Model != model {
		return
	}
	job := s.startCodexAccountTicketJob(ctx, account.ID, false)
	if job != nil {
		select {
		case <-job.done:
		case <-ctx.Done():
		}
	}
}

// IsOpenAICodexTicketExtraKey identifies server-managed ticket material.
func IsOpenAICodexTicketExtraKey(key string) bool {
	return key == codexTicketWatchdogExtraKey || key == codexAccountTicketConfigKey || strings.HasPrefix(key, openAICodexTicketExtraKeyPrefix)
}

// MergeOpenAICodexTicketExtra preserves only persisted tickets, never summaries or
// blobs supplied by an account edit. The repository repeats this under the row
// lock so a concurrent harvest cannot be overwritten by a stale admin snapshot.
func MergeOpenAICodexTicketExtra(extra, current map[string]any) map[string]any {
	result := maps.Clone(extra)
	for key := range result {
		if IsOpenAICodexTicketPrivateExtraKey(key) {
			delete(result, key)
		}
	}
	for key, value := range current {
		if IsOpenAICodexTicketPrivateExtraKey(key) {
			if result == nil {
				result = make(map[string]any)
			}
			result[key] = value
		}
	}
	return result
}

// ValidateOpenAICodexTicketHarvestProxyURL validates only syntax, without making
// a network request or including credentials in validation errors.
func ValidateOpenAICodexTicketHarvestProxyURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parsed, err := url.Parse(strings.ReplaceAll(raw, "{sid}", "%7Bsid%7D"))
	if err != nil || parsed.Hostname() == "" || parsed.Opaque != "" || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return errors.New("harvest proxy must be an HTTP(S) or SOCKS5(h) URL with a host and no path, query or fragment")
	}
	switch parsed.Scheme {
	case "http", "https", "socks5", "socks5h":
	default:
		return errors.New("harvest proxy scheme must be http, https, socks5 or socks5h")
	}
	if port := parsed.Port(); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return errors.New("harvest proxy port must be between 1 and 65535")
		}
	}
	return nil
}

// MaskProxyURL never returns a stored proxy password, even for invalid legacy data.
func MaskProxyURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || ValidateOpenAICodexTicketHarvestProxyURL(raw) != nil {
		return ""
	}
	parsed, _ := url.Parse(strings.ReplaceAll(raw, "{sid}", "%7Bsid%7D"))
	// Proxy usernames frequently contain customer IDs or access tokens too.
	parsed.User = nil
	return parsed.String()
}

// IsMaskedProxyURL recognizes the exact password placeholder emitted by the API.
func IsMaskedProxyURL(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return true
	}
	parsed, err := url.Parse(strings.ReplaceAll(raw, "{sid}", "%7Bsid%7D"))
	if err != nil || parsed.User == nil {
		return false
	}
	password, ok := parsed.User.Password()
	return ok && password == "***"
}

// Credential shadows do not own tickets. Keep their existing forwarding policy
// instead of imposing a gate for a key the harvester never populates.
func isOpenAICodexTicketAccount(account *Account) bool {
	return account != nil && account.IsOpenAIOAuthLike() && !account.IsShadow() && !account.IsOpenAIAgentIdentity()
}

// IsOpenAICodexTicketPrivateExtraKey also covers the retired account-level proxy
// override, whose credentials may remain in older account records.
func IsOpenAICodexTicketPrivateExtraKey(key string) bool {
	return IsOpenAICodexTicketExtraKey(key) || key == "codex_harvest_proxy_url"
}

// RedactOpenAICodexTicketExtra strips ephemeral ticket material from exports
// without changing the source account or unrelated backup fields.
func RedactOpenAICodexTicketExtra(extra map[string]any) map[string]any {
	redacted := maps.Clone(extra)
	for key := range redacted {
		if IsOpenAICodexTicketPrivateExtraKey(key) {
			delete(redacted, key)
		}
	}
	return redacted
}
