// Purpose: Handles interactive TTY startup path that spawns a detached daemon.
// Why: Keeps entrypoint dispatch separate from background daemon bootstrap mechanics.

package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/util"
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
