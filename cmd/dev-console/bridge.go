// bridge.go — Bridge mode: stdio-to-HTTP transport for MCP.
// Spawns persistent HTTP server daemon if not running,
// forwards JSON-RPC messages between stdio (MCP client) and HTTP (server).
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/dev-console/dev-console/internal/bridge"
	"github.com/dev-console/dev-console/internal/schema"
	statecfg "github.com/dev-console/dev-console/internal/state"
	"github.com/dev-console/dev-console/internal/util"
)

// toolCallTimeout delegates to internal/bridge for per-request timeout logic.
func toolCallTimeout(req JSONRPCRequest) time.Duration {
	return bridge.ToolCallTimeout(req.Method, req.Params)
}

// mcpStdoutMu serializes all writes to stdout so concurrent bridgeForwardRequest
// goroutines cannot interleave JSON-RPC responses.
var mcpStdoutMu sync.Mutex

// daemonStartupGracePeriod is a short wait window for first tool calls so
// clients don't fail on daemon boot races.
var daemonStartupGracePeriod = 250 * time.Millisecond

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
	s.mu.Lock()
	s.ready = true
	s.failed = false
	s.err = ""
	readyCh := s.readyCh
	shouldClose := !s.readySig
	if shouldClose {
		s.readySig = true
	}
	s.mu.Unlock()
	if shouldClose {
		close(readyCh)
	}
}

// markFailed atomically marks the daemon state as failed and closes failedCh once.
func (s *daemonState) markFailed(errMsg string) {
	s.mu.Lock()
	s.ready = false
	s.failed = true
	s.err = errMsg
	failedCh := s.failedCh
	shouldClose := !s.failedSig
	if shouldClose {
		s.failedSig = true
	}
	s.mu.Unlock()
	if shouldClose {
		close(failedCh)
	}
}

// buildDaemonCmd resolves the current executable and builds an exec.Cmd for the
// daemon process with the appropriate flags and detached-process settings.
func (s *daemonState) buildDaemonCmd() (*exec.Cmd, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("Cannot find executable: %w", err)
	}

	args := []string{"--daemon", "--port", fmt.Sprintf("%d", s.port)}
	if stateDir := os.Getenv(statecfg.StateDirEnv); stateDir != "" {
		args = append(args, "--state-dir", stateDir)
	}
	if s.logFile != "" {
		args = append(args, "--log-file", s.logFile)
	}
	if s.maxEntries > 0 {
		args = append(args, "--max-entries", fmt.Sprintf("%d", s.maxEntries))
	}
	cmd := exec.Command(exe, args...) // #nosec G702 -- exe is our own binary path from os.Executable with fixed flags // nosemgrep: go.lang.security.audit.dangerous-exec-command.dangerous-exec-command, go_subproc_rule-subproc -- bridge spawns own daemon
	cmd.Args[0] = daemonProcessArgv0(exe)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	cmd.Stdin = nil
	util.SetDetachedProcess(cmd)
	return cmd, nil
}

// respawnIfNeeded re-launches the daemon if it's not responding.
// Safe to call from multiple goroutines — only one respawn runs at a time.
// Returns true if the daemon is ready after the respawn attempt.
func (s *daemonState) respawnIfNeeded() bool {
	s.mu.Lock()

	// Already responsive? Quick health check to confirm.
	if s.ready && isServerRunning(s.port) {
		s.mu.Unlock()
		return true
	}

	// Already respawning (channels still open from a concurrent call)?
	// Wait on readyCh/failedCh without spawning again.
	if !s.ready && !s.failed {
		readyCh := s.readyCh
		failedCh := s.failedCh
		s.mu.Unlock()
		select {
		case <-readyCh:
			return true
		case <-failedCh:
			return false
		}
	}

	// Reset state for new spawn attempt.
	s.ready = false
	s.failed = false
	s.err = ""
	s.resetSignalsLocked()
	s.mu.Unlock()

	stderrf("[gasoline] daemon not responding, respawning on port %d\n", s.port)

	cmd, err := s.buildDaemonCmd()
	if err != nil {
		s.markFailed(err.Error())
		return false
	}
	if err := cmd.Start(); err != nil {
		s.markFailed("Failed to start daemon: " + err.Error())
		return false
	}

	if waitForServer(s.port, 4*time.Second) {
		s.markReady()
		stderrf("[gasoline] daemon respawned successfully on port %d\n", s.port)
		return true
	}

	s.markFailed(fmt.Sprintf("Daemon respawned but not responding on port %d after 4s", s.port))
	return false
}

// isConnectionError delegates to internal/bridge for connection error detection.
func isConnectionError(err error) bool {
	return bridge.IsConnectionError(err)
}

// flushStdout syncs stdout and logs any errors (best-effort)
func flushStdout() {
	syncStdoutBestEffort()
}

// sendJSONResponse marshals a response and sends it to stdout, handling errors gracefully
func sendJSONResponse(resp any, id any) {
	respJSON, err := json.Marshal(resp)
	if err != nil {
		sendBridgeError(id, -32603, "Failed to serialize response: "+err.Error(), bridge.StdioFramingLine)
		return
	}
	writeMCPPayload(respJSON, bridge.StdioFramingLine)
}

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

	// Check if server is already running
	if isServerRunning(port) {
		compatible, runningVersion, serviceName := runningServerVersionCompatible(port)
		if compatible {
			state.markReady()
			shouldSpawn = false
		} else {
			if strings.EqualFold(serviceName, "gasoline") {
				if !stopServerForUpgrade(port) {
					state.markFailed(fmt.Sprintf("found running daemon version %s but could not recycle it", runningVersion))
					shouldSpawn = false
				}
			} else {
				if serviceName == "" {
					serviceName = "unknown"
				}
				state.markFailed(fmt.Sprintf("port %d is occupied by non-gasoline service %q", port, serviceName))
				shouldSpawn = false
			}
		}
	}

	if shouldSpawn {
		spawnDaemonAsync(state)
	}

	// Bridge stdio <-> HTTP with fast-start support
	bridgeStdioToHTTPFast(serverURL+"/mcp", state, port)
}

func spawnDaemonAsync(state *daemonState) {
	// Spawn daemon in background (don't block on it)
	util.SafeGo(func() {
		cmd, err := state.buildDaemonCmd()
		if err != nil {
			state.markFailed(err.Error())
			return
		}
		if err := cmd.Start(); err != nil {
			state.markFailed("Failed to start daemon: " + err.Error())
			return
		}

		// Wait for server to be ready (max 4 seconds - fail fast)
		if waitForServer(state.port, 4*time.Second) {
			state.markReady()
		} else {
			state.markFailed(fmt.Sprintf("Daemon started but not responding on port %d after 4s", state.port))
		}
	})
}

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
	if !strings.EqualFold(serviceName, "gasoline") {
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

func waitForDaemonReadinessSignal(state *daemonState, timeout time.Duration) (ready bool, failed bool) {
	if timeout <= 0 {
		return false, false
	}
	state.mu.Lock()
	readyCh := state.readyCh
	failedCh := state.failedCh
	state.mu.Unlock()
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

// checkDaemonStatus returns an error string if the daemon is not ready, or "" if ready.
func checkDaemonStatus(state *daemonState, req JSONRPCRequest, port int) string {
	// Validate method requires daemon
	if req.Method != "tools/call" && !strings.HasPrefix(req.Method, "tools/") && !strings.HasPrefix(req.Method, "resources/") {
		return "method_not_found"
	}

	state.mu.Lock()
	isReady := state.ready
	isFailed := state.failed
	failErr := state.err
	state.mu.Unlock()

	// Heal stale bridge state: daemon is up but local ready flag drifted.
	// Only run this check when daemon state has a concrete port (state.port > 0)
	// to avoid test and fast-path false positives from unrelated local daemons.
	if state.port > 0 && isServerRunning(state.port) {
		if !isReady || isFailed {
			state.mu.Lock()
			state.ready = true
			state.failed = false
			state.err = ""
			state.mu.Unlock()
		}
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
			state.mu.Lock()
			failErr = state.err
			state.mu.Unlock()
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

// bridgeStdioToHTTPFast forwards JSON-RPC with fast-start: responds to initialize/tools/list
// immediately while daemon starts in background. Only blocks on tools/call.
// #lizard forgives
func bridgeStdioToHTTPFast(endpoint string, state *daemonState, port int) {
	reader := bufio.NewReaderSize(os.Stdin, 64*1024)

	client := &http.Client{} // per-request timeouts via context

	var wg sync.WaitGroup
	responseSent := make(chan bool, 1)
	var responseOnce sync.Once
	stats := &bridgeSessionStats{}
	signalResponseSent := func() {
		responseOnce.Do(func() { responseSent <- true })
	}

	toolsList := schema.AllTools()

	var readErr error
	for {
		line, framing, err := readMCPStdioMessage(reader)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			debugf("stdin read error: %v", err)
			readErr = err
			break
		}
		if len(line) == 0 {
			continue
		}
		stats.requests++
		if framing == bridge.StdioFramingContentLength {
			stats.contentLengthFraming++
		} else {
			stats.lineFraming++
		}

		var req JSONRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			stats.parseErrors++
			sendBridgeParseError(line, err, framing)
			signalResponseSent()
			continue
		}
		if req.HasInvalidID() {
			stats.invalidIDs++
			sendBridgeError(nil, -32600, "Invalid Request: id must be string or number when present", framing)
			signalResponseSent()
			continue
		}
		debugf("request method=%s id=%v", req.Method, req.ID)
		stats.lastMethod = req.Method

		// FAST PATH: Handle initialize and tools/list directly (no daemon needed)
		if handleFastPath(req, toolsList, framing) {
			stats.fastPath++
			signalResponseSent()
			continue
		}

		// RESTART FAST PATH: configure(action="restart") handled in bridge, not daemon
		if handleBridgeRestart(req, state, port, framing) {
			stats.fastPath++
			signalResponseSent()
			continue
		}

		// SLOW PATH: Check daemon status for tools/call and other methods
		if status := checkDaemonStatus(state, req, port); status != "" {
			if status == "method_not_found" {
				stats.methodNotFound++
			}
			if status == "starting" {
				stats.starting++
			}
			handleDaemonNotReady(req, status, signalResponseSent, framing)
			continue
		}

		// Forward to HTTP server concurrently
		timeout := toolCallTimeout(req)
		reqCopy, lineCopy := req, append([]byte(nil), line...)
		stats.forwarded++
		wg.Add(1)
		util.SafeGo(func() {
			defer wg.Done()
			bridgeForwardRequest(client, endpoint, reqCopy, lineCopy, timeout, state, signalResponseSent, framing)
		})
	}

	bridgeShutdown(&wg, readErr, responseSent, stats)
}

// Forwarding, error responses, and restart handling moved to bridge_forward.go
