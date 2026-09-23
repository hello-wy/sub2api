//go:build unit

package repository

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestScheduledTestDeletePostgresOwnershipAtomicityAndScope(t *testing.T) {
	dsn := os.Getenv("GROUP_STATUS_POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("GROUP_STATUS_POSTGRES_TEST_DSN is not set")
	}
	admin, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer admin.Close()
	schema := "test_delete_" + uuid.NewString()[:8]
	_, err = admin.Exec("CREATE SCHEMA " + pq.QuoteIdentifier(schema))
	require.NoError(t, err)
	defer admin.Exec("DROP SCHEMA " + pq.QuoteIdentifier(schema) + " CASCADE")
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, err := sql.Open("postgres", u.String())
	require.NoError(t, err)
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE scheduled_test_plans(id bigint primary key,account_id bigint,model_id text default '',cron_expression text default '* * * * *',enabled boolean default true,max_results int default 50,auto_recover boolean default false,last_run_at timestamptz,next_run_at timestamptz,created_at timestamptz default now(),updated_at timestamptz default now());
CREATE TABLE scheduled_test_results(id bigint primary key,plan_id bigint references scheduled_test_plans(id));
CREATE TABLE usage_logs(id bigint primary key); CREATE TABLE finance_transactions(id bigint primary key);
INSERT INTO scheduled_test_plans(id,account_id) VALUES(1,10),(2,20); INSERT INTO scheduled_test_results VALUES(11,1),(12,1),(21,2); INSERT INTO usage_logs VALUES(1); INSERT INTO finance_transactions VALUES(1);`)
	require.NoError(t, err)
	svc := service.NewScheduledTestService(NewScheduledTestPlanRepository(db), NewScheduledTestResultRepository(db))
	ctx := context.Background()
	_, err = svc.DeleteResults(ctx, 1, []int64{11, 21})
	require.Error(t, err)
	var n int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM scheduled_test_results`).Scan(&n))
	require.Equal(t, 3, n, "mixed ownership must roll back the whole batch")
	for _, ids := range [][]int64{{}, {0}, {11, 999}, make([]int64, 101)} {
		_, err = svc.DeleteResults(ctx, 1, ids)
		require.Error(t, err)
	}
	deleted, err := svc.DeleteResults(ctx, 1, []int64{11, 11})
	require.NoError(t, err)
	require.EqualValues(t, 1, deleted)
	deleted, err = svc.DeleteResults(ctx, 1, []int64{12})
	require.NoError(t, err)
	require.EqualValues(t, 1, deleted)
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM scheduled_test_results WHERE plan_id=2`).Scan(&n))
	require.Equal(t, 1, n)
	for _, table := range []string{"scheduled_test_plans", "usage_logs", "finance_transactions"} {
		require.NoError(t, db.QueryRow("SELECT count(*) FROM "+table).Scan(&n))
		require.Positive(t, n)
	}
}
