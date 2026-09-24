package service

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	apperrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/google/uuid"
	"github.com/tidwall/gjson"
)

const codexAccountTicketConfigKey = "codex_ticket_config"
const codexTicketMaxAttempts = 8
const codexTicketRetryCooldown = 5 * time.Minute

const (
	codexTicketPlanPro  = "pro"
	codexTicketPlanTeam = "team"
)

// This is a manual account setting, never inferred from subscription metadata.
func codexTicketTargetLength(plan string) int {
	switch plan {
	case "", codexTicketPlanPro:
		return 292
	case codexTicketPlanTeam:
		return 332
	default:
		return 0
	}
}

// This key is server managed and is never accepted through general account edits.
type codexAccountTicketConfig struct {
	RequireVerified bool   `json:"require_verified,omitempty"` // New-account automatic defaults gate every model until current verification succeeds.
	TicketPlan      string `json:"ticket_plan"`
	Enabled         bool   `json:"enabled"`
	Model           string `json:"model"`
	ProxyURL        string `json:"proxy_url,omitempty"` // Legacy data only; harvesting always uses the global pool.
	Revision        string `json:"revision"`
}

type CodexAccountTicketUpdate struct {
	TicketPlan string `json:"ticket_plan"`
	Enabled    bool   `json:"enabled"`
	ProxyURL   string `json:"proxy_url"`
	Model      string `json:"model"`
	ClearProxy bool   `json:"clear_proxy"`
}

type CodexAccountTicketStatus struct {
	Watchdog              CodexTicketWatchdogStatus `json:"watchdog"`
	TicketPlan            string                    `json:"ticket_plan"`
	TargetLength          int                       `json:"target_length"`
	ActualLength          int                       `json:"actual_length,omitempty"`
	Enabled               bool                      `json:"enabled"`
	RequireVerified       bool                      `json:"require_verified,omitempty"`
	GlobalEnabled         bool                      `json:"global_enabled"`
	Model                 string                    `json:"model"`
	ProxyConfigured       bool                      `json:"proxy_configured"`
	ProxyDisplay          string                    `json:"proxy_display"`
	FixedProxyConfigured  bool                      `json:"fixed_proxy_configured"`
	State                 string                    `json:"state"`
	TicketUsable          bool                      `json:"ticket_usable"`
	Refreshing            bool                      `json:"refreshing"`
	CapturedAt            *time.Time                `json:"captured_at,omitempty"`
	RetryAfter            *time.Time                `json:"retry_after,omitempty"`
	RemainingSeconds      int64                     `json:"remaining_seconds"`
	ExpiresAt             *time.Time                `json:"expires_at,omitempty"`
	LastError             string                    `json:"last_error"`
	Attempts              int                       `json:"attempts"`
	VerifiedAt            *time.Time                `json:"verified_at,omitempty"`
	VerifiedModel         string                    `json:"verified_model,omitempty"`
	VerificationScope     string                    `json:"verification_scope"`
	ExpiryKind            string                    `json:"expiry_kind"`
	CredentialCurrent     *bool                     `json:"credential_current,omitempty"`
	IdentityCurrent       *bool                     `json:"identity_current,omitempty"`
	AuthenticationBlocked bool                      `json:"authentication_blocked"`
}

type codexAccountTicketJob struct {
	coordinator      CodexTicketHarvestCoordinator
	source           string
	owner            string
	remoteRunning    bool
	remoteUntil      time.Time
	revision         string
	fixedFingerprint string
	harvestProxyURL  string // Immutable global pool snapshot for this job; never returned to clients.
	cancel           context.CancelFunc
	done             chan struct{}
	running          bool
	attempts         int
	lastError        string
	retryAfter       time.Time
}

func codexAccountTicketConfigOf(account *Account) codexAccountTicketConfig {
	out := codexAccountTicketConfig{Model: openAICodexTicketDefaultModel, TicketPlan: codexTicketPlanPro}
	if account == nil || account.Extra == nil {
		return out
	}
	raw, err := json.Marshal(account.Extra[codexAccountTicketConfigKey])
	if err != nil {
		return out
	}
	if err = json.Unmarshal(raw, &out); err != nil {
		return codexAccountTicketConfig{Model: openAICodexTicketDefaultModel, TicketPlan: codexTicketPlanPro}
	}
	if out.Model == "" {
		out.Model = openAICodexTicketDefaultModel
	}
	if out.Model != openAICodexTicketDefaultModel && out.Model != openAICodexTicketDefaultSolModel {
		out.Enabled = false
	}
	if out.TicketPlan == "" {
		out.TicketPlan = codexTicketPlanPro
	}
	if codexTicketTargetLength(out.TicketPlan) == 0 {
		out.Enabled = false
	}
	// An incomplete or imported legacy blob must never opt an account in.
	if out.Revision == "" {
		out.Enabled = false
	}
	return out
}

func codexAccountTicketEligible(account *Account) bool {
	return isOpenAICodexTicketAccount(account) && account.Status == StatusActive && codexAccountTicketFixedRouteUsable(account)
}

// Manual scheduling pauses/model audits may still acquire a recovery ticket.
// The global 429 switch decides whether quota observations also stop probes.
func codexTicketHarvestRetryAfter(ctx context.Context, account *Account) *time.Time {
	if account == nil {
		return nil
	}
	now := time.Now()
	var until time.Time
	candidates := []*time.Time{account.OverloadUntil}
	if Context429Enforcement(ctx) {
		candidates = append(candidates, account.RateLimitResetAt)
	}
	for _, candidate := range candidates {
		if candidate != nil && candidate.After(now) && candidate.After(until) {
			until = *candidate
		}
	}
	if candidate := account.TempUnschedulableUntil; candidate != nil && IsOAuthRefreshCooldown(account.TempUnschedulableReason) && candidate.After(now) && candidate.After(until) {
		until = *candidate
	}
	if until.IsZero() {
		return nil
	}
	return &until
}

func codexAccountTicketFixedRouteUsable(account *Account) bool {
	return account != nil && !account.IsRandomProxy() && account.Proxy != nil && account.ProxyID != nil && account.Proxy.IsActive() && !account.Proxy.IsExpired(time.Now())
}

func codexTicketFixedProxyFingerprint(account *Account) string {
	if account == nil || account.Proxy == nil || account.ProxyID == nil {
		return ""
	}
	// The route identity also includes the authorization and protection
	// generation. A new authorization/configuration must start a new job, not
	// inherit an old worker's success or cooldown merely because its IP is equal.
	raw := fmt.Sprintf("v2\x00%d\x00%d\x00%s\x00%s\x00%s", account.ID, *account.ProxyID, account.Proxy.URL(), codexTicketCredentialFingerprint(account), codexTicketIdentityFingerprint(account))
	digest := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(digest[:])
}

func (s *OpenAIGatewayService) codexTicketLiveAccount(ctx context.Context, account *Account) (*Account, error) {
	if account == nil {
		return nil, errors.New("account unavailable")
	}
	if s.accountRepo == nil {
		return account, nil
	}
	// Repository lookup prevents stale scheduler snapshots from re-enabling disabled tickets.
	live, err := s.accountRepo.GetByID(ctx, account.ID)
	if err != nil || live == nil {
		return nil, errors.New("account unavailable")
	}
	return live, nil
}
func (s *OpenAIGatewayService) codexTicketAccountByID(ctx context.Context, id int64) (*Account, error) {
	if s == nil || s.accountRepo == nil {
		return nil, apperrors.New(503, "CODEX_TICKET_UNAVAILABLE", "Ticket service is unavailable")
	}
	account, err := s.accountRepo.GetByID(ctx, id)
	if err != nil || account == nil {
		return nil, apperrors.New(404, "ACCOUNT_NOT_FOUND", "Account not found")
	}
	if !isOpenAICodexTicketAccount(account) {
		return nil, apperrors.BadRequest("CODEX_TICKET_ACCOUNT", "STATE tickets require a non-shadow OpenAI OAuth account")
	}
	return account, nil
}

func (s *OpenAIGatewayService) GetCodexAccountTicketStatus(ctx context.Context, id int64) (*CodexAccountTicketStatus, error) {
	account, err := s.codexTicketAccountByID(ctx, id)
	if err != nil {
		return nil, err
	}
	s.syncCodexTicketCoordinationStatus(ctx, account)
	return s.CodexAccountTicketStatusFromAccount(ctx, account), nil
}

// CodexAccountTicketStatusFromAccount renders an already loaded account without
// a per-row repository lookup. It exposes metadata only, never the STATE value.
func (s *OpenAIGatewayService) CodexAccountTicketStatusFromAccount(ctx context.Context, account *Account) *CodexAccountTicketStatus {
	if s == nil || !isOpenAICodexTicketAccount(account) {
		return nil
	}
	id := account.ID
	ac := codexAccountTicketConfigOf(account)
	pool := s.openAICodexTicketHarvestProxyURLContext(ctx)
	poolConfigured := pool != "" && ValidateOpenAICodexTicketHarvestProxyURL(pool) == nil
	status := &CodexAccountTicketStatus{TicketPlan: ac.TicketPlan, TargetLength: codexTicketTargetLength(ac.TicketPlan), Enabled: ac.Enabled, GlobalEnabled: s.openAICodexTicketEnabledContext(ctx), Model: ac.Model, ProxyConfigured: poolConfigured, FixedProxyConfigured: account.Proxy != nil && account.ProxyID != nil, State: "waiting"}
	status.RequireVerified = ac.Enabled && ac.RequireVerified
	status.VerificationScope = codexTicketVerificationScope
	status.ExpiryKind = codexTicketExpiryKind
	status.AuthenticationBlocked = codexTicketAuthenticationBlocked(account, time.Now())
	if previous := parseOpenAICodexTicketFromAny(id, ac.Model, account.Extra[openAICodexTicketExtraKey(ac.Model)]); previous != nil {
		credentialCurrent := previous.CredentialFingerprint != "" && previous.CredentialFingerprint == codexTicketCredentialFingerprint(account)
		identityCurrent := previous.IdentityFingerprint != "" && previous.IdentityFingerprint == codexTicketIdentityFingerprint(account) && previous.RuntimeFingerprint == s.codexTicketRuntimeFingerprint()
		status.CredentialCurrent = &credentialCurrent
		status.IdentityCurrent = &identityCurrent
	}
	status.Watchdog = codexTicketWatchdogStatusOf(account, ac.Enabled && status.GlobalEnabled)
	if parsed, err := url.Parse(strings.ReplaceAll(pool, "{sid}", "%7Bsid%7D")); err == nil {
		status.ProxyDisplay = parsed.Host
	}
	if !ac.Enabled {
		status.State = "disabled"
		status.CredentialCurrent, status.IdentityCurrent = nil, nil
		return status
	}
	if !status.GlobalEnabled {
		status.State = "global_disabled"
		status.CredentialCurrent, status.IdentityCurrent = nil, nil
		return status
	}
	if ticket := s.lookupOpenAICodexTicket(account, ac.Model); ticket.validFor(account, ac, time.Now()) {
		status.State = "ready"
		status.TicketUsable = true
		status.ActualLength = ticket.Length
		captured := ticket.CapturedAt
		status.CapturedAt = &captured
		status.RemainingSeconds = int64(time.Until(ticket.ExpiresAt) / time.Second)
		expiry := ticket.ExpiresAt
		status.ExpiresAt = &expiry
		verified := ticket.VerifiedAt
		status.VerifiedAt = &verified
		status.VerifiedModel = ticket.VerifiedModel
	}
	s.openaiCodexAccountMu.Lock()
	if job := s.openaiCodexAccountJobs[id]; job != nil && job.revision == ac.Revision && job.harvestProxyURL == pool && job.fixedFingerprint == codexTicketFixedProxyFingerprint(account) {
		status.Attempts = job.attempts
		status.LastError = job.lastError
		if job.running || (job.remoteRunning && time.Now().Before(job.remoteUntil)) {
			status.State = "harvesting"
			status.Refreshing = status.TicketUsable
		} else if job.lastError != "" && status.State != "ready" {
			status.State = "error"
		}
		if !job.running && (!codexTicket429Observation(job.lastError) || Context429Enforcement(ctx)) && time.Now().Before(job.retryAfter) {
			retryAfter := job.retryAfter
			status.RetryAfter = &retryAfter
		}
	}
	s.openaiCodexAccountMu.Unlock()
	if !poolConfigured && !status.TicketUsable {
		status.State = "error"
		status.LastError = "Configure the global dynamic proxy pool in gateway settings"
	}
	if retryAfter := codexTicketHarvestRetryAfter(ctx, account); retryAfter != nil && (!status.TicketUsable || status.AuthenticationBlocked) {
		status.State = "error"
		status.LastError = "STATE acquisition is paused by the account authentication, quota or overload cooldown"
		if status.RetryAfter == nil || retryAfter.After(*status.RetryAfter) {
			status.RetryAfter = retryAfter
		}
	}
	if !codexAccountTicketEligible(account) {
		status.State = "error"
		status.TicketUsable = false
		status.ActualLength = 0
		status.Refreshing = false
		status.CapturedAt = nil
		status.ExpiresAt = nil
		status.RemainingSeconds = 0
		status.LastError = "Account must be active and have a fixed business proxy"
	}
	if status.AuthenticationBlocked {
		status.State = "authentication_blocked"
		status.TicketUsable = false
		status.Refreshing = false
		status.LastError = "Account authentication is unavailable; a cached STATE does not restore authorization"
	}
	return status
}

func (s *OpenAIGatewayService) ConfigureCodexAccountTicket(ctx context.Context, id int64, input CodexAccountTicketUpdate) (*CodexAccountTicketStatus, error) {
	return s.configureCodexAccountTicket(ctx, id, input, true)
}

func (s *OpenAIGatewayService) configureCodexAccountTicket(ctx context.Context, id int64, input CodexAccountTicketUpdate, startHarvest bool) (*CodexAccountTicketStatus, error) {
	if _, err := s.codexTicketAccountByID(ctx, id); err != nil {
		return nil, err
	}
	if input.ClearProxy || strings.TrimSpace(input.ProxyURL) != "" {
		return nil, apperrors.BadRequest("CODEX_TICKET_GLOBAL_PROXY", "Configure the dynamic proxy pool in gateway settings, not per account")
	}
	pool := s.openAICodexTicketHarvestProxyURLContext(ctx)
	var changed, enabled bool
	err := s.mutateCodexTicket(ctx, id, func(account *Account) (map[string]any, error) {
		if !isOpenAICodexTicketAccount(account) {
			return nil, errCodexTicketSourceChanged
		}
		old := codexAccountTicketConfigOf(account)
		next := old
		next.Enabled = input.Enabled
		if !input.Enabled {
			next.RequireVerified = false // Explicit account-level opt-out also revokes the automatic admission requirement.
		}
		if input.TicketPlan != "" {
			next.TicketPlan = strings.ToLower(strings.TrimSpace(input.TicketPlan))
		}
		if next.TicketPlan != codexTicketPlanPro && next.TicketPlan != codexTicketPlanTeam {
			return nil, apperrors.BadRequest("CODEX_TICKET_PLAN", "Ticket plan must be pro (292) or team (332)")
		}
		if input.Model != "" {
			next.Model = strings.TrimSpace(input.Model)
		}
		if next.Model != openAICodexTicketDefaultModel && next.Model != openAICodexTicketDefaultSolModel {
			return nil, apperrors.BadRequest("CODEX_TICKET_MODEL", "Ticket model must be gpt-6-astra or gpt-5.6-sol")
		}
		if next.Enabled && (pool == "" || ValidateOpenAICodexTicketHarvestProxyURL(pool) != nil || !codexAccountTicketFixedRouteUsable(account)) {
			return nil, apperrors.BadRequest("CODEX_TICKET_PROXY_REQUIRED", "Configure the global dynamic proxy pool and an active fixed business proxy; random business proxies are unsupported")
		}
		next.ProxyURL = ""
		enabled = next.Enabled
		changed = next.TicketPlan != old.TicketPlan || next.Enabled != old.Enabled || next.RequireVerified != old.RequireVerified || next.Model != old.Model || old.Revision == ""
		updates := map[string]any{}
		if changed {
			next.Revision = uuid.NewString()
			updates[codexAccountTicketConfigKey] = next
			updates[codexTicketWatchdogExtraKey] = nil
			for key := range account.Extra {
				if strings.HasPrefix(key, openAICodexTicketExtraKeyPrefix) {
					updates[key] = nil
				}
			}
		} else if old.ProxyURL != "" {
			updates[codexAccountTicketConfigKey] = next
		}
		return updates, nil
	})
	if err != nil {
		return nil, err
	}
	if changed {
		s.openaiCodexAccountMu.Lock()
		if job := s.openaiCodexAccountJobs[id]; job != nil {
			if job.cancel != nil {
				job.cancel()
			}
			delete(s.openaiCodexAccountJobs, id)
		}
		s.openaiCodexAccountMu.Unlock()
		s.openaiCodexTickets.Range(func(key, value any) bool {
			if ticket, ok := value.(*openAICodexTicket); ok && ticket.AccountID == id {
				s.openaiCodexTickets.Delete(key)
			}
			return true
		})
		s.InvalidateAgentIdentityWSConnections(id)
	}
	if startHarvest && enabled && changed && s.openAICodexTicketEnabledContext(ctx) {
		s.startCodexAccountTicketJob(ctx, id, false)
	}
	return s.GetCodexAccountTicketStatus(ctx, id)
}
func (s *OpenAIGatewayService) HarvestCodexAccountTicket(ctx context.Context, id int64) (*CodexAccountTicketStatus, error) {
	account, err := s.codexTicketAccountByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !s.openAICodexTicketEnabledContext(ctx) {
		return nil, apperrors.BadRequest("CODEX_TICKET_GLOBAL_DISABLED", "Enable the gateway STATE master switch first")
	}
	if !codexAccountTicketConfigOf(account).Enabled {
		return nil, apperrors.BadRequest("CODEX_TICKET_DISABLED", "Enable STATE tickets for this account first")
	}
	if !codexAccountTicketEligible(account) {
		return nil, apperrors.BadRequest("CODEX_TICKET_ACCOUNT_INACTIVE", "Account must be active and have a fixed business proxy")
	}
	if codexTicketHarvestRetryAfter(ctx, account) != nil {
		return nil, apperrors.BadRequest("CODEX_TICKET_COOLDOWN", "Wait for the account authentication, quota or overload cooldown before acquiring STATE")
	}
	pool := s.openAICodexTicketHarvestProxyURLContext(ctx)
	if pool == "" || ValidateOpenAICodexTicketHarvestProxyURL(pool) != nil {
		return nil, apperrors.BadRequest("CODEX_TICKET_GLOBAL_PROXY", "Configure the global dynamic proxy pool in gateway settings")
	}
	s.startCodexAccountTicketJob(context.Background(), id, true)
	return s.GetCodexAccountTicketStatus(ctx, id)
}

func (s *OpenAIGatewayService) cancelCodexTicketJobsLocked() {
	for _, job := range s.openaiCodexAccountJobs {
		if job.cancel != nil {
			job.cancel()
		}
	}
}
func (s *OpenAIGatewayService) cancelCodexTicketJobs() {
	s.openaiCodexAccountMu.Lock()
	defer s.openaiCodexAccountMu.Unlock()
	s.cancelCodexTicketJobsLocked()
}

func (s *OpenAIGatewayService) startCodexAccountTicketJob(ctx context.Context, id int64, _ bool) *codexAccountTicketJob {
	if s == nil || ctx.Err() != nil || !s.openAICodexTicketEnabledContext(ctx) {
		return nil
	}
	account, err := s.codexTicketAccountByID(ctx, id)
	if err != nil || !codexAccountTicketEligible(account) || codexTicketHarvestRetryAfter(ctx, account) != nil {
		return nil
	}
	ac := codexAccountTicketConfigOf(account)
	pool := s.openAICodexTicketHarvestProxyURLContext(ctx)
	if !ac.Enabled || pool == "" || ValidateOpenAICodexTicketHarvestProxyURL(pool) != nil {
		return nil
	}
	s.openaiCodexTicketLifecycleMu.Lock()
	parentCtx := s.openaiCodexTicketContext
	stopped := s.openaiCodexTicketStopped
	s.openaiCodexTicketLifecycleMu.Unlock()
	if stopped {
		return nil
	}
	if parentCtx == nil {
		parentCtx = context.Background()
	}
	s.openaiCodexAccountMu.Lock()
	defer s.openaiCodexAccountMu.Unlock()
	if s.openaiCodexAccountStopping || parentCtx.Err() != nil {
		return nil
	}
	if s.openaiCodexAccountJobs == nil {
		s.openaiCodexAccountJobs = make(map[int64]*codexAccountTicketJob)
	}
	fingerprint := codexTicketFixedProxyFingerprint(account)
	if job := s.openaiCodexAccountJobs[id]; job != nil {
		sameSource := job.revision == ac.Revision && job.fixedFingerprint == fingerprint && job.harvestProxyURL == pool
		if job.running && sameSource {
			return job
		}
		if job.running && job.cancel != nil {
			job.cancel()
		}
		// 429 remains visible but never creates or revives a local cooldown.
		if sameSource && (!codexTicket429Observation(job.lastError) || Context429Enforcement(ctx)) && time.Now().Before(job.retryAfter) {
			return nil
		}
	}
	if s.openaiCodexTicketSlots == nil {
		s.openaiCodexTicketSlots = make(chan struct{}, s.openAICodexTicketConfig().MaxConcurrentHarvests)
	}
	select {
	case s.openaiCodexTicketSlots <- struct{}{}:
	default:
		return nil
	}
	jobCtx, cancel := context.WithCancel(parentCtx)
	job := &codexAccountTicketJob{revision: ac.Revision, fixedFingerprint: fingerprint, harvestProxyURL: pool, cancel: cancel, done: make(chan struct{}), running: true, source: codexTicketHarvestSource(account, pool), owner: uuid.NewString()}
	s.openaiCodexAccountJobs[id] = job
	s.openaiCodexAccountWG.Add(1)
	go func() {
		defer cancel()
		defer s.openaiCodexAccountWG.Done()
		defer close(job.done)
		defer func() { <-s.openaiCodexTicketSlots }()
		s.runCoordinatedCodexTicketJob(jobCtx, cancel, id, job)
	}()
	return job
}
func (s *OpenAIGatewayService) runCodexAccountTicketJob(ctx context.Context, id int64, job *codexAccountTicketJob) {
	lastError := "Unable to obtain a verified STATE ticket"
	previousFixedFailure := ""
	defer func() {
		s.openaiCodexAccountMu.Lock()
		defer s.openaiCodexAccountMu.Unlock()
		job.running = false
		job.retryAfter = time.Time{}
		if ctx.Err() == nil && lastError != "" && (!codexTicket429Observation(lastError) || Context429Enforcement(ctx)) {
			job.retryAfter = time.Now().Add(codexTicketRetryCooldown)
		}
		if ctx.Err() != nil {
			job.lastError = ""
		} else {
			job.lastError = lastError
		}
	}()
	timeout := time.Duration(s.openAICodexTicketConfig().HarvestAttemptTimeoutSeconds) * time.Second
	for attempt := 1; attempt <= codexTicketMaxAttempts; attempt++ {
		if ctx.Err() != nil || !s.openAICodexTicketEnabledContext(ctx) {
			lastError = ""
			return
		}
		account, err := s.codexTicketAccountByID(ctx, id)
		if err != nil {
			return
		}
		ac := codexAccountTicketConfigOf(account)
		if !codexAccountTicketEligible(account) || codexTicketHarvestRetryAfter(ctx, account) != nil || !ac.Enabled || ac.Revision != job.revision || codexTicketFixedProxyFingerprint(account) != job.fixedFingerprint || s.openAICodexTicketHarvestProxyURLContext(ctx) != job.harvestProxyURL {
			lastError = ""
			return
		}
		// Token helpers are permitted to update metadata, but not shared account maps.
		account.Extra = maps.Clone(account.Extra)
		account.Credentials = maps.Clone(account.Credentials)
		s.openaiCodexAccountMu.Lock()
		job.attempts = attempt
		s.openaiCodexAccountMu.Unlock()
		token, _, err := s.GetAccessToken(ctx, account)
		if err != nil || token == "" {
			lastError = "Account authentication failed"
			return
		}
		// Token providers may refresh without mutating the caller's account.
		// Re-read after token resolution, and reject cached/late generations.
		live, err := s.codexTicketAccountByID(ctx, id)
		if err != nil || live.GetOpenAIAccessToken() != token || codexTicketFixedProxyFingerprint(live) != job.fixedFingerprint || codexTicketAuthenticationBlocked(live, time.Now()) {
			lastError = ""
			return
		}
		account = live
		runtimeFingerprint := s.codexTicketRuntimeFingerprint()
		// A configuration/credential change invalidates its proof, not necessarily
		// the opaque STATE itself. Try that persisted candidate once on the CURRENT
		// fixed business route before depending on the dynamic collection pool.
		if attempt == 1 {
			if candidate := s.codexTicketUnverifiedCandidate(account, ac, time.Now()); candidate != nil {
				candidateCtx := context.WithValue(ctx, codexTicketCandidateKey{}, candidate)
				finished, reason := s.verifyAndPublishCodexTicket(candidateCtx, account, token, candidate.State, runtimeFingerprint, candidate.ExpiresAt, timeout, attempt, job)
				if finished {
					lastError = reason
					return
				}
				previousFixedFailure = reason
				// A failed fixed probe may overlap reauthorization or an auth
				// failure from another request. Do not send the old token again
				// through the dynamic fallback after that generation is obsolete.
				latest, latestErr := s.codexTicketAccountByID(ctx, id)
				if latestErr != nil || !codexAccountTicketEligible(latest) || codexTicketFixedProxyFingerprint(latest) != job.fixedFingerprint || codexTicketAuthenticationBlocked(latest, time.Now()) {
					lastError = ""
					return
				}
			}
		}
		harvestProxy := freshCodexTicketProxyURL(job.harvestProxyURL)
		dynamicEvidence := &codexTicketProbeEvidence{}
		dynamicCtx := context.WithValue(ctx, codexTicketProbeEvidenceKey{}, dynamicEvidence)
		state, status, err := s.fireCodexAccountTicketProbe(dynamicCtx, account, token, ac.Model, harvestProxy, "", timeout)
		if reason := codexTicketProbeRejection(ctx, status); reason != "" {
			lastError = codexTicketFailureWithPrevious(codexTicketProbeFailure("dynamic", status, err, state, codexTicketTargetLength(ac.TicketPlan), dynamicEvidence.model)+"; "+reason, previousFixedFailure)
			return
		}
		if err != nil || status != 200 || !validCodexTicketState(state) {
			lastError = codexTicketFailureWithPrevious(codexTicketProbeFailure("dynamic", status, err, state, codexTicketTargetLength(ac.TicketPlan), dynamicEvidence.model), previousFixedFailure)
		} else if len(state) != codexTicketTargetLength(ac.TicketPlan) {
			lastError = codexTicketFailureWithPrevious(codexTicketProbeFailure("dynamic", status, nil, state, codexTicketTargetLength(ac.TicketPlan), dynamicEvidence.model), previousFixedFailure)
		} else {
			finished, reason := s.verifyAndPublishCodexTicket(ctx, account, token, state, runtimeFingerprint, time.Time{}, timeout, attempt, job)
			if finished {
				lastError = reason
				return
			}
			lastError, previousFixedFailure = reason, reason
		}
		if attempt < codexTicketMaxAttempts {
			timer := time.NewTimer(time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
		}
	}
}

func (s *OpenAIGatewayService) codexTicketUnverifiedCandidate(account *Account, ac codexAccountTicketConfig, now time.Time) *openAICodexTicket {
	candidate := parseOpenAICodexTicketFromAny(account.ID, ac.Model, account.Extra[openAICodexTicketExtraKey(ac.Model)])
	if candidate == nil || !validCodexTicketState(candidate.State) || candidate.Length != len(candidate.State) || candidate.Length != codexTicketTargetLength(ac.TicketPlan) ||
		candidate.CapturedAt.IsZero() || candidate.CapturedAt.After(now) || !candidate.ExpiresAt.After(now) || candidate.ExpiresAt.Sub(candidate.CapturedAt) > time.Hour || s.codexTicketRejectedByWatchdog(candidate) {
		return nil
	}
	// A current ticket outside its renewal window needs no fixed probe. Near
	// expiry, the fixed response may issue a distinct STATE that can itself be
	// verified once; an unchanged STATE never receives an extended deadline.
	refreshBefore := time.Duration(s.openAICodexTicketConfig().RefreshBeforeSeconds) * time.Second
	if candidate.validFor(account, ac, now) && candidate.RuntimeFingerprint == s.codexTicketRuntimeFingerprint() && !candidate.needsRefresh(now, refreshBefore) {
		return nil
	}
	return candidate
}

// Both dynamically collected and persisted untrusted candidates use precisely
// the same fixed-route proof, lease renewal and current-generation publication.
// A false result permits bounded dynamic fallback; authentication rejections
// and generation changes stop the job instead of retrying a known bad account.
func (s *OpenAIGatewayService) verifyAndPublishCodexTicket(ctx context.Context, account *Account, token, state, runtimeFingerprint string, expiresBefore time.Time, timeout time.Duration, attempt int, job *codexAccountTicketJob) (bool, string) {
	if ctx.Err() != nil || !s.openAICodexTicketEnabledContext(ctx) || s.openAICodexTicketHarvestProxyURLContext(ctx) != job.harvestProxyURL {
		return true, ""
	}
	live, readErr := s.codexTicketAccountByID(ctx, account.ID)
	if readErr != nil || !codexAccountTicketEligible(live) || codexTicketFixedProxyFingerprint(live) != job.fixedFingerprint || codexTicketAuthenticationBlocked(live, time.Now()) || codexAccountTicketConfigOf(live).Revision != job.revision {
		return true, ""
	}
	ac := codexAccountTicketConfigOf(account)
	evidence := &codexTicketProbeEvidence{}
	verifyCtx := context.WithValue(ctx, codexTicketProbeEvidenceKey{}, evidence)
	replayState, status, err := s.fireCodexAccountTicketProbe(verifyCtx, account, token, ac.Model, account.Proxy.URL(), state, timeout)
	if reason := codexTicketProbeRejection(ctx, status); reason != "" {
		return true, codexTicketProbeFailure("fixed", status, err, replayState, 0, evidence.model) + "; " + reason
	}
	if err != nil || status != http.StatusOK || (len(replayState) == 312 && validCodexTicketState(replayState)) {
		return false, codexTicketProbeFailure("fixed", status, err, replayState, 0, evidence.model)
	}
	if candidate, _ := ctx.Value(codexTicketCandidateKey{}).(*openAICodexTicket); candidate != nil {
		if replayState != state && validCodexTicketState(replayState) && len(replayState) == codexTicketTargetLength(ac.TicketPlan) {
			// The first response issued a new opaque STATE. It is only a
			// candidate until a second complete request proves that exact value
			// on this same current fixed route. Never recurse on further headers.
			fresh, freshErr := s.codexTicketAccountByID(ctx, account.ID)
			if freshErr != nil || !codexAccountTicketEligible(fresh) || codexTicketFixedProxyFingerprint(fresh) != job.fixedFingerprint || codexTicketAuthenticationBlocked(fresh, time.Now()) || codexAccountTicketConfigOf(fresh).Revision != job.revision {
				return true, ""
			}
			newState := replayState
			evidence = &codexTicketProbeEvidence{}
			verifyCtx = context.WithValue(ctx, codexTicketProbeEvidenceKey{}, evidence)
			replayState, status, err = s.fireCodexAccountTicketProbe(verifyCtx, account, token, ac.Model, account.Proxy.URL(), newState, timeout)
			if reason := codexTicketProbeRejection(ctx, status); reason != "" {
				return true, codexTicketProbeFailure("fixed_replacement", status, err, replayState, 0, evidence.model) + "; " + reason
			}
			if err != nil || status != http.StatusOK || (len(replayState) == 312 && validCodexTicketState(replayState)) {
				return false, codexTicketProbeFailure("fixed_replacement", status, err, replayState, 0, evidence.model)
			}
			state = newState
			expiresBefore = time.Time{}
			ctx = context.WithValue(ctx, codexTicketFreshReplacementKey{}, newState)
		} else if candidate.validFor(account, ac, time.Now()) && candidate.RuntimeFingerprint == runtimeFingerprint {
			// A still-current ticket has nothing to rebind. Keep its old proof
			// and try fresh dynamic collection instead of resetting CapturedAt
			// on every scan while its unchanged deadline approaches.
			return false, "STATE stage=fixed status=200 reason=no_new_state"
		}
	}
	// Establish the deadline before renewing ownership so a suspended worker
	// cannot obtain a fresh publication budget for an expired lease result.
	publishCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := refreshCodexTicketJobLease(publishCtx, account.ID, job); err != nil {
		return true, "STATE harvest lease is no longer owned; result was discarded"
	}
	now := time.Now()
	expires := now.Add(time.Duration(s.openAICodexTicketConfig().TTLSeconds) * time.Second)
	if !expiresBefore.IsZero() && expiresBefore.Before(expires) {
		expires = expiresBefore
	}
	ticket := &openAICodexTicket{AccountID: account.ID, Model: ac.Model, State: state, Length: len(state), CapturedAt: now, ExpiresAt: expires, Attempts: attempt, Verified: true, ConfigRevision: job.revision, FixedProxyFingerprint: job.fixedFingerprint,
		CredentialFingerprint: codexTicketCredentialFingerprint(account), IdentityFingerprint: codexTicketIdentityFingerprint(account), RuntimeFingerprint: runtimeFingerprint, VerifiedAt: now, VerifiedModel: evidence.model}
	if err := s.publishCodexTicket(publishCtx, ticket, job.harvestProxyURL); err != nil {
		return true, "Could not save verified STATE; account configuration or candidate may have changed"
	}
	return true, ""
}

// A rejected probe cannot produce a valid ticket. End this attempt and retain
// any usable ticket; 429 records the result without delaying the next job.
func codexTicket429Observation(reason string) bool {
	if strings.HasPrefix(reason, "STATE stage=") {
		// The current stage decides scheduling; previous diagnostic stages and
		// a model name containing "429" must not change the saved 429 policy.
		fields := strings.Fields(reason)
		return len(fields) >= 3 && fields[2] == "status=429"
	}
	return strings.Contains(reason, "429")
}

var codexTicketSafeModelPattern = regexp.MustCompile(`^[a-zA-Z0-9._-]{1,100}$`)

type codexTicketProbeDiagnosticError struct {
	cause       error
	stateLength int
}

func (e *codexTicketProbeDiagnosticError) Error() string { return "STATE probe rejected response" }
func (e *codexTicketProbeDiagnosticError) Unwrap() error { return e.cause }

// Only finite classifier output crosses into status/Redis. Never append an
// arbitrary error, URL, response body, header, STATE or credential value.
func codexTicketProbeFailure(stage string, status int, err error, state string, targetLength int, completedModels ...string) string {
	if stage != "dynamic" && stage != "fixed" && stage != "fixed_replacement" {
		stage = "unknown"
	}
	if status < 0 || status > 599 {
		status = 0
	}
	reason, actualModel := "verification_failed", ""
	stateLength := len(state)
	var diagnostic *codexTicketProbeDiagnosticError
	if errors.As(err, &diagnostic) {
		stateLength = diagnostic.stateLength
	}
	if len(completedModels) > 0 && codexTicketSafeModelPattern.MatchString(completedModels[0]) && !strings.Contains(completedModels[0], "429") {
		actualModel = " actual_model=" + completedModels[0]
	}
	var mismatch *codexTicketModelMismatchError
	var stream *codexTicketStreamError
	var network net.Error
	switch {
	case errors.As(err, &mismatch):
		reason = "model_mismatch"
		// Older rolling workers/Redis scripts identify 429 observations by
		// substring. Omit this optional field if it could impersonate that code.
		if codexTicketSafeModelPattern.MatchString(mismatch.actual) && !strings.Contains(mismatch.actual, "429") {
			actualModel = " actual_model=" + mismatch.actual
		}
	case errors.Is(err, context.DeadlineExceeded):
		reason = "timeout"
	case errors.Is(err, context.Canceled):
		reason = "cancelled"
	case errors.As(err, &network) && network.Timeout():
		reason = "timeout"
	case errors.As(err, &stream):
		reason = "stream_rejected"
	case status != http.StatusOK && status != 0:
		reason = "http_rejected"
	case err != nil:
		// Equality against parser-owned constants is intentional: unknown
		// upstream error strings are never included in the result.
		parserErr := err
		if diagnostic != nil {
			parserErr = diagnostic.cause
		}
		switch parserErr.Error() {
		case "missing completion", "response did not complete", "incomplete response":
			reason = "response_incomplete"
		case "invalid completion":
			reason = "response_invalid"
		case "completed response has no model":
			reason = "model_missing"
		default:
			reason = "transport_or_verification_error"
		}
	case len(state) == 312 && validCodexTicketState(state):
		reason = "state_312"
	case targetLength > 0 && state == "":
		reason = "state_missing"
	case targetLength > 0 && !validCodexTicketState(state):
		reason = "state_invalid"
	case targetLength > 0 && len(state) != targetLength:
		reason = "state_length_mismatch"
	}
	transportError := err != nil && status == 0 || reason == "timeout" || reason == "cancelled"
	// Preserve compatibility with old workers whose 429 policy scans the
	// string: an unrelated STATE length must never look like an HTTP 429.
	lengthLabel := "other"
	switch stateLength {
	case 0, 292, 312, 332:
		lengthLabel = fmt.Sprint(stateLength)
	}
	return fmt.Sprintf("STATE stage=%s status=%d reason=%s state_length=%s transport_error=%t%s", stage, status, reason, lengthLabel, transportError, actualModel)
}

func codexTicketFailureWithPrevious(current, previous string) string {
	if previous == "" {
		return current
	}
	return current + "; prior_fixed={" + previous + "}"
}

func codexTicketProbeRejection(ctx context.Context, status int) string {
	switch status {
	case http.StatusUnauthorized:
		return "Upstream rejected authentication (HTTP 401); acquisition paused for cooldown"
	case http.StatusForbidden:
		return "Upstream denied access (HTTP 403); acquisition paused for cooldown"
	case http.StatusTooManyRequests:
		if Context429Enforcement(ctx) {
			return "Upstream rate limit (HTTP 429); acquisition paused for cooldown"
		}
		return "Upstream HTTP 429 recorded; account scheduling and next acquisition are not paused"
	default:
		return ""
	}
}

var codexTicketSIDPattern = regexp.MustCompile(`(?i)-sid-[^-]+(-t-[0-9]+)`)

func freshCodexTicketProxyURL(raw string) string {
	parsed, err := url.Parse(strings.ReplaceAll(raw, "{sid}", "%7Bsid%7D"))
	if err != nil || parsed.User == nil {
		return raw
	}
	username := parsed.User.Username()
	sid := strings.ReplaceAll(uuid.NewString(), "-", "")[:20]
	if strings.Contains(username, "{sid}") {
		username = strings.ReplaceAll(username, "{sid}", sid)
	} else if strings.HasSuffix(strings.ToLower(parsed.Hostname()), ".1024proxy.io") || strings.EqualFold(parsed.Hostname(), "1024proxy.io") {
		username = codexTicketSIDPattern.ReplaceAllString(username, "-sid-"+sid+"${1}")
	}
	if password, ok := parsed.User.Password(); ok {
		parsed.User = url.UserPassword(username, password)
	} else {
		parsed.User = url.User(username)
	}
	return parsed.String()
}

type codexTicketModelMismatchError struct{ actual string }

type codexTicketStreamError struct {
	status  int
	payload []byte
}

func (*codexTicketStreamError) Error() string { return "upstream rejected the verification stream" }

func (*codexTicketModelMismatchError) Error() string {
	return "returned model differs from requested model"
}

func validateCodexTicketCompletedModel(body io.Reader, model string) error {
	return validateCodexTicketCompletedModelEvidence(body, model, nil)
}

func validateCodexTicketCompletedModelEvidence(body io.Reader, model string, evidence *codexTicketProbeEvidence) error {
	if body == nil {
		return errors.New("missing completion")
	}
	scanner := bufio.NewScanner(io.LimitReader(body, 2<<20))
	scanner.Buffer(make([]byte, 4096), 1<<20)
	eventType := ""
	var data strings.Builder
	validate := func() (bool, error) {
		raw := strings.TrimSpace(data.String())
		if raw == "" {
			return false, nil
		}
		if !gjson.Valid(raw) {
			return false, errors.New("invalid completion")
		}
		typ := gjson.Get(raw, "type").String()
		if typ == "" {
			typ = eventType
		}
		if typ == "response.failed" || typ == "response.incomplete" || typ == "error" {
			payload := []byte(raw)
			status := openAIStreamFailureStatus(payload, extractOpenAISSEErrorMessage(payload))
			if status == http.StatusUnauthorized || status == http.StatusTooManyRequests {
				return false, &codexTicketStreamError{status: status, payload: payload}
			}
			return false, errors.New("incomplete response")
		}
		if typ != "response.completed" {
			return false, nil
		}
		status := gjson.Get(raw, "response.status").String()
		if status != "" && status != "completed" {
			return false, errors.New("incomplete response")
		}
		actual := strings.TrimSpace(gjson.Get(raw, "response.model").String())
		if actual == "" {
			return false, errors.New("completed response has no model")
		}
		if !upstreamModelsMatchForAudit(model, actual) {
			return false, &codexTicketModelMismatchError{actual: actual}
		}
		if evidence != nil {
			evidence.model = actual
		}
		return true, nil
	}
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			okay, err := validate()
			if err != nil || okay {
				return err
			}
			data.Reset()
			eventType = ""
			continue
		}
		if strings.HasPrefix(line, "event:") {
			eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		}
		if strings.HasPrefix(line, "data:") {
			if data.Len() > 0 {
				_ = data.WriteByte('\n')
			}
			_, _ = data.WriteString(strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	if okay, err := validate(); err != nil || okay {
		return err
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return errors.New("response did not complete")
}
