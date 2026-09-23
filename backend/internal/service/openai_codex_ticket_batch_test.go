package service

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	apperrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type codexTicketBatchRepo struct {
	*stateTicketTestRepo
	groups map[int64][]AccountIPChannel
	err    error
}

func (r *codexTicketBatchRepo) GetAccountIPChannels(context.Context, []int64) (map[int64][]AccountIPChannel, error) {
	return r.groups, r.err
}

func codexTicketBatchTestService(t *testing.T, ids ...int64) (*OpenAIGatewayService, *codexTicketBatchRepo) {
	t.Helper()
	s, original := stateTicketTestService(t, stateTicketTestAccount(ids[0]))
	for _, id := range ids[1:] {
		original.accounts[id] = stateTicketTestAccount(id)
	}
	repo := &codexTicketBatchRepo{stateTicketTestRepo: original, groups: map[int64][]AccountIPChannel{}}
	s.accountRepo = repo
	return s, repo
}

func codexTicketBatchEnable(ids ...int64) CodexAccountTicketBatchUpdate {
	return CodexAccountTicketBatchUpdate{AccountIDs: ids, Enabled: true, TicketPlan: "pro", Model: openAICodexTicketDefaultModel}
}

func TestCodexTicketBatchExpandsLogicalChannelsDeduplicatesAndPreservesDisableSettings(t *testing.T) {
	s, repo := codexTicketBatchTestService(t, 1, 2, 3)
	// The retired logical root 1 is absent from the repository's active routes.
	repo.groups[1] = []AccountIPChannel{{Account: repo.accounts[2], LogicalAccountID: 1}, {Account: repo.accounts[3], LogicalAccountID: 1}}
	repo.groups[2] = repo.groups[1]
	repo.accounts[2].Extra[codexAccountTicketConfigKey] = codexAccountTicketConfig{Enabled: true, Model: openAICodexTicketDefaultSolModel, TicketPlan: "team", Revision: "old-team"}
	repo.accounts[2].Schedulable = false
	repo.accounts[2].Extra[AccountModelMismatchExtraKey] = map[string]any{"expected_model": "gpt-6-astra", "actual_model": "gpt-5.6-luna"}
	s.cfg.Gateway.OpenAICodexTicket.Enabled = false
	s.cfg.Gateway.OpenAICodexTicket.HarvestProxyURL = ""
	var probes atomic.Int32
	s.openaiCodexTicketProbe = func(context.Context, *Account, string, string, string, string, time.Duration) (string, int, error) {
		probes.Add(1)
		return "", 200, nil
	}
	result, err := s.BatchCodexAccountTickets(context.Background(), CodexAccountTicketBatchUpdate{AccountIDs: []int64{1, 2, 1}, Enabled: false, TicketPlan: "pro", Model: openAICodexTicketDefaultModel, Harvest: true})
	require.NoError(t, err)
	require.Equal(t, 2, result.SelectedAccounts)
	require.Equal(t, 2, result.TotalChannels)
	require.Equal(t, 2, result.Saved)
	require.Equal(t, int64(2), result.Results[0].AccountID)
	require.Equal(t, int64(1), result.Results[0].LogicalAccountID)
	require.Equal(t, "saved", result.Results[0].Outcome)
	require.Zero(t, probes.Load())
	root, _ := repo.GetByID(context.Background(), 1)
	require.True(t, codexAccountTicketConfigOf(root).Enabled)
	live, _ := repo.GetByID(context.Background(), 2)
	ac := codexAccountTicketConfigOf(live)
	require.False(t, ac.Enabled)
	require.Equal(t, "team", ac.TicketPlan)
	require.Equal(t, openAICodexTicketDefaultSolModel, ac.Model)
	require.True(t, live.HasModelMismatch())
	require.False(t, live.Schedulable)
}

func TestCodexTicketBatchRejectsInvalidPrerequisitesBeforeAnyWrite(t *testing.T) {
	for _, scenario := range []string{"global_disabled", "missing_pool", "missing_plan", "invalid_model", "empty_selection", "invalid_id", "too_many_accounts", "settings_read_failure", "channel_read_failure", "too_many_channels"} {
		t.Run(scenario, func(t *testing.T) {
			s, repo := codexTicketBatchTestService(t, 1)
			input := codexTicketBatchEnable(1)
			switch scenario {
			case "global_disabled":
				s.cfg.Gateway.OpenAICodexTicket.Enabled = false
			case "missing_pool":
				s.cfg.Gateway.OpenAICodexTicket.HarvestProxyURL = ""
			case "missing_plan":
				input.TicketPlan = ""
			case "invalid_model":
				input.Model = "gpt-5.6-luna"
			case "empty_selection":
				input.AccountIDs = nil
			case "invalid_id":
				input.AccountIDs = []int64{1, -2}
			case "too_many_accounts":
				input.AccountIDs = make([]int64, CodexTicketBatchMaxAccounts+1)
			case "settings_read_failure":
				s.settingService = NewSettingService(&codexTicketSettingRepo{readErr: errors.New("private password in SQL")}, nil)
			case "channel_read_failure":
				repo.err = errors.New("proxy password should never be exposed")
			case "too_many_channels":
				for id := int64(1); id <= CodexTicketBatchMaxChannels+1; id++ {
					repo.groups[1] = append(repo.groups[1], AccountIPChannel{Account: &Account{ID: id}, LogicalAccountID: 1})
				}
			}
			_, err := s.BatchCodexAccountTickets(context.Background(), input)
			require.Error(t, err)
			require.Zero(t, repo.mutations)
			require.NotContains(t, err.Error(), "password")
		})
	}
}

func TestCodexTicketBatchSkipsUnsupportedChannelsAndLeavesTheirSettingsUnchanged(t *testing.T) {
	s, repo := codexTicketBatchTestService(t, 1, 2, 3, 4)
	repo.accounts[2].ProxyID, repo.accounts[2].Proxy = nil, nil
	repo.accounts[3].Platform = PlatformAnthropic
	repo.accounts[4].Status = StatusDisabled
	input := codexTicketBatchEnable(1, 2, 3, 4, 99)
	input.TicketPlan = "team"
	result, err := s.BatchCodexAccountTickets(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, 1, result.Saved)
	require.Equal(t, 4, result.Skipped)
	require.Zero(t, result.Failed)
	require.Equal(t, 1, repo.writes)
	require.Equal(t, "team", result.Results[0].Status.TicketPlan)
	require.Empty(t, s.openaiCodexAccountJobs, "save-only batch must not immediately start acquisition")
	for _, id := range []int64{2, 3, 4} {
		live, _ := repo.GetByID(context.Background(), id)
		require.Equal(t, "pro", codexAccountTicketConfigOf(live).TicketPlan)
	}
}

func TestCodexTicketBatchAcquisitionIsBoundedAndWaitingDoesNotMeanFailure(t *testing.T) {
	s, _ := codexTicketBatchTestService(t, 1, 2, 3)
	s.cfg.Gateway.OpenAICodexTicket.MaxConcurrentHarvests = 2
	var probes atomic.Int32
	s.openaiCodexTicketProbe = func(ctx context.Context, _ *Account, _, _, _, _ string, _ time.Duration) (string, int, error) {
		probes.Add(1)
		<-ctx.Done()
		return "", 0, ctx.Err()
	}
	input := codexTicketBatchEnable(1, 2, 3)
	input.Harvest = true
	result, err := s.BatchCodexAccountTickets(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, 3, result.Saved)
	require.Equal(t, 2, result.Harvesting)
	require.Equal(t, 1, result.Waiting)
	require.Zero(t, result.Failed)
	require.Equal(t, "waiting", result.Results[2].Outcome)
	for _, item := range result.Results {
		require.False(t, item.Status.TicketUsable)
	}
	// Repeating an identical action reuses running jobs, without resetting them.
	second, err := s.BatchCodexAccountTickets(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, 2, second.Harvesting)
	require.Equal(t, 1, second.Waiting)
	s.StopOpenAICodexTicketHarvester()
	require.LessOrEqual(t, probes.Load(), int32(2))
}

func TestCodexTicketBatchKeepsUpstreamRejectionCooldown(t *testing.T) {
	s, _ := codexTicketBatchTestService(t, 1)
	var probes atomic.Int32
	s.openaiCodexTicketProbe = func(context.Context, *Account, string, string, string, string, time.Duration) (string, int, error) {
		probes.Add(1)
		return "", http.StatusTooManyRequests, errors.New("rate limit")
	}
	waitStateTicketJob(t, s.startCodexAccountTicketJob(context.Background(), 1, true))
	input := codexTicketBatchEnable(1)
	input.Harvest = true
	result, err := s.BatchCodexAccountTickets(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, 1, result.Saved)
	require.Equal(t, 1, result.Cooldown)
	require.Zero(t, result.Failed)
	require.Equal(t, int32(1), probes.Load())
	require.NotNil(t, result.Results[0].Status.RetryAfter)
}

func TestCodexTicketBatchHidesInternalErrorDetails(t *testing.T) {
	s, repo := codexTicketBatchTestService(t, 1)
	repo.failMutation = true
	result, err := s.BatchCodexAccountTickets(context.Background(), codexTicketBatchEnable(1))
	require.NoError(t, err)
	require.Equal(t, 1, result.Failed)
	require.Zero(t, result.Saved)
	require.NotContains(t, result.Results[0].Message, "simulated")
	require.NotContains(t, codexTicketBatchPublicError(errors.New("http://name:secret@proxy.invalid")), "secret")
	require.Equal(t, "Safe public message", codexTicketBatchPublicError(apperrors.BadRequest("PUBLIC", "Safe public message")))
	require.False(t, strings.Contains(result.Results[0].Message, "proxy.invalid"))
}
