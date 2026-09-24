package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type modelMismatchMarkerStub struct {
	AccountRepository
	ids                         []int64
	calls                       int
	expected, actual, requestID string
	err                         error
	ctxErr                      error
	deadline                    bool
	latest                      *Account
	getErr                      error
	onGet                       func()
}

func (r *modelMismatchMarkerStub) MarkAccountModelMismatch(ctx context.Context, id int64, expected, actual, requestID string) error {
	r.calls++
	r.expected, r.actual, r.requestID = expected, actual, requestID
	r.ctxErr = ctx.Err()
	_, r.deadline = ctx.Deadline()
	return r.err
}
func (r *modelMismatchMarkerStub) AccountIPChannelSiblingIDs(context.Context, int64) ([]int64, error) {
	return r.ids, nil
}
func (r *modelMismatchMarkerStub) GetByID(context.Context, int64) (*Account, error) {
	if r.onGet != nil {
		r.onGet()
	}
	return r.latest, r.getErr
}

func TestModelMismatchQuarantineUsesMappedModelAndDetachedContext(t *testing.T) {
	defer ClearPendingAccountModelMismatch(9701, 9702)
	repo := &modelMismatchMarkerStub{ids: []int64{9701, 9702}}
	svc := &OpenAIGatewayService{accountRepo: repo}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result := &OpenAIForwardResult{Model: "public-alias", UpstreamModel: "gpt-6-astra", UpstreamResponseModel: "gpt-5.6-luna", RequestID: "response-1"}
	svc.quarantineForwardResultModelMismatch(ctx, &Account{ID: 9701}, result)
	require.Equal(t, 1, repo.calls)
	require.Equal(t, "gpt-6-astra", repo.expected)
	require.Equal(t, "gpt-5.6-luna", repo.actual)
	require.Equal(t, "response-1", repo.requestID)
	require.NoError(t, repo.ctxErr)
	require.True(t, repo.deadline)
	require.True(t, hasPendingAccountModelMismatch(&Account{ID: 9702}))
	svc.quarantineForwardResultModelMismatch(ctx, &Account{ID: 9701}, result)
	require.Equal(t, 1, repo.calls, "usage fallback must not repeat a completed quarantine")
}

func TestModelMismatchQuarantineFailureBlocksAndRetries(t *testing.T) {
	defer ClearPendingAccountModelMismatch(9703, 9704)
	repo := &modelMismatchMarkerStub{ids: []int64{9703, 9704}, err: errors.New("database unavailable")}
	svc := &OpenAIGatewayService{accountRepo: repo}
	account := &Account{ID: 9703, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true}
	result := &OpenAIForwardResult{Model: "gpt-6-astra", UpstreamResponseModel: "gpt-5.6-luna"}
	svc.quarantineForwardResultModelMismatch(context.Background(), account, result)
	require.False(t, result.modelMismatchQuarantined)
	require.True(t, svc.isOpenAIAccountRequestRuntimeBlocked(account, "gpt-6-astra"))
	require.False(t, (&GatewayService{}).isAccountSchedulableForSelection(&Account{ID: 9704, Status: StatusActive, Schedulable: true}))
	repo.err = nil
	svc.quarantineForwardResultModelMismatch(context.Background(), account, result)
	require.True(t, result.modelMismatchQuarantined)
	require.Equal(t, 2, repo.calls)
	ClearPendingAccountModelMismatch(9703, 9704)
	require.False(t, svc.isOpenAIAccountRequestRuntimeBlocked(account, "gpt-6-astra"))
}

func TestModelMismatchQuarantineIgnoresUnknownAndConfiguredMapping(t *testing.T) {
	repo := &modelMismatchMarkerStub{}
	svc := &OpenAIGatewayService{accountRepo: repo}
	for _, result := range []*OpenAIForwardResult{
		{Model: "gpt-6-astra"},
		{UpstreamResponseModel: "gpt-5.6-luna"},
		{Model: "gpt-6-astra", UpstreamModel: "gpt-5.6-luna", UpstreamResponseModel: "gpt-5.6-luna"},
		{Model: "grok-4.6", UpstreamResponseModel: "grok-4.6-build"},
		{Model: "gpt-5.4", UpstreamResponseModel: "gpt-5.4-2026-03-05"},
	} {
		svc.quarantineForwardResultModelMismatch(context.Background(), &Account{ID: 9705}, result)
	}
	require.Zero(t, repo.calls)
}

func TestModelMismatchPendingObservesRecoveryFromAnotherInstance(t *testing.T) {
	defer ClearPendingAccountModelMismatch(9706, 9707)
	repo := &modelMismatchMarkerStub{ids: []int64{9706, 9707}, latest: &Account{ID: 9706, Schedulable: false, Extra: map[string]any{AccountModelMismatchExtraKey: map[string]any{"expected_model": "gpt-6-astra", "actual_model": "gpt-5.6-luna"}}}}
	require.True(t, quarantineAccountModelMismatch(context.Background(), repo, &Account{ID: 9706}, "gpt-6-astra", "gpt-5.6-luna", "req"))
	value, _ := pendingAccountModelMismatches.Load(int64(9706))
	state, ok := value.(*pendingAccountModelMismatch)
	require.True(t, ok)
	state.checkedAt = time.Time{}
	require.True(t, hasPendingAccountModelMismatch(&Account{ID: 9707, Schedulable: true}), "stale Redis must not reopen the account")
	repo.latest = &Account{ID: 9706, Schedulable: false} // source IP remains independently disabled
	state.checkedAt = time.Time{}
	require.False(t, hasPendingAccountModelMismatch(&Account{ID: 9707}))
	require.False(t, hasPendingAccountModelMismatch(&Account{ID: 9706}))
}

func TestModelMismatchPendingRetriesPersistenceWithoutReopening(t *testing.T) {
	defer ClearPendingAccountModelMismatch(9710, 9711)
	repo := &modelMismatchMarkerStub{err: errors.New("temporary database failure")}
	account := &Account{ID: 9710}
	require.False(t, quarantineAccountModelMismatch(context.Background(), repo, account, "gpt-6-astra", "gpt-5.6-luna", "retry-request"))
	repo.err = nil
	repo.ids = []int64{9710, 9711}
	value, _ := pendingAccountModelMismatches.Load(int64(9710))
	state, ok := value.(*pendingAccountModelMismatch)
	require.True(t, ok)
	state.checkedAt = time.Time{}
	require.True(t, hasPendingAccountModelMismatch(account))
	require.True(t, state.persisted)
	require.Equal(t, 2, repo.calls)
	require.Equal(t, "retry-request", repo.requestID)
	require.True(t, hasPendingAccountModelMismatch(&Account{ID: 9711}))
}

func TestModelMismatchResumeDoesNotClearLaterDetection(t *testing.T) {
	defer ClearPendingAccountModelMismatch(9712)
	old := &pendingAccountModelMismatch{}
	pendingAccountModelMismatches.Store(int64(9712), old)
	clear := CapturePendingAccountModelMismatchReset(9712)
	newer := &pendingAccountModelMismatch{inFlight: true}
	pendingAccountModelMismatches.Store(int64(9712), newer)
	clear()
	current, _ := pendingAccountModelMismatches.Load(int64(9712))
	require.Same(t, newer, current)
	CapturePendingAccountModelMismatchReset(9712)()
	current, _ = pendingAccountModelMismatches.Load(int64(9712))
	require.Same(t, newer, current, "a detection waiting on the recovery transaction must be kept")
}

func TestModelMismatchRecoveryReadCannotPassNewDetection(t *testing.T) {
	defer ClearPendingAccountModelMismatch(9713)
	repo := &modelMismatchMarkerStub{latest: &Account{ID: 9713}}
	old := &pendingAccountModelMismatch{repo: repo, persisted: true, ids: []int64{9713}}
	pendingAccountModelMismatches.Store(int64(9713), old)
	newer := &pendingAccountModelMismatch{inFlight: true}
	repo.onGet = func() { pendingAccountModelMismatches.Store(int64(9713), newer) }
	require.True(t, hasPendingAccountModelMismatch(&Account{ID: 9713}))
	current, _ := pendingAccountModelMismatches.Load(int64(9713))
	require.Same(t, newer, current)
}

func TestModelMismatchWebSocketRechecksDurableQuarantine(t *testing.T) {
	repo := &modelMismatchMarkerStub{latest: &Account{ID: 9708, Schedulable: false, Extra: map[string]any{AccountModelMismatchExtraKey: map[string]any{"expected_model": "gpt-6-astra", "actual_model": "gpt-5.6-luna"}}}}
	svc := &OpenAIGatewayService{accountRepo: repo}
	stale := &Account{ID: 9708, Schedulable: true}
	require.Error(t, svc.checkWebSocketAccountModelMismatch(context.Background(), stale))
	repo.latest = &Account{ID: 9708, Schedulable: true}
	require.NoError(t, svc.checkWebSocketAccountModelMismatch(context.Background(), stale))
	repo.getErr = errors.New("database unavailable")
	require.Error(t, svc.checkWebSocketAccountModelMismatch(context.Background(), stale))
}

func TestUpstreamSnapshotModelAliasesDoNotHideDifferentModels(t *testing.T) {
	for _, pair := range [][2]string{{"gpt-6", "gpt-6-astra"}, {"gpt-5.6", "gpt-5.6-sol"}, {"gpt-5.4", "gpt-5.4-2026-03-05"}, {"claude-3-5-sonnet-latest", "claude-3-5-sonnet-20241022"}, {"claude-sonnet-4", "claude-sonnet-4-20250514"}, {"claude-sonnet-4@20250514", "claude-sonnet-4-20250514"}, {"gemini-2.0-flash", "gemini-2.0-flash-001"}} {
		require.False(t, *upstreamModelMismatch(pair[0], pair[1]), "%v", pair)
	}
	for _, pair := range [][2]string{{"gpt-6-astra", "gpt-5.6-luna"}, {"gpt-6-astra", "gpt-6-astra-mini-2026-09-18"}, {"gpt-5.4-2026-03-05", "gpt-5.4-2026-03-06"}, {"gpt-5.4-2026-03-05", "gpt-5.4-2026-03-05-2026-09-18"}, {"gpt-5.4", "gpt-5.4-2026-02-30"}, {"claude-sonnet-4-20250514", "claude-sonnet-4@20250515"}, {"gemini-2.0-flash-001", "gemini-2.0-flash-002"}} {
		require.True(t, *upstreamModelMismatch(pair[0], pair[1]), "%v", pair)
	}
}
