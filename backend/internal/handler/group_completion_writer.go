package handler

import (
	"bytes"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// A tiny streaming observer tracks termination without retaining prompts or
// generated content. Fragmented terminal frames survive Write boundaries.
type groupCompletionWriter struct {
	gin.ResponseWriter
	pending                 []byte
	sse, complete, dropping bool
	largeTerminal           bool
	failed                  bool
}

const groupCompletionBufferLimit = 64 << 10

func (w *groupCompletionWriter) Write(p []byte) (int, error) {
	n, err := w.ResponseWriter.Write(p)
	w.observe(p[:n])
	return n, err
}
func (w *groupCompletionWriter) WriteString(s string) (int, error) { return w.Write([]byte(s)) }
func (w *groupCompletionWriter) observe(p []byte) {
	if !w.sse {
		w.sse = strings.Contains(w.Header().Get("Content-Type"), "text/event-stream")
	}
	if !w.sse {
		return
	}
	for len(p) > 0 {
		index := bytes.IndexByte(p, '\n')
		part := p
		if index >= 0 {
			part = p[:index]
		}
		if !w.dropping {
			remaining := groupCompletionBufferLimit - len(w.pending)
			if len(part) > remaining {
				// Retain only a bounded prefix, including when a huge line ends
				// in this Write or its type was split across earlier Writes.
				w.appendPrefix(part[:remaining])
				complete, failed := groupLargeEventPrefix(w.pending)
				w.largeTerminal = complete
				w.failed = w.failed || failed
				w.pending = nil
				w.dropping = true
			} else {
				w.appendPrefix(part)
			}
		}
		if index < 0 {
			return
		}
		if !w.dropping {
			w.line(w.pending)
		}
		if w.dropping && w.largeTerminal {
			w.complete = true
		}
		w.pending = w.pending[:0]
		w.dropping = false
		w.largeTerminal = false
		p = p[index+1:]
	}
}

func (w *groupCompletionWriter) appendPrefix(p []byte) {
	if len(p) == 0 {
		return
	}
	// Explicit growth caps retained capacity without allocating 64 KB for
	// every small event. Go's automatic growth could exceed the bound.
	if needed := len(w.pending) + len(p); needed > cap(w.pending) {
		capacity := max(1024, max(needed, cap(w.pending)*2))
		capacity = min(capacity, groupCompletionBufferLimit)
		buf := make([]byte, len(w.pending), capacity)
		copy(buf, w.pending)
		w.pending = buf
	}
	w.pending = append(w.pending, p...)
}

// Responses terminal payloads and upstream error details can exceed the cap.
// Read only top-level metadata from the bounded prefix so a following [DONE]
// cannot erase an error. Generated content containing those words does not
// qualify. Completed events still require their line boundary.
func groupLargeEventPrefix(prefix []byte) (complete, failed bool) {
	prefix = bytes.TrimSpace(prefix)
	if !bytes.HasPrefix(prefix, []byte("data:")) {
		return false, false
	}
	data := bytes.TrimSpace(prefix[5:])
	switch gjson.GetBytes(data, "type").String() {
	case "response.completed", "message_stop":
		complete = true
	case "error", "response.failed", "response.incomplete":
		failed = true
	}
	if err := gjson.GetBytes(data, "error"); err.Exists() && err.Type != gjson.Null {
		failed = true
	}
	// gjson cannot expose an unterminated string value. Detect that field's
	// opening quote without retaining or allocating the large error message.
	failed = failed || groupLargeStringErrorPrefix(data)
	return complete, failed
}

func groupLargeStringErrorPrefix(data []byte) bool {
	depth := 0
	for i := 0; i < len(data); i++ {
		switch data[i] {
		case '{', '[':
			depth++
		case '}', ']':
			depth--
		case '"':
			start := i
			for i++; i < len(data); i++ {
				if data[i] == '\\' {
					i++
				} else if data[i] == '"' {
					break
				}
			}
			if i >= len(data) {
				return false
			}
			if depth != 1 || !bytes.Equal(data[start:i+1], []byte(`"error"`)) {
				continue
			}
			rest := bytes.TrimSpace(data[i+1:])
			if len(rest) > 0 && rest[0] == ':' {
				rest = bytes.TrimSpace(rest[1:])
				return len(rest) > 0 && rest[0] == '"'
			}
		}
	}
	return false
}

func (w *groupCompletionWriter) finish() {
	if !w.dropping && len(w.pending) > 0 {
		w.line(w.pending)
	}
	w.pending = nil
}
func (w *groupCompletionWriter) line(line []byte) {
	line = bytes.TrimSpace(line)
	if !bytes.HasPrefix(line, []byte("data:")) {
		return
	}
	data := bytes.TrimSpace(line[5:])
	if bytes.Equal(data, []byte("[DONE]")) {
		w.complete = true
		return
	}
	if !bytes.Contains(data, []byte("finish_reason")) && !bytes.Contains(data, []byte("finishReason")) && !bytes.Contains(data, []byte("response.completed")) && !bytes.Contains(data, []byte("message_stop")) && !bytes.Contains(data, []byte("error")) && !bytes.Contains(data, []byte("response.failed")) && !bytes.Contains(data, []byte("response.incomplete")) {
		return
	}
	if !gjson.ValidBytes(data) {
		return
	}
	v := gjson.ParseBytes(data)
	if err := v.Get("error"); err.Exists() && err.Type != gjson.Null {
		w.failed = true
	}
	switch v.Get("type").String() {
	case "response.completed", "message_stop":
		w.complete = true
	case "error", "response.failed", "response.incomplete":
		w.failed = true
	}
	for _, choice := range v.Get("choices").Array() {
		f := choice.Get("finish_reason")
		if f.Exists() && f.Type != gjson.Null && f.String() != "" {
			w.complete = true
		}
	}
	for _, candidate := range v.Get("candidates").Array() {
		if candidate.Get("finishReason").String() != "" {
			w.complete = true
		}
	}
}
