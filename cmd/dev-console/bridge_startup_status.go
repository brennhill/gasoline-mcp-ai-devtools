// Purpose: Bridge daemon status checks and startup failure diagnostics.

package main

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/bridge"
)

// isServerRunning delegates to internal/bridge for health check.
func isServerRunning(port int) bool {
	return bridge.IsServerRunning(port)
}

func runningServerVersionCompatible(port int) (bool, string, string) {
	client := &http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/health", port)) // #nosec G704 -- localhost-only health probe
	if err != nil {
		return false, "", ""
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return false, "", ""
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 16*1024))
	if err != nil {
		return false, "", ""
	}

	meta, ok := decodeHealthMetadata(body)
	if !ok {
		return false, "", ""
	}

	serviceName := meta.resolvedServiceName()
	if !isGasolineService(serviceName) {
		return false, strings.TrimSpace(meta.Version), serviceName
	}

	runningVersion := strings.TrimSpace(meta.Version)
	if runningVersion == "" {
		return false, "<missing>", serviceName
	}
	return versionsMatch(runningVersion, version), runningVersion, serviceName
}

// waitForServer delegates to internal/bridge for server startup wait.
func waitForServer(port int, timeout time.Duration) bool {
	return bridge.WaitForServer(port, timeout)
}

func daemonStartupSuggestion(failErr string, port int) string {
	suggestion := fmt.Sprintf("Server failed to start: %s. ", failErr)
	if strings.Contains(failErr, "port") || strings.Contains(failErr, "bind") || strings.Contains(failErr, "address") {
		suggestion += fmt.Sprintf("Port may be in use. Try: npx gasoline-mcp --port %d", port+1)
	} else {
		suggestion += "Try: npx gasoline-mcp --doctor"
	}
	return suggestion
}

func daemonStatusSnapshot(state *daemonState) (ready bool, failed bool, err string) {
	state.mu.Lock()
	defer state.mu.Unlock()
	return state.ready, state.failed, state.err
}

func daemonFailureErr(state *daemonState) string {
	state.mu.Lock()
	defer state.mu.Unlock()
	return state.err
}

func healDaemonReadyStateIfRunning(state *daemonState, isReady bool, isFailed bool) bool {
	// Only run this check when daemon state has a concrete port (state.port > 0)
	// to avoid test and fast-path false positives from unrelated local daemons.
	if state.port <= 0 || !isServerRunning(state.port) {
		return false
	}
	// Heal stale bridge state: daemon is up but local ready flag drifted.
	if !isReady || isFailed {
		state.mu.Lock()
		defer state.mu.Unlock()
		state.ready = true
		state.failed = false
		state.err = ""
	}
	return true
}

// checkDaemonStatus returns an error string if the daemon is not ready, or "" if ready.
func checkDaemonStatus(state *daemonState, req JSONRPCRequest, port int) string {
	// Validate method requires daemon
	if req.Method != "tools/call" && !strings.HasPrefix(req.Method, "tools/") && !strings.HasPrefix(req.Method, "resources/") {
		return "method_not_found"
	}

	isReady, isFailed, failErr := daemonStatusSnapshot(state)

	if healDaemonReadyStateIfRunning(state, isReady, isFailed) {
		return ""
	}

	if isFailed {
		// Previous spawn failed — try again before giving up.
		if state.respawnIfNeeded() {
			return ""
		}
		return daemonStartupSuggestion(failErr, port)
	}

	if !isReady {
		readySignal, failedSignal := waitForDaemonReadinessSignal(state, daemonStartupGracePeriod)
		if readySignal {
			return ""
		}
		if failedSignal {
			failErr = daemonFailureErr(state)
			if state.respawnIfNeeded() {
				return ""
			}
			return daemonStartupSuggestion(failErr, port)
		}

		// Grace period elapsed: re-check daemon health once before returning startup retry.
		if state.port > 0 && isServerRunning(state.port) {
			state.markReady()
			return ""
		}
		return "starting"
	}
	return ""
}
