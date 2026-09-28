package basispoints

import (
	"io"
	"strings"
	"testing"
)

// Reproduce the client-visible failure when an upstream run_officejs call
// applies the FUNCTION envelope to a tool declared as custom in the catalog.
func TestCustomArgumentsEnvelopeStreamingFailure(t *testing.T) {
	const input = "text(\"diagnostic-only\");\n"
	const errorMessage = "basispoints custom tools require input text, not arguments"
	for _, tc := range []struct {
		name     string
		kind     string
		envelope object
		raw      bool
		fails    bool
	}{
		{name: "custom with object arguments", kind: "custom", envelope: object{"name": "functions.exec", "arguments": object{"code": input}}, fails: true},
		{name: "custom with string arguments", kind: "custom", envelope: object{"name": "functions.exec", "arguments": input}, fails: true},
		{name: "custom with null arguments", kind: "custom", envelope: object{"name": "functions.exec", "arguments": nil}, fails: true},
		{name: "custom with input and arguments", kind: "custom", envelope: object{"name": "functions.exec", "input": input, "arguments": object{}}, fails: true},
		{name: "custom with input", kind: "custom", envelope: object{"name": "functions.exec", "input": input}},
		{name: "custom with args alias", kind: "custom", envelope: object{"name": "functions.exec", "args": input}},
		{name: "custom with marked raw transport", kind: "custom", raw: true},
		{name: "function with object arguments", kind: "function", envelope: object{"name": "functions.exec", "arguments": object{"code": input}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := testSource()
			source["tools"] = []any{object{"type": "namespace", "name": "functions", "tools": []any{object{"type": tc.kind, "name": "exec"}}}}
			cache := new(ReplayCache)
			_, bridge := mustPrepare(t, source, "diagnostic", cache)
			native := nativeCall(tc.envelope)
			if tc.raw {
				native["arguments"] = object{"summary": customTransportPrefix + "functions.exec", "code": input}
			}
			wire := sse(object{"type": "response.output_text.delta", "delta": "Checking."}) +
				sse(object{"type": "response.output_item.added", "output_index": 0, "item": native}) +
				sse(object{"type": "response.function_call_arguments.delta", "output_index": 0, "delta": "PRIVATE_TRANSPORT"}) +
				sse(object{"type": "response.output_item.done", "output_index": 0, "item": native}) +
				sse(object{"type": "response.completed", "response": object{"status": "completed", "output": []any{native}}})
			body := bridge.Stream(io.NopCloser(strings.NewReader(wire)))
			defer func() { _ = body.Close() }()
			out, err := io.ReadAll(body)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(out), "PRIVATE_TRANSPORT") || strings.Contains(string(out), "run_officejs") {
				t.Fatal("native transport leaked to the client")
			}
			var events []object
			if err := readEvents(strings.NewReader(string(out)), func(_ string, data []byte) error {
				var event object
				if err := decode(data, &event); err != nil {
					return err
				}
				events = append(events, event)
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if len(events) < 2 || events[0]["type"] != "response.output_text.delta" || events[0]["delta"] != "Checking." {
				t.Fatalf("text before the terminal event was lost: %s", out)
			}
			terminal := events[len(events)-1]
			response := mustTestValue[object](t, terminal["response"])
			output := mustTestValue[[]any](t, response["output"])
			if tc.fails {
				failure := mustTestValue[object](t, response["error"])
				if len(events) != 2 || terminal["type"] != "response.failed" || response["status"] != "failed" || len(output) != 0 ||
					failure["code"] != "basispoints_protocol_error" || failure["message"] != errorMessage {
					t.Fatalf("expected the reported failure without tool dispatch: %s", out)
				}
				if cache.get("diagnostic", "call_native") != nil || strings.Contains(string(out), "diagnostic-only") {
					t.Fatal("rejected input was cached or disclosed in the error")
				}
				return
			}
			if terminal["type"] != "response.completed" || len(output) != 1 {
				t.Fatalf("valid transport did not complete: %s", out)
			}
			call := mustTestValue[object](t, output[0])
			if call["namespace"] != "functions" || call["name"] != "exec" || call["call_id"] != "call_native" {
				t.Fatalf("client tool identity changed: %+v", call)
			}
			if tc.kind == "custom" && (call["type"] != "custom_tool_call" || call["input"] != input) {
				t.Fatal("custom transport must preserve the exact raw input")
			}
			if tc.kind == "function" && call["type"] != "function_call" {
				t.Fatal("function envelope must remain a function call")
			}
			if cache.get("diagnostic", "call_native") == nil {
				t.Fatal("valid tool call was not retained for replay")
			}
		})
	}
}
