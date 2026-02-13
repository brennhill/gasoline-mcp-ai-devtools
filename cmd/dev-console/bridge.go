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
		fast          = 10 * time.Second
		slow          = 35 * time.Second
		blockingPoll  = 65 * time.Second // annotation observe: server blocks up to 55s
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
		// Annotation command_result polling blocks server-side for up to 55s
		var args struct {
			What          string `json:"what"`
			CorrelationID string `json:"correlation_id"`
		}
		if json.Unmarshal(params.Arguments, &args) == nil &&
			args.What == "command_result" &&
			len(args.CorrelationID) > 4 && args.CorrelationID[:4] == "ann_" {
			return blockingPoll
		}
		return fast
	default:
		return fast
	}
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

	fmt.Fprintf(os.Stderr, "[gasoline] daemon not responding, respawning on port %d\n", s.port)

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
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
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
		fmt.Fprintf(os.Stderr, "[gasoline] daemon respawned successfully on port %d\n", s.port)
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
	if err := os.Stdout.Sync(); err != nil {
		fmt.Fprintf(os.Stderr, "[gasoline] warning: stdout.Sync failed: %v\n", err)
	}
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

	// Check if server is already running
	if isServerRunning(port) {
		state.ready = true
		close(state.readyCh)
	} else {
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

			args := []string{"--daemon", "--port", fmt.Sprintf("%d", port)}
			if stateDir := os.Getenv(statecfg.StateDirEnv); stateDir != "" {
				args = append(args, "--state-dir", stateDir)
			}
			if logFile != "" {
				args = append(args, "--log-file", logFile)
			}
			if maxEntries > 0 {
				args = append(args, "--max-entries", fmt.Sprintf("%d", maxEntries))
			}
			cmd := exec.Command(exe, args...) // #nosec G702 -- exe is our own binary path from os.Executable with fixed flags // nosemgrep: go.lang.security.audit.dangerous-exec-command.dangerous-exec-command, go_subproc_rule-subproc -- CLI bridge mode launches user-specified editor
			cmd.Stdout = nil
			cmd.Stderr = nil
			cmd.Stdin = nil
			if err := cmd.Start(); err != nil {
				state.mu.Lock()
				state.failed = true
				state.err = "Failed to start daemon: " + err.Error()
				state.mu.Unlock()
				close(state.failedCh)
				return
			}

			// Wait for server to be ready (max 4 seconds - fail fast)
			if waitForServer(port, 4*time.Second) {
				state.mu.Lock()
				state.ready = true
				state.mu.Unlock()
				close(state.readyCh)
			} else {
				state.mu.Lock()
				state.failed = true
				state.err = fmt.Sprintf("Daemon started but not responding on port %d after 4s", port)
				state.mu.Unlock()
				close(state.failedCh)
			}
		})
	}

	// Bridge stdio <-> HTTP with fast-start support
	bridgeStdioToHTTPFast(serverURL+"/mcp", state, port)
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
	"ping":                     `{}`,
	"prompts/list":             `{"prompts":[]}`,
	"resources/list":           `{"resources":[{"uri":"gasoline://guide","name":"Gasoline Usage Guide","description":"How to use Gasoline MCP tools for browser debugging","mimeType":"text/markdown"}]}`,
	"resources/templates/list": `{"resourceTemplates":[]}`,
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

// handleFastPath handles MCP methods that don't require the daemon.
// Returns true if the method was handled.
func handleFastPath(req JSONRPCRequest, toolsList []MCPTool) bool {
	switch req.Method {
	case "initialize":
		result := map[string]any{
			"protocolVersion": "2024-11-05",
			"serverInfo":      map[string]any{"name": "gasoline", "version": version},
			"capabilities":    map[string]any{"tools": map[string]any{}, "resources": map[string]any{}},
			"instructions":    serverInstructions,
		}
		// Error impossible: map contains only primitive types and nested maps
		resultJSON, _ := json.Marshal(result)
		sendFastResponse(req.ID, resultJSON)
		return true

	case "initialized":
		if req.ID != nil {
			sendFastResponse(req.ID, json.RawMessage(`{}`))
		}
		return true

	case "tools/list":
		result := map[string]any{"tools": toolsList}
		// Error impossible: map contains only serializable tool definitions
		resultJSON, _ := json.Marshal(result)
		sendFastResponse(req.ID, resultJSON)
		return true
	}

	if staticResult, ok := fastPathResponses[req.Method]; ok {
		sendFastResponse(req.ID, json.RawMessage(staticResult))
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
	scanner := bufio.NewScanner(os.Stdin)

	const maxScanTokenSize = 10 * 1024 * 1024
	buf := make([]byte, maxScanTokenSize)
	scanner.Buffer(buf, maxScanTokenSize)

	client := &http.Client{} // per-request timeouts via context

	var wg sync.WaitGroup
	responseSent := make(chan bool, 1)
	var responseOnce sync.Once
	signalResponseSent := func() {
		responseOnce.Do(func() { responseSent <- true })
	}

	toolsHandler := &ToolHandler{}
	toolsList := toolsHandler.ToolsList()

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req JSONRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			sendBridgeParseError(line, err)
			signalResponseSent()
			continue
		}

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

	bridgeShutdown(&wg, scanner, responseSent)
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
	defer cancel()

	resp, err := bridgeDoHTTP(ctx, client, endpoint, line)
	if err != nil && isConnectionError(err) && state != nil {
		// Daemon died — attempt respawn and retry with fresh context
		// (original context may have little time left after respawn delay).
		if state.respawnIfNeeded() {
			cancel()
			ctx, cancel = context.WithTimeout(context.Background(), timeout)
			resp, err = bridgeDoHTTP(ctx, client, endpoint, line)
		}
	}
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

// sendBridgeParseError sends a JSON-RPC parse error, extracting the ID from malformed JSON if possible.
func sendBridgeParseError(line []byte, err error) {
	var partial map[string]any
	var extractedID any = "error"
	if json.Unmarshal(line, &partial) == nil {
		if id, ok := partial["id"]; ok && id != nil {
			extractedID = id
		}
	}
	errResp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      extractedID,
		Error:   &JSONRPCError{Code: -32700, Message: "Parse error: " + err.Error()},
	}
	// Error impossible: simple struct with no circular refs or unsupported types
	respJSON, _ := json.Marshal(errResp)
	mcpStdoutMu.Lock()
	fmt.Println(string(respJSON))
	flushStdout()
	mcpStdoutMu.Unlock()
}

// bridgeShutdown waits for in-flight requests and performs clean shutdown.
func bridgeShutdown(wg *sync.WaitGroup, scanner *bufio.Scanner, responseSent chan bool) {
	wg.Wait()
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "[gasoline-bridge] ERROR: stdin scanner error: %v\n", err)
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
	scanner := bufio.NewScanner(os.Stdin)

	const maxScanTokenSize = 10 * 1024 * 1024 // 10MB
	buf := make([]byte, maxScanTokenSize)
	scanner.Buffer(buf, maxScanTokenSize)

	client := &http.Client{} // per-request timeouts via context

	var wg sync.WaitGroup
	responseSent := make(chan bool, 1)
	var responseOnce sync.Once
	signalResponseSent := func() {
		responseOnce.Do(func() { responseSent <- true })
	}

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req JSONRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			sendBridgeParseError(line, err)
			signalResponseSent()
			continue
		}

		timeout := toolCallTimeout(req)
		reqCopy, lineCopy := req, append([]byte(nil), line...)
		wg.Add(1)
		util.SafeGo(func() {
			defer wg.Done()
			bridgeForwardRequest(client, endpoint, reqCopy, lineCopy, timeout, nil, signalResponseSent)
		})
	}

	bridgeShutdown(&wg, scanner, responseSent)
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
