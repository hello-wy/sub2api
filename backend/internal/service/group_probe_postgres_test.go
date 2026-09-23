//go:build unit

package service

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func groupProbeTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("GROUP_STATUS_POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("GROUP_STATUS_POSTGRES_TEST_DSN is not set")
	}
	admin, e := sql.Open("postgres", dsn)
	require.NoError(t, e)
	schema := "probe_" + uuid.NewString()[:8]
	_, e = admin.Exec("CREATE SCHEMA " + pq.QuoteIdentifier(schema))
	require.NoError(t, e)
	u, e := url.Parse(dsn)
	require.NoError(t, e)
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, e := sql.Open("postgres", u.String())
	require.NoError(t, e)
	t.Cleanup(func() {
		db.Close()
		admin.Exec("DROP SCHEMA " + pq.QuoteIdentifier(schema) + " CASCADE")
		admin.Close()
	})
	_, e = db.Exec(`CREATE TABLE groups(id bigint PRIMARY KEY);CREATE TABLE users(id bigint PRIMARY KEY);INSERT INTO groups VALUES(1),(2),(3);INSERT INTO users VALUES(1);
 CREATE TABLE channel_monitor_v2_metrics_1m(id bigint);CREATE TABLE channel_monitor_v2_metrics_rollup(id bigint);CREATE TABLE channel_monitor_v2_watermarks(id bigint,backfill_cursor timestamptz,usage_coverage_start timestamptz,error_coverage_start timestamptz);`)
	require.NoError(t, e)
	migration, e := os.ReadFile(filepath.Join("..", "..", "migrations", "252_group_service_status.sql"))
	require.NoError(t, e)
	_, e = db.Exec(string(migration))
	require.NoError(t, e)
	migration, e = os.ReadFile(filepath.Join("..", "..", "migrations", "254_group_status_request_identity.sql"))
	require.NoError(t, e)
	_, e = db.Exec(string(migration))
	require.NoError(t, e)
	return db
}

type groupProbeMonitorRepo struct {
	ChannelMonitorV2Repository
	disabled atomic.Bool
}

func (r *groupProbeMonitorRepo) GetConfig(context.Context) (*ChannelMonitorV2Config, error) {
	return &ChannelMonitorV2Config{Enabled: !r.disabled.Load()}, nil
}

func groupProbeTestMonitor() *ChannelMonitorV2Service {
	return NewChannelMonitorV2Service(&groupProbeMonitorRepo{})
}

type groupProbeGroups struct{ GroupRepository }

func (groupProbeGroups) GetByID(_ context.Context, id int64) (*Group, error) {
	return &Group{ID: id, Name: "group", Status: StatusActive, Platform: PlatformOpenAI}, nil
}

type groupProbeUsers struct{ UserRepository }

func (groupProbeUsers) GetByID(context.Context, int64) (*User, error) {
	return &User{ID: 1, Role: RoleAdmin, Status: StatusActive}, nil
}

func TestGroupProbePostgresDurableAdmissionBudgetAndScope(t *testing.T) {
	db := groupProbeTestDB(t)
	ctx := context.Background()
	s := NewGroupStatusService(groupProbeTestMonitor(), db, groupProbeGroups{}, groupProbeUsers{}, nil, nil, nil)
	defer s.Stop()
	_, e := s.SaveProbeConfig(ctx, GroupProbeConfig{GroupID: 1, Model: "gpt-5", MaxOutputTokens: 64, DailyTokenBudget: 640}, 1)
	require.NoError(t, e)
	// Two services use independent Go locks; only the database protects a group.
	s2 := NewGroupStatusService(groupProbeTestMonitor(), db, groupProbeGroups{}, groupProbeUsers{}, nil, nil, nil)
	defer s2.Stop()
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, svc := range []*GroupStatusService{s, s2} {
		wg.Add(1)
		go func(svc *GroupStatusService) {
			defer wg.Done()
			_, _, err := svc.claimProbe(ctx, 1, false)
			results <- err
		}(svc)
	}
	wg.Wait()
	close(results)
	successes, busy := 0, 0
	for err := range results {
		if err == nil {
			successes++
		} else if err == ErrGroupProbeBusy {
			busy++
		} else {
			t.Fatal(err)
		}
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, busy)
	_, e = db.Exec(`UPDATE group_probe_runs SET lease_until=now()-interval '1 second'`)
	require.NoError(t, e)
	run, _, e := s.claimProbe(ctx, 1, false)
	require.NoError(t, e)
	require.NotNil(t, run)
	_, e = db.Exec(`UPDATE group_probe_runs SET status='failed' WHERE status='running'`)
	require.NoError(t, e)
	_, _, e = s.claimProbe(ctx, 1, false)
	require.ErrorIs(t, e, ErrGroupProbeBudget)
	// Same request id cannot overwrite another user's or group's completion.
	at := time.Now().UTC()
	e = s.writeOutcomeBatch(ctx, []GroupRequestOutcome{{RequestID: "client:same", UserID: 1, GroupID: 1, Platform: "openai", Model: "gpt-5", Success: true, CompletedAt: at}, {RequestID: "client:same", UserID: 2, GroupID: 1, Platform: "openai", Model: "gpt-5", Success: false, CompletedAt: at}, {RequestID: "client:same", UserID: 1, GroupID: 2, Platform: "openai", Model: "gpt-5", Success: false, CompletedAt: at}})
	require.NoError(t, e)
	var count int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM channel_monitor_request_outcomes`).Scan(&count))
	require.Equal(t, 3, count)
	var success bool
	require.NoError(t, db.QueryRow(`SELECT success FROM channel_monitor_request_outcomes WHERE request_id='client:same' AND user_id=1 AND group_id=1`).Scan(&success))
	require.True(t, success)
}
func TestGroupProbePostgresExecutesCapabilityCapturesCostsAndStops(t *testing.T) {
	db := groupProbeTestDB(t)
	ctx := context.Background()
	s := NewGroupStatusService(groupProbeTestMonitor(), db, groupProbeGroups{}, groupProbeUsers{}, nil, nil, nil)
	defer s.Stop()
	_, e := s.SaveProbeConfig(ctx, GroupProbeConfig{GroupID: 1, Model: "gpt-5"}, 1)
	require.NoError(t, e)
	entered := make(chan struct{})
	release := make(chan struct{})
	s.SetProbeRunner(func(ctx context.Context, e GroupProbeExecution) GroupProbeResult {
		require.True(t, IsGroupProbe(ctx))
		require.Equal(t, int64(1), *e.Key.GroupID)
		close(entered)
		select {
		case <-release:
		case <-ctx.Done():
		}
		CaptureGroupProbeUsage(e.Key, &UsageLog{InputTokens: 10, OutputTokens: 2, TotalCost: .003})
		return GroupProbeResult{Success: true, LatencyMs: 25}
	})
	run, e := s.StartProbe(ctx, 1)
	require.NoError(t, e)
	<-entered
	_, e = s.StartProbe(ctx, 1)
	require.ErrorIs(t, e, ErrGroupProbeBusy)
	close(release)
	require.Eventually(t, func() bool {
		var status string
		_ = db.QueryRow(`SELECT status FROM group_probe_runs WHERE id=$1`, run.ID).Scan(&status)
		return status == "success"
	}, time.Second, 10*time.Millisecond)
	history, e := s.ProbeHistory(ctx, 1)
	require.NoError(t, e)
	require.Len(t, history, 1)
	require.True(t, history[0].UsageRecorded)
	require.Equal(t, int64(10), history[0].InputTokens)
	require.Equal(t, .003, *history[0].EstimatedCostUSD)
	s.Stop()
	_, e = s.StartProbe(ctx, 1)
	require.ErrorIs(t, e, ErrGroupProbeUnavailable)
}

func TestGroupProbePostgresDisabledConfigurationBlocksAdmissionAndDispatch(t *testing.T) {
	db := groupProbeTestDB(t)
	repo := &groupProbeMonitorRepo{}
	s := NewGroupStatusService(NewChannelMonitorV2Service(repo), db, groupProbeGroups{}, groupProbeUsers{}, nil, nil, nil)
	defer s.Stop()
	s.settings = channelMonitorV2RuntimeStub{rt: ChannelMonitorRuntime{Enabled: true, Mode: "v2"}}
	_, err := s.SaveProbeConfig(context.Background(), GroupProbeConfig{GroupID: 1, Model: "gpt-5", Enabled: true}, 1)
	require.NoError(t, err)
	var called atomic.Bool
	runner := func(context.Context, GroupProbeExecution) GroupProbeResult {
		called.Store(true)
		return GroupProbeResult{Success: true}
	}
	s.SetProbeRunner(runner)
	repo.disabled.Store(true)
	_, err = s.StartProbe(context.Background(), 1)
	require.ErrorIs(t, err, ErrChannelMonitorDisabled)
	_, err = s.startProbe(context.Background(), 1, true, false)
	require.ErrorIs(t, err, ErrChannelMonitorDisabled)
	s.probeTick()
	var count int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM group_probe_runs`).Scan(&count))
	require.Zero(t, count)
	status, err := s.Status(context.Background(), ChannelMonitorV2Filter{})
	require.NoError(t, err)
	require.False(t, status.Enabled)
	require.Empty(t, status.Groups)
	// A change after the durable claim is caught immediately before dispatch.
	repo.disabled.Store(false)
	run, cfg, err := s.claimProbe(context.Background(), 1, false)
	require.NoError(t, err)
	repo.disabled.Store(true)
	s.executeProbe(run, cfg, runner)
	require.False(t, called.Load())
	var runStatus, code string
	var reserved int
	require.NoError(t, db.QueryRow(`SELECT status,error_code,reserved_tokens FROM group_probe_runs WHERE id=$1`, run.ID).Scan(&runStatus, &code, &reserved))
	require.Equal(t, "skipped", runStatus)
	require.Equal(t, "monitoring_disabled", code)
	require.Zero(t, reserved)
}

func TestGroupProbePostgresDisableCancelsInFlightButRetainsUnknownSpend(t *testing.T) {
	db := groupProbeTestDB(t)
	repo := &groupProbeMonitorRepo{}
	s := NewGroupStatusService(NewChannelMonitorV2Service(repo), db, groupProbeGroups{}, groupProbeUsers{}, nil, nil, nil)
	defer s.Stop()
	_, err := s.SaveProbeConfig(context.Background(), GroupProbeConfig{GroupID: 1, Model: "gpt-5"}, 1)
	require.NoError(t, err)
	entered := make(chan struct{})
	s.SetProbeRunner(func(ctx context.Context, _ GroupProbeExecution) GroupProbeResult {
		close(entered)
		<-ctx.Done()
		return GroupProbeResult{ErrorCode: "timeout"}
	})
	run, err := s.StartProbe(context.Background(), 1)
	require.NoError(t, err)
	<-entered
	repo.disabled.Store(true)
	require.Eventually(t, func() bool {
		var status, code string
		var reserved int
		err := db.QueryRow(`SELECT status,error_code,reserved_tokens FROM group_probe_runs WHERE id=$1`, run.ID).Scan(&status, &code, &reserved)
		return err == nil && status == "skipped" && code == "monitoring_disabled" && reserved == 320
	}, 7*time.Second, 20*time.Millisecond)
}

func TestGroupProbePostgresTwoGroupsShareAdministratorWithoutSharingSlots(t *testing.T) {
	db := groupProbeTestDB(t)
	s := NewGroupStatusService(groupProbeTestMonitor(), db, groupProbeGroups{}, groupProbeUsers{}, nil, nil, nil)
	defer s.Stop()
	for _, id := range []int64{1, 2, 3} {
		_, err := s.SaveProbeConfig(context.Background(), GroupProbeConfig{GroupID: id, Model: "gpt-5"}, 1)
		require.NoError(t, err)
	}
	entered := make(chan int64, 2)
	release := make(chan struct{})
	s.SetProbeRunner(func(ctx context.Context, e GroupProbeExecution) GroupProbeResult {
		free, acquired := AcquireGroupProbeUserSlot(ctx)
		if !acquired {
			return GroupProbeResult{ErrorCode: "busy"}
		}
		defer free()
		entered <- e.Key.UserID
		select {
		case <-release:
		case <-ctx.Done():
		}
		return GroupProbeResult{Success: true}
	})
	for _, id := range []int64{1, 2} {
		_, err := s.StartProbe(context.Background(), id)
		require.NoError(t, err)
	}
	for range 2 {
		select {
		case actor := <-entered:
			require.Equal(t, int64(1), actor)
		case <-time.After(time.Second):
			t.Fatal("second probe was blocked by the administrator's first probe")
		}
	}
	_, err := s.StartProbe(context.Background(), 3)
	require.ErrorIs(t, err, ErrGroupProbeBusy, "the independent probe budget still caps concurrent work at two")
	close(release)
}

func TestGroupProbePostgresBatchedLatestAndBoundedMultiBatchMaintenance(t *testing.T) {
	db := groupProbeTestDB(t)
	s := NewGroupStatusService(groupProbeTestMonitor(), db, groupProbeGroups{}, groupProbeUsers{}, nil, nil, nil)
	defer s.Stop()
	_, err := db.Exec(`INSERT INTO group_probe_runs(id,group_id,model,status,checked_at,lease_until) VALUES
 ('first',1,'model-a','success',now()-interval '2 minutes',now()),
 ('latest',1,'model-b','failed',now()-interval '1 minute',now()),
 ('hidden',2,'model-a','success',now(),now());
 INSERT INTO group_probe_runs(id,group_id,model,status,checked_at,lease_until)
 SELECT 'old-'||n,1,'old','success',now()-interval '100 days',now()-interval '100 days' FROM generate_series(1,2501) n;
 INSERT INTO channel_monitor_request_outcomes(request_id,group_id,user_id,platform,model,success,completed_at)
 SELECT 'old-'||n,1,1,'openai','old',true,now()-interval '100 days' FROM generate_series(1,2501) n;
 INSERT INTO channel_monitor_request_outcomes(request_id,group_id,user_id,platform,model,success) VALUES('retained',1,1,'openai','new',true);`)
	require.NoError(t, err)
	latest, err := s.latestProbes(context.Background(), []int64{1, 3}, []string{"model-a", "model-b"})
	require.NoError(t, err)
	require.Len(t, latest, 1)
	require.Equal(t, "model-b", latest[1].Model)
	require.Equal(t, "failed", latest[1].Status)
	filtered, err := s.latestProbes(context.Background(), []int64{1}, []string{"model-a"})
	require.NoError(t, err)
	require.Equal(t, "model-a", filtered[1].Model)
	s.probeMaintenance()
	var n int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM channel_monitor_request_outcomes`).Scan(&n))
	require.Equal(t, 1, n)
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM group_probe_runs`).Scan(&n))
	require.Equal(t, 3, n)
}
