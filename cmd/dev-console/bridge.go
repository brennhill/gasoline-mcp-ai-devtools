package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"time"
)

// runBridgeMode bridges stdio (from MCP client) to HTTP (to persistent server)
func runBridgeMode(port int) {
	serverURL := fmt.Sprintf("http://127.0.0.1:%d", port)

	// Check if server is already running
	if !isServerRunning(port) {
		fmt.Fprintf(os.Stderr, "[gasoline-bridge] Server not running, starting daemon...\n")

		// Start daemon in background
		exe, err := os.Executable()
		if err != nil {
			fmt.Fprintf(os.Stderr, "[gasoline-bridge] ERROR: cannot get executable path: %v\n", err)
			os.Exit(1)
		}

		cmd := exec.Command(exe, "--port", fmt.Sprintf("%d", port))
		cmd.Stdout = os.Stderr // Redirect server logs to stderr
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "[gasoline-bridge] ERROR: failed to start daemon: %v\n", err)
			os.Exit(1)
		}

		// Wait for server to be ready (max 10 seconds)
		if !waitForServer(port, 10*time.Second) {
			fmt.Fprintf(os.Stderr, "[gasoline-bridge] ERROR: server failed to start within 10 seconds\n")
			os.Exit(1)
		}

		fmt.Fprintf(os.Stderr, "[gasoline-bridge] Daemon started successfully\n")
	} else {
		fmt.Fprintf(os.Stderr, "[gasoline-bridge] Connecting to existing daemon on port %d\n", port)
	}

	// Bridge stdio <-> HTTP
	bridgeStdioToHTTP(serverURL + "/mcp")
}

// isServerRunning checks if a server is listening on the given port
func isServerRunning(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 500*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// waitForServer waits for the server to start accepting connections
func waitForServer(port int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if isServerRunning(port) {
			// Additional check: try to hit the health endpoint
			resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/health", port))
			if err == nil && resp.StatusCode == 200 {
				resp.Body.Close()
				return true
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

// bridgeStdioToHTTP forwards JSON-RPC messages between stdin/stdout and HTTP endpoint
func bridgeStdioToHTTP(endpoint string) {
	scanner := bufio.NewScanner(os.Stdin)

	// Increase buffer size for large messages (screenshots, etc.)
	const maxScanTokenSize = 10 * 1024 * 1024 // 10MB
	buf := make([]byte, maxScanTokenSize)
	scanner.Buffer(buf, maxScanTokenSize)

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		// Validate it's JSON-RPC before forwarding
		var req JSONRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			// Send parse error back to client
			errResp := JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      nil,
				Error: &JSONRPCError{
					Code:    -32700,
					Message: "Parse error: " + err.Error(),
				},
			}
			respJSON, _ := json.Marshal(errResp)
			fmt.Println(string(respJSON))
			continue
		}

		// Forward to HTTP server
		httpReq, err := http.NewRequest("POST", endpoint, bytes.NewReader(line))
		if err != nil {
			sendBridgeError(req.ID, -32603, "Bridge error: "+err.Error())
			continue
		}

		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(httpReq)
		if err != nil {
			sendBridgeError(req.ID, -32603, "Server connection error: "+err.Error())
			continue
		}

		// Read response
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()

		if err != nil {
			sendBridgeError(req.ID, -32603, "Failed to read response: "+err.Error())
			continue
		}

		if resp.StatusCode != 200 {
			sendBridgeError(req.ID, -32603, fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body)))
			continue
		}

		// Forward response to stdout
		fmt.Println(string(body))
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "[gasoline-bridge] ERROR: stdin scanner error: %v\n", err)
	}

	fmt.Fprintf(os.Stderr, "[gasoline-bridge] stdin closed, bridge shutting down\n")
}

// sendBridgeError sends a JSON-RPC error response to stdout
func sendBridgeError(id interface{}, code int, message string) {
	errResp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &JSONRPCError{
			Code:    code,
			Message: message,
		},
	}
	respJSON, _ := json.Marshal(errResp)
	fmt.Println(string(respJSON))
}
