package service

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const SettingKeyCanvas = "canvas_integration"

type CanvasSettings struct {
	Enabled            bool   `json:"enabled"`
	URL                string `json:"url"`
	UpstreamRetryCount int    `json:"upstream_retry_count"`
}

func (s *SettingService) GetCanvasSettings(ctx context.Context) (CanvasSettings, error) {
	values, err := s.settingRepo.GetMultiple(ctx, []string{SettingKeyCanvas})
	if err != nil {
		return CanvasSettings{}, err
	}
	settings := CanvasSettings{UpstreamRetryCount: 5}
	if raw := values[SettingKeyCanvas]; raw != "" {
		if err := json.Unmarshal([]byte(raw), &settings); err != nil {
			return settings, err
		}
	}
	return settings, nil
}
func (s *SettingService) UpdateCanvasSettings(ctx context.Context, settings CanvasSettings) error {
	if settings.UpstreamRetryCount < 0 || settings.UpstreamRetryCount > 5 {
		return infraerrors.BadRequest("INVALID_CANVAS_RETRY_COUNT", "画布上游额外重试次数必须为 0–5 的整数")
	}
	settings.URL = strings.TrimSpace(settings.URL)
	if settings.Enabled || settings.URL != "" {
		parsed, err := url.Parse(settings.URL)
		if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil || parsed.Fragment != "" {
			return infraerrors.BadRequest("INVALID_CANVAS_URL", "画布地址必须是无账号密码和片段的 HTTP/HTTPS 地址")
		}
	}
	data, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	if err := s.settingRepo.Set(ctx, SettingKeyCanvas, string(data)); err != nil {
		return err
	}
	if s.onUpdate != nil {
		s.onUpdate()
	}
	return nil
}
