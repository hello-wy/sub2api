//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestClassifyOpenAIRiskControlState(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		status    string
		length    *int
		suspected bool
	}{
		{name: "missing", status: OpenAIRiskControlStatusMissing},
		{name: "normal 292", value: strings.Repeat("x", 292), status: OpenAIRiskControlStatusNormal, length: riskControlIntPtr(292)},
		{name: "normal 332 trimmed", value: " " + strings.Repeat("x", 332) + " ", status: OpenAIRiskControlStatusNormal, length: riskControlIntPtr(332)},
		{name: "suspected 312", value: strings.Repeat("x", 312), status: OpenAIRiskControlStatusSuspected, length: riskControlIntPtr(312), suspected: true},
		{name: "abnormal", value: strings.Repeat("x", 311), status: OpenAIRiskControlStatusAbnormal, length: riskControlIntPtr(311)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, length, suspected, _ := classifyOpenAIRiskControlState(tt.value)
			require.Equal(t, tt.status, status)
			require.Equal(t, tt.length, length)
			require.Equal(t, tt.suspected, suspected)
		})
	}
}

func TestProbeOpenAIRiskControlPersistsOnlyDerivedObservation(t *testing.T) {
	state := strings.Repeat("s", 312)
	resp := newJSONResponse(http.StatusOK, "stream body must not be parsed")
	resp.Header.Set(openAICodexTurnStateHeader, state)

	account := &Account{
		ID:          42,
		Name:        "oauth",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 2,
		Credentials: map[string]any{"access_token": "secret-token", "chatgpt_account_id": "chatgpt-42"},
	}
	repo := &openAIAccountTestRepo{mockAccountRepoForGemini: mockAccountRepoForGemini{
		accountsByID: map[int64]*Account{account.ID: account},
	}}
	upstream := &queuedHTTPUpstream{responses: []*http.Response{resp}}
	svc := &AccountTestService{accountRepo: repo, httpUpstream: upstream}

	result, err := svc.ProbeOpenAIRiskControl(context.Background(), account.ID)
	require.NoError(t, err)
	require.Equal(t, OpenAIRiskControlStatusSuspected, result.Status)
	require.True(t, result.Suspected)
	require.Equal(t, 312, *result.StateLength)
	require.Equal(t, http.StatusOK, result.HTTPStatus)

	require.Len(t, upstream.requests, 1)
	req := upstream.requests[0]
	require.Equal(t, chatgptCodexAPIURL, req.URL.String())
	require.Equal(t, "Bearer secret-token", req.Header.Get("Authorization"))
	require.Equal(t, "chatgpt-42", req.Header.Get("chatgpt-account-id"))
	body, readErr := io.ReadAll(req.Body)
	require.NoError(t, readErr)
	require.Contains(t, string(body), `"text":"ping"`)
	require.Contains(t, string(body), `"stream":true`)

	persisted, ok := repo.updatedExtra[OpenAIRiskControlExtraKey].(*OpenAIRiskControlSnapshot)
	require.True(t, ok)
	require.Equal(t, result, persisted)
	require.NotContains(t, repo.updatedExtra, openAICodexTurnStateHeader)
	require.NotContains(t, string(body), state)
}

func TestProbeOpenAIRiskControlRejectsIneligibleAccountsBeforeUpstream(t *testing.T) {
	tests := []struct {
		name    string
		account *Account
	}{
		{name: "API key", account: &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}},
		{name: "other platform", account: &Account{ID: 2, Platform: PlatformGemini, Type: AccountTypeOAuth}},
		{name: "shadow", account: &Account{ID: 3, Platform: PlatformOpenAI, Type: AccountTypeOAuth, ParentAccountID: riskControlInt64Ptr(9)}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &openAIAccountTestRepo{mockAccountRepoForGemini: mockAccountRepoForGemini{
				accountsByID: map[int64]*Account{tt.account.ID: tt.account},
			}}
			upstream := &queuedHTTPUpstream{}
			svc := &AccountTestService{accountRepo: repo, httpUpstream: upstream}

			_, err := svc.ProbeOpenAIRiskControl(context.Background(), tt.account.ID)
			require.ErrorIs(t, err, ErrOpenAIRiskControlUnsupported)
			require.Empty(t, upstream.requests)
		})
	}
}

func TestProbeOpenAIRiskControlRejectsEmptyUpstreamResponse(t *testing.T) {
	account := &Account{
		ID:          42,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Credentials: map[string]any{"access_token": "secret-token"},
	}
	repo := &openAIAccountTestRepo{mockAccountRepoForGemini: mockAccountRepoForGemini{
		accountsByID: map[int64]*Account{account.ID: account},
	}}
	upstream := &queuedHTTPUpstream{responses: []*http.Response{nil}}
	svc := &AccountTestService{accountRepo: repo, httpUpstream: upstream}

	result, err := svc.ProbeOpenAIRiskControl(context.Background(), account.ID)
	require.Nil(t, result)
	require.ErrorContains(t, err, "upstream returned no response")
	require.Nil(t, repo.updatedExtra)
}

func TestOpenAIRiskControlAutoProbeAfterRepeated429And529(t *testing.T) {
	now := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	called := make(chan int64, 1)
	service := &OpenAIRiskControlService{
		inFlight:     make(map[int64]struct{}),
		errorWindows: make(map[int64]openAIRiskControlErrorWindowState),
		now:          func() time.Time { return now },
		probe: func(_ context.Context, accountID int64) (*OpenAIRiskControlSnapshot, error) {
			called <- accountID
			return &OpenAIRiskControlSnapshot{Status: OpenAIRiskControlStatusNormal}, nil
		},
	}
	account := &Account{ID: 71, Platform: PlatformOpenAI, Type: AccountTypeOAuth}

	require.False(t, service.ObserveOpenAIRiskControlError(account, http.StatusTooManyRequests))
	require.False(t, service.ObserveOpenAIRiskControlError(account, 529))
	require.True(t, service.ObserveOpenAIRiskControlError(account, http.StatusTooManyRequests))
	require.Equal(t, account.ID, <-called)

	// The threshold starts a cooldown, so further incidents cannot fan out
	// automatic probes during the same upstream outage.
	require.False(t, service.ObserveOpenAIRiskControlError(account, 529))
}

func TestOpenAIRiskControlRepeatedErrorWindowExpires(t *testing.T) {
	now := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	called := make(chan struct{}, 1)
	service := &OpenAIRiskControlService{
		inFlight:     make(map[int64]struct{}),
		errorWindows: make(map[int64]openAIRiskControlErrorWindowState),
		now:          func() time.Time { return now },
		probe: func(context.Context, int64) (*OpenAIRiskControlSnapshot, error) {
			called <- struct{}{}
			return &OpenAIRiskControlSnapshot{Status: OpenAIRiskControlStatusNormal}, nil
		},
	}
	account := &Account{ID: 72, Platform: PlatformOpenAI, Type: AccountTypeOAuth}

	require.False(t, service.ObserveOpenAIRiskControlError(account, 529))
	require.False(t, service.ObserveOpenAIRiskControlError(account, 529))
	now = now.Add(openAIRiskControlErrorWindow)
	require.False(t, service.ObserveOpenAIRiskControlError(account, 529), "expired errors must not count toward the new window")
	require.False(t, service.ObserveOpenAIRiskControlError(account, 529))
	require.True(t, service.ObserveOpenAIRiskControlError(account, 529))
	<-called
}

func TestOpenAIRiskControlAutomaticProbeCoalescesConcurrentTriggers(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int64
	service := &OpenAIRiskControlService{
		inFlight:     make(map[int64]struct{}),
		errorWindows: make(map[int64]openAIRiskControlErrorWindowState),
		now:          time.Now,
		probe: func(context.Context, int64) (*OpenAIRiskControlSnapshot, error) {
			if calls.Add(1) == 1 {
				close(started)
			}
			<-release
			return &OpenAIRiskControlSnapshot{Status: OpenAIRiskControlStatusNormal}, nil
		},
	}

	require.True(t, service.ScheduleOpenAIRiskControlProbe(73, openAIRiskControlTriggerAccountCreated))
	<-started

	var scheduled atomic.Int64
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if service.ScheduleOpenAIRiskControlProbe(73, openAIRiskControlTriggerRepeatedErrors) {
				scheduled.Add(1)
			}
		}()
	}
	wg.Wait()
	require.Zero(t, scheduled.Load())
	close(release)
	require.Eventually(t, func() bool {
		service.mu.Lock()
		defer service.mu.Unlock()
		return len(service.inFlight) == 0
	}, time.Second, time.Millisecond)
	require.Equal(t, int64(1), calls.Load())
}

func TestOpenAIRiskControlErrorObservationEligibility(t *testing.T) {
	service := &OpenAIRiskControlService{
		inFlight:     make(map[int64]struct{}),
		errorWindows: make(map[int64]openAIRiskControlErrorWindowState),
		now:          time.Now,
		probe: func(context.Context, int64) (*OpenAIRiskControlSnapshot, error) {
			t.Fatal("ineligible error must not schedule a probe")
			return nil, nil
		},
	}
	parentID := int64(9)
	tests := []struct {
		name    string
		account *Account
		status  int
	}{
		{name: "API key", account: &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}, status: 429},
		{name: "other platform", account: &Account{ID: 2, Platform: PlatformGemini, Type: AccountTypeOAuth}, status: 429},
		{name: "shadow", account: &Account{ID: 3, Platform: PlatformOpenAI, Type: AccountTypeOAuth, ParentAccountID: &parentID}, status: 529},
		{name: "other status", account: &Account{ID: 4, Platform: PlatformOpenAI, Type: AccountTypeOAuth}, status: 500},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for range openAIRiskControlErrorThreshold {
				require.False(t, service.ObserveOpenAIRiskControlError(tt.account, tt.status))
			}
		})
	}
}

func riskControlIntPtr(value int) *int { return &value }

func riskControlInt64Ptr(value int64) *int64 { return &value }
