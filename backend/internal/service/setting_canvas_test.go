//go:build unit

package service

import (
	"context"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
	"testing"
)

type canvasSettingsRepo struct{ authSourceDefaultsRepoStub }

func (r *canvasSettingsRepo) Set(_ context.Context, key, value string) error {
	r.values[key] = value
	return nil
}

func TestCanvasAPISettings(t *testing.T) {
	ctx := context.Background()
	repo := &canvasSettingsRepo{authSourceDefaultsRepoStub{values: map[string]string{SettingKeyAPIBaseURL: "https://global.example/"}}}
	svc := NewSettingService(repo, &config.Config{})
	inherited, err := svc.GetCanvasSettings(ctx)
	require.NoError(t, err)
	require.Empty(t, inherited.APIBaseURL)
	require.Equal(t, "https://global.example", inherited.EffectiveAPIBaseURL)
	require.NoError(t, svc.UpdateCanvasSettings(ctx, CanvasSettings{APIBaseURL: " https://canvas-api.example/proxy/ ", EffectiveAPIBaseURL: "https://forged.example"}))
	saved, err := svc.GetCanvasSettings(ctx)
	require.NoError(t, err)
	require.Equal(t, "https://canvas-api.example/proxy", saved.APIBaseURL)
	require.Equal(t, saved.APIBaseURL, saved.EffectiveAPIBaseURL)
	require.NotContains(t, repo.values[SettingKeyCanvas], "forged")
	require.NotContains(t, repo.values[SettingKeyCanvas], "effective_api_base_url")
	for _, address := range []string{"/relative", "javascript:alert(1)", "https://user:pass@api.example", "https://api.example?key=secret", "https://api.example?", "https://api.example#token", "https://api.example#", "https://"} {
		t.Run(address, func(t *testing.T) {
			require.Error(t, svc.UpdateCanvasSettings(ctx, CanvasSettings{APIBaseURL: address}))
			unchanged, err := svc.GetCanvasSettings(ctx)
			require.NoError(t, err)
			require.Equal(t, saved, unchanged)
		})
	}
	require.NoError(t, svc.UpdateCanvasSettings(ctx, CanvasSettings{}))
	cleared, err := svc.GetCanvasSettings(ctx)
	require.NoError(t, err)
	require.Equal(t, "https://global.example", cleared.EffectiveAPIBaseURL)
	delete(repo.values, SettingKeyAPIBaseURL)
	fallback, err := svc.GetCanvasSettings(ctx)
	require.NoError(t, err)
	require.Empty(t, fallback.EffectiveAPIBaseURL)
}
func TestCanvasSettings(t *testing.T) {
	ctx := context.Background()
	repo := &canvasSettingsRepo{authSourceDefaultsRepoStub{values: map[string]string{}}}
	svc := NewSettingService(repo, &config.Config{})
	initial, err := svc.GetCanvasSettings(ctx)
	require.NoError(t, err)
	require.False(t, initial.Enabled)
	settings := CanvasSettings{Enabled: true, URL: "https://canvas.example/app"}
	require.NoError(t, svc.UpdateCanvasSettings(ctx, settings))
	saved, err := svc.GetCanvasSettings(ctx)
	require.NoError(t, err)
	require.Equal(t, settings, saved)
	for _, url := range []string{"", "javascript:alert(1)", "https://user:secret@canvas.example", "https://canvas.example/#secret"} {
		require.Error(t, svc.UpdateCanvasSettings(ctx, CanvasSettings{Enabled: true, URL: url}))
	}
	unchanged, err := svc.GetCanvasSettings(ctx)
	require.NoError(t, err)
	require.Equal(t, saved, unchanged)
	require.NoError(t, svc.UpdateCanvasSettings(ctx, CanvasSettings{}))
	disabled, err := svc.GetCanvasSettings(ctx)
	require.NoError(t, err)
	require.False(t, disabled.Enabled)
}
