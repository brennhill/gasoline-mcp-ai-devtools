// main.go — Entry point, mode dispatch, process management, and panic recovery.
// Docs: docs/features/feature/observe/index.md

// Gasoline - Browser observability for AI coding agents
// A zero-dependency server that receives logs from the browser extension
// and streams them to your AI coding agent via MCP.
//
// Error Handling Strategy:
//  1. HTTP handlers: Return HTTP status codes (400/404/405/500), log to stderr
//  2. MCP JSON-RPC: Return JSON-RPC error responses with code/message
//  3. Background operations: Log to stderr and continue (e.g., file close errors)
//  4. Fatal startup errors: Log to stderr and os.Exit(1)
//  5. Context timeouts: Handled gracefully with error messages
//
// Logging Strategy (zero-dependency policy means no logging library):
//  1. User-facing messages: fmt.Printf() to stdout
//  2. Errors and warnings: fmt.Fprintf(os.Stderr, "[gasoline] ...") to stderr
//  3. Lifecycle events: Written to log file via server.appendToFile()
//  4. Debug output: Only when explicitly enabled
package main

import (
	"encoding/json"
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

	"github.com/dev-console/dev-console/internal/state"
	"github.com/dev-console/dev-console/internal/util"
)

// version is set at build time via -ldflags "-X main.version=..."
// Fallback used for `go run` and `make dev` (no ldflags).
var version = "0.7.7"

// startTime tracks when the server started for uptime calculation
var startTime = time.Now()

const (
	defaultPort     = 7890
	maxPostBodySize = 10 * 1024 * 1024 // 10 MB

	// Server health check parameters
	healthCheckMaxAttempts   = 30                     // 30 attempts * 100ms = 3 seconds total
	healthCheckRetryInterval = 100 * time.Millisecond // Retry interval between health check attempts
)

var (
	// Screenshot rate limiting: prevent DoS by limiting uploads to 1/second per client
	screenshotRateLimiter = make(map[string]time.Time) // clientID -> last upload time
	screenshotRateMu      sync.Mutex

	// Upload automation security flags (set by CLI flags, consumed by ToolHandler)
	osUploadAutomationFlag bool            // --enable-os-upload-automation (Stage 4 only)
	uploadSecurityConfig   *UploadSecurity // validated upload security config

	startupWarnings []string
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
		filepath.Join(home, ".claude.json"),                            // Claude
		filepath.Join(home, ".cursor", "mcp.json"),                     // Cursor
		filepath.Join(home, ".codeium", "windsurf", "mcp_config.json"), // Windsurf
		filepath.Join(home, ".continue", "config.json"),                // Continue
		filepath.Join(home, ".config", "zed", "settings.json"),         // Zed
	}

	for _, path := range locations {
		if _, err := os.Stat(path); err == nil {
			// Verify it actually contains gasoline config
			// #nosec G304 -- paths are from a fixed list of known MCP config locations, not user input
			data, err := os.ReadFile(path) // nosemgrep: go_filesystem_rule-fileread -- CLI tool reads known MCP config locations
			if err == nil && (strings.Contains(string(data), "gasoline") || strings.Contains(string(data), "gasoline-mcp")) {
				return path
			}
		}
	}

	return ""
}

// pidFilePath returns the path to the PID file for a given port
func pidFilePath(port int) string {
	path, err := state.PIDFile(port)
	if err != nil {
		return ""
	}
	return path
}

// legacyPIDFilePath returns the old PID path used in previous releases.
func legacyPIDFilePath(port int) string {
	path, err := state.LegacyPIDFile(port)
	if err != nil {
		return ""
	}
	return path
}

// writePIDFile writes the current process ID to the PID file
func writePIDFile(port int) error {
	path := pidFilePath(port)
	if path == "" {
		return fmt.Errorf("cannot determine PID file path")
	}
	// #nosec G301 -- runtime state directory: owner rwx, group rx for diagnostics
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("cannot create PID directory: %w", err)
	}
	return os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0600)
}

// readPIDFile reads the PID from the PID file, returns 0 if not found or invalid
func readPIDFile(port int) int {
	paths := []string{pidFilePath(port), legacyPIDFilePath(port)}
	for _, path := range paths {
		if path == "" {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
		if err == nil {
			return pid
		}
	}
	return 0
}

// removePIDFile removes the PID file for a given port
func removePIDFile(port int) {
	paths := []string{pidFilePath(port), legacyPIDFilePath(port)}
	for _, path := range paths {
		if path != "" {
			_ = os.Remove(path)
		}
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

// handlePanicRecovery logs crash details and writes a crash file for diagnostic discovery.
func handlePanicRecovery(r any) {
	stack := make([]byte, 4096)
	n := runtime.Stack(stack, false)
	stack = stack[:n]

	fmt.Fprintf(os.Stderr, "\n[gasoline] FATAL ERROR\n")

	logFile, err := state.DefaultLogFile()
	if err != nil {
		logFile = filepath.Join(os.TempDir(), "gasoline.jsonl")
	}
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
		// #nosec G301 -- runtime state directory: owner rwx, group rx for diagnostics
		_ = os.MkdirAll(filepath.Dir(logFile), 0o750)
		// #nosec G304 -- crash logs path resolved from trusted runtime state directory
		if f, err := os.OpenFile(logFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600); err == nil { // nosemgrep: go_filesystem_rule-fileread -- CLI tool writes to local crash log
			_, _ = f.Write(data)         // #nosec G104 -- best-effort crash logging
			_, _ = f.Write([]byte{'\n'}) // #nosec G104 -- best-effort crash logging
			_ = f.Close()                // #nosec G104 -- best-effort crash logging
		}
	}

	if diagPath := appendExitDiagnostic("panic", map[string]any{
		"reason": fmt.Sprintf("%v", r),
		"stack":  string(stack),
	}); diagPath != "" {
		fmt.Fprintf(os.Stderr, "[gasoline] Crash details written to: %s\n", diagPath)
	}
	os.Exit(1)
}

// resolveCrashFile returns the best available crash log file path.
func resolveCrashFile() string {
	crashFile, err := state.CrashLogFile()
	if err == nil {
		return crashFile
	}
	if legacy, legacyErr := state.LegacyCrashLogFile(); legacyErr == nil {
		return legacy
	}
	return filepath.Join(os.TempDir(), "gasoline-crash.log")
}

// runTTYMode spawns a background daemon when the user runs gasoline interactively.
func runTTYMode(server *Server, cfg *serverConfig) {
	server.logLifecycle("spawn_background", cfg.port, nil)

	if mcpConfigPath := findMCPConfig(); mcpConfigPath != "" {
		fmt.Fprintf(os.Stderr, "Warning: MCP configuration detected at %s\n", mcpConfigPath)
		fmt.Fprintf(os.Stderr, "   Manual start may conflict with MCP server management.\n")
		fmt.Fprintf(os.Stderr, "   Recommended: Let your AI tool spawn gasoline automatically.\n")
		fmt.Fprintf(os.Stderr, "   Continuing anyway...\n\n")
		server.logLifecycle("mcp_config_detected", cfg.port, map[string]any{"config_path": mcpConfigPath})
	}

	preflightPortCheckOrExit(server, cfg.port)
	cmd := spawnBackgroundDaemon(server, cfg)
	waitForDaemonReady(server, cmd, cfg)
}

// preflightPortCheckOrExit verifies the port is available before spawning a daemon.
func preflightPortCheckOrExit(server *Server, port int) {
	testAddr := fmt.Sprintf("127.0.0.1:%d", port)
	ln, err := net.Listen("tcp", testAddr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Port %d is already in use\n", port)
		fmt.Fprintf(os.Stderr, "  Fix: kill existing process with: %s\n", portKillHint(port))
		fmt.Fprintf(os.Stderr, "  Or use a different port: gasoline --port %d\n", port+1)
		server.logLifecycle("preflight_failed", port, map[string]any{"error": "port already in use"})
		os.Exit(1)
	}
	_ = ln.Close() //nolint:errcheck // pre-flight check; port will be re-bound by child process
}

// spawnBackgroundDaemon starts a detached daemon process and returns the exec.Cmd.
func spawnBackgroundDaemon(server *Server, cfg *serverConfig) *exec.Cmd {
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[gasoline] Failed to resolve executable path: %v\n", err)
		os.Exit(1)
	}
	args := []string{"--daemon", "--port", fmt.Sprintf("%d", cfg.port), "--log-file", cfg.logFile, "--max-entries", fmt.Sprintf("%d", cfg.maxEntries)}
	if cfg.stateDir != "" {
		args = append(args, "--state-dir", cfg.stateDir)
	}
	if cfg.apiKey != "" {
		args = append(args, "--api-key", cfg.apiKey)
	}

	cmd := exec.Command(exe, args...) // #nosec G204,G702 -- exe is our own binary path from os.Executable with fixed flags // nosemgrep: go.lang.security.audit.dangerous-exec-command.dangerous-exec-command, go_subproc_rule-subproc -- CLI opens browser with known URL
	cmd.Args[0] = daemonProcessArgv0(exe)
	cmd.Stdout = nil
	cmd.Stderr = nil
	_, err = cmd.StdinPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create stdin pipe: %v\n", err)
		os.Exit(1)
	}
	util.SetDetachedProcess(cmd)
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to spawn background server: %v\n", err)
		server.logLifecycle("spawn_failed", cfg.port, map[string]any{"error": err.Error()})
		os.Exit(1)
	}

	server.logLifecycle("spawn_success", cfg.port, map[string]any{
		"foreground_pid": os.Getpid(),
		"background_pid": cmd.Process.Pid,
	})
	return cmd
}

// waitForDaemonReady polls the daemon's health endpoint and exits with status.
func waitForDaemonReady(server *Server, cmd *exec.Cmd, cfg *serverConfig) {
	backgroundPID := cmd.Process.Pid
	healthURL := fmt.Sprintf("http://127.0.0.1:%d/health", cfg.port)
	fmt.Printf("Starting server (pid %d)...\n", backgroundPID)

	for attempt := 0; attempt < healthCheckMaxAttempts; attempt++ {
		time.Sleep(healthCheckRetryInterval)

		if err := cmd.Process.Signal(syscall.Signal(0)); err != nil {
			fmt.Fprintf(os.Stderr, "Background server (pid %d) died during startup\n", backgroundPID)
			fmt.Fprintf(os.Stderr, "  Check logs: tail -20 %s\n", cfg.logFile)
			server.logLifecycle("startup_failed_process_died", 0, map[string]any{"pid": backgroundPID})
			os.Exit(1)
		}

		client := &http.Client{Timeout: 200 * time.Millisecond}
		resp, err := client.Get(healthURL)
		if err == nil && resp.StatusCode == 200 {
			_ = resp.Body.Close() //nolint:errcheck // best-effort cleanup after health check success
			fmt.Printf("Server ready on http://127.0.0.1:%d\n", cfg.port)
			fmt.Printf("  Log file: %s\n", cfg.logFile)
			fmt.Printf("  Stop with: kill %d\n", backgroundPID)
			server.logLifecycle("startup_verified", cfg.port, map[string]any{"pid": backgroundPID})
			os.Exit(0)
		}
		if resp != nil {
			_ = resp.Body.Close() //nolint:errcheck // best-effort cleanup after health check
		}
	}

	fmt.Fprintf(os.Stderr, "Server (pid %d) failed to respond within 3 seconds\n", backgroundPID)
	fmt.Fprintf(os.Stderr, "  The process is still running but not responding to health checks\n")
	fmt.Fprintf(os.Stderr, "  Check logs: tail -20 %s\n", cfg.logFile)
	fmt.Fprintf(os.Stderr, "  Kill it with: kill %d\n", backgroundPID)
	server.logLifecycle("startup_timeout", 0, map[string]any{"pid": backgroundPID})
	os.Exit(1)
}

// detectStdinMode returns whether stdin is a TTY and the file mode for diagnostics.
func detectStdinMode() (isTTY bool, stdinMode os.FileMode) {
	stat, err := os.Stdin.Stat()
	if err == nil {
		isTTY = (stat.Mode() & os.ModeCharDevice) != 0
		stdinMode = stat.Mode()
	}
	return isTTY, stdinMode
}

// selectRuntimeMode decides how to run based on flags.
// Default is bridge mode so MCP startup is reliable regardless of PTY/stdio behavior.
func selectRuntimeMode(cfg *serverConfig, _ bool) runtimeMode {
	if cfg.bridgeMode {
		return modeBridge
	}
	if cfg.daemonMode {
		return modeDaemon
	}
	return modeBridge
}

// dispatchMode selects and runs the appropriate runtime mode based on config and stdin.
func dispatchMode(server *Server, cfg *serverConfig) {
	isTTY, stdinMode := detectStdinMode()
	mcpConfigPath := findMCPConfig()
	mode := selectRuntimeMode(cfg, isTTY)
	if mode == modeDaemon {
		setStderrSink(os.Stderr)
	}

	server.logLifecycle("mode_detection", cfg.port, map[string]any{
		"is_tty":           isTTY,
		"stdin_mode":       fmt.Sprintf("%v", stdinMode),
		"has_mcp_config":   mcpConfigPath != "",
		"selected_runtime": mode,
	})

	switch mode {
	case modeDaemon:
		server.logLifecycle("daemon_mode_start", cfg.port, nil)
		if err := runMCPMode(server, cfg.port, cfg.apiKey); err != nil {
			diagPath := appendExitDiagnostic("daemon_start_failed", map[string]any{
				"port":  cfg.port,
				"error": err.Error(),
			})
			if diagPath != "" {
				stderrf("[gasoline] Startup diagnostics written to: %s\n", diagPath)
			}
			stderrf("[gasoline] Daemon error: %v\n", err)
			os.Exit(1)
		}
		return
	case modeBridge:
		if err := ensureBridgeIOIsolation(cfg.logFile); err != nil {
			sendStartupError("Bridge stdio isolation failed: " + err.Error())
			os.Exit(1)
		}
		server.logLifecycle("bridge_mode_start", cfg.port, bridgeLaunchFingerprint())
		if cfg.bridgeMode {
			stderrf("[gasoline] Starting in bridge mode (stdio -> HTTP)\n")
		} else if isTTY && mcpConfigPath != "" {
			stderrf("[gasoline] MCP config detected at %s; running in bridge mode for tool compatibility.\n", mcpConfigPath)
		} else if isTTY {
			stderrf("[gasoline] Running in bridge mode by default. Use --daemon for server-only mode.\n")
		}
		if os.Getenv("GASOLINE_TEST_BRIDGE_NOISE") == "1" {
			// Test-only probe: verifies transport isolation prevents accidental
			// stdout/stderr writes from corrupting MCP responses.
			fmt.Fprintln(os.Stdout, "GASOLINE_TEST_NOISE_STDOUT")
			fmt.Fprintln(os.Stderr, "GASOLINE_TEST_NOISE_STDERR")
		}
		runBridgeMode(cfg.port, cfg.logFile, cfg.maxEntries)
		return
	default:
		// Defensive fallback (should be unreachable).
		runBridgeMode(cfg.port, cfg.logFile, cfg.maxEntries)
		return
	}
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			handlePanicRecovery(r)
		}
	}()

	if len(os.Args) >= 2 && IsCLIMode(os.Args[1:]) {
		os.Exit(runCLIMode(os.Args[1:]))
	}

	cfg := parseAndValidateFlags()

	server, err := NewServer(cfg.logFile, cfg.maxEntries)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[gasoline] Error creating server: %v\n", err)
		os.Exit(1)
	}
	for _, warning := range startupWarnings {
		server.AddWarning(warning)
	}

	dispatchMode(server, cfg)
}

// #lizard forgives
func printHelp() {
	fmt.Print(`
Gasoline - Browser observability for AI coding agents

Usage: gasoline [options]

Options:
  --port <number>        Port to listen on (default: 7890)
  --log-file <path>      Path to log file (default: in runtime state dir)
  --state-dir <path>     Directory for runtime state (default: OS app state dir)
  --max-entries <number> Max log entries before rotation (default: 1000)
  --stop                 Stop the running server on the specified port
  --force                Force kill ALL running gasoline daemons (used during install)
  --api-key <key>        Require API key for HTTP requests (optional)
  --connect              Connect to existing server (multi-client mode)
  --client-id <id>       Override client ID (default: derived from CWD)
  --check                Verify setup (check port availability, print status)
  --doctor               Run full diagnostics (alias of --check)
  --fastpath-min-samples Minimum telemetry samples required for threshold check (default: 50)
  --fastpath-max-failure-ratio Maximum allowed fast-path failure ratio for --check (disabled by default)
  --persist              Deprecated no-op (kept for backwards compatibility)
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

CLI Mode (direct tool access):
  gasoline observe errors --limit 50
  gasoline analyze dom --selector "button"
  gasoline observe logs --min-level warn
  gasoline generate har --save-to out.har
  gasoline configure health
  gasoline interact click --selector "#btn"

  CLI flags: --port, --format (human|json|csv), --timeout (ms)
  Env vars: GASOLINE_PORT, GASOLINE_FORMAT, GASOLINE_STATE_DIR

MCP Configuration:
  gasoline-mcp --install     Auto-install to all detected AI clients
  gasoline-mcp --config      Show configuration and detected clients
  gasoline-mcp --doctor      Run diagnostics on installed configs

  Supported clients: Claude Code, Claude Desktop, Cursor, Windsurf, VS Code
`)
}
