// Purpose: Bridge restart fast-path helpers.

package main

import (
	"encoding/json"
	"os"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/bridge"
)

const (
	// restartGracePeriod is the maximum time to wait for the daemon to become ready
	// after a bridge-initiated restart before reporting a timeout.
	restartGracePeriod = 6 * time.Second
)

// extractToolAction delegates to internal/bridge for tool action extraction.
func extractToolAction(req JSONRPCRequest) (toolName, action string) {
	return bridge.ExtractToolAction(req.Method, req.Params)
}

// forceKillOnPort sends SIGCONT then SIGKILL to any process on the given port.
// This handles edge cases where the daemon is frozen (SIGSTOP) and can't process
// SIGTERM from stopServerForUpgrade's normal escalation path.
func forceKillOnPort(port int) {
	pids, err := findProcessOnPort(port)
	if err != nil {
		return
	}
	self := os.Getpid()
	for _, pid := range pids {
		if pid == self {
			continue
		}
		p, err := os.FindProcess(pid)
		if err != nil {
			continue
		}
		// On Unix this sends SIGCONT; on Windows this is a no-op.
		signalResumeProcess(p)
	}
}

// handleBridgeRestart handles configure(action="restart") in the bridge layer.
// This is a fast path that works even when the daemon is unresponsive.
// Returns true if the request was handled.
func handleBridgeRestart(req JSONRPCRequest, state *daemonState, port int, framing bridge.StdioFraming) bool {
	tool, action := extractToolAction(req)
	if tool != "configure" || action != "restart" {
		return false
	}

	stderrf("[Kaboom] bridge restart requested, stopping daemon on port %d\n", port)

	// Kill the daemon (3-layer: HTTP → PID → lsof).
	// First send SIGCONT to unfreeze any SIGSTOP'd process so signals can be delivered.
	forceKillOnPort(port)
	stopped := stopServerForUpgrade(port)

	// Reset bridge state for fresh spawn.
	readyCh, failedCh := func() (chan struct{}, chan struct{}) {
		state.mu.Lock()
		defer state.mu.Unlock()
		state.ready = false
		state.failed = false
		state.err = ""
		state.resetSignalsLocked()
		return state.readyCh, state.failedCh
	}()

	// Spawn fresh daemon.
	spawnDaemonAsync(state)

	// Wait for daemon to become ready (6s timeout).
	var status, message string
	select {
	case <-readyCh:
		status = "ok"
		message = "Daemon restarted successfully"
		stderrf("[Kaboom] daemon restarted successfully on port %d\n", port)
	case <-failedCh:
		errMsg := daemonFailureErr(state)
		status = "error"
		message = "Daemon restart failed: " + errMsg
		stderrf("[Kaboom] daemon restart failed: %s\n", errMsg)
	case <-time.After(restartGracePeriod):
		status = "error"
		message = "Daemon restart timed out after 6s"
		stderrf("[Kaboom] daemon restart timed out\n")
	}

	result := map[string]any{
		"status":           status,
		"restarted":        status == "ok",
		"message":          message,
		"previous_stopped": stopped,
	}
	resultJSON, _ := json.Marshal(result)
	toolResult := map[string]any{
		"content": []map[string]any{
			{"type": "text", "text": string(resultJSON)},
		},
	}
	if status != "ok" {
		toolResult["isError"] = true
	}
	toolResultJSON, _ := json.Marshal(toolResult)
	sendFastResponse(req.ID, toolResultJSON, framing)
	return true
}
