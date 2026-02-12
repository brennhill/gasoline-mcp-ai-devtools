// main_connection_mcp.go — MCP daemon mode: HTTP server startup, PID management, and signal handling.
package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/dev-console/dev-console/internal/capture"
	"github.com/dev-console/dev-console/internal/util"
)

// runMCPMode runs the server in MCP mode:
// - HTTP server runs in a goroutine (for browser extension)
// - MCP protocol runs over stdin/stdout (for Claude Code)
// If stdin closes (EOF), the HTTP server keeps running until killed.
// Returns error if port binding fails (race condition with another client).
// Never returns on success (blocks forever serving MCP protocol).
func runMCPMode(server *Server, port int, apiKey string) error {
	cap := initCapture(server, port)
	mux := setupHTTPRoutes(server, cap)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	startVersionCheckLoop(ctx)
	startScreenshotRateLimiterCleanup(ctx)

	if err := cleanupStalePIDFile(server, port); err != nil {
		return err
	}
	if err := preflightPortCheck(server, port); err != nil {
		return err
	}

	srv, err := startHTTPServer(server, port, apiKey, mux)
	if err != nil {
		return err
	}

	if err := writePIDFile(port); err != nil {
		server.logLifecycle("pid_file_error", port, map[string]any{"error": err.Error()})
	}

	server.logLifecycle("startup", port, map[string]any{
		"version":    version,
		"go_version": runtime.Version(),
		"os":         runtime.GOOS,
		"arch":       runtime.GOARCH,
	})
	server.logLifecycle("mcp_transport_ready", port, nil)

	awaitShutdownSignal(server, srv, port)
	return nil
}

// initCapture creates and configures the capture buffers with lifecycle logging.
func initCapture(server *Server, port int) *capture.Capture {
	cap := capture.NewCapture()
	cap.SetServerVersion(version)
	cap.SetLifecycleCallback(func(event string, data map[string]any) {
		entry := LogEntry{
			"type":      "lifecycle",
			"event":     event,
			"pid":       os.Getpid(),
			"port":      port,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}
		for k, v := range data {
			entry[k] = v
		}
		_ = server.appendToFile([]LogEntry{entry})
	})

	server.logLifecycle("loading_settings", port, nil)
	cap.LoadSettingsFromDisk()
	server.logLifecycle("settings_loaded", port, nil)
	return cap
}

// startScreenshotRateLimiterCleanup starts a background goroutine that removes
// stale entries from the screenshot rate limiter every 30 seconds.
func startScreenshotRateLimiterCleanup(ctx context.Context) {
	util.SafeGo(func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				screenshotRateMu.Lock()
				now := time.Now()
				for clientID, lastUpload := range screenshotRateLimiter {
					if now.Sub(lastUpload) > time.Minute {
						delete(screenshotRateLimiter, clientID)
					}
				}
				screenshotRateMu.Unlock()
			}
		}
	})
}

// cleanupStalePIDFile checks for an existing PID file and removes it if the
// process is dead. Returns an error if a live process already holds the port.
func cleanupStalePIDFile(server *Server, port int) error {
	pidFile := pidFilePath(port)
	if _, err := os.Stat(pidFile); err != nil {
		return nil // No PID file
	}

	pidBytes, err := os.ReadFile(pidFile)
	if err != nil {
		return nil
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(pidBytes)))
	if err != nil {
		return nil
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return nil
	}

	// Signal 0 checks existence without killing
	if process.Signal(syscall.Signal(0)) == nil {
		// Process is alive - real conflict
		server.logLifecycle("port_conflict_detected", port, map[string]any{"existing_pid": pid})
		return fmt.Errorf("port %d already in use by PID %d (run 'gasoline --stop --port %d' to stop it)", port, pid, port)
	}

	// Process is dead - remove stale PID file
	server.logLifecycle("stale_pid_removed", port, map[string]any{"stale_pid": pid})
	if err := os.Remove(pidFile); err != nil && !os.IsNotExist(err) {
		server.logLifecycle("stale_pid_remove_failed", port, map[string]any{
			"stale_pid": pid,
			"error":     err.Error(),
		})
	}
	return nil
}

// preflightPortCheck verifies the port is available before attempting to bind.
// Provides better error messages than a raw bind failure.
func preflightPortCheck(server *Server, port int) error {
	testAddr := fmt.Sprintf("127.0.0.1:%d", port)
	testLn, err := net.Listen("tcp", testAddr)
	if err != nil {
		server.logLifecycle("port_conflict_detected", port, map[string]any{"error": err.Error()})
		return fmt.Errorf("port %d already in use (unknown process, try '%s'): %w", port, portKillHintForce(port), err)
	}
	return testLn.Close()
}

// startHTTPServer launches the HTTP server in a background goroutine and waits
// for it to bind successfully. Returns the server instance for later shutdown.
func startHTTPServer(server *Server, port int, apiKey string, mux *http.ServeMux) (*http.Server, error) {
	httpReady := make(chan error, 1)
	srv := &http.Server{
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
		Handler:      AuthMiddleware(apiKey)(mux),
	}
	util.SafeGo(func() {
		addr := fmt.Sprintf("127.0.0.1:%d", port)
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			httpReady <- err
			return
		}
		httpReady <- nil
		// #nosec G114 -- localhost-only MCP background server
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "[gasoline] HTTP server error: %v\n", err)
		}
	})

	if err := <-httpReady; err != nil {
		server.logLifecycle("http_bind_failed", port, map[string]any{"error": err.Error()})
		return nil, fmt.Errorf("cannot bind port %d: %w", port, err)
	}

	server.logLifecycle("http_bind_success", port, nil)
	return srv, nil
}

// awaitShutdownSignal blocks until a termination signal is received, then
// performs a graceful shutdown of the HTTP server and cleanup.
func awaitShutdownSignal(server *Server, srv *http.Server, port int) {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	s := <-sig

	shutdownSource := mapSignalSource(s)
	server.logLifecycle("shutdown", port, map[string]any{
		"signal":          s.String(),
		"signal_num":      int(s.(syscall.Signal)),
		"shutdown_source": shutdownSource,
		"uptime_seconds":  time.Since(startTime).Seconds(),
	})

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		server.logLifecycle("http_shutdown_error", port, map[string]any{"error": err.Error()})
	}

	server.shutdownAsyncLogger(2 * time.Second)
	globalAnnotationStore.Close()
	removePIDFile(port)
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
