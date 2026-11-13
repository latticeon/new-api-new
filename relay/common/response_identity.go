package common

import (
	"bytes"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// NormalizeResponseIdentity changes only protocol-owned identity fields in a
// serialized delivery copy. Callers must observe and account from the original.
func NormalizeResponseIdentity(body []byte, format types.RelayFormat, model string) []byte {
	if model == "" || !operation_setting.GetGeneralSetting().NormalizeResponseModelEnabled || !gjson.ValidBytes(body) {
		return body
	}
	root := gjson.ParseBytes(body)
	if format == types.RelayFormatGemini && root.IsArray() {
		root.ForEach(func(key, value gjson.Result) bool {
			for _, field := range []string{"modelVersion", "model_version"} {
				if value.Get(field).Type == gjson.String {
					if next, err := sjson.SetBytes(body, strconv.FormatInt(key.Int(), 10)+"."+field, model); err == nil {
						body = next
					}
				}
			}
			return true
		})
		return body
	}
	if !root.IsObject() || root.Get("object").String() == "list" {
		return body
	}
	var paths []string
	switch format {
	case types.RelayFormatGemini:
		paths = []string{"modelVersion", "model_version"}
	case types.RelayFormatClaude:
		paths = []string{"model"}
		if root.Get("type").String() == "message_start" {
			paths = append(paths, "message.model")
		}
	case types.RelayFormatOpenAIResponses, types.RelayFormatOpenAIResponsesCompaction:
		paths = []string{"model"}
		if strings.HasPrefix(root.Get("type").String(), "response.") {
			paths = append(paths, "response.model")
		}
	case types.RelayFormatOpenAIRealtime:
		switch root.Get("type").String() {
		case "session.created", "session.updated":
			paths = []string{"session.model"}
		case "response.created", "response.done":
			paths = []string{"response.model"}
		}
	case types.RelayFormatOpenAI, types.RelayFormatEmbedding, types.RelayFormatRerank, types.RelayFormatOpenAIImage, types.RelayFormatOpenAIAudio:
		paths = []string{"model"}
	}
	for _, path := range paths {
		if value := root.Get(path); value.Type == gjson.String && value.String() != model {
			if next, err := sjson.SetBytes(body, path, model); err == nil {
				body = next
			}
		}
	}
	return body
}

// ResponseIdentityBoundary is installed after report capture and before reply
// checks. It never changes the handler's DTOs, upstream bytes or usage facts.
// Finish must run after the final handler write (including reply-check output).
func ResponseIdentityBoundary(c *gin.Context, format types.RelayFormat, model string) func() {
	if model == "" || !operation_setting.GetGeneralSetting().NormalizeResponseModelEnabled || format == types.RelayFormatOpenAIRealtime {
		return func() {}
	}
	writer := &responseIdentityWriter{ResponseWriter: c.Writer, format: format, model: model}
	c.Writer = writer
	return func() {
		writer.finish()
		if c.Writer == writer {
			c.Writer = writer.ResponseWriter
		}
	}
}

type responseIdentityWriter struct {
	gin.ResponseWriter
	mu      sync.Mutex
	format  types.RelayFormat
	model   string
	buffer  []byte
	passing bool
	array   bool
	scan    int
	depth   int
	quoted  bool
	escaped bool
}

// Match the largest inline media response supported by the host. Malformed or
// oversized framing passes through rather than retaining an unbounded buffer.
const maxResponseIdentityBuffer = 64 << 20

func (w *responseIdentityWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w *responseIdentityWriter) WriteHeaderNow() {
	if strings.Contains(w.Header().Get("Content-Type"), "json") || strings.HasPrefix(w.Header().Get("Content-Type"), "text/event-stream") {
		w.Header().Del("Content-Length")
	}
	w.ResponseWriter.WriteHeaderNow()
}
func (w *responseIdentityWriter) WriteString(text string) (int, error) { return w.Write([]byte(text)) }
func (w *responseIdentityWriter) Written() bool {
	return len(w.buffer) > 0 || w.ResponseWriter.Written()
}
func (w *responseIdentityWriter) Size() int {
	if len(w.buffer) > 0 {
		return max(0, w.ResponseWriter.Size()) + len(w.buffer)
	}
	return w.ResponseWriter.Size()
}
func (w *responseIdentityWriter) Write(body []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	contentType := w.Header().Get("Content-Type")
	if w.passing || w.Status() >= 400 || w.Header().Get("Content-Encoding") != "" || (!strings.Contains(contentType, "json") && !strings.HasPrefix(contentType, "text/event-stream")) {
		return w.ResponseWriter.Write(body)
	}
	w.Header().Del("Content-Length")
	if len(w.buffer)+len(body) > maxResponseIdentityBuffer {
		w.passing = true
		if len(w.buffer) > 0 {
			if _, err := w.ResponseWriter.Write(w.buffer); err != nil {
				w.buffer = nil
				return 0, err
			}
			w.buffer = nil
		}
		return w.ResponseWriter.Write(body)
	}
	w.buffer = append(w.buffer, body...)
	if strings.HasPrefix(contentType, "text/event-stream") {
		for {
			end := bytes.Index(w.buffer, []byte("\n\n"))
			delimiter := 2
			if crlf := bytes.Index(w.buffer, []byte("\r\n\r\n")); crlf >= 0 && (end < 0 || crlf < end) {
				end, delimiter = crlf, 4
			}
			if end < 0 {
				break
			}
			event := w.buffer[:end+delimiter]
			if _, err := w.ResponseWriter.Write(w.normalizeEvent(event)); err != nil {
				return 0, err
			}
			w.buffer = w.buffer[end+delimiter:]
		}
	}
	if w.format == types.RelayFormatGemini && !strings.HasPrefix(contentType, "text/event-stream") {
		if err := w.drainGeminiArray(); err != nil {
			return 0, err
		}
	}
	if !strings.HasPrefix(contentType, "text/event-stream") && gjson.ValidBytes(w.buffer) {
		_, err := w.ResponseWriter.Write(NormalizeResponseIdentity(w.buffer, w.format, w.model))
		w.buffer = nil
		w.passing = true
		if err != nil {
			return 0, err
		}
	}
	return len(body), nil
}
func (w *responseIdentityWriter) normalizeEvent(event []byte) []byte {
	newline := "\n"
	if bytes.Contains(event, []byte("\r\n")) {
		newline = "\r\n"
	}
	lines := strings.Split(string(event), newline)
	var data []string
	first := -1
	for i, line := range lines {
		if value, ok := strings.CutPrefix(line, "data:"); ok {
			value, _ = strings.CutPrefix(value, " ")
			if first < 0 {
				first = i
			}
			data = append(data, value)
		}
	}
	payload := []byte(strings.Join(data, "\n"))
	normalized := NormalizeResponseIdentity(payload, w.format, w.model)
	if bytes.Equal(payload, normalized) {
		return event
	}
	var out []string
	for i, line := range lines {
		if i == first {
			for value := range strings.SplitSeq(string(normalized), "\n") {
				out = append(out, "data: "+value)
			}
		} else if !strings.HasPrefix(line, "data:") {
			out = append(out, line)
		}
	}
	return []byte(strings.Join(out, newline))
}
func (w *responseIdentityWriter) Flush() { _ = w.FlushError() }
func (w *responseIdentityWriter) FlushError() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	// Never release an incomplete protocol frame on a transport flush.
	// Gemini arrays drain complete objects in Write; SSE drains whole events.
	return http.NewResponseController(w.ResponseWriter).Flush()
}

// drainGeminiArray preserves JSON array framing while delivering complete
// GenerateContent objects without waiting for the array's closing bracket.
func (w *responseIdentityWriter) drainGeminiArray() error {
	if !w.array {
		trimmed := bytes.TrimLeft(w.buffer, " \t\r\n")
		if len(trimmed) == 0 || trimmed[0] != '[' {
			return nil
		}
		prefix := len(w.buffer) - len(trimmed) + 1
		if _, err := w.ResponseWriter.Write(w.buffer[:prefix]); err != nil {
			return err
		}
		w.buffer = w.buffer[prefix:]
		w.array = true
	}
	for w.scan < len(w.buffer) {
		ch := w.buffer[w.scan]
		w.scan++
		if w.quoted {
			if w.escaped {
				w.escaped = false
			} else if ch == '\\' {
				w.escaped = true
			} else if ch == '"' {
				w.quoted = false
			}
			continue
		}
		switch ch {
		case '"':
			w.quoted = true
		case '{', '[':
			w.depth++
		case '}', ']':
			if w.depth > 0 {
				w.depth--
			}
		}
		if w.depth != 0 {
			continue
		}
		frame := w.buffer[:w.scan]
		if _, err := w.ResponseWriter.Write(NormalizeResponseIdentity(frame, w.format, w.model)); err != nil {
			return err
		}
		w.buffer = w.buffer[w.scan:]
		w.scan = 0
	}
	return nil
}
func (w *responseIdentityWriter) finish() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.buffer) > 0 {
		body := w.buffer
		if !strings.HasPrefix(w.Header().Get("Content-Type"), "text/event-stream") {
			body = NormalizeResponseIdentity(body, w.format, w.model)
		}
		_, _ = w.ResponseWriter.Write(body)
		w.buffer = nil
	}
	w.passing = true
}
