package common

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResponseIdentityProtocolFields(t *testing.T) {
	setting := operation_setting.GetGeneralSetting()
	previous := setting.NormalizeResponseModelEnabled
	t.Cleanup(func() { setting.NormalizeResponseModelEnabled = previous })
	for _, tc := range []struct {
		name       string
		format     types.RelayFormat
		body, want string
	}{
		{"chat body and tools", types.RelayFormatOpenAI, `{"model":"up","choices":[{"message":{"content":"up","tool_calls":[{"function":{"arguments":"{\"model\":\"up\"}"}}]}}],"usage":{"model":"up","total_tokens":5}}`, `{"model":"client","choices":[{"message":{"content":"up","tool_calls":[{"function":{"arguments":"{\"model\":\"up\"}"}}]}}],"usage":{"model":"up","total_tokens":5}}`},
		{"responses", types.RelayFormatOpenAIResponses, `{"type":"response.completed","response":{"model":"up","metadata":{"model":"up"}}}`, `{"type":"response.completed","response":{"model":"client","metadata":{"model":"up"}}}`},
		{"claude", types.RelayFormatClaude, `{"type":"message_start","message":{"model":"up","content":[]}}`, `{"type":"message_start","message":{"model":"client","content":[]}}`},
		{"gemini", types.RelayFormatGemini, `[{"modelVersion":"up","candidates":[{"content":{"parts":[{"text":"up"}]}}]},{"model_version":"up"}]`, `[{"modelVersion":"client","candidates":[{"content":{"parts":[{"text":"up"}]}}]},{"model_version":"client"}]`},
		{"realtime session", types.RelayFormatOpenAIRealtime, `{"type":"session.updated","session":{"model":"up","instructions":"up"}}`, `{"type":"session.updated","session":{"model":"client","instructions":"up"}}`},
		{"realtime response", types.RelayFormatOpenAIRealtime, `{"type":"response.done","response":{"model":"up","usage":{"total_tokens":5}}}`, `{"type":"response.done","response":{"model":"client","usage":{"total_tokens":5}}}`},
		{"private plugin", types.RelayFormatTask, `{"model":"up","result":{"model":"up"}}`, `{"model":"up","result":{"model":"up"}}`},
		{"model list", types.RelayFormatOpenAI, `{"object":"list","model":"up","data":[{"id":"up"}]}`, `{"object":"list","model":"up","data":[{"id":"up"}]}`},
		{"missing identity", types.RelayFormatEmbedding, `{"data":[{"embedding":[0,1]}]}`, `{"data":[{"embedding":[0,1]}]}`},
		{"malformed", types.RelayFormatOpenAI, `{"model":"up"`, `{"model":"up"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := []byte(tc.body)
			setting.NormalizeResponseModelEnabled = false
			assert.Equal(t, tc.body, string(NormalizeResponseIdentity(input, tc.format, "client")))
			setting.NormalizeResponseModelEnabled = true
			assert.Equal(t, tc.want, string(NormalizeResponseIdentity(input, tc.format, "client")))
			assert.Equal(t, tc.body, string(input), "delivery cannot mutate upstream bytes")
			assert.Equal(t, tc.body, string(NormalizeResponseIdentity(input, tc.format, "")))
		})
	}
}

func TestResponseIdentityHTTPDelivery(t *testing.T) {
	setting := operation_setting.GetGeneralSetting()
	previous := setting.NormalizeResponseModelEnabled
	t.Cleanup(func() { setting.NormalizeResponseModelEnabled = previous })
	for _, enabled := range []bool{false, true} {
		for _, tc := range []struct {
			name, contentType string
			format            types.RelayFormat
			chunks            []string
			want              string
		}{
			{"fragmented JSON", "application/json", types.RelayFormatOpenAI, []string{`{"model":"`, `up","content":{"model":"up"}}`}, `{"model":"client","content":{"model":"up"}}`},
			{"SSE", "text/event-stream", types.RelayFormatOpenAIResponses, []string{": ping\r\n\r\nevent: response.created\r\nid: 9\r\ndata: {\r\ndata: \"type\":\"response.created\",\"response\":{\"model\":\"up\"}}\r\n", "\r\ndata: [DONE]\r\n\r\n"}, ": ping\r\n\r\nevent: response.created\r\nid: 9\r\ndata: {\r\ndata: \"type\":\"response.created\",\"response\":{\"model\":\"client\"}}\r\n\r\ndata: [DONE]\r\n\r\n"},
			{"Gemini array", "application/json", types.RelayFormatGemini, []string{`[{"modelVersion":"up","candidates":[{"text":"brace } and \\"}],"usageMetadata":{"totalTokenCount":5}}`, `,{"model_version":"up"}`, `]`}, `[{"modelVersion":"client","candidates":[{"text":"brace } and \\"}],"usageMetadata":{"totalTokenCount":5}},{"model_version":"client"}]`},
			{"binary", "audio/mpeg", types.RelayFormatOpenAIAudio, []string{`{"model":"up"}`}, `{"model":"up"}`},
			{"private task", "application/json", types.RelayFormatTask, []string{`{"model":"up"}`}, `{"model":"up"}`},
		} {
			t.Run(tc.name+map[bool]string{true: " enabled", false: " disabled"}[enabled], func(t *testing.T) {
				setting.NormalizeResponseModelEnabled = enabled
				recorder := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(recorder)
				c.Header("Content-Type", tc.contentType)
				parent := c.Writer
				finish := ResponseIdentityBoundary(c, tc.format, "client")
				for _, chunk := range tc.chunks {
					n, err := c.Writer.Write([]byte(chunk))
					require.NoError(t, err)
					assert.Equal(t, len(chunk), n)
					c.Writer.Flush()
				}
				finish()
				want := tc.want
				if !enabled {
					want = strings.Join(tc.chunks, "")
				}
				assert.Equal(t, want, recorder.Body.String())
				assert.Equal(t, parent, c.Writer)
			})
		}
	}
}

func TestResponseIdentityPreservesIncompleteFramesUntilComplete(t *testing.T) {
	setting := operation_setting.GetGeneralSetting()
	previous := setting.NormalizeResponseModelEnabled
	setting.NormalizeResponseModelEnabled = true
	t.Cleanup(func() { setting.NormalizeResponseModelEnabled = previous })
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Header("Content-Type", "text/event-stream")
	c.Header("Content-Length", "999")
	finish := ResponseIdentityBoundary(c, types.RelayFormatOpenAI, "client")
	_, err := c.Writer.WriteString("data: {\"model\":\"up\"}")
	require.NoError(t, err)
	c.Writer.Flush()
	assert.Empty(t, recorder.Body.String())
	assert.Empty(t, recorder.Header().Get("Content-Length"))
	_, err = c.Writer.WriteString("\n\n")
	require.NoError(t, err)
	assert.Equal(t, "data: {\"model\":\"client\"}\n\n", recorder.Body.String())
	finish()
	// Error envelopes, even with a model extension, are not successful protocol responses.
	recorder = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(recorder)
	finish = ResponseIdentityBoundary(c, types.RelayFormatOpenAI, "client")
	c.JSON(http.StatusBadRequest, gin.H{"model": "up"})
	finish()
	assert.JSONEq(t, `{"model":"up"}`, recorder.Body.String())
}
