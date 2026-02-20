// bridge_forward.go — HTTP forwarding, error responses, and restart handling for bridge mode.
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
	"sync"
	"time"

	"syscall"

	"github.com/dev-console/dev-console/internal/bridge"
	"github.com/dev-console/dev-console/internal/util"
)

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
