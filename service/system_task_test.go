package service

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/glebarez/sqlite"
	"gorm.io/driver/clickhouse"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withSystemTaskRegistry swaps the package registry for the given handlers for
// the duration of a test and restores the original registry afterward.
func withSystemTaskRegistry(t *testing.T, handlers ...SystemTaskHandler) {
	t.Helper()
	systemTaskHandlersMu.Lock()
	saved := systemTaskHandlers
	systemTaskHandlers = map[string]SystemTaskHandler{}
	for _, h := range handlers {
		systemTaskHandlers[h.Type()] = h
	}
	systemTaskHandlersMu.Unlock()
	t.Cleanup(func() {
		systemTaskHandlersMu.Lock()
		systemTaskHandlers = saved
		systemTaskHandlersMu.Unlock()
	})
}

type stubScheduledHandler struct {
	taskType string
	enabled  bool
	interval time.Duration
	onRun    func(ctx context.Context, task *model.SystemTask, runnerID string)
}

type stubSystemTaskRunResult struct {
	taskID   string
	taskType string
	err      error
}

func (h *stubScheduledHandler) Type() string { return h.taskType }

func (h *stubScheduledHandler) Run(ctx context.Context, task *model.SystemTask, runnerID string) {
	if h.onRun != nil {
		h.onRun(ctx, task, runnerID)
	}
}

func (h *stubScheduledHandler) Enabled() bool           { return h.enabled }
func (h *stubScheduledHandler) Interval() time.Duration { return h.interval }
func (h *stubScheduledHandler) NewPayload() any         { return nil }

func countSystemTasks(t *testing.T, taskType string) int64 {
	t.Helper()
	var count int64
	require.NoError(t, model.DB.Model(&model.SystemTask{}).Where("type = ?", taskType).Count(&count).Error)
	return count
}

func TestSystemTaskSchedulerCreatesWhenDueAndDedups(t *testing.T) {
	truncate(t)

	handler := &stubScheduledHandler{taskType: "test_scheduled", enabled: true, interval: time.Minute}
	withSystemTaskRegistry(t, handler)

	runSystemTaskScheduler()
	require.Equal(t, int64(1), countSystemTasks(t, handler.taskType))

	// An active (pending) row already exists, so a second pass must not create
	// another row.
	runSystemTaskScheduler()
	require.Equal(t, int64(1), countSystemTasks(t, handler.taskType))

	// Finish the run; with a fresh updated_at the next run is not due yet.
	latest, err := model.GetLatestSystemTask(handler.taskType)
	require.NoError(t, err)
	require.NotNil(t, latest)
	_, claimed, err := model.ClaimSystemTask(latest.ID, handler.taskType, "runner-a", common.GetTimestamp()+60)
	require.NoError(t, err)
	require.True(t, claimed)
	require.NoError(t, model.FinishSystemTask(latest.TaskID, "runner-a", model.SystemTaskStatusSucceeded, nil, ""))

	runSystemTaskScheduler()
	require.Equal(t, int64(1), countSystemTasks(t, handler.taskType))

	// Backdate the finished row beyond the interval -> the job becomes due again.
	require.NoError(t, model.DB.Model(&model.SystemTask{}).
		Where("task_id = ?", latest.TaskID).
		Update("updated_at", common.GetTimestamp()-120).Error)

	runSystemTaskScheduler()
	require.Equal(t, int64(2), countSystemTasks(t, handler.taskType))
}

func TestSystemTaskSchedulerSkipsDisabled(t *testing.T) {
	truncate(t)

	handler := &stubScheduledHandler{taskType: "test_disabled", enabled: false, interval: time.Minute}
	withSystemTaskRegistry(t, handler)

	runSystemTaskScheduler()
	assert.Equal(t, int64(0), countSystemTasks(t, handler.taskType))
}

func TestSystemTaskClaimPassDispatchesByType(t *testing.T) {
	truncate(t)

	ran := make(chan stubSystemTaskRunResult, 1)
	handler := &stubScheduledHandler{
		taskType: "test_dispatch",
		enabled:  true,
		interval: time.Minute,
		onRun: func(_ context.Context, task *model.SystemTask, runnerID string) {
			ran <- stubSystemTaskRunResult{
				taskType: task.Type,
				err:      model.FinishSystemTask(task.TaskID, runnerID, model.SystemTaskStatusSucceeded, nil, ""),
			}
		},
	}
	withSystemTaskRegistry(t, handler)

	_, err := model.CreateSystemTask(handler.taskType, nil, nil)
	require.NoError(t, err)

	runSystemTaskClaimPass("runner-dispatch")

	select {
	case got := <-ran:
		require.NoError(t, got.err)
		assert.Equal(t, handler.taskType, got.taskType)
	case <-time.After(2 * time.Second):
		t.Fatal("claimed task was not dispatched to its handler")
	}

	require.Eventually(t, func() bool {
		latest, err := model.GetLatestSystemTask(handler.taskType)
		return err == nil && latest != nil && latest.Status == model.SystemTaskStatusSucceeded
	}, 2*time.Second, 20*time.Millisecond)
}

func TestSystemTaskClaimPassDispatchesEarliestPendingByType(t *testing.T) {
	truncate(t)

	ran := make(chan stubSystemTaskRunResult, 2)
	handlerA := &stubScheduledHandler{
		taskType: "test_dispatch_a",
		enabled:  true,
		interval: time.Minute,
		onRun: func(_ context.Context, task *model.SystemTask, runnerID string) {
			ran <- stubSystemTaskRunResult{
				taskID: task.TaskID,
				err:    model.FinishSystemTask(task.TaskID, runnerID, model.SystemTaskStatusSucceeded, nil, ""),
			}
		},
	}
	handlerB := &stubScheduledHandler{
		taskType: "test_dispatch_b",
		enabled:  true,
		interval: time.Minute,
		onRun: func(_ context.Context, task *model.SystemTask, runnerID string) {
			ran <- stubSystemTaskRunResult{
				taskID: task.TaskID,
				err:    model.FinishSystemTask(task.TaskID, runnerID, model.SystemTaskStatusSucceeded, nil, ""),
			}
		},
	}
	withSystemTaskRegistry(t, handlerA, handlerB)

	firstA, err := model.CreateSystemTask(handlerA.taskType, nil, nil)
	require.NoError(t, err)
	secondTaskID, err := model.GenerateSystemTaskID()
	require.NoError(t, err)
	secondA := &model.SystemTask{
		TaskID: secondTaskID,
		Type:   handlerA.taskType,
		Status: model.SystemTaskStatusPending,
	}
	require.NoError(t, model.DB.Create(secondA).Error)
	firstB, err := model.CreateSystemTask(handlerB.taskType, nil, nil)
	require.NoError(t, err)

	runSystemTaskClaimPass("runner-dispatch")

	got := map[string]bool{}
	for range 2 {
		select {
		case result := <-ran:
			require.NoError(t, result.err)
			got[result.taskID] = true
		case <-time.After(2 * time.Second):
			t.Fatal("claimed tasks were not dispatched to their handlers")
		}
	}

	assert.True(t, got[firstA.TaskID])
	assert.True(t, got[firstB.TaskID])
	assert.False(t, got[secondA.TaskID])

	require.Eventually(t, func() bool {
		reloaded, err := model.GetSystemTaskByTaskID(secondA.TaskID)
		return err == nil && reloaded != nil && reloaded.Status == model.SystemTaskStatusPending
	}, 2*time.Second, 20*time.Millisecond)
}

func TestEnqueueSystemTaskReportsCreatedAndExistingActive(t *testing.T) {
	truncate(t)

	first, created, err := EnqueueSystemTask("test_enqueue", map[string]bool{"manual": true})
	require.NoError(t, err)
	require.True(t, created)
	require.NotNil(t, first)

	existing, created, err := EnqueueSystemTask("test_enqueue", nil)
	require.NoError(t, err)
	require.False(t, created)
	require.NotNil(t, existing)
	assert.Equal(t, first.TaskID, existing.TaskID)

	_, claimed, err := model.ClaimSystemTask(first.ID, first.Type, "runner-a", common.GetTimestamp()+60)
	require.NoError(t, err)
	require.True(t, claimed)
	require.NoError(t, model.FinishSystemTask(first.TaskID, "runner-a", model.SystemTaskStatusSucceeded, nil, ""))

	second, created, err := EnqueueSystemTask("test_enqueue", nil)
	require.NoError(t, err)
	require.True(t, created)
	require.NotNil(t, second)
	assert.NotEqual(t, first.TaskID, second.TaskID)
}

func TestLogCleanupScheduledScopeAndDedup(t *testing.T) {
	truncate(t)
	s := operation_setting.GetLogSetting()
	saved := *s
	t.Cleanup(func() { *s = saved })
	*s = operation_setting.LogSetting{CleanupRetentionDays: 30, CleanupScope: "error"}
	h := scopedLogCleanupHandler{}
	withSystemTaskRegistry(t, h)
	runSystemTaskScheduler()
	assert.Zero(t, countSystemTasks(t, h.Type()), "default off")
	s.AutoCleanupEnabled = true
	runSystemTaskScheduler()
	runSystemTaskScheduler()
	assert.Equal(t, int64(1), countSystemTasks(t, h.Type()))
	task, err := model.GetActiveSystemTask(h.Type())
	require.NoError(t, err)
	require.NotNil(t, task)
	var payload ScopedLogCleanupPayload
	require.NoError(t, task.DecodePayload(&payload))
	assert.Equal(t, "error", payload.Scope)
	assert.True(t, payload.Automatic)
	assert.Equal(t, 30, payload.RetentionDays)
	assert.InDelta(t, common.GetTimestamp()-30*86400, payload.TargetTimestamp, 1)
	manual, err := StartScopedLogCleanupTask(common.GetTimestamp()-86400, "consume")
	require.NoError(t, err)
	assert.Equal(t, task.TaskID, manual.TaskID)
	for _, scope := range []string{"all", "topup", "login"} {
		_, err := StartScopedLogCleanupTask(common.GetTimestamp()-86400, scope)
		assert.Error(t, err, scope)
	}
	for _, timestamp := range []int64{0, -1, common.GetTimestamp() + 86400} {
		_, err := StartScopedLogCleanupTask(timestamp, "")
		assert.Error(t, err)
	}
	for _, value := range []string{"0", "-1", "36501", "abc"} {
		assert.Error(t, operation_setting.ValidateLogCleanupOption("log_setting.cleanup_retention_days", value))
	}
	assert.NoError(t, operation_setting.ValidateLogCleanupOption("log_setting.cleanup_retention_days", "1"))
	assert.Error(t, operation_setting.ValidateLogCleanupOption("log_setting.cleanup_scope", "all"))
	s.CleanupRetentionDays = 0
	assert.False(t, h.Enabled(), "invalid persisted configuration must fail closed")
}

// These optional DSNs must point to disposable databases: this test owns logs.
// The main/task database stays separate, including when LOG_DB is ClickHouse.
func TestLogCleanupDatabaseMatrix(t *testing.T) {
	for _, dialect := range []string{"sqlite", "mysql", "postgres", "clickhouse"} {
		t.Run(dialect, func(t *testing.T) {
			var driver gorm.Dialector = sqlite.Open(":memory:")
			dbType := common.DatabaseTypeSQLite
			if dialect != "sqlite" {
				env := map[string]string{"mysql": "LOG_CLEANUP_MYSQL_DSN", "postgres": "LOG_CLEANUP_POSTGRES_DSN", "clickhouse": "LOG_CLEANUP_CLICKHOUSE_DSN"}[dialect]
				dsn := os.Getenv(env)
				if dsn == "" {
					t.Skip("real engine not configured: " + env)
				}
				switch dialect {
				case "mysql":
					driver = mysql.Open(dsn)
					dbType = common.DatabaseTypeMySQL
				case "postgres":
					driver = postgres.Open(dsn)
					dbType = common.DatabaseTypePostgreSQL
				case "clickhouse":
					driver = clickhouse.Open(dsn)
					dbType = common.DatabaseTypeClickHouse
				}
			}
			db, err := gorm.Open(driver, &gorm.Config{})
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			sqlDB.SetMaxOpenConns(1)
			t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
			savedLogDB, savedType := model.LOG_DB, common.LogDatabaseType()
			model.LOG_DB = db
			common.SetLogDatabaseType(dbType)
			t.Cleanup(func() { model.LOG_DB = savedLogDB; common.SetLogDatabaseType(savedType) })
			if dialect == "clickhouse" {
				require.NoError(t, db.Exec("CREATE TABLE IF NOT EXISTS logs (id Int64, type Int64, created_at Int64) ENGINE = MergeTree ORDER BY (created_at, id)").Error)
				require.NoError(t, db.Exec("TRUNCATE TABLE logs").Error)
			} else {
				require.NoError(t, db.AutoMigrate(&model.Log{}))
				require.NoError(t, db.AutoMigrate(&model.Log{}))
				require.NoError(t, db.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&model.Log{}).Error)
			}
			var version string
			versionSQL := "SELECT version()"
			if dialect == "sqlite" {
				versionSQL = "SELECT sqlite_version()"
			}
			require.NoError(t, db.Raw(versionSQL).Scan(&version).Error)
			t.Logf("real log engine %s version %s", dialect, version)
			cutoff := common.GetTimestamp() - 86400
			for _, row := range []map[string]any{
				{"id": 1, "type": model.LogTypeConsume, "created_at": cutoff - 1},
				{"id": 10, "type": model.LogTypeConsume, "created_at": cutoff - 2},
				{"id": 2, "type": model.LogTypeError, "created_at": cutoff - 1},
				{"id": 3, "type": model.LogTypeConsume, "created_at": cutoff},
				{"id": 4, "type": model.LogTypeTopup, "created_at": cutoff - 1},
				{"id": 5, "type": model.LogTypeRefund, "created_at": cutoff - 1},
				{"id": 6, "type": model.LogTypeManage, "created_at": cutoff - 1},
				{"id": 7, "type": model.LogTypeLogin, "created_at": cutoff - 1},
				{"id": 8, "type": model.LogTypeSystem, "created_at": cutoff - 1},
				{"id": 9, "type": 999, "created_at": cutoff - 1},
			} {
				require.NoError(t, db.Table("logs").Create(row).Error)
			}
			ctx := context.Background()
			count, err := model.CountOldLog(ctx, cutoff)
			require.NoError(t, err)
			assert.Equal(t, int64(3), count)
			_, err = model.DeleteOldLogBatch(ctx, cutoff, 1, model.LogTypeTopup)
			assert.Error(t, err)
			cancelled, cancel := context.WithCancel(ctx)
			cancel()
			_, err = model.DeleteOldLogBatch(cancelled, cutoff, 1)
			assert.ErrorIs(t, err, context.Canceled)
			deleted, err := model.DeleteOldLogBatch(ctx, cutoff, 1, model.LogTypeConsume)
			require.NoError(t, err)
			if dialect == "clickhouse" {
				assert.Equal(t, int64(2), deleted)
			} else {
				assert.Equal(t, int64(1), deleted)
			}
			if dialect != "clickhouse" {
				remaining, err := model.CountOldLog(ctx, cutoff, model.LogTypeConsume)
				require.NoError(t, err)
				assert.Equal(t, int64(1), remaining, "relational batch deletes at most the requested limit")
			}
			for _, automatic := range []bool{true, false} {
				truncate(t)
				scope := ""
				if !automatic {
					scope = "error"
					require.NoError(t, db.Table("logs").Create(map[string]any{"id": 11, "type": model.LogTypeError, "created_at": cutoff - 1}).Error)
					require.NoError(t, db.Table("logs").Create(map[string]any{"id": 12, "type": model.LogTypeConsume, "created_at": cutoff - 1}).Error)
				}
				task, err := model.CreateSystemTask(model.SystemTaskTypeLogCleanup, ScopedLogCleanupPayload{
					LogCleanupPayload: LogCleanupPayload{TargetTimestamp: cutoff, BatchSize: 1}, Scope: scope, Automatic: automatic,
				}, nil)
				require.NoError(t, err)
				task, claimed, err := model.ClaimSystemTask(task.ID, task.Type, "cleanup-test", common.GetTimestamp()+60)
				require.NoError(t, err)
				require.True(t, claimed)
				scopedLogCleanupHandler{}.Run(ctx, task, "cleanup-test")
				reloaded, err := model.GetSystemTaskByTaskID(task.TaskID)
				require.NoError(t, err)
				assert.Equal(t, model.SystemTaskStatusSucceeded, reloaded.Status)
				var result LogCleanupResult
				require.NoError(t, common.UnmarshalJsonStr(reloaded.Result, &result))
				if automatic {
					expected := int64(2)
					if dialect == "clickhouse" {
						expected = 1 // existing ClickHouse mutation cleans its scope synchronously
					}
					assert.Equal(t, expected, result.DeletedCount)
				} else {
					assert.Equal(t, int64(1), result.DeletedCount)
				}
			}
			var ids []int
			require.NoError(t, db.Table("logs").Order("id").Pluck("id", &ids).Error)
			assert.Equal(t, []int{3, 4, 5, 6, 7, 8, 9, 12}, ids, "fund, audit, future types, retention boundary and unselected consumption survive")
		})
	}
}

func TestArrivalRebateRetrySchedulesDurablePendingOnce(t *testing.T) {
	truncate(t)
	require.NoError(t, model.DB.AutoMigrate(&model.ArrivalEvent{}))
	t.Cleanup(func() { model.DB.Exec("DELETE FROM arrival_events") })
	payment := operation_setting.GetPaymentSetting()
	before := *payment
	t.Cleanup(func() { *payment = before })
	payment.ComplianceConfirmed = true
	payment.ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion
	handler := arrivalRebateRetryHandler{}
	withSystemTaskRegistry(t, handler)
	assert.False(t, handler.Enabled(), "no pending business means no retry task")
	user := model.User{Username: "retry-recipient", AffCode: "retry-recipient", Quota: 10}
	require.NoError(t, model.DB.Create(&user).Error)
	event := model.ArrivalEvent{Source: "online", Reference: "retry-receipt", InviterId: user.Id, Reward: 25, Destination: "wallet", Status: "pending"}
	require.NoError(t, model.DB.Create(&event).Error)
	runSystemTaskScheduler()
	runSystemTaskScheduler()
	assert.EqualValues(t, 1, countSystemTasks(t, handler.Type()))
	task, err := model.GetActiveSystemTask(handler.Type())
	require.NoError(t, err)
	require.NotNil(t, task)
	claimed, ok, err := model.ClaimSystemTask(task.ID, handler.Type(), "retry-runner", common.GetTimestamp()+60)
	require.NoError(t, err)
	require.True(t, ok)
	handler.Run(context.Background(), claimed, "retry-runner")
	_, err = model.RetryPendingArrivalRebates(100)
	require.NoError(t, err)
	var recipient model.User
	require.NoError(t, model.DB.First(&recipient, user.Id).Error)
	assert.Equal(t, 35, recipient.Quota)
	assert.Equal(t, 25, recipient.AffHistoryQuota)
	assert.False(t, handler.Enabled())
	finished, err := model.GetLatestSystemTask(handler.Type())
	require.NoError(t, err)
	assert.Equal(t, model.SystemTaskStatusSucceeded, finished.Status)
}
