package service

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func groupRunnerFixture() (*ScheduledTestRunnerService, *ScheduledTestService, *ScheduledTestPlan, *pelicanPlanRepo, *pelicanResults) {
	svc, plan, _ := groupPelicanFixture()
	plans, results := &pelicanPlanRepo{plan: plan}, &pelicanResults{}
	svc.planRepo, svc.resultRepo = plans, results
	runner := NewScheduledTestRunnerService(plans, svc, nil, nil, nil)
	runner.retryDelay = func(int) time.Duration { return 0 }
	return runner, svc, plan, plans, results
}

func TestGroupPelicanManualStartImmediatelyAndSurvivesRequestCancellation(t *testing.T) {
	runner, svc, plan, plans, results := groupRunnerFixture()
	defer runner.Stop()
	future := time.Now().Add(time.Hour)
	plan.NextRunAt = &future
	entered, release := make(chan struct{}), make(chan struct{})
	svc.SetGroupGateway(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		select {
		case <-release:
		case <-r.Context().Done():
			return
		}
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"<svg></svg>\"},\"finish_reason\":\"stop\"}]}\n")
	}))
	ctx, cancel := context.WithCancel(context.Background())
	require.NoError(t, svc.TriggerGroupPlan(ctx, plan.ID))
	cancel()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("manual test waited for scheduler")
	}
	require.Error(t, svc.TriggerGroupPlan(context.Background(), plan.ID), "a running group cannot be started twice")
	close(release)
	runner.manualWG.Wait()
	require.True(t, plans.immediate)
	require.True(t, plans.finished)
	require.Len(t, results.results, 1)
	require.Equal(t, "success", results.results[0].Status, "closing the triggering request must not cancel the test")
	require.Equal(t, "success", plans.states[len(plans.states)-1].Status)
}

func TestGroupPelicanRetriesOnlyFailedParallelOutputs(t *testing.T) {
	runner, svc, plan, plans, results := groupRunnerFixture()
	defer runner.Stop()
	plan.PelicanConfig.ParallelCount = 2
	var calls atomic.Int32
	svc.SetGroupGateway(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			http.Error(w, "temporary failure", http.StatusServiceUnavailable)
			return
		}
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"<svg></svg>\"},\"finish_reason\":\"stop\"}]}\n")
	}))
	runner.runOnePlan(context.Background(), plan)
	require.Equal(t, int32(3), calls.Load(), "successful parallel output is not charged again")
	require.Len(t, results.results, 3, "every attempt is retained")
	last := plans.states[len(plans.states)-1]
	require.Equal(t, "success", last.Status)
	require.Equal(t, 2, last.Attempt)
	require.Equal(t, 2, last.Succeeded)
	require.Equal(t, 2, last.Completed)
	require.Zero(t, last.Failed)
	require.Empty(t, last.LastError)
	require.NotNil(t, last.FinishedAt)
	sawRetry, sawPartial := false, false
	for _, state := range plans.states {
		if state.Status == "retrying" {
			sawRetry = true
			require.NotNil(t, state.RetryAt)
		}
		if state.Completed == 1 && state.Total == 2 {
			sawPartial = true
		}
	}
	require.True(t, sawRetry)
	require.True(t, sawPartial, "progress must update before all parallel requests finish")
}

func TestGroupPelicanRetryLimitAndPause(t *testing.T) {
	for _, pause := range []bool{false, true} {
		t.Run(fmt.Sprintf("pause=%v", pause), func(t *testing.T) {
			runner, svc, plan, plans, results := groupRunnerFixture()
			defer runner.Stop()
			var calls atomic.Int32
			svc.SetGroupGateway(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				http.Error(w, "temporary failure", http.StatusServiceUnavailable)
			}))
			if pause {
				runner.retryDelay = func(int) time.Duration { plans.mu.Lock(); plans.plan.Enabled = false; plans.mu.Unlock(); return 0 }
			}
			runner.runOnePlan(context.Background(), plan)
			last := plans.states[len(plans.states)-1]
			if pause {
				require.Equal(t, int32(1), calls.Load())
				require.Equal(t, "interrupted", last.Status)
			} else {
				require.Equal(t, int32(3), calls.Load())
				require.Len(t, results.results, 3)
				require.Equal(t, "failed", last.Status)
				require.Equal(t, 3, last.Attempt)
			}
			require.True(t, plans.finished)
			require.NotEmpty(t, last.LastError)
			require.Nil(t, last.RetryAt)
		})
	}
}

func TestGroupPelicanCancellationStopsRetryWait(t *testing.T) {
	runner, svc, plan, plans, _ := groupRunnerFixture()
	defer runner.Stop()
	retrying, done := make(chan struct{}), make(chan struct{})
	runner.retryDelay = func(int) time.Duration { close(retrying); return time.Hour }
	var calls atomic.Int32
	svc.SetGroupGateway(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { runner.runOnePlan(ctx, plan); close(done) }()
	select {
	case <-retrying:
	case <-time.After(2 * time.Second):
		t.Fatal("did not enter retry")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("retry wait ignored cancellation")
	}
	require.Equal(t, int32(1), calls.Load())
	require.Equal(t, "interrupted", plans.states[len(plans.states)-1].Status)
	require.True(t, plans.finished)
}

func TestGroupPelicanManualStartRejectsCapacityAndShutdown(t *testing.T) {
	runner, svc, plan, plans, _ := groupRunnerFixture()
	for i := 0; i < scheduledTestDefaultMaxWorkers; i++ {
		runner.workers <- struct{}{}
	}
	require.ErrorContains(t, svc.TriggerGroupPlan(context.Background(), plan.ID), "busy")
	require.False(t, plans.claimed)
	runner.Stop()
	require.ErrorContains(t, svc.TriggerGroupPlan(context.Background(), plan.ID), "shutting down")
}
