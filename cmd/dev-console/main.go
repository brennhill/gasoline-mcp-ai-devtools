// Purpose: Program entry point — dispatches to MCP server, bridge, CLI, connect, stop, doctor, or install modes based on flags.
// Why: Provides a single binary with multiple operating modes selected at startup via command-line arguments.

package main

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// version is set at build time via -ldflags "-X main.version=..."
// Fallback used for `go run` and `make dev` (no ldflags).
var version = "0.7.12"

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
