package service

import (
	"context"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

// Pending awards belong to their immutable receipt, not the current global switch.
// The shared runner supplies cross-process leases and hot-restart admission.
type arrivalRebateRetryHandler struct{}

func init() { RegisterSystemTaskHandler(arrivalRebateRetryHandler{}) }

func (arrivalRebateRetryHandler) Type() string            { return "arrival_rebate_retry" }
func (arrivalRebateRetryHandler) Interval() time.Duration { return time.Minute }
func (arrivalRebateRetryHandler) NewPayload() any         { return nil }
func (arrivalRebateRetryHandler) Enabled() bool {
	if !operation_setting.IsPaymentComplianceConfirmed() {
		return false
	}
	var count int64
	return model.DB.Model(&model.ArrivalEvent{}).Where("status = ?", "pending").Limit(1).Count(&count).Error == nil && count > 0
}
func (arrivalRebateRetryHandler) Run(ctx context.Context, task *model.SystemTask, runnerID string) {
	if err := ctx.Err(); err != nil {
		failSystemTask(task, runnerID, err)
		return
	}
	paid, err := model.RetryPendingArrivalRebates(100)
	if err != nil {
		failSystemTask(task, runnerID, err)
		return
	}
	if err := model.FinishSystemTask(task.TaskID, runnerID, model.SystemTaskStatusSucceeded, map[string]int{"paid_count": paid}, ""); err != nil {
		logSystemTaskLockError(ctx, task, err)
	}
}
