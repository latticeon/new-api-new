package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/hotrestart"
	"github.com/joho/godotenv"
)

// The supervisor does not load .env into its own environment: each candidate
// reloads it. Opt-in preserves standalone deployments and process managers.
func runSupervisorIfEnabled() {
	if !hotrestart.Supported || hotrestart.IsWorker() {
		return
	}
	dotenv, _ := godotenv.Read(".env")
	lookup := func(key string) string {
		if value, ok := os.LookupEnv(key); ok {
			return value
		}
		return dotenv[key]
	}
	if !strings.EqualFold(strings.TrimSpace(lookup("HOT_RESTART_ENABLED")), "true") {
		return
	}
	flag.Parse()
	if *common.PrintVersion || *common.PrintHelp {
		return
	}
	// Random process-local defaults must not change encryption/session keys on
	// handoff. Require the existing deployment secret, never manufacture one.
	if secret := strings.TrimSpace(lookup("SESSION_SECRET")); secret == "" || secret == "random_string" {
		fmt.Fprintln(os.Stderr, "HOT_RESTART_ENABLED requires a stable SESSION_SECRET")
		os.Exit(1)
	}
	port := lookup("PORT")
	if port == "" {
		port = strconv.Itoa(*common.Port)
	}
	timeout := 300
	if value := lookup("HOT_RESTART_READY_TIMEOUT"); value != "" {
		seconds, err := strconv.Atoi(value)
		if err != nil || seconds < 1 || seconds > 3600 {
			fmt.Fprintln(os.Stderr, "HOT_RESTART_READY_TIMEOUT must be between 1 and 3600 seconds")
			os.Exit(1)
		}
		timeout = seconds
	}
	os.Exit(hotrestart.Supervise(hotrestart.Options{Addr: net.JoinHostPort(lookup("HOST"), port), ReadyTimeout: time.Duration(timeout) * time.Second}))
}

// trackRequests encloses the entire HTTP handler, including hijacked WebSocket
// handlers: Shutdown alone does not wait for hijacked connections.
var requests hotrestart.WorkGroup

func trackRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		done, ok := requests.Begin()
		if !ok {
			w.Header().Set("Connection", "close")
			http.Error(w, "Server is draining", http.StatusServiceUnavailable)
			return
		}
		defer done()
		next.ServeHTTP(w, r)
	})
}

func serveHTTP(handler http.Handler, addr string, started time.Time, host, port string) error {
	ln, err := hotrestart.Listen(addr)
	if err != nil {
		return err
	}
	defer ln.Close()
	srv := &http.Server{Addr: addr, Handler: trackRequests(handler)}
	signals := make(chan os.Signal, 2)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	serveErr := make(chan error, 1)
	go func() {
		result := srv.Serve(&hotrestart.ReadyListener{Listener: ln})
		hotrestart.NotifyAcceptStopped()
		serveErr <- result
	}()
	common.LogStartupSuccess(started, host, port)
	var serveResult error
	serveFinished := false
	select {
	case serveResult = <-serveErr:
		serveFinished = true
	case <-signals:
	case <-hotrestart.Drain():
	case <-hotrestart.ParentGone():
	}
	hotrestart.StopAccepting()
	// Hot restarts intentionally have no drain deadline: SSE and WebSocket
	// sessions can outlive SHUTDOWN_TIMEOUT_SECONDS. Explicit second termination
	// remains the operator's force-stop mechanism, not a seamless-restart promise.
	hot := hotrestart.IsWorker()
	ctx := context.Background()
	var cancel context.CancelFunc
	if !hot {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(common.GetEnvOrDefault("SHUTDOWN_TIMEOUT_SECONDS", 120))*time.Second)
		defer cancel()
	}
	stopped := make(chan error, 1)
	go func() {
		// Shutdown closes the accept loop immediately; background draining runs in
		// parallel so a slow periodic job cannot leave the old listener accepting.
		stopped <- srv.Shutdown(ctx)
	}()
	hotrestart.Quiesce()
	err = <-stopped
	if err != nil {
		return err
	}
	requests.DisableAndWait()
	hotrestart.Deferred.DisableAndWait()
	model.FlushBatchUpdates()
	if common.DataExportEnabled {
		model.SaveQuotaDataCache()
	}
	if !serveFinished {
		serveResult = <-serveErr
	}
	if serveResult != nil && !errors.Is(serveResult, http.ErrServerClosed) {
		return serveResult
	}
	common.SysLog(common.LogText("server exited"))
	return nil
}
