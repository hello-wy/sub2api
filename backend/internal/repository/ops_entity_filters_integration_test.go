//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestOpsErrorEntityFiltersChangeRowsAndTotals(t *testing.T) {
	ctx := context.Background()
	repo := NewOpsRepository(integrationDB)
	start := time.Date(2040, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	for i, row := range []struct {
		user, account, group int64
		phase                string
		status               int
	}{
		{7, 839, 3, "upstream", 499}, {8, 839, 3, "upstream", 502}, {7, 840, 3, "upstream", 499}, {7, 839, 4, "upstream", 499}, {7, 839, 3, "account_auth", 401},
	} {
		var id int64
		err := integrationDB.QueryRowContext(ctx, `INSERT INTO ops_error_logs (user_id, account_id, group_id, error_phase, error_type, error_owner, status_code, upstream_status_code, created_at) VALUES ($1,$2,$3,$4,'upstream_error','provider',200,$5,$6) RETURNING id`, row.user, row.account, row.group, row.phase, row.status, start.Add(time.Duration(i)*time.Second)).Scan(&id)
		require.NoError(t, err)
		t.Cleanup(func() { _, _ = integrationDB.ExecContext(ctx, "DELETE FROM ops_error_logs WHERE id=$1", id) })
	}
	f := &service.OpsErrorLogFilter{StartTime: &start, EndTime: &end, View: "all", Owner: "provider", ErrorPhasesAny: []string{"upstream", "account_auth"}, IncludeRecoveredUpstream: true}
	assertTotal := func(want int) {
		t.Helper()
		result, err := repo.ListErrorLogs(ctx, f)
		require.NoError(t, err)
		require.Equal(t, want, result.Total)
		require.Len(t, result.Errors, want)
	}
	assertTotal(5)
	user, account, group := int64(7), int64(839), int64(3)
	f.UserID = &user
	assertTotal(4)
	f.AccountID = &account
	assertTotal(3)
	f.GroupID = &group
	assertTotal(2)
	f.Phase = "upstream"
	assertTotal(1)
	f.StatusCodesOther = true
	assertTotal(1)
	f.StatusCodes = []int{502}
	assertTotal(0)
}
