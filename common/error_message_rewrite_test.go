package common_test

import (
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErrorMessageRewrite(t *testing.T) {
	for _, tc := range []struct {
		name, rules, message, expected string
		source                         common.ErrorMessageSource
	}{
		{"contains case insensitive and request id", `[{"enabled":true,"patterns":["timeout"],"replace":"Try later"}]`, "TIMEOUT (request id: req_123)", "Try later (request id: req_123)", common.ErrorMessageSourceUpstream},
		{"exact preserves unmatched", `[{"enabled":true,"match_type":"exact_any","patterns":["timeout"],"replace":"Try later"}]`, "timeout now", "timeout now", common.ErrorMessageSourceLocal},
		{"exact ignores surrounding whitespace", `[{"enabled":true,"match_type":"exact_any","patterns":["timeout"],"replace":"Try later"}]`, " TIMEOUT ", "Try later", common.ErrorMessageSourceLocal},
		{"regex uses Go semantics", `[{"enabled":true,"match_type":"regex_any","patterns":["(?i)^timeout [0-9]+$"],"replace":"Try later"}]`, "Timeout 42", "Try later", common.ErrorMessageSourceUpstream},
		{"source cannot broaden", `[{"enabled":true,"source":"upstream","patterns":["timeout"],"replace":"Try later"}]`, "timeout", "timeout", common.ErrorMessageSourceLocal},
		{"local source", `[{"enabled":true,"source":"local","patterns":["timeout"],"replace":"Local"}]`, "timeout", "Local", common.ErrorMessageSourceLocal},
		{"priority then key", `[{"key":"z","enabled":true,"priority":2,"patterns":["x"],"replace":"Z"},{"key":"a","enabled":true,"priority":2,"patterns":["x"],"replace":"A"},{"enabled":true,"priority":1,"patterns":["x"],"replace":"Low"}]`, "x", "A", common.ErrorMessageSourceUpstream},
		{"disabled ignored", `[{"enabled":false,"patterns":["x"],"replace":"Y"}]`, "x", "x", common.ErrorMessageSourceLocal},
		{"does not rewrite twice", `[{"enabled":true,"priority":2,"patterns":["x"],"replace":"y"},{"enabled":true,"patterns":["y"],"replace":"z"}]`, "x", "y", common.ErrorMessageSourceLocal},
		{"empty ignored", `[{"enabled":true,"patterns":["x"],"replace":"Y"}]`, " ", " ", common.ErrorMessageSourceLocal},
	} {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := common.PreviewErrorMessageRewrite(tc.message, tc.rules, tc.source)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestErrorMessageRewriteValidation(t *testing.T) {
	for _, raw := range []string{`{}`, `null`, `[null]`, `[`, `[{"patterns":[],"replace":"y"}]`, `[{"patterns":["x"],"replace":" "}]`, `[{"source":"spoofed","patterns":["x"],"replace":"y"}]`, `[{"match_type":"bad","patterns":["x"],"replace":"y"}]`, `[{"match_type":"regex_any","patterns":["("],"replace":"y"}]`} {
		t.Run(raw, func(t *testing.T) { assert.Error(t, common.ValidateErrorMessageRewriteRules(raw)) })
	}
	_, err := common.PreviewErrorMessageRewrite("x", "[]", common.ErrorMessageSourceAll)
	assert.Error(t, err)
}

func TestErrorMessageRewriteOutputAndConfiguration(t *testing.T) {
	common.OptionMapRWMutex.Lock()
	original := common.OptionMap
	common.OptionMap = map[string]string{
		common.OptionErrorMessageRewriteEnabled: "true",
		common.OptionErrorMessageRewriteRules:   `[{"enabled":true,"patterns":["Invalid parameters"],"replace":"Custom message"}]`,
	}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() { common.OptionMapRWMutex.Lock(); common.OptionMap = original; common.OptionMapRWMutex.Unlock() })

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	common.ApiError(c, common.NewMessage("Invalid parameters"))
	var response map[string]any
	require.NoError(t, common.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, "Custom message", response["message"])
	assert.NotContains(t, response, "message_key")
	assert.NotContains(t, response, "message_params")

	common.OptionMapRWMutex.Lock()
	common.OptionMap[common.OptionErrorMessageRewriteEnabled] = "false"
	common.OptionMapRWMutex.Unlock()
	assert.Equal(t, "Invalid parameters", common.RewriteErrorMessage("Invalid parameters"))

	common.OptionMapRWMutex.Lock()
	common.OptionMap[common.OptionErrorMessageRewriteEnabled] = "true"
	common.OptionMap[common.OptionErrorMessageRewriteRules] = `invalid`
	common.OptionMapRWMutex.Unlock()
	assert.Equal(t, "Invalid parameters", common.RewriteErrorMessage("Invalid parameters"))

	upstream := types.WithOpenAIError(types.OpenAIError{Message: "Invalid parameters", Type: "local", Code: "invalid_request"}, 502)
	assert.Equal(t, types.ErrorMessageSourceUpstream, upstream.GetErrorSource())
	wrapped := types.NewErrorWithStatusCode(upstream, types.ErrorCodeInvalidRequest, 502)
	assert.Equal(t, types.ErrorMessageSourceUpstream, wrapped.GetErrorSource())
	local := types.NewOpenAIError(errors.New("local"), types.ErrorCodeInvalidRequest, 400)
	assert.Equal(t, types.ErrorMessageSourceLocal, local.GetErrorSource())
	blocked := types.NewOpenAIError(errors.New("blocked"), types.ErrorCodePromptBlocked, 400)
	assert.Equal(t, types.ErrorMessageSourceUpstream, blocked.GetErrorSource())
}
