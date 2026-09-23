package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// ValidateIntelligentReasoningEffort accepts wire values without silently
// coercing a user's selection to a different model's reasoning scale.
func ValidateIntelligentReasoningEffort(effort string) error {
	switch effort {
	case "", "none", "minimal", "low", "medium", "high", "xhigh":
		return nil
	default:
		return infraerrors.BadRequest("INTELLIGENT_TEST_INVALID_REASONING_EFFORT", "思考强度必须为默认、none、minimal、low、medium、high 或 xhigh")
	}
}

// Capability tests use the existing protocol adapters directly. An explicit
// effort must not disappear on an adapter with no equivalent native field.
func validateIntelligentAccountReasoning(account *Account, model, effort string) error {
	if err := ValidateIntelligentReasoningEffort(effort); err != nil {
		return err
	}
	if effort == "" {
		return nil
	}
	if account != nil {
		if account.IsOpenAI() {
			return nil
		}
		if account.Platform == PlatformGrok {
			if model == "" {
				model = grokDefaultResponsesModel
			}
			model = account.GetMappedModel(model)
			normalized, supported := normalizeGrokReasoningEffortValue(effort, model)
			if !grokSupportsReasoningEffort(model) || !supported || normalized != effort {
				return infraerrors.BadRequest("INTELLIGENT_TEST_REASONING_UNSUPPORTED", "当前 Grok 模型不支持所选思考强度；请选择默认或该模型原生支持的强度")
			}
			return nil
		}
		if account.IsCNProvider() {
			switch account.GetAPIProtocol() {
			case APIProtocolResponses, APIProtocolChatCompletions, APIProtocolAdaptive:
				return nil
			}
		}
	}
	return infraerrors.BadRequest("INTELLIGENT_TEST_REASONING_UNSUPPORTED", "当前账号测试协议不支持指定思考强度；请选择默认，或使用 Responses / Chat Completions 协议")
}

// Apply before staging the protected request, so account protection validates
// the explicit selection too. Manual tests use their separate options context;
// an omitted effort retains the adapter's existing request body.
func applyIntelligentPayloadReasoning(ctx context.Context, payload map[string]any, protocol string) error {
	run := intelligentContext(ctx)
	effort := ""
	if run != nil {
		effort = run.reasoningEffort
	} else if manual := manualAccountTest(ctx); manual != nil {
		effort = manual.reasoningEffort
	}
	if effort == "" {
		return nil
	}
	if err := ValidateIntelligentReasoningEffort(effort); err != nil {
		return err
	}
	switch protocol {
	case APIProtocolResponses:
		reasoning, _ := payload["reasoning"].(map[string]any)
		if reasoning == nil {
			reasoning = map[string]any{}
		}
		reasoning["effort"] = effort
		payload["reasoning"] = reasoning
	case APIProtocolChatCompletions:
		payload["reasoning_effort"] = effort
	default:
		return fmt.Errorf("当前测试协议 %s 不支持指定思考强度", protocol)
	}
	return nil
}

// Record the request handed to the transport only after a response is observed.
// This is evidence of the outgoing value, not a claim about hidden model work.
// Read GetBody's independent copy so streaming or retries keep their own body.
func observeIntelligentRequestReasoning(req *http.Request) {
	run := intelligentContext(req.Context())
	if run == nil || run.execution == nil || req.GetBody == nil {
		return
	}
	body, err := req.GetBody()
	if err != nil {
		return
	}
	defer body.Close()
	var payload map[string]any
	if json.NewDecoder(io.LimitReader(body, 1<<20)).Decode(&payload) != nil {
		return
	}
	if reasoning, ok := payload["reasoning"].(map[string]any); ok {
		if effort, ok := reasoning["effort"].(string); ok {
			run.execution.SentReasoningEffort = effort
			run.execution.ReasoningProtocol = APIProtocolResponses
			return
		}
	}
	if effort, ok := payload["reasoning_effort"].(string); ok {
		run.execution.SentReasoningEffort = effort
		run.execution.ReasoningProtocol = APIProtocolChatCompletions
	}
}
