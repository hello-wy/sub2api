package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestIntelligentScheduleValidation(t *testing.T) {
	for _, tc := range []struct {
		name     string
		schedule *IntelligentTestSchedule
		valid    bool
	}{
		{"older client", nil, true},
		{"disabled default", &IntelligentTestSchedule{}, true},
		{"minimum interval", &IntelligentTestSchedule{Enabled: true, IntervalMinutes: 5, AccountIDs: []int64{1}}, true},
		{"weekly interval", &IntelligentTestSchedule{Enabled: true, IntervalMinutes: 10080, AccountIDs: []int64{1}}, true},
		{"too frequent", &IntelligentTestSchedule{Enabled: true, IntervalMinutes: 4, AccountIDs: []int64{1}}, false},
		{"too long", &IntelligentTestSchedule{Enabled: true, IntervalMinutes: 10081, AccountIDs: []int64{1}}, false},
		{"no explicit accounts", &IntelligentTestSchedule{Enabled: true, IntervalMinutes: 5}, false},
		{"duplicate accounts", &IntelligentTestSchedule{Enabled: true, IntervalMinutes: 5, AccountIDs: []int64{1, 1}}, false},
		{"invalid account", &IntelligentTestSchedule{Enabled: true, IntervalMinutes: 5, AccountIDs: []int64{0}}, false},
		{"too many accounts", &IntelligentTestSchedule{Enabled: true, IntervalMinutes: 5, AccountIDs: make([]int64, 101)}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setting := &IntelligentTestSetting{Enabled: true, Schedule: tc.schedule}
			err := validateIntelligentTestSchedule(setting)
			if tc.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestIntelligentScheduleNormalizesWritableFields(t *testing.T) {
	forged := time.Now().Add(-time.Hour)
	setting := &IntelligentTestSetting{Enabled: true, Schedule: &IntelligentTestSchedule{
		Enabled: true, IntervalMinutes: 5, AccountIDs: []int64{3, 1, 2},
		NextRunAt: &forged, LastRunAt: &forged, LastError: "forged",
	}}
	require.NoError(t, validateIntelligentTestSchedule(setting))
	require.Equal(t, []int64{1, 2, 3}, setting.Schedule.AccountIDs)
	require.Nil(t, setting.Schedule.NextRunAt)
	require.Nil(t, setting.Schedule.LastRunAt)
	require.Empty(t, setting.Schedule.LastError)
	setting.Enabled = false
	require.NoError(t, validateIntelligentTestSchedule(setting))
	require.False(t, setting.Schedule.Enabled, "disabling the test also disables future scheduled work")
	setting.Schedule = &IntelligentTestSchedule{}
	require.NoError(t, validateIntelligentTestSchedule(setting))
	require.Equal(t, 60, setting.Schedule.IntervalMinutes)
	require.NotNil(t, setting.Schedule.AccountIDs)
}

type intelligentScheduleLoopRepo struct {
	cancel  context.CancelFunc
	calls   int
	limit   int
	bounded bool
}

func (r *intelligentScheduleLoopRepo) EnqueueDueSchedules(ctx context.Context, limit int) (int, error) {
	r.calls++
	r.limit = limit
	_, r.bounded = ctx.Deadline()
	r.cancel()
	return 0, nil
}

func TestIntelligentScheduleLoopStartsBoundedAndStops(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	repo := &intelligentScheduleLoopRepo{cancel: cancel}
	s := &IntelligentTestService{}
	s.runScheduleLoop(ctx, repo)
	require.Equal(t, 1, repo.calls)
	require.Equal(t, 10, repo.limit)
	require.True(t, repo.bounded)
	s.runScheduleLoop(ctx, repo)
	require.Equal(t, 1, repo.calls, "a stopped service cannot enqueue another interval")
}

type intelligentScheduleShutdownRepo struct {
	IntelligentTestRepository
	started chan struct{}
	once    sync.Once
}

func (r *intelligentScheduleShutdownRepo) EnqueueDueSchedules(ctx context.Context, _ int) (int, error) {
	r.once.Do(func() { close(r.started) })
	<-ctx.Done()
	return 0, ctx.Err()
}

func (r *intelligentScheduleShutdownRepo) Claim(context.Context) (*IntelligentTestRecord, error) {
	return nil, nil
}

func TestIntelligentScheduleServiceStopCancelsPendingDispatch(t *testing.T) {
	repo := &intelligentScheduleShutdownRepo{started: make(chan struct{})}
	s := NewIntelligentTestService(repo, nil)
	s.Start()
	defer s.Stop()
	select {
	case <-repo.started:
	case <-time.After(2 * time.Second):
		t.Fatal("service did not start the persisted scheduler")
	}
	stopped := make(chan struct{})
	go func() {
		s.Stop()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("stopping the service did not cancel the scheduler")
	}
}
