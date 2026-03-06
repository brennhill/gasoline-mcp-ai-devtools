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

const (
	// terminalShutdownTimeout is the deadline for the terminal server graceful shutdown.
	terminalShutdownTimeout = 2 * time.Second

	// httpShutdownTimeout is the deadline for the main HTTP server graceful shutdown.
	httpShutdownTimeout = 3 * time.Second

	// asyncLoggerDrainTimeout is the time allowed for the async logger to flush
	// remaining entries during shutdown.
	asyncLoggerDrainTimeout = 2 * time.Second
)

// awaitShutdownSignal blocks until a termination signal is received or the
// HTTP listener dies unexpectedly, then performs graceful cleanup.
// The httpDone channel closes if srv.Serve() exits for any reason other than
// a clean Shutdown — this prevents zombie daemons that are alive but deaf.
// termSrv and termDone are optional (nil if terminal server failed to start).
func awaitShutdownSignal(server *Server, srv *http.Server, port int, httpDone <-chan struct{}, termSrv *http.Server, termDone <-chan struct{}, mcpHandler *MCPHandler) {
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

	// Shut down terminal server first (if running) — non-blocking, best-effort.
	if termSrv != nil {
		termCtx, termCancel := context.WithTimeout(context.Background(), terminalShutdownTimeout)
		if err := termSrv.Shutdown(termCtx); err != nil {
			server.logLifecycle("terminal_shutdown_error", port, map[string]any{"error": err.Error()})
		}
		termCancel()
	}

	// Close the ToolHandler first to cancel in-flight readiness gates (requireExtension
	// blocking waits) so they unblock before the HTTP server shuts down.
	if mcpHandler != nil && mcpHandler.toolHandler != nil {
		if th, ok := mcpHandler.toolHandler.(*ToolHandler); ok {
			th.Close()
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), httpShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		server.logLifecycle("http_shutdown_error", port, map[string]any{"error": err.Error()})
	}

	server.shutdownAsyncLogger(asyncLoggerDrainTimeout)
	server.closeAnnotationStore()
	// Close capture store to stop background cleanup goroutines (QueryDispatcher).
	if mcpHandler != nil && mcpHandler.toolHandler != nil {
		if th, ok := mcpHandler.toolHandler.(*ToolHandler); ok && th.capture != nil {
			th.capture.Close()
		}
	}
	if server.ptyManager != nil {
		server.ptyManager.StopAll()
	}
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
