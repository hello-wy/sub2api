//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type canvasSettingsRepo struct{ authSourceDefaultsRepoStub }

func (r *canvasSettingsRepo) Set(_ context.Context, key, value string) error {
	r.values[key] = value
	return nil
}

func TestCanvasSettings(t *testing.T) {
	ctx := context.Background()
	repo := &canvasSettingsRepo{authSourceDefaultsRepoStub{values: map[string]string{}}}
	svc := NewSettingService(repo, &config.Config{})
	initial, err := svc.GetCanvasSettings(ctx)
	require.NoError(t, err)
	require.False(t, initial.Enabled)
	require.Equal(t, 5, initial.UpstreamRetryCount)
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

func TestCanvasRetrySettings(t *testing.T) {
	ctx := context.Background()
	repo := &canvasSettingsRepo{authSourceDefaultsRepoStub{values: map[string]string{SettingKeyCanvas: `{"enabled":true,"url":"https://canvas.example"}`}}}
	svc := NewSettingService(repo, &config.Config{})
	settings, err := svc.GetCanvasSettings(ctx)
	require.NoError(t, err)
	require.Equal(t, 5, settings.UpstreamRetryCount)
	for _, count := range []int{0, 5, 2} {
		settings.UpstreamRetryCount = count
		require.NoError(t, svc.UpdateCanvasSettings(ctx, settings))
		saved, err := svc.GetCanvasSettings(ctx)
		require.NoError(t, err)
		require.Equal(t, count, saved.UpstreamRetryCount)
	}
	for _, count := range []int{-1, 6} {
		settings.UpstreamRetryCount = count
		require.Error(t, svc.UpdateCanvasSettings(ctx, settings))
		saved, err := svc.GetCanvasSettings(ctx)
		require.NoError(t, err)
		require.Equal(t, 2, saved.UpstreamRetryCount)
	}
}

func TestCanvasFrameSrcOrigins(t *testing.T) {
	for _, tt := range []struct {
		name   string
		canvas string
		want   []string
	}{
		{"enabled", `{"enabled":true,"url":"https://canvas.example/editor?view=canvas"}`, []string{"https://canvas.example"}},
		{"disabled", `{"enabled":false,"url":"https://canvas.example/editor"}`, nil},
		{"unset", "", nil},
		{"port", `{"enabled":true,"url":"http://localhost:3010/canvas"}`, []string{"http://localhost:3010"}},
		{"duplicate", `{"enabled":true,"url":"https://home.example/canvas"}`, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo := &canvasSettingsRepo{authSourceDefaultsRepoStub{values: map[string]string{
				SettingKeyCanvas:                      tt.canvas,
				SettingKeyHomeContent:                 "https://home.example/start",
				SettingKeyPurchaseSubscriptionEnabled: "true",
				SettingKeyPurchaseSubscriptionURL:     "https://shop.example/plans",
				SettingKeyCustomMenuItems:             `[{"url":"https://menu.example/page"},{"url":"https://home.example/other"}]`,
			}}}
			svc := NewSettingService(repo, &config.Config{})
			origins, err := svc.GetFrameSrcOrigins(context.Background())
			require.NoError(t, err)
			want := append([]string{"https://home.example", "https://shop.example", "https://menu.example"}, tt.want...)
			require.Equal(t, want, origins)
		})
	}
}

func TestCanvasSettingsRefreshFrameSrcOrigins(t *testing.T) {
	ctx := context.Background()
	repo := &canvasSettingsRepo{authSourceDefaultsRepoStub{values: map[string]string{}}}
	svc := NewSettingService(repo, &config.Config{})
	var cachedOrigins []string
	calls := 0
	svc.SetOnUpdateCallback(func() {
		calls++
		var err error
		cachedOrigins, err = svc.GetFrameSrcOrigins(ctx)
		require.NoError(t, err)
	})
	require.NoError(t, svc.UpdateCanvasSettings(ctx, CanvasSettings{Enabled: true, URL: "https://first.example/canvas"}))
	require.Equal(t, []string{"https://first.example"}, cachedOrigins)
	require.NoError(t, svc.UpdateCanvasSettings(ctx, CanvasSettings{Enabled: true, URL: "https://second.example:8443/app"}))
	require.Equal(t, []string{"https://second.example:8443"}, cachedOrigins)
	require.NoError(t, svc.UpdateCanvasSettings(ctx, CanvasSettings{URL: "https://second.example:8443/app"}))
	require.Empty(t, cachedOrigins)
	require.Equal(t, 3, calls)
	require.Error(t, svc.UpdateCanvasSettings(ctx, CanvasSettings{Enabled: true, URL: "javascript:alert(1)"}))
	require.Equal(t, 3, calls)
}

type failingCanvasSettingsRepo struct {
	canvasSettingsRepo
	err error
}

func (r *failingCanvasSettingsRepo) Set(context.Context, string, string) error {
	return r.err
}

func (r *failingCanvasSettingsRepo) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	for _, key := range keys {
		if key == SettingKeyCanvas {
			return nil, r.err
		}
	}
	return r.canvasSettingsRepo.GetMultiple(ctx, keys)
}

func TestCanvasSettingsFailureDoesNotRefresh(t *testing.T) {
	repo := &failingCanvasSettingsRepo{err: errors.New("settings unavailable")}
	svc := NewSettingService(repo, &config.Config{})
	svc.SetOnUpdateCallback(func() { t.Fatal("failed save must not refresh settings") })
	err := svc.UpdateCanvasSettings(context.Background(), CanvasSettings{Enabled: true, URL: "https://canvas.example"})
	require.ErrorIs(t, err, repo.err)
	origins, err := svc.GetFrameSrcOrigins(context.Background())
	require.ErrorIs(t, err, repo.err)
	require.Nil(t, origins)
}

func TestCanvasFrameSrcOriginsInvalidSettings(t *testing.T) {
	repo := &canvasSettingsRepo{authSourceDefaultsRepoStub{values: map[string]string{SettingKeyCanvas: "invalid json"}}}
	svc := NewSettingService(repo, &config.Config{})
	origins, err := svc.GetFrameSrcOrigins(context.Background())
	require.Error(t, err)
	require.Nil(t, origins)
}
