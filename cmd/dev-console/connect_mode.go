// connect_mode.go — Connect mode for multi-client MCP support.
// Enables multiple Claude Code sessions to share a single server.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/dev-console/dev-console/internal/session"
)

// runConnectMode connects to an existing Gasoline server as an MCP client.
// This enables multiple Claude Code sessions to share a single server.
// The client ID is sent via X-Gasoline-Client header for state isolation.
func runConnectMode(port int, clientID string, cwd string) {
	serverURL := fmt.Sprintf("http://127.0.0.1:%d", port)

	// Check if server is running
	healthURL := serverURL + "/health"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[gasoline] Failed to create health check request: %v\n", err)
		os.Exit(1)
	}

	resp, err := http.DefaultClient.Do(req) // #nosec G107 -- localhost URL constructed from port flag
	if err != nil {
		fmt.Fprintf(os.Stderr, "[gasoline] Cannot connect to server at %s: %v\n", serverURL, err)
		fmt.Fprintf(os.Stderr, "[gasoline] Start a server first: gasoline --server --port %d\n", port)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "[gasoline] Server health check failed: %d\n", resp.StatusCode)
		os.Exit(1)
	}

	// Register this client with the server
	registerURL := serverURL + "/clients"
	regBody, _ := json.Marshal(map[string]string{"cwd": cwd})

	regCtx, regCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer regCancel()

	regReq, err := http.NewRequestWithContext(regCtx, "POST", registerURL, strings.NewReader(string(regBody)))
	if err != nil {
		fmt.Fprintf(os.Stderr, "[gasoline] Warning: could not create registration request: %v\n", err)
	} else {
		regReq.Header.Set("Content-Type", "application/json")
		regReq.Header.Set("X-Gasoline-Client", clientID)
		regResp, err := http.DefaultClient.Do(regReq)
		if err != nil {
			// Server might not have /clients endpoint yet (backwards compat)
			fmt.Fprintf(os.Stderr, "[gasoline] Warning: could not register client: %v\n", err)
		} else {
			_ = regResp.Body.Close() //nolint:errcheck -- best-effort cleanup after client registration
		}
	}

	fmt.Fprintf(os.Stderr, "[gasoline] Connected to %s (client: %s)\n", serverURL, clientID)

	// Run MCP protocol over stdin/stdout, forwarding to HTTP server
	scanner := bufio.NewScanner(os.Stdin)
	const maxScanTokenSize = 10 * 1024 * 1024
	buf := make([]byte, maxScanTokenSize)
	scanner.Buffer(buf, maxScanTokenSize)

	mcpURL := serverURL + "/mcp"

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		// Forward request to server with client ID header
		forwardCtx, forwardCancel := context.WithTimeout(context.Background(), 30*time.Second)
		req, err := http.NewRequestWithContext(forwardCtx, "POST", mcpURL, strings.NewReader(line))
		if err != nil {
			forwardCancel()
			sendMCPError(nil, -32603, "Internal error: "+err.Error())
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Gasoline-Client", clientID)

		resp, err := http.DefaultClient.Do(req)
		forwardCancel()
		if err != nil {
			// Try to extract request ID for error response
			var jsonReq JSONRPCRequest
			if json.Unmarshal([]byte(line), &jsonReq) == nil {
				sendMCPError(jsonReq.ID, -32603, "Server connection error: "+err.Error())
			} else {
				sendMCPError(nil, -32603, "Server connection error: "+err.Error())
			}
			continue
		}

		// Stream response back to stdout
		var respData json.RawMessage
		if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
			_ = resp.Body.Close() //nolint:errcheck -- best-effort cleanup after decode error
			var jsonReq JSONRPCRequest
			if json.Unmarshal([]byte(line), &jsonReq) == nil {
				sendMCPError(jsonReq.ID, -32603, "Invalid server response")
			}
			continue
		}
		_ = resp.Body.Close() //nolint:errcheck -- best-effort cleanup after successful decode

		fmt.Println(string(respData))
	}

	// stdin closed - unregister and exit
	if clientID != "" {
		unregURL := serverURL + "/clients/" + clientID
		unregCtx, unregCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer unregCancel()

		unregReq, err := http.NewRequestWithContext(unregCtx, "DELETE", unregURL, nil)
		if err == nil {
			unregReq.Header.Set("X-Gasoline-Client", clientID)
			resp, err := http.DefaultClient.Do(unregReq)
			if err == nil {
				_ = resp.Body.Close() //nolint:errcheck -- best-effort cleanup after unregister
			}
		}
	}

	fmt.Fprintf(os.Stderr, "[gasoline] Disconnected from %s\n", serverURL)
}

// sendMCPError sends a JSON-RPC error response to stdout (used in connect mode)
func sendMCPError(id any, code int, message string) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &JSONRPCError{
			Code:    code,
			Message: message,
		},
	}
	respJSON, _ := json.Marshal(resp)
	fmt.Println(string(respJSON))
}

// DeriveClientID is re-exported from session package for use in main
var DeriveClientID = session.DeriveClientID
