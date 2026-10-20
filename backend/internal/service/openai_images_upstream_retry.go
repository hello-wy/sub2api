package service

import (
	"context"
	"errors"
	"net/http"
)

var ErrOpenAIImagesUpstreamRequestFailed = errors.New("upstream request failed")

func isOpenAIImagesTransientStatus(status int) bool {
	switch status {
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout, 520, 521, 522, 523, 524:
		return true
	}
	return false
}

func IsOpenAIImagesTransientUpstreamFailure(err error) bool {
	if errors.Is(err, ErrOpenAIImagesUpstreamRequestFailed) {
		return true
	}
	var failoverErr *UpstreamFailoverError
	if errors.As(err, &failoverErr) {
		return !failoverErr.RetryableOnSameAccount && isOpenAIImagesTransientStatus(failoverErr.StatusCode)
	}
	var imageErr *OpenAIImagesUpstreamError
	if errors.As(err, &imageErr) {
		return !imageErr.SynthesizedFromModelText && isOpenAIImagesTransientStatus(imageErr.StatusCode)
	}
	return false
}

func (s *OpenAIGatewayService) OpenAIImagesUpstreamRetryCount(ctx context.Context) int {
	if s == nil || s.settingService == nil {
		return 5
	}
	settings, err := s.settingService.GetCanvasSettings(ctx)
	if err != nil {
		return 5
	}
	return settings.UpstreamRetryCount
}
