package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIMaintenanceRejectsBeforeDistribution(t *testing.T) {
	setting := operation_setting.GetMaintenanceSetting()
	previous := *setting
	t.Cleanup(func() { *setting = previous })
	setting.Enabled, setting.Message = true, "planned maintenance"
	for _, tc := range []struct{ path, fragment string }{
		{"/v1/chat/completions", `"code":"api_maintenance"`},
		{"/v1/messages", `"type":"overloaded_error"`},
		{"/v1beta/models/gemini:generateContent", `"status":"UNAVAILABLE"`},
		{"/mj/submit/imagine", `"description":"planned maintenance`},
		{"/vendor/jobs", `"code":"api_maintenance"`},
	} {
		t.Run(tc.path, func(t *testing.T) {
			engine := gin.New()
			called := false
			engine.POST(tc.path, APIExecutionMaintenance(), Distribute(), func(c *gin.Context) { called = true })
			response := httptest.NewRecorder()
			// Deliberately invalid input proves distribution/body parsing is not entered.
			engine.ServeHTTP(response, httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader("invalid")))
			require.Equal(t, http.StatusServiceUnavailable, response.Code)
			assert.Contains(t, response.Body.String(), tc.fragment)
			assert.False(t, called)
		})
	}
	engine := gin.New()
	engine.POST("/execute", APIExecutionMaintenance(), func(c *gin.Context) { c.Status(http.StatusAccepted) })
	setting.Enabled = false
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/execute", nil))
	assert.Equal(t, http.StatusAccepted, response.Code)
}

func TestRealtimeMaintenanceBlocksImplicitAndExplicitGeneration(t *testing.T) {
	setting := operation_setting.GetMaintenanceSetting()
	previous := *setting
	t.Cleanup(func() { *setting = previous })
	setting.Enabled, setting.Message = true, "planned maintenance"
	for _, event := range []string{"response.create", "input_audio_buffer.append", "input_audio_buffer.commit", "session.update", "conversation.item.create"} {
		payload, err := common.Marshal(map[string]string{"type": event, "event_id": "next-turn"})
		require.NoError(t, err)
		result := service.RealtimeMaintenanceRejection(payload)
		var body map[string]any
		require.NoError(t, common.Unmarshal(result, &body))
		assert.Equal(t, "error", body["type"])
		assert.Equal(t, "api_maintenance", body["error"].(map[string]any)["code"])
		assert.Equal(t, "next-turn", body["error"].(map[string]any)["event_id"])
	}
	for _, event := range []string{"response.cancel", "input_audio_buffer.clear", "conversation.item.truncate"} {
		payload, err := common.Marshal(map[string]string{"type": event})
		require.NoError(t, err)
		assert.Nil(t, service.RealtimeMaintenanceRejection(payload))
	}
	setting.Enabled = false
	assert.Nil(t, service.RealtimeMaintenanceRejection([]byte(`{"type":"response.create"}`)))
}

func TestNativePluginMaintenancePreservesMessageAnd503(t *testing.T) {
	plugin := compileTaskRoutePlugin(t, genericTaskPluginSource+`
export const native = { error: function(ctx, error) {
  return {__status: 200, __body: {code: error.code, message: error.message, retryable: error.retryable}};
}};
`)
	setting := operation_setting.GetMaintenanceSetting()
	previous := *setting
	t.Cleanup(func() { *setting = previous })
	setting.Enabled, setting.Message = true, "planned maintenance"
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request = httptest.NewRequest(http.MethodPost, "/vendor/jobs", nil)
	c.Set(jsplugin.ContextKeyPinnedRoute, jsplugin.PinnedRoute{Plugin: plugin})
	c.Set(jsplugin.ContextKeyRouteRequest, jsplugin.RouteRequestContext{Path: "/vendor/jobs", Method: http.MethodPost})
	require.True(t, RejectAPIExecution(c))
	assert.Equal(t, http.StatusServiceUnavailable, response.Code)
	assert.JSONEq(t, `{"code":"api_maintenance","message":"planned maintenance","retryable":false}`, response.Body.String())
}
