package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"
)

type groupPelicanContextKey struct{}

// ScheduledPelicanGroupID is an internal context marker, never a client header.
// Gateway authentication checks this before composite routing can change the group.
func ScheduledPelicanGroupID(ctx context.Context) int64 {
	id, _ := ctx.Value(groupPelicanContextKey{}).(int64)
	return id
}

func (s *ScheduledTestService) SetGroupGateway(gateway http.Handler) {
	s.gatewayMu.Lock()
	defer s.gatewayMu.Unlock()
	s.groupGateway = gateway
}

func (s *ScheduledTestService) ListPlansByGroup(ctx context.Context, groupID int64) ([]*ScheduledTestPlan, error) {
	plans, err := s.planRepo.ListByGroupID(ctx, groupID)
	for _, plan := range plans {
		if state := plan.Execution; state != nil && (state.Status == "running" || state.Status == "retrying" || state.Status == "cancelling") && (plan.RunningUntil == nil || !plan.RunningUntil.After(time.Now())) {
			copy := *state
			copy.Status, copy.RetryAt = "interrupted", nil
			plan.Execution = &copy
		}
	}
	return plans, err
}

func (s *ScheduledTestService) ListGroupTestKeys(ctx context.Context, groupID int64) ([]*GroupTestKey, error) {
	return s.planRepo.ListGroupTestKeys(ctx, groupID)
}

func (s *ScheduledTestService) TriggerGroupPlan(ctx context.Context, id int64) error {
	plan, err := s.planRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if plan.GroupID <= 0 || !plan.Enabled {
		return fmt.Errorf("only enabled group plans can be triggered")
	}
	if err := s.validatePlanTarget(ctx, plan); err != nil {
		return err
	}
	if s.startGroupPlan == nil {
		return fmt.Errorf("group test runner is not ready")
	}
	return s.startGroupPlan(ctx, plan)
}

func (s *ScheduledTestService) validatePlanTarget(ctx context.Context, plan *ScheduledTestPlan) error {
	if plan == nil || plan.AccountID < 0 || plan.GroupID < 0 || plan.APIKeyID < 0 {
		return fmt.Errorf("invalid test target")
	}
	if plan.GroupID == 0 {
		if plan.AccountID <= 0 || plan.APIKeyID != 0 {
			return fmt.Errorf("select exactly one account or group")
		}
		return nil
	}
	if plan.AccountID != 0 || plan.PelicanConfig == nil || plan.AutoRecover {
		return fmt.Errorf("group plans require a Pelican configuration and cannot recover individual accounts")
	}
	if plan.PelicanConfig.QuestionKind == "candy" || isBuiltinCandyPlan(plan.PelicanConfig) {
		return fmt.Errorf("group plans require a Pelican drawing question")
	}
	// A broken credential must not prevent an administrator from pausing a plan.
	if !plan.Enabled {
		return nil
	}
	_, err := s.groupTestKey(ctx, plan)
	return err
}

func (s *ScheduledTestService) groupTestKey(ctx context.Context, plan *ScheduledTestPlan) (*APIKey, error) {
	if s.groupRepo == nil || s.keyRepo == nil {
		return nil, fmt.Errorf("group test service is unavailable")
	}
	group, err := s.groupRepo.GetByID(ctx, plan.GroupID)
	if err != nil || group == nil || !group.IsActive() {
		return nil, fmt.Errorf("test group is deleted or inactive")
	}
	if plan.APIKeyID <= 0 {
		return nil, fmt.Errorf("select an active API Key belonging to this group")
	}
	key, err := s.keyRepo.GetByID(ctx, plan.APIKeyID)
	if err != nil || key == nil || key.GroupID == nil || *key.GroupID != plan.GroupID || !key.IsActive() || key.IsExpired() || key.Key == "" {
		return nil, fmt.Errorf("test API Key is unavailable, expired or no longer belongs to this group")
	}
	return key, nil
}

// RunGroupPelican invokes the real local HTTP router: authentication, model access,
// composite routing, scheduling, usage recording and billing retain normal behavior.
// No external base URL or credential-bearing loopback network call is needed.
func (s *ScheduledTestService) RunGroupPelican(ctx context.Context, plan *ScheduledTestPlan) (*ScheduledTestResult, error) {
	return s.runGroupPelican(ctx, plan, nil)
}

func (s *ScheduledTestService) runGroupPelican(ctx context.Context, plan *ScheduledTestPlan, onProgress func(string)) (*ScheduledTestResult, error) {
	if plan == nil || plan.PelicanConfig == nil {
		return nil, fmt.Errorf("missing Pelican configuration")
	}
	key, err := s.groupTestKey(ctx, plan)
	if err != nil {
		return nil, err
	}
	s.gatewayMu.RLock()
	gateway := s.groupGateway
	s.gatewayMu.RUnlock()
	if gateway == nil {
		return nil, fmt.Errorf("group test gateway is not ready")
	}
	body, err := json.Marshal(map[string]any{
		"model": plan.ModelID, "stream": true, "max_tokens": 16384,
		"reasoning_effort": plan.PelicanConfig.ReasoningEffort,
		"messages":         []map[string]string{{"role": "user", "content": intelligenceTestPrompt(plan.PelicanConfig)}},
	})
	if err != nil {
		return nil, err
	}
	started := time.Now()
	ctx, cancel := context.WithCancel(context.WithValue(ctx, groupPelicanContextKey{}, plan.GroupID))
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://localhost/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.RemoteAddr = "127.0.0.1:0"
	request.Header.Set("Authorization", "Bearer "+key.Key)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "text/event-stream")
	request.Header.Set("User-Agent", "Sub2API-Pelican-ScheduledTest/1.0")
	recorder := &pelicanRecorder{ResponseRecorder: httptest.NewRecorder(), cancel: cancel}
	if onProgress != nil {
		progress := &groupPelicanStreamProgress{onProgress: onProgress}
		recorder.onWrite = progress.write
	}
	gateway.ServeHTTP(recorder, request)
	output, message := parseGroupPelicanOutput(recorder.Body.Bytes())
	if recorder.Code < 200 || recorder.Code >= 300 {
		message = fmt.Sprintf("Gateway HTTP %d: %s", recorder.Code, groupPelicanHTTPError(recorder.Body.Bytes()))
	}
	if recorder.overflow || len(output) > 2<<20 {
		output, message = "", "Output exceeds the test capture limit"
	} else if message == "" {
		if ctx.Err() != nil {
			message = ctx.Err().Error()
		} else {
			message = intelligenceTestOutputError(plan.PelicanConfig, output)
		}
	}
	message = strings.ReplaceAll(message, key.Key, "[redacted]")
	snapshot := *plan.PelicanConfig
	snapshot.ModelID = plan.ModelID
	result := &ScheduledTestResult{Status: "success", ResponseText: output, ErrorMessage: message,
		StartedAt: started, FinishedAt: time.Now(), LatencyMs: time.Since(started).Milliseconds(), PelicanConfig: &snapshot}
	if message != "" {
		result.Status = "failed"
	}
	return result, nil
}

func groupPelicanHTTPError(body []byte) string {
	var envelope struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &envelope) == nil && envelope.Error.Message != "" {
		message := envelope.Error.Message
		if len(message) > 1024 {
			message = message[:1024]
		}
		return message
	}
	return "request rejected; check the API Key, group restrictions and gateway logs"
}

func parseGroupPelicanOutput(body []byte) (string, string) {
	scanner := bufio.NewScanner(bytes.NewReader(body))
	scanner.Buffer(make([]byte, 4096), 4<<20)
	var output strings.Builder
	complete := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			continue
		}
		var event struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
				FinishReason *string `json:"finish_reason"`
			} `json:"choices"`
			Error json.RawMessage `json:"error"`
		}
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			return output.String(), "Invalid gateway stream event"
		}
		if len(event.Error) > 0 && string(event.Error) != "null" {
			return output.String(), groupPelicanHTTPError([]byte(data))
		}
		for _, choice := range event.Choices {
			_, _ = output.WriteString(choice.Delta.Content)
			if choice.FinishReason != nil && *choice.FinishReason != "" {
				if *choice.FinishReason != "stop" {
					return output.String(), "Generation did not finish normally: " + *choice.FinishReason
				}
				complete = true
			}
		}
	}
	if scanner.Err() != nil {
		return output.String(), "Gateway stream exceeded the capture limit"
	}
	if !complete {
		return output.String(), "Generation stream ended before completion"
	}
	return output.String(), ""
}
