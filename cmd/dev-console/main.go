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
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/dev-console/dev-console/internal/session"
	"github.com/dev-console/dev-console/internal/util"
)

// version is set at build time via -ldflags "-X main.version=..."
// Fallback used for `go run` and `make dev` (no ldflags).
var version = "6.0.0"

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

	// Upload automation security flags (set by CLI flags, consumed by ToolHandler)
	uploadAutomationFlag bool // --enable-upload-automation
	trustLLMContextFlag  bool // --trust-llm-context
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
			if err == nil && (strings.Contains(string(data), "gasoline") || strings.Contains(string(data), "gasoline-mcp")) {
				return path
			}
		}
	}

	return ""
}

// pidFilePath returns the path to the PID file for a given port
func pidFilePath(port int) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, fmt.Sprintf(".gasoline-%d.pid", port))
}

// writePIDFile writes the current process ID to the PID file
func writePIDFile(port int) error {
	path := pidFilePath(port)
	if path == "" {
		return fmt.Errorf("cannot determine PID file path")
	}
	return os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0600)
}

// readPIDFile reads the PID from the PID file, returns 0 if not found or invalid
func readPIDFile(port int) int {
	path := pidFilePath(port)
	if path == "" {
		return 0
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0
	}
	return pid
}

// removePIDFile removes the PID file for a given port
func removePIDFile(port int) {
	path := pidFilePath(port)
	if path != "" {
		_ = os.Remove(path)
	}
}

// isProcessAlive checks if a process with the given PID is still running
func isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, FindProcess always succeeds. Use Signal(0) to check if process exists.
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

func main() {
	// Install panic recovery with diagnostic logging
	defer func() {
		if r := recover(); r != nil {
			// Get stack trace
			stack := make([]byte, 4096)
			n := runtime.Stack(stack, false)
			stack = stack[:n]

			// Log generic error to stderr (avoid leaking sensitive file paths, env vars, etc)
			fmt.Fprintf(os.Stderr, "\n[gasoline] FATAL ERROR\n")

			// Try to log full details to file for debugging
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
				// #nosec G304 -- crash logs file path from trusted home directory
				if f, err := os.OpenFile(logFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600); err == nil {
					_, _ = f.Write(data)         // #nosec G104 -- best-effort crash logging
					_, _ = f.Write([]byte{'\n'}) // #nosec G104 -- best-effort crash logging
					_ = f.Close()                // #nosec G104 -- best-effort crash logging
				}
			}

			// Also write to a dedicated crash file for easy discovery
			crashFile := filepath.Join(home, "gasoline-crash.log")
			crashContent := fmt.Sprintf("GASOLINE CRASH at %s\nPanic: %v\nStack:\n%s\n",
				time.Now().Format(time.RFC3339), r, stack)
			_ = os.WriteFile(crashFile, []byte(crashContent), 0600) // #nosec G104 -- best-effort crash logging; owner-only for privacy

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
	stopMode := flag.Bool("stop", false, "Stop the running server on the specified port")
	connectMode := flag.Bool("connect", false, "Connect to existing server (multi-client mode)")
	clientID := flag.String("client-id", "", "Override client ID (default: derived from CWD)")
	bridgeMode := flag.Bool("bridge", false, "Run as stdio-to-HTTP bridge (spawns daemon if needed)")
	daemonMode := flag.Bool("daemon", false, "Run as background server daemon (internal use)")
	enableUploadAutomation := flag.Bool("enable-upload-automation", false, "Enable file upload automation (all 4 escalation stages)")
	enableTrustLLMContext := flag.Bool("trust-llm-context", false, "Auto-grant escalation if request has LLM session context")
	forceCleanup := flag.Bool("force", false, "Force kill all running gasoline daemons (used during install to ensure clean upgrade)")
	flag.Bool("mcp", false, "Run in MCP mode (default, kept for backwards compatibility)")

	flag.Parse()

	// Set package-level upload automation flags (consumed by ToolHandler)
	uploadAutomationFlag = *enableUploadAutomation
	trustLLMContextFlag = *enableTrustLLMContext

	// Validate port is in valid range (prevents SSRF through invalid port values)
	if *port < 1 || *port > 65535 {
		fmt.Fprintf(os.Stderr, "[gasoline] Invalid port: %d (must be 1-65535)\n", *port)
		os.Exit(1)
	}

	if *showVersion {
		fmt.Printf("gasoline v%s\n", version)
		os.Exit(0)
	}

	if *showHelp {
		printHelp()
		os.Exit(0)
	}

	// Force cleanup: kill all running gasoline daemons (used during package install)
	if *forceCleanup {
		runForceCleanup()
		os.Exit(0)
	}

	if *checkSetup {
		runSetupCheck(*port)
		os.Exit(0)
	}

	// Stop mode: gracefully stop a running server
	if *stopMode {
		runStopMode(*port)
		return
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
		runBridgeMode(*port, *logFile, *maxEntries)
		return
	}

	// Daemon mode: run server directly (internal use - spawned by MCP client)
	// This bypasses spawn logic to avoid infinite recursion
	if *daemonMode {
		_ = server.appendToFile([]LogEntry{{
			"type":      "lifecycle",
			"event":     "daemon_mode_start",
			"pid":       os.Getpid(),
			"port":      *port,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}})
		if err := runMCPMode(server, *port, *apiKey); err != nil {
			fmt.Fprintf(os.Stderr, "[gasoline] Daemon error: %v\n", err)
			os.Exit(1)
		}
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
		args := []string{"--daemon", "--port", fmt.Sprintf("%d", *port), "--log-file", *logFile, "--max-entries", fmt.Sprintf("%d", *maxEntries)}
		if *apiKey != "" {
			args = append(args, "--api-key", *apiKey)
		}

		// Spawn background server in daemon mode
		cmd := exec.Command(exe, args...) // #nosec G204 -- exe is our own binary path from os.Executable()
		cmd.Stdout = nil
		cmd.Stderr = nil
		// Stdin not needed - daemon mode doesn't read from stdin
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
	// Use bridge mode with fast-start: responds to initialize/tools/list immediately
	// while spawning daemon in background. This gives MCP clients instant feedback.
	runBridgeMode(*port, *logFile, *maxEntries)
}

// sendStartupError sends a JSON-RPC error response before exiting.
// This ensures the parent process (IDE) receives a proper error instead of empty response.
func sendStartupError(message string) {
	errResp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      "startup",
		Error: &JSONRPCError{
			Code:    -32603,
			Message: message,
		},
	}
	respJSON, _ := json.Marshal(errResp)
	fmt.Println(string(respJSON))
	os.Stdout.Sync()
	time.Sleep(100 * time.Millisecond) // Allow OS to flush pipe to parent
}

func printHelp() {
	fmt.Print(`
Gasoline - Browser observability for AI coding agents

Usage: gasoline [options]

Options:
  --port <number>        Port to listen on (default: 7890)
  --log-file <path>      Path to log file (default: ~/gasoline-logs.jsonl)
  --max-entries <number> Max log entries before rotation (default: 1000)
  --stop                 Stop the running server on the specified port
  --force                Force kill ALL running gasoline daemons (used during install)
  --api-key <key>        Require API key for HTTP requests (optional)
  --connect              Connect to existing server (multi-client mode)
  --client-id <id>       Override client ID (default: derived from CWD)
  --check                Verify setup (check port availability, print status)
  --enable-upload-automation  Enable file upload automation (all 4 stages)
  --trust-llm-context    Auto-grant escalation for LLM-driven uploads
  --version              Show version
  --help                 Show this help message

Gasoline always runs in MCP mode: the HTTP server starts in the background
(for the browser extension) and MCP protocol runs over stdio (for Claude Code, Cursor, etc.).
The server persists until explicitly stopped with --stop or killed.

Examples:
  gasoline                              # Start server (daemon mode)
  gasoline --stop                       # Stop server on default port
  gasoline --stop --port 8080           # Stop server on specific port
  gasoline --force                      # Force kill all daemons (for clean upgrade)
  gasoline --api-key s3cret             # Start with API key auth
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
