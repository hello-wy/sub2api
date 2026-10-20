package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
)

var ErrOpenAIImagesUpstreamRequestFailed = errors.New("upstream request failed")

func openAIImagesUpstreamRequestError(err error) error {
	message := sanitizeUpstreamErrorMessage(err.Error())
	var networkErr *net.OpError
	// A failed dial has not submitted a generation. Read/write failures and
	// timeouts have an unknown outcome and must not duplicate a charged task.
	if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) &&
		errors.As(err, &networkErr) && networkErr.Op == "dial" && !networkErr.Timeout() {
		return fmt.Errorf("%w: %s", ErrOpenAIImagesUpstreamRequestFailed, message)
	}
	return fmt.Errorf("upstream request failed: %s", message)
}

func isOpenAIImagesTransientStatus(status int) bool {
	switch status {
	case http.StatusBadGateway, http.StatusServiceUnavailable, 520, 521, 523:
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
		return !failoverErr.RetryableOnSameAccount && isOpenAIImagesTransientStatus(failoverErr.StatusCode) &&
			!openAIImagesPermanentFailure(string(failoverErr.ResponseBody))
	}
	var imageErr *OpenAIImagesUpstreamError
	if errors.As(err, &imageErr) {
		return !imageErr.SynthesizedFromModelText && isOpenAIImagesTransientStatus(imageErr.StatusCode) &&
			!openAIImagesPermanentFailure(imageErr.Code+" "+imageErr.ErrorType+" "+imageErr.Message)
	}
	return false
}

func openAIImagesPermanentFailure(message string) bool {
	message = strings.ToLower(message)
	for _, reason := range []string{"quota", "billing", "balance", "credit", "insufficient", "permission", "auth", "invalid", "moderation", "policy", "timeout", "timed out", "额度", "余额", "配额", "超时"} {
		if strings.Contains(message, reason) {
			return true
		}
	}
	return false
}

func (s *OpenAIGatewayService) OpenAIImagesUpstreamRetryCount(ctx context.Context) int {
	if s == nil || s.settingService == nil {
		return 0
	}
	settings, err := s.settingService.GetCanvasSettings(ctx)
	if err != nil {
		return 0
	}
	if settings.UpstreamRetryCount < 0 || settings.UpstreamRetryCount > 5 {
		return 0
	}
	return settings.UpstreamRetryCount
}
