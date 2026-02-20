// main_connection_mcp.go — MCP daemon mode: HTTP server startup, PID management, and signal handling.
// Docs: docs/features/feature/observe/index.md
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
	"github.com/dev-console/dev-console/internal/state"
	"github.com/dev-console/dev-console/internal/util"
)

// binaryUpgradeState tracks whether a binary upgrade has been detected on disk.
// Read by maybeAddUpgradeWarning() in handler.go and buildUpgradeInfo() in health.go.
var binaryUpgradeState *BinaryWatcherState

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

	binaryUpgradeState = startBinaryWatcher(ctx, version,
		func(newVersion string) {
			server.logLifecycle("binary_upgrade_detected", port, map[string]any{
				"current_version": version,
				"new_version":     newVersion,
			})
			server.AddWarning("UPGRADE DETECTED: v" + newVersion + " installed. Auto-restart in ~5s.")
		},
		func() {
			if binaryUpgradeState != nil {
				if _, newVer, _ := binaryUpgradeState.UpgradeInfo(); newVer != "" {
					if markerPath, err := state.UpgradeMarkerFile(); err == nil {
						_ = writeUpgradeMarker(version, newVer, markerPath)
					}
				}
			}
			server.logLifecycle("binary_upgrade_shutdown", port, map[string]any{"version": version})
			p, _ := os.FindProcess(os.Getpid())
			_ = p.Signal(syscall.SIGTERM)
		},
	)

	if markerPath, err := state.UpgradeMarkerFile(); err == nil {
		if marker, err := readAndClearUpgradeMarker(markerPath); err == nil && marker != nil {
			server.AddWarning(fmt.Sprintf("Upgraded from v%s to v%s", marker.FromVersion, marker.ToVersion))
			server.logLifecycle("binary_upgrade_complete", port, map[string]any{
				"from_version": marker.FromVersion,
				"to_version":   marker.ToVersion,
			})
		}
	}

	if err := cleanupStalePIDFile(server, port); err != nil {
		return err
	}
	if err := preflightPortCheck(server, port); err != nil {
		return err
	}

	srv, httpDone, err := startHTTPServer(server, port, apiKey, mux)
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

	awaitShutdownSignal(server, srv, port, httpDone)
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

	// Signal 0 checks existence without killing.
	// A live PID alone is not enough: PID reuse can point to an unrelated process.
	// Only treat it as a conflict if that PID actually owns the target port.
	if process.Signal(syscall.Signal(0)) == nil {
		ownerPIDs, findErr := findProcessOnPort(port)
		if findErr == nil {
			for _, ownerPID := range ownerPIDs {
				if ownerPID == pid {
					server.logLifecycle("port_conflict_detected", port, map[string]any{"existing_pid": pid})
					return fmt.Errorf("port %d already in use by PID %d (run 'gasoline --stop --port %d' to stop it)", port, pid, port)
				}
			}
			server.logLifecycle("stale_pid_owner_mismatch", port, map[string]any{
				"stale_pid":  pid,
				"owner_pids": ownerPIDs,
			})
		} else {
			// If process inspection fails, prefer removing stale metadata and allowing
			// the later preflight bind check to authoritatively detect real conflicts.
			server.logLifecycle("stale_pid_port_lookup_failed", port, map[string]any{
				"stale_pid": pid,
				"error":     findErr.Error(),
			})
		}

		if err := os.Remove(pidFile); err != nil && !os.IsNotExist(err) {
			server.logLifecycle("stale_pid_remove_failed", port, map[string]any{
				"stale_pid": pid,
				"error":     err.Error(),
			})
		}
		return nil
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
// for it to bind successfully. Returns the server instance and a channel that
// closes if the listener exits unexpectedly (crash, network error, etc.).
func startHTTPServer(server *Server, port int, apiKey string, mux *http.ServeMux) (*http.Server, <-chan struct{}, error) {
	httpReady := make(chan error, 1)
	httpDone := make(chan struct{})
	srv := &http.Server{
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 65 * time.Second, // Must accommodate blocking tool waits (screenshot 20s, interact 35s, annotations 55s)
		IdleTimeout:  120 * time.Second,
		Handler:      AuthMiddleware(apiKey)(mux),
	}
	util.SafeGo(func() {
		defer close(httpDone)
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
		return nil, nil, fmt.Errorf("cannot bind port %d: %w", port, err)
	}

	server.logLifecycle("http_bind_success", port, nil)
	return srv, httpDone, nil
}

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
		fmt.Fprintf(os.Stderr, "[gasoline] HTTP listener exited unexpectedly, shutting down to avoid zombie process\n")
	}

	server.logLifecycle("shutdown", port, map[string]any{
		"signal":          s.String(),
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
