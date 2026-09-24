package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type mismatchPolicyStub struct {
	modelMismatchMarkerStub
	quarantine bool
}

type blockingMismatchPolicyStub struct {
	mismatchPolicyStub
	entered chan struct{}
	release chan struct{}
}

func (r *blockingMismatchPolicyStub) RecordAccountModelMismatch(ctx context.Context, id int64, expected, actual, requestID string, gates ...func([]int64)) (bool, error) {
	if r.quarantine {
		for _, gate := range gates {
			gate(r.ids)
		}
	}
	close(r.entered)
	<-r.release
	return r.quarantine, nil
}

func TestModelMismatchObservationNeverGatesConcurrentRequestsWhilePersistenceIsBlocked(t *testing.T) {
	for _, quarantine := range []bool{false, true} {
		t.Run(fmt.Sprintf("quarantine=%t", quarantine), func(t *testing.T) {
			defer ClearPendingAccountModelMismatch(9840, 9841)
			account := &Account{ID: 9840, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true}
			repo := &blockingMismatchPolicyStub{mismatchPolicyStub: mismatchPolicyStub{modelMismatchMarkerStub: modelMismatchMarkerStub{ids: []int64{9840, 9841}, latest: account}, quarantine: quarantine}, entered: make(chan struct{}), release: make(chan struct{})}
			complete := make(chan bool, 1)
			go func() {
				complete <- quarantineAccountModelMismatch(context.Background(), repo, account, "gpt-6-astra", "gpt-5.6-luna", "blocked-record")
			}()
			defer func() {
				close(repo.release)
				select {
				case handled := <-complete:
					require.True(t, handled)
				case <-time.After(time.Second):
					t.Error("record did not finish")
				}
			}()
			select {
			case <-repo.entered:
			case <-time.After(time.Second):
				t.Fatal("record did not start")
			}
			require.Equal(t, quarantine, hasPendingAccountModelMismatch(account))
			require.Equal(t, quarantine, hasPendingAccountModelMismatch(&Account{ID: 9841}))
			gateway := &OpenAIGatewayService{accountRepo: repo}
			err := gateway.checkWebSocketAccountModelMismatch(context.Background(), account)
			require.Equal(t, quarantine, err != nil)
		})
	}
}

func (r *mismatchPolicyStub) RecordAccountModelMismatch(ctx context.Context, id int64, expected, actual, requestID string, gates ...func([]int64)) (bool, error) {
	if r.quarantine {
		for _, gate := range gates {
			gate(r.ids)
		}
	}
	return r.quarantine, r.MarkAccountModelMismatch(ctx, id, expected, actual, requestID)
}

func TestModelMismatchDisabledPolicyClearsTemporaryGateAndKeepsWebSocket(t *testing.T) {
	defer ClearPendingAccountModelMismatch(9812, 9813)
	observed := &Account{ID: 9812, Schedulable: true, Extra: map[string]any{AccountModelMismatchExtraKey: map[string]any{"expected_model": "gpt-6-astra", "quarantined": false, "actual_model": "gpt-5.6-luna"}}}
	repo := &mismatchPolicyStub{modelMismatchMarkerStub: modelMismatchMarkerStub{ids: []int64{9812, 9813}, latest: observed}}
	s := &OpenAIGatewayService{accountRepo: repo}
	result := &OpenAIForwardResult{Model: "gpt-6-astra", UpstreamResponseModel: "gpt-5.6-luna"}
	s.quarantineForwardResultModelMismatch(context.Background(), observed, result)
	require.True(t, result.modelMismatchQuarantined, "handled observation must be deduplicated")
	require.False(t, hasPendingAccountModelMismatch(observed))
	require.False(t, hasPendingAccountModelMismatch(&Account{ID: 9813}))
	require.NoError(t, s.checkWebSocketAccountModelMismatch(context.Background(), observed))
	s.quarantineForwardResultModelMismatch(context.Background(), observed, result)
	require.Equal(t, 1, repo.calls)
}

func TestModelMismatchDisabledPolicyRetryDoesNotLeaveAQuarantine(t *testing.T) {
	defer ClearPendingAccountModelMismatch(9814)
	repo := &mismatchPolicyStub{modelMismatchMarkerStub: modelMismatchMarkerStub{err: errors.New("database unavailable")}, quarantine: true}
	account := &Account{ID: 9814}
	require.False(t, quarantineAccountModelMismatch(context.Background(), repo, account, "gpt-6-astra", "gpt-5.6-luna", "req"))
	repo.err = nil
	repo.quarantine = false
	value, _ := pendingAccountModelMismatches.Load(account.ID)
	state, ok := value.(*pendingAccountModelMismatch)
	require.True(t, ok)
	state.checkedAt = time.Time{}
	require.False(t, hasPendingAccountModelMismatch(account))
	require.Equal(t, 2, repo.calls)
}

type mismatchAccountTestPolicyRepo struct {
	*stateTicketTestRepo
	quarantine bool
}

func (r *mismatchAccountTestPolicyRepo) RecordAccountModelMismatch(_ context.Context, id int64, expected, actual, requestID string, gates ...func([]int64)) (bool, error) {
	if r.quarantine {
		for _, gate := range gates {
			gate([]int64{id})
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	a := r.accounts[id]
	a.Extra[AccountModelMismatchExtraKey] = map[string]any{"expected_model": expected, "actual_model": actual, "quarantined": r.quarantine}
	if r.quarantine {
		a.Schedulable = false
	}
	return r.quarantine, nil
}

func TestAccountTestModelMismatchRespectsPolicyWithAndWithoutSTATE(t *testing.T) {
	for _, stateEnabled := range []bool{false, true} {
		for _, autoQuarantine := range []bool{false, true} {
			t.Run(fmt.Sprintf("state=%t/quarantine=%t", stateEnabled, autoQuarantine), func(t *testing.T) {
				a := stateTicketTestAccount(9820)
				defer ClearPendingAccountModelMismatch(a.ID)
				ticket := stateTicketVerified(a, time.Now())
				a.Extra[openAICodexTicketExtraKey(ticket.Model)] = ticket
				gateway, base := stateTicketTestService(t, a)
				repo := &mismatchAccountTestPolicyRepo{stateTicketTestRepo: base, quarantine: autoQuarantine}
				gateway.accountRepo = repo
				gateway.cfg.Gateway.OpenAICodexTicket.Enabled = stateEnabled
				gateway.openaiCodexTicketProbe = func(context.Context, *Account, string, string, string, string, time.Duration) (string, int, error) {
					return "", http.StatusTooManyRequests, errors.New("mock probe stopped")
				}
				body := `data: {"type":"response.completed","response":{"status":"completed","model":"gpt-5.6-luna"}}` + "\n\n"
				upstream := stateAccountTestTransport(body)
				s := &AccountTestService{accountRepo: repo, openaiGatewayService: gateway, httpUpstream: upstream}
				resp, err := s.doOpenAIAccountTestUpstream(stateAccountTestRequest(t, ticket.Model), a.Proxy.URL(), a, false)
				require.NoError(t, err)
				actual, err := io.ReadAll(resp.Body)
				require.NoError(t, err)
				require.Equal(t, body, string(actual))
				require.NoError(t, resp.Body.Close())
				require.Eventually(t, func() bool {
					live, _ := repo.GetByID(context.Background(), a.ID)
					return live.HasModelMismatch() && live.IsModelMismatchQuarantined() == autoQuarantine && (autoQuarantine || !hasPendingAccountModelMismatch(live))
				}, time.Second, time.Millisecond)
				live, _ := repo.GetByID(context.Background(), a.ID)
				require.Equal(t, !autoQuarantine, live.Schedulable)
				if stateEnabled {
					require.Eventually(t, func() bool {
						live, _ := repo.GetByID(context.Background(), a.ID)
						return codexTicketWatchdogStatusOf(live, true).LastReason == "model_mismatch"
					}, time.Second, time.Millisecond)
				}
			})
		}
	}
}

func TestAccountTestMismatchObserverIgnoresGeneratedTextAndIncompleteResults(t *testing.T) {
	for _, body := range []string{
		`{"object":"response","status":"completed","model":"gpt-6-astra","output_text":"I am gpt-5.6-luna"}`,
		`{"object":"response","status":"incomplete","model":"gpt-5.6-luna"}`,
		`data: {"type":"response.created","response":{"model":"gpt-5.6-luna"}}` + "\n\n",
		`data: {"type":"response.output_text.delta","delta":"gpt-5.6-luna"}` + "\n\n",
	} {
		observations := 0
		reader := &codexTicketWatchdogBody{ReadCloser: io.NopCloser(strings.NewReader(body)), model: "gpt-6-astra", onModelMismatch: func(string) { observations++ }}
		actual, err := io.ReadAll(reader)
		require.NoError(t, err)
		require.NoError(t, reader.Close())
		require.Equal(t, body, string(actual))
		require.Zero(t, observations)
	}
}
