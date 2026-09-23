package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type codexTicketSettingRepo struct {
	SettingRepository
	values   map[string]string
	reads    int
	writes   int
	readErr  error
	writeErr error
}

func (r *codexTicketSettingRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	r.reads++
	if r.readErr != nil {
		return nil, r.readErr
	}
	values := map[string]string{}
	for _, key := range keys {
		if value, ok := r.values[key]; ok {
			values[key] = value
		}
	}
	return values, nil
}

func (r *codexTicketSettingRepo) SetMultiple(_ context.Context, values map[string]string) error {
	if r.writeErr != nil {
		return r.writeErr
	}
	if r.values == nil {
		r.values = map[string]string{}
	}
	for key, value := range values {
		r.values[key] = value
	}
	r.writes++
	return nil
}

func TestCodexTicketSettingsDefaultAndCache(t *testing.T) {
	ctx := context.Background()
	r := &codexTicketSettingRepo{}
	s := NewSettingService(r, nil)
	require.False(t, s.GetOpenAICodexTicketEnabled(ctx, false))
	require.Empty(t, s.GetOpenAICodexTicketHarvestProxyURL(ctx))
	require.True(t, s.GetOpenAICodexTicketEnabled(ctx, true), "only a missing explicit switch honors config fallback")
	require.Equal(t, 1, r.reads)
	state, err := s.GetCodexTicketSettings(ctx)
	require.NoError(t, err)
	require.Equal(t, &CodexTicketSettings{}, state)
}

func TestCodexTicketSettingsProxyIsWriteOnlyAndBlankPreserves(t *testing.T) {
	ctx := context.Background()
	r := &codexTicketSettingRepo{values: map[string]string{"unrelated": "keep"}}
	s := NewSettingService(r, nil)
	proxy := "http://private-user-{sid}:private-password@pool.example.test:8080"
	state, err := s.UpdateCodexTicketSettings(ctx, CodexTicketSettingsUpdate{Enabled: true, HarvestProxyURL: proxy})
	require.NoError(t, err)
	require.True(t, state.Enabled)
	require.True(t, state.HarvestProxyConfigured)
	data, err := json.Marshal(state)
	require.NoError(t, err)
	require.NotContains(t, string(data), "private-user")
	require.NotContains(t, string(data), "private-password")
	require.NotContains(t, string(data), `"harvest_proxy_url"`)
	require.Contains(t, state.HarvestProxyDisplay, "pool.example.test:8080")
	require.Equal(t, proxy, s.GetOpenAICodexTicketHarvestProxyURL(ctx))
	require.True(t, s.GetOpenAICodexTicketEnabled(ctx, false))
	state, err = s.UpdateCodexTicketSettings(ctx, CodexTicketSettingsUpdate{Enabled: false, HarvestProxyURL: "   "})
	require.NoError(t, err)
	require.False(t, state.Enabled)
	require.True(t, state.HarvestProxyConfigured)
	require.Equal(t, proxy, r.values[SettingKeyOpenAICodexTicketHarvestProxyURL])
	require.Equal(t, "keep", r.values["unrelated"])
	require.False(t, s.GetOpenAICodexTicketEnabled(ctx, true), "explicit disable overrides config fallback immediately")
}

func TestCodexTicketSettingsEnableAndClearValidation(t *testing.T) {
	ctx := context.Background()
	r := &codexTicketSettingRepo{}
	s := NewSettingService(r, nil)
	_, err := s.UpdateCodexTicketSettings(ctx, CodexTicketSettingsUpdate{Enabled: true})
	require.Error(t, err)
	require.Zero(t, r.writes)
	_, err = s.UpdateCodexTicketSettings(ctx, CodexTicketSettingsUpdate{Enabled: true, HarvestProxyURL: "socks5h://pool.example.test:1080"})
	require.NoError(t, err)
	_, err = s.UpdateCodexTicketSettings(ctx, CodexTicketSettingsUpdate{Enabled: true, ClearProxy: true})
	require.Error(t, err)
	require.Equal(t, 1, r.writes)
	state, err := s.UpdateCodexTicketSettings(ctx, CodexTicketSettingsUpdate{ClearProxy: true})
	require.NoError(t, err)
	require.Equal(t, &CodexTicketSettings{}, state)
	require.Empty(t, s.GetOpenAICodexTicketHarvestProxyURL(ctx))
	require.False(t, s.GetOpenAICodexTicketEnabled(ctx, true))
}

func TestCodexTicketSettingsRejectUnsafeOrMaskedURLs(t *testing.T) {
	for _, proxy := range []string{"file:///secret", "http://user:private-password@pool.example.test:99999", "http://user:private-password@pool.example.test/a", "http://user:private-password@pool.example.test/?secret=x", "http://user:***@pool.example.test:8080", "http://user:private-password@", strings.Repeat("x", 4097)} {
		t.Run(proxy[:min(40, len(proxy))], func(t *testing.T) {
			r := &codexTicketSettingRepo{}
			_, err := NewSettingService(r, nil).UpdateCodexTicketSettings(context.Background(), CodexTicketSettingsUpdate{HarvestProxyURL: proxy})
			require.Error(t, err)
			require.NotContains(t, err.Error(), "private-password")
			require.Zero(t, r.writes)
		})
	}
	r := &codexTicketSettingRepo{}
	_, err := NewSettingService(r, nil).UpdateCodexTicketSettings(context.Background(), CodexTicketSettingsUpdate{HarvestProxyURL: "http://pool.example.test:8080", ClearProxy: true})
	require.Error(t, err)
	require.Zero(t, r.writes)
}

func TestCodexTicketSettingsRuntimeFailsClosedOnReadFailure(t *testing.T) {
	ctx := context.Background()
	r := &codexTicketSettingRepo{values: map[string]string{SettingKeyOpenAICodexTicketEnabled: "true", SettingKeyOpenAICodexTicketHarvestProxyURL: "http://pool.example.test:8080"}}
	s := NewSettingService(r, nil)
	require.True(t, s.GetOpenAICodexTicketEnabled(ctx, false))
	r.readErr = errors.New("database failed with private-password")
	s.codexTicketSettingsCache.expiresAt = time.Now().Add(-time.Second)
	require.False(t, s.GetOpenAICodexTicketEnabled(ctx, true))
	require.Empty(t, s.GetOpenAICodexTicketHarvestProxyURL(ctx))
	require.Equal(t, 2, r.reads, "failed runtime reads are briefly cached")
	_, _, runtimeErr := s.GetOpenAICodexTicketRuntime(ctx, true)
	require.Error(t, runtimeErr, "runtime must distinguish a failed read from an explicit disabled setting")
	_, err := s.GetCodexTicketSettings(ctx)
	require.Error(t, err)
	require.NotContains(t, err.Error(), "private-password")
}

func TestCodexTicketSettingsFailedWriteDoesNotEnable(t *testing.T) {
	ctx := context.Background()
	r := &codexTicketSettingRepo{values: map[string]string{SettingKeyOpenAICodexTicketEnabled: "false"}, writeErr: errors.New("write private-password")}
	s := NewSettingService(r, nil)
	_, err := s.UpdateCodexTicketSettings(ctx, CodexTicketSettingsUpdate{Enabled: true, HarvestProxyURL: "http://pool.example.test:8080"})
	require.Error(t, err)
	require.NotContains(t, err.Error(), "private-password")
	require.False(t, s.GetOpenAICodexTicketEnabled(ctx, true))
	require.Empty(t, s.GetOpenAICodexTicketHarvestProxyURL(ctx))
}

func TestCodexTicketSettingsCorruptStoredProxyFailsClosed(t *testing.T) {
	r := &codexTicketSettingRepo{values: map[string]string{SettingKeyOpenAICodexTicketEnabled: "true", SettingKeyOpenAICodexTicketHarvestProxyURL: "malformed-private-password"}}
	s := NewSettingService(r, nil)
	require.False(t, s.GetOpenAICodexTicketEnabled(context.Background(), true))
	require.Empty(t, s.GetOpenAICodexTicketHarvestProxyURL(context.Background()))
	enabled, _, runtimeErr := s.GetOpenAICodexTicketRuntime(context.Background(), false)
	require.True(t, enabled)
	require.Error(t, runtimeErr, "invalid stored proxy must not bypass an opted-in account gate")
	state, err := s.GetCodexTicketSettings(context.Background())
	require.NoError(t, err)
	require.Empty(t, state.HarvestProxyDisplay)
}
