// main_connection.go — MCP client connection lifecycle: spawn, retry, and zombie recovery.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/dev-console/dev-console/internal/state"
	"github.com/dev-console/dev-console/internal/util"
)

// debugWriter accumulates debug info to a lazily-created temp file.
type debugWriter struct {
	path string
	port int
}

type serverVersionMismatchError struct {
	expected string
	actual   string
}

func (e *serverVersionMismatchError) Error() string {
	return fmt.Sprintf("server version mismatch: expected %s, got %s", e.expected, e.actual)
}

type nonGasolineServiceError struct {
	serviceName string
}

func (e *nonGasolineServiceError) Error() string {
	if e.serviceName == "" {
		return "port occupied by non-gasoline service"
	}
	return fmt.Sprintf("port occupied by non-gasoline service %q", e.serviceName)
}

type healthMetadata struct {
	Version     string `json:"version"`
	ServiceName string `json:"service-name"`
	Name        string `json:"name"`
}

func decodeHealthMetadata(body []byte) (healthMetadata, bool) {
	var meta healthMetadata
	if err := json.Unmarshal(body, &meta); err != nil {
		return healthMetadata{}, false
	}
	return meta, true
}

func (m healthMetadata) resolvedServiceName() string {
	if strings.TrimSpace(m.ServiceName) != "" {
		return strings.TrimSpace(m.ServiceName)
	}
	return strings.TrimSpace(m.Name)
}

// write appends a debug entry to the debug file, creating it on first call.
func (d *debugWriter) write(phase string, err error, details map[string]interface{}) {
	if d.path == "" {
		timestamp := time.Now().Format("20060102-150405")
		d.path = filepath.Join(os.TempDir(), fmt.Sprintf("gasoline-debug-%s.log", timestamp))
	}

	info := map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"phase":     phase,
		"error":     fmt.Sprintf("%v", err),
		"port":      d.port,
		"pid":       os.Getpid(),
	}
	for k, v := range details {
		info[k] = v
	}

	// Error impossible: map contains only primitive types from input
	data, _ := json.MarshalIndent(info, "", "  ")
	// #nosec G703 -- debug path is always under os.TempDir with server-generated timestamp
	f, err := os.OpenFile(d.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600) // nosemgrep: go_filesystem_rule-fileread -- CLI tool writes to local debug log
	if err == nil {
		_, _ = f.Write(data)
		_, _ = f.WriteString("\n")
		_ = f.Close()
	}
}

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
func reportConnectionFailure(server *Server, port int, lastErr error, diagnostics map[string]interface{}, dw *debugWriter) {
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
	cmd.Stdout = nil
	cmd.Stderr = nil
	if apiKey != "" {
		cmd.Env = append(os.Environ(), "GASOLINE_API_KEY="+apiKey)
	}
	util.SetDetachedProcess(cmd)
	return cmd, nil
}

// connectWithRetries attempts to connect to an existing server's health endpoint
// with up to maxRetries. Returns nil on success, or the last error on failure.
func connectWithRetries(server *Server, healthURL string, mcpEndpoint string, dw *debugWriter) error {
	maxRetries := 2
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			server.logLifecycle("connection_retry", 0, map[string]any{
				"attempt": attempt,
				"error":   fmt.Sprintf("%v", lastErr),
			})
			dw.write(fmt.Sprintf("connection_attempt_%d", attempt), lastErr, map[string]interface{}{"health_url": healthURL})
			time.Sleep(1 * time.Second)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		req, err := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
		if err != nil {
			cancel()
			lastErr = err
			continue
		}

		resp, err := http.DefaultClient.Do(req) // #nosec G704 -- healthURL is localhost-only from trusted port
		cancel()
		if err == nil && resp.StatusCode == http.StatusOK {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 16*1024))
			_ = resp.Body.Close()
			meta, ok := decodeHealthMetadata(body)
			if !ok {
				return &nonGasolineServiceError{serviceName: ""}
			}
			serviceName := meta.resolvedServiceName()
			if !strings.EqualFold(serviceName, "gasoline") {
				return &nonGasolineServiceError{serviceName: serviceName}
			}
			runningVersion := strings.TrimSpace(meta.Version)
			if runningVersion == "" {
				return &serverVersionMismatchError{
					expected: version,
					actual:   "<missing>",
				}
			}
			if !versionsMatch(runningVersion, version) {
				return &serverVersionMismatchError{
					expected: version,
					actual:   runningVersion,
				}
			}
			if attempt > 0 {
				fmt.Fprintf(os.Stderr, "[gasoline] Connection successful after %d retries\n", attempt)
			}
			bridgeStdioToHTTP(mcpEndpoint)
			return nil
		}
		if resp != nil {
			_ = resp.Body.Close()
		}
		if err != nil {
			lastErr = err
		} else if resp != nil {
			lastErr = fmt.Errorf("health endpoint returned status %d", resp.StatusCode)
		} else {
			lastErr = errors.New("health request failed with empty response")
		}
	}
	return lastErr
}

func normalizeVersionString(v string) string {
	return strings.TrimPrefix(strings.TrimSpace(v), "v")
}

func versionsMatch(a string, b string) bool {
	return normalizeVersionString(a) == normalizeVersionString(b)
}

func recoverVersionMismatchServer(server *Server, port int, apiKey string, mcpEndpoint string) bool {
	server.logLifecycle("version_mismatch_recycle_start", port, nil)
	if !stopServerForUpgrade(port) {
		server.logLifecycle("version_mismatch_recycle_failed", port, nil)
		return false
	}
	return respawnDaemon(server, port, apiKey, mcpEndpoint)
}

func stopServerForUpgrade(port int) bool {
	_ = tryShutdownViaHTTP(port)
	if waitForPortRelease(port, 500*time.Millisecond) {
		removePIDFile(port)
		return true
	}

	pid := readPIDFile(port)
	if pid > 0 && pid != os.Getpid() {
		terminatePIDQuiet(pid, false)
	}

	pids, err := findProcessOnPort(port)
	if err == nil {
		for _, pid := range pids {
			if pid == os.Getpid() {
				continue
			}
			terminatePIDQuiet(pid, false)
		}
	}

	if waitForPortRelease(port, 1500*time.Millisecond) {
		removePIDFile(port)
		return true
	}

	pids, err = findProcessOnPort(port)
	if err == nil {
		for _, pid := range pids {
			if pid == os.Getpid() {
				continue
			}
			terminatePIDQuiet(pid, true)
		}
	}

	released := waitForPortRelease(port, 1500*time.Millisecond)
	if released {
		removePIDFile(port)
	}
	return released
}

func tryShutdownViaHTTP(port int) bool {
	shutdownURL := fmt.Sprintf("http://127.0.0.1:%d/shutdown", port)
	client := &http.Client{Timeout: 500 * time.Millisecond}
	req, _ := http.NewRequest(http.MethodPost, shutdownURL, nil)
	resp, err := client.Do(req) // #nosec G704 -- shutdownURL is localhost-only from trusted port
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func waitForPortRelease(port int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !isServerRunning(port) {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return !isServerRunning(port)
}

func terminatePIDQuiet(pid int, force bool) {
	process, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	if force {
		_ = process.Kill()
		return
	}

	if runtime.GOOS == "windows" {
		_ = process.Kill()
		return
	}

	_ = process.Signal(syscall.SIGTERM)
	time.Sleep(200 * time.Millisecond)
	if isProcessAlive(pid) {
		_ = process.Kill()
	}
}

// recoverZombieServer attempts to detect and kill a zombie server process,
// then respawn a fresh one. Returns true if recovery succeeded.
func recoverZombieServer(server *Server, port int, apiKey string, mcpEndpoint string) bool {
	zombiePID := readPIDFile(port)
	if zombiePID <= 0 {
		return false
	}
	if !killZombieProcess(server, port, zombiePID) {
		return false
	}
	return respawnDaemon(server, port, apiKey, mcpEndpoint)
}

// killZombieProcess sends SIGTERM then SIGKILL to a zombie server process.
// Returns true if the process was found alive and terminated.
func killZombieProcess(server *Server, port int, zombiePID int) bool {
	zombieProcess, err := os.FindProcess(zombiePID)
	if err != nil {
		return false
	}
	if zombieProcess.Signal(syscall.Signal(0)) != nil {
		return false
	}

	server.logLifecycle("zombie_sigterm", port, map[string]any{"zombie_pid": zombiePID})
	_ = zombieProcess.Signal(syscall.SIGTERM)
	time.Sleep(2 * time.Second)

	if zombieProcess.Signal(syscall.Signal(0)) != nil {
		removePIDFile(port)
		return true
	}

	server.logLifecycle("zombie_sigkill", port, map[string]any{"zombie_pid": zombiePID})
	_ = zombieProcess.Signal(syscall.SIGKILL)
	time.Sleep(500 * time.Millisecond)
	removePIDFile(port)
	return true
}

// respawnDaemon starts a fresh daemon server and bridges stdin/stdout if successful.
func respawnDaemon(server *Server, port int, apiKey string, mcpEndpoint string) bool {
	server.logLifecycle("zombie_recovery_respawn", port, nil)
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[gasoline] Failed to resolve executable path for respawn: %v\n", err)
		return false
	}
	args := []string{"--daemon", "--port", fmt.Sprintf("%d", port)}
	if stateDir := os.Getenv(state.StateDirEnv); stateDir != "" {
		args = append(args, "--state-dir", stateDir)
	}
	if apiKey != "" {
		args = append(args, "--api-key", apiKey)
	}

	cmd := exec.Command(exe, args...) // #nosec G204,G702 -- exe is our own binary path from os.Executable with fixed flags
	cmd.Stdout = nil
	cmd.Stderr = nil
	util.SetDetachedProcess(cmd)
	if err := cmd.Start(); err != nil {
		sendStartupError("Failed to respawn after zombie recovery: " + err.Error())
		os.Exit(1)
	}

	if waitForServer(port, 10*time.Second) {
		server.logLifecycle("zombie_recovery_success", port, nil)
		bridgeStdioToHTTP(mcpEndpoint)
		return true
	}
	return false
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
	_ = s.appendToFile([]LogEntry{entry})
}
