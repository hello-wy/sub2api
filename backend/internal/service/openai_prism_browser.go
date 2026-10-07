package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const prismBrowserMaxResponseBytes = 2 << 20

type prismBrowserRequest struct {
	Account *Account
	Body    []byte
	Runtime PrismBrowserRuntime
	Started time.Time
}

func prismBrowserAdapterURL(baseURL string) (string, error) {
	parsed, err := url.Parse(prismBrowserResponsesURL(baseURL))
	if err != nil || parsed.Scheme != "http" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Opaque != "" {
		return "", errors.New("prism adapter must use a local HTTP endpoint")
	}
	if ip := net.ParseIP(parsed.Hostname()); ip == nil || (!ip.Equal(net.ParseIP("127.0.0.1")) && !ip.Equal(net.IPv6loopback)) {
		return "", errors.New("prism adapter must bind to a numeric loopback address")
	}
	port, err := strconv.Atoi(parsed.Port())
	if err != nil || port < 1 || port > 65535 || parsed.Path != "/v1/responses" {
		return "", errors.New("invalid Prism adapter endpoint")
	}
	return parsed.String(), nil
}

func (s *OpenAIGatewayService) forwardPrismBrowser(ctx context.Context, c *gin.Context, request prismBrowserRequest) (*OpenAIForwardResult, error) {
	if isOpenAIResponsesCompactPath(c) {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"type": "invalid_request_error", "message": "Prism adapter does not support responses/compact"}})
		return nil, errors.New("prism adapter does not support responses/compact")
	}
	model := strings.TrimSpace(gjson.GetBytes(request.Body, "model").String())
	stream := gjson.GetBytes(request.Body, "stream").Bool()
	if model == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"type": "invalid_request_error", "message": "model is required"}})
		return nil, errors.New("prism adapter model is required")
	}
	responseBody, upstreamHeaders, status, err := s.callPrismBrowser(ctx, request)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		c.Data(status, "application/json", responseBody)
		return nil, fmt.Errorf("prism adapter returned HTTP %d", status)
	}
	responseID := strings.TrimSpace(upstreamHeaders.Get("X-Request-Id"))
	contentType := "application/json"
	if stream {
		if !bytes.Contains(responseBody, []byte("event: response.completed")) {
			return nil, errors.New("prism adapter stream has no completed event")
		}
		contentType = "text/event-stream"
	} else {
		if !gjson.ValidBytes(responseBody) || gjson.GetBytes(responseBody, "status").String() != "completed" ||
			!gjson.GetBytes(responseBody, "output.0.content.0.text").Exists() {
			return nil, errors.New("prism adapter returned an invalid terminal response")
		}
		responseID = gjson.GetBytes(responseBody, "id").String()
	}
	SetActualOpenAIUpstreamEndpoint(c, "/v1/responses")
	c.Header("X-Prism-Usage", "unavailable")
	c.Data(http.StatusOK, contentType, responseBody)
	return &OpenAIForwardResult{
		RequestID:       upstreamHeaders.Get("X-Request-Id"),
		ResponseID:      responseID,
		UpstreamHeaders: upstreamHeaders,
		Model:           model,
		UpstreamModel:   model,
		Stream:          stream,
		Duration:        time.Since(request.Started),
	}, nil
}

func (s *OpenAIGatewayService) callPrismBrowser(ctx context.Context, request prismBrowserRequest) ([]byte, http.Header, int, error) {
	endpoint, err := prismBrowserAdapterURL(request.Runtime.BaseURL)
	if err != nil {
		return nil, nil, 0, err
	}
	key := request.Runtime.APIKey
	if key == "" {
		return nil, nil, 0, errors.New("prism adapter key is not configured")
	}
	token, _, err := s.GetAccessToken(ctx, request.Account)
	if err != nil {
		return nil, nil, 0, err
	}
	if token == "" || strings.ContainsAny(token, "\r\n") {
		return nil, nil, 0, errors.New("invalid Prism OAuth token")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(request.Body))
	if err != nil {
		return nil, nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("X-Prism-Account-ID", strconv.FormatInt(request.Account.ID, 10))
	req.Header.Set("X-Prism-OAuth-Token", token)
	// The token must never pass through an account proxy, environment proxy,
	// plugin transport, or an HTTP redirect.
	client := &http.Client{
		Timeout:       5 * time.Minute,
		Transport:     &http.Transport{Proxy: nil},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("prism adapter request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, prismBrowserMaxResponseBytes+1))
	if err != nil || len(responseBody) > prismBrowserMaxResponseBytes {
		return nil, nil, 0, errors.New("prism adapter response exceeded limit")
	}
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		return nil, nil, 0, errors.New("prism adapter redirected unexpectedly")
	}
	return responseBody, resp.Header, resp.StatusCode, nil
}
