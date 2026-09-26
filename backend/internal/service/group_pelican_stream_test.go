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

func TestGroupPelicanStreamProgressHandlesSplitChunksAndIgnoresHeartbeats(t *testing.T) {
	var phases []string
	progress := &groupPelicanStreamProgress{onProgress: func(phase string) { phases = append(phases, phase) }}
	progress.write([]byte(`: ping

data: {"choices":[]}

data: {"error":{"message":"failed"}}

`))
	require.Empty(t, phases)
	progress.write([]byte(`data: {"choices":[{"delta":{"role":`))
	require.Empty(t, phases, "partial JSON is not a stream event")
	progress.write([]byte(`"assistant"}}]}

`))
	require.Equal(t, []string{"receiving"}, phases)
	progress.write([]byte(`data: {"choices":[{"delta":{"reasoning_content":"思考"}}]}

`))
	progress.write([]byte(`data: {"choices":[{"delta":{"content":"<svg>"}}]}

`))
	progress.write([]byte(`data: {"choices":[{"delta":{"reasoning":"late reasoning"}}]}

data: [DONE]

`))
	require.Equal(t, []string{"receiving", "thinking", "generating"}, phases)
	// Providers also use reasoning without the _content suffix.
	var reasoning string
	alternate := &groupPelicanStreamProgress{onProgress: func(phase string) { reasoning = phase }}
	alternate.write([]byte(`data: {"choices":[{"delta":{"reasoning":"thinking"}}]}
`))
	require.Equal(t, "thinking", reasoning)
}

func waitGroupExecution(t *testing.T, plans *pelicanPlanRepo, predicate func(ScheduledTestExecution) bool) {
	t.Helper()
	require.Eventually(t, func() bool {
		plans.mu.Lock()
		defer plans.mu.Unlock()
		return len(plans.states) > 0 && predicate(plans.states[len(plans.states)-1])
	}, 2*time.Second, time.Millisecond)
}

func TestGroupPelicanStreamsUpdateStateBeforeResponseCompletes(t *testing.T) {
	runner, svc, plan, plans, results := groupRunnerFixture()
	defer runner.Stop()
	chunks := make(chan string)
	svc.SetGroupGateway(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for {
			select {
			case <-r.Context().Done():
				return
			case chunk, ok := <-chunks:
				if !ok {
					return
				}
				fmt.Fprint(w, chunk)
				flusher, ok := w.(http.Flusher)
				if !ok {
					t.Error("gateway writer must support streaming flush")
					return
				}
				flusher.Flush()
			}
		}
	}))
	require.NoError(t, svc.TriggerGroupPlan(context.Background(), plan.ID))
	waitGroupExecution(t, plans, func(s ScheduledTestExecution) bool { return s.Phase == "waiting" })
	for _, step := range []struct{ body, phase string }{
		{`data: {"choices":[{"delta":{"role":"assistant"}}]}

`, "receiving"},
		{`data: {"choices":[{"delta":{"reasoning_content":"plan the drawing"}}]}

`, "thinking"},
		{`data: {"choices":[{"delta":{"content":"<svg></svg>"}}]}

`, "generating"},
	} {
		select {
		case chunks <- step.body:
		case <-time.After(2 * time.Second):
			t.Fatal("gateway did not receive test chunk")
		}
		waitGroupExecution(t, plans, func(s ScheduledTestExecution) bool {
			return s.Status == "running" && s.Phase == step.phase && s.Completed == 0 && s.FinishedAt == nil
		})
	}
	chunks <- `data: {"choices":[{"delta":{},"finish_reason":"stop"}]}

data: [DONE]

`
	close(chunks)
	runner.manualWG.Wait()
	require.Len(t, results.results, 1)
	require.Equal(t, "success", results.results[0].Status)
	last := plans.states[len(plans.states)-1]
	require.Equal(t, "success", last.Status)
	require.Empty(t, last.Phase, "finished state must not retain a streaming phase")
}

func TestGroupPelicanPhaseTracksRemainingParallelRequests(t *testing.T) {
	runner, svc, plan, plans, _ := groupRunnerFixture()
	defer runner.Stop()
	plan.PelicanConfig.ParallelCount = 2
	release := make(chan struct{})
	var calls atomic.Int32
	svc.SetGroupGateway(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 2 {
			select {
			case <-release:
			case <-r.Context().Done():
				return
			}
		}
		fmt.Fprint(w, `data: {"choices":[{"delta":{"content":"<svg></svg>"},"finish_reason":"stop"}]}

`)
	}))
	require.NoError(t, svc.TriggerGroupPlan(context.Background(), plan.ID))
	waitGroupExecution(t, plans, func(s ScheduledTestExecution) bool { return s.Succeeded == 1 && s.Phase == "waiting" })
	close(release)
	runner.manualWG.Wait()
	last := plans.states[len(plans.states)-1]
	require.Equal(t, "success", last.Status)
	require.Equal(t, 2, last.Succeeded)
}
