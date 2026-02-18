// Purpose: Owns bridge.go runtime behavior and integration logic.
// Docs: docs/features/feature/observe/index.md

// Bridge mode: stdio-to-HTTP transport for MCP
// Spawns persistent HTTP server daemon if not running,
// forwards JSON-RPC messages between stdio (MCP client) and HTTP (server).
package main

import (
	"bufio"
	"bytes"
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
	"strconv"
	"strings"
	"sync"
	"time"

	statecfg "github.com/dev-console/dev-console/internal/state"
	"github.com/dev-console/dev-console/internal/util"
)

// toolCallTimeout returns the per-request timeout based on the MCP tool name.
// Fast tools (observe, generate, configure, resources/read) get 10s; slow tools
// (analyze, interact) that round-trip to the extension get 35s.
// Annotation observe (observe command_result for ann_*) gets 65s for blocking poll.
func toolCallTimeout(req JSONRPCRequest) time.Duration {
	const (
		fast         = 10 * time.Second
		slow         = 35 * time.Second
		blockingPoll = 65 * time.Second // annotation observe: server blocks up to 55s
	)

	if req.Method == "resources/read" {
		return fast
	}
	if req.Method != "tools/call" {
		return fast // non-tool calls (ping, list, etc.)
	}

	// Extract tool name and arguments without full unmarshal
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if json.Unmarshal(req.Params, &params) != nil {
		return fast
	}

	switch params.Name {
	case "analyze", "interact":
		return slow
	case "observe":
		var args struct {
			What          string `json:"what"`
			CorrelationID string `json:"correlation_id"`
		}
		if json.Unmarshal(params.Arguments, &args) == nil {
			// Annotation command_result polling blocks server-side for up to 55s
			if args.What == "command_result" &&
				len(args.CorrelationID) > 4 && args.CorrelationID[:4] == "ann_" {
				return blockingPoll
			}
			// Screenshot round-trips to extension (sync poll + capture + upload)
			if args.What == "screenshot" {
				return slow
			}
		}
		return fast
	default:
		return fast
	}
}

// mcpStdoutMu serializes all writes to stdout so concurrent bridgeForwardRequest
// goroutines cannot interleave JSON-RPC responses.
var mcpStdoutMu sync.Mutex

type bridgeFastPathResourceReadCounters struct {
	mu      sync.Mutex
	success int64
	failure int64
}

var fastPathResourceReadCounters bridgeFastPathResourceReadCounters

func recordFastPathResourceRead(uri string, success bool, errorCode int) {
	fastPathResourceReadCounters.mu.Lock()
	defer fastPathResourceReadCounters.mu.Unlock()
	if success {
		fastPathResourceReadCounters.success++
	} else {
		fastPathResourceReadCounters.failure++
	}
	appendFastPathResourceReadTelemetry(uri, success, errorCode, fastPathResourceReadCounters.success, fastPathResourceReadCounters.failure)
}

func snapshotFastPathResourceReadCounters() (success int64, failure int64) {
	fastPathResourceReadCounters.mu.Lock()
	defer fastPathResourceReadCounters.mu.Unlock()
	return fastPathResourceReadCounters.success, fastPathResourceReadCounters.failure
}

func resetFastPathResourceReadCounters() {
	fastPathResourceReadCounters.mu.Lock()
	defer fastPathResourceReadCounters.mu.Unlock()
	fastPathResourceReadCounters.success = 0
	fastPathResourceReadCounters.failure = 0
}

func fastPathResourceReadLogPath() (string, error) {
	return statecfg.InRoot("logs", "bridge-fastpath-resource-read.jsonl")
}

func appendFastPathResourceReadTelemetry(uri string, success bool, errorCode int, successCount int64, failureCount int64) {
	path, err := fastPathResourceReadLogPath()
	if err != nil {
		return
	}
	if mkErr := os.MkdirAll(filepath.Dir(path), 0o750); mkErr != nil {
		return
	}
	entry := map[string]any{
		"timestamp":      time.Now().UTC().Format(time.RFC3339Nano),
		"event":          "bridge_fastpath_resources_read",
		"uri":            uri,
		"success":        success,
		"error_code":     errorCode,
		"success_count":  successCount,
		"failure_count":  failureCount,
		"pid":            os.Getpid(),
		"bridge_version": version,
	}
	line, marshalErr := json.Marshal(entry)
	if marshalErr != nil {
		return
	}
	// #nosec G304 -- path is deterministic under state root
	f, openErr := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if openErr != nil {
		return
	}
	defer func() { _ = f.Close() }()
	_, _ = f.Write(line)
	_, _ = f.Write([]byte("\n"))
}

type bridgeFastPathCounters struct {
	mu      sync.Mutex
	success int
	failure int
}

var fastPathCounters bridgeFastPathCounters

func resetFastPathCounters() {
	fastPathCounters.mu.Lock()
	fastPathCounters.success = 0
	fastPathCounters.failure = 0
	fastPathCounters.mu.Unlock()
}

func fastPathTelemetryLogPath() (string, error) {
	return statecfg.InRoot("logs", "bridge-fastpath-events.jsonl")
}

func recordFastPathEvent(method string, success bool, errorCode int) {
	fastPathCounters.mu.Lock()
	if success {
		fastPathCounters.success++
	} else {
		fastPathCounters.failure++
	}
	successCount := fastPathCounters.success
	failureCount := fastPathCounters.failure
	fastPathCounters.mu.Unlock()

	path, err := fastPathTelemetryLogPath()
	if err != nil {
		return
	}
	// #nosec G301 -- runtime state directory for local diagnostics.
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return
	}
	event := map[string]any{
		"timestamp":     time.Now().UTC().Format(time.RFC3339Nano),
		"event":         "bridge_fastpath_method",
		"method":        method,
		"success":       success,
		"error_code":    errorCode,
		"success_count": successCount,
		"failure_count": failureCount,
		"pid":           os.Getpid(),
		"version":       version,
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return
	}
	// #nosec G304 -- deterministic diagnostics path rooted in runtime state directory.
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) // nosemgrep: go_filesystem_rule-fileread -- local diagnostics log append
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()
	_, _ = f.Write(append(payload, '\n'))
}

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

// isConnectionError returns true if the error indicates the daemon is unreachable.
func isConnectionError(err error) bool {
	if err == nil {
		return false
	}
	// Prefer typed error checks over string matching
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return true
	}
	// Fallback: string check for wrapped errors that lose type info
	msg := err.Error()
	return strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "no such host")
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

// isServerRunning checks if a server is healthy on the given port via HTTP health check.
// This catches zombie servers that accept TCP connections but don't respond to HTTP.
func isServerRunning(port int) bool {
	client := &http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/health", port)) // #nosec G704 -- localhost-only health probe
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }() //nolint:errcheck // best-effort cleanup
	return resp.StatusCode == http.StatusOK
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

// waitForServer waits for the server to start accepting connections
func waitForServer(port int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if isServerRunning(port) {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

// fastPathResponses maps MCP methods to their static JSON result bodies.
// Methods in this map are handled without waiting for the daemon.
var fastPathResponses = map[string]string{
	"ping":         `{}`,
	"prompts/list": `{"prompts":[]}`,
}

// sendFastResponse marshals and sends a JSON-RPC response for the fast path.
func sendFastResponse(id any, result json.RawMessage) {
	resp := JSONRPCResponse{JSONRPC: "2.0", ID: id, Result: result}
	// Error impossible: simple struct with no circular refs or unsupported types
	respJSON, _ := json.Marshal(resp)
	mcpStdoutMu.Lock()
	fmt.Println(string(respJSON))
	flushStdout()
	mcpStdoutMu.Unlock()
}

func sendFastError(id any, code int, message string) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &JSONRPCError{Code: code, Message: message},
	}
	respJSON, _ := json.Marshal(resp)
	mcpStdoutMu.Lock()
	fmt.Println(string(respJSON))
	flushStdout()
	mcpStdoutMu.Unlock()
}

// handleFastPath handles MCP methods that don't require the daemon.
// Returns true if the method was handled.
func handleFastPath(req JSONRPCRequest, toolsList []MCPTool) bool {
	// JSON-RPC 2.0: "A Notification is a Request object without an 'id' member."
	if req.ID == nil {
		return true
	}

	switch req.Method {
	case "initialize":
		// Negotiate protocol version: echo if supported, otherwise use latest.
		protocolVersion := "2025-06-18"
		var initParams struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		if json.Unmarshal(req.Params, &initParams) == nil {
			switch initParams.ProtocolVersion {
			case "2024-11-05", "2025-06-18":
				protocolVersion = initParams.ProtocolVersion
			}
		}
		result := map[string]any{
			"protocolVersion": protocolVersion,
			"serverInfo":      map[string]any{"name": "gasoline", "version": version},
			"capabilities":    map[string]any{"tools": map[string]any{}, "resources": map[string]any{}},
			"instructions":    serverInstructions,
		}
		// Error impossible: map contains only primitive types and nested maps
		resultJSON, _ := json.Marshal(result)
		sendFastResponse(req.ID, resultJSON)
		recordFastPathEvent(req.Method, true, 0)
		return true

	case "initialized":
		if req.ID != nil {
			sendFastResponse(req.ID, json.RawMessage(`{}`))
			recordFastPathEvent(req.Method, true, 0)
		}
		return true

	case "tools/list":
		result := map[string]any{"tools": toolsList}
		// Error impossible: map contains only serializable tool definitions
		resultJSON, _ := json.Marshal(result)
		sendFastResponse(req.ID, resultJSON)
		recordFastPathEvent(req.Method, true, 0)
		return true

	case "resources/list":
		result := MCPResourcesListResult{Resources: mcpResources()}
		resultJSON, _ := json.Marshal(result)
		sendFastResponse(req.ID, resultJSON)
		return true
	case "resources/templates/list":
		result := MCPResourceTemplatesListResult{ResourceTemplates: mcpResourceTemplates()}
		resultJSON, _ := json.Marshal(result)
		sendFastResponse(req.ID, resultJSON)
		return true
	case "resources/read":
		var params struct {
			URI string `json:"uri"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			recordFastPathResourceRead("", false, -32602)
			recordFastPathEvent(req.Method, false, -32602)
			sendFastError(req.ID, -32602, "Invalid params: "+err.Error())
			return true
		}
		canonicalURI, text, ok := resolveResourceContent(params.URI)
		if !ok {
			recordFastPathResourceRead(params.URI, false, -32002)
			recordFastPathEvent(req.Method, false, -32002)
			sendFastError(req.ID, -32002, "Resource not found: "+params.URI)
			return true
		}
		recordFastPathResourceRead(params.URI, true, 0)
		recordFastPathEvent(req.Method, true, 0)
		result := map[string]any{
			"contents": []map[string]any{
				{
					"uri":      canonicalURI,
					"mimeType": "text/markdown",
					"text":     text,
				},
			},
		}
		resultJSON, _ := json.Marshal(result)
		sendFastResponse(req.ID, resultJSON)
		return true
	}

	if staticResult, ok := fastPathResponses[req.Method]; ok {
		sendFastResponse(req.ID, json.RawMessage(staticResult))
		recordFastPathEvent(req.Method, true, 0)
		return true
	}

	return false
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

	toolsHandler := &ToolHandler{}
	toolsList := toolsHandler.ToolsList()

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
		debugf("request method=%s id=%v", req.Method, req.ID)

		// FAST PATH: Handle initialize and tools/list directly (no daemon needed)
		if handleFastPath(req, toolsList) {
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

// bridgeDoHTTP sends the raw JSON-RPC payload to the daemon and returns the HTTP response.
// The caller must provide a context that outlives the response body read — creating the
// context here with defer cancel() would cancel it before the caller reads resp.Body.
func bridgeDoHTTP(ctx context.Context, client *http.Client, endpoint string, line []byte) (*http.Response, error) {
	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(line)) // #nosec G704 -- endpoint is localhost-only serverURL/mcp from runBridgeMode // nosemgrep: go_injection_rule-ssrf -- localhost-only bridge forwarding to own server
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	return client.Do(httpReq) // #nosec G704 -- endpoint is localhost-only serverURL/mcp from runBridgeMode
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

// readMCPStdioMessage reads one MCP message from stdin.
// Supports both line-delimited JSON and Content-Length framed messages.
func readMCPStdioMessage(reader *bufio.Reader) ([]byte, error) {
	for {
		firstLineBytes, err := reader.ReadBytes('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				trimmed := strings.TrimSpace(string(firstLineBytes))
				if trimmed == "" {
					return nil, io.EOF
				}
				// Trailing non-empty bytes without newline: treat as final line-delimited message.
				return []byte(trimmed), nil
			}
			return nil, err
		}

		firstLine := strings.TrimSpace(string(firstLineBytes))
		if firstLine == "" {
			continue
		}

		if !strings.HasPrefix(strings.ToLower(firstLine), "content-length:") {
			// Line-delimited JSON (or malformed input handled by upstream JSON parse error path).
			debugf("stdio line message bytes=%d", len(firstLine))
			return []byte(firstLine), nil
		}

		parts := strings.SplitN(firstLine, ":", 2)
		if len(parts) != 2 {
			return []byte(firstLine), nil
		}
		contentLength, convErr := strconv.Atoi(strings.TrimSpace(parts[1]))
		if convErr != nil || contentLength < 0 || contentLength > maxPostBodySize {
			debugf("stdio framed header invalid length=%q", strings.TrimSpace(parts[1]))
			return []byte(firstLine), nil
		}

		// Consume remaining headers until blank line.
		for {
			headerLine, headerErr := reader.ReadBytes('\n')
			if headerErr != nil {
				if errors.Is(headerErr, io.EOF) {
					return nil, io.EOF
				}
				return nil, headerErr
			}
			if strings.TrimSpace(string(headerLine)) == "" {
				break
			}
		}

		payload := make([]byte, contentLength)
		if _, readErr := io.ReadFull(reader, payload); readErr != nil {
			debugf("stdio framed read error: %v", readErr)
			return nil, readErr
		}
		debugf("stdio framed message bytes=%d", len(payload))
		return bytes.TrimSpace(payload), nil
	}
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
