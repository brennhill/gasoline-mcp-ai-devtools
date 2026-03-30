// bridge_startup_orchestration.go -- Bridge daemon startup orchestration and peer-race handling.

package bridge

import (
	"fmt"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/telemetry"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// RunMode bridges stdio (from MCP client) to HTTP (to persistent server)
// Uses fast-start: responds to initialize/tools/list immediately while spawning daemon async.
// #lizard forgives
func RunMode(port int, logFile string, maxEntries int) {
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
	StdioToHTTPFast(serverURL+"/mcp", state, port)
}

// startDaemonSpawnCoordinator runs peer-wait/spawn policy asynchronously so MCP
// stdio handling can start immediately.
func startDaemonSpawnCoordinator(state *daemonState, port int) {
	util.SafeGo(func() {
		if coordinateDaemonStartup(state, port) {
			return
		}
		spawnDaemonAsync(state)
	})
}

func coordinateDaemonStartup(state *daemonState, port int) bool {
	lock, acquired, err := tryAcquireBridgeStartupLock(port)
	if err != nil {
		// Coordination failed (state dir/lock issue). Fall back to local spawn.
		return false
	}
	if acquired {
		startAsStartupLeader(state, port, lock)
		return true
	}

	// Another bridge owns startup leadership. Give it time to bring the daemon up.
	if waitForPeerDaemon(state, port) {
		return true
	}

	// Leader appears stalled. Reclaim stale/dead lock and try to take over.
	_ = clearStaleBridgeStartupLock(port, daemonStartupLockStaleAfter)
	lock, acquired, err = tryAcquireBridgeStartupLock(port)
	if err != nil {
		return false
	}
	if acquired {
		startAsStartupLeader(state, port, lock)
		return true
	}

	// Last short wait in case leader just completed while lock handoff converges.
	return waitForPeerDaemonWithin(state, port, daemonPeerFallbackWaitTimeout)
}

func startAsStartupLeader(state *daemonState, port int, lock *bridgeStartupLock) {
	if lock != nil {
		defer lock.release()
	}
	if tryConnectToExisting(state, port) {
		return
	}
	spawnDaemonAsync(state)
	// Hold leadership until this spawn attempt resolves so followers don't stampede.
	if ready, failed := waitForDaemonReadinessSignal(state, daemonStartupReadyTimeout+daemonPeerPollInterval); ready || failed {
		return
	}
	if isServerRunning(state.port) {
		state.markReady()
	}
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
	if IsKaboomService(serviceName) {
		// Version mismatch — stop old server, let caller spawn new one.
		if !deps.StopServerForUpgrade(port) {
			state.markFailed(fmt.Sprintf("found running daemon version %s but could not recycle it", runningVersion))
			return true // fatally blocked, don't retry/spawn
		}
		return false // port freed, caller should spawn
	}
	// Non-kaboom service occupies the port.
	if serviceName == "" {
		serviceName = "unknown"
	}
	telemetry.BeaconError("bridge_port_blocked", map[string]string{"port": fmt.Sprintf("%d", port)})
	state.markFailed(fmt.Sprintf("port %d is occupied by non-kaboom service %q", port, serviceName))
	return true // fatally blocked
}

// waitForPeerDaemon retries connecting to a server that another bridge may be spawning.
// Returns true if a compatible server appeared before the follower wait budget expires.
func waitForPeerDaemon(state *daemonState, port int) bool {
	return waitForPeerDaemonWithin(state, port, daemonPeerWaitTimeout)
}

func waitForPeerDaemonWithin(state *daemonState, port int, timeout time.Duration) bool {
	if timeout <= 0 {
		return tryConnectToExisting(state, port)
	}
	deadline := time.Now().Add(timeout)
	for {
		if tryConnectToExisting(state, port) {
			return true
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return false
		}
		sleepFor := daemonPeerPollInterval
		if remaining < sleepFor {
			sleepFor = remaining
		}
		time.Sleep(sleepFor)
	}
}
