package service

import (
	"bytes"
	"encoding/json"
)

// Observe complete SSE data lines while the gateway is still serving the request.
// Heartbeats, errors and incomplete chunks are not evidence of model output.
// Phases only advance within one output; reasoning after content cannot regress it.
type groupPelicanStreamProgress struct {
	pending    []byte
	phase      string
	onProgress func(string)
}

func groupPelicanPhaseRank(phase string) int {
	switch phase {
	case "receiving":
		return 1
	case "thinking":
		return 2
	case "generating":
		return 3
	default:
		return 0
	}
}

func (p *groupPelicanStreamProgress) write(data []byte) {
	p.pending = append(p.pending, data...)
	for {
		end := bytes.IndexByte(p.pending, '\n')
		if end < 0 {
			return
		}
		line := bytes.TrimSpace(p.pending[:end])
		p.pending = p.pending[end+1:]
		if !bytes.HasPrefix(line, []byte("data:")) {
			continue
		}
		var event struct {
			Choices []struct {
				Delta struct {
					Content          string `json:"content"`
					ReasoningContent string `json:"reasoning_content"`
					Reasoning        string `json:"reasoning"`
				} `json:"delta"`
			} `json:"choices"`
			Error json.RawMessage `json:"error"`
		}
		if json.Unmarshal(bytes.TrimSpace(line[len("data:"):]), &event) != nil || len(event.Choices) == 0 || (len(event.Error) > 0 && string(event.Error) != "null") {
			continue
		}
		phase := "receiving"
		for _, choice := range event.Choices {
			if choice.Delta.Content != "" {
				phase = "generating"
				break
			}
			if choice.Delta.ReasoningContent != "" || choice.Delta.Reasoning != "" {
				phase = "thinking"
			}
		}
		if groupPelicanPhaseRank(phase) > groupPelicanPhaseRank(p.phase) {
			p.phase = phase
			p.onProgress(phase)
		}
	}
}
