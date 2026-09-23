//go:build unit

package repository

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAccountUsageDurationPostgresMeasuredSamples(t *testing.T) {
	accounts, client, ctx := openAICredentialGroupIntegrationRepo(t)
	repo := newUsageLogRepositoryWithSQL(client, accounts.sql)
	// These usage-statistics columns are owned by SQL migrations, not Ent.
	_, err := accounts.sql.ExecContext(ctx, `ALTER TABLE usage_logs
		ADD COLUMN IF NOT EXISTS account_stats_cost NUMERIC(20,10),
		ADD COLUMN IF NOT EXISTS inbound_endpoint VARCHAR(128),
		ADD COLUMN IF NOT EXISTS upstream_endpoint VARCHAR(128)`)
	require.NoError(t, err)
	user, err := client.User.Create().SetEmail("duration-samples@test.com").SetPasswordHash("unused-test-hash").Save(ctx)
	require.NoError(t, err)
	apiKey, err := client.APIKey.Create().SetUserID(user.ID).SetKey("sk-duration-samples").SetName("duration-samples").Save(ctx)
	require.NoError(t, err)
	account := createGroupTestAccount(t, ctx, client, "duration-samples", "rt", "at")
	now := time.Now().UTC()
	zero, measured := 0, 1000
	for i, duration := range []*int{nil, &zero, &measured} {
		_, err := client.UsageLog.Create().SetUserID(user.ID).SetAPIKeyID(apiKey.ID).
			SetAccountID(account.ID).SetRequestID(fmt.Sprintf("duration-%d", i)).
			SetModel("gpt-5").SetNillableDurationMs(duration).SetCreatedAt(now).Save(ctx)
		require.NoError(t, err)
	}
	stats, err := repo.GetAccountUsageStats(ctx, account.ID, now.Add(-time.Hour), now.Add(time.Hour))
	require.NoError(t, err)
	require.Equal(t, int64(3), stats.Summary.TotalRequests)
	require.Equal(t, int64(2), stats.Summary.DurationSamples)
	require.Equal(t, 500.0, stats.Summary.AvgDurationMs)
}
