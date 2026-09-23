package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGroupStatusVisibilityUsesServerPermissionsNotOldMonitorPicker(t *testing.T) {
	g := Group{ID: 7, Platform: PlatformOpenAI}
	cfg := ChannelMonitorV2Config{GroupIDs: []int64{99}}
	require.True(t, groupStatusVisible(g, ChannelMonitorV2Filter{}, cfg))
	require.False(t, groupStatusVisible(g, ChannelMonitorV2Filter{RestrictGroups: true}, cfg))
	require.False(t, groupStatusVisible(g, ChannelMonitorV2Filter{RestrictGroups: true, AllowedGroupIDs: []int64{8}, GroupIDs: []int64{7}}, cfg))
	require.True(t, groupStatusVisible(g, ChannelMonitorV2Filter{RestrictGroups: true, AllowedGroupIDs: []int64{7}}, cfg))
}
func TestGroupStatusMetricsIgnoreClientErrorsAndWeightGenerationSamples(t *testing.T) {
	a := &groupStatusAccumulator{}
	speed1, speed2 := 20.0, 40.0
	ttft := 1500.0
	a.add(ChannelMonitorV2Metric{SuccessRequests: 10, ErrorRequests: 5, EvaluatedErrorRequests: 0, GenerationTokensPerSecond: &speed1, GenerationSampleCount: 1, TTFT: ChannelMonitorV2Latency{AvgMs: &ttft, SampleCount: 10}})
	a.add(ChannelMonitorV2Metric{SuccessRequests: 10, ErrorRequests: 1, EvaluatedErrorRequests: 1, GenerationTokensPerSecond: &speed2, GenerationSampleCount: 3})
	m := a.metrics()
	require.InDelta(t, 20.0/21, *m.SuccessRate, 0.000001)
	require.Equal(t, 35.0, *m.OutputTokensPerSecond)
	require.Equal(t, ttft, *m.TTFTMs)
	require.Equal(t, "unknown", (&groupStatusAccumulator{}).status(DefaultChannelMonitorV2HealthThresholds()))
	require.Equal(t, "unavailable", (&groupStatusAccumulator{errors: 1}).status(DefaultChannelMonitorV2HealthThresholds()))
}
func TestGroupProbeConfigBoundsAndDefaults(t *testing.T) {
	c := GroupProbeConfig{GroupID: 1, Model: "gpt-5"}
	require.NoError(t, ValidateGroupProbeConfig(&c))
	require.False(t, c.Enabled)
	require.Equal(t, 300, c.IntervalSeconds)
	for _, mutate := range []func(*GroupProbeConfig){func(c *GroupProbeConfig) { c.TimeoutSeconds = 121 }, func(c *GroupProbeConfig) { c.IntervalSeconds = 5 }, func(c *GroupProbeConfig) { c.MaxOutputTokens = 8192 }, func(c *GroupProbeConfig) { c.DailyTokenBudget = 256 }, func(c *GroupProbeConfig) { c.ReasoningEffort = "unlimited" }} {
		bad := c
		mutate(&bad)
		require.Error(t, ValidateGroupProbeConfig(&bad))
	}
}
func TestGroupProbeCapabilityCannotBeSerialized(t *testing.T) {
	key := &APIKey{ID: 1, groupProbe: &groupProbeCapture{}}
	require.Nil(t, GroupProbeKeyFromContext(context.Background()))
	data, e := json.Marshal(key)
	require.NoError(t, e)
	require.NotContains(t, string(data), "groupProbe")
	ctx := context.WithValue(context.Background(), groupProbeContextKey{}, key)
	require.True(t, IsGroupProbe(ctx))
	require.True(t, CaptureGroupProbeUsage(key, &UsageLog{InputTokens: 10, OutputTokens: 3, TotalCost: .004}))
	require.Equal(t, int64(10), key.groupProbe.input)
	require.True(t, key.groupProbe.seen)
	var restored APIKey
	require.NoError(t, json.Unmarshal(data, &restored))
	require.False(t, CaptureGroupProbeUsage(&restored, &UsageLog{}))
}

func TestGroupProbePrivateSlotsAreIndependentAndReleaseOnce(t *testing.T) {
	contexts := []context.Context{}
	for range 2 {
		key := &APIKey{UserID: 17, groupProbe: &groupProbeCapture{}}
		contexts = append(contexts, context.WithValue(context.Background(), groupProbeContextKey{}, key))
	}
	release1, acquired := AcquireGroupProbeUserSlot(contexts[0])
	require.True(t, acquired)
	release2, acquired := AcquireGroupProbeUserSlot(contexts[1])
	require.True(t, acquired, "the same administrator can run two independently admitted groups")
	_, acquired = AcquireGroupProbeUserSlot(contexts[0])
	require.False(t, acquired, "one internal capability cannot multiply its gateway admission")
	release1()
	release3, acquired := AcquireGroupProbeUserSlot(contexts[0])
	require.True(t, acquired)
	release1()
	_, acquired = AcquireGroupProbeUserSlot(contexts[0])
	require.False(t, acquired, "stale release cannot clear a newer lease")
	release2()
	release3()
	_, acquired = AcquireGroupProbeUserSlot(context.Background())
	require.False(t, acquired)
}
func TestGroupStatusPublicDTOHasNoTopologyOrConfiguration(t *testing.T) {
	b, e := json.Marshal(GroupStatusCard{GroupID: 1, GroupName: "public", Platform: "openai", Status: "unknown"})
	require.NoError(t, e)
	for _, secret := range []string{"account", "proxy", "channel", "request_count", "budget", "error_code", "credentials"} {
		require.NotContains(t, string(b), secret)
	}
}
