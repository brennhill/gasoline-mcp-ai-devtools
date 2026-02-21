// bridge_forward.go — HTTP forwarding, error responses, and restart handling for bridge mode.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"syscall"

	"github.com/dev-console/dev-console/internal/bridge"
	"github.com/dev-console/dev-console/internal/util"
)

type bridgeSessionStats struct {
	requests             int
	parseErrors          int
	invalidIDs           int
	fastPath             int
	forwarded            int
	methodNotFound       int
	starting             int
	lineFraming          int
	contentLengthFraming int
	lastMethod           string
}

type bridgeToolErrorOptions struct {
	ErrorCode     string
	Subsystem     string
	Reason        string
	Retryable     bool
	RetryAfterMs  int
	FallbackUsed  bool
	CorrelationID string
	Detail        string
}

// handleDaemonNotReady sends appropriate error responses when the daemon is not available.
func handleDaemonNotReady(req JSONRPCRequest, status string, signal func(), framing bridge.StdioFraming) {
	switch status {
	case "method_not_found":
		sendBridgeError(req.ID, -32601, "Method not found: "+req.Method, framing)
	case "starting":
		sendToolErrorWithOptions(req.ID, "Server is starting up. Please retry this tool call in 2 seconds.", framing, bridgeToolErrorOptions{
			ErrorCode:    "daemon_starting",
			Subsystem:    "bridge_startup",
			Reason:       "daemon_starting",
			Retryable:    true,
			RetryAfterMs: 2000,
		})
	default:
		sendToolErrorWithOptions(req.ID, status, framing, bridgeToolErrorOptions{
			ErrorCode:    "daemon_not_ready",
			Subsystem:    "bridge_startup",
			Reason:       "daemon_not_ready",
			Retryable:    true,
			RetryAfterMs: 2000,
		})
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
func bridgeForwardRequest(client *http.Client, endpoint string, req JSONRPCRequest, line []byte, timeout time.Duration, state *daemonState, signal func(), framing bridge.StdioFraming) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	activeCancel := cancel
	fallbackUsed := false

	resp, err := bridgeDoHTTP(ctx, client, endpoint, line)
	if err != nil && isConnectionError(err) && state != nil {
		fallbackUsed = true
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
		message := "Server connection error: " + err.Error()
		if req.Method == "tools/call" {
			sendToolErrorWithOptions(req.ID, message, framing, bridgeToolErrorOptions{
				ErrorCode:    "bridge_connection_error",
				Subsystem:    "bridge_http_forwarder",
				Reason:       "http_forward_failed",
				Retryable:    true,
				RetryAfterMs: 2000,
				FallbackUsed: fallbackUsed,
				Detail:       err.Error(),
			})
		} else {
			sendBridgeError(req.ID, -32603, message, framing)
		}
		signal()
		return
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxPostBodySize))
	_ = resp.Body.Close() //nolint:errcheck // best-effort cleanup
	if err != nil {
		message := "Failed to read response: " + err.Error()
		if req.Method == "tools/call" {
			sendToolErrorWithOptions(req.ID, message, framing, bridgeToolErrorOptions{
				ErrorCode:    "bridge_response_read_error",
				Subsystem:    "bridge_http_forwarder",
				Reason:       "response_read_failed",
				Retryable:    true,
				RetryAfterMs: 1000,
				FallbackUsed: fallbackUsed,
				Detail:       err.Error(),
			})
		} else {
			sendBridgeError(req.ID, -32603, message, framing)
		}
		signal()
		return
	}

	if resp.StatusCode == 204 {
		if req.HasID() {
			message := "Server returned no content for request with an id"
			if req.Method == "tools/call" {
				sendToolErrorWithOptions(req.ID, message, framing, bridgeToolErrorOptions{
					ErrorCode:    "bridge_unexpected_no_content",
					Subsystem:    "bridge_http_forwarder",
					Reason:       "unexpected_no_content",
					Retryable:    true,
					RetryAfterMs: 500,
					FallbackUsed: fallbackUsed,
				})
			} else {
				sendBridgeError(req.ID, -32603, message, framing)
			}
		}
		signal()
		return
	}

	if resp.StatusCode != 200 {
		message := fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body))
		if req.Method == "tools/call" {
			retryable := resp.StatusCode >= 500
			retryAfter := 0
			if retryable {
				retryAfter = 1000
			}
			sendToolErrorWithOptions(req.ID, message, framing, bridgeToolErrorOptions{
				ErrorCode:    "bridge_http_status_error",
				Subsystem:    "bridge_http_forwarder",
				Reason:       "http_status_error",
				Retryable:    retryable,
				RetryAfterMs: retryAfter,
				FallbackUsed: fallbackUsed,
				Detail:       fmt.Sprintf("status_code=%d", resp.StatusCode),
			})
		} else {
			sendBridgeError(req.ID, -32603, message, framing)
		}
		signal()
		return
	}

	if req.HasID() && len(bytes.TrimSpace(body)) == 0 {
		message := "Server returned an empty body for request with an id"
		if req.Method == "tools/call" {
			sendToolErrorWithOptions(req.ID, message, framing, bridgeToolErrorOptions{
				ErrorCode:    "bridge_empty_response",
				Subsystem:    "bridge_http_forwarder",
				Reason:       "empty_response",
				Retryable:    true,
				RetryAfterMs: 500,
				FallbackUsed: fallbackUsed,
			})
		} else {
			sendBridgeError(req.ID, -32603, message, framing)
		}
		signal()
		return
	}

	if req.HasID() && !json.Valid(body) {
		message := "Server returned invalid JSON response"
		if req.Method == "tools/call" {
			sendToolErrorWithOptions(req.ID, message, framing, bridgeToolErrorOptions{
				ErrorCode:    "bridge_invalid_response",
				Subsystem:    "bridge_http_forwarder",
				Reason:       "invalid_json_response",
				Retryable:    true,
				RetryAfterMs: 1000,
				FallbackUsed: fallbackUsed,
			})
		} else {
			sendBridgeError(req.ID, -32603, message, framing)
		}
		signal()
		return
	}

	writeMCPPayload(body, framing)
	signal()
}

// sendBridgeParseError sends a JSON-RPC parse error (id must be null per spec).
func sendBridgeParseError(_ []byte, err error, framing bridge.StdioFraming) {
	errResp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      nil, // JSON-RPC: parse errors must have null id
		Error:   &JSONRPCError{Code: -32700, Message: "Parse error: " + err.Error()},
	}
	// Error impossible: simple struct with no circular refs or unsupported types
	respJSON, _ := json.Marshal(errResp)
	writeMCPPayload(respJSON, framing)
}

// readMCPStdioMessage delegates to internal/bridge for stdio message parsing.
func readMCPStdioMessage(reader *bufio.Reader) ([]byte, bridge.StdioFraming, error) {
	return bridge.ReadStdioMessageWithMode(reader, maxPostBodySize)
}

// bridgeShutdown waits for in-flight requests and performs clean shutdown.
func bridgeShutdown(wg *sync.WaitGroup, readErr error, responseSent chan bool, stats *bridgeSessionStats) {
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

	if stats != nil {
		reason := "stdin_eof"
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			reason = "stdin_read_error"
		}
		extra := map[string]any{
			"reason":                 reason,
			"requests":               stats.requests,
			"parse_errors":           stats.parseErrors,
			"invalid_ids":            stats.invalidIDs,
			"fast_path":              stats.fastPath,
			"forwarded":              stats.forwarded,
			"method_not_found":       stats.methodNotFound,
			"starting_retries":       stats.starting,
			"line_framing":           stats.lineFraming,
			"content_length_framing": stats.contentLengthFraming,
			"last_method":            stats.lastMethod,
		}
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			extra["read_error"] = readErr.Error()
		}
		_ = appendExitDiagnostic("bridge_exit", extra)
	}
}

// bridgeStdioToHTTP forwards JSON-RPC messages between stdin/stdout and HTTP endpoint
func bridgeStdioToHTTP(endpoint string) {
	reader := bufio.NewReaderSize(os.Stdin, 64*1024)

	client := &http.Client{} // per-request timeouts via context

	var wg sync.WaitGroup
	responseSent := make(chan bool, 1)
	var responseOnce sync.Once
	stats := &bridgeSessionStats{}
	signalResponseSent := func() {
		responseOnce.Do(func() { responseSent <- true })
	}

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

		timeout := toolCallTimeout(req)
		reqCopy, lineCopy := req, append([]byte(nil), line...)
		stats.forwarded++
		wg.Add(1)
		util.SafeGo(func() {
			defer wg.Done()
			bridgeForwardRequest(client, endpoint, reqCopy, lineCopy, timeout, nil, signalResponseSent, framing)
		})
	}

	bridgeShutdown(&wg, readErr, responseSent, stats)
}

// sendBridgeError sends a JSON-RPC error response to stdout
func sendBridgeError(id any, code int, message string, framing bridge.StdioFraming) {
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
	writeMCPPayload(respJSON, framing)
}

// sendToolError sends a tool result with isError: true (soft error, not protocol error)
// This tells the LLM the tool ran but returned an error, allowing it to retry.
func sendToolError(id any, message string, framing bridge.StdioFraming) {
	sendToolErrorWithOptions(id, message, framing, bridgeToolErrorOptions{})
}

func sendToolErrorWithOptions(id any, message string, framing bridge.StdioFraming, opts bridgeToolErrorOptions) {
	errorCode := opts.ErrorCode
	if errorCode == "" {
		errorCode = "bridge_tool_error"
	}
	subsystem := opts.Subsystem
	if subsystem == "" {
		subsystem = "bridge"
	}
	reason := opts.Reason
	if reason == "" {
		reason = "tool_error"
	}
	correlationID := opts.CorrelationID
	if correlationID == "" {
		correlationID = bridgeRequestIDString(id)
	}

	result := map[string]any{
		"content": []map[string]any{
			{"type": "text", "text": message},
		},
		"isError":        true,
		"status":         "error",
		"error_code":     errorCode,
		"subsystem":      subsystem,
		"reason":         reason,
		"retryable":      opts.Retryable,
		"fallback_used":  opts.FallbackUsed,
		"correlation_id": correlationID,
	}
	if opts.Retryable && opts.RetryAfterMs > 0 {
		result["retry_after_ms"] = opts.RetryAfterMs
	}
	if opts.Detail != "" {
		result["detail"] = opts.Detail
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
	writeMCPPayload(respJSON, framing)
}

func bridgeRequestIDString(id any) string {
	switch v := id.(type) {
	case nil:
		return ""
	case string:
		return v
	case json.RawMessage:
		if len(v) == 0 {
			return ""
		}
		var s string
		if err := json.Unmarshal(v, &s); err == nil {
			return s
		}
		var n float64
		if err := json.Unmarshal(v, &n); err == nil {
			return strconv.FormatFloat(n, 'f', -1, 64)
		}
		return strings.TrimSpace(string(v))
	case fmt.Stringer:
		return v.String()
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	default:
		return fmt.Sprintf("%v", v)
	}
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
func handleBridgeRestart(req JSONRPCRequest, state *daemonState, port int, framing bridge.StdioFraming) bool {
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
	state.resetSignalsLocked()
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
	sendFastResponse(req.ID, toolResultJSON, framing)
	return true
}
