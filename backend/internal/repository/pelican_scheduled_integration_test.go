//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestPelicanSchedulePersistenceLeaseAndRetention(t *testing.T) {
	ctx := context.Background()
	var account int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO accounts (name,platform,type,status) VALUES ('pelican-integration','openai','apikey','active') RETURNING id`).Scan(&account))
	defer func() { _, _ = integrationDB.ExecContext(ctx, `DELETE FROM accounts WHERE id=$1`, account) }()
	plans := NewScheduledTestPlanRepository(integrationDB)
	results := NewScheduledTestResultRepository(integrationDB)
	svc := service.NewScheduledTestService(plans, results)
	config := &service.PelicanTestConfig{Prompt: "<pelican>", ReasoningEffort: "medium", ParallelCount: 2, IntervalMinutes: 30, RunForHours: 24}
	plan, err := svc.CreatePlan(ctx, &service.ScheduledTestPlan{AccountID: account, ModelID: "gpt-6-astra", CronExpression: "*/30 * * * *", Enabled: true, MaxResults: 2, PelicanConfig: config})
	require.NoError(t, err)
	require.Equal(t, config, plan.PelicanConfig)
	require.NotNil(t, plan.ExpiresAt)
	_, err = svc.CreatePlan(ctx, &service.ScheduledTestPlan{AccountID: account, ModelID: "gpt-6-astra", CronExpression: "*/30 * * * *", Enabled: true, PelicanConfig: config})
	require.Error(t, err, "one pelican schedule per account")
	legacy, err := svc.CreatePlan(ctx, &service.ScheduledTestPlan{AccountID: account, CronExpression: "*/30 * * * *", Enabled: true})
	require.NoError(t, err)
	require.Nil(t, legacy.PelicanConfig)
	now := time.Now().Truncate(time.Microsecond)
	_, err = integrationDB.ExecContext(ctx, `UPDATE scheduled_test_plans SET next_run_at=$2 WHERE id=$1`, plan.ID, now.Add(-time.Minute))
	require.NoError(t, err)
	until := now.Add(15 * time.Minute)
	claimed, err := plans.ClaimPelican(ctx, plan, now, until, now.Add(30*time.Minute))
	require.NoError(t, err)
	require.True(t, claimed)
	claimed, err = plans.ClaimPelican(ctx, plan, now, until, now.Add(30*time.Minute))
	require.NoError(t, err)
	require.False(t, claimed)
	plan.Enabled = false
	_, err = svc.UpdatePlan(ctx, plan)
	require.NoError(t, err)
	require.NoError(t, plans.FinishPelican(ctx, plan.ID, until, now))
	stored, err := plans.GetByID(ctx, plan.ID)
	require.NoError(t, err)
	require.False(t, stored.Enabled)
	require.Nil(t, stored.RunningUntil)
	for i := 0; i < 3; i++ {
		require.NoError(t, svc.SaveResult(ctx, plan.ID, 2, &service.ScheduledTestResult{Status: "success", ResponseText: "<html></html>", StartedAt: now, FinishedAt: now, PelicanConfig: config}))
	}
	saved, err := results.ListByPlanID(ctx, plan.ID, 50)
	require.NoError(t, err)
	require.Len(t, saved, 2)
	require.Equal(t, config, saved[0].PelicanConfig)
	summaries, err := results.ListByPlanID(ctx, plan.ID, 50, false)
	require.NoError(t, err)
	require.Len(t, summaries, 2)
	require.Empty(t, summaries[0].ResponseText)
	detail, err := results.GetResult(ctx, plan.ID, summaries[0].ID)
	require.NoError(t, err)
	require.Equal(t, "<html></html>", detail.ResponseText)
	_, err = results.GetResult(ctx, legacy.ID, summaries[0].ID)
	require.Error(t, err)
	_, err = integrationDB.ExecContext(ctx, `UPDATE scheduled_test_results SET created_at=$2 WHERE plan_id=$1`, plan.ID, now.Add(-8*24*time.Hour))
	require.NoError(t, err)
	_, err = results.Create(ctx, &service.ScheduledTestResult{PlanID: legacy.ID, Status: "success", StartedAt: now, FinishedAt: now})
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `UPDATE scheduled_test_results SET created_at=$2 WHERE plan_id=$1`, legacy.ID, now.Add(-8*24*time.Hour))
	require.NoError(t, err)
	require.NoError(t, results.PruneExpiredPelican(ctx, now.Add(-7*24*time.Hour)))
	saved, err = results.ListByPlanID(ctx, plan.ID, 50)
	require.NoError(t, err)
	require.Empty(t, saved)
	saved, err = results.ListByPlanID(ctx, legacy.ID, 50)
	require.NoError(t, err)
	require.Len(t, saved, 1, "do not expire connectivity history")
	_, err = integrationDB.ExecContext(ctx, `UPDATE scheduled_test_plans SET enabled=true,expires_at=$2,next_run_at=$2 WHERE id=$1`, plan.ID, now.Add(-time.Minute))
	require.NoError(t, err)
	claimed, err = plans.ClaimPelican(ctx, stored, now, until, now.Add(time.Minute))
	require.NoError(t, err)
	require.False(t, claimed, "expired task cannot start before expiry sweep")
	require.NoError(t, plans.ExpirePelican(ctx, now))
	stored, err = plans.GetByID(ctx, plan.ID)
	require.NoError(t, err)
	require.False(t, stored.Enabled)
}
