// Package hotrestart implements an opt-in Linux supervisor with inherited sockets.
// A replacement serves only after readiness; background ownership moves only after
// the previous worker has stopped admitting jobs and finished every admitted job.
package hotrestart

import (
	"errors"
	"log"
	"os"
	"sync"
	"time"
)

const EnvWorker = "NEW_API_HOT_RESTART_WORKER"
const EnvListenAddr = "NEW_API_HOT_RESTART_ADDR"

type Options struct {
	Addr         string
	ReadyTimeout time.Duration
	Logger       *log.Logger
}

type RestartStatus struct {
	Supported  bool   `json:"supported"`
	Pending    bool   `json:"pending"`
	Draining   bool   `json:"draining"`
	Background bool   `json:"background"`
	LastError  string `json:"last_error"`
	PID        int    `json:"pid"`
	StartedAt  int64  `json:"started_at"`
}

var state = struct {
	sync.Mutex
	pending, draining, handoff bool
	lastError                  string
}{}
var startedAt = time.Now()

func IsWorker() bool { return Supported && os.Getenv(EnvWorker) == "1" }
func Status() RestartStatus {
	state.Lock()
	defer state.Unlock()
	return RestartStatus{Supported: IsWorker(), Pending: state.pending || state.handoff, Draining: state.draining,
		Background: Background.Enabled(), LastError: state.lastError, PID: os.Getpid(), StartedAt: startedAt.Unix()}
}
func reserveRestart() error {
	state.Lock()
	defer state.Unlock()
	if state.pending || state.draining || state.handoff {
		return errors.New("Hot restart is already in progress")
	}
	state.pending = true
	state.lastError = ""
	return nil
}
func markFailed(reason string) {
	state.Lock()
	defer state.Unlock()
	state.pending, state.lastError = false, reason
}

// WorkGroup fences admission and counts work before it is queued, not when its
// goroutine eventually runs. DisableAndWait is a one-way handoff barrier.
type WorkGroup struct {
	mu       sync.Mutex
	cond     *sync.Cond
	disabled bool
	retired  bool
	running  int
}

func (g *WorkGroup) Begin() (func(), bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.disabled {
		return nil, false
	}
	g.running++
	var once sync.Once
	return func() {
		once.Do(func() {
			g.mu.Lock()
			defer g.mu.Unlock()
			g.running--
			if g.running == 0 && g.cond != nil {
				g.cond.Broadcast()
			}
		})
	}, true
}
func (g *WorkGroup) Enabled() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return !g.disabled
}
func (g *WorkGroup) enable() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.retired {
		g.disabled = false
	}
}
func (g *WorkGroup) DisableAndWait() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.disabled = true
	g.retired = true
	if g.cond == nil {
		g.cond = sync.NewCond(&g.mu)
	}
	for g.running != 0 {
		g.cond.Wait()
	}
}

// Background is fenced by the supervisor control channel, across worker PIDs.
// Standalone processes retain their existing behavior. It is not a distributed
// election across independently launched masters; existing DB leases still apply.
var Background = WorkGroup{disabled: IsWorker()}

// Deferred tracks request-owned asynchronous writes/refunds through final drain.
var Deferred WorkGroup
