//go:build linux

package hotrestart

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"
)

const Supported = true
const listenerFD, controlFD = 3, 4

var prepareOnce sync.Once
var control net.Conn
var controlErr error
var controlMu sync.Mutex
var parentGone = make(chan struct{})
var drain = make(chan struct{})
var drainOnce sync.Once
var quiesceOnce sync.Once
var serving = make(chan struct{})
var servingOnce sync.Once

func PrepareWorker() {
	if !IsWorker() {
		return
	}
	prepareOnce.Do(func() {
		syscall.CloseOnExec(listenerFD)
		syscall.CloseOnExec(controlFD)
		f := os.NewFile(controlFD, "hot-restart-control")
		control, controlErr = net.FileConn(f)
		_ = f.Close()
		if controlErr != nil {
			close(parentGone)
			return
		}
		go func() {
			defer close(parentGone)
			scanner := bufio.NewScanner(control)
			for scanner.Scan() {
				line := scanner.Text()
				switch {
				case line == "serve":
					servingOnce.Do(func() { close(serving) })
				case line == "pending":
					state.Lock()
					state.pending = true
					state.lastError = ""
					state.Unlock()
				case line == "lead":
					Background.enable()
				case line == "idle":
					state.Lock()
					state.handoff = false
					state.Unlock()
				case line == "drain":
					state.Lock()
					state.draining = true
					state.Unlock()
					drainOnce.Do(func() { close(drain) })
					go Quiesce()
				case strings.HasPrefix(line, "failed "):
					markFailed(strings.TrimPrefix(line, "failed "))
				}
			}
		}()
	})
}

func sendControl(line string) error {
	controlMu.Lock()
	defer controlMu.Unlock()
	if control == nil {
		return errors.New("Hot restart control channel unavailable")
	}
	_ = control.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_, err := io.WriteString(control, line+"\n")
	return err
}
func RequestRestart() error {
	if !IsWorker() {
		return errors.New("Hot restart requires Linux and HOT_RESTART_ENABLED=true")
	}
	PrepareWorker()
	select {
	case <-parentGone:
		return errors.New("Hot restart supervisor unavailable")
	default:
	}
	if err := reserveRestart(); err != nil {
		return err
	}
	if err := sendControl("restart"); err != nil {
		markFailed(err.Error())
		return err
	}
	return nil
}
func Listen(addr string) (net.Listener, error) {
	if !IsWorker() {
		return net.Listen("tcp", addr)
	}
	PrepareWorker()
	if expected := os.Getenv(EnvListenAddr); expected != "" && addr != expected {
		return nil, fmt.Errorf("listener address changed from %q to %q; stop and start the supervisor to change HOST/PORT", expected, addr)
	}
	if controlErr != nil {
		return nil, controlErr
	}
	f := os.NewFile(listenerFD, "hot-restart-listener")
	ln, err := net.FileListener(f)
	_ = f.Close()
	if err != nil {
		return nil, fmt.Errorf("inherited listener: %w", err)
	}
	return ln, nil
}
func NotifyReady() error {
	if !IsWorker() {
		return nil
	}
	state.Lock()
	state.handoff = true
	state.Unlock()
	if err := sendControl("ready"); err != nil {
		return err
	}
	select {
	case <-serving:
		return nil
	case <-parentGone:
		return errors.New("Hot restart supervisor unavailable")
	case <-drain:
		return errors.New("Worker drained before serving")
	}
}
func ParentGone() <-chan struct{} {
	if !IsWorker() {
		return nil
	}
	return parentGone
}
func Drain() <-chan struct{} {
	if !IsWorker() {
		return nil
	}
	return drain
}

// NotifyAcceptStopped acknowledges that Shutdown has closed the old accept
// loop. The candidate does not accept until this cross-process barrier.
func NotifyAcceptStopped() {
	if IsWorker() {
		_ = sendControl("accept-stopped")
	}
}

// Quiesce is also called before process exit, so the acknowledgement cannot
// be lost merely because the last HTTP request finishes before the control goroutine.
func Quiesce() {
	quiesceOnce.Do(func() {
		Background.DisableAndWait()
		select {
		case <-drain:
			_ = sendControl("quiesced")
		default:
		}
	})
}

// StopAccepting also releases a candidate waiting for serving permission when
// the supervisor or operator terminates it during the readiness handoff.
func StopAccepting() { drainOnce.Do(func() { close(drain) }) }
