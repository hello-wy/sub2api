package service

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var errGroupPelicanCancelled = errors.New("Test interrupted by administrator")

type groupPelicanRun struct {
	until  time.Time
	cancel context.CancelCauseFunc
}

// Cancellation targets an execution lease, not a mutable plan ID. A delayed
// request from an old tab cannot interrupt a newer run of the same plan.
func (s *ScheduledTestService) CancelGroupPlan(ctx context.Context, id int64, until time.Time) error {
	plan, err := s.planRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if plan == nil || plan.GroupID <= 0 || until.IsZero() {
		return fmt.Errorf("only an active group execution can be interrupted")
	}
	accepted, err := s.planRepo.RequestPelicanCancellation(ctx, id, until)
	if err != nil {
		return err
	}
	if !accepted {
		return fmt.Errorf("test already finished or a different execution is running; refresh its status")
	}
	if s.cancelGroupPlan != nil {
		s.cancelGroupPlan(id, until)
	}
	return nil
}

func (s *ScheduledTestRunnerService) cancelGroupPlanNow(id int64, until time.Time) {
	s.groupRunsMu.Lock()
	defer s.groupRunsMu.Unlock()
	if run, ok := s.groupRuns[id]; ok && run.until.Equal(until) {
		run.cancel(errGroupPelicanCancelled)
	}
}

// The database request covers other replicas and cancellation between claiming
// a lease and registering the local worker. Keep the lease until requests stop.
func (s *ScheduledTestRunnerService) watchGroupCancellation(ctx context.Context, planID int64, until time.Time, cancel context.CancelCauseFunc) func() {
	s.groupRunsMu.Lock()
	if s.groupRuns == nil {
		s.groupRuns = make(map[int64]groupPelicanRun)
	}
	s.groupRuns[planID] = groupPelicanRun{until: until, cancel: cancel}
	s.groupRunsMu.Unlock()
	check := func() {
		readCtx, stop := context.WithTimeout(ctx, 2*time.Second)
		defer stop()
		current, err := s.planRepo.GetByID(readCtx, planID)
		if err != nil || current == nil {
			return
		}
		if current.RunningUntil == nil || !current.RunningUntil.Equal(until) || !current.RunningUntil.After(time.Now()) {
			cancel(errors.New("Test execution lease expired or changed"))
		} else if current.Execution != nil && current.Execution.CancelRequested {
			cancel(errGroupPelicanCancelled)
		}
	}
	check()
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				check()
			}
		}
	}()
	return func() {
		cancel(nil)
		<-done
		s.groupRunsMu.Lock()
		defer s.groupRunsMu.Unlock()
		if run, ok := s.groupRuns[planID]; ok && run.until.Equal(until) {
			delete(s.groupRuns, planID)
		}
	}
}
