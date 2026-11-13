package service

import (
	"context"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

// ScopedLogCleanupPayload preserves the legacy timestamp/batch payload contract.
// A missing scope means only consumption and error logs, never all log types.
type ScopedLogCleanupPayload struct {
	LogCleanupPayload
	Scope         string `json:"scope"`
	Automatic     bool   `json:"automatic"`
	RetentionDays int    `json:"retention_days,omitempty"`
}

type scopedLogCleanupHandler struct{}

// RegisterLogCleanupHandler replaces the legacy on-demand handler with the
// scheduled, scope-aware implementation. Call from the system-task init hook.
func RegisterLogCleanupHandler() {
	RegisterSystemTaskHandler(scopedLogCleanupHandler{})
}

func (scopedLogCleanupHandler) Type() string { return model.SystemTaskTypeLogCleanup }
func (scopedLogCleanupHandler) Enabled() bool {
	s := operation_setting.GetLogSetting()
	return s.AutoCleanupEnabled && s.CleanupValid()
}
func (scopedLogCleanupHandler) Interval() time.Duration { return 24 * time.Hour }
func (scopedLogCleanupHandler) NewPayload() any {
	s := *operation_setting.GetLogSetting()
	return ScopedLogCleanupPayload{
		LogCleanupPayload: LogCleanupPayload{
			TargetTimestamp: common.GetTimestamp() - int64(s.CleanupRetentionDays)*86400,
			BatchSize:       logCleanupBatchSize,
		},
		Scope: s.CleanupScope, Automatic: true, RetentionDays: s.CleanupRetentionDays,
	}
}

func LogCleanupScopeTypes(scope string) ([]int, error) {
	switch scope {
	case "", "consume_error":
		return model.LogCleanupTypes()
	case "consume":
		return model.LogCleanupTypes(model.LogTypeConsume)
	case "error":
		return model.LogCleanupTypes(model.LogTypeError)
	default:
		return nil, common.NewMessage("Only consumption and error logs can be cleaned")
	}
}

// StartScopedLogCleanupTask uses the existing per-type active key and lease, so
// manual requests, schedules and other nodes cannot run competing cleanup jobs.
func StartScopedLogCleanupTask(targetTimestamp int64, scope string) (*model.SystemTask, error) {
	if targetTimestamp <= 0 || targetTimestamp > common.GetTimestamp() {
		return nil, common.NewMessage("Log cleanup timestamp must be in the past")
	}
	if _, err := LogCleanupScopeTypes(scope); err != nil {
		return nil, err
	}
	if scope == "" {
		scope = "consume_error"
	}
	task, _, err := EnqueueSystemTask(model.SystemTaskTypeLogCleanup, ScopedLogCleanupPayload{
		LogCleanupPayload: LogCleanupPayload{TargetTimestamp: targetTimestamp, BatchSize: logCleanupBatchSize},
		Scope:             scope,
	})
	return task, err
}

func (scopedLogCleanupHandler) Run(ctx context.Context, task *model.SystemTask, runnerID string) {
	var payload ScopedLogCleanupPayload
	if err := task.DecodePayload(&payload); err != nil {
		failSystemTask(task, runnerID, err)
		return
	}
	types, err := LogCleanupScopeTypes(payload.Scope)
	if err != nil {
		failSystemTask(task, runnerID, err)
		return
	}
	if payload.TargetTimestamp <= 0 || payload.TargetTimestamp > common.GetTimestamp() {
		failSystemTask(task, runnerID, common.NewMessage("Log cleanup timestamp must be in the past"))
		return
	}
	if payload.BatchSize <= 0 {
		payload.BatchSize = logCleanupBatchSize
	}
	var state LogCleanupState
	if err := task.DecodeState(&state); err != nil {
		failSystemTask(task, runnerID, err)
		return
	}
	for {
		if err := ctx.Err(); err != nil {
			failSystemTask(task, runnerID, err)
			return
		}
		remaining, err := model.CountOldLog(ctx, payload.TargetTimestamp, types...)
		if err != nil {
			failSystemTask(task, runnerID, err)
			return
		}
		// Only count rows actually deleted by this task, not concurrent removals.
		state.Total = state.Processed + remaining
		state.Remaining = remaining
		state.Progress = logCleanupProgress(state.Processed, state.Total)
		if err := model.UpdateSystemTaskState(task.TaskID, runnerID, state); err != nil {
			logSystemTaskLockError(ctx, task, err)
			return
		}
		if remaining == 0 {
			break
		}
		deleted, err := model.DeleteOldLogBatch(ctx, payload.TargetTimestamp, payload.BatchSize, types...)
		if err != nil {
			failSystemTask(task, runnerID, err)
			return
		}
		state.Processed += deleted
		if deleted == 0 {
			failSystemTask(task, runnerID, common.NewMessage("No log rows were deleted"))
			return
		}
		// Yield between batches and observe lease loss before issuing more SQL.
		timer := time.NewTimer(50 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			failSystemTask(task, runnerID, ctx.Err())
			return
		case <-timer.C:
		}
	}
	result := struct {
		LogCleanupResult
		Scope           string `json:"scope"`
		TargetTimestamp int64  `json:"target_timestamp"`
		Automatic       bool   `json:"automatic"`
	}{LogCleanupResult{DeletedCount: state.Processed}, payload.Scope, payload.TargetTimestamp, payload.Automatic}
	if err := model.FinishSystemTask(task.TaskID, runnerID, model.SystemTaskStatusSucceeded, result, ""); err != nil {
		logSystemTaskLockError(ctx, task, err)
	}
}
