package basispoints

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func TestMultipleTerminalToolsReplayWithoutLoss(t *testing.T) {
	cache := new(ReplayCache)
	source := testSource()
	source["tools"] = []any{object{"type": "function", "name": "lookup"}, object{"type": "custom", "name": "apply_patch"}}
	_, bridge := mustPrepare(t, source, "thread-A", cache)
	first := nativeCall(object{"tool": "lookup", "args": object{"key": "one"}})
	second := nativeCall(object{"name": "apply_patch", "input": "  *** Begin Patch\n*** End Patch\n"})
	first["id"], first["call_id"] = "fc_A", "call_A"
	second["id"], second["call_id"] = "fc_B", "call_B"
	wire := sse(object{"type": "response.completed", "response": object{"id": "resp_1", "output": []any{first, second}}})
	stream := bridge.Stream(io.NopCloser(strings.NewReader(wire)))
	defer func() { _ = stream.Close() }()
	output, err := io.ReadAll(stream)
	if err != nil {
		t.Fatal(err)
	}
	var calls []any
	err = readEvents(strings.NewReader(string(output)), func(_ string, data []byte) error {
		var e object
		if err := decode(data, &e); err != nil {
			return err
		}
		if text(e["type"]) == "response.output_item.done" {
			calls = append(calls, e["item"])
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 2 {
		t.Fatalf("lost tool: %s", output)
	}
	// Results can arrive in another order; native call IDs must remain paired.
	source["input"] = []any{object{"type": "custom_tool_call_output", "call_id": "call_B", "output": "patched"}, object{"type": "function_call_output", "call_id": "call_A", "output": "found"}}
	replay, _ := mustPrepare(t, source, "thread-A", cache)
	input, _ := replay["input"].([]any)
	if !reflect.DeepEqual(input[len(input)-4], second) || !reflect.DeepEqual(input[len(input)-2], first) {
		t.Fatal("native metadata or result pairing changed")
	}
}

func TestConcurrentThreadsWithIdenticalCallIDsStayIsolated(t *testing.T) {
	cache := new(ReplayCache)
	var wg sync.WaitGroup
	for i := 0; i < 24; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			scope := fmt.Sprint(i)
			native := nativeCall(object{"name": "lookup", "arguments": object{"thread": scope}})
			cache.put(scope, "same-id", native)
			for j := 0; j < 20; j++ {
				got := cache.get(scope, "same-id")
				a, _ := json.Marshal(got)
				b, _ := json.Marshal(native)
				if string(a) != string(b) {
					t.Errorf("thread %d replay collision", i)
					return
				}
			}
		}(i)
	}
	wg.Wait()
}
