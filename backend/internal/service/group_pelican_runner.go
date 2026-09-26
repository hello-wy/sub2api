package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

const groupPelicanMaxAttempts = 3

func (s *ScheduledTestRunnerService) initRuntime() {
	s.runtimeOnce.Do(func() {
		s.runContext, s.cancelRuns = context.WithCancel(context.Background())
		s.workers = make(chan struct{}, scheduledTestDefaultMaxWorkers)
	})
}

// Claim synchronously before acknowledging the request. No cron tick or detached
// request context is involved, and scheduled/manual runs share the same capacity.
func (s *ScheduledTestRunnerService) startGroupPlanNow(ctx context.Context, plan *ScheduledTestPlan) error {
	s.initRuntime()
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if s.stopping {
		return fmt.Errorf("test runner is shutting down")
	}
	select {
	case s.workers <- struct{}{}:
	default:
		return fmt.Errorf("test workers are busy; try again shortly")
	}
	started := false
	defer func() {
		if !started {
			<-s.workers
		}
	}()
	now := time.Now()
	next, err := nextPlanRun(plan, now)
	if err != nil {
		return err
	}
	until := now.Add(15 * time.Minute).Truncate(time.Microsecond)
	claimed, err := s.planRepo.ClaimPelican(ctx, plan, now, until, next, true)
	if err != nil {
		return err
	}
	if !claimed {
		return fmt.Errorf("plan was paused or changed, or this group already has a running test")
	}
	s.manualWG.Add(1)
	started = true
	go func() {
		defer s.manualWG.Done()
		defer func() { <-s.workers }()
		s.runClaimedGroupPelican(s.runContext, plan, now, until)
	}()
	return nil
}

func (s *ScheduledTestRunnerService) groupRetryDelay(attempt int) time.Duration {
	if s.retryDelay != nil {
		return s.retryDelay(attempt)
	}
	if attempt == 1 {
		return 5 * time.Second
	}
	return 15 * time.Second
}

func (s *ScheduledTestRunnerService) saveGroupExecution(planID int64, until time.Time, state *ScheduledTestExecution) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := s.planRepo.UpdatePelicanExecution(ctx, planID, until, state)
	if err != nil {
		logger.LegacyPrintf("service.scheduled_test_runner", "group plan=%d progress update failed: %v", planID, err)
	}
	return err
}

// Retry only failed outputs, preserving successful parallel outputs and every
// attempt's history. The lease spans all retries; progress is shared by replicas.
func (s *ScheduledTestRunnerService) runClaimedGroupPelican(ctx context.Context, plan *ScheduledTestPlan, started, until time.Time) {
	timeoutCtx, stopTimeout := context.WithTimeout(ctx, 10*time.Minute)
	defer stopTimeout()
	runCtx, cancelRun := context.WithCancelCause(timeoutCtx)
	defer cancelRun(nil)
	stopWatching := s.watchGroupCancellation(runCtx, plan.ID, until, cancelRun)
	defer stopWatching()
	state := &ScheduledTestExecution{Status: "running", Phase: "waiting", Attempt: 1, MaxAttempts: groupPelicanMaxAttempts,
		Total: plan.PelicanConfig.ParallelCount, StartedAt: started}
	defer func() {
		finished := time.Now()
		state.FinishedAt, state.RetryAt, state.Phase = &finished, nil, ""
		if state.Status == "running" || state.Status == "retrying" {
			state.Status = "failed"
			if ctx.Err() != nil {
				state.Status = "interrupted"
			}
		}
		if errors.Is(context.Cause(runCtx), errGroupPelicanCancelled) {
			state.Status, state.LastError, state.CancelRequested = "interrupted", errGroupPelicanCancelled.Error(), true
		}
		_ = s.saveGroupExecution(plan.ID, until, state)
		saveCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		if err := s.planRepo.FinishPelican(saveCtx, plan.ID, until, finished); err != nil {
			logger.LegacyPrintf("service.scheduled_test_runner", "group plan=%d finish failed: %v", plan.ID, err)
		}
	}()
	if err := s.saveGroupExecution(plan.ID, until, state); err != nil {
		state.LastError = "Unable to persist execution status"
		return
	}
	pending := state.Total
	for attempt := 1; attempt <= groupPelicanMaxAttempts; attempt++ {
		if runCtx.Err() != nil {
			return
		}
		state.Status, state.Attempt, state.RetryAt, state.Phase = "running", attempt, nil, "waiting"
		state.Completed, state.Failed = state.Succeeded, 0
		if err := s.saveGroupExecution(plan.ID, until, state); err != nil {
			return
		}
		attemptCtx, stopAttempt := context.WithTimeout(runCtx, 3*time.Minute)
		type outputEvent struct {
			index  int
			phase  string
			result *ScheduledTestResult
		}
		// At most three phase transitions plus one result per output. Streaming
		// callbacks never wait on persistence and events retain each output's order.
		events := make(chan outputEvent, pending*4)
		phases := make(map[int]string, pending)
		for i := 0; i < pending; i++ {
			phases[i] = "waiting"
		}
		updatePhase := func() {
			state.Phase = "waiting"
			for _, phase := range phases {
				if groupPelicanPhaseRank(phase) > groupPelicanPhaseRank(state.Phase) {
					state.Phase = phase
				}
			}
			if len(phases) == 0 {
				state.Phase = "saving"
			}
		}
		for i := 0; i < pending; i++ {
			go func(index int) {
				begin := time.Now()
				result, err := s.scheduledSvc.runGroupPelican(attemptCtx, plan, func(phase string) {
					events <- outputEvent{index: index, phase: phase}
				})
				if err != nil || result == nil {
					message := "Test returned no result"
					if err != nil {
						message = err.Error()
					}
					snapshot := *plan.PelicanConfig
					snapshot.ModelID = plan.ModelID
					result = &ScheduledTestResult{Status: "failed", ErrorMessage: message, StartedAt: begin,
						FinishedAt: time.Now(), LatencyMs: time.Since(begin).Milliseconds(), PelicanConfig: &snapshot}
				}
				events <- outputEvent{index: index, result: result}
			}(i)
		}
		persistenceFailed := false
		for finished := 0; finished < pending; {
			event := <-events
			if event.result == nil {
				phases[event.index] = event.phase
				previous := state.Phase
				updatePhase()
				if state.Phase != previous {
					if err := s.saveGroupExecution(plan.ID, until, state); err != nil {
						persistenceFailed = true
					}
				}
				continue
			}
			finished++
			delete(phases, event.index)
			updatePhase()
			if err := s.saveGroupExecution(plan.ID, until, state); err != nil {
				persistenceFailed = true
			}
			result := event.result
			state.Completed++
			if result.Status == "success" {
				state.Succeeded++
			} else {
				state.Failed++
				state.LastError = result.ErrorMessage
			}
			saveCtx, stopSave := context.WithTimeout(context.Background(), 10*time.Second)
			err := s.scheduledSvc.SaveResult(saveCtx, plan.ID, plan.MaxResults, result)
			stopSave()
			if err != nil {
				persistenceFailed = true
				state.LastError = "Unable to save test history; automatic retries stopped"
				logger.LegacyPrintf("service.scheduled_test_runner", "group plan=%d save failed: %v", plan.ID, err)
			}
			if err := s.saveGroupExecution(plan.ID, until, state); err != nil {
				persistenceFailed = true
			}
		}
		stopAttempt()
		if persistenceFailed || runCtx.Err() != nil {
			return
		}
		if state.Failed == 0 {
			state.Status, state.LastError = "success", ""
			return
		}
		if attempt == groupPelicanMaxAttempts || runCtx.Err() != nil {
			return
		}
		pending = state.Failed
		delay := s.groupRetryDelay(attempt)
		retryAt := time.Now().Add(delay)
		state.Status, state.RetryAt, state.Phase = "retrying", &retryAt, ""
		if err := s.saveGroupExecution(plan.ID, until, state); err != nil {
			return
		}
		timer := time.NewTimer(delay)
		select {
		case <-runCtx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		// A pause, edit or deletion stops further billable requests, even across replicas.
		current, err := s.planRepo.GetByID(runCtx, plan.ID)
		if err != nil || current == nil || !current.Enabled || !current.UpdatedAt.Equal(plan.UpdatedAt) || current.RunningUntil == nil || !current.RunningUntil.Equal(until) {
			state.Status, state.LastError = "interrupted", "Plan paused, changed or removed; automatic retries stopped"
			return
		}
	}
}
