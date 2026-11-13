//go:build linux

package hotrestart

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

type workerProc struct {
	cmd       *exec.Cmd
	ctl       net.Conn
	ready     bool
	abandoned bool
	quiesced  bool
}
type workerEvent struct {
	worker *workerProc
	line   string
	exited bool
	err    error
}
type supervisor struct {
	opts                       Options
	log                        *log.Logger
	executable                 string
	socket                     *os.File
	workers                    map[*workerProc]bool
	current, pending, retiring *workerProc
	timer                      *time.Timer
	timeout                    <-chan time.Time
	events                     chan workerEvent
	done                       chan struct{}
	stopping                   bool
	exitCode                   int
}

// Supervise holds the listening socket while at most one candidate and one
// draining worker exist. No worker is retired before the candidate's handshake.
func Supervise(opts Options) int {
	if opts.ReadyTimeout <= 0 {
		opts.ReadyTimeout = 300 * time.Second
	}
	if opts.Logger == nil {
		opts.Logger = log.New(os.Stderr, "[hot-restart] ", log.LstdFlags)
	}
	exe, err := resolveExecutable()
	if err != nil {
		opts.Logger.Print(err)
		return 1
	}
	ln, err := net.Listen("tcp", opts.Addr)
	if err != nil {
		opts.Logger.Print(err)
		return 1
	}
	socket, err := ln.(*net.TCPListener).File()
	_ = ln.Close()
	if err != nil {
		opts.Logger.Print(err)
		return 1
	}
	// ExtraFiles calls File.Fd on every spawn. A pollable os.File switches its
	// shared open-file description to blocking mode there, which can strand the
	// current worker's Accept if a candidate fails before restoring nonblocking
	// mode. Retain a deliberately non-pollable wrapper instead; workers configure
	// their own net.Listener without later spawns resetting the shared flags.
	fd, err := syscall.Dup(int(socket.Fd()))
	_ = socket.Close()
	if err != nil {
		opts.Logger.Print(err)
		return 1
	}
	syscall.CloseOnExec(fd)
	socket = os.NewFile(uintptr(fd), "hot-restart-listener")
	defer socket.Close()
	s := &supervisor{opts: opts, log: opts.Logger, executable: exe, socket: socket,
		workers: make(map[*workerProc]bool), events: make(chan workerEvent), done: make(chan struct{})}
	defer close(s.done)
	signals := make(chan os.Signal, 4)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR2)
	defer signal.Stop(signals)
	s.startWorker()
	for len(s.workers) > 0 {
		select {
		case sig := <-signals:
			if sig == syscall.SIGUSR2 {
				s.restart(nil)
			} else if s.stopping {
				for w := range s.workers {
					_ = w.cmd.Process.Kill()
				}
			} else {
				s.stop(0)
			}
		case <-s.timeout:
			s.abandonPending("Replacement readiness timed out")
		case ev := <-s.events:
			if ev.exited {
				s.onExit(ev.worker, ev.err)
			} else {
				s.onMessage(ev.worker, ev.line)
			}
		}
	}
	return s.exitCode
}
func resolveExecutable() (string, error) {
	if strings.Contains(os.Args[0], "/") {
		return filepath.Abs(os.Args[0])
	}
	p, err := exec.LookPath(os.Args[0])
	if err != nil {
		return "", err
	}
	return filepath.Abs(p)
}
func (s *supervisor) send(w *workerProc, line string) error {
	_ = w.ctl.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_, err := io.WriteString(w.ctl, line+"\n")
	return err
}
func (s *supervisor) failure(w *workerProc, reason string) {
	s.log.Print(reason)
	if w != nil {
		_ = s.send(w, "failed "+strings.ReplaceAll(reason, "\n", " "))
	}
}
func (s *supervisor) restart(requester *workerProc) {
	if requester != nil && requester != s.current {
		s.failure(requester, "Only the current worker can restart")
		return
	}
	if s.stopping || s.pending != nil || s.retiring != nil || len(s.workers) > 1 {
		s.failure(requester, "Hot restart is already in progress")
		return
	}
	s.startWorker()
}
func (s *supervisor) startWorker() {
	w, err := s.spawn()
	if err != nil {
		s.failure(s.current, "Failed to start replacement: "+err.Error())
		if s.current == nil {
			s.stop(1)
		}
		return
	}
	s.pending = w
	if s.current != nil {
		if err := s.send(s.current, "pending"); err != nil {
			s.stop(1)
			return
		}
	}
	s.timer = time.NewTimer(s.opts.ReadyTimeout)
	s.timeout = s.timer.C
}
func (s *supervisor) spawn() (*workerProc, error) {
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM|syscall.SOCK_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	parent := os.NewFile(uintptr(fds[0]), "hot-restart-parent")
	child := os.NewFile(uintptr(fds[1]), "hot-restart-child")
	defer child.Close()
	ctl, err := net.FileConn(parent)
	_ = parent.Close()
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(s.executable, os.Args[1:]...)
	cmd.Args[0] = os.Args[0]
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	cmd.ExtraFiles = []*os.File{s.socket, child}
	// Replace reserved environment entries, rather than inheriting stale values.
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, EnvWorker+"=") && !strings.HasPrefix(entry, EnvListenAddr+"=") {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	cmd.Env = append(cmd.Env, EnvWorker+"=1", EnvListenAddr+"="+s.opts.Addr)
	if err := cmd.Start(); err != nil {
		ctl.Close()
		return nil, err
	}
	w := &workerProc{cmd: cmd, ctl: ctl}
	s.workers[w] = true
	s.log.Printf("worker pid=%d starting", cmd.Process.Pid)
	go s.watch(w)
	return w, nil
}
func (s *supervisor) emit(ev workerEvent) {
	select {
	case s.events <- ev:
	case <-s.done:
	}
}
func (s *supervisor) watch(w *workerProc) {
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		scanner := bufio.NewScanner(w.ctl)
		for scanner.Scan() {
			s.emit(workerEvent{worker: w, line: scanner.Text()})
		}
	}()
	err := w.cmd.Wait()
	<-readDone // Drain all control messages (including quiesced) before the exit event.
	_ = w.ctl.Close()
	s.emit(workerEvent{worker: w, exited: true, err: err})
}
func (s *supervisor) onMessage(w *workerProc, line string) {
	if !s.workers[w] || w.abandoned || s.stopping {
		return
	}
	switch line {
	case "restart":
		s.restart(w)
	case "ready":
		if s.pending != w || w.ready {
			return
		}
		w.ready = true
		s.pending = nil
		s.stopTimer()
		old := s.current
		s.current = w
		if old == nil {
			if err := s.send(w, "serve"); err != nil {
				s.stop(1)
				return
			}
			if err := s.send(w, "lead"); err != nil {
				s.stop(1)
				return
			}
			if err := s.send(w, "idle"); err != nil {
				s.stop(1)
			}
			return
		}
		s.retiring = old
		// Accepting waits for the old accept loop to stop. Background ownership
		// independently waits for its admitted business jobs to finish.
		if err := s.send(old, "drain"); err != nil {
			s.stop(1)
		}
	case "accept-stopped":
		if s.retiring == w {
			if err := s.send(s.current, "serve"); err != nil {
				s.stop(1)
			}
		}
	case "quiesced":
		if s.retiring != w || w.quiesced {
			return
		}
		w.quiesced = true
		if err := s.send(s.current, "lead"); err != nil {
			s.stop(1)
		}
	}
}
func (s *supervisor) onExit(w *workerProc, err error) {
	delete(s.workers, w)
	s.log.Printf("worker pid=%d exited: %v", w.cmd.Process.Pid, err)
	if s.stopping {
		return
	}
	switch w {
	case s.pending:
		s.pending = nil
		s.stopTimer()
		if s.current == nil {
			s.stop(1)
		} else {
			s.failure(s.current, "Replacement exited before readiness; previous worker retained")
		}
	case s.current:
		s.current = nil
		s.stop(1)
	case s.retiring:
		// Normal drain cannot exit until Background.DisableAndWait completes.
		// An abnormal exit is not an acknowledged handoff: fail closed.
		s.retiring = nil
		if err != nil || !w.quiesced {
			s.stop(1)
		} else if err := s.send(s.current, "idle"); err != nil {
			s.stop(1)
		}
	}
}
func (s *supervisor) abandonPending(reason string) {
	s.stopTimer()
	if s.pending == nil {
		return
	}
	w := s.pending
	s.pending = nil
	w.abandoned = true
	_ = w.cmd.Process.Kill()
	s.failure(s.current, reason)
	if s.current == nil {
		s.stop(1)
	}
}
func (s *supervisor) stopTimer() {
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
	s.timeout = nil
}
func (s *supervisor) stop(code int) {
	if s.stopping {
		return
	}
	s.stopping, s.exitCode = true, code
	s.stopTimer()
	_ = s.socket.Close()
	for w := range s.workers {
		if !w.ready {
			_ = w.cmd.Process.Kill()
		} else {
			_ = w.cmd.Process.Signal(syscall.SIGTERM)
		}
	}
}

// ReadyListener announces readiness only when http.Server has installed its
// accept loop, not after an arbitrary startup sleep.
type ReadyListener struct {
	net.Listener
	once bool
}

func (l *ReadyListener) Accept() (net.Conn, error) {
	if !l.once {
		l.once = true
		if err := NotifyReady(); err != nil {
			return nil, fmt.Errorf("notify readiness: %w", err)
		}
	}
	return l.Listener.Accept()
}
