//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGroupPelicanPersistenceLeaseShowcaseAndKeyLifecycle(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	suffix := time.Now().UnixNano()
	name := func(label string) string { return fmt.Sprintf("group-pelican-%s-%d", label, suffix) }
	group := mustCreateGroup(t, client, &service.Group{Name: name("entry")})
	t.Cleanup(func() {
		_, err := integrationDB.ExecContext(context.Background(), "DELETE FROM groups WHERE id = $1", group.ID)
		require.NoError(t, err)
	})
	other := mustCreateGroup(t, client, &service.Group{Name: name("other")})
	t.Cleanup(func() {
		_, err := integrationDB.ExecContext(context.Background(), "DELETE FROM groups WHERE id = $1", other.ID)
		require.NoError(t, err)
	})
	user := mustCreateUser(t, client, &service.User{Email: name("owner") + "@example.com", Balance: 100})
	t.Cleanup(func() {
		_, err := integrationDB.ExecContext(context.Background(), "DELETE FROM users WHERE id = $1", user.ID)
		require.NoError(t, err)
	})
	keyRepo := NewAPIKeyRepository(client, integrationDB)
	key := &service.APIKey{UserID: user.ID, Key: name("key"), Name: name("test"), GroupID: &group.ID, Status: service.StatusActive}
	require.NoError(t, keyRepo.Create(ctx, key))
	plans := NewScheduledTestPlanRepository(integrationDB)
	results := NewScheduledTestResultRepository(integrationDB)
	svc := service.ProvideScheduledTestService(plans, results, nil, NewGroupRepository(client, integrationDB), keyRepo)
	config := &service.PelicanTestConfig{QuestionKind: "pelican", Prompt: "draw a pelican", ReasoningEffort: "medium", ParallelCount: 1}
	makePlan := func() *service.ScheduledTestPlan {
		p, err := svc.CreatePlan(ctx, &service.ScheduledTestPlan{GroupID: group.ID, APIKeyID: key.ID, ModelID: "public-model", CronExpression: "0 * * * *", Enabled: true, MaxResults: 2, PelicanConfig: config})
		require.NoError(t, err)
		return p
	}
	plan, second := makePlan(), makePlan()
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM pelican_showcase_items WHERE group_id IN ($1,$2)`, group.ID, other.ID)
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM scheduled_test_plans WHERE group_id = $1`, group.ID)
	})
	require.Zero(t, plan.AccountID)
	require.Equal(t, group.ID, plan.GroupID)
	require.Equal(t, key.ID, plan.APIKeyID)
	keys, err := svc.ListGroupTestKeys(ctx, group.ID)
	require.NoError(t, err)
	require.Len(t, keys, 1)
	require.Equal(t, user.Email, keys[0].UserEmail)
	listed, err := svc.ListPlansByGroup(ctx, group.ID)
	require.NoError(t, err)
	require.Len(t, listed, 2)
	runner := service.NewScheduledTestRunnerService(plans, svc, nil, nil, nil)
	t.Cleanup(runner.Stop)
	now := time.Now().Truncate(time.Microsecond)
	until := now.Add(15 * time.Minute)
	claimed, err := plans.ClaimPelican(ctx, plan, now, until, now.Add(time.Hour))
	require.NoError(t, err)
	require.False(t, claimed, "scheduled execution respects the future due time")
	claimed, err = plans.ClaimPelican(ctx, plan, now, until, now.Add(time.Hour), true)
	require.NoError(t, err)
	require.True(t, claimed, "manual execution claims immediately without waiting for cron")
	running, err := plans.GetByID(ctx, plan.ID)
	require.NoError(t, err)
	require.Equal(t, "running", running.Execution.Status)
	require.Equal(t, "waiting", running.Execution.Phase)
	require.Equal(t, 3, running.Execution.MaxAttempts)
	streaming := &service.ScheduledTestExecution{Status: "running", Phase: "generating", Attempt: 1, MaxAttempts: 3, Total: 1, StartedAt: now}
	require.NoError(t, plans.UpdatePelicanExecution(ctx, plan.ID, until, streaming))
	running, err = plans.GetByID(ctx, plan.ID)
	require.NoError(t, err)
	require.Equal(t, "generating", running.Execution.Phase)
	state := &service.ScheduledTestExecution{Status: "retrying", Attempt: 1, MaxAttempts: 3, Total: 1, Failed: 1, StartedAt: now}
	require.NoError(t, plans.UpdatePelicanExecution(ctx, plan.ID, until, state))
	require.Error(t, plans.UpdatePelicanExecution(ctx, plan.ID, until.Add(-time.Second), &service.ScheduledTestExecution{Status: "success"}))
	listed, err = svc.ListPlansByGroup(ctx, group.ID)
	require.NoError(t, err)
	for _, p := range listed {
		if p.ID == plan.ID {
			require.Equal(t, "retrying", p.Execution.Status)
		}
	}
	claimed, err = plans.ClaimPelican(ctx, second, now, until, now.Add(time.Hour), true)
	require.NoError(t, err)
	require.False(t, claimed, "group plans share the group lease")
	require.Error(t, svc.TriggerGroupPlan(ctx, plan.ID), "running plans cannot be started again")
	accepted, err := plans.RequestPelicanCancellation(ctx, plan.ID, until.Add(-time.Second))
	require.NoError(t, err)
	require.False(t, accepted, "stale cancellation must not affect this run")
	accepted, err = plans.RequestPelicanCancellation(ctx, plan.ID, until)
	require.NoError(t, err)
	require.True(t, accepted)
	require.NoError(t, plans.UpdatePelicanExecution(ctx, plan.ID, until, streaming))
	running, err = plans.GetByID(ctx, plan.ID)
	require.NoError(t, err)
	require.Equal(t, "cancelling", running.Execution.Status, "progress cannot overwrite cancellation")
	require.True(t, running.Execution.CancelRequested)
	require.Empty(t, running.Execution.Phase)
	require.NotNil(t, running.RunningUntil, "keep the lease while the request is stopping")
	claimed, err = plans.ClaimPelican(ctx, second, now, until, now.Add(time.Hour), true)
	require.NoError(t, err)
	require.False(t, claimed, "cancelling runs still hold the group lease")
	streaming.Status, streaming.FinishedAt = "success", &now
	require.NoError(t, plans.UpdatePelicanExecution(ctx, plan.ID, until, streaming))
	running, err = plans.GetByID(ctx, plan.ID)
	require.NoError(t, err)
	require.Equal(t, "interrupted", running.Execution.Status, "accepted cancellation wins a race with completion")
	require.NoError(t, plans.FinishPelican(ctx, plan.ID, until, now))
	require.Error(t, svc.CancelGroupPlan(ctx, plan.ID, until))
	newUntil := until.Add(time.Second)
	claimed, err = plans.ClaimPelican(ctx, plan, now, newUntil, now.Add(time.Hour), true)
	require.NoError(t, err)
	require.True(t, claimed)
	running, err = plans.GetByID(ctx, plan.ID)
	require.NoError(t, err)
	require.False(t, running.Execution.CancelRequested, "new executions clear old cancellation")
	require.Equal(t, "running", running.Execution.Status)
	require.Error(t, svc.CancelGroupPlan(ctx, plan.ID, until), "old tabs cannot cancel a new run")
	require.NoError(t, plans.FinishPelican(ctx, plan.ID, newUntil, now))
	plan.APIKeyID = 0
	plan.Enabled = false
	plan, err = svc.UpdatePlan(ctx, plan)
	require.NoError(t, err, "missing credentials must not prevent pausing")
	require.Error(t, svc.TriggerGroupPlan(ctx, plan.ID))
	plan.APIKeyID = key.ID
	plan.Enabled = true
	plan, err = svc.UpdatePlan(ctx, plan)
	require.NoError(t, err)
	showcase := NewPelicanShowcaseRepository(integrationDB)
	result, err := results.Create(ctx, &service.ScheduledTestResult{PlanID: plan.ID, Status: "success", ResponseText: "<svg></svg>", StartedAt: now, FinishedAt: now, PelicanConfig: config})
	require.NoError(t, err)
	require.NoError(t, showcase.Publish(ctx, result, []int64{group.ID, other.ID}, 3))
	require.NoError(t, showcase.Publish(ctx, result, []int64{group.ID, other.ID}, 3))
	items, err := showcase.ListItems(ctx, []int64{group.ID, other.ID}, 3, time.Time{})
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, group.ID, items[0].GroupID, "only the entry group receives the result")
	require.Equal(t, "group", items[0].SourceScope)
	item, err := showcase.GetItem(ctx, items[0].ID, []int64{group.ID}, 3, time.Time{})
	require.NoError(t, err)
	require.Equal(t, "<svg></svg>", item.ResponseText)
	require.Equal(t, "group", item.SourceScope)
	require.NoError(t, keyRepo.Delete(ctx, key.ID))
	require.Error(t, svc.TriggerGroupPlan(ctx, plan.ID), "deleted API Keys fail before gateway execution")
	require.NoError(t, svc.DeletePlan(ctx, plan.ID))
	items, err = showcase.ListItems(ctx, []int64{group.ID}, 3, time.Time{})
	require.NoError(t, err)
	require.Len(t, items, 1, "showcase snapshots survive plan deletion")
}
