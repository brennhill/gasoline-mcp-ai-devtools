// Purpose: Bridge daemon startup orchestration and peer-race handling.

package main

import (
	"fmt"
	"time"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/util"
)

// runBridgeMode bridges stdio (from MCP client) to HTTP (to persistent server)
// Uses fast-start: responds to initialize/tools/list immediately while spawning daemon async.
// #lizard forgives
func runBridgeMode(port int, logFile string, maxEntries int) {
	serverURL := fmt.Sprintf("http://127.0.0.1:%d", port)

	// Track daemon state with proper failure handling
	state := &daemonState{
		readyCh:    make(chan struct{}),
		failedCh:   make(chan struct{}),
		port:       port,
		logFile:    logFile,
		maxEntries: maxEntries,
	}

	shouldSpawn := true

	// Phase 1: Check if a compatible server is already running.
	if tryConnectToExisting(state, port) {
		shouldSpawn = false
	}

	// Phase 2: No server found. Wait for a peer bridge to finish spawning
	// before we start our own daemon (avoids multi-bridge spawn races).
	//
	// IMPORTANT: This wait must not block MCP stdio startup. The bridge read loop
	// must begin immediately so initialize/tools/list fast-path responses are not
	// delayed during cold start.
	if shouldSpawn {
		startDaemonSpawnCoordinator(state, port)
	}

	// Bridge stdio <-> HTTP with fast-start support
	bridgeStdioToHTTPFast(serverURL+"/mcp", state, port)
}

// startDaemonSpawnCoordinator runs peer-wait/spawn policy asynchronously so MCP
// stdio handling can start immediately.
func startDaemonSpawnCoordinator(state *daemonState, port int) {
	util.SafeGo(func() {
		if waitForPeerDaemon(state, port) {
			return
		}
		spawnDaemonAsync(state)
	})
}

// tryConnectToExisting checks for a running server and validates compatibility.
// Returns true if connected (markReady), or fatally blocked (markFailed) — no point retrying.
// Returns false if no server is running or the port was freed for a new spawn.
func tryConnectToExisting(state *daemonState, port int) bool {
	if !isServerRunning(port) {
		return false
	}
	compatible, runningVersion, serviceName := runningServerVersionCompatible(port)
	if compatible {
		state.markReady()
		return true
	}
	if isGasolineService(serviceName) {
		// Version mismatch — stop old server, let caller spawn new one.
		if !stopServerForUpgrade(port) {
			state.markFailed(fmt.Sprintf("found running daemon version %s but could not recycle it", runningVersion))
			return true // fatally blocked, don't retry/spawn
		}
		return false // port freed, caller should spawn
	}
	// Non-gasoline service occupies the port.
	if serviceName == "" {
		serviceName = "unknown"
	}
	state.markFailed(fmt.Sprintf("port %d is occupied by non-gasoline service %q", port, serviceName))
	return true // fatally blocked
}

// waitForPeerDaemon retries connecting to a server that another bridge may be spawning.
// Backoff: 500ms, then 2s. Returns true if a compatible server appeared.
func waitForPeerDaemon(state *daemonState, port int) bool {
	// Retry 1: 500ms — quick check in case another bridge just beat us.
	time.Sleep(500 * time.Millisecond)
	if tryConnectToExisting(state, port) {
		return true
	}
	// Retry 2: 2s — longer wait for daemon startup.
	time.Sleep(2 * time.Second)
	return tryConnectToExisting(state, port)
}
