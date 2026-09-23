//go:build unit

package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestCodexTicketBindingRejectsReauthorizationAndIdentityChanges(t *testing.T) {
	for _, change := range []string{"access", "refresh", "generation", "device", "protection", "ua", "runtime", "legacy_unbound"} {
		t.Run(change, func(t *testing.T) {
			a := stateTicketTestAccount(8201)
			ticket := stateTicketVerified(a, time.Now())
			a.Extra[openAICodexTicketExtraKey(ticket.Model)] = ticket
			s, _ := stateTicketTestService(t, a)
			s.accountRepo = nil
			require.NotNil(t, s.lookupOpenAICodexTicket(a, ticket.Model))
			switch change {
			case "access":
				a.Credentials["access_token"] = "new-access"
			case "refresh":
				a.Credentials["refresh_token"] = "new-refresh"
			case "generation":
				a.Credentials["_token_version"] = 2
			case "device":
				a.Extra["openai_device_id"] = "new-device"
			case "protection":
				PrepareNewAccountProtection(a)
			case "ua":
				a.Credentials["user_agent"] = "codex_cli_rs/0.153.4 (Mac OS 15.0; arm64)"
			case "runtime":
				s.cfg.Gateway.ForceCodexCLI = true
			case "legacy_unbound":
				ticket.CredentialFingerprint = ""
				// Simulate loading a legacy row after restart, without a separately
				// verified, current in-memory copy from the warm-up assertion above.
				s.openaiCodexTickets.Delete(openAICodexTicketKey(a.ID, ticket.Model))
			}
			require.Nil(t, s.lookupOpenAICodexTicket(a, ticket.Model))
			status := s.CodexAccountTicketStatusFromAccount(context.Background(), a)
			require.False(t, status.TicketUsable)
			require.NotEqual(t, "ready", status.State)
		})
	}
}

func TestCodexTicketAuthCooldownOverridesValidCachedTicket(t *testing.T) {
	a := stateTicketTestAccount(8202)
	ticket := stateTicketVerified(a, time.Now())
	a.Extra[openAICodexTicketExtraKey(ticket.Model)] = ticket
	s, repo := stateTicketTestService(t, a)
	until := time.Now().Add(time.Minute)
	repo.accounts[a.ID].TempUnschedulableUntil = &until
	repo.accounts[a.ID].TempUnschedulableReason = "OAuth 401: token_expired"
	ctx := With429Enforcement(context.Background(), false)
	status, err := s.GetCodexAccountTicketStatus(ctx, a.ID)
	require.NoError(t, err)
	require.Equal(t, "authentication_blocked", status.State)
	require.True(t, status.AuthenticationBlocked)
	require.False(t, status.TicketUsable)
	require.NotNil(t, status.CredentialCurrent)
	require.True(t, *status.CredentialCurrent)
	require.Equal(t, codexTicketExpiryKind, status.ExpiryKind)
	require.ErrorIs(t, s.applyOpenAICodexTicket(ctx, a, ticket.Model, make(http.Header)), ErrOpenAICodexTicketUnavailable)
	statuses := OpenAICodexTicketStatuses(repo.accounts[a.ID], s.cfg.Gateway.OpenAICodexTicket, time.Now())
	require.False(t, statuses[0].Ready)
}

func TestCodexTicketPublicationRacesDiscardStaleAuthorization(t *testing.T) {
	for _, stage := range []string{"dynamic", "fixed"} {
		for _, change := range []string{"reauthorize", "refresh", "protection", "runtime", "401"} {
			t.Run(stage+"/"+change, func(t *testing.T) {
				a := stateTicketTestAccount(8203)
				s, repo := stateTicketTestService(t, a)
				calls := 0
				s.openaiCodexTicketProbe = func(_ context.Context, account *Account, _, _, _, state string, _ time.Duration) (string, int, error) {
					calls++
					if (stage == "dynamic") == (state == "") {
						repo.mu.Lock()
						live := repo.accounts[a.ID]
						switch change {
						case "reauthorize":
							live.Credentials["access_token"] = "reimported-token"
						case "refresh":
							live.Credentials["refresh_token"] = "rotated-refresh"
							live.Credentials["_token_version"] = 3
						case "protection":
							PrepareNewAccountProtection(live)
						case "runtime":
							s.cfg.Gateway.ForceCodexCLI = true
						case "401":
							until := time.Now().Add(time.Hour)
							live.TempUnschedulableUntil = &until
							live.TempUnschedulableReason = "OAuth 401: token_expired"
						}
						repo.mu.Unlock()
					}
					return stateTicketVerified(account, time.Now()).State, 200, nil
				}
				waitStateTicketJob(t, s.startCodexAccountTicketJob(context.Background(), a.ID, false))
				live, err := repo.GetByID(context.Background(), a.ID)
				require.NoError(t, err)
				require.Nil(t, live.Extra[openAICodexTicketExtraKey(openAICodexTicketDefaultModel)])
				require.Nil(t, s.lookupOpenAICodexTicket(live, openAICodexTicketDefaultModel))
				require.LessOrEqual(t, calls, 2)
			})
		}
	}
}

func TestCodexTicketTokenCacheCannotVerifyDifferentCredentialGeneration(t *testing.T) {
	a := stateTicketTestAccount(8204)
	s, repo := stateTicketTestService(t, a)
	cache := newOpenAITokenCacheStub()
	cache.tokens[OpenAITokenCacheKey(a)] = "different-cached-token"
	s.openAITokenProvider = NewOpenAITokenProvider(repo, cache, nil)
	s.openaiCodexTicketProbe = func(context.Context, *Account, string, string, string, string, time.Duration) (string, int, error) {
		t.Fatal("a cached token must be bound to the current durable credentials before either probe")
		return "", 0, nil
	}
	waitStateTicketJob(t, s.startCodexAccountTicketJob(context.Background(), a.ID, false))
}

func TestCodexTicketFixedProbeUsesBusinessLegacyIdentity(t *testing.T) {
	a := stateTicketTestAccount(8205)
	PrepareNewAccountProtection(a)
	a.Credentials["user_agent"] = "codex_cli_rs/0.153.4 (Mac OS 15.0; arm64)"
	s, _ := stateTicketTestService(t, a)
	state := stateTicketVerified(a, time.Now()).State
	req, err := s.buildCodexTicketProbeRequest(context.Background(), a, a.GetOpenAIAccessToken(), openAICodexTicketDefaultModel, state)
	require.NoError(t, err)
	want := resolveCodexOutboundIdentity(s.codexIdentityOverrideUA(a))
	require.Equal(t, want.userAgent, req.Header.Get("User-Agent"))
	require.Equal(t, want.version, req.Header.Get("version"))
	require.Equal(t, want.originator, req.Header.Get("originator"))
	require.Equal(t, state, req.Header.Get(openAICodexTurnStateHeader))
	require.Equal(t, "local-account", req.Header.Get("ChatGPT-Account-ID"))
	require.True(t, HTTPUpstreamRedirectsDisabled(req.Context()))
	body, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(body, &payload))
	metadata := payload["client_metadata"].(map[string]any)
	require.NotEmpty(t, req.Header.Get("x-codex-installation-id"))
	require.Equal(t, req.Header.Get("x-codex-installation-id"), metadata["x-codex-installation-id"])
	require.Equal(t, req.Header.Get("session_id"), metadata["session_id"])
	require.Equal(t, req.Header.Get("thread-id"), metadata["thread_id"])
	seed, _ := codexFingerprintSeed(a.Extra)
	require.Equal(t, resolveConvergedSessionID(seed), req.Header.Get("session_id"))
}

func TestCodexTicketOnlyFixedRouteFailureChangesBusinessHealth(t *testing.T) {
	for _, fixed := range []bool{false, true} {
		for _, reason := range []string{"model", "http401", "sse401"} {
			t.Run(reason+map[bool]string{false: "/candidate", true: "/fixed"}[fixed], func(t *testing.T) {
				a := stateTicketTestAccount(8206)
				s, repo := stateTicketTestService(t, a)
				t.Cleanup(func() { ClearPendingAccountModelMismatch(a.ID) })
				health := &openAIHealthRepoStub{}
				s.rateLimitService = NewRateLimitService(health, nil, &config.Config{}, nil, nil)
				status := http.StatusOK
				body := `data: {"type":"response.completed","response":{"status":"completed","model":"gpt-5.6-luna"}}` + "\n\n"
				if reason == "http401" {
					status = 401
					body = `{"error":{"code":"token_revoked"}}`
				}
				if reason == "sse401" {
					body = `data: {"type":"response.failed","response":{"error":{"code":"token_revoked"}}}` + "\n\n"
				}
				state := ""
				if fixed {
					state = stateTicketVerified(a, time.Now()).State
				}
				response := &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
				_, gotStatus, err := s.validateCodexTicketProbeResponse(context.Background(), a, "actual-token", openAICodexTicketDefaultModel, state, response)
				require.Error(t, err)
				live, readErr := repo.GetByID(context.Background(), a.ID)
				require.NoError(t, readErr)
				if reason == "model" {
					require.Equal(t, fixed, live.HasModelMismatch())
				} else {
					require.Equal(t, 401, gotStatus)
					if fixed {
						require.NotNil(t, health.expected)
						require.True(t, health.mutation.Permanent)
						require.Equal(t, "actual-token", health.expected.GetCredential("access_token"))
					} else {
						require.Nil(t, health.expected)
					}
				}
			})
		}
	}
}

func TestCodexTicketLateFixedMismatchDoesNotQuarantineReauthorizedAccount(t *testing.T) {
	a := stateTicketTestAccount(8207)
	s, repo := stateTicketTestService(t, a)
	repo.accounts[a.ID].Credentials["access_token"] = "new-authorization"
	body := `data: {"type":"response.completed","response":{"status":"completed","model":"gpt-5.6-luna"}}` + "\n\n"
	response := &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
	_, _, err := s.validateCodexTicketProbeResponse(context.Background(), a, a.GetOpenAIAccessToken(), openAICodexTicketDefaultModel, stateTicketVerified(a, time.Now()).State, response)
	require.Error(t, err)
	live, readErr := repo.GetByID(context.Background(), a.ID)
	require.NoError(t, readErr)
	require.False(t, live.HasModelMismatch())
	require.True(t, live.Schedulable)
}

func TestCodexTicketStatusOmitsNonexistentAndDisabledBindingEvidence(t *testing.T) {
	a := stateTicketTestAccount(8208)
	s, _ := stateTicketTestService(t, a)
	for _, disabled := range []bool{false, true} {
		if disabled {
			ticket := stateTicketVerified(a, time.Now())
			a.Extra[openAICodexTicketExtraKey(ticket.Model)] = ticket
			ac := codexAccountTicketConfigOf(a)
			ac.Enabled = false
			a.Extra[codexAccountTicketConfigKey] = ac
		}
		status := s.CodexAccountTicketStatusFromAccount(context.Background(), a)
		require.Nil(t, status.CredentialCurrent)
		require.Nil(t, status.IdentityCurrent)
		encoded, err := json.Marshal(status)
		require.NoError(t, err)
		require.NotContains(t, string(encoded), "credential_current")
		require.NotContains(t, string(encoded), "identity_current")
	}
}

func TestCodexTicketAutomaticAdmissionRequiresVerificationAcrossModels(t *testing.T) {
	for _, model := range []string{openAICodexTicketDefaultModel, "gpt-5.6-luna"} {
		t.Run(model, func(t *testing.T) {
			a := stateTicketTestAccount(8209)
			ac := codexAccountTicketConfigOf(a)
			ac.RequireVerified = true
			a.Extra[codexAccountTicketConfigKey] = ac
			s, repo := stateTicketTestService(t, a)
			ctx := context.Background()
			for _, masterEnabled := range []bool{false, true} {
				s.cfg.Gateway.OpenAICodexTicket.Enabled = masterEnabled
				require.True(t, s.openAICodexTicketBlocksAccount(ctx, a, model))
				require.ErrorIs(t, s.applyOpenAICodexTicket(ctx, a, model, make(http.Header)), ErrOpenAICodexTicketUnavailable)
			}
			ticket := stateTicketVerified(a, time.Now())
			repo.accounts[a.ID].Extra[openAICodexTicketExtraKey(ticket.Model)] = ticket
			require.False(t, s.openAICodexTicketBlocksAccount(ctx, a, model))
			headers := make(http.Header)
			require.NoError(t, s.applyOpenAICodexTicket(ctx, a, model, headers))
			require.Equal(t, model == ac.Model, headers.Get(openAICodexTurnStateHeader) != "")
			s.cfg.Gateway.OpenAICodexTicket.Enabled = false
			require.True(t, s.openAICodexTicketBlocksAccount(ctx, a, model))
			_, err := s.ConfigureCodexAccountTicket(ctx, a.ID, CodexAccountTicketUpdate{Enabled: false})
			require.NoError(t, err)
			live, err := repo.GetByID(ctx, a.ID)
			require.NoError(t, err)
			require.False(t, codexAccountTicketConfigOf(live).RequireVerified)
			require.False(t, s.openAICodexTicketBlocksAccount(ctx, live, model))
		})
	}
}
