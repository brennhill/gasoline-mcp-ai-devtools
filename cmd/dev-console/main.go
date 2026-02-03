// Gasoline - Browser observability for AI coding agents
// A zero-dependency server that receives logs from the browser extension
// and streams them to your AI coding agent via MCP.
//
// Error Handling Strategy:
//   1. HTTP handlers: Return HTTP status codes (400/404/405/500), log to stderr
//   2. MCP JSON-RPC: Return JSON-RPC error responses with code/message
//   3. Background operations: Log to stderr and continue (e.g., file close errors)
//   4. Fatal startup errors: Log to stderr and os.Exit(1)
//   5. Context timeouts: Handled gracefully with error messages
//
// Logging Strategy (zero-dependency policy means no logging library):
//   1. User-facing messages: fmt.Printf() to stdout
//   2. Errors and warnings: fmt.Fprintf(os.Stderr, "[gasoline] ...") to stderr
//   3. Lifecycle events: Written to log file via server.appendToFile()
//   4. Debug output: Only when explicitly enabled
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/dev-console/dev-console/internal/capture"
	"github.com/dev-console/dev-console/internal/session"
	"github.com/dev-console/dev-console/internal/util"
)

// version is set at build time via -ldflags "-X main.version=..."
// Fallback used for `go run` and `make dev` (no ldflags).
var version = "5.4.1"

// startTime tracks when the server started for uptime calculation
var startTime = time.Now()

const (
	defaultPort     = 7890
	maxPostBodySize = 100 * 1024 * 1024 // 100 MB

	// Server health check parameters
	healthCheckMaxAttempts   = 30                      // 30 attempts * 100ms = 3 seconds total
	healthCheckRetryInterval = 100 * time.Millisecond // Retry interval between health check attempts
)

var (
	// Screenshot rate limiting: prevent DoS by limiting uploads to 1/second per client
	screenshotRateLimiter = make(map[string]time.Time) // clientID -> last upload time
	screenshotRateMu      sync.Mutex
)

// findMCPConfig checks for MCP configuration files in common locations
// Returns the path if found, empty string otherwise
func findMCPConfig() string {
	// Claude Code - project-local config
	if _, err := os.Stat(".mcp.json"); err == nil {
		return ".mcp.json"
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	// Check common MCP config locations
	locations := []string{
		filepath.Join(home, ".cursor", "mcp.json"),                     // Cursor
		filepath.Join(home, ".codeium", "windsurf", "mcp_config.json"), // Windsurf
		filepath.Join(home, ".continue", "config.json"),                // Continue
		filepath.Join(home, ".config", "zed", "settings.json"),         // Zed
	}

	for _, path := range locations {
		if _, err := os.Stat(path); err == nil {
			// Verify it actually contains gasoline config
			// #nosec G304 -- paths are from a fixed list of known MCP config locations, not user input
			data, err := os.ReadFile(path)
			if err == nil && (contains(string(data), "gasoline") || contains(string(data), "gasoline-mcp")) {
				return path
			}
		}
	}

	return ""
}

// contains checks if s contains substr (simple replacement for strings.Contains to reduce imports)
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func main() {
	// Install panic recovery with diagnostic logging
	defer func() {
		if r := recover(); r != nil {
			// Get stack trace
			stack := make([]byte, 4096)
			n := runtime.Stack(stack, false)
			stack = stack[:n]

			// Log to stderr
			fmt.Fprintf(os.Stderr, "\n[gasoline] FATAL PANIC: %v\n", r)
			fmt.Fprintf(os.Stderr, "[gasoline] Stack trace:\n%s\n", stack)

			// Try to log to file
			home, _ := os.UserHomeDir()
			logFile := filepath.Join(home, "gasoline-logs.jsonl")
			entry := map[string]any{
				"type":       "lifecycle",
				"event":      "crash",
				"reason":     fmt.Sprintf("%v", r),
				"stack":      string(stack),
				"timestamp":  time.Now().UTC().Format(time.RFC3339),
				"go_version": runtime.Version(),
				"os":         runtime.GOOS,
				"arch":       runtime.GOARCH,
			}
			if data, err := json.Marshal(entry); err == nil {
				// #nosec G302 G304 -- crash logs are intentionally world-readable for debugging
				if f, err := os.OpenFile(logFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644); err == nil {
					_, _ = f.Write(data)         // #nosec G104 -- best-effort crash logging
					_, _ = f.Write([]byte{'\n'}) // #nosec G104 -- best-effort crash logging
					_ = f.Close()                // #nosec G104 -- best-effort crash logging
				}
			}

			// Also write to a dedicated crash file for easy discovery
			crashFile := filepath.Join(home, "gasoline-crash.log")
			crashContent := fmt.Sprintf("GASOLINE CRASH at %s\nPanic: %v\nStack:\n%s\n",
				time.Now().Format(time.RFC3339), r, stack)
			_ = os.WriteFile(crashFile, []byte(crashContent), 0644) // #nosec G104 G306 -- best-effort crash logging; intentionally world-readable

			fmt.Fprintf(os.Stderr, "[gasoline] Crash details written to: %s\n", crashFile)
			os.Exit(1)
		}
	}()

	// Parse flags
	port := flag.Int("port", defaultPort, "Port to listen on")
	logFile := flag.String("log-file", "", "Path to log file (default: ~/gasoline-logs.jsonl)")
	maxEntries := flag.Int("max-entries", defaultMaxEntries, "Max log entries before rotation")
	showVersion := flag.Bool("version", false, "Show version")
	showHelp := flag.Bool("help", false, "Show help")
	apiKey := flag.String("api-key", "", "API key for HTTP authentication (optional)")
	checkSetup := flag.Bool("check", false, "Verify setup: check if port is available and print status")
	persistMode := flag.Bool("persist", true, "Keep server running after MCP client disconnects (default: true)")
	connectMode := flag.Bool("connect", false, "Connect to existing server (multi-client mode)")
	clientID := flag.String("client-id", "", "Override client ID (default: derived from CWD)")
	bridgeMode := flag.Bool("bridge", false, "Run as stdio-to-HTTP bridge (spawns daemon if needed)")
	flag.Bool("mcp", false, "Run in MCP mode (default, kept for backwards compatibility)")

	flag.Parse()

	if *showVersion {
		fmt.Printf("gasoline v%s\n", version)
		os.Exit(0)
	}

	if *showHelp {
		printHelp()
		os.Exit(0)
	}

	if *checkSetup {
		runSetupCheck(*port)
		os.Exit(0)
	}

	// Connect mode: forward MCP to existing server
	if *connectMode {
		// Error acceptable: cwd defaults to empty string if inaccessible; DeriveClientID handles this
		cwd, _ := os.Getwd()
		id := *clientID
		if id == "" {
			id = session.DeriveClientID(cwd)
		}
		runConnectMode(*port, id, cwd)
		return
	}

	// Default log file to home directory
	if *logFile == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "[gasoline] Error getting home directory: %v\n", err)
			os.Exit(1)
		}
		*logFile = filepath.Join(home, "gasoline-logs.jsonl")
	}

	// Create server
	server, err := NewServer(*logFile, *maxEntries)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[gasoline] Error creating server: %v\n", err)
		os.Exit(1)
	}

	// Always run in MCP mode: HTTP server for browser extension + MCP protocol over stdio
	// Bridge mode: stdio-to-HTTP proxy (spawns daemon if needed)
	if *bridgeMode {
		_ = server.appendToFile([]LogEntry{{
			"type":      "lifecycle",
			"event":     "bridge_mode_start",
			"pid":       os.Getpid(),
			"port":      *port,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}})
		fmt.Fprintf(os.Stderr, "[gasoline] Starting in bridge mode (stdio -> HTTP)\n")
		runBridgeMode(*port)
		return
	}

	// Determine if stdin is TTY (user ran "gasoline" interactively) or piped (launched by MCP host)
	stat, err := os.Stdin.Stat()
	var isTTY bool
	var stdinMode os.FileMode
	if err == nil {
		isTTY = (stat.Mode() & os.ModeCharDevice) != 0
		stdinMode = stat.Mode()
	}

	// Log mode detection for diagnostics
	_ = server.appendToFile([]LogEntry{{
		"type":       "lifecycle",
		"event":      "mode_detection",
		"is_tty":     isTTY,
		"stdin_mode": fmt.Sprintf("%v", stdinMode),
		"pid":        os.Getpid(),
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
	}})

	if isTTY {
		// User ran "gasoline" directly - start server as background process (MCP mode)
		_ = server.appendToFile([]LogEntry{{
			"type":      "lifecycle",
			"event":     "spawn_background",
			"pid":       os.Getpid(),
			"port":      *port,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}})

		// Pre-flight check: Warn if MCP config exists (manual start will conflict)
		if mcpConfigPath := findMCPConfig(); mcpConfigPath != "" {
			fmt.Fprintf(os.Stderr, "Warning: MCP configuration detected at %s\n", mcpConfigPath)
			fmt.Fprintf(os.Stderr, "   Manual start may conflict with MCP server management.\n")
			fmt.Fprintf(os.Stderr, "   Recommended: Let your AI tool spawn gasoline automatically.\n")
			fmt.Fprintf(os.Stderr, "   Continuing anyway...\n\n")
			_ = server.appendToFile([]LogEntry{{
				"type":        "lifecycle",
				"event":       "mcp_config_detected",
				"config_path": mcpConfigPath,
				"pid":         os.Getpid(),
				"timestamp":   time.Now().UTC().Format(time.RFC3339),
			}})
		}

		// Pre-flight check: Is port already in use?
		testAddr := fmt.Sprintf("127.0.0.1:%d", *port)
		if ln, err := net.Listen("tcp", testAddr); err != nil {
			fmt.Fprintf(os.Stderr, "Port %d is already in use\n", *port)
			fmt.Fprintf(os.Stderr, "  Fix: kill existing process with: lsof -ti :%d | xargs kill\n", *port)
			fmt.Fprintf(os.Stderr, "  Or use a different port: gasoline --port %d\n", *port+1)
			_ = server.appendToFile([]LogEntry{{
				"type":      "lifecycle",
				"event":     "preflight_failed",
				"error":     "port already in use",
				"port":      *port,
				"pid":       os.Getpid(),
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			}})
			os.Exit(1)
		} else {
			_ = ln.Close() //nolint:errcheck -- pre-flight check; port will be re-bound by child process
		}

		exe, _ := os.Executable()
		args := []string{"--port", fmt.Sprintf("%d", *port), "--log-file", *logFile, "--max-entries", fmt.Sprintf("%d", *maxEntries)}
		if !*persistMode {
			args = append(args, "--persist=false")
		}
		if *apiKey != "" {
			args = append(args, "--api-key", *apiKey)
		}

		// Spawn background process with piped stdin (so it detects as MCP mode, not TTY)
		cmd := exec.Command(exe, args...) // #nosec G204 -- exe is our own binary path from os.Executable()
		cmd.Stdout = nil
		cmd.Stderr = nil
		// Create a pipe for stdin so child process sees piped input (not TTY)
		_, err := cmd.StdinPipe()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create stdin pipe: %v\n", err)
			os.Exit(1)
		}
		util.SetDetachedProcess(cmd)
		if err := cmd.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to spawn background server: %v\n", err)
			_ = server.appendToFile([]LogEntry{{
				"type":      "lifecycle",
				"event":     "spawn_failed",
				"error":     err.Error(),
				"pid":       os.Getpid(),
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			}})
			os.Exit(1)
		}

		backgroundPID := cmd.Process.Pid
		_ = server.appendToFile([]LogEntry{{
			"type":           "lifecycle",
			"event":          "spawn_success",
			"foreground_pid": os.Getpid(),
			"background_pid": backgroundPID,
			"port":           *port,
			"timestamp":      time.Now().UTC().Format(time.RFC3339),
		}})

		// Wait for server to be ready (health check)
		healthURL := fmt.Sprintf("http://127.0.0.1:%d/health", *port)
		fmt.Printf("Starting server (pid %d)...\n", backgroundPID)

		for attempt := 0; attempt < healthCheckMaxAttempts; attempt++ {
			time.Sleep(healthCheckRetryInterval)

			// Check if process is still alive
			if err := cmd.Process.Signal(syscall.Signal(0)); err != nil {
				fmt.Fprintf(os.Stderr, "Background server (pid %d) died during startup\n", backgroundPID)
				fmt.Fprintf(os.Stderr, "  Check logs: tail -20 %s\n", *logFile)
				_ = server.appendToFile([]LogEntry{{
					"type":      "lifecycle",
					"event":     "startup_failed_process_died",
					"pid":       backgroundPID,
					"timestamp": time.Now().UTC().Format(time.RFC3339),
				}})
				os.Exit(1)
			}

			// Try health check
			client := &http.Client{Timeout: 200 * time.Millisecond}
			resp, err := client.Get(healthURL)
			if err == nil && resp.StatusCode == 200 {
				_ = resp.Body.Close() //nolint:errcheck -- best-effort cleanup after health check success
				fmt.Printf("Server ready on http://127.0.0.1:%d\n", *port)
				fmt.Printf("  Log file: %s\n", *logFile)
				fmt.Printf("  Stop with: kill %d\n", backgroundPID)
				_ = server.appendToFile([]LogEntry{{
					"type":      "lifecycle",
					"event":     "startup_verified",
					"pid":       backgroundPID,
					"port":      *port,
					"timestamp": time.Now().UTC().Format(time.RFC3339),
				}})
				os.Exit(0)
			}
			if resp != nil {
				_ = resp.Body.Close() //nolint:errcheck -- best-effort cleanup after health check
			}
		}

		// Timeout: server didn't become ready
		fmt.Fprintf(os.Stderr, "Server (pid %d) failed to respond within 3 seconds\n", backgroundPID)
		fmt.Fprintf(os.Stderr, "  The process is still running but not responding to health checks\n")
		fmt.Fprintf(os.Stderr, "  Check logs: tail -20 %s\n", *logFile)
		fmt.Fprintf(os.Stderr, "  Kill it with: kill %d\n", backgroundPID)
		_ = server.appendToFile([]LogEntry{{
			"type":      "lifecycle",
			"event":     "startup_timeout",
			"pid":       backgroundPID,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}})
		os.Exit(1)
	}

	// stdin is piped -> MCP mode (HTTP + MCP protocol)
	// Enhanced lifecycle with retry and auto-recovery
	handleMCPConnection(server, *port, *apiKey, *persistMode)
}

// handleMCPConnection implements the enhanced connection lifecycle with retry and auto-recovery.
// Lifecycle:
//  1. Check if server is running on port
//  2. If not running: spawn new server
//  3. If running: connect as client
//  4. If connection fails: retry once
//  5. If still fails: kill existing server and spawn new one
//  6. If final attempt fails: write debug file and exit
func handleMCPConnection(server *Server, port int, apiKey string, persist bool) {
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
				"event":     "mcp_mode_start",
				"pid":       os.Getpid(),
				"port":      port,
				"persist":   persist,
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			}})
			fmt.Fprintf(os.Stderr, "[gasoline] Starting in MCP mode (HTTP + MCP protocol, persist=%v)\n", persist)
			runMCPMode(server, port, apiKey, persist)
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
		fmt.Fprintf(os.Stderr, "[gasoline] Another client is spawning server, will connect after it's ready\n")
	}

	// Step 3: Server exists (or is being spawned) - connect with retries
	_ = server.appendToFile([]LogEntry{{
		"type":      "lifecycle",
		"event":     "connect_to_existing",
		"pid":       os.Getpid(),
		"port":      port,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}})
	fmt.Fprintf(os.Stderr, "[gasoline] Connecting to existing server on port %d\n", port)

	// Random backoff (1-3 seconds) to stagger concurrent connections (prevents thundering herd)
	// Use PID for deterministic-but-varied backoff
	backoffMs := 1000 + (os.Getpid() % 2000) // 1-3 seconds
	time.Sleep(time.Duration(backoffMs) * time.Millisecond)

	// Test health endpoint with retries before committing to bridge mode
	// In cold start scenarios, the server may take 1-2 seconds to become ready
	healthURL := serverURL + "/health"
	maxRetries := 3 // Increased for cold start tolerance
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff for retries: 1s, 2s, 3s
			retryDelay := time.Duration(attempt) * time.Second
			_ = server.appendToFile([]LogEntry{{
				"type":      "lifecycle",
				"event":     "connection_retry",
				"attempt":   attempt,
				"error":     fmt.Sprintf("%v", lastErr),
				"pid":       os.Getpid(),
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			}})
			fmt.Fprintf(os.Stderr, "[gasoline] Connection failed (attempt %d/%d), retrying in %v...\n", attempt, maxRetries, retryDelay)
			writeDebugInfo(fmt.Sprintf("connection_attempt_%d", attempt), lastErr, map[string]interface{}{"health_url": healthURL})
			time.Sleep(retryDelay)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		req, err := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
		cancel()

		if err == nil {
			resp, err := http.DefaultClient.Do(req)
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
		} else {
			lastErr = err
		}
	}

	// Step 5: Still failing after retries - gather comprehensive diagnostics before recovery
	diagnostics := gatherConnectionDiagnostics(port, serverURL, healthURL)

	_ = server.appendToFile([]LogEntry{{
		"type":        "lifecycle",
		"event":       "server_recovery",
		"error":       fmt.Sprintf("%v", lastErr),
		"pid":         os.Getpid(),
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
		"diagnostics": diagnostics,
	}})

	fmt.Fprintf(os.Stderr, "[gasoline] Server unresponsive after %d retries\n", maxRetries)
	fmt.Fprintf(os.Stderr, "[gasoline] Diagnostics:\n")
	fmt.Fprintf(os.Stderr, "  Port %d status: %s\n", port, diagnostics["port_status"])
	fmt.Fprintf(os.Stderr, "  Process info: %s\n", diagnostics["process_info"])
	fmt.Fprintf(os.Stderr, "  Health check: %s\n", diagnostics["health_check"])
	fmt.Fprintf(os.Stderr, "[gasoline] Attempting automatic recovery...\n")

	writeDebugInfo("connection_failure_with_diagnostics", lastErr, diagnostics)

	// Kill processes on the port
	killCmd := exec.Command("lsof", "-ti", fmt.Sprintf(":%d", port))
	if output, err := killCmd.Output(); err == nil {
		pids := string(output)
		for _, pidStr := range []string{pids} {
			pidStr = strings.TrimSpace(pidStr)
			if pidStr != "" {
				_ = exec.Command("kill", "-9", pidStr).Run()
			}
		}
	}

	// Wait for port to be free
	time.Sleep(500 * time.Millisecond)

	// Start fresh server
	_ = server.appendToFile([]LogEntry{{
		"type":      "lifecycle",
		"event":     "mcp_mode_start_recovery",
		"pid":       os.Getpid(),
		"port":      port,
		"persist":   persist,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}})
	fmt.Fprintf(os.Stderr, "[gasoline] Starting fresh server on port %d\n", port)

	runMCPMode(server, port, apiKey, persist)
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
			body, err := io.ReadAll(resp.Body)
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
func runMCPMode(server *Server, port int, apiKey string, persist bool) {
	// Create capture buffers for WebSocket, network, and actions
	cap := capture.NewCapture()

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

	// Create SSE registry for MCP connections
	sseRegistry := NewSSERegistry()

	// Register HTTP routes before starting the goroutine.
	setupHTTPRoutes(server, cap, sseRegistry)

	// Create context for clean shutdown of background goroutines
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start version checking loop (checks GitHub daily for new releases)
	startVersionCheckLoop(ctx)

	// Start HTTP server in background for browser extension
	httpReady := make(chan error, 1)
	go func() {
		addr := fmt.Sprintf("127.0.0.1:%d", port)
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			httpReady <- err
			return
		}
		httpReady <- nil
		srv := &http.Server{
			ReadTimeout:  5 * time.Second,   // Localhost should be fast
			WriteTimeout: 10 * time.Second,  // Localhost should be fast
			IdleTimeout:  120 * time.Second, // Keep-alive for polling connections
			Handler:      AuthMiddleware(apiKey)(http.DefaultServeMux),
		}
		// #nosec G114 -- localhost-only MCP background server
		if err := srv.Serve(ln); err != nil {
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
		fmt.Fprintf(os.Stderr, "[gasoline] Fatal: cannot bind port %d: %v\n", port, err)
		fmt.Fprintf(os.Stderr, "[gasoline] Fix: kill existing process with: lsof -ti :%d | xargs kill\n", port)
		fmt.Fprintf(os.Stderr, "[gasoline] Or use a different port: --port %d\n", port+1)
		os.Exit(1)
	}

	_ = server.appendToFile([]LogEntry{{
		"type":      "lifecycle",
		"event":     "http_bind_success",
		"pid":       os.Getpid(),
		"port":      port,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}})
	fmt.Fprintf(os.Stderr, "[gasoline] v%s — HTTP on port %d, log: %s\n", version, port, server.logFile)
	fmt.Fprintf(os.Stderr, "[gasoline] Verify: curl http://localhost:%d/health\n", port)

	// Show first-run help if log file is new/empty
	if fi, err := os.Stat(server.logFile); err != nil || fi.Size() == 0 {
		fmt.Fprintf(os.Stderr, "[gasoline] ─────────────────────────────────────────────────\n")
		fmt.Fprintf(os.Stderr, "[gasoline] First run? Next steps:\n")
		fmt.Fprintf(os.Stderr, "[gasoline]   1. Install extension: chrome://extensions → Load unpacked → extension/\n")
		fmt.Fprintf(os.Stderr, "[gasoline]   2. Open any website in Chrome\n")
		fmt.Fprintf(os.Stderr, "[gasoline]   3. Extension popup should show 'Connected'\n")
		fmt.Fprintf(os.Stderr, "[gasoline] ─────────────────────────────────────────────────\n")
	}

	_ = server.appendToFile([]LogEntry{{"type": "lifecycle", "event": "startup", "version": version, "port": port, "timestamp": time.Now().UTC().Format(time.RFC3339)}})

	// MCP SSE transport ready
	_ = server.appendToFile([]LogEntry{{
		"type":      "lifecycle",
		"event":     "mcp_sse_ready",
		"pid":       os.Getpid(),
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}})
	fmt.Fprintf(os.Stderr, "[gasoline] MCP SSE handler ready at /mcp/sse\n")

	// Wait for shutdown signal
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	s := <-sig
	fmt.Fprintf(os.Stderr, "[gasoline] Received %s, shutting down\n", s)
	_ = server.appendToFile([]LogEntry{{"type": "lifecycle", "event": "shutdown", "reason": s.String(), "timestamp": time.Now().UTC().Format(time.RFC3339)}})
	fmt.Fprintf(os.Stderr, "[gasoline] Shutdown complete\n")
}

func printHelp() {
	fmt.Print(`
Gasoline - Browser observability for AI coding agents

Usage: gasoline [options]

Options:
  --port <number>        Port to listen on (default: 7890)
  --log-file <path>      Path to log file (default: ~/gasoline-logs.jsonl)
  --max-entries <number> Max log entries before rotation (default: 1000)
  --persist              Keep server running after MCP client disconnects (default: true)
  --persist=false        Exit after MCP client disconnects
  --api-key <key>        Require API key for HTTP requests (optional)
  --connect              Connect to existing server (multi-client mode)
  --client-id <id>       Override client ID (default: derived from CWD)
  --check                Verify setup (check port availability, print status)
  --version              Show version
  --help                 Show this help message

Gasoline always runs in MCP mode: the HTTP server starts in the background
(for the browser extension) and MCP protocol runs over stdio (for Claude Code, Cursor, etc.).
The server persists by default, even after the MCP client disconnects.

Examples:
  gasoline                              # MCP mode (default, persist on)
  gasoline --persist=false              # Exit when MCP client disconnects
  gasoline --api-key s3cret             # MCP mode with API key auth
  gasoline --connect --port 7890        # Connect to existing server
  gasoline --check                      # Verify setup before running
  gasoline --port 8080 --max-entries 500

MCP Configuration:
  Add to your Claude Code settings.json or project .mcp.json:
  {
    "mcpServers": {
      "gasoline": {
        "command": "npx",
        "args": ["gasoline-mcp"]
      }
    }
  }
`)
}

// runSetupCheck verifies the setup and prints diagnostic information
func runSetupCheck(port int) {
	fmt.Println()
	fmt.Println("GASOLINE SETUP CHECK")
	fmt.Println("────────────────────────────────────────────────────────────────")
	fmt.Println()
	fmt.Printf("Version: %s\n", version)
	fmt.Printf("Port:    %d\n", port)
	fmt.Println()

	// Check 1: Port availability
	fmt.Print("Checking port availability... ")
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		fmt.Println("FAILED")
		fmt.Printf("  Port %d is already in use.\n", port)
		fmt.Printf("  Fix: lsof -ti :%d | xargs kill\n", port)
		fmt.Printf("  Or use a different port: --port %d\n", port+1)
		fmt.Println()
	} else {
		_ = ln.Close() //nolint:errcheck -- pre-flight check; port availability test only
		fmt.Println("OK")
		fmt.Printf("  Port %d is available.\n", port)
		fmt.Println()
	}

	// Check 2: Log file directory
	fmt.Print("Checking log file directory... ")
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("FAILED")
		fmt.Printf("  Cannot determine home directory: %v\n", err)
		fmt.Println()
	} else {
		logFile := filepath.Join(home, "gasoline-logs.jsonl")
		fmt.Println("OK")
		fmt.Printf("  Log file: %s\n", logFile)
		fmt.Println()
	}

	// Summary
	fmt.Println("────────────────────────────────────────────────────────────────")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Start server:    npx gasoline-mcp")
	fmt.Println("  2. Install extension:")
	fmt.Println("     - Open chrome://extensions")
	fmt.Println("     - Enable Developer mode")
	fmt.Println("     - Click 'Load unpacked' → select extension/ folder")
	fmt.Println("  3. Open any website")
	fmt.Println("  4. Extension popup should show 'Connected'")
	fmt.Println()
	fmt.Printf("Verify:  curl http://localhost:%d/health\n", port)
	fmt.Println()
}
