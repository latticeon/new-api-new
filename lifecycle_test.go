package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/hotrestart"
	"github.com/QuantumNous/new-api/service"
	"github.com/glebarez/sqlite"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestHotRestartWorkBarrier(t *testing.T) {
	var jobs hotrestart.WorkGroup
	done, ok := jobs.Begin()
	require.True(t, ok)
	drained := make(chan struct{})
	go func() { jobs.DisableAndWait(); close(drained) }()
	require.Eventually(t, func() bool { return !jobs.Enabled() }, time.Second, time.Millisecond)
	_, ok = jobs.Begin()
	assert.False(t, ok, "new jobs cannot enter a draining generation")
	select {
	case <-drained:
		t.Fatal("drained before the admitted job completed")
	default:
	}
	done()
	done() // cleanup is idempotent
	select {
	case <-drained:
	case <-time.After(time.Second):
		t.Fatal("drain did not complete")
	}
}

func TestHotRestartPlatform(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skip("standalone Linux status exercised by integration fixture")
	}
	assert.False(t, hotrestart.Supported)
	assert.False(t, hotrestart.Status().Supported)
	require.Error(t, hotrestart.RequestRestart())
}

// Re-executes the test binary, so the Linux tests exercise actual inheritance,
// process startup/exit, control socket acknowledgements and the production drain.
func TestHotRestartProcess(t *testing.T) {
	if os.Getenv("HOT_RESTART_TEST_HELPER") != "1" {
		return
	}
	addr := os.Getenv("HOT_RESTART_TEST_ADDR")
	if !hotrestart.IsWorker() {
		os.Exit(hotrestart.Supervise(hotrestart.Options{Addr: addr, ReadyTimeout: 10 * time.Second}))
	}
	hotrestart.PrepareWorker()
	mode, _ := os.ReadFile(os.Getenv("HOT_RESTART_TEST_MODE"))
	switch string(mode) {
	case "fail":
		os.Exit(42)
	case "timeout":
		select {}
	}
	common.DataExportEnabled = false
	common.RedisEnabled = false
	common.LogConsumeEnabled = true
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	db := hotRestartBusinessDB(t, os.Getenv("HOT_RESTART_TEST_DB"))
	model.DB, model.LOG_DB = db, db
	// Both generations capture the same in-progress task before the old job
	// finishes. The replacement must not settle this stale snapshot again.
	var pending model.Task
	require.NoError(t, db.First(&pending).Error)
	releaseStream, releaseJob := make(chan struct{}), make(chan struct{})
	var streamOnce, jobOnce sync.Once
	mux := http.NewServeMux()
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		body, err := common.Marshal(hotrestart.Status())
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		_, _ = w.Write(body)
	})
	mux.HandleFunc("/restart", func(w http.ResponseWriter, r *http.Request) {
		if err := hotrestart.RequestRestart(); err != nil {
			http.Error(w, err.Error(), 409)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	})
	mux.HandleFunc("/settle", func(w http.ResponseWriter, r *http.Request) {
		done, ok := hotrestart.Background.Begin()
		if !ok {
			fmt.Fprint(w, "not leader")
			return
		}
		defer done()
		fmt.Fprint(w, hotRestartRefund(t, pending))
	})
	mux.HandleFunc("/sse", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: first\n\n")
		w.(http.Flusher).Flush()
		<-releaseStream
		fmt.Fprint(w, "data: last\n\n")
	})
	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		// The test starts this before replacement, releasing it on the old WS.
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, "first\n")
		w.(http.Flusher).Flush()
		<-releaseStream
		fmt.Fprint(w, "last\n")
	})
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				return
			}
			switch string(message) {
			case "job":
				done, ok := hotrestart.Background.Begin()
				if !ok {
					message = []byte("not leader")
					break
				}
				go func() {
					defer done()
					<-releaseJob
					require.Equal(t, "settled", hotRestartRefund(t, pending))
				}()
			case "release-job":
				jobOnce.Do(func() { close(releaseJob) })
			case "release-stream":
				streamOnce.Do(func() { close(releaseStream) })
			}
			if err := conn.WriteMessage(websocket.TextMessage, append([]byte(strconv.Itoa(os.Getpid())+":"), message...)); err != nil {
				return
			}
		}
	})
	if err := serveHTTP(mux, addr, time.Now(), "127.0.0.1", ""); err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}

// These fixtures use the real task CAS and refund accounting, not an ownership
// marker: durable balances, task quota and refund logs are checked by the parent.
func hotRestartBusinessDB(t *testing.T, path string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(path+"?_pragma=busy_timeout(5000)"), &gorm.Config{})
	require.NoError(t, err)
	pool, err := db.DB()
	require.NoError(t, err)
	pool.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, pool.Close()) })
	return db
}

func hotRestartRefund(t *testing.T, task model.Task) string {
	t.Helper()
	task.Status = model.TaskStatusFailure
	task.FailReason = "hot restart integration failure"
	won, err := task.UpdateWithStatus(model.TaskStatusInProgress)
	require.NoError(t, err)
	if !won {
		return "stale"
	}
	require.True(t, service.RefundTaskQuota(context.Background(), &task, task.FailReason))
	return "settled"
}

func TestLinuxHotRestartHandoffAndRollback(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("requires real Linux; cross-compilation does not validate socket inheritance")
	}
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := probe.Addr().String()
	require.NoError(t, probe.Close())
	modeFile := filepath.Join(t.TempDir(), "candidate-mode")
	require.NoError(t, os.WriteFile(modeFile, nil, 0600))
	dbPath := filepath.Join(t.TempDir(), "business.db")
	db := hotRestartBusinessDB(t, dbPath)
	require.NoError(t, db.AutoMigrate(&model.Task{}, &model.User{}, &model.Token{}, &model.Channel{}, &model.Log{}))
	user := model.User{Username: "hotrestart", Quota: 1000, UsedQuota: 100, RequestCount: 1}
	require.NoError(t, db.Create(&user).Error)
	token := model.Token{UserId: user.Id, Key: "hotrestart-test", RemainQuota: 500, UsedQuota: 100}
	require.NoError(t, db.Create(&token).Error)
	channel := model.Channel{UsedQuota: 100}
	require.NoError(t, db.Create(&channel).Error)
	task := model.Task{TaskID: "hotrestart-refund", UserId: user.Id, ChannelId: channel.Id,
		Status: model.TaskStatusInProgress, Quota: 100, Group: "default",
		PrivateData: model.TaskPrivateData{TokenId: token.Id, BillingSource: service.BillingSourceWallet}}
	require.NoError(t, db.Create(&task).Error)
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestHotRestartProcess$")
	cmd.Env = append(os.Environ(), "HOT_RESTART_TEST_HELPER=1", "HOT_RESTART_TEST_ADDR="+addr, "HOT_RESTART_TEST_MODE="+modeFile, "HOT_RESTART_TEST_DB="+dbPath)
	output, err := os.Create(filepath.Join(t.TempDir(), "supervisor.log"))
	require.NoError(t, err)
	defer output.Close()
	cmd.Stdout, cmd.Stderr = output, output
	require.NoError(t, cmd.Start())
	t.Cleanup(func() {
		defer cancel()
		_ = cmd.Process.Signal(os.Interrupt)
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		select {
		case err := <-done:
			assert.NoError(t, err, "supervisor must reap workers after rollback and shutdown")
		case <-time.After(5 * time.Second):
			t.Error("supervisor failed to drain and reap workers")
			_ = cmd.Process.Signal(os.Interrupt) // explicit force-stop, not seamless
			cancel()
			<-done
		}
		if t.Failed() {
			log, _ := os.ReadFile(output.Name())
			t.Logf("supervisor output:\n%s", log)
		}
	})
	client := &http.Client{Timeout: 2 * time.Second, Transport: &http.Transport{DisableKeepAlives: true}}
	defer client.CloseIdleConnections()
	readStatus := func() (hotrestart.RestartStatus, error) {
		response, err := client.Get("http://" + addr + "/status")
		if err != nil {
			return hotrestart.RestartStatus{}, err
		}
		defer response.Body.Close()
		var status hotrestart.RestartStatus
		err = common.DecodeJson(response.Body, &status)
		return status, err
	}
	var initial hotrestart.RestartStatus
	require.Eventually(t, func() bool {
		initial, err = readStatus()
		return err == nil && initial.Background && !initial.Pending
	}, 30*time.Second, 10*time.Millisecond)
	assert.True(t, initial.Supported)
	ws, _, err := websocket.DefaultDialer.Dial("ws://"+addr+"/ws", nil)
	require.NoError(t, err)
	defer ws.Close()
	send := func(message string) string {
		require.NoError(t, ws.SetReadDeadline(time.Now().Add(3*time.Second)))
		require.NoError(t, ws.WriteMessage(websocket.TextMessage, []byte(message)))
		_, body, err := ws.ReadMessage()
		require.NoError(t, err)
		return string(body)
	}
	assert.Equal(t, fmt.Sprintf("%d:job", initial.PID), send("job"))
	streamClient := &http.Client{Timeout: 20 * time.Second}
	sse, err := streamClient.Get("http://" + addr + "/sse")
	require.NoError(t, err)
	defer sse.Body.Close()
	sseReader := bufio.NewReader(sse.Body)
	line, err := sseReader.ReadString('\n')
	require.NoError(t, err)
	assert.Equal(t, "data: first\n", line)
	slow, err := streamClient.Get("http://" + addr + "/slow")
	require.NoError(t, err)
	defer slow.Body.Close()
	slowReader := bufio.NewReader(slow.Body)
	line, err = slowReader.ReadString('\n')
	require.NoError(t, err)
	assert.Equal(t, "first\n", line)
	restart := func() {
		resp, err := client.Post("http://"+addr+"/restart", "application/json", nil)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusAccepted, resp.StatusCode)
	}
	restart()
	var replacement hotrestart.RestartStatus
	require.Eventually(t, func() bool {
		replacement, err = readStatus()
		return err == nil && replacement.PID != initial.PID
	}, 10*time.Second, 10*time.Millisecond)
	assert.False(t, replacement.Background, "old admitted job still owns background work")
	assert.Equal(t, fmt.Sprintf("%d:echo", initial.PID), send("echo"), "hijacked WS stays on old PID")
	settle := func(want string) {
		resp, e := client.Post("http://"+addr+"/settle", "application/json", nil)
		require.NoError(t, e)
		defer resp.Body.Close()
		body, e := io.ReadAll(resp.Body)
		require.NoError(t, e)
		require.Equal(t, want, string(body))
	}
	settle("not leader")
	// Repeated trigger cannot create an unbounded chain while an old WS drains.
	assert.True(t, replacement.Pending)
	response, err := client.Post("http://"+addr+"/restart", "application/json", nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusConflict, response.StatusCode)
	require.NoError(t, response.Body.Close())
	assert.Equal(t, fmt.Sprintf("%d:not leader", initial.PID), send("job"))
	send("release-job")
	require.Eventually(t, func() bool {
		status, e := readStatus()
		return e == nil && status.Background && status.Pending
	}, 3*time.Second, 10*time.Millisecond)
	settle("stale")
	settle("stale") // repeated poll must not duplicate the old worker's refund
	require.NoError(t, db.First(&user, user.Id).Error)
	assert.Equal(t, 1100, user.Quota)
	assert.Zero(t, user.UsedQuota)
	assert.Equal(t, 1, user.RequestCount)
	require.NoError(t, db.First(&token, token.Id).Error)
	assert.Equal(t, 600, token.RemainQuota)
	assert.Zero(t, token.UsedQuota)
	require.NoError(t, db.First(&channel, channel.Id).Error)
	assert.Zero(t, channel.UsedQuota)
	require.NoError(t, db.First(&task, task.ID).Error)
	assert.EqualValues(t, model.TaskStatusFailure, task.Status)
	assert.Zero(t, task.Quota)
	var logs []model.Log
	require.NoError(t, db.Find(&logs).Error)
	require.Len(t, logs, 1)
	assert.Equal(t, model.LogTypeRefund, logs[0].Type)
	assert.Equal(t, 100, logs[0].Quota)
	t.Logf("business handoff old=%d new=%d: one CAS refund, wallet=1100 token=600 usage=0 requests=1 logs=1", initial.PID, replacement.PID)
	send("release-stream")
	remainder, err := io.ReadAll(sseReader)
	require.NoError(t, err)
	assert.Equal(t, "\ndata: last\n\n", string(remainder))
	remainder, err = io.ReadAll(slowReader)
	require.NoError(t, err)
	assert.Equal(t, "last\n", string(remainder))
	assert.Equal(t, fmt.Sprintf("%d:still-alive", initial.PID), send("still-alive"))
	require.NoError(t, ws.Close())
	// Wait until the old worker has exited: another restart is now admitted.
	require.Eventually(t, func() bool {
		status, e := readStatus()
		return e == nil && status.PID == replacement.PID && !status.Pending
	}, 5*time.Second, 10*time.Millisecond)
	for _, mode := range []string{"fail", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			require.NoError(t, os.WriteFile(modeFile, []byte(mode), 0600))
			wantError := "Replacement exited before readiness"
			if mode == "timeout" {
				wantError = "Replacement readiness timed out"
			}
			if mode == "fail" {
				// Supervisor-triggered restarts use the same rollback/status path
				// as API requests (SIGUSR2 is 12 on the Linux test platform).
				require.NoError(t, cmd.Process.Signal(syscall.Signal(12)))
			} else {
				restart()
			}
			require.Eventually(t, func() bool {
				status, e := readStatus()
				return e == nil && !status.Pending && strings.Contains(status.LastError, wantError)
			}, 20*time.Second, 10*time.Millisecond)
			status, e := readStatus()
			require.NoError(t, e)
			assert.Equal(t, replacement.PID, status.PID, "failed replacement must retain old server")
			assert.True(t, status.Background)
		})
	}
}
