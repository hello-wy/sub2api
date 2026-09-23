//go:build unit

package repository

import (
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func Test429ObservationPostgresSchedulingAndGroupCountsAreReversible(t *testing.T) {
	r, c, ctx := ipChannelIntegrationRepo(t)
	g, err := c.Group.Create().SetName("429-toggle").SetPlatform(service.PlatformOpenAI).Save(ctx)
	require.NoError(t, err)
	accounts := createIPChannelFixture(t, r, c, ctx, "429-toggle", 5)
	for _, a := range accounts {
		require.NoError(t, r.BindGroups(ctx, a.ID, []int64{g.ID}))
	}
	until := time.Now().Add(time.Hour)
	_, err = r.sql.ExecContext(ctx, `UPDATE accounts SET rate_limit_reset_at=$1 WHERE id=$2`, until, accounts[0].ID)
	require.NoError(t, err)
	for i, reason := range []string{`{"status_code":429,"error_message":"limited"}`, `OAuth 401: refresh required`, `manual pause`, `{"source":"account_scheduling_threshold","error_message":"5h reached"}`} {
		_, err = r.sql.ExecContext(ctx, `UPDATE accounts SET temp_unschedulable_until=$1,temp_unschedulable_reason=$2 WHERE id=$3`, until, reason, accounts[i+1].ID)
		require.NoError(t, err)
	}
	groups := newGroupRepositoryWithSQL(c, r.sql)
	for _, on := range []bool{true, false, true} {
		policyCtx := service.With429Enforcement(ctx, on)
		rows, err := r.ListSchedulableByGroupID(policyCtx, g.ID)
		require.NoError(t, err)
		counts, err := groups.loadAccountCounts(policyCtx, []int64{g.ID})
		require.NoError(t, err)
		if on {
			require.Empty(t, rows)
			require.EqualValues(t, 0, counts[g.ID].Active)
		} else {
			require.Len(t, rows, 3)
			require.EqualValues(t, 3, counts[g.ID].Active)
			for _, a := range rows {
				require.True(t, a.IsSchedulableWithContext(policyCtx), "SQL and Go use the same request policy")
			}
		}
	}
	stored, err := r.GetByID(ctx, accounts[0].ID)
	require.NoError(t, err)
	require.NotNil(t, stored.RateLimitResetAt, "switch changes routing without deleting observed429 records")
}
