package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type intelligentPelicanSettingRepo struct {
	IntelligentTestRepository
	saved *IntelligentTestSetting
}

func (r *intelligentPelicanSettingRepo) IsAdmin(context.Context, int64) (bool, error) {
	return true, nil
}

func (r *intelligentPelicanSettingRepo) UpdateSetting(_ context.Context, _ int64, setting *IntelligentTestSetting) error {
	copy := *setting
	r.saved = &copy
	return nil
}

func TestIntelligentPelicanLegacySettingIgnoresAnswerGradingRules(t *testing.T) {
	repo := &intelligentPelicanSettingRepo{}
	svc := NewIntelligentTestService(repo, nil)
	legacy := IntelligentTestConfig{
		Prompt: "Draw a pelican", Model: "gpt-test", ReasoningEffort: "high", TimeoutSeconds: 60,
		Evaluator: "exact_answer", ExpectedAnswer: "", AnswerType: "old-invalid-type", AnswerUnitMode: "configured",
	}
	setting := &IntelligentTestSetting{TestType: "pelican", Enabled: true, Config: legacy}
	require.NoError(t, svc.UpdateSetting(context.Background(), 1, setting))
	require.NotNil(t, repo.saved)
	require.Equal(t, "svg_structure", repo.saved.Config.Evaluator)
	require.Empty(t, repo.saved.Config.ExpectedAnswer)
	require.Empty(t, repo.saved.Config.AnswerType)
	require.Empty(t, repo.saved.Config.AnswerUnitMode)
	require.Equal(t, legacy.Prompt, repo.saved.Config.Prompt)
	require.Equal(t, legacy.Model, repo.saved.Config.Model)
	require.Equal(t, "high", repo.saved.Config.ReasoningEffort)
	// The same missing-answer config remains invalid for a graded candy test.
	repo.saved = nil
	setting.TestType, setting.Config = "candy", legacy
	require.ErrorContains(t, svc.UpdateSetting(context.Background(), 1, setting), "expected_answer is required")
	require.Nil(t, repo.saved)
}

type intelligentPelicanLegacyRunner struct{ output string }

func (r intelligentPelicanLegacyRunner) RunIntelligentTest(_ context.Context, record *IntelligentTestRecord) error {
	record.Result = r.output
	return nil
}

func TestIntelligentPelicanLegacyQueuedSnapshotProducesOnlyImage(t *testing.T) {
	legacy := IntelligentTestConfig{Prompt: "Draw a pelican", Evaluator: "exact_answer", ExpectedAnswer: "12", AnswerType: "number", TimeoutSeconds: 60}
	svc := &IntelligentTestService{
		runner:     intelligentPelicanLegacyRunner{output: `<html><body><svg viewBox="0 0 10 10"><circle cx="5" cy="5" r="4"/></svg></body></html>`},
		evaluators: DefaultIntelligentTestEvaluators(),
	}
	record := &IntelligentTestRecord{TestType: "pelican", Status: "running", ConfigSnapshot: &legacy}
	svc.run(context.Background(), record)
	require.Equal(t, "completed", record.Status)
	require.Nil(t, record.Score)
	require.NotEmpty(t, record.ResultImage)
	require.Contains(t, record.ResultImage, "<svg")
	require.Equal(t, "svg_structure", record.Evaluation["method"])
	require.Equal(t, "exact_answer", record.ConfigSnapshot.Evaluator, "the original queued snapshot is retained for audit")

	svc.runner = intelligentPelicanLegacyRunner{output: "ANSWER: 12"}
	candy := &IntelligentTestRecord{TestType: "candy", Status: "running", ConfigSnapshot: &legacy}
	svc.run(context.Background(), candy)
	require.Equal(t, "completed", candy.Status)
	require.Equal(t, "correct", candy.Evaluation["answer_verdict"])
	require.Equal(t, 100.0, *candy.Score)
	require.Empty(t, candy.ResultImage)
}
