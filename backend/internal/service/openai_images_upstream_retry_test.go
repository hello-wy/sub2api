package service

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
)

func TestIsOpenAIImagesTransientUpstreamFailure(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "transport failure", err: fmt.Errorf("wrapped: %w", ErrOpenAIImagesUpstreamRequestFailed), want: true},
		{name: "gateway response", err: &OpenAIImagesUpstreamError{StatusCode: http.StatusBadGateway}, want: true},
		{name: "same-account retry owns failure", err: &UpstreamFailoverError{StatusCode: http.StatusServiceUnavailable, RetryableOnSameAccount: true}, want: false},
		{name: "client error", err: &OpenAIImagesUpstreamError{StatusCode: http.StatusBadRequest}, want: false},
		{name: "model text is not an upstream failure", err: &OpenAIImagesUpstreamError{StatusCode: http.StatusServiceUnavailable, SynthesizedFromModelText: true}, want: false},
		{name: "unrelated error", err: errors.New("invalid prompt"), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsOpenAIImagesTransientUpstreamFailure(tt.err); got != tt.want {
				t.Fatalf("IsOpenAIImagesTransientUpstreamFailure() = %v, want %v", got, tt.want)
			}
		})
	}
}
