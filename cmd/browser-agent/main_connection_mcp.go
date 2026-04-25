// Purpose: Orchestrates MCP daemon startup wiring and runtime lifecycle.
// Why: Keeps top-level daemon flow readable while delegating setup/shutdown details to focused modules.
//
// Metrics emitted from this file:
//   - telemetry.BeaconEvent("daemon_start", {mode, port})    — once per
//     successful daemon boot, after install ID is warmed.
//   - telemetry.StartUsageBeaconLoop                          — kicks the
//     5-minute periodic usage_summary aggregator (see
//     internal/telemetry/usage_beacon.go).
//   - logLifecycle("startup", port, …)                        — daemon
//     reached steady state.
//   - logLifecycle("mcp_transport_ready")                     — stdio
//     transport is accepting traffic.
//   - logLifecycle("terminal_server_started"
//                 |"terminal_server_bind_failed"
//                 |"terminal_server_died")                    — terminal
//     sub-server lifecycle.
//
// Wire contract: docs/core/app-metrics.md.

package main

import (
	"context"
	"fmt"
	"runtime"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/terminal"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/telemetry"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
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
	server.startScreenshotRateLimiterCleanup(ctx)
	server.startScreenshotDiskCleanup(ctx)
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
	termPort := port + terminal.PortOffset
	termMux, termRelays := setupTerminalMux(server, server.ptyManager, cap)
	server.ptyRelays = termRelays
	termSrv, termDone, termErr := startTerminalServer(termPort, termMux)
	if termErr != nil {
		stderrf("[Kaboom] WARNING: terminal server failed to start on port %d: %v\n", termPort, termErr)
		stderrf("[Kaboom] Terminal features are unavailable. Free port %d or use a different base port.\n", termPort)
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
			stderrf("[Kaboom] terminal server on port %d exited unexpectedly\n", termPort)
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

	// Surface install-id drift (host rename, machine_id change) into the
	// lifecycle log so operators can see when a stored ID stopped matching
	// the deterministic derivation. install_id.go also fires an
	// `install_id_migrated` app_error so analytics can stitch the lineage.
	telemetry.SetInstallIDDriftLogFn(func(stored, derived string) {
		server.logLifecycle("install_id_drift", port, map[string]any{
			"stored_iid":  stored,
			"derived_iid": derived,
		})
	})
	telemetry.Warm() // Pre-load install ID and session off the hot path.
	telemetry.BeaconEvent("daemon_start", map[string]string{
		"mode": "daemon",
		"port": fmt.Sprintf("%d", port),
	})

	// Start periodic usage beacon loop (structured tool stats every 5 minutes).
	if tracker := mcpHandler.GetUsageTracker(); tracker != nil {
		telemetry.StartUsageBeaconLoop(ctx, tracker)
	}

	awaitShutdownSignal(server, srv, port, httpDone, termSrv, termDone, mcpHandler)
	return nil
}
