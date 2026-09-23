//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type codexProbeErrorReader struct {
	io.Reader
	read int
}

func (r *codexProbeErrorReader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	r.read += n
	return n, err
}

func TestCodexTicketProbeRejectionsMaintainSharedCredentialHealth(t *testing.T) {
	for _, tc := range []struct {
		name      string
		status    int
		code      string
		permanent bool
	}{
		{"expired", 401, "token_expired", false}, {"revoked", 401, "token_revoked", true}, {"invalid_grant", 401, "invalid_grant", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := stateTicketTestAccount(7981)
			s, _ := stateTicketTestService(t, a)
			health := &openAIHealthRepoStub{}
			s.rateLimitService = NewRateLimitService(health, nil, &config.Config{}, nil, nil)
			body := &codexProbeErrorReader{Reader: strings.NewReader(`{"error":{"code":"` + tc.code + `","message":"rejected"}}` + strings.Repeat(" ", 100000))}
			s.httpUpstream = &stateTicketProbeUpstream{response: &http.Response{StatusCode: tc.status, Header: http.Header{"Retry-After": []string{"90"}}, Body: io.NopCloser(body)}}
			_, status, err := s.fireCodexAccountTicketProbe(context.Background(), a, "actual-probe-token", openAICodexTicketDefaultModel, a.Proxy.URL(), stateTicketVerified(a, time.Now()).State, time.Second)
			require.Error(t, err)
			require.Equal(t, tc.status, status)
			require.NotNil(t, health.expected)
			require.LessOrEqual(t, body.read, 64*1024)
			if tc.status == 401 {
				require.Equal(t, "actual-probe-token", health.expected.GetCredential("access_token"))
				require.Equal(t, tc.permanent, health.mutation.Permanent)
				require.NotNil(t, health.mutation.AuthCooldownUntil)
			} else {
				require.NotNil(t, health.mutation.RateLimitUntil)
				require.Greater(t, time.Until(*health.mutation.RateLimitUntil), 89*time.Second)
			}
		})
	}
}

func TestCodexTicketJobsRespectSharedHealthBeforeSending(t *testing.T) {
	for _, reason := range []string{"revoked", "authentication", "overload"} {
		t.Run(reason, func(t *testing.T) {
			a := stateTicketTestAccount(7982)
			until := time.Now().Add(time.Hour)
			switch reason {
			case "revoked":
				a.Status = StatusError
			case "authentication":
				a.TempUnschedulableUntil = &until
				a.TempUnschedulableReason = "OAuth 401: refresh required"
			case "overload":
				a.OverloadUntil = &until
			}
			s, _ := stateTicketTestService(t, a)
			s.openaiCodexTicketProbe = func(context.Context, *Account, string, string, string, string, time.Duration) (string, int, error) {
				t.Error("probe started during shared cooldown")
				return "", 200, nil
			}
			require.Nil(t, s.startCodexAccountTicketJob(context.Background(), a.ID, false))
			_, err := s.HarvestCodexAccountTicket(context.Background(), a.ID)
			require.Error(t, err)
		})
	}
}
