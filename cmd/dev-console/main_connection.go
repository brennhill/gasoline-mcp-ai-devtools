// Purpose: Orchestrates daemon discovery, spawn, health-check, and version-mismatch handling for bridge client connections.
// Why: Handles the complex startup handshake where a bridge client must find or launch a compatible daemon.

package main

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"time"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/state"
	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/util"
)

// handleMCPConnection implements the enhanced connection lifecycle with retry and auto-recovery.
func handleMCPConnection(server *Server, port int, apiKey string) {
	serverURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	mcpEndpoint := serverURL + "/mcp"
	dw := &debugWriter{port: port}

	server.logLifecycle("connection_check", port, nil)

	if !isServerRunning(port) {
		if trySpawnServer(server, port, apiKey, mcpEndpoint) {
			return
		}
	}

	server.logLifecycle("connect_to_existing", port, nil)
	healthURL := serverURL + "/health"
	lastErr := connectWithRetries(server, healthURL, mcpEndpoint, dw)
	if lastErr == nil {
		return
	}
	var nonGasolineErr *nonGasolineServiceError
	if errors.As(lastErr, &nonGasolineErr) {
		diagnostics := gatherConnectionDiagnostics(port, serverURL, healthURL)
		reportConnectionFailure(server, port, lastErr, diagnostics, dw)
		return
	}
	var mismatchErr *serverVersionMismatchError
	if errors.As(lastErr, &mismatchErr) {
		server.logLifecycle("version_mismatch_detected", port, map[string]any{
			"expected_version": mismatchErr.expected,
			"actual_version":   mismatchErr.actual,
		})
		if recoverVersionMismatchServer(server, port, apiKey, mcpEndpoint) {
			return
		}
	}

	diagnostics := gatherConnectionDiagnostics(port, serverURL, healthURL)
	server.logLifecycle("zombie_recovery_start", port, map[string]any{
		"error":       fmt.Sprintf("%v", lastErr),
		"diagnostics": diagnostics,
	})

	if recoverZombieServer(server, port, apiKey, mcpEndpoint) {
		return
	}

	reportConnectionFailure(server, port, lastErr, diagnostics, dw)
}

// reportConnectionFailure logs the failure, prints user-facing messages, and exits.
func reportConnectionFailure(server *Server, port int, lastErr error, diagnostics map[string]any, dw *debugWriter) {
	server.logLifecycle("connection_failed", port, map[string]any{
		"error":       fmt.Sprintf("%v", lastErr),
		"diagnostics": diagnostics,
	})

	var nonGasolineErr *nonGasolineServiceError
	if errors.As(lastErr, &nonGasolineErr) {
		fmt.Fprintf(os.Stderr, "[gasoline] ERROR: Port %d is occupied by another service\n", port)
		if nonGasolineErr.serviceName != "" {
			fmt.Fprintf(os.Stderr, "[gasoline] Service name: %s\n", nonGasolineErr.serviceName)
		}
		fmt.Fprintf(os.Stderr, "[gasoline] Use --port to select a free port, or stop that service first\n")
		dw.write("connection_failure_non_gasoline", lastErr, diagnostics)
		sendStartupError(fmt.Sprintf("Port %d is occupied by another service", port))
		os.Exit(1)
	}

	maxRetries := 2
	fmt.Fprintf(os.Stderr, "[gasoline] ERROR: Server unresponsive after %d retries and recovery failed\n", maxRetries)
	fmt.Fprintf(os.Stderr, "[gasoline] Port %d status: %s\n", port, diagnostics["port_status"])
	fmt.Fprintf(os.Stderr, "[gasoline] Process info: %s\n", diagnostics["process_info"])
	fmt.Fprintf(os.Stderr, "[gasoline]\n")
	fmt.Fprintf(os.Stderr, "[gasoline] To fix: gasoline --stop --port %d\n", port)
	fmt.Fprintf(os.Stderr, "[gasoline] Or kill manually: pkill -9 gasoline\n")

	dw.write("connection_failure_with_diagnostics", lastErr, diagnostics)
	sendStartupError(fmt.Sprintf("Server unresponsive on port %d after %d retries", port, maxRetries))
	os.Exit(1)
}

// trySpawnServer attempts to bind the port and spawn a new daemon server.
// Returns true if the server was spawned and the client bridged successfully,
// false if another client is racing to spawn (port bind failed).
func trySpawnServer(server *Server, port int, apiKey string, mcpEndpoint string) bool {
	testAddr := fmt.Sprintf("127.0.0.1:%d", port)
	ln, err := net.Listen("tcp", testAddr)
	if err != nil {
		server.logLifecycle("spawn_race_detected", port, nil)
		return false
	}
	_ = ln.Close()
	server.logLifecycle("mcp_mode_spawn_server", port, nil)

	cmd, err := spawnDaemonCmd(port, apiKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[gasoline] Failed to resolve executable path: %v\n", err)
		return true
	}
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "[gasoline] ERROR: Failed to spawn background server: %v\n", err)
		sendStartupError("Failed to spawn background server: " + err.Error())
		os.Exit(1)
	}

	server.logLifecycle("mcp_server_spawned", port, map[string]any{
		"client_pid": os.Getpid(),
		"server_pid": cmd.Process.Pid,
	})

	if !waitForServer(port, 10*time.Second) {
		fmt.Fprintf(os.Stderr, "[gasoline] ERROR: Server failed to start within 10 seconds\n")
		sendStartupError("Server failed to start within 10 seconds")
		os.Exit(1)
	}

	bridgeStdioToHTTP(mcpEndpoint)
	return true
}

// spawnDaemonCmd builds an exec.Cmd to launch a detached daemon process.
func spawnDaemonCmd(port int, apiKey string) (*exec.Cmd, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	args := []string{"--daemon", "--port", fmt.Sprintf("%d", port)}
	if stateDir := os.Getenv(state.StateDirEnv); stateDir != "" {
		args = append(args, "--state-dir", stateDir)
	}

	cmd := exec.Command(exe, args...) // #nosec G204,G702 -- exe is our own binary path from os.Executable with fixed flags // nosemgrep: go.lang.security.audit.dangerous-exec-command.dangerous-exec-command, go_subproc_rule-subproc -- CLI opens browser with known URL
	cmd.Args[0] = daemonProcessArgv0(exe)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if apiKey != "" {
		cmd.Env = append(os.Environ(), "GASOLINE_API_KEY="+apiKey)
	}
	util.SetDetachedProcess(cmd)
	return cmd, nil
}

// logLifecycle is a convenience method to emit a structured lifecycle log entry.
func (s *Server) logLifecycle(event string, port int, extra map[string]any) {
	entry := LogEntry{
		"type":      "lifecycle",
		"event":     event,
		"pid":       os.Getpid(),
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	if port != 0 {
		entry["port"] = port
	}
	for k, v := range extra {
		entry[k] = v
	}
	s.addEntries([]LogEntry{entry})
}
