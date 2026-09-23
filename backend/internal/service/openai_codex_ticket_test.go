package service

import (
	"context"
	"errors"
	"io"
	"maps"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type stateTicketTestRepo struct {
	AccountRepository
	mu           sync.Mutex
	accounts     map[int64]*Account
	writes       int
	mutations    int
	failMutation bool
}

func cloneStateTicketAccount(a *Account) *Account {
	if a == nil {
		return nil
	}
	c := *a
	c.Extra = maps.Clone(a.Extra)
	c.Credentials = maps.Clone(a.Credentials)
	if a.Proxy != nil {
		p := *a.Proxy
		c.Proxy = &p
	}
	return &c
}
func (r *stateTicketTestRepo) GetByID(_ context.Context, id int64) (*Account, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	a := cloneStateTicketAccount(r.accounts[id])
	if a == nil {
		return nil, errors.New("not found")
	}
	return a, nil
}
func (r *stateTicketTestRepo) ListByPlatform(context.Context, string) ([]Account, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []Account
	for _, a := range r.accounts {
		out = append(out, *cloneStateTicketAccount(a))
	}
	return out, nil
}
func (r *stateTicketTestRepo) MutateOpenAICodexTicketExtra(_ context.Context, id int64, mutate func(*Account) (map[string]any, error)) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.mutations++
	if r.failMutation {
		return errors.New("local simulated persistence failure")
	}
	a := r.accounts[id]
	if a == nil {
		return errors.New("not found")
	}
	updates, err := mutate(cloneStateTicketAccount(a))
	if err != nil {
		return err
	}
	for k, v := range updates {
		a.Extra[k] = v
	}
	if len(updates) > 0 {
		r.writes++
	}
	return nil
}

func (r *stateTicketTestRepo) MarkAccountModelMismatch(_ context.Context, id int64, expected, actual, requestID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	a := r.accounts[id]
	a.Schedulable = false
	a.Extra[AccountModelMismatchExtraKey] = map[string]any{"expected_model": expected, "actual_model": actual, "request_id": requestID}
	return nil
}

func stateTicketTestAccount(id int64) *Account {
	proxyID := id + 100
	return &Account{ID: id, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 10, ProxyID: &proxyID,
		Proxy:       &Proxy{ID: proxyID, Protocol: "http", Host: "fixed.invalid", Port: 8080, Status: StatusActive},
		Credentials: map[string]any{"access_token": "local-test-token", "chatgpt_account_id": "local-account"},
		Extra:       map[string]any{codexAccountTicketConfigKey: codexAccountTicketConfig{Enabled: true, Model: openAICodexTicketDefaultModel, TicketPlan: "pro", Revision: "revision-1"}}}
}
func stateTicketVerified(account *Account, now time.Time) *openAICodexTicket {
	ac := codexAccountTicketConfigOf(account)
	state := "gAAAAA" + strings.Repeat("A", codexTicketTargetLength(ac.TicketPlan)-6)
	return &openAICodexTicket{AccountID: account.ID, Model: ac.Model, State: state, Length: len(state), CapturedAt: now, ExpiresAt: now.Add(time.Hour), Verified: true, ConfigRevision: ac.Revision, FixedProxyFingerprint: codexTicketFixedProxyFingerprint(account), CredentialFingerprint: codexTicketCredentialFingerprint(account), IdentityFingerprint: codexTicketIdentityFingerprint(account), RuntimeFingerprint: (&OpenAIGatewayService{cfg: &config.Config{}}).codexTicketRuntimeFingerprint(), VerifiedAt: now, VerifiedModel: ac.Model}
}
func stateTicketTestService(t *testing.T, account *Account) (*OpenAIGatewayService, *stateTicketTestRepo) {
	t.Helper()
	repo := &stateTicketTestRepo{accounts: map[int64]*Account{account.ID: cloneStateTicketAccount(account)}}
	s := &OpenAIGatewayService{accountRepo: repo, cfg: &config.Config{Gateway: config.GatewayConfig{OpenAICodexTicket: config.OpenAICodexTicketConfig{Enabled: true, HarvestProxyURL: "http://user-{sid}:secret@pool.invalid:8080"}}}}
	t.Cleanup(s.StopOpenAICodexTicketHarvester)
	return s, repo
}
func waitStateTicketJob(t *testing.T, job *codexAccountTicketJob) {
	t.Helper()
	require.NotNil(t, job)
	select {
	case <-job.done:
	case <-time.After(3 * time.Second):
		t.Fatal("ticket job did not stop")
	}
}

func TestCodexTicketHarvestUsesDynamicThenExactFixedRouteAndPreservesQuarantine(t *testing.T) {
	a := stateTicketTestAccount(7001)
	a.Schedulable = false
	a.Extra[AccountModelMismatchExtraKey] = map[string]any{"expected_model": "gpt-6-astra", "actual_model": "gpt-5.6-luna"}
	s, repo := stateTicketTestService(t, a)
	s.cfg.Gateway.OpenAICodexTicket.TTLSeconds = 120
	var routes []string
	s.openaiCodexTicketProbe = func(_ context.Context, account *Account, token, model, proxy, state string, _ time.Duration) (string, int, error) {
		routes = append(routes, proxy)
		if len(routes) == 1 {
			require.Empty(t, state)
			require.NotContains(t, proxy, "{sid}")
			return stateTicketVerified(account, time.Now()).State, 200, nil
		}
		require.Equal(t, a.Proxy.URL(), proxy)
		require.Len(t, state, 292)
		return "", 200, nil
	}
	waitStateTicketJob(t, s.startCodexAccountTicketJob(context.Background(), a.ID, false))
	require.Len(t, routes, 2)
	require.NotEqual(t, routes[0], routes[1])
	live, err := repo.GetByID(context.Background(), a.ID)
	require.NoError(t, err)
	ticket := s.lookupOpenAICodexTicket(live, openAICodexTicketDefaultModel)
	require.NotNil(t, ticket)
	require.Equal(t, 120*time.Second, ticket.ExpiresAt.Sub(ticket.CapturedAt))
	require.True(t, live.HasModelMismatch())
	require.False(t, live.Schedulable)
}

func TestCodexTicketRejectedRoundRetainsOldTicketAndManualCooldown(t *testing.T) {
	for _, status := range []int{401, 403} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			a := stateTicketTestAccount(int64(status))
			old := stateTicketVerified(a, time.Now().Add(-30*time.Minute))
			a.Extra[openAICodexTicketExtraKey(old.Model)] = old
			s, repo := stateTicketTestService(t, a)
			var probes atomic.Int32
			s.openaiCodexTicketProbe = func(context.Context, *Account, string, string, string, string, time.Duration) (string, int, error) {
				probes.Add(1)
				return "", status, errors.New("rejected")
			}
			waitStateTicketJob(t, s.startCodexAccountTicketJob(context.Background(), a.ID, false))
			require.EqualValues(t, 1, probes.Load())
			require.Nil(t, s.startCodexAccountTicketJob(context.Background(), a.ID, true))
			live, _ := repo.GetByID(context.Background(), a.ID)
			require.Equal(t, old.State, s.lookupOpenAICodexTicket(live, old.Model).State)
		})
	}
}

func TestCodexTicketPublicationRejectsChangedRouteOrConfig(t *testing.T) {
	for _, kind := range []string{"proxy", "config"} {
		t.Run(kind, func(t *testing.T) {
			a := stateTicketTestAccount(7010)
			s, repo := stateTicketTestService(t, a)
			s.openaiCodexTicketProbe = func(_ context.Context, account *Account, _ string, _ string, _ string, state string, _ time.Duration) (string, int, error) {
				if state == "" {
					return stateTicketVerified(account, time.Now()).State, 200, nil
				}
				repo.mu.Lock()
				if kind == "proxy" {
					repo.accounts[a.ID].Proxy.Host = "new.invalid"
				} else {
					ac := codexAccountTicketConfigOf(a)
					ac.Revision = "new"
					repo.accounts[a.ID].Extra[codexAccountTicketConfigKey] = ac
				}
				repo.mu.Unlock()
				return "", 200, nil
			}
			waitStateTicketJob(t, s.startCodexAccountTicketJob(context.Background(), a.ID, false))
			live, _ := repo.GetByID(context.Background(), a.ID)
			require.Nil(t, s.lookupOpenAICodexTicket(live, openAICodexTicketDefaultModel))
		})
	}
}

func TestCodexTicketBoundedJobsAndStop(t *testing.T) {
	a := stateTicketTestAccount(7100)
	s, repo := stateTicketTestService(t, a)
	s.cfg.Gateway.OpenAICodexTicket.MaxConcurrentHarvests = 1
	repo.accounts[7101] = stateTicketTestAccount(7101)
	started := make(chan struct{})
	s.openaiCodexTicketProbe = func(ctx context.Context, _ *Account, _ string, _ string, _ string, _ string, _ time.Duration) (string, int, error) {
		close(started)
		<-ctx.Done()
		return "", 0, ctx.Err()
	}
	job := s.startCodexAccountTicketJob(context.Background(), 7100, false)
	<-started
	require.Nil(t, s.startCodexAccountTicketJob(context.Background(), 7101, false))
	s.StopOpenAICodexTicketHarvester()
	waitStateTicketJob(t, job)
	require.Nil(t, s.startCodexAccountTicketJob(context.Background(), 7101, false))
	s.StartOpenAICodexTicketHarvester()
	require.Nil(t, s.openaiCodexTicketDone)
}

func TestCodexTicketDatabaseRevocationOverridesMemoryAndOldResponseCannotRevokeNew(t *testing.T) {
	a := stateTicketTestAccount(7200)
	old := stateTicketVerified(a, time.Now().Add(-time.Minute))
	a.Extra[openAICodexTicketExtraKey(old.Model)] = old
	s, repo := stateTicketTestService(t, a)
	s.cfg.Gateway.OpenAICodexTicket.HarvestProxyURL = ""
	s.openaiCodexTickets.Store(openAICodexTicketKey(a.ID, old.Model), old)
	require.NoError(t, repo.MutateOpenAICodexTicketExtra(context.Background(), a.ID, func(*Account) (map[string]any, error) {
		return map[string]any{openAICodexTicketExtraKey(old.Model): nil}, nil
	}))
	live, _ := repo.GetByID(context.Background(), a.ID)
	require.Nil(t, s.lookupOpenAICodexTicket(live, old.Model))
	newer := stateTicketVerified(a, time.Now())
	require.NoError(t, repo.MutateOpenAICodexTicketExtra(context.Background(), a.ID, func(*Account) (map[string]any, error) {
		return map[string]any{openAICodexTicketExtraKey(old.Model): newer}, nil
	}))
	s.invalidateCodexTicketFromResponse(receiptForCodexTicket(old), "state_312")
	live, _ = repo.GetByID(context.Background(), a.ID)
	require.True(t, receiptForCodexTicket(newer).matches(s.lookupOpenAICodexTicket(live, newer.Model)))
	require.Zero(t, codexTicketWatchdogStatusOf(live, true).TriggerCount)
}

func TestCodexTicketObserver312IsHeaderLengthNotHTTPCode(t *testing.T) {
	for _, status := range []int{200, 312} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			a := stateTicketTestAccount(int64(8000 + status))
			ticket := stateTicketVerified(a, time.Now())
			a.Extra[openAICodexTicketExtraKey(ticket.Model)] = ticket
			s, repo := stateTicketTestService(t, a)
			s.cfg.Gateway.OpenAICodexTicket.HarvestProxyURL = ""
			req, _ := http.NewRequest(http.MethodPost, "https://example.invalid", nil)
			require.NoError(t, s.applyOpenAICodexTicketToRequest(context.Background(), a, ticket.Model, req))
			resp := &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("unchanged"))}
			resp.Header.Set(openAICodexTurnStateHeader, "gAAAAA"+strings.Repeat("B", 306))
			s.observeCodexTicketResponse(req, resp)
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.Equal(t, "unchanged", string(body))
			s.StopOpenAICodexTicketHarvester()
			live, _ := repo.GetByID(context.Background(), a.ID)
			if status == 200 {
				require.Nil(t, s.lookupOpenAICodexTicket(live, ticket.Model))
				require.Equal(t, "state_312", codexTicketWatchdogStatusOf(live, true).LastReason)
				require.False(t, live.HasModelMismatch())
			} else {
				require.NotNil(t, s.lookupOpenAICodexTicket(live, ticket.Model))
			}
		})
	}
}

func TestCodexTicketCompletionAndTransparentObserver(t *testing.T) {
	for _, tc := range []struct {
		name, body       string
		accept, mismatch bool
	}{
		{"completed", "data: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\",\"model\":\"gpt-6-astra\"}}\n\n", true, false},
		{"alias", "event: response.completed\ndata: {\"response\":{\"model\":\"gpt-6\"}}\n\n", true, false},
		{"wrong-model", "data: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\",\"model\":\"gpt-5.6-luna\"}}\n\n", false, true},
		{"failed", "data: {\"type\":\"response.failed\",\"response\":{\"status\":\"failed\",\"model\":\"gpt-5.6-luna\"}}\n\n", false, false},
		{"prose", "data: {\"type\":\"response.output_text.delta\",\"delta\":\"model: gpt-5.6-luna\"}\n\n", false, false},
		{"truncated", "data: {\"type\":\"response.completed\",\"response\":{", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateCodexTicketCompletedModel(strings.NewReader(tc.body), openAICodexTicketDefaultModel)
			require.Equal(t, tc.accept, err == nil)
			triggered := false
			observer := &codexTicketWatchdogBody{ReadCloser: io.NopCloser(strings.NewReader(tc.body)), model: openAICodexTicketDefaultModel, trigger: func(reason string) { require.Equal(t, "model_mismatch", reason); triggered = true }}
			var got strings.Builder
			p := make([]byte, 3)
			for {
				n, err := observer.Read(p)
				got.Write(p[:n])
				if err == io.EOF {
					break
				}
				require.NoError(t, err)
			}
			require.NoError(t, observer.Close())
			require.Equal(t, tc.body, got.String())
			require.Equal(t, tc.mismatch, triggered)
		})
	}
}

func TestCodexTicketProxyAndPlanIsolation(t *testing.T) {
	a := stateTicketTestAccount(7300)
	s, _ := stateTicketTestService(t, a)
	a.Extra[ProxyModeExtraKey] = ProxyModeRandom
	require.False(t, codexAccountTicketEligible(a))
	delete(a.Extra, ProxyModeExtraKey)
	ac := codexAccountTicketConfigOf(a)
	ac.TicketPlan = "team"
	a.Extra[codexAccountTicketConfigKey] = ac
	ticket := stateTicketVerified(a, time.Now())
	require.Len(t, ticket.State, 332)
	require.True(t, ticket.validFor(a, ac, time.Now()))
	other := stateTicketTestAccount(7301)
	require.False(t, ticket.validFor(other, codexAccountTicketConfigOf(other), time.Now()))
	s.settingService = &SettingService{codexTicketSettingsCache: &cachedCodexTicketSettings{expiresAt: time.Now().Add(time.Hour)}}
	require.Empty(t, s.openAICodexTicketHarvestProxyURLContext(context.Background()))
	require.True(t, s.openAICodexTicketBlocksAccount(context.Background(), a, ac.Model)) // Settings storage unavailable fails closed.
	require.False(t, s.openAICodexTicketBlocksAccount(context.Background(), a, "other-model"))
	require.Equal(t, "http://pool.invalid:8080", MaskProxyURL("http://secret-user:secret-password@pool.invalid:8080"))
}

type stateTicketProbeUpstream struct {
	HTTPUpstream
	response *http.Response
	request  *http.Request
	proxy    string
}

func (u *stateTicketProbeUpstream) Do(req *http.Request, proxy string, _ int64, _ int) (*http.Response, error) {
	u.request = req
	u.proxy = proxy
	return u.response, nil
}

func TestCodexTicketFixedProbeRequiresCompletionAndQuarantinesActualMismatch(t *testing.T) {
	a := stateTicketTestAccount(7400)
	s, repo := stateTicketTestService(t, a)
	t.Cleanup(func() { ClearPendingAccountModelMismatch(a.ID) })
	body := `data: {"type":"response.completed","response":{"status":"completed","model":"gpt-5.6-luna"}}` + "\n\n"
	upstream := &stateTicketProbeUpstream{response: &http.Response{StatusCode: 200, Header: http.Header{"X-Request-Id": []string{"local-probe"}}, Body: io.NopCloser(strings.NewReader(body))}}
	s.httpUpstream = upstream
	_, status, err := s.fireCodexAccountTicketProbe(context.Background(), a, "test-token", openAICodexTicketDefaultModel, a.Proxy.URL(), stateTicketVerified(a, time.Now()).State, time.Second)
	require.Equal(t, 200, status)
	var mismatch *codexTicketModelMismatchError
	require.ErrorAs(t, err, &mismatch)
	live, _ := repo.GetByID(context.Background(), a.ID)
	require.True(t, live.HasModelMismatch())
	require.False(t, live.Schedulable)
	require.Equal(t, a.Proxy.URL(), upstream.proxy)
	require.True(t, HTTPUpstreamRedirectsDisabled(upstream.request.Context()))
}

func TestCodexTicketInjectionChecksNewOptInAndNewModel(t *testing.T) {
	for _, scenario := range []string{"new_opt_in", "new_model"} {
		t.Run(scenario, func(t *testing.T) {
			live := stateTicketTestAccount(7500)
			snapshot := cloneStateTicketAccount(live)
			old := codexAccountTicketConfigOf(snapshot)
			if scenario == "new_opt_in" {
				old.Enabled = false
			} else {
				old.Model = openAICodexTicketDefaultSolModel
			}
			snapshot.Extra[codexAccountTicketConfigKey] = old
			s, _ := stateTicketTestService(t, live)
			req, _ := http.NewRequest(http.MethodPost, "https://example.invalid", nil)
			require.ErrorIs(t, s.applyOpenAICodexTicketToRequest(context.Background(), snapshot, openAICodexTicketDefaultModel, req), ErrOpenAICodexTicketUnavailable)
		})
	}
}

func TestCodexTicketConfigurationUsesAtomicMutationAndDoesNotClearQuarantine(t *testing.T) {
	a := stateTicketTestAccount(7600)
	ticket := stateTicketVerified(a, time.Now())
	a.Extra[openAICodexTicketExtraKey(ticket.Model)] = ticket
	a.Schedulable = false
	a.Extra[AccountModelMismatchExtraKey] = map[string]any{"expected_model": "gpt-6-astra", "actual_model": "gpt-5.6-luna"}
	s, repo := stateTicketTestService(t, a)
	s.cfg.Gateway.OpenAICodexTicket.Enabled = false
	status, err := s.ConfigureCodexAccountTicket(context.Background(), a.ID, CodexAccountTicketUpdate{Enabled: true, Model: openAICodexTicketDefaultModel, TicketPlan: "team"})
	require.NoError(t, err)
	require.Equal(t, 332, status.TargetLength)
	live, _ := repo.GetByID(context.Background(), a.ID)
	require.NotEqual(t, "revision-1", codexAccountTicketConfigOf(live).Revision)
	require.Nil(t, live.Extra[openAICodexTicketExtraKey(ticket.Model)])
	require.True(t, live.HasModelMismatch())
	require.False(t, live.Schedulable)
}

func TestCodexTicketSyntheticClientCannotFallBackToDirectOrRedirect(t *testing.T) {
	_, err := newCodexTicketChainedClient("", "")
	require.Error(t, err)
	client, err := newCodexTicketChainedClient("http://pool.invalid:8080", "")
	require.NoError(t, err)
	defer client.CloseIdleConnections()
	transport := client.Transport.(*http.Transport)
	require.True(t, transport.DisableKeepAlives)
	require.ErrorIs(t, client.CheckRedirect(nil, nil), http.ErrUseLastResponse)
}

func TestCodexTicketLateSignalCannotRevokeNewReceiptWithClockSkew(t *testing.T) {
	a := stateTicketTestAccount(7700)
	s, _ := stateTicketTestService(t, a)
	s.cfg.Gateway.OpenAICodexTicket.HarvestProxyURL = ""
	old := stateTicketVerified(a, time.Now())
	replacement := stateTicketVerified(a, time.Now().Add(-30*time.Second))
	replacement.State = "gAAAAA" + strings.Repeat("C", 286)
	s.enqueueCodexTicketInvalidation(receiptForCodexTicket(old), "state_312")
	require.True(t, s.codexTicketRejectedByWatchdog(old))
	require.False(t, s.codexTicketRejectedByWatchdog(replacement))
}

func TestCodexTicketConcurrentSignalsAreDeduplicatedAfterPersistence(t *testing.T) {
	a := stateTicketTestAccount(7800)
	ticket := stateTicketVerified(a, time.Now())
	a.Extra[openAICodexTicketExtraKey(ticket.Model)] = ticket
	s, repo := stateTicketTestService(t, a)
	s.cfg.Gateway.OpenAICodexTicket.HarvestProxyURL = ""
	for range 1000 {
		s.enqueueCodexTicketInvalidation(receiptForCodexTicket(ticket), "state_312")
	}
	s.StopOpenAICodexTicketHarvester()
	repo.mu.Lock()
	defer repo.mu.Unlock()
	require.Equal(t, 1, repo.mutations)
}

func TestCodexTicketFailedInvalidationRetriesWithoutTrafficWhileGlobalDisabled(t *testing.T) {
	a := stateTicketTestAccount(7900)
	ticket := stateTicketVerified(a, time.Now())
	a.Extra[openAICodexTicketExtraKey(ticket.Model)] = ticket
	s, repo := stateTicketTestService(t, a)
	s.cfg.Gateway.OpenAICodexTicket.HarvestProxyURL = ""
	repo.failMutation = true
	receipt := receiptForCodexTicket(ticket)
	s.enqueueCodexTicketInvalidation(receipt, "state_312")
	require.Eventually(t, func() bool { _, queued := s.openaiCodexWatchdogRetry.Load(receipt.key()); return queued }, time.Second, time.Millisecond)
	require.True(t, s.codexTicketRejectedByWatchdog(ticket))
	repo.mu.Lock()
	repo.failMutation = false
	repo.mu.Unlock()
	s.cfg.Gateway.OpenAICodexTicket.Enabled = false
	s.openaiCodexWatchdogPending.Store(receipt.key(), time.Now().Add(-time.Second))
	s.refreshOpenAICodexTickets(context.Background())
	require.Eventually(t, func() bool {
		live, _ := repo.GetByID(context.Background(), a.ID)
		return live.Extra[openAICodexTicketExtraKey(ticket.Model)] == nil && codexTicketWatchdogStatusOf(live, false).LastReason == "state_312"
	}, time.Second, time.Millisecond)
	s.StopOpenAICodexTicketHarvester()
	_, queued := s.openaiCodexWatchdogRetry.Load(receipt.key())
	require.False(t, queued)
}
