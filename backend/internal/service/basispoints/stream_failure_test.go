package basispoints

import (
	"io"
	"reflect"
	"strings"
	"testing"
)

func TestStreamFailurePreservesResponseMetadata(t *testing.T) {
	for _, protocolFailure := range []bool{false, true} {
		name := "upstream_error"
		if protocolFailure {
			name = "invalid_custom_transport"
		}
		t.Run(name, func(t *testing.T) {
			source := testSource()
			source["tools"] = []any{object{"type": "custom", "name": "exec"}}
			_, bridge := mustPrepare(t, source, "", nil)
			usage := object{"input_tokens": 100, "output_tokens": 8, "input_tokens_details": object{"cached_tokens": 90}}
			response := object{"id": "resp_failure", "model": "gpt-6-sol", "object": "response", "status": "in_progress", "output": []any{}}
			wire := sse(object{"type": "response.created", "response": response})
			if protocolFailure {
				response["status"] = "completed"
				response["usage"] = usage
				response["output"] = []any{nativeCall(object{"name": "exec", "arguments": "PRIVATE_INVALID_CODE"})}
				wire += sse(object{"type": "response.completed", "response": response})
			} else {
				wire += sse(object{"type": "error", "error": object{"code": "rate_limit_exceeded", "message": "TPM exhausted", "headers": object{"retry-after": "1"}}})
			}
			body := bridge.Stream(io.NopCloser(strings.NewReader(wire)))
			defer func() { _ = body.Close() }()
			out, err := io.ReadAll(body)
			if err != nil {
				t.Fatal(err)
			}
			var last object
			if err := readEvents(strings.NewReader(string(out)), func(_ string, data []byte) error { return decode(data, &last) }); err != nil {
				t.Fatal(err)
			}
			if last["type"] != "response.failed" {
				t.Fatalf("Responses clients need a failure terminal, got %s", out)
			}
			failed := mustTestValue[object](t, last["response"])
			if failed["id"] != "resp_failure" || failed["model"] != "gpt-6-sol" || failed["status"] != "failed" {
				t.Fatalf("response identity was lost: %s", out)
			}
			if protocolFailure {
				var want object
				_ = decode([]byte("{\"input_tokens\":100,\"output_tokens\":8,\"input_tokens_details\":{\"cached_tokens\":90}}"), &want)
				if !reflect.DeepEqual(failed["usage"], want) {
					t.Fatalf("measured upstream usage was discarded: %s", out)
				}
				if strings.Contains(string(out), "PRIVATE_INVALID_CODE") || strings.Contains(string(out), "run_officejs") {
					t.Fatal("invalid tool content leaked")
				}
			} else if failed["usage"] != nil {
				t.Fatal("a rejected request must not invent usage")
			}
		})
	}
}
