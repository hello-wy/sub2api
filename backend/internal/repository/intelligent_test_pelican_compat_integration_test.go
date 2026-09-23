package repository

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestIntelligentPelicanLegacySettingsNormalizeReadSaveAndQueue(t *testing.T) {
	db := intelligentTestDB(t)
	repo := &intelligentTestRepository{db: db}
	ctx := context.Background()
	_, err := db.Exec(`UPDATE test_settings SET config=config || '{"evaluator":"exact_answer","expected_answer":"","answer_type":"old-invalid-type","answer_unit_mode":"configured","reasoning_effort":"high"}'::jsonb WHERE test_type='pelican'`)
	require.NoError(t, err)
	var raw []byte
	require.NoError(t, db.QueryRow(`SELECT config FROM test_settings WHERE test_type='pelican'`).Scan(&raw))
	var legacy service.IntelligentTestConfig
	require.NoError(t, json.Unmarshal(raw, &legacy))
	settings, err := repo.Settings(ctx)
	require.NoError(t, err)
	var pelican service.IntelligentTestSetting
	for _, setting := range settings {
		if setting.TestType == "pelican" {
			pelican = setting
			require.Equal(t, "svg_structure", setting.Config.Evaluator)
			require.Empty(t, setting.Config.AnswerType)
			require.Empty(t, setting.Config.ExpectedAnswer)
		} else if setting.TestType == "candy" {
			require.Equal(t, "exact_answer", setting.Config.Evaluator)
			require.Equal(t, "12", setting.Config.ExpectedAnswer)
		}
	}
	request := service.IntelligentTestEnqueue{AccountIDs: []int64{10}, TestTypes: []string{"pelican"}, IdempotencyKey: uuid.NewString()}
	queued, err := repo.Enqueue(ctx, 1, request)
	require.NoError(t, err)
	record, err := repo.Get(ctx, queued.Records[0].ID)
	require.NoError(t, err)
	require.Equal(t, "svg_structure", record.ConfigSnapshot.Evaluator)
	require.Empty(t, record.ConfigSnapshot.AnswerType)
	require.Equal(t, "high", record.ConfigSnapshot.ReasoningEffort)
	// A pre-upgrade queued snapshot has the same effective configuration and
	// must be reused, without rewriting its original private configuration.
	_, err = db.Exec(`UPDATE account_tests SET config_snapshot=$2 WHERE id=$1`, record.ID, string(raw))
	require.NoError(t, err)
	request.IdempotencyKey = uuid.NewString()
	reused, err := repo.Enqueue(ctx, 1, request)
	require.NoError(t, err)
	require.Equal(t, 1, reused.ReusedCount)
	require.Zero(t, reused.CreatedCount)
	pelican.Config = legacy
	require.NoError(t, service.NewIntelligentTestService(repo, nil).UpdateSetting(ctx, 1, &pelican))
	var evaluator string
	require.NoError(t, db.QueryRow(`SELECT config->>'evaluator' FROM test_settings WHERE test_type='pelican'`).Scan(&evaluator))
	require.Equal(t, "svg_structure", evaluator)
	require.NoError(t, db.QueryRow(`SELECT config_snapshot->>'evaluator' FROM account_tests WHERE id=$1`, record.ID).Scan(&evaluator))
	require.Equal(t, "exact_answer", evaluator, "saving settings does not rewrite existing records")
}
