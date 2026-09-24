package service

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	coderws "github.com/coder/websocket"
)

// AccountModelMismatchMarker quarantines the logical account and every fixed IP
// channel in one transaction. Implementations also invalidate scheduler state.
type AccountModelMismatchMarker interface {
	MarkAccountModelMismatch(ctx context.Context, accountID int64, expectedModel, actualModel, requestID string) error
}

// Policy-aware repositories atomically record evidence and decide whether to
// quarantine. This common path is used by HTTP, WebSocket and account tests.
type AccountModelMismatchRecorder interface {
	RecordAccountModelMismatch(context.Context, int64, string, string, string, ...func([]int64)) (bool, error)
}

func persistAccountModelMismatch(ctx context.Context, repo AccountRepository, id int64, expected, actual, requestID string, gate func([]int64)) (bool, error) {
	if recorder, ok := repo.(AccountModelMismatchRecorder); ok {
		return recorder.RecordAccountModelMismatch(ctx, id, expected, actual, requestID, gate)
	}
	gate(nil)
	marker, ok := repo.(AccountModelMismatchMarker)
	if !ok {
		return false, errors.New("account repository does not support model mismatch quarantine")
	}
	err := marker.MarkAccountModelMismatch(ctx, id, expected, actual, requestID)
	return true, err
}

// Pending marks cover the period before the durable quarantine is visible, and
// remain fail-closed if persistence fails. Administrative recovery clears them.
var pendingAccountModelMismatches sync.Map

type pendingAccountModelMismatch struct {
	mu                          sync.Mutex
	repo                        AccountRepository
	persisted                   bool
	inFlight                    bool
	checkedAt                   time.Time
	ids                         []int64
	expected, actual, requestID string
	credentialExpectation       *modelMismatchCredentialExpectation
	fixedRouteExpectation       *modelMismatchFixedRouteExpectation
}

func (state *pendingAccountModelMismatch) registerQuarantine(ids []int64) {
	state.mu.Lock()
	defer state.mu.Unlock()
	for _, id := range ids {
		known := false
		for _, current := range state.ids {
			known = known || current == id
		}
		if !known {
			state.ids = append(state.ids, id)
		}
	}
	for _, id := range state.ids {
		pendingAccountModelMismatches.Store(id, state)
	}
}

func (state *pendingAccountModelMismatch) clearObservation() {
	state.mu.Lock()
	ids := append([]int64(nil), state.ids...)
	state.mu.Unlock()
	for _, id := range ids {
		pendingAccountModelMismatches.CompareAndDelete(id, state)
	}
}

func ClearPendingAccountModelMismatch(accountIDs ...int64) {
	for _, id := range accountIDs {
		pendingAccountModelMismatches.Delete(id)
	}
}

// CapturePendingAccountModelMismatchReset must be called while administrative
// recovery holds the account family's row locks. Its callback only removes the
// captured generation after commit, preserving any later detection.
func CapturePendingAccountModelMismatchReset(accountIDs ...int64) func() {
	states := make(map[int64]any, len(accountIDs))
	for _, id := range accountIDs {
		if state, ok := pendingAccountModelMismatches.Load(id); ok {
			pending, valid := state.(*pendingAccountModelMismatch)
			if !valid {
				continue
			}
			pending.mu.Lock()
			if !pending.inFlight {
				states[id] = state
			}
			pending.mu.Unlock()
		}
	}
	return func() {
		for id, state := range states {
			pendingAccountModelMismatches.CompareAndDelete(id, state)
		}
	}
}

func hasPendingAccountModelMismatch(account *Account) bool {
	if account == nil {
		return false
	}
	value, blocked := pendingAccountModelMismatches.Load(account.ID)
	if !blocked && account.ParentAccountID != nil {
		value, blocked = pendingAccountModelMismatches.Load(*account.ParentAccountID)
	}
	if !blocked {
		return false
	}
	state, ok := value.(*pendingAccountModelMismatch)
	if !ok {
		return false
	}
	state.mu.Lock()
	if state.inFlight || account.IsModelMismatchQuarantined() || time.Since(state.checkedAt) < 5*time.Second {
		state.mu.Unlock()
		return true
	}
	state.checkedAt = time.Now()
	persisted := state.persisted
	ids := append([]int64(nil), state.ids...)
	if !persisted {
		state.inFlight = true
	}
	state.mu.Unlock()

	// Failed writes are retried at most every five seconds when the scheduler
	// encounters this blocked account. There is no independent worker lifecycle.
	if !persisted {
		ctx, cancel := openAIAccountStateContext(context.Background())
		if state.credentialExpectation != nil {
			ctx = context.WithValue(ctx, modelMismatchCredentialExpectationKey{}, state.credentialExpectation)
		}
		if state.fixedRouteExpectation != nil {
			ctx = context.WithValue(ctx, modelMismatchFixedRouteExpectationKey{}, state.fixedRouteExpectation)
		}
		if siblings, ok := state.repo.(interface {
			AccountIPChannelSiblingIDs(context.Context, int64) ([]int64, error)
		}); ok {
			if family, err := siblings.AccountIPChannelSiblingIDs(ctx, ids[0]); err == nil {
				state.mu.Lock()
				for _, id := range family {
					known := false
					for _, existing := range state.ids {
						known = known || existing == id
					}
					if !known {
						state.ids = append(state.ids, id)
					}
					pendingAccountModelMismatches.LoadOrStore(id, state)
				}
				state.mu.Unlock()
			}
		}
		quarantined, err := persistAccountModelMismatch(ctx, state.repo, ids[0], state.expected, state.actual, state.requestID, state.registerQuarantine)
		cancel()
		state.mu.Lock()
		state.inFlight = false
		state.persisted = err == nil
		state.mu.Unlock()
		if err != nil {
			slog.Error("account_model_mismatch.quarantine_retry_failed", "account_id", ids[0], "error", err)
		} else if !quarantined {
			state.clearObservation()
			return hasPendingAccountModelMismatch(account)
		}
		return true
	}
	// Redis may lag both quarantine and recovery. Query the durable row only
	// while a local quarantine disagrees with a snapshot. Never hold the state
	// mutex across I/O, including the slow or unavailable database case.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	found := false
	for _, id := range ids {
		latest, err := state.repo.GetByID(ctx, id)
		if errors.Is(err, ErrAccountNotFound) {
			continue
		}
		if err != nil || latest == nil || latest.IsModelMismatchQuarantined() {
			return true
		}
		// Restored channels can remain disabled by their own fixed-IP settings.
		found = true
		break
	}
	if !found {
		return true
	}
	for _, id := range ids {
		pendingAccountModelMismatches.CompareAndDelete(id, state)
	}
	// A newer detection may have replaced this generation while the durable
	// read was in flight. Its gate must remain effective for this selection.
	_, blocked = pendingAccountModelMismatches.Load(account.ID)
	if !blocked && account.ParentAccountID != nil {
		_, blocked = pendingAccountModelMismatches.Load(*account.ParentAccountID)
	}
	return blocked
}

// quarantineAccountModelMismatch only consumes structured upstream declarations,
// never generated text. It runs synchronously before asynchronous usage work,
// with a detached deadline so disconnects cannot cancel the safety update.
func quarantineAccountModelMismatch(ctx context.Context, repo AccountRepository, account *Account, expectedModel, actualModel, requestID string) bool {
	expectedModel = strings.TrimSpace(expectedModel)
	actualModel = strings.TrimSpace(actualModel)
	if account == nil || account.ID <= 0 || expectedModel == "" || actualModel == "" {
		return false
	}
	if !IsAccountModelDegradation(expectedModel, actualModel) {
		return false
	}
	if _, exists := ctx.Value(modelMismatchCredentialExpectationKey{}).(*modelMismatchCredentialExpectation); !exists {
		ctx = WithModelMismatchCredentialExpectation(ctx, account)
	}
	_, ok := repo.(AccountModelMismatchMarker)
	if !ok {
		slog.Error("account_model_mismatch.quarantine_unavailable", "account_id", account.ID,
			"expected_model", expectedModel, "actual_model", actualModel, "request_id", requestID)
		return false
	}
	stateCtx, cancel := openAIAccountStateContext(ctx)
	defer cancel()
	state := &pendingAccountModelMismatch{repo: repo, ids: []int64{account.ID}, inFlight: true, expected: expectedModel, actual: actualModel, requestID: strings.TrimSpace(requestID), checkedAt: time.Now()}
	state.credentialExpectation, _ = ctx.Value(modelMismatchCredentialExpectationKey{}).(*modelMismatchCredentialExpectation)
	state.fixedRouteExpectation, _ = ctx.Value(modelMismatchFixedRouteExpectationKey{}).(*modelMismatchFixedRouteExpectation)
	defer func() {
		state.mu.Lock()
		state.inFlight = false
		state.mu.Unlock()
	}()
	var familyIDs []int64
	if siblings, ok := repo.(interface {
		AccountIPChannelSiblingIDs(context.Context, int64) ([]int64, error)
	}); ok {
		family, err := siblings.AccountIPChannelSiblingIDs(stateCtx, account.ID)
		if err != nil {
			slog.Error("account_model_mismatch.family_lookup_failed", "account_id", account.ID, "error", err)
		} else {
			familyIDs = family
		}
	}
	if _, policyAware := repo.(AccountModelMismatchRecorder); !policyAware {
		state.registerQuarantine(familyIDs)
	}
	quarantined, err := persistAccountModelMismatch(stateCtx, repo, account.ID, expectedModel, actualModel, strings.TrimSpace(requestID), state.registerQuarantine)
	if err != nil {
		// A failed durable write still needs a fail-closed local gate. A policy
		// aware repository intentionally does not invoke its gate for an
		// observation-only result, so only the error path installs it here.
		state.registerQuarantine(familyIDs)
		slog.Error("account_model_mismatch.quarantine_failed", "account_id", account.ID,
			"expected_model", expectedModel, "actual_model", actualModel, "request_id", requestID, "error", err)
		return false
	}
	if !quarantined {
		state.clearObservation()
		// The result was handled, so asynchronous usage logging must not record
		// it a second time. Detection evidence remains in the durable account.
		return true
	}
	// Cache refresh is best effort; retain the local gate until an explicit
	// administrative recovery instead of reopening on a stale Redis snapshot.
	state.mu.Lock()
	state.persisted = true
	state.checkedAt = time.Now()
	state.mu.Unlock()
	return true
}

func quarantineForwardResultModelMismatch(ctx context.Context, repo AccountRepository, account *Account, result *ForwardResult) {
	if result == nil || result.modelMismatchQuarantined {
		return
	}
	result.modelMismatchQuarantined = quarantineAccountModelMismatch(ctx, repo, account,
		upstreamSentModel(result.Model, result.UpstreamModel), result.UpstreamResponseModel, result.RequestID)
}

func (s *OpenAIGatewayService) quarantineForwardResultModelMismatch(ctx context.Context, account *Account, result *OpenAIForwardResult) {
	if s == nil || result == nil || result.modelMismatchQuarantined {
		return
	}
	result.modelMismatchQuarantined = quarantineAccountModelMismatch(ctx, s.accountRepo, account,
		upstreamSentModel(result.Model, result.UpstreamModel), result.UpstreamResponseModel, result.RequestID)
}

// Long-lived sockets retain their original Account pointer. Recheck durable
// quarantine before each subsequent turn so an old connection cannot bypass a
// disable performed by another request or another application instance.
func (s *OpenAIGatewayService) checkWebSocketAccountModelMismatch(ctx context.Context, account *Account) error {
	blocked := account != nil && (hasPendingAccountModelMismatch(account) || account.IsModelMismatchQuarantined())
	if !blocked && s != nil && account != nil {
		if _, supported := s.accountRepo.(AccountModelMismatchMarker); supported {
			latest, err := s.accountRepo.GetByID(ctx, account.ID)
			if err != nil {
				return NewOpenAIWSClientCloseError(coderws.StatusTryAgainLater, "account availability could not be verified, please reconnect", err)
			}
			blocked = latest == nil || latest.IsModelMismatchQuarantined()
		}
	}
	if blocked {
		return NewOpenAIWSClientCloseError(coderws.StatusTryAgainLater, "account is no longer eligible for this connection, please reconnect", nil)
	}
	return nil
}
