package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"syscall"
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
		{name: "upstream timeout", err: &OpenAIImagesUpstreamError{StatusCode: http.StatusGatewayTimeout}, want: false},
		{name: "cloudflare timeout", err: &UpstreamFailoverError{StatusCode: 524}, want: false},
		{name: "balance error under 503", err: &OpenAIImagesUpstreamError{StatusCode: 503, Code: "insufficient_balance"}, want: false},
		{name: "timeout under 502", err: &UpstreamFailoverError{StatusCode: 502, ResponseBody: []byte(`{"error":{"message":"request timed out"}}`)}, want: false},
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

func TestOpenAIImagesUpstreamRequestError(t *testing.T) {
	for _, tt := range []struct {
		name string
		err  error
		want bool
	}{
		{name: "failed dial", err: &net.OpError{Op: "dial", Err: syscall.ECONNREFUSED}, want: true},
		{name: "read reset has unknown outcome", err: &net.OpError{Op: "read", Err: syscall.ECONNRESET}},
		{name: "timeout", err: context.DeadlineExceeded},
		{name: "canceled", err: context.Canceled},
		{name: "dial timeout", err: &net.OpError{Op: "dial", Err: context.DeadlineExceeded}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsOpenAIImagesTransientUpstreamFailure(openAIImagesUpstreamRequestError(tt.err)); got != tt.want {
				t.Fatalf("retryable = %v, want %v", got, tt.want)
			}
		})
	}
}
