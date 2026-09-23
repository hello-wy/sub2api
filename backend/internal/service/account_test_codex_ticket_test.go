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

type stateAccountTestHTTP struct {
	requests []*http.Request
	proxies  []string
	response *http.Response
}

func (u *stateAccountTestHTTP) Do(req *http.Request, proxy string, _ int64, _ int) (*http.Response, error) {
	u.requests = append(u.requests, req)
	u.proxies = append(u.proxies, proxy)
	return u.response, nil
}
func (u *stateAccountTestHTTP) DoWithTLS(req *http.Request, proxy string, id int64, concurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxy, id, concurrency)
}

func stateAccountTestRequest(t *testing.T, model string) *http.Request {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, chatgptCodexURL, strings.NewReader(`{"model":"`+model+`","input":"account test"}`))
	require.NoError(t, err)
	return req
}

func stateAccountTestTransport(responseBody string) *stateAccountTestHTTP {
	return &stateAccountTestHTTP{response: &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(responseBody))}}
}

func TestAccountTestCodexTicketUsesFixedRouteAndPreservesBody(t *testing.T) {
	a := stateTicketTestAccount(8101)
	ticket := stateTicketVerified(a, time.Now())
	a.Extra[openAICodexTicketExtraKey(ticket.Model)] = ticket
	gateway, _ := stateTicketTestService(t, a)
	upstream := stateAccountTestTransport(`{"model":"` + ticket.Model + `"}`)
	s := &AccountTestService{openaiGatewayService: gateway, httpUpstream: upstream}
	req := stateAccountTestRequest(t, ticket.Model)
	originalBody := req.Body
	req.Header.Set(openAICodexTurnStateHeader, "untrusted-client-state")
	resp, err := s.doOpenAIAccountTestUpstream(req, a.Proxy.URL(), a, false)
	require.NoError(t, err)
	require.True(t, originalBody == req.Body)
	payload, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	require.JSONEq(t, `{"model":"`+ticket.Model+`","input":"account test"}`, string(payload))
	require.Len(t, upstream.requests, 1)
	require.Equal(t, ticket.State, upstream.requests[0].Header.Get(openAICodexTurnStateHeader))
	require.Equal(t, []string{a.Proxy.URL()}, upstream.proxies)
	receipt, _ := upstream.requests[0].Context().Value(codexTicketReceiptContextKey{}).(*codexTicketReceipt)
	require.NotNil(t, receipt)
	require.True(t, receipt.matches(ticket))
	_, observed := resp.Body.(*codexTicketWatchdogBody)
	require.True(t, observed)
	_ = resp.Body.Close()
}

func TestAccountTestCodexTicketBlocksUnavailableOrWrongRoute(t *testing.T) {
	for _, reason := range []string{"missing", "expired", "wrong-route", "changed-live-route", "unreadable-body", "service-unavailable"} {
		t.Run(reason, func(t *testing.T) {
			a := stateTicketTestAccount(8102)
			ticket := stateTicketVerified(a, time.Now())
			if reason == "expired" {
				ticket.CapturedAt = time.Now().Add(-2 * time.Hour)
				ticket.ExpiresAt = time.Now().Add(-time.Hour)
			}
			if reason != "missing" {
				a.Extra[openAICodexTicketExtraKey(ticket.Model)] = ticket
			}
			gateway, repo := stateTicketTestService(t, a)
			upstream := stateAccountTestTransport("unchanged")
			s := &AccountTestService{openaiGatewayService: gateway, httpUpstream: upstream}
			req := stateAccountTestRequest(t, ticket.Model)
			proxy := a.Proxy.URL()
			switch reason {
			case "wrong-route":
				proxy = "http://other.invalid:8080"
			case "changed-live-route":
				repo.accounts[a.ID].Proxy.Host = "changed.invalid"
			case "unreadable-body":
				req.GetBody = nil
			case "service-unavailable":
				s.openaiGatewayService = nil
			}
			_, err := s.doOpenAIAccountTestUpstream(req, proxy, a, false)
			require.Error(t, err)
			require.Contains(t, err.Error(), "STATE")
			require.Empty(t, upstream.requests, "blocked tests must not send an upstream request")
		})
	}
}

func TestAccountTestCodexTicketRetainsNormalAndDisabledBehavior(t *testing.T) {
	for _, scenario := range []string{"global-disabled", "account-disabled", "different-model", "ordinary-without-gateway"} {
		t.Run(scenario, func(t *testing.T) {
			a := stateTicketTestAccount(8103)
			if scenario == "account-disabled" || scenario == "ordinary-without-gateway" {
				delete(a.Extra, codexAccountTicketConfigKey)
			}
			gateway, _ := stateTicketTestService(t, a)
			if scenario == "global-disabled" {
				gateway.cfg.Gateway.OpenAICodexTicket.Enabled = false
			}
			upstream := stateAccountTestTransport("unaltered")
			s := &AccountTestService{openaiGatewayService: gateway, httpUpstream: upstream}
			model := openAICodexTicketDefaultModel
			if scenario == "different-model" {
				model = "other-model"
			}
			req := stateAccountTestRequest(t, model)
			req.Header.Set(openAICodexTurnStateHeader, "existing-native-state")
			if scenario == "ordinary-without-gateway" {
				s.openaiGatewayService = nil
				req.GetBody = nil
			}
			resp, err := s.doOpenAIAccountTestUpstream(req, a.Proxy.URL(), a, false)
			require.NoError(t, err)
			require.Len(t, upstream.requests, 1)
			require.Equal(t, "existing-native-state", upstream.requests[0].Header.Get(openAICodexTurnStateHeader))
			payload, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.Equal(t, "unaltered", string(payload))
			_ = resp.Body.Close()
		})
	}
}

func TestAccountTestCodexTicketSeesNewlyEnabledAccount(t *testing.T) {
	a := stateTicketTestAccount(8104)
	gateway, _ := stateTicketTestService(t, a)
	stale := cloneStateTicketAccount(a)
	delete(stale.Extra, codexAccountTicketConfigKey)
	upstream := stateAccountTestTransport("must not send")
	s := &AccountTestService{openaiGatewayService: gateway, httpUpstream: upstream}
	_, err := s.doOpenAIAccountTestUpstream(stateAccountTestRequest(t, openAICodexTicketDefaultModel), a.Proxy.URL(), stale, false)
	require.ErrorIs(t, err, ErrOpenAICodexTicketUnavailable)
	require.Empty(t, upstream.requests)
}

func TestAccountTestCodexTicketObserves312AndModelMismatch(t *testing.T) {
	for _, reason := range []string{"state_312", "model_mismatch"} {
		t.Run(reason, func(t *testing.T) {
			a := stateTicketTestAccount(8105)
			ticket := stateTicketVerified(a, time.Now())
			a.Extra[openAICodexTicketExtraKey(ticket.Model)] = ticket
			gateway, repo := stateTicketTestService(t, a)
			// Refresh must remain entirely mocked, including the asynchronous job.
			gateway.openaiCodexTicketProbe = func(context.Context, *Account, string, string, string, string, time.Duration) (string, int, error) {
				return "", http.StatusTooManyRequests, errors.New("local mocked refresh stop")
			}
			body := `{"object":"response","status":"completed","model":"` + ticket.Model + `"}`
			if reason == "model_mismatch" {
				body = `{"object":"response","status":"completed","model":"unexpected-model"}`
			}
			upstream := stateAccountTestTransport(body)
			if reason == "state_312" {
				upstream.response.Header.Set(openAICodexTurnStateHeader, "gAAAAA"+strings.Repeat("A", 306))
			}
			s := &AccountTestService{openaiGatewayService: gateway, httpUpstream: upstream}
			resp, err := s.doOpenAIAccountTestUpstream(stateAccountTestRequest(t, ticket.Model), a.Proxy.URL(), a, false)
			require.NoError(t, err)
			actual, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.Equal(t, body, string(actual), "the existing test result parser must receive the original response")
			_ = resp.Body.Close()
			require.Eventually(t, func() bool {
				live, _ := repo.GetByID(context.Background(), a.ID)
				return codexTicketWatchdogStatusOf(live, true).LastReason == reason && live.Extra[openAICodexTicketExtraKey(ticket.Model)] == nil
			}, 3*time.Second, 10*time.Millisecond)
			require.Len(t, upstream.requests, 1)
		})
	}
}
