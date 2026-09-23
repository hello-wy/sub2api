package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCodexTicketDiagnosticsAreFiniteAndDoNotLeakSourceErrors(t *testing.T) {
	validState := "gAAAAA" + strings.Repeat("A", 286)
	for _, tc := range []struct {
		name, stage string
		status      int
		err         error
		state       string
		target      int
		want        string
	}{
		{"http", "dynamic", 503, errors.New("https://user:secret@proxy.invalid response private"), "", 292, "reason=http_rejected"},
		{"deadline", "dynamic", 0, &url.Error{Op: "POST", URL: "https://user:secret@proxy.invalid", Err: context.DeadlineExceeded}, "", 292, "reason=timeout"},
		{"incomplete", "dynamic", 200, errors.New("response did not complete"), "", 292, "reason=response_incomplete"},
		{"malformed", "fixed", 200, errors.New("invalid completion"), "", 0, "reason=response_invalid"},
		{"missing model", "fixed", 200, errors.New("completed response has no model"), "", 0, "reason=model_missing"},
		{"missing state", "dynamic", 200, nil, "", 292, "reason=state_missing"},
		{"invalid state", "dynamic", 200, nil, "secret value", 292, "reason=state_invalid"},
		{"wrong length", "dynamic", 200, nil, validState, 332, "reason=state_length_mismatch"},
		{"312", "fixed_replacement", 200, nil, "gAAAAA" + strings.Repeat("A", 306), 0, "reason=state_312"},
		{"nested401", "fixed_replacement", 401, &codexTicketStreamError{status: 401, payload: []byte("secret body")}, "", 0, "reason=stream_rejected"},
		{"unknown", "dynamic", 200, errors.New("secret arbitrary upstream message"), "", 292, "reason=transport_or_verification_error"},
		{"model", "fixed", 200, &codexTicketModelMismatchError{actual: "gpt-5.6-luna"}, "", 0, "actual_model=gpt-5.6-luna"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := codexTicketProbeFailure(tc.stage, tc.status, tc.err, tc.state, tc.target)
			require.Contains(t, got, "stage="+tc.stage)
			require.Contains(t, got, fmt.Sprintf("status=%d", tc.status))
			require.Contains(t, got, tc.want)
			for _, secret := range []string{"secret", "proxy.invalid", "https://", validState, "secret body"} {
				require.NotContains(t, got, secret)
			}
		})
	}
	for _, actual := range []string{"https://secret.invalid", "gpt-6\nsecret", strings.Repeat("x", 101), "model-429"} {
		got := codexTicketProbeFailure("fixed", 200, &codexTicketModelMismatchError{actual: actual}, "", 0)
		require.NotContains(t, got, "actual_model=")
		require.False(t, codexTicket429Observation(got))
	}
	diagnostic := codexTicketProbeFailure("dynamic", 200, nil, "gAAAAA"+strings.Repeat("A", 423), 292)
	require.Contains(t, diagnostic, "state_length=other")
	require.NotContains(t, diagnostic, "429", "old Redis substring classifiers must not mistake a STATE length for an HTTP status")
}

func TestCodexTicketDiagnosticsPreserveFixedFailureDuringDynamicFallback(t *testing.T) {
	s, _, account, _, job := stateTicketCandidateFixture(t)
	calls := 0
	s.openaiCodexTicketProbe = func(_ context.Context, _ *Account, _, _, _ string, state string, _ time.Duration) (string, int, error) {
		calls++
		if state != "" {
			return "", 200, &codexTicketModelMismatchError{actual: "safe-other-model"}
		}
		return "", 401, errors.New("private body must not escape")
	}
	s.runCodexAccountTicketJob(context.Background(), account.ID, job)
	require.Equal(t, 2, calls)
	require.Contains(t, job.lastError, "stage=dynamic status=401 reason=http_rejected")
	require.Contains(t, job.lastError, "prior_fixed={STATE stage=fixed status=200 reason=model_mismatch")
	require.Contains(t, job.lastError, "actual_model=safe-other-model}")
	require.NotContains(t, job.lastError, "private")
	require.False(t, codexTicket429Observation(job.lastError))
	require.True(t, codexTicket429Observation(codexTicketProbeFailure("dynamic", http.StatusTooManyRequests, nil, "", 292)))
}

func TestCodexTicketDiagnosticsRetainHeaderLengthWhenCompletedModelDiffers(t *testing.T) {
	a := stateTicketTestAccount(8510)
	s, _ := stateTicketTestService(t, a)
	response := &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(candidateCompletedBody("gpt-5.6-luna")))}
	response.Header.Set(openAICodexTurnStateHeader, "gAAAAA"+strings.Repeat("A", 306))
	state, status, err := s.validateCodexTicketProbeResponse(context.Background(), a, a.GetOpenAIAccessToken(), "gpt-6-astra", "", response)
	require.Error(t, err)
	require.Empty(t, state, "failed validation must not return a usable STATE")
	diagnostic := codexTicketProbeFailure("dynamic", status, err, state, 292)
	require.Contains(t, diagnostic, "status=200 reason=model_mismatch state_length=312 transport_error=false")
	require.Contains(t, diagnostic, "actual_model=gpt-5.6-luna")
	require.NotContains(t, diagnostic, "gAAAAA")
}

type codexTicketTimeoutReader struct{}

func (codexTicketTimeoutReader) Read([]byte) (int, error) { return 0, context.DeadlineExceeded }

func TestCodexTicketIncompleteReaderKeepsTimeoutClassification(t *testing.T) {
	err := validateCodexTicketCompletedModel(codexTicketTimeoutReader{}, "gpt-6-astra")
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Contains(t, codexTicketProbeFailure("fixed", 200, err, "", 0), "reason=timeout")
}
