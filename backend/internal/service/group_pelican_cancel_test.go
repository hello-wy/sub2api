package service

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGroupPelicanInterruptRunningRequestAndRejectStaleLease(t *testing.T) {
	runner, svc, plan, plans, results := groupRunnerFixture()
	defer runner.Stop()
	entered, cancelled, release := make(chan struct{}), make(chan struct{}), make(chan struct{}, 1)
	defer close(release)
	var calls atomic.Int32
	svc.SetGroupGateway(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		close(entered)
		<-r.Context().Done()
		close(cancelled)
		select {
		case <-release:
		case <-time.After(2 * time.Second):
		}
	}))
	require.NoError(t, svc.TriggerGroupPlan(context.Background(), plan.ID))
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("request did not start")
	}
	current, err := plans.GetByID(context.Background(), plan.ID)
	require.NoError(t, err)
	require.Error(t, svc.CancelGroupPlan(context.Background(), plan.ID, current.RunningUntil.Add(-time.Second)))
	select {
	case <-cancelled:
		t.Fatal("a stale tab interrupted the current run")
	default:
	}
	require.NoError(t, svc.CancelGroupPlan(context.Background(), plan.ID, *current.RunningUntil))
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("local interrupt did not cancel the gateway request")
	}
	require.NoError(t, svc.CancelGroupPlan(context.Background(), plan.ID, *current.RunningUntil), "repeated cancellation is idempotent before the worker stops")
	plans.mu.Lock()
	finished := plans.finished
	plans.mu.Unlock()
	require.False(t, finished, "lease must remain held while the request is unwinding")
	release <- struct{}{}
	runner.manualWG.Wait()
	require.Equal(t, int32(1), calls.Load(), "interrupted requests must not automatically retry")
	require.Len(t, results.results, 1)
	last := plans.states[len(plans.states)-1]
	require.Equal(t, "interrupted", last.Status)
	require.True(t, last.CancelRequested)
	require.Nil(t, last.RetryAt)
	require.Contains(t, last.LastError, "administrator")
	require.True(t, plans.finished)
	require.True(t, plans.plan.Enabled, "interrupting this run preserves the cron plan")
	require.Error(t, svc.CancelGroupPlan(context.Background(), plan.ID, *current.RunningUntil), "completed runs cannot be interrupted")
}

func TestGroupPelicanInterruptStopsRetryWait(t *testing.T) {
	runner, svc, plan, plans, _ := groupRunnerFixture()
	defer runner.Stop()
	runner.retryDelay = func(int) time.Duration { return time.Hour }
	var calls atomic.Int32
	svc.SetGroupGateway(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		http.Error(w, "temporary failure", http.StatusServiceUnavailable)
	}))
	require.NoError(t, svc.TriggerGroupPlan(context.Background(), plan.ID))
	waitGroupExecution(t, plans, func(s ScheduledTestExecution) bool { return s.Status == "retrying" })
	current, err := plans.GetByID(context.Background(), plan.ID)
	require.NoError(t, err)
	require.NoError(t, svc.CancelGroupPlan(context.Background(), plan.ID, *current.RunningUntil))
	done := make(chan struct{})
	go func() { runner.manualWG.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("interrupt did not stop the retry timer")
	}
	require.Equal(t, int32(1), calls.Load())
	require.Equal(t, "interrupted", plans.states[len(plans.states)-1].Status)
}

func TestGroupPelicanInterruptFromAnotherReplica(t *testing.T) {
	runner, svc, plan, plans, _ := groupRunnerFixture()
	defer runner.Stop()
	entered, stopped := make(chan struct{}), make(chan struct{})
	svc.SetGroupGateway(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		close(entered)
		<-r.Context().Done()
		close(stopped)
	}))
	require.NoError(t, svc.TriggerGroupPlan(context.Background(), plan.ID))
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("request did not start")
	}
	current, err := plans.GetByID(context.Background(), plan.ID)
	require.NoError(t, err)
	// This service has no in-process callback to the worker, only shared storage.
	otherReplica := NewScheduledTestService(plans, nil)
	require.NoError(t, otherReplica.CancelGroupPlan(context.Background(), plan.ID, *current.RunningUntil))
	select {
	case <-stopped:
	case <-time.After(3 * time.Second):
		t.Fatal("worker ignored persisted interruption")
	}
	runner.manualWG.Wait()
	require.Equal(t, "interrupted", plans.states[len(plans.states)-1].Status)
}

func TestGroupPelicanInterruptBeforeWorkerRegisters(t *testing.T) {
	runner, svc, plan, plans, results := groupRunnerFixture()
	defer runner.Stop()
	until := time.Now().Add(15 * time.Minute)
	claimed, err := plans.ClaimPelican(context.Background(), plan, time.Now(), until, time.Now().Add(time.Hour), true)
	require.NoError(t, err)
	require.True(t, claimed)
	require.NoError(t, svc.CancelGroupPlan(context.Background(), plan.ID, until))
	svc.SetGroupGateway(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("cancelled run issued a request") }))
	runner.runClaimedGroupPelican(context.Background(), plan, time.Now(), until)
	require.Empty(t, results.results)
	require.Equal(t, "interrupted", plans.states[len(plans.states)-1].Status)
}
