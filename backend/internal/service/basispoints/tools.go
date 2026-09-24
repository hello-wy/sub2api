package basispoints

import (
	"container/list"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

type tool struct {
	Name      string
	Namespace string
	Kind      string
}

type replayEntry struct {
	key string
	raw []byte
}

// ReplayCache retains native tool identities without mixing accounts or sessions.
// Both entry count and bytes are bounded because tool arguments can be large.
type ReplayCache struct {
	mu      sync.Mutex
	entries map[string]*list.Element
	order   list.List
	bytes   int
}

func (c *ReplayCache) put(scope, id string, item object) {
	if c == nil || id == "" {
		return
	}
	raw, err := json.Marshal(item)
	if err != nil || len(raw) > 1<<20 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil {
		c.entries = make(map[string]*list.Element)
	}
	key := scope + "\x00" + id
	if old := c.entries[key]; old != nil {
		c.bytes -= len(old.Value.(replayEntry).raw)
		c.order.Remove(old)
	}
	c.entries[key] = c.order.PushBack(replayEntry{key: key, raw: raw})
	c.bytes += len(raw)
	for len(c.entries) > 1024 || c.bytes > 16<<20 {
		old := c.order.Front()
		entry := old.Value.(replayEntry)
		delete(c.entries, entry.key)
		c.bytes -= len(entry.raw)
		c.order.Remove(old)
	}
}

func (c *ReplayCache) get(scope, id string) object {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	entry := c.entries[scope+"\x00"+id]
	if entry == nil {
		return nil
	}
	c.order.MoveToBack(entry)
	var item object
	if decode(entry.Value.(replayEntry).raw, &item) != nil {
		return nil
	}
	return item
}

func (b *Bridge) collectTools(value any, namespace string) ([]any, error) {
	var catalog []any
	items, _ := value.([]any)
	for _, raw := range items {
		item, ok := raw.(object)
		if !ok {
			return nil, fmt.Errorf("invalid Basispoints client tool")
		}
		kind, name := text(item["type"]), text(item["name"])
		if kind == "namespace" {
			nested, err := b.collectTools(item["tools"], name)
			if err != nil {
				return nil, err
			}
			catalog = append(catalog, nested...)
			continue
		}
		if kind != "function" && kind != "custom" {
			return nil, fmt.Errorf("Basispoints does not support hosted tool %q; use client function or custom tools", kind)
		}
		if name == "" {
			return nil, fmt.Errorf("Basispoints client tools require a name")
		}
		key := name
		if namespace != "" {
			key = namespace + "." + name
		}
		if _, exists := b.tools[key]; exists {
			return nil, fmt.Errorf("duplicate Basispoints client tool %q", key)
		}
		b.tools[key] = tool{Name: name, Namespace: namespace, Kind: kind}
		entry := object{"type": kind, "name": key}
		for _, field := range []string{"description", "format", "parameters"} {
			if v, exists := item[field]; exists {
				entry[field] = v
			}
		}
		if kind == "function" && entry["parameters"] == nil {
			entry["parameters"] = item["inputSchema"]
			if entry["parameters"] == nil {
				entry["parameters"] = item["input_schema"]
			}
		}
		catalog = append(catalog, entry)
	}
	return catalog, nil
}

func (b *Bridge) translateHistory(input []any) ([]any, error) {
	result := make([]any, 0, len(input))
	seenCalls := make(map[string]bool)
	var trigger any
	for _, raw := range input {
		item, ok := raw.(object)
		if !ok {
			return nil, fmt.Errorf("invalid Basispoints input item")
		}
		delete(item, "internal_chat_message_metadata_passthrough")
		switch text(item["type"]) {
		case "additional_tools":
			continue
		case "item_reference":
			return nil, fmt.Errorf("Basispoints requires full history; item_reference is unsupported")
		case "compaction_trigger":
			trigger = item
			continue
		case "reasoning":
			if encrypted := text(item["encrypted_content"]); encrypted != "" {
				result = append(result, object{"type": "reasoning", "summary": []any{}, "encrypted_content": encrypted})
			}
			continue
		case "function_call", "custom_tool_call":
			id := text(item["call_id"])
			if native := b.replay.get(b.scope, id); native != nil {
				item = native
			} else {
				return nil, fmt.Errorf("Basispoints original tool item is unavailable after a restart, account change or cache eviction; start a new conversation")
			}
			seenCalls[id] = true
		case "function_call_output", "custom_tool_call_output":
			id := text(item["call_id"])
			if !seenCalls[id] {
				native := b.replay.get(b.scope, id)
				if native == nil {
					return nil, fmt.Errorf("Basispoints original tool item is unavailable for this tool result; start a new conversation")
				}
				result = append(result, native)
				seenCalls[id] = true
			}
			item["type"] = "function_call_output"
			if text(item["id"]) == "" {
				itemID := "fc_" + id
				if len(itemID) > 64 {
					itemID = "fc_" + fingerprint(id)
				}
				item["id"] = itemID
			}
		case "configuration_update":
			return nil, fmt.Errorf("Basispoints does not support configuration_update; start a new request with the desired effort")
		}
		if content, ok := item["content"].([]any); ok {
			for _, rawPart := range content {
				part, _ := rawPart.(object)
				switch text(part["type"]) {
				case "input_text", "output_text", "text", "refusal":
				case "input_image":
					if err := validateImage(part); err != nil {
						return nil, err
					}
				default:
					return nil, fmt.Errorf("Basispoints supports text and HTTPS input_image content only")
				}
			}
		}
		result = append(result, item)
	}
	if trigger != nil {
		result = append(result, trigger)
	}
	return result, nil
}

func isTool(item object) bool {
	return text(item["type"]) == "function_call" || text(item["type"]) == "custom_tool_call"
}

// translateCall accepts only the declared relay transport and a caller-declared tool.
// It does not evaluate code or dispatch any Excel operation.
func (b *Bridge) translateCall(native object) (object, error) {
	name := text(native["name"])
	if name != "run_officejs" && name != "functions.run_officejs" {
		return nil, fmt.Errorf("Basispoints returned an unsupported native tool; no tool was executed")
	}
	var arguments object
	if value, ok := native["arguments"].(object); ok {
		arguments = value
	} else if err := decode([]byte(text(native["arguments"])), &arguments); err != nil {
		return nil, fmt.Errorf("Basispoints returned invalid tool transport arguments")
	}
	if arguments == nil {
		return nil, fmt.Errorf("Basispoints returned empty tool transport arguments")
	}
	envelope, err := decodeTransportEnvelope(arguments["code"])
	if err != nil {
		return nil, err
	}
	toolName, err := envelopeName(envelope)
	if err != nil {
		return nil, err
	}
	info, allowed := b.tools[toolName]
	if !allowed {
		return nil, fmt.Errorf("Basispoints returned a tool outside the client's catalog")
	}
	id := text(native["call_id"])
	if id == "" {
		return nil, fmt.Errorf("Basispoints tool call is missing call_id")
	}
	itemID := text(native["id"])
	if itemID == "" {
		itemID = "fc_" + fingerprint(id)
	}
	result := object{"type": info.Kind + "_call", "id": itemID, "call_id": id, "name": info.Name, "status": "completed"}
	if info.Namespace != "" {
		result["namespace"] = info.Namespace
	}
	if info.Kind == "custom" {
		value, hasInput := envelope["input"]
		if alias, hasAlias := envelope["args"]; hasAlias {
			if hasInput {
				return nil, fmt.Errorf("Basispoints custom tool envelope contains conflicting input fields")
			}
			value = alias
		}
		if _, exists := envelope["arguments"]; exists {
			return nil, fmt.Errorf("Basispoints custom tools require input text, not arguments")
		}
		input, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("Basispoints custom tool input must be a string")
		}
		result["type"] = "custom_tool_call"
		result["id"] = "ctc_" + fingerprint(id)
		result["input"] = input
	} else {
		args, err := envelopeArguments(envelope)
		if err != nil {
			return nil, err
		}
		if raw, ok := args.(string); ok {
			if decode([]byte(raw), &args) != nil {
				return nil, fmt.Errorf("Basispoints function arguments are invalid JSON")
			}
		}
		if _, ok := args.(object); !ok {
			return nil, fmt.Errorf("Basispoints function arguments must be an object")
		}
		encoded, _ := json.Marshal(args)
		result["arguments"] = string(encoded)
	}
	b.replay.put(b.scope, id, native)
	return result, nil
}

func (b *Bridge) translateResponse(response object) error {
	if response == nil {
		return nil
	}
	output, _ := response["output"].([]any)
	for i, raw := range output {
		item, _ := raw.(object)
		if isTool(item) {
			translated, err := b.translateCall(item)
			if err != nil {
				return err
			}
			output[i] = translated
		}
	}
	response["reasoning"] = object{"effort": b.Effort}
	response["parallel_tool_calls"] = false
	return nil
}

func isToolEvent(kind string) bool {
	return strings.HasPrefix(kind, "response.function_call_arguments.") || strings.HasPrefix(kind, "response.custom_tool_call_input.")
}
