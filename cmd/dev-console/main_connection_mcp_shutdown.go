// Purpose: Shutdown and signal handling for MCP daemon runtime.
// Why: Isolates termination semantics and cleanup sequence from startup orchestration.

package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// awaitShutdownSignal blocks until a termination signal is received or the
// HTTP listener dies unexpectedly, then performs graceful cleanup.
// The httpDone channel closes if srv.Serve() exits for any reason other than
// a clean Shutdown — this prevents zombie daemons that are alive but deaf.
func awaitShutdownSignal(server *Server, srv *http.Server, port int, httpDone <-chan struct{}) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)

	var s os.Signal
	var shutdownSource string

	select {
	case s = <-sigCh:
		shutdownSource = mapSignalSource(s)
	case <-httpDone:
		// HTTP listener died unexpectedly — exit instead of hanging forever
		shutdownSource = "http_listener_died"
		s = syscall.SIGTERM // synthetic, for logging
		stderrf("[gasoline] HTTP listener exited unexpectedly, shutting down to avoid zombie process\n")
	}

	server.logLifecycle("shutdown", port, map[string]any{
		"signal":          s.String(),
		"shutdown_source": shutdownSource,
		"uptime_seconds":  time.Since(startTime).Seconds(),
	})
	if diagPath := appendExitDiagnostic("daemon_shutdown", map[string]any{
		"port":            port,
		"signal":          s.String(),
		"shutdown_source": shutdownSource,
		"uptime_seconds":  time.Since(startTime).Seconds(),
		"unexpected":      shutdownSource == "http_listener_died",
	}); diagPath != "" && shutdownSource == "http_listener_died" {
		stderrf("[gasoline] Shutdown diagnostics written to: %s\n", diagPath)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		server.logLifecycle("http_shutdown_error", port, map[string]any{"error": err.Error()})
	}

	server.shutdownAsyncLogger(2 * time.Second)
	server.closeAnnotationStore()
	removePIDFile(port)
	removeDaemonLockIfOwned(os.Getpid())
}

// mapSignalSource returns a human-readable description for a termination signal.
func mapSignalSource(s os.Signal) string {
	switch s {
	case os.Interrupt:
		return "Ctrl+C (SIGINT)"
	case syscall.SIGTERM:
		return "SIGTERM (likely --stop or kill)"
	case syscall.SIGHUP:
		return "SIGHUP (terminal closed)"
	default:
		return s.String()
	}
}
