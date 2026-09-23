package repository

import (
	"context"
	"database/sql"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func intelligentScheduleSetting(t *testing.T, repo *intelligentTestRepository, ids ...int64) service.IntelligentTestSetting {
	t.Helper()
	settings, err := repo.Settings(context.Background())
	require.NoError(t, err)
	for _, setting := range settings {
		if setting.TestType == "pelican" {
			setting.Schedule = &service.IntelligentTestSchedule{Enabled: true, IntervalMinutes: 5, AccountIDs: ids}
			setting.Config.ReasoningEffort = "high"
			require.NoError(t, service.NewIntelligentTestService(repo, nil).UpdateSetting(context.Background(), 1, &setting))
			return setting
		}
	}
	t.Fatal("missing pelican setting")
	return service.IntelligentTestSetting{}
}

func makeIntelligentScheduleDue(t *testing.T, db *sql.DB) {
	t.Helper()
	_, err := db.Exec(`UPDATE intelligent_test_schedules SET next_run_at=NOW()-INTERVAL '2 days' WHERE test_type='pelican'`)
	require.NoError(t, err)
}

func TestIntelligentScheduleSettingsPersistWithoutAcceptingClientDates(t *testing.T) {
	db := intelligentTestDB(t)
	repo := &intelligentTestRepository{db: db}
	ctx := context.Background()
	svc := service.NewIntelligentTestService(repo, nil)
	defaults, err := repo.Settings(ctx)
	require.NoError(t, err)
	for _, setting := range defaults {
		require.NotNil(t, setting.Schedule)
		require.False(t, setting.Schedule.Enabled)
		require.Equal(t, 60, setting.Schedule.IntervalMinutes)
		require.Empty(t, setting.Schedule.AccountIDs)
		require.Nil(t, setting.Schedule.NextRunAt)
	}
	setting := intelligentScheduleSetting(t, repo, 12, 10)
	require.Equal(t, []int64{10, 12}, setting.Schedule.AccountIDs)
	require.WithinDuration(t, time.Now().Add(5*time.Minute), *setting.Schedule.NextRunAt, 5*time.Second)
	firstNext := *setting.Schedule.NextRunAt
	forged := time.Now().Add(-24 * time.Hour)
	setting.Schedule.NextRunAt, setting.Schedule.LastRunAt, setting.Schedule.LastError = &forged, &forged, "forged"
	require.NoError(t, svc.UpdateSetting(ctx, 1, &setting))
	require.Equal(t, firstNext, *setting.Schedule.NextRunAt, "saving unrelated settings preserves the interval")
	require.Nil(t, setting.Schedule.LastRunAt)
	require.Empty(t, setting.Schedule.LastError)
	setting.Schedule = nil // Existing clients do not know about schedules.
	setting.UserVisible = true
	require.NoError(t, svc.UpdateSetting(ctx, 1, &setting))
	require.True(t, setting.Schedule.Enabled)
	require.Equal(t, firstNext, *setting.Schedule.NextRunAt)
	restarted := &intelligentTestRepository{db: db}
	loaded, err := restarted.Settings(ctx)
	require.NoError(t, err)
	for _, loadedSetting := range loaded {
		if loadedSetting.TestType == "pelican" {
			require.Equal(t, firstNext, *loadedSetting.Schedule.NextRunAt)
		}
	}
	setting.Schedule.AccountIDs = []int64{999}
	require.ErrorContains(t, svc.UpdateSetting(ctx, 1, &setting), "所选账号不存在")
	setting.Enabled, setting.Schedule = false, nil
	require.NoError(t, svc.UpdateSetting(ctx, 1, &setting))
	require.False(t, setting.Schedule.Enabled)
	require.Nil(t, setting.Schedule.NextRunAt)
}

func TestIntelligentScheduleDispatchIsAtomicAndDoesNotReplayMissedIntervals(t *testing.T) {
	db := intelligentTestDB(t)
	repo := &intelligentTestRepository{db: db}
	ctx := context.Background()
	intelligentScheduleSetting(t, repo, 10)
	makeIntelligentScheduleDue(t, db)
	var wg sync.WaitGroup
	results := make(chan int, 8)
	errors := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			instance := &intelligentTestRepository{db: db}
			processed, err := instance.EnqueueDueSchedules(ctx, 1)
			results <- processed
			errors <- err
		}()
	}
	wg.Wait()
	close(results)
	close(errors)
	processed := 0
	for n := range results {
		processed += n
	}
	for err := range errors {
		require.NoError(t, err)
	}
	require.Equal(t, 1, processed)
	var id, actor int64
	var count int
	var scheduled bool
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM account_tests`).Scan(&count))
	require.Equal(t, 1, count)
	require.NoError(t, db.QueryRow(`SELECT id,requested_by,scheduled FROM account_tests`).Scan(&id, &actor, &scheduled))
	require.EqualValues(t, 1, actor)
	require.True(t, scheduled)
	record, err := repo.Get(ctx, id)
	require.NoError(t, err)
	require.Equal(t, "high", record.ConfigSnapshot.ReasoningEffort)
	require.True(t, record.Scheduled)
	var next, last time.Time
	require.NoError(t, db.QueryRow(`SELECT next_run_at,last_run_at FROM intelligent_test_schedules WHERE test_type='pelican'`).Scan(&next, &last))
	require.WithinDuration(t, time.Now().Add(5*time.Minute), next, 5*time.Second)
	require.WithinDuration(t, time.Now(), last, 5*time.Second)
	count, err = (&intelligentTestRepository{db: db}).EnqueueDueSchedules(ctx, 10)
	require.NoError(t, err)
	require.Zero(t, count, "restart must honor the persisted next deadline")
	// Changing the saved configuration while this run is active must skip it,
	// without creating a conflicting duplicate or replaying overdue work.
	_, err = db.Exec(`UPDATE test_settings SET config=jsonb_set(config,'{model}','"changed-model"') WHERE test_type='pelican'`)
	require.NoError(t, err)
	makeIntelligentScheduleDue(t, db)
	count, err = repo.EnqueueDueSchedules(ctx, 10)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM account_tests`).Scan(&count))
	require.Equal(t, 1, count)
	var lastError string
	require.NoError(t, db.QueryRow(`SELECT last_error FROM intelligent_test_schedules WHERE test_type='pelican'`).Scan(&lastError))
	require.Contains(t, lastError, "跳过 1")
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM audit_logs WHERE action='admin.intelligent_tests.schedule' AND actor_user_id=1 AND actor_role='super_admin'`).Scan(&count))
	require.Equal(t, 2, count)
}

func TestIntelligentScheduleSkipsUnavailableAccountsAndRevokedOwner(t *testing.T) {
	db := intelligentTestDB(t)
	repo := &intelligentTestRepository{db: db}
	ctx := context.Background()
	intelligentScheduleSetting(t, repo, 10, 11, 12)
	_, err := db.Exec(`UPDATE accounts SET rate_limit_reset_at=NOW()+INTERVAL '1 hour' WHERE id=10;
UPDATE accounts SET status='disabled' WHERE id=11`)
	require.NoError(t, err)
	makeIntelligentScheduleDue(t, db)
	n, err := repo.EnqueueDueSchedules(ctx, 10)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	var accountID int64
	require.NoError(t, db.QueryRow(`SELECT account_id FROM account_tests`).Scan(&accountID))
	require.EqualValues(t, 12, accountID)
	_, err = db.Exec(`UPDATE users SET role='user' WHERE id=1`)
	require.NoError(t, err)
	makeIntelligentScheduleDue(t, db)
	n, err = repo.EnqueueDueSchedules(ctx, 10)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	var enabled bool
	var next *time.Time
	var lastError string
	require.NoError(t, db.QueryRow(`SELECT enabled,next_run_at,last_error FROM intelligent_test_schedules WHERE test_type='pelican'`).Scan(&enabled, &next, &lastError))
	require.False(t, enabled)
	require.Nil(t, next)
	require.Contains(t, lastError, "管理员已失效")
	claimed, err := repo.Claim(ctx)
	require.NoError(t, err)
	require.Nil(t, claimed, "revoked owners cannot run already queued scheduled work")
	var status string
	require.NoError(t, db.QueryRow(`SELECT status FROM account_tests`).Scan(&status))
	require.Equal(t, "cancelled", status)
}

func TestIntelligentScheduleDisablingCancelsOnlyScheduledWork(t *testing.T) {
	for _, cause := range []string{"schedule", "test", "account"} {
		t.Run(cause, func(t *testing.T) {
			db := intelligentTestDB(t)
			repo := &intelligentTestRepository{db: db}
			ctx := context.Background()
			setting := intelligentScheduleSetting(t, repo, 10)
			makeIntelligentScheduleDue(t, db)
			_, err := repo.EnqueueDueSchedules(ctx, 10)
			require.NoError(t, err)
			manual, err := repo.Enqueue(ctx, 1, service.IntelligentTestEnqueue{AccountIDs: []int64{11}, TestTypes: []string{"candy"}, IdempotencyKey: "manual_preserved_test"})
			require.NoError(t, err)
			switch cause {
			case "schedule":
				setting.Schedule.Enabled = false
				require.NoError(t, service.NewIntelligentTestService(repo, nil).UpdateSetting(ctx, 1, &setting))
			case "test":
				setting.Enabled, setting.Schedule = false, nil
				require.NoError(t, service.NewIntelligentTestService(repo, nil).UpdateSetting(ctx, 1, &setting))
			case "account":
				_, err = db.Exec(`UPDATE accounts SET schedulable=false WHERE id=10`)
				require.NoError(t, err)
			}
			claimed, err := repo.Claim(ctx)
			require.NoError(t, err)
			require.NotNil(t, claimed)
			require.Equal(t, manual.Records[0].ID, claimed.ID)
			require.False(t, claimed.Scheduled)
			var status string
			require.NoError(t, db.QueryRow(`SELECT status FROM account_tests WHERE scheduled`).Scan(&status))
			require.Equal(t, "cancelled", status)
		})
	}
}

func TestIntelligentScheduleFullQueueSkipsCycleWithoutPartialWork(t *testing.T) {
	db := intelligentTestDB(t)
	repo := &intelligentTestRepository{db: db}
	ctx := context.Background()
	intelligentScheduleSetting(t, repo, 10, 11)
	_, err := db.Exec(`INSERT INTO accounts(id) SELECT generate_series(1000,2998);
INSERT INTO account_tests(account_id,test_type,input,anti_degradation,config_snapshot,requested_by)
SELECT id,'candy','test',false,'{}',1 FROM accounts WHERE id>=1000`)
	require.NoError(t, err)
	makeIntelligentScheduleDue(t, db)
	n, err := repo.EnqueueDueSchedules(ctx, 10)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	var count int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM account_tests`).Scan(&count))
	require.Equal(t, 1999, count, "a partially enqueued schedule is rolled back when capacity runs out")
	var next time.Time
	var lastError string
	require.NoError(t, db.QueryRow(`SELECT next_run_at,last_error FROM intelligent_test_schedules WHERE test_type='pelican'`).Scan(&next, &lastError))
	require.True(t, next.After(time.Now()))
	require.Contains(t, lastError, "队列已满")
}
