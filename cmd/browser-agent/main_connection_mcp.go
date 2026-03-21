// Purpose: Orchestrates MCP daemon startup wiring and runtime lifecycle.
// Why: Keeps top-level daemon flow readable while delegating setup/shutdown details to focused modules.

package main

import (
	"context"
	"fmt"
	"runtime"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/telemetry"
	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/util"
)

// runMCPMode runs the server in MCP mode:
// - HTTP server runs in a goroutine (for browser extension)
// - MCP protocol runs over stdin/stdout (for Claude Code)
// If stdin closes (EOF), the HTTP server keeps running until killed.
// Returns error if port binding fails (race condition with another client).
// Never returns on success (blocks forever serving MCP protocol).
func runMCPMode(server *Server, port int, apiKey string, opts daemonLaunchOptions) error {
	server.setListenPort(port)
	cap := initCapture(server, port)
	mux, mcpHandler := setupHTTPRoutes(server, cap)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	startVersionCheckLoop(ctx)
	startScreenshotRateLimiterCleanup(ctx)
	configureBinaryUpgradeMonitoring(ctx, server, port)

	if err := enforceDaemonStartupPolicy(server, port, opts); err != nil {
		return err
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
	persistDaemonRuntimeState(server, port)

	// Start dedicated terminal server on port+1.
	// Non-fatal: if the terminal port is busy, log a warning and continue without terminal.
	termPort := port + terminalPortOffset
	termMux := setupTerminalMux(server, server.ptyManager, cap)
	termSrv, termDone, termErr := startTerminalServer(server, termPort, termMux)
	if termErr != nil {
		stderrf("[gasoline] WARNING: terminal server failed to start on port %d: %v\n", termPort, termErr)
		stderrf("[gasoline] Terminal features are unavailable. Free port %d or use a different base port.\n", termPort)
		server.logLifecycle("terminal_server_bind_failed", termPort, map[string]any{
			"error":     termErr.Error(),
			"term_port": termPort,
		})
	} else {
		server.setTerminalPort(termPort)
		server.logLifecycle("terminal_server_started", termPort, nil)
		// Monitor terminal server — log if it dies, but do NOT bring down main daemon.
		util.SafeGo(func() {
			<-termDone
			stderrf("[gasoline] terminal server on port %d exited unexpectedly\n", termPort)
			server.logLifecycle("terminal_server_died", termPort, nil)
			server.setTerminalPort(0) // Mark as unavailable
		})
	}

	server.logLifecycle("startup", port, map[string]any{
		"version":       version,
		"go_version":    runtime.Version(),
		"os":            runtime.GOOS,
		"arch":          runtime.GOARCH,
		"terminal_port": termPort,
	})
	server.logLifecycle("mcp_transport_ready", port, nil)

	telemetry.BeaconEvent("daemon_start", map[string]string{
		"mode": "daemon",
		"port": fmt.Sprintf("%d", port),
	})

	// Start periodic usage beacon loop (aggregated tool counts every 10 minutes).
	if usageCounter := mcpHandler.GetUsageCounter(); usageCounter != nil {
		telemetry.StartUsageBeaconLoop(ctx, usageCounter)
	}

	awaitShutdownSignal(server, srv, port, httpDone, termSrv, termDone, mcpHandler)
	return nil
}
