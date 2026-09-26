package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type groupPelicanGroups struct {
	GroupRepository
	group *Group
}

func (r *groupPelicanGroups) GetByID(context.Context, int64) (*Group, error) { return r.group, nil }

type groupPelicanKeys struct {
	APIKeyRepository
	key *APIKey
}

func (r *groupPelicanKeys) GetByID(context.Context, int64) (*APIKey, error) { return r.key, nil }

func groupPelicanFixture() (*ScheduledTestService, *ScheduledTestPlan, *APIKey) {
	groupID := int64(17)
	key := &APIKey{ID: 23, Key: "test-secret-key", GroupID: &groupID, Status: StatusActive}
	svc := &ScheduledTestService{groupRepo: &groupPelicanGroups{group: &Group{ID: groupID, Status: StatusActive}}, keyRepo: &groupPelicanKeys{key: key}}
	plan := &ScheduledTestPlan{ID: 9, GroupID: groupID, APIKeyID: key.ID, Enabled: true, ModelID: "public-model", CronExpression: "*/30 * * * *", MaxResults: 5,
		PelicanConfig: &PelicanTestConfig{QuestionKind: "pelican", Prompt: "draw a pelican", ReasoningEffort: "high", ParallelCount: 1}}
	return svc, plan, key
}

func TestGroupPelicanUsesAuthenticatedGatewayRequest(t *testing.T) {
	svc, plan, key := groupPelicanFixture()
	calls := 0
	svc.SetGroupGateway(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		require.Equal(t, "/v1/chat/completions", r.URL.Path)
		require.Equal(t, "Bearer "+key.Key, r.Header.Get("Authorization"))
		require.Equal(t, plan.GroupID, ScheduledPelicanGroupID(r.Context()))
		require.Equal(t, "127.0.0.1:0", r.RemoteAddr)
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		require.Equal(t, "public-model", body["model"])
		require.Equal(t, true, body["stream"])
		require.Equal(t, "high", body["reasoning_effort"])
		require.Contains(t, fmt.Sprint(body["messages"]), PelicanDeliveryContract)
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"<svg></svg>\"},\"finish_reason\":null}]}\n\ndata: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
	}))
	result, err := svc.RunGroupPelican(context.Background(), plan)
	require.NoError(t, err)
	require.Equal(t, 1, calls)
	require.Equal(t, "success", result.Status)
	require.Equal(t, "<svg></svg>", result.ResponseText)
	require.Equal(t, plan.ModelID, result.PelicanConfig.ModelID)
	require.Empty(t, plan.PelicanConfig.ModelID, "history snapshot must not mutate the plan")
}

func TestGroupPelicanRejectsInvalidTargetsAndAllowsPause(t *testing.T) {
	for _, mutate := range []func(*ScheduledTestPlan, *APIKey){
		func(p *ScheduledTestPlan, _ *APIKey) { p.AccountID = 1 },
		func(p *ScheduledTestPlan, _ *APIKey) { p.AutoRecover = true },
		func(p *ScheduledTestPlan, _ *APIKey) { p.PelicanConfig = nil },
		func(p *ScheduledTestPlan, _ *APIKey) { p.PelicanConfig.QuestionKind = "candy" },
		func(p *ScheduledTestPlan, _ *APIKey) { p.APIKeyID = 0 },
		func(_ *ScheduledTestPlan, k *APIKey) { id := int64(18); k.GroupID = &id },
		func(_ *ScheduledTestPlan, k *APIKey) { k.Status = "inactive" },
		func(_ *ScheduledTestPlan, k *APIKey) { expired := time.Now().Add(-time.Minute); k.ExpiresAt = &expired },
	} {
		svc, plan, key := groupPelicanFixture()
		mutate(plan, key)
		require.Error(t, svc.validatePlanTarget(context.Background(), plan))
	}
	svc, plan, key := groupPelicanFixture()
	require.NoError(t, svc.validatePlanTarget(context.Background(), plan))
	key.Status = "inactive"
	plan.Enabled = false
	require.NoError(t, svc.validatePlanTarget(context.Background(), plan), "an invalid key must not block pausing")
	svc.SetGroupGateway(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("invalid credential reached gateway") }))
	_, err := svc.RunGroupPelican(context.Background(), plan)
	require.Error(t, err)
}

func TestGroupPelicanIncompleteAndErrorStreamsNeverPublishAsSuccess(t *testing.T) {
	for _, body := range []string{
		"data: {\"choices\":[{\"delta\":{\"content\":\"<svg/>\"}}]}\n\ndata: [DONE]\n",
		"data: {\"choices\":[{\"delta\":{\"content\":\"<svg/>\"},\"finish_reason\":\"length\"}]}\n",
		"data: {\"error\":{\"message\":\"upstream failed\"}}\n",
		"data: not-json\n",
	} {
		_, message := parseGroupPelicanOutput([]byte(body))
		require.NotEmpty(t, message)
	}
	svc, plan, key := groupPelicanFixture()
	svc.SetGroupGateway(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"message": "invalid " + key.Key}}))
	}))
	result, err := svc.RunGroupPelican(context.Background(), plan)
	require.NoError(t, err)
	require.Equal(t, "failed", result.Status)
	require.Contains(t, result.ErrorMessage, "HTTP 403")
	require.NotContains(t, result.ErrorMessage, key.Key)
}

func TestGroupPelicanRunnerUsesGroupGatewayAndSavesHistory(t *testing.T) {
	svc, plan, _ := groupPelicanFixture()
	plans, results := &pelicanPlanRepo{}, &pelicanResults{}
	svc.planRepo, svc.resultRepo = plans, results
	svc.SetGroupGateway(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"<svg></svg>\"},\"finish_reason\":\"stop\"}]}\n")
	}))
	runner := &ScheduledTestRunnerService{planRepo: plans, scheduledSvc: svc, runPelican: func(context.Context, int64, string, *PelicanTestConfig) (*ScheduledTestResult, error) {
		t.Fatal("group plan must not invoke direct account test")
		return nil, nil
	}}
	runner.runOnePlan(context.Background(), plan)
	runner.runOnePlan(context.Background(), plan)
	require.Len(t, results.results, 1, "database lease blocks duplicate execution")
	require.Equal(t, plan.ID, results.results[0].PlanID)
	require.Equal(t, "success", results.results[0].Status)
	require.True(t, plans.finished)
}
