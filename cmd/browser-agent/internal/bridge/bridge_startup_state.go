// bridge_startup_state.go -- Daemon startup state and respawn synchronization for bridge fast-start.
//
// Metrics emitted from this file (all via telemetry.AppError):
//   - bridge_spawn_build_error  — go build failed before spawning daemon.
//                                 Classified internal/fatal — local toolchain
//                                 broken; we cannot recover.
//   - bridge_spawn_start_error  — exec.Start returned an error (binary
//                                 missing, permissions). internal/fatal.
//   - bridge_spawn_timeout      — daemon failed health check inside the
//                                 startup deadline. internal/error.
//
// Wire contract: docs/core/app-metrics.md (classifyAppErrorEvent in
// internal/telemetry/beacon.go owns the kind/severity assignment).

package bridge

import (
	"fmt"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/telemetry"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// daemonState tracks the state of daemon startup for fast-start mode.
// Supports respawning: if the daemon dies mid-session, the bridge detects
// connection errors and re-launches the daemon transparently.
type daemonState struct {
	ready     bool
	failed    bool
	err       string
	mu        sync.Mutex
	readyCh   chan struct{}
	failedCh  chan struct{}
	readySig  bool
	failedSig bool

	// Spawn config — set once at startup, read-only after.
	port       int
	logFile    string
	maxEntries int
}

type respawnPlan struct {
	alreadyReady bool
	waitForPeer  bool
	readyCh      <-chan struct{}
	failedCh     <-chan struct{}
}

type peerSignalWaitResult struct {
	ready    bool
	failed   bool
	timedOut bool
}

// resetSignalsLocked replaces readiness/failure channels for a fresh spawn cycle.
// Caller must hold s.mu.
func (s *daemonState) resetSignalsLocked() {
	s.readyCh = make(chan struct{})
	s.failedCh = make(chan struct{})
	s.readySig = false
	s.failedSig = false
}

// markReady atomically marks the daemon as ready and closes readyCh once.
func (s *daemonState) markReady() {
	readyCh, shouldClose := s.setReadyState()
	if shouldClose {
		close(readyCh)
	}
}

func (s *daemonState) setReadyState() (readyCh chan struct{}, shouldClose bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ready = true
	s.failed = false
	s.err = ""
	readyCh = s.readyCh
	shouldClose = !s.readySig
	if shouldClose {
		s.readySig = true
	}
	return readyCh, shouldClose
}

// markFailed atomically marks the daemon state as failed and closes failedCh once.
func (s *daemonState) markFailed(errMsg string) {
	failedCh, shouldClose := s.setFailedState(errMsg)
	if shouldClose {
		close(failedCh)
	}
}

func (s *daemonState) setFailedState(errMsg string) (failedCh chan struct{}, shouldClose bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ready = false
	s.failed = true
	s.err = errMsg
	failedCh = s.failedCh
	shouldClose = !s.failedSig
	if shouldClose {
		s.failedSig = true
	}
	return failedCh, shouldClose
}

func (s *daemonState) planRespawnAttempt() respawnPlan {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Already responsive? Quick health check to confirm.
	if s.ready && isServerRunning(s.port) {
		return respawnPlan{alreadyReady: true}
	}

	// Already respawning (channels still open from a concurrent call)?
	// Wait on readyCh/failedCh without spawning again.
	if !s.ready && !s.failed {
		return respawnPlan{
			waitForPeer: true,
			readyCh:     s.readyCh,
			failedCh:    s.failedCh,
		}
	}

	// Reset state for new spawn attempt.
	s.ready = false
	s.failed = false
	s.err = ""
	s.resetSignalsLocked()
	return respawnPlan{}
}

func waitForRespawnPeerSignals(plan respawnPlan, timeout time.Duration) peerSignalWaitResult {
	if timeout <= 0 {
		timeout = daemonStartupReadyTimeout
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-plan.readyCh:
		return peerSignalWaitResult{ready: true}
	case <-plan.failedCh:
		return peerSignalWaitResult{failed: true}
	case <-timer.C:
		return peerSignalWaitResult{timedOut: true}
	}
}

func (s *daemonState) reclaimRespawnLeadership(expectedReady <-chan struct{}, expectedFailed <-chan struct{}) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ready {
		return false
	}
	if !s.failed && (s.readyCh != expectedReady || s.failedCh != expectedFailed) {
		return false
	}

	s.ready = false
	s.failed = false
	s.err = ""
	s.resetSignalsLocked()
	return true
}

// respawnIfNeeded re-launches the daemon if it's not responding.
// Safe to call from multiple goroutines — only one respawn runs at a time.
// Returns true if the daemon is ready after the respawn attempt.
func (s *daemonState) respawnIfNeeded() bool {
	for {
		plan := s.planRespawnAttempt()
		if plan.alreadyReady {
			return true
		}
		if !plan.waitForPeer {
			break
		}

		waitResult := waitForRespawnPeerSignals(plan, daemonStartupReadyTimeout)
		if waitResult.ready {
			return true
		}
		if waitResult.failed {
			return false
		}
		if waitResult.timedOut {
			if s.reclaimRespawnLeadership(plan.readyCh, plan.failedCh) {
				break
			}
			// Another goroutine changed state while this caller was waiting; re-plan.
			continue
		}
	}

	if s.port <= 0 {
		s.markFailed("respawn requested without a valid daemon port")
		return false
	}

	deps.Stderrf("[Kaboom] daemon not responding, respawning on port %d\n", s.port)

	cmd, err := s.buildDaemonCmd()
	if err != nil {
		s.markFailed(err.Error())
		return false
	}
	if err := cmd.Start(); err != nil {
		s.markFailed("Failed to start daemon: " + err.Error())
		return false
	}

	if waitForServer(s.port, daemonStartupReadyTimeout) {
		s.markReady()
		deps.Stderrf("[Kaboom] daemon respawned successfully on port %d\n", s.port)
		return true
	}

	s.markFailed(fmt.Sprintf("Daemon respawned but not responding on port %d after %s", s.port, daemonStartupReadyTimeout))
	return false
}

func spawnDaemonAsync(state *daemonState) {
	// Spawn daemon in background (don't block on it)
	util.SafeGo(func() {
		cmd, err := state.buildDaemonCmd()
		if err != nil {
			telemetry.AppError("bridge_spawn_build_error", nil)
			state.markFailed(err.Error())
			return
		}
		if err := cmd.Start(); err != nil {
			telemetry.AppError("bridge_spawn_start_error", nil)
			state.markFailed("Failed to start daemon: " + err.Error())
			return
		}

		// Wait for server to be ready (bounded startup budget).
		if waitForServer(state.port, daemonStartupReadyTimeout) {
			state.markReady()
		} else {
			telemetry.AppError("bridge_spawn_timeout", nil)
			state.markFailed(fmt.Sprintf("Daemon started but not responding on port %d after %s", state.port, daemonStartupReadyTimeout))
		}
	})
}

func waitForDaemonReadinessSignal(state *daemonState, timeout time.Duration) (ready bool, failed bool) {
	if timeout <= 0 {
		return false, false
	}
	readyCh, failedCh := func() (chan struct{}, chan struct{}) {
		state.mu.Lock()
		defer state.mu.Unlock()
		return state.readyCh, state.failedCh
	}()
	if readyCh == nil || failedCh == nil {
		return false, false
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-readyCh:
		return true, false
	case <-failedCh:
		return false, true
	case <-timer.C:
		return false, false
	}
}
