package service

import (
	"errors"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

// APIMaintenanceError is evaluated for each new execution, including those on
// an already established socket. Accepted executions are allowed to drain.
func APIMaintenanceError() *types.NewAPIError {
	setting := operation_setting.GetMaintenanceSetting()
	if !setting.Enabled {
		return nil
	}
	message := strings.TrimSpace(setting.Message)
	if message == "" {
		message = "The API is temporarily under maintenance. Please try again later."
	}
	return types.NewErrorWithStatusCode(errors.New(message), types.ErrorCode("api_maintenance"), http.StatusServiceUnavailable,
		types.ErrOptionWithSkipRetry(), types.ErrOptionWithNoRecordErrorLog())
}

// RealtimeMaintenanceRejection also blocks audio/VAD and session changes that
// can implicitly start a new response. Cancellation and buffer clearing remain
// available, while upstream output from an already accepted turn is untouched.
func RealtimeMaintenanceRejection(message []byte) []byte {
	apiError := APIMaintenanceError()
	if apiError == nil {
		return nil
	}
	var event struct {
		Type    string `json:"type"`
		EventID string `json:"event_id"`
	}
	_ = common.Unmarshal(message, &event)
	switch event.Type {
	case "response.cancel", "input_audio_buffer.clear", "conversation.item.delete", "conversation.item.truncate":
		return nil
	}
	payload, _ := common.Marshal(map[string]any{
		"type":  "error",
		"error": map[string]any{"type": "server_error", "code": "api_maintenance", "message": apiError.Error(), "event_id": event.EventID},
	})
	return payload
}
