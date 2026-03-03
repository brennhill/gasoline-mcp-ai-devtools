// Purpose: Forwards JSON-RPC requests from bridge stdin to the daemon HTTP endpoint and writes responses to stdout.
// Why: Keeps the core HTTP-forwarding path isolated from transport/error helper machinery.
// Docs: docs/features/feature/bridge-restart/index.md

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/bridge"
)

// isSamplingResponse detects JSON-RPC responses to outgoing sampling requests.
// Responses have an ID but no Method.
func isSamplingResponse(req JSONRPCRequest) bool {
	return req.Method == "" && req.HasID()
}

// forwardSamplingResponse extracts text from a sampling response and POSTs it to /chat/response.
func forwardSamplingResponse(client *http.Client, endpoint string, rawJSON []byte) {
	var resp struct {
		ID     json.Number `json:"id"`
		Result struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rawJSON, &resp); err != nil {
		debugf("sampling response parse error: %v", err)
		return
	}

	requestID, err := resp.ID.Int64()
	if err != nil {
		debugf("sampling response ID not int64: %v", err)
		return
	}

	// Concatenate all text content blocks
	var sb strings.Builder
	for _, c := range resp.Result.Content {
		if c.Type == "text" {
			sb.WriteString(c.Text)
		}
	}
	if sb.Len() == 0 {
		return
	}

	reqBody := map[string]any{
		"request_id": requestID,
		"text":       sb.String(),
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return
	}

	chatURL := endpoint + "/chat/response"
	// Best-effort POST with 5s timeout — don't block indefinitely
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(ctx, "POST", chatURL, bytes.NewReader(payload))
	if err != nil {
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")
	postResp, err := client.Do(httpReq)
	if err != nil {
		debugf("chat response forward failed: %v", err)
		return
	}
	_ = postResp.Body.Close()
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
