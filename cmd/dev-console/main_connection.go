// main_connection.go — MCP connection lifecycle, diagnostics, and server management.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/dev-console/dev-console/internal/capture"
	"github.com/dev-console/dev-console/internal/util"
)

// handleMCPConnection implements the enhanced connection lifecycle with retry and auto-recovery.
// Lifecycle:
//  1. Check if server is running on port
//  2. If not running: spawn new server
//  3. If running: connect as client
//  4. If connection fails: retry once
//  5. If still fails: kill existing server and spawn new one
//  6. If final attempt fails: write debug file and exit
func handleMCPConnection(server *Server, port int, apiKey string) {
	serverURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	mcpEndpoint := serverURL + "/mcp"
	debugFile := ""

	// Helper to write debug info
	writeDebugInfo := func(phase string, err error, details map[string]interface{}) {
		if debugFile == "" {
			timestamp := time.Now().Format("20060102-150405")
			debugFile = filepath.Join(os.TempDir(), fmt.Sprintf("gasoline-debug-%s.log", timestamp))
		}

		debugInfo := map[string]interface{}{
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"phase":     phase,
			"error":     fmt.Sprintf("%v", err),
			"port":      port,
			"pid":       os.Getpid(),
		}
		for k, v := range details {
			debugInfo[k] = v
		}

		debugJSON, _ := json.MarshalIndent(debugInfo, "", "  ")
		// #nosec G304 -- debugFile is constructed from trusted timestamp, not user input
		f, err := os.OpenFile(debugFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err == nil {
			_, _ = f.Write(debugJSON)
			_, _ = f.WriteString("\n")
			_ = f.Close()
		}
	}

	// Step 1: Check if server is running
	_ = server.appendToFile([]LogEntry{{
		"type":      "lifecycle",
		"event":     "connection_check",
		"port":      port,
		"pid":       os.Getpid(),
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}})

	serverRunning := isServerRunning(port)

	// Step 2: If not running, try to start new server
	// Note: In cold start with multiple concurrent clients, only one will succeed in binding.
	// Others will fail and fall through to connection logic (step 3).
	if !serverRunning {
		// Try to bind port to see if we can spawn
		testAddr := fmt.Sprintf("127.0.0.1:%d", port)
		ln, err := net.Listen("tcp", testAddr)
		if err == nil {
			// We successfully bound the port - we're the first client, spawn server
			_ = ln.Close()
			_ = server.appendToFile([]LogEntry{{
				"type":      "lifecycle",
				"event":     "mcp_mode_spawn_server",
				"pid":       os.Getpid(),
				"port":      port,
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			}})
			// Quiet mode: Startup message goes to log file only (MCP stdio silence invariant)

			// Spawn server as SEPARATE background process so it persists after client exits
			exe, _ := os.Executable()
			args := []string{"--daemon", "--port", fmt.Sprintf("%d", port)}
			if apiKey != "" {
				args = append(args, "--api-key", apiKey)
			}

			cmd := exec.Command(exe, args...) // #nosec G204 -- exe is our own binary
			cmd.Stdout = nil
			cmd.Stderr = nil
			util.SetDetachedProcess(cmd)
			if err := cmd.Start(); err != nil {
				fmt.Fprintf(os.Stderr, "[gasoline] ERROR: Failed to spawn background server: %v\n", err)
				sendStartupError("Failed to spawn background server: " + err.Error())
				os.Exit(1)
			}

			_ = server.appendToFile([]LogEntry{{
				"type":           "lifecycle",
				"event":          "mcp_server_spawned",
				"client_pid":     os.Getpid(),
				"server_pid":     cmd.Process.Pid,
				"port":           port,
				"timestamp":      time.Now().UTC().Format(time.RFC3339),
			}})

			// Wait for server to be ready
			if !waitForServer(port, 10*time.Second) {
				fmt.Fprintf(os.Stderr, "[gasoline] ERROR: Server failed to start within 10 seconds\n")
				sendStartupError("Server failed to start within 10 seconds")
				os.Exit(1)
			}

			// Server is ready - bridge this client's stdio to HTTP, then exit normally
			bridgeStdioToHTTP(mcpEndpoint)
			return
		}

		// Port bind failed - another client is spawning right now
		// Fall through to connection logic with retries
		_ = server.appendToFile([]LogEntry{{
			"type":      "lifecycle",
			"event":     "spawn_race_detected",
			"pid":       os.Getpid(),
			"port":      port,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}})
		// Quiet mode: Race detection goes to log file only
	}

	// Step 3: Server exists (or is being spawned) - connect with retries
	_ = server.appendToFile([]LogEntry{{
		"type":      "lifecycle",
		"event":     "connect_to_existing",
		"pid":       os.Getpid(),
		"port":      port,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}})
	// Quiet mode: Connection attempts go to log file only

	// Immediately try to connect - no artificial backoff delays
	// The retry logic below provides natural spacing if needed
	healthURL := serverURL + "/health"
	maxRetries := 2 // Fail fast: max 4 seconds total (2 retries × 1s delay + initial)
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Fast retry with 1 second delay (total: 0s, 1s, 2s)
			retryDelay := 1 * time.Second
			_ = server.appendToFile([]LogEntry{{
				"type":      "lifecycle",
				"event":     "connection_retry",
				"attempt":   attempt,
				"error":     fmt.Sprintf("%v", lastErr),
				"pid":       os.Getpid(),
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			}})
			// Quiet mode: Retry details go to log file only, not stderr
			writeDebugInfo(fmt.Sprintf("connection_attempt_%d", attempt), lastErr, map[string]interface{}{"health_url": healthURL})
			time.Sleep(retryDelay)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		req, err := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
		if err != nil {
			cancel()
			lastErr = err
			continue
		}

		resp, err := http.DefaultClient.Do(req)
		cancel() // Cancel after request completes, not before
		if err == nil && resp.StatusCode == http.StatusOK {
			_ = resp.Body.Close()
			// Connection successful - use bridge mode
			if attempt > 0 {
				fmt.Fprintf(os.Stderr, "[gasoline] Connection successful after %d retries\n", attempt)
			}
			bridgeStdioToHTTP(mcpEndpoint)
			return
		}
		if resp != nil {
			_ = resp.Body.Close()
		}
		lastErr = err
	}

	// Step 5: Server unresponsive - attempt graceful zombie recovery
	diagnostics := gatherConnectionDiagnostics(port, serverURL, healthURL)

	_ = server.appendToFile([]LogEntry{{
		"type":        "lifecycle",
		"event":       "zombie_recovery_start",
		"error":       fmt.Sprintf("%v", lastErr),
		"pid":         os.Getpid(),
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
		"diagnostics": diagnostics,
	}})

	// Check if we can identify and recover from a zombie server
	zombiePID := readPIDFile(port)
	if zombiePID > 0 {
		// Check if process is alive
		zombieProcess, err := os.FindProcess(zombiePID)
		if err == nil {
			// Try to signal the process (Signal 0 checks existence without killing)
			if zombieProcess.Signal(syscall.Signal(0)) == nil {
				// Process is alive but not responding - attempt graceful shutdown
				_ = server.appendToFile([]LogEntry{{
					"type":       "lifecycle",
					"event":      "zombie_sigterm",
					"zombie_pid": zombiePID,
					"pid":        os.Getpid(),
					"timestamp":  time.Now().UTC().Format(time.RFC3339),
				}})

				_ = zombieProcess.Signal(syscall.SIGTERM)
				time.Sleep(2 * time.Second)

				// Check if it exited
				if zombieProcess.Signal(syscall.Signal(0)) != nil {
					// Process exited - remove PID file and respawn
					removePIDFile(port)
				} else {
					// Still alive after SIGTERM - force kill
					_ = server.appendToFile([]LogEntry{{
						"type":       "lifecycle",
						"event":      "zombie_sigkill",
						"zombie_pid": zombiePID,
						"pid":        os.Getpid(),
						"timestamp":  time.Now().UTC().Format(time.RFC3339),
					}})
					_ = zombieProcess.Signal(syscall.SIGKILL)
					time.Sleep(500 * time.Millisecond)
					removePIDFile(port)
				}

				// Now respawn fresh server
				_ = server.appendToFile([]LogEntry{{
					"type":      "lifecycle",
					"event":     "zombie_recovery_respawn",
					"pid":       os.Getpid(),
					"port":      port,
					"timestamp": time.Now().UTC().Format(time.RFC3339),
				}})

				exe, _ := os.Executable()
				args := []string{"--daemon", "--port", fmt.Sprintf("%d", port)}
				if apiKey != "" {
					args = append(args, "--api-key", apiKey)
				}

				cmd := exec.Command(exe, args...) // #nosec G204 -- exe is our own binary
				cmd.Stdout = nil
				cmd.Stderr = nil
				util.SetDetachedProcess(cmd)
				if err := cmd.Start(); err != nil {
					sendStartupError("Failed to respawn after zombie recovery: " + err.Error())
					os.Exit(1)
				}

				// Wait for fresh server
				if waitForServer(port, 10*time.Second) {
					_ = server.appendToFile([]LogEntry{{
						"type":      "lifecycle",
						"event":     "zombie_recovery_success",
						"pid":       os.Getpid(),
						"timestamp": time.Now().UTC().Format(time.RFC3339),
					}})
					bridgeStdioToHTTP(mcpEndpoint)
					return
				}
			}
		}
	}

	// Recovery failed or no PID file - give up
	_ = server.appendToFile([]LogEntry{{
		"type":        "lifecycle",
		"event":       "connection_failed",
		"error":       fmt.Sprintf("%v", lastErr),
		"pid":         os.Getpid(),
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
		"diagnostics": diagnostics,
	}})

	fmt.Fprintf(os.Stderr, "[gasoline] ERROR: Server unresponsive after %d retries and recovery failed\n", maxRetries)
	fmt.Fprintf(os.Stderr, "[gasoline] Port %d status: %s\n", port, diagnostics["port_status"])
	fmt.Fprintf(os.Stderr, "[gasoline] Process info: %s\n", diagnostics["process_info"])
	fmt.Fprintf(os.Stderr, "[gasoline]\n")
	fmt.Fprintf(os.Stderr, "[gasoline] To fix: gasoline --stop --port %d\n", port)
	fmt.Fprintf(os.Stderr, "[gasoline] Or kill manually: pkill -9 gasoline\n")

	writeDebugInfo("connection_failure_with_diagnostics", lastErr, diagnostics)
	sendStartupError(fmt.Sprintf("Server unresponsive on port %d after %d retries", port, maxRetries))
	os.Exit(1)
}

// gatherConnectionDiagnostics collects detailed information about why connection failed.
// Returns a map with diagnostic data for debug logging and user error messages.
func gatherConnectionDiagnostics(port int, serverURL string, healthURL string) map[string]interface{} {
	diagnostics := make(map[string]interface{})

	// 1. Check if port is in use
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 500*time.Millisecond)
	if err != nil {
		diagnostics["port_status"] = "not listening"
		diagnostics["port_error"] = err.Error()
	} else {
		_ = conn.Close()
		diagnostics["port_status"] = "listening"
	}

	// 2. Check what process is on the port
	lsofCmd := exec.Command("lsof", "-ti", fmt.Sprintf(":%d", port))
	if pidBytes, err := lsofCmd.Output(); err == nil {
		pids := strings.TrimSpace(string(pidBytes))
		diagnostics["process_pids"] = pids

		// Get process details
		if pids != "" {
			psCmd := exec.Command("ps", "-p", strings.Split(pids, "\n")[0], "-o", "command=")
			if cmdBytes, err := psCmd.Output(); err == nil {
				cmdLine := strings.TrimSpace(string(cmdBytes))
				diagnostics["process_command"] = cmdLine

				// Check if it's actually gasoline
				if strings.Contains(cmdLine, "gasoline") {
					diagnostics["process_type"] = "gasoline (correct)"
				} else {
					diagnostics["process_type"] = "NOT gasoline (conflict)"
					diagnostics["process_info"] = fmt.Sprintf("Port %d is occupied by: %s", port, cmdLine)
				}
			}
		}
	} else {
		diagnostics["process_info"] = "no process found on port"
	}

	// 3. Check if health endpoint responds
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		diagnostics["health_check"] = "failed"
		diagnostics["health_error"] = err.Error()

		// Distinguish types of health check failures
		if strings.Contains(err.Error(), "connection refused") {
			diagnostics["health_diagnosis"] = "port not accepting connections"
		} else if strings.Contains(err.Error(), "timeout") {
			diagnostics["health_diagnosis"] = "server not responding (may be overloaded)"
		} else if strings.Contains(err.Error(), "no route to host") {
			diagnostics["health_diagnosis"] = "network/firewall issue"
		} else {
			diagnostics["health_diagnosis"] = "unknown connection error"
		}
	} else {
		defer resp.Body.Close()
		diagnostics["health_status_code"] = resp.StatusCode

		if resp.StatusCode == http.StatusOK {
			diagnostics["health_check"] = "passed"

			// Try to read response body to verify it's actually gasoline
			body, err := io.ReadAll(io.LimitReader(resp.Body, maxPostBodySize))
			if err == nil && len(body) > 0 {
				bodyStr := string(body)
				if strings.Contains(bodyStr, "gasoline") || strings.Contains(bodyStr, "status") {
					diagnostics["health_response"] = "valid gasoline response"
				} else {
					diagnostics["health_response"] = "unexpected response format"
					previewLen := 100
					if len(bodyStr) < previewLen {
						previewLen = len(bodyStr)
					}
					diagnostics["health_body_preview"] = bodyStr[:previewLen]
				}
			}
		} else {
			diagnostics["health_check"] = fmt.Sprintf("unexpected status %d", resp.StatusCode)
		}
	}

	// 4. Try hitting /mcp endpoint to see if it's reachable
	mcpURL := serverURL + "/mcp"
	mcpReq := `{"jsonrpc":"2.0","id":0,"method":"initialize","params":{}}`
	mcpCtx, mcpCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer mcpCancel()

	httpReq, _ := http.NewRequestWithContext(mcpCtx, "POST", mcpURL, strings.NewReader(mcpReq))
	httpReq.Header.Set("Content-Type", "application/json")
	mcpResp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		diagnostics["mcp_endpoint"] = "unreachable"
		diagnostics["mcp_error"] = err.Error()
	} else {
		defer mcpResp.Body.Close()
		diagnostics["mcp_endpoint"] = fmt.Sprintf("status %d", mcpResp.StatusCode)
		if mcpResp.StatusCode == http.StatusOK {
			diagnostics["mcp_status"] = "responsive"
		}
	}

	// 5. Summary diagnosis
	if diagnostics["port_status"] == "not listening" {
		diagnostics["diagnosis"] = "No server running on port"
		diagnostics["recommended_action"] = "Server should auto-spawn but didn't - check logs"
	} else if diagnostics["process_type"] == "NOT gasoline (conflict)" {
		diagnostics["diagnosis"] = "Port occupied by different service"
		diagnostics["recommended_action"] = fmt.Sprintf("Kill process or use different port: --port %d", port+1)
	} else if diagnostics["health_check"] == "failed" {
		diagnostics["diagnosis"] = "Gasoline process exists but not responding"
		diagnostics["recommended_action"] = "Process may be hung or crashed - will attempt recovery"
	} else if diagnostics["health_check"] == "passed" && diagnostics["mcp_endpoint"] == "unreachable" {
		diagnostics["diagnosis"] = "Health endpoint works but MCP endpoint doesn't"
		diagnostics["recommended_action"] = "Server partially initialized - will attempt recovery"
	} else {
		diagnostics["diagnosis"] = "Unknown connection failure"
		diagnostics["recommended_action"] = "Check debug log for details"
	}

	return diagnostics
}

// runMCPMode runs the server in MCP mode:
// - HTTP server runs in a goroutine (for browser extension)
// - MCP protocol runs over stdin/stdout (for Claude Code)
// If stdin closes (EOF), the HTTP server keeps running until killed.
// Returns error if port binding fails (race condition with another client).
// Never returns on success (blocks forever serving MCP protocol).
func runMCPMode(server *Server, port int, apiKey string) error {
	// Create capture buffers for WebSocket, network, and actions
	cap := capture.NewCapture()

	// Set server version for /sync endpoint compatibility checking
	cap.SetServerVersion(version)

	// Set up lifecycle event logging callback
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

	// Load cached settings from disk (pilot state, etc.)
	_ = server.appendToFile([]LogEntry{{
		"type":      "lifecycle",
		"event":     "loading_settings",
		"pid":       os.Getpid(),
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}})
	cap.LoadSettingsFromDisk()

	// Log settings load result
	_ = server.appendToFile([]LogEntry{{
		"type":      "lifecycle",
		"event":     "settings_loaded",
		"pid":       os.Getpid(),
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}})

	// Register HTTP routes before starting the goroutine.
	mux := setupHTTPRoutes(server, cap)

	// Create context for clean shutdown of background goroutines
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start version checking loop (checks GitHub daily for new releases)
	startVersionCheckLoop(ctx)

	// Start screenshot rate limiter cleanup (removes entries older than 1 minute)
	go func() {
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
	}()

	// Startup cleanup: Check PID file and kill stale process before binding
	pidFile := pidFilePath(port)
	if _, err := os.Stat(pidFile); err == nil {
		// PID file exists - check if process is alive
		pidBytes, err := os.ReadFile(pidFile)
		if err == nil {
			if pid, err := strconv.Atoi(strings.TrimSpace(string(pidBytes))); err == nil {
				// Check if process exists
				process, err := os.FindProcess(pid)
				if err == nil {
					// Try to signal the process (Signal 0 doesn't kill, just checks existence)
					err := process.Signal(syscall.Signal(0))
					if err == nil {
						// Process is alive - this is a real conflict, fail fast
						_ = server.appendToFile([]LogEntry{{
							"type":      "lifecycle",
							"event":     "port_conflict_detected",
							"pid":       os.Getpid(),
							"port":      port,
							"existing_pid": pid,
							"timestamp": time.Now().UTC().Format(time.RFC3339),
						}})
						return fmt.Errorf("port %d already in use by PID %d (run 'gasoline --stop --port %d' to stop it)", port, pid, port)
					}
					// Process is dead - remove stale PID file
					_ = server.appendToFile([]LogEntry{{
						"type":      "lifecycle",
						"event":     "stale_pid_removed",
						"pid":       os.Getpid(),
						"port":      port,
						"stale_pid": pid,
						"timestamp": time.Now().UTC().Format(time.RFC3339),
					}})
					os.Remove(pidFile)
				}
			}
		}
	}

	// Fast-fail: Check if port is available before trying to bind.
	// Intentional double-bind: the pre-flight check provides better error messages
	// and handles concurrent server launch attempts intelligently for the bridge wrapper.
	testAddr := fmt.Sprintf("127.0.0.1:%d", port)
	testLn, err := net.Listen("tcp", testAddr)
	if err != nil {
		_ = server.appendToFile([]LogEntry{{
			"type":      "lifecycle",
			"event":     "port_conflict_detected",
			"pid":       os.Getpid(),
			"port":      port,
			"error":     err.Error(),
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}})
		return fmt.Errorf("port %d already in use (unknown process, try 'lsof -ti :%d | xargs kill -9'): %w", port, port, err)
	}
	testLn.Close()

	// Start HTTP server in background for browser extension
	httpReady := make(chan error, 1)
	srv := &http.Server{
		ReadTimeout:  5 * time.Second,   // Localhost should be fast
		WriteTimeout: 10 * time.Second,  // Localhost should be fast
		IdleTimeout:  120 * time.Second, // Keep-alive for polling connections
		Handler:      AuthMiddleware(apiKey)(mux),
	}
	go func() {
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
	}()

	// Wait for HTTP server to bind before proceeding
	if err := <-httpReady; err != nil {
		_ = server.appendToFile([]LogEntry{{
			"type":      "lifecycle",
			"event":     "http_bind_failed",
			"pid":       os.Getpid(),
			"port":      port,
			"error":     err.Error(),
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}})
		// Return error instead of exiting - allows caller to handle race conditions gracefully
		return fmt.Errorf("cannot bind port %d: %w", port, err)
	}

	_ = server.appendToFile([]LogEntry{{
		"type":      "lifecycle",
		"event":     "http_bind_success",
		"pid":       os.Getpid(),
		"port":      port,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}})
	// Quiet mode: Server startup details go to log file only (MCP standard compliance)
	// All diagnostics preserved in server.logFile for debugging

	// Write PID file for clean shutdown support
	if err := writePIDFile(port); err != nil {
		_ = server.appendToFile([]LogEntry{{
			"type":      "lifecycle",
			"event":     "pid_file_error",
			"error":     err.Error(),
			"pid":       os.Getpid(),
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}})
		// Non-fatal: server can still run without PID file
	}

	_ = server.appendToFile([]LogEntry{{
		"type":       "lifecycle",
		"event":      "startup",
		"version":    version,
		"port":       port,
		"pid":        os.Getpid(),
		"go_version": runtime.Version(),
		"os":         runtime.GOOS,
		"arch":       runtime.GOARCH,
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
	}})

	// MCP SSE transport ready
	_ = server.appendToFile([]LogEntry{{
		"type":      "lifecycle",
		"event":     "mcp_sse_ready",
		"pid":       os.Getpid(),
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}})

	// Wait for shutdown signal
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	s := <-sig

	// Map signal to human-readable source
	var shutdownSource string
	switch s {
	case os.Interrupt:
		shutdownSource = "Ctrl+C (SIGINT)"
	case syscall.SIGTERM:
		shutdownSource = "SIGTERM (likely --stop or kill)"
	case syscall.SIGHUP:
		shutdownSource = "SIGHUP (terminal closed)"
	default:
		shutdownSource = s.String()
	}

	_ = server.appendToFile([]LogEntry{{
		"type":            "lifecycle",
		"event":           "shutdown",
		"signal":          s.String(),
		"signal_num":      int(s.(syscall.Signal)),
		"shutdown_source": shutdownSource,
		"uptime_seconds":  time.Since(startTime).Seconds(),
		"pid":             os.Getpid(),
		"port":            port,
		"timestamp":       time.Now().UTC().Format(time.RFC3339),
	}})

	// Graceful HTTP server shutdown: finish in-flight requests (3s timeout)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		_ = server.appendToFile([]LogEntry{{
			"type":      "lifecycle",
			"event":     "http_shutdown_error",
			"error":     err.Error(),
			"pid":       os.Getpid(),
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}})
	}

	// Shutdown async logger (drain remaining logs)
	server.shutdownAsyncLogger(2 * time.Second)

	// Clean up PID file on shutdown
	removePIDFile(port)

	// Quiet mode: Shutdown messages go to log file only
	return nil // Graceful shutdown via signal
}

// runStopMode gracefully stops a running server on the specified port.
// Uses hybrid approach: PID file (fast) → HTTP /shutdown (graceful) → lsof+kill (fallback).
func runStopMode(port int) {
	fmt.Printf("Stopping gasoline server on port %d...\n", port)

	// Log the stop command invocation to the shared log file
	// This helps diagnose "who stopped the server" questions
	home, _ := os.UserHomeDir()
	logFile := filepath.Join(home, "gasoline-logs.jsonl")
	stopEntry := map[string]any{
		"type":      "lifecycle",
		"event":     "stop_command_invoked",
		"port":      port,
		"source":    "gasoline --stop",
		"caller_pid": os.Getpid(),
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	if data, err := json.Marshal(stopEntry); err == nil {
		// #nosec G304 -- log file path from trusted home directory
		if f, err := os.OpenFile(logFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600); err == nil {
			_, _ = f.Write(data)
			_, _ = f.Write([]byte{'\n'})
			_ = f.Close()
		}
	}

	// Step 1: Try PID file (fast path)
	pid := readPIDFile(port)
	if pid > 0 && isProcessAlive(pid) {
		fmt.Printf("Found server (PID %d) via PID file\n", pid)
		process, err := os.FindProcess(pid)
		if err == nil {
			if err := process.Signal(syscall.SIGTERM); err == nil {
				fmt.Printf("Sent SIGTERM to PID %d\n", pid)
				// Wait briefly for process to exit
				for i := 0; i < 20; i++ {
					time.Sleep(100 * time.Millisecond)
					if !isProcessAlive(pid) {
						fmt.Println("Server stopped successfully")
						removePIDFile(port)
						return
					}
				}
				fmt.Println("Server did not exit within 2 seconds, sending SIGKILL")
				_ = process.Kill()
				removePIDFile(port)
				fmt.Println("Server killed")
				return
			}
		}
	}

	// Step 2: Try HTTP /shutdown endpoint (graceful)
	shutdownURL := fmt.Sprintf("http://127.0.0.1:%d/shutdown", port)
	client := &http.Client{Timeout: 3 * time.Second}
	req, _ := http.NewRequest("POST", shutdownURL, nil)
	resp, err := client.Do(req)
	if err == nil && resp.StatusCode == http.StatusOK {
		_ = resp.Body.Close()
		fmt.Println("Server stopped via HTTP endpoint")
		removePIDFile(port)
		return
	}
	if resp != nil {
		_ = resp.Body.Close()
	}

	// Step 3: Fallback to lsof+kill
	fmt.Println("Trying lsof fallback...")
	lsofCmd := exec.Command("lsof", "-ti", fmt.Sprintf(":%d", port))
	pidBytes, err := lsofCmd.Output()
	if err != nil || len(pidBytes) == 0 {
		fmt.Printf("No server found on port %d\n", port)
		// Clean up stale PID file if it exists
		removePIDFile(port)
		return
	}

	pidStr := strings.TrimSpace(string(pidBytes))
	for _, p := range strings.Split(pidStr, "\n") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		pidNum, err := strconv.Atoi(p)
		if err != nil {
			continue
		}
		process, err := os.FindProcess(pidNum)
		if err == nil {
			fmt.Printf("Sending SIGTERM to PID %d\n", pidNum)
			_ = process.Signal(syscall.SIGTERM)
		}
	}

	// Wait briefly then check
	time.Sleep(500 * time.Millisecond)
	if !isServerRunning(port) {
		fmt.Println("Server stopped successfully")
		removePIDFile(port)
	} else {
		fmt.Printf("Server may still be running, try: kill -9 $(lsof -ti :%d)\n", port)
	}
}

// runForceCleanup kills ALL running gasoline daemons across all ports
// Used during package install to ensure clean upgrade from older versions
func runForceCleanup() {
	fmt.Println("Force cleanup: Killing all running gasoline daemons...")

	// Log the force cleanup to help diagnose version upgrade issues
	home, _ := os.UserHomeDir()
	logFile := filepath.Join(home, "gasoline-logs.jsonl")
	cleanupEntry := map[string]any{
		"type":       "lifecycle",
		"event":      "force_cleanup_invoked",
		"source":     "gasoline --force",
		"caller_pid": os.Getpid(),
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
	}
	if data, err := json.Marshal(cleanupEntry); err == nil {
		// #nosec G304 -- log file path from trusted home directory
		if f, err := os.OpenFile(logFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600); err == nil {
			_, _ = f.Write(data)
			_, _ = f.Write([]byte{'\n'})
			_ = f.Close()
		}
	}

	killed := 0
	failedToKill := 0

	// Step 1: Try to kill processes using /proc or lsof (Unix-like systems)
	if runtime.GOOS != "windows" {
		// Try lsof to find all gasoline processes on any port
		cmd := exec.Command("lsof", "-c", "gasoline")
		output, err := cmd.Output()
		if err == nil {
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				fields := strings.Fields(line)
				// PID is typically the 2nd field in lsof output
				if len(fields) >= 2 {
					pidStr := fields[1]
					pid, err := strconv.Atoi(pidStr)
					if err != nil || pid <= 0 {
						continue
					}
					process, err := os.FindProcess(pid)
					if err == nil {
						// Try SIGTERM first
						if err := process.Signal(syscall.SIGTERM); err == nil {
							fmt.Printf("  Sent SIGTERM to PID %d\n", pid)
							killed++
							// Wait a moment before checking if it exited
							time.Sleep(100 * time.Millisecond)
							if !isProcessAlive(pid) {
								continue
							}
						}
						// If SIGTERM didn't work, use SIGKILL
						if err := process.Kill(); err == nil {
							fmt.Printf("  Sent SIGKILL to PID %d\n", pid)
							killed++
						} else {
							failedToKill++
						}
					}
				}
			}
		}

		// Also try pkill as fallback
		pkillCmd := exec.Command("pkill", "-f", "gasoline.*--daemon")
		_ = pkillCmd.Run() // Best effort
	} else {
		// Windows: use taskkill
		cmd := exec.Command("taskkill", "/IM", "gasoline.exe", "/F")
		output, err := cmd.CombinedOutput()
		if err == nil {
			// taskkill output indicates processes terminated
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				if strings.Contains(line, "SUCCESS") || strings.Contains(line, "terminated") {
					killed++
				}
			}
		}
	}

	// Step 2: Clean up all PID files for ports in common range (7890-7910 + some extras)
	ports := []int{7890, 7891, 7892, 7893, 7894, 7895, 7896, 7897, 7898, 7899}
	for _, p := range ports {
		pidFile := pidFilePath(p)
		if pidFile != "" {
			// Remove stale PID files
			_ = os.Remove(pidFile)
		}
	}

	// Summary
	fmt.Println()
	if killed > 0 {
		fmt.Printf("✓ Successfully killed %d gasoline process(es)\n", killed)
	}
	if failedToKill > 0 {
		fmt.Printf("⚠ Failed to kill %d process(es) (may have already exited)\n", failedToKill)
	}
	if killed == 0 && failedToKill == 0 {
		fmt.Println("✓ No running gasoline processes found")
	}
	fmt.Println()
	fmt.Println("Cleaned up PID files. Safe to proceed with installation.")
}
