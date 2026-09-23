package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/stretchr/testify/require"
)

type stateTicketCandidateTransport struct {
	requests []*http.Request
	proxies  []string
	body     string
	state    string
	before   func()
	respond  func(int, *http.Request) (string, string)
}

func (u *stateTicketCandidateTransport) Do(req *http.Request, proxy string, _ int64, _ int) (*http.Response, error) {
	u.requests = append(u.requests, req)
	u.proxies = append(u.proxies, proxy)
	if u.before != nil {
		u.before()
	}
	body, state := u.body, u.state
	if u.respond != nil {
		body, state = u.respond(len(u.requests), req)
	}
	header := make(http.Header)
	if state != "" {
		header.Set(openAICodexTurnStateHeader, state)
	}
	return &http.Response{StatusCode: 200, Header: header, Body: io.NopCloser(strings.NewReader(body))}, nil
}
func (u *stateTicketCandidateTransport) DoWithTLS(req *http.Request, proxy string, id int64, limit int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxy, id, limit)
}

func stateTicketCandidateFixture(t *testing.T) (*OpenAIGatewayService, *stateTicketTestRepo, *Account, *openAICodexTicket, *codexAccountTicketJob) {
	t.Helper()
	a := stateTicketTestAccount(8501)
	a.Concurrency = 100
	PrepareNewAccountProtection(a)
	old := stateTicketVerified(a, time.Now().Add(-time.Minute))
	old.ExpiresAt = time.Now().Add(10 * time.Minute)
	a.Extra[openAICodexTicketExtraKey(old.Model)] = old
	a.Concurrency = 50
	BoundAccountProtectionConcurrency(a)
	s, repo := stateTicketTestService(t, a)
	ac := codexAccountTicketConfigOf(a)
	job := &codexAccountTicketJob{revision: ac.Revision, fixedFingerprint: codexTicketFixedProxyFingerprint(a), harvestProxyURL: s.openAICodexTicketHarvestProxyURLContext(context.Background())}
	return s, repo, a, old, job
}

func candidateCompletedBody(model string) string {
	return `data: {"type":"response.completed","response":{"status":"completed","model":"` + model + `"}}` + "\n\n"
}

func TestCodexTicketConcurrencyChangeReverifiesCandidateOnlyOnFixedRoute(t *testing.T) {
	for _, oldVerified := range []bool{true, false} {
		t.Run(map[bool]string{true: "old-proof", false: "untrusted-candidate"}[oldVerified], func(t *testing.T) {
			s, repo, a, old, job := stateTicketCandidateFixture(t)
			old.Verified = oldVerified
			require.False(t, old.validFor(a, codexAccountTicketConfigOf(a), time.Now()))
			require.Nil(t, s.lookupOpenAICodexTicket(a, old.Model))
			upstream := &stateTicketCandidateTransport{body: candidateCompletedBody(old.Model)}
			s.httpUpstream = upstream
			s.runCodexAccountTicketJob(context.Background(), a.ID, job)
			require.Empty(t, job.lastError)
			require.Len(t, upstream.requests, 1)
			require.Equal(t, []string{a.Proxy.URL()}, upstream.proxies)
			require.Equal(t, "Bearer "+a.GetOpenAIAccessToken(), upstream.requests[0].Header.Get("Authorization"))
			require.Equal(t, old.State, upstream.requests[0].Header.Get(openAICodexTurnStateHeader))
			live, err := repo.GetByID(context.Background(), a.ID)
			require.NoError(t, err)
			ticket := s.lookupOpenAICodexTicket(live, old.Model)
			require.NotNil(t, ticket)
			require.True(t, ticket.Verified)
			require.Equal(t, codexTicketIdentityFingerprint(a), ticket.IdentityFingerprint)
			require.Equal(t, codexTicketCredentialFingerprint(a), ticket.CredentialFingerprint)
			require.Equal(t, 50, live.Concurrency)
			require.False(t, ticket.ExpiresAt.After(old.ExpiresAt), "reverification must never extend the old STATE lifetime")
			require.Equal(t, 1, repo.writes)
		})
	}
}

func TestCodexTicketCandidateRequiresRealCompletedTargetModel(t *testing.T) {
	for _, scenario := range []string{"missing-model", "wrong-model", "312"} {
		t.Run(scenario, func(t *testing.T) {
			s, repo, a, old, job := stateTicketCandidateFixture(t)
			old.Verified = false
			upstream := &stateTicketCandidateTransport{body: candidateCompletedBody(old.Model)}
			switch scenario {
			case "missing-model":
				upstream.body = `data: {"type":"response.completed","response":{"status":"completed"}}` + "\n\n"
			case "wrong-model":
				upstream.body = candidateCompletedBody("other-model")
			case "312":
				upstream.state = "gAAAAA" + strings.Repeat("A", 306)
			}
			s.httpUpstream = upstream
			ctx := context.WithValue(context.Background(), codexTicketCandidateKey{}, old)
			finished, reason := s.verifyAndPublishCodexTicket(ctx, a, a.GetOpenAIAccessToken(), old.State, s.codexTicketRuntimeFingerprint(), old.ExpiresAt, time.Second, 1, job)
			require.False(t, finished, "invalid candidate permits a bounded fresh dynamic attempt")
			require.NotEmpty(t, reason)
			require.Equal(t, 0, repo.writes)
			require.Nil(t, s.lookupOpenAICodexTicket(a, old.Model))
		})
	}
}

func TestCodexTicketRejectedCandidateFallsBackOnceBeforeDynamicAuthStop(t *testing.T) {
	s, repo, a, old, job := stateTicketCandidateFixture(t)
	var routes []string
	s.openaiCodexTicketProbe = func(_ context.Context, _ *Account, _, _, proxy, state string, _ time.Duration) (string, int, error) {
		routes = append(routes, proxy)
		if len(routes) == 1 {
			require.Equal(t, a.Proxy.URL(), proxy)
			require.Equal(t, old.State, state)
			return "", 200, errors.New("no completed model")
		}
		require.Empty(t, state)
		require.NotEqual(t, a.Proxy.URL(), proxy)
		return "", 401, errors.New("rejected")
	}
	s.runCodexAccountTicketJob(context.Background(), a.ID, job)
	require.Len(t, routes, 2)
	require.Contains(t, job.lastError, "401")
	require.Zero(t, repo.writes)
}

func TestCodexTicketCandidatePublicationRejectsConcurrentChanges(t *testing.T) {
	for _, change := range []string{"expired", "revoked", "removed", "reauthorized"} {
		t.Run(change, func(t *testing.T) {
			s, repo, a, old, job := stateTicketCandidateFixture(t)
			candidate := *old
			upstream := &stateTicketCandidateTransport{body: candidateCompletedBody(old.Model), before: func() {
				switch change {
				case "expired":
					candidate.ExpiresAt = time.Now().Add(-time.Second)
				case "revoked":
					s.openaiCodexWatchdogRevoked.Store(receiptForCodexTicket(old).key(), true)
				case "removed":
					delete(repo.accounts[a.ID].Extra, openAICodexTicketExtraKey(old.Model))
				case "reauthorized":
					repo.accounts[a.ID].Credentials["access_token"] = "replacement-token"
				}
			}}
			s.httpUpstream = upstream
			ctx := context.WithValue(context.Background(), codexTicketCandidateKey{}, &candidate)
			finished, reason := s.verifyAndPublishCodexTicket(ctx, a, a.GetOpenAIAccessToken(), old.State, s.codexTicketRuntimeFingerprint(), old.ExpiresAt, time.Second, 1, job)
			require.True(t, finished)
			require.NotEmpty(t, reason)
			require.Zero(t, repo.writes)
		})
	}
}

func TestCodexTicketFailedCandidateDoesNotSendObsoleteTokenToDynamicPool(t *testing.T) {
	s, repo, a, _, job := stateTicketCandidateFixture(t)
	calls := 0
	s.openaiCodexTicketProbe = func(_ context.Context, _ *Account, _, _, proxy, _ string, _ time.Duration) (string, int, error) {
		calls++
		require.Equal(t, a.Proxy.URL(), proxy)
		repo.accounts[a.ID].Credentials["access_token"] = "new-token"
		return "", 200, errors.New("incomplete")
	}
	s.runCodexAccountTicketJob(context.Background(), a.ID, job)
	require.Equal(t, 1, calls)
	require.Zero(t, repo.writes)
}

func TestCodexTicketCurrentValidRenewalCollectsNewStateLifetime(t *testing.T) {
	a := stateTicketTestAccount(8502)
	old := stateTicketVerified(a, time.Now().Add(-time.Minute))
	old.ExpiresAt = time.Now().Add(30 * time.Minute)
	a.Extra[openAICodexTicketExtraKey(old.Model)] = old
	s, repo := stateTicketTestService(t, a)
	require.Nil(t, s.codexTicketUnverifiedCandidate(a, codexAccountTicketConfigOf(a), time.Now()))
	calls := 0
	s.openaiCodexTicketProbe = func(_ context.Context, account *Account, _, _, proxy, state string, _ time.Duration) (string, int, error) {
		calls++
		if calls == 1 {
			require.Empty(t, state)
			require.NotEqual(t, a.Proxy.URL(), proxy)
			return stateTicketVerified(account, time.Now()).State, 200, nil
		}
		require.Equal(t, a.Proxy.URL(), proxy)
		return "", 200, nil
	}
	job := &codexAccountTicketJob{revision: codexAccountTicketConfigOf(a).Revision, fixedFingerprint: codexTicketFixedProxyFingerprint(a), harvestProxyURL: s.openAICodexTicketHarvestProxyURLContext(context.Background())}
	s.runCodexAccountTicketJob(context.Background(), a.ID, job)
	require.Equal(t, 2, calls)
	require.Empty(t, job.lastError)
	live, err := repo.GetByID(context.Background(), a.ID)
	require.NoError(t, err)
	renewed := s.lookupOpenAICodexTicket(live, old.Model)
	require.NotNil(t, renewed)
	require.True(t, renewed.ExpiresAt.After(old.ExpiresAt))
}

func TestCodexTicketFixedReplacementMustBeSentAndVerifiedBeforeFreshTTL(t *testing.T) {
	for _, scenario := range []string{"success", "second-wrong-model", "second-312", "second-reauthorize", "source-removed"} {
		t.Run(scenario, func(t *testing.T) {
			s, repo, a, old, job := stateTicketCandidateFixture(t)
			newState := "gAAAAA" + strings.Repeat("B", old.Length-6)
			thirdState := "gAAAAA" + strings.Repeat("C", old.Length-6)
			upstream := &stateTicketCandidateTransport{}
			upstream.respond = func(call int, req *http.Request) (string, string) {
				require.Equal(t, "Bearer "+a.GetOpenAIAccessToken(), req.Header.Get("Authorization"))
				if call == 1 {
					require.Equal(t, old.State, req.Header.Get(openAICodexTurnStateHeader))
					require.Zero(t, repo.writes, "fresh response header alone is never publishable proof")
					return candidateCompletedBody(old.Model), newState
				}
				require.Equal(t, 2, call, "fresh STATE validation must not recurse")
				require.Equal(t, newState, req.Header.Get(openAICodexTurnStateHeader))
				switch scenario {
				case "second-wrong-model":
					return candidateCompletedBody("other-model"), thirdState
				case "second-312":
					return candidateCompletedBody(old.Model), "gAAAAA" + strings.Repeat("A", 306)
				case "second-reauthorize":
					repo.accounts[a.ID].Credentials["access_token"] = "replaced-after-second-probe"
				case "source-removed":
					delete(repo.accounts[a.ID].Extra, openAICodexTicketExtraKey(old.Model))
				}
				return candidateCompletedBody(old.Model), thirdState
			}
			s.httpUpstream = upstream
			ctx := context.WithValue(context.Background(), codexTicketCandidateKey{}, old)
			_, reason := s.verifyAndPublishCodexTicket(ctx, a, a.GetOpenAIAccessToken(), old.State, s.codexTicketRuntimeFingerprint(), old.ExpiresAt, time.Second, 1, job)
			require.Len(t, upstream.requests, 2)
			require.Equal(t, []string{a.Proxy.URL(), a.Proxy.URL()}, upstream.proxies)
			if scenario != "success" {
				require.NotEmpty(t, reason)
				require.Zero(t, repo.writes)
				return
			}
			require.Empty(t, reason)
			require.Equal(t, 1, repo.writes)
			live, err := repo.GetByID(context.Background(), a.ID)
			require.NoError(t, err)
			ticket := s.lookupOpenAICodexTicket(live, old.Model)
			require.NotNil(t, ticket)
			require.Equal(t, newState, ticket.State, "publish exactly the STATE actually sent and verified, not the next unverified response header")
			require.True(t, ticket.ExpiresAt.After(old.ExpiresAt))
			require.LessOrEqual(t, ticket.ExpiresAt.Sub(ticket.CapturedAt), time.Hour)
			require.Equal(t, old.Model, ticket.VerifiedModel)
		})
	}
}

func TestCodexTicketCurrentNearExpiryWithoutFreshHeaderFallsBackWithoutExtending(t *testing.T) {
	a := stateTicketTestAccount(8503)
	old := stateTicketVerified(a, time.Now().Add(-time.Minute))
	old.ExpiresAt = time.Now().Add(time.Minute)
	a.Extra[openAICodexTicketExtraKey(old.Model)] = old
	s, repo := stateTicketTestService(t, a)
	require.NotNil(t, s.codexTicketUnverifiedCandidate(a, codexAccountTicketConfigOf(a), time.Now()))
	calls := 0
	s.openaiCodexTicketProbe = func(_ context.Context, _ *Account, _, _, proxy, state string, _ time.Duration) (string, int, error) {
		calls++
		if calls == 1 {
			require.Equal(t, old.State, state)
			require.Equal(t, a.Proxy.URL(), proxy)
			return old.State, 200, nil
		}
		require.Empty(t, state)
		require.NotEqual(t, a.Proxy.URL(), proxy)
		return "", 401, errors.New("dynamic stop")
	}
	job := &codexAccountTicketJob{revision: codexAccountTicketConfigOf(a).Revision, fixedFingerprint: codexTicketFixedProxyFingerprint(a), harvestProxyURL: s.openAICodexTicketHarvestProxyURLContext(context.Background())}
	s.runCodexAccountTicketJob(context.Background(), a.ID, job)
	require.Equal(t, 2, calls)
	require.Zero(t, repo.writes)
	live, err := repo.GetByID(context.Background(), a.ID)
	require.NoError(t, err)
	unchanged := parseOpenAICodexTicketFromAny(a.ID, old.Model, live.Extra[openAICodexTicketExtraKey(old.Model)])
	require.Equal(t, old.ExpiresAt.UnixNano(), unchanged.ExpiresAt.UnixNano())
	require.Equal(t, old.CapturedAt.UnixNano(), unchanged.CapturedAt.UnixNano())
}
