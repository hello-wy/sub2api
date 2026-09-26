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
	other := mustCreateGroup(t, client, &service.Group{Name: name("other")})
	user := mustCreateUser(t, client, &service.User{Email: name("owner") + "@example.com", Balance: 100})
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
	require.NoError(t, svc.TriggerGroupPlan(ctx, plan.ID))
	plan, err = plans.GetByID(ctx, plan.ID)
	require.NoError(t, err)
	require.NoError(t, svc.TriggerGroupPlan(ctx, second.ID))
	second, err = plans.GetByID(ctx, second.ID)
	require.NoError(t, err)
	now := time.Now().Truncate(time.Microsecond)
	until := now.Add(15 * time.Minute)
	claimed, err := plans.ClaimPelican(ctx, plan, now, until, now.Add(time.Hour))
	require.NoError(t, err)
	require.True(t, claimed)
	claimed, err = plans.ClaimPelican(ctx, second, now, until, now.Add(time.Hour))
	require.NoError(t, err)
	require.False(t, claimed, "group plans share the group lease")
	require.Error(t, svc.TriggerGroupPlan(ctx, plan.ID), "running plans cannot be queued again")
	require.NoError(t, plans.FinishPelican(ctx, plan.ID, until, now))
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
