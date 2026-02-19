// Purpose: Owns bridge.go runtime behavior and integration logic.
// Docs: docs/features/feature/observe/index.md

// Bridge mode: stdio-to-HTTP transport for MCP
// Spawns persistent HTTP server daemon if not running,
// forwards JSON-RPC messages between stdio (MCP client) and HTTP (server).
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
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

// daemonState tracks the state of daemon startup for fast-start mode.
// Supports respawning: if the daemon dies mid-session, the bridge detects
// connection errors and re-launches the daemon transparently.
type daemonState struct {
	ready    bool
	failed   bool
	err      string
	mu       sync.Mutex
	readyCh  chan struct{}
	failedCh chan struct{}

	// Spawn config — set once at startup, read-only after.
	port       int
	logFile    string
	maxEntries int
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
	s.readyCh = make(chan struct{})
	s.failedCh = make(chan struct{})
	readyCh := s.readyCh
	failedCh := s.failedCh
	s.mu.Unlock()

	stderrf("[gasoline] daemon not responding, respawning on port %d\n", s.port)

	exe, err := os.Executable()
	if err != nil {
		s.mu.Lock()
		s.failed = true
		s.err = "Cannot find executable: " + err.Error()
		s.mu.Unlock()
		close(failedCh)
		return false
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
	cmd := exec.Command(exe, args...) // #nosec G702 -- exe is our own binary path from os.Executable with fixed flags // nosemgrep: go.lang.security.audit.dangerous-exec-command.dangerous-exec-command, go_subproc_rule-subproc -- bridge respawns own daemon
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	cmd.Stdin = nil
	util.SetDetachedProcess(cmd)
	if err := cmd.Start(); err != nil {
		s.mu.Lock()
		s.failed = true
		s.err = "Failed to start daemon: " + err.Error()
		s.mu.Unlock()
		close(failedCh)
		return false
	}

	if waitForServer(s.port, 4*time.Second) {
		s.mu.Lock()
		s.ready = true
		s.mu.Unlock()
		close(readyCh)
		stderrf("[gasoline] daemon respawned successfully on port %d\n", s.port)
		return true
	}

	s.mu.Lock()
	s.failed = true
	s.err = fmt.Sprintf("Daemon respawned but not responding on port %d after 4s", s.port)
	s.mu.Unlock()
	close(failedCh)
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
		sendBridgeError(id, -32603, "Failed to serialize response: "+err.Error())
		return
	}
	mcpStdoutMu.Lock()
	fmt.Println(string(respJSON))
	flushStdout()
	mcpStdoutMu.Unlock()
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
			state.ready = true
			close(state.readyCh)
			shouldSpawn = false
		} else {
			if strings.EqualFold(serviceName, "gasoline") {
				if !stopServerForUpgrade(port) {
					state.failed = true
					state.err = fmt.Sprintf("found running daemon version %s but could not recycle it", runningVersion)
					close(state.failedCh)
					shouldSpawn = false
				}
			} else {
				if serviceName == "" {
					serviceName = "unknown"
				}
				state.failed = true
				state.err = fmt.Sprintf("port %d is occupied by non-gasoline service %q", port, serviceName)
				close(state.failedCh)
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
		exe, err := os.Executable()
		if err != nil {
			state.mu.Lock()
			state.failed = true
			state.err = "Cannot find executable: " + err.Error()
			state.mu.Unlock()
			close(state.failedCh)
			return
		}

		args := []string{"--daemon", "--port", fmt.Sprintf("%d", state.port)}
		if stateDir := os.Getenv(statecfg.StateDirEnv); stateDir != "" {
			args = append(args, "--state-dir", stateDir)
		}
		if state.logFile != "" {
			args = append(args, "--log-file", state.logFile)
		}
		if state.maxEntries > 0 {
			args = append(args, "--max-entries", fmt.Sprintf("%d", state.maxEntries))
		}
		cmd := exec.Command(exe, args...) // #nosec G702 -- exe is our own binary path from os.Executable with fixed flags // nosemgrep: go.lang.security.audit.dangerous-exec-command.dangerous-exec-command, go_subproc_rule-subproc -- CLI bridge mode launches user-specified editor
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
		cmd.Stdin = nil
		util.SetDetachedProcess(cmd)
		if err := cmd.Start(); err != nil {
			state.mu.Lock()
			state.failed = true
			state.err = "Failed to start daemon: " + err.Error()
			state.mu.Unlock()
			close(state.failedCh)
			return
		}

		// Wait for server to be ready (max 4 seconds - fail fast)
		if waitForServer(state.port, 4*time.Second) {
			state.mu.Lock()
			state.ready = true
			state.mu.Unlock()
			close(state.readyCh)
		} else {
			state.mu.Lock()
			state.failed = true
			state.err = fmt.Sprintf("Daemon started but not responding on port %d after 4s", state.port)
			state.mu.Unlock()
			close(state.failedCh)
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
		suggestion := fmt.Sprintf("Server failed to start: %s. ", failErr)
		if strings.Contains(failErr, "port") || strings.Contains(failErr, "bind") || strings.Contains(failErr, "address") {
			suggestion += fmt.Sprintf("Port may be in use. Try: npx gasoline-mcp --port %d", port+1)
		} else {
			suggestion += "Try: npx gasoline-mcp --doctor"
		}
		return suggestion
	}

	if !isReady {
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
	signalResponseSent := func() {
		responseOnce.Do(func() { responseSent <- true })
	}

	toolsList := schema.AllTools()

	var readErr error
	for {
		line, err := readMCPStdioMessage(reader)
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

		var req JSONRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			sendBridgeParseError(line, err)
			signalResponseSent()
			continue
		}
		if req.HasInvalidID() {
			sendBridgeError(nil, -32600, "Invalid Request: id must be string or number when present")
			signalResponseSent()
			continue
		}
		debugf("request method=%s id=%v", req.Method, req.ID)

		// FAST PATH: Handle initialize and tools/list directly (no daemon needed)
		if handleFastPath(req, toolsList) {
			signalResponseSent()
			continue
		}

		// RESTART FAST PATH: configure(action="restart") handled in bridge, not daemon
		if handleBridgeRestart(req, state, port) {
			signalResponseSent()
			continue
		}

		// SLOW PATH: Check daemon status for tools/call and other methods
		if status := checkDaemonStatus(state, req, port); status != "" {
			handleDaemonNotReady(req, status, signalResponseSent)
			continue
		}

		// Forward to HTTP server concurrently
		timeout := toolCallTimeout(req)
		reqCopy, lineCopy := req, append([]byte(nil), line...)
		wg.Add(1)
		util.SafeGo(func() {
			defer wg.Done()
			bridgeForwardRequest(client, endpoint, reqCopy, lineCopy, timeout, state, signalResponseSent)
		})
	}

	bridgeShutdown(&wg, readErr, responseSent)
}

// handleDaemonNotReady sends appropriate error responses when the daemon is not available.
func handleDaemonNotReady(req JSONRPCRequest, status string, signal func()) {
	switch status {
	case "method_not_found":
		sendBridgeError(req.ID, -32601, "Method not found: "+req.Method)
	case "starting":
		sendToolError(req.ID, "Server is starting up. Please retry this tool call in 2 seconds.")
	default:
		sendToolError(req.ID, status)
	}
	signal()
}

// bridgeDoHTTP delegates to internal/bridge for HTTP forwarding.
func bridgeDoHTTP(ctx context.Context, client *http.Client, endpoint string, line []byte) (*http.Response, error) {
	return bridge.DoHTTP(ctx, client, endpoint, line)
}

// bridgeForwardRequest forwards a JSON-RPC request to the HTTP server and writes the response.
// If state is non-nil and the daemon is unreachable, attempts a single respawn + retry.
// #lizard forgives
func bridgeForwardRequest(client *http.Client, endpoint string, req JSONRPCRequest, line []byte, timeout time.Duration, state *daemonState, signal func()) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	activeCancel := cancel

	resp, err := bridgeDoHTTP(ctx, client, endpoint, line)
	if err != nil && isConnectionError(err) && state != nil {
		// Daemon died — attempt respawn and retry with fresh context
		// (original context may have little time left after respawn delay).
		if state.respawnIfNeeded() {
			cancel()
			retryCtx, retryCancel := context.WithTimeout(context.Background(), timeout)
			resp, err = bridgeDoHTTP(retryCtx, client, endpoint, line)
			activeCancel = retryCancel
		}
	}
	defer activeCancel()
	if err != nil {
		sendBridgeError(req.ID, -32603, "Server connection error: "+err.Error())
		signal()
		return
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxPostBodySize))
	_ = resp.Body.Close() //nolint:errcheck // best-effort cleanup
	if err != nil {
		sendBridgeError(req.ID, -32603, "Failed to read response: "+err.Error())
		signal()
		return
	}

	if resp.StatusCode == 204 {
		signal()
		return
	}

	if resp.StatusCode != 200 {
		sendBridgeError(req.ID, -32603, fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body)))
		signal()
		return
	}

	mcpStdoutMu.Lock()
	fmt.Print(string(body))
	flushStdout()
	mcpStdoutMu.Unlock()
	signal()
}

// sendBridgeParseError sends a JSON-RPC parse error (id must be null per spec).
func sendBridgeParseError(_ []byte, err error) {
	errResp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      nil, // JSON-RPC: parse errors must have null id
		Error:   &JSONRPCError{Code: -32700, Message: "Parse error: " + err.Error()},
	}
	// Error impossible: simple struct with no circular refs or unsupported types
	respJSON, _ := json.Marshal(errResp)
	mcpStdoutMu.Lock()
	fmt.Println(string(respJSON))
	flushStdout()
	mcpStdoutMu.Unlock()
}

// readMCPStdioMessage delegates to internal/bridge for stdio message parsing.
func readMCPStdioMessage(reader *bufio.Reader) ([]byte, error) {
	return bridge.ReadStdioMessage(reader, maxPostBodySize)
}

// bridgeShutdown waits for in-flight requests and performs clean shutdown.
func bridgeShutdown(wg *sync.WaitGroup, readErr error, responseSent chan bool) {
	wg.Wait()
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		stderrf("[gasoline-bridge] ERROR: stdin read error: %v\n", readErr)
	}

	select {
	case <-responseSent:
	case <-time.After(5 * time.Second):
	}
	close(responseSent)

	flushStdout()
	time.Sleep(100 * time.Millisecond)
}

// bridgeStdioToHTTP forwards JSON-RPC messages between stdin/stdout and HTTP endpoint
func bridgeStdioToHTTP(endpoint string) {
	reader := bufio.NewReaderSize(os.Stdin, 64*1024)

	client := &http.Client{} // per-request timeouts via context

	var wg sync.WaitGroup
	responseSent := make(chan bool, 1)
	var responseOnce sync.Once
	signalResponseSent := func() {
		responseOnce.Do(func() { responseSent <- true })
	}

	var readErr error
	for {
		line, err := readMCPStdioMessage(reader)
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

		var req JSONRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			sendBridgeParseError(line, err)
			signalResponseSent()
			continue
		}
		if req.HasInvalidID() {
			sendBridgeError(nil, -32600, "Invalid Request: id must be string or number when present")
			signalResponseSent()
			continue
		}
		debugf("request method=%s id=%v", req.Method, req.ID)

		timeout := toolCallTimeout(req)
		reqCopy, lineCopy := req, append([]byte(nil), line...)
		wg.Add(1)
		util.SafeGo(func() {
			defer wg.Done()
			bridgeForwardRequest(client, endpoint, reqCopy, lineCopy, timeout, nil, signalResponseSent)
		})
	}

	bridgeShutdown(&wg, readErr, responseSent)
}

// sendBridgeError sends a JSON-RPC error response to stdout
func sendBridgeError(id any, code int, message string) {
	errResp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &JSONRPCError{
			Code:    code,
			Message: message,
		},
	}
	// Error impossible: simple struct with no circular refs or unsupported types
	respJSON, _ := json.Marshal(errResp)
	mcpStdoutMu.Lock()
	fmt.Println(string(respJSON))
	flushStdout()
	mcpStdoutMu.Unlock()
}

// sendToolError sends a tool result with isError: true (soft error, not protocol error)
// This tells the LLM the tool ran but returned an error, allowing it to retry.
func sendToolError(id any, message string) {
	result := map[string]any{
		"content": []map[string]any{
			{"type": "text", "text": message},
		},
		"isError": true,
	}
	// Error impossible: map contains only primitive types and nested maps
	resultJSON, _ := json.Marshal(result)
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  resultJSON,
	}
	// Error impossible: simple struct with no circular refs or unsupported types
	respJSON, _ := json.Marshal(resp)
	mcpStdoutMu.Lock()
	fmt.Println(string(respJSON))
	flushStdout()
	mcpStdoutMu.Unlock()
}

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
		// SIGCONT unfreezes a SIGSTOP'd process so subsequent signals are delivered.
		_ = p.Signal(syscall.SIGCONT)
	}
}

// handleBridgeRestart handles configure(action="restart") in the bridge layer.
// This is a fast path that works even when the daemon is unresponsive.
// Returns true if the request was handled.
func handleBridgeRestart(req JSONRPCRequest, state *daemonState, port int) bool {
	tool, action := extractToolAction(req)
	if tool != "configure" || action != "restart" {
		return false
	}

	stderrf("[gasoline] bridge restart requested, stopping daemon on port %d\n", port)

	// Kill the daemon (3-layer: HTTP → PID → lsof).
	// First send SIGCONT to unfreeze any SIGSTOP'd process so signals can be delivered.
	forceKillOnPort(port)
	stopped := stopServerForUpgrade(port)

	// Reset bridge state for fresh spawn
	state.mu.Lock()
	state.ready = false
	state.failed = false
	state.err = ""
	state.readyCh = make(chan struct{})
	state.failedCh = make(chan struct{})
	readyCh := state.readyCh
	failedCh := state.failedCh
	state.mu.Unlock()

	// Spawn fresh daemon
	spawnDaemonAsync(state)

	// Wait for daemon to become ready (6s timeout)
	var status, message string
	select {
	case <-readyCh:
		status = "ok"
		message = "Daemon restarted successfully"
		stderrf("[gasoline] daemon restarted successfully on port %d\n", port)
	case <-failedCh:
		state.mu.Lock()
		errMsg := state.err
		state.mu.Unlock()
		status = "error"
		message = "Daemon restart failed: " + errMsg
		stderrf("[gasoline] daemon restart failed: %s\n", errMsg)
	case <-time.After(6 * time.Second):
		status = "error"
		message = "Daemon restart timed out after 6s"
		stderrf("[gasoline] daemon restart timed out\n")
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
	sendFastResponse(req.ID, toolResultJSON)
	return true
}
