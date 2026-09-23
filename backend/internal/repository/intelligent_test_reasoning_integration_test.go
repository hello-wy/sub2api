package repository

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestIntelligentReasoningQueueSnapshotsOverridesAndConflicts(t *testing.T) {
	db := intelligentTestDB(t)
	repo := &intelligentTestRepository{db: db}
	ctx := context.Background()
	_, err := db.Exec(`UPDATE test_settings SET config=jsonb_set(config,'{reasoning_effort}','"high"') WHERE test_type='pelican'`)
	require.NoError(t, err)
	request := service.IntelligentTestEnqueue{AccountIDs: []int64{10}, TestTypes: []string{"pelican"}, IdempotencyKey: uuid.NewString()}
	first, err := repo.Enqueue(ctx, 1, request)
	require.NoError(t, err)
	stored, err := repo.Get(ctx, first.Records[0].ID)
	require.NoError(t, err)
	require.Equal(t, "high", stored.ConfigSnapshot.ReasoningEffort)

	// An idempotent key with a changed effort is a different request even if
	// its account, model and test type are otherwise identical.
	changed := request
	changed.ReasoningEfforts = map[string]string{"pelican": "low"}
	_, err = repo.Enqueue(ctx, 1, changed)
	require.ErrorIs(t, err, service.ErrIntelligentTestConflict)
	changed.IdempotencyKey = uuid.NewString()
	_, err = repo.Enqueue(ctx, 1, changed)
	require.ErrorContains(t, err, "不同模型或配置")
	changed.ReasoningEfforts["pelican"] = "high"
	reused, err := repo.Enqueue(ctx, 1, changed)
	require.NoError(t, err)
	require.Equal(t, 1, reused.ReusedCount)
	require.Equal(t, first.Records[0].ID, reused.Records[0].ID)

	// Missing inherits the setting; present empty explicitly resets to the
	// upstream default and survives JSON persistence.
	changed.AccountIDs = []int64{11}
	changed.IdempotencyKey = uuid.NewString()
	changed.ReasoningEfforts["pelican"] = ""
	defaultRun, err := repo.Enqueue(ctx, 1, changed)
	require.NoError(t, err)
	stored, err = repo.Get(ctx, defaultRun.Records[0].ID)
	require.NoError(t, err)
	require.Empty(t, stored.ConfigSnapshot.ReasoningEffort)

	changed.AccountIDs = []int64{12}
	changed.IdempotencyKey = uuid.NewString()
	changed.ReasoningEfforts["pelican"] = "low"
	lowRun, err := repo.Enqueue(ctx, 1, changed)
	require.NoError(t, err)
	stored, err = repo.Get(ctx, lowRun.Records[0].ID)
	require.NoError(t, err)
	require.Equal(t, "low", stored.ConfigSnapshot.ReasoningEffort)
	var saved string
	require.NoError(t, db.QueryRow(`SELECT config->>'reasoning_effort' FROM test_settings WHERE test_type='pelican'`).Scan(&saved))
	require.Equal(t, "high", saved, "one-off overrides must not update the saved setting")
}
