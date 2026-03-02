// Purpose: Program entry point — dispatches to MCP server, bridge, CLI, connect, stop, doctor, or install modes based on flags.
// Why: Provides a single binary with multiple operating modes selected at startup via command-line arguments.

package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/dev-console/dev-console/internal/util"
)

// version is set at build time via -ldflags "-X main.version=..."
// Fallback used for `go run` and `make dev` (no ldflags).
var version = "0.7.9"

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
  --parallel             Opt-in parallel mode (isolated state dir, no takeover)
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
