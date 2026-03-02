// Purpose: Runtime mode detection and dispatch for daemon/bridge startup.
// Why: Keeps main entrypoint small while isolating mode policy and launch behavior.
// Docs: docs/features/architecture/index.md

package main

import (
	"fmt"
	"os"
)

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
		if err := runMCPMode(server, cfg.port, cfg.apiKey, daemonLaunchOptions{Parallel: cfg.parallelMode}); err != nil {
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
