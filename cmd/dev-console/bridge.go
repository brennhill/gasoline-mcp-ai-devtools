// Bridge mode: stdio-to-HTTP transport for MCP
// Spawns persistent HTTP server daemon if not running,
// forwards JSON-RPC messages between stdio (MCP client) and HTTP (server).
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"sync"
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

// isServerRunning checks if a server is healthy on the given port via HTTP health check.
// This catches zombie servers that accept TCP connections but don't respond to HTTP.
func isServerRunning(port int) bool {
	client := &http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/health", port))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
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
		Timeout: 5 * time.Second, // Fast failure for MCP clients
	}

	// Track in-flight HTTP requests to ensure all responses sent before exit
	var wg sync.WaitGroup

	// Exit gate: Prevent process exit until at least one response has been sent
	// This ensures the parent process receives the response before we exit
	responseSent := make(chan bool, 1)
	var responseOnce sync.Once
	signalResponseSent := func() {
		responseOnce.Do(func() {
			responseSent <- true
		})
	}

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		// Validate it's JSON-RPC before forwarding
		var req JSONRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			// Try to extract ID from malformed JSON for better error response
			var partial map[string]any
			var extractedID any = "error"  // Fallback ID for parse errors (never null - Cursor rejects it)
			if json.Unmarshal(line, &partial) == nil {
				if id, ok := partial["id"]; ok && id != nil {
					extractedID = id  // Use whatever ID was in the request
				}
			}

			// Send parse error with extracted or fallback ID
			errResp := JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      extractedID,
				Error: &JSONRPCError{
					Code:    -32700,
					Message: "Parse error: " + err.Error(),
				},
			}
			respJSON, _ := json.Marshal(errResp)
			fmt.Println(string(respJSON))
			os.Stdout.Sync()
			signalResponseSent()
			continue
		}

		// Process request in current goroutine to maintain order
		wg.Add(1)

		// Forward to HTTP server
		httpReq, err := http.NewRequest("POST", endpoint, bytes.NewReader(line))
		if err != nil {
			sendBridgeError(req.ID, -32603, "Bridge error: "+err.Error())
			os.Stdout.Sync()
			signalResponseSent()
			wg.Done()
			continue
		}

		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(httpReq)
		if err != nil {
			sendBridgeError(req.ID, -32603, "Server connection error: "+err.Error())
			os.Stdout.Sync()
			signalResponseSent()
			wg.Done()
			continue
		}

		// Read response
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()

		if err != nil {
			sendBridgeError(req.ID, -32603, "Failed to read response: "+err.Error())
			os.Stdout.Sync()
			signalResponseSent()
			wg.Done()
			continue
		}

		// Handle 204 No Content (notification response - no output needed)
		if resp.StatusCode == 204 {
			// Notification was processed, no response to forward
			// Don't signal responseSent - notifications don't count as responses
			wg.Done()
			continue
		}

		if resp.StatusCode != 200 {
			sendBridgeError(req.ID, -32603, fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body)))
			os.Stdout.Sync()
			signalResponseSent()
			wg.Done()
			continue
		}

		// Forward response to stdout
		// Use Print not Println - HTTP response already has trailing newline from json.Encoder.Encode()
		fmt.Print(string(body))
		os.Stdout.Sync()  // Flush immediately
		signalResponseSent()  // Signal that response was sent
		wg.Done()
	}

	// CRITICAL: Wait for all in-flight requests to complete before exiting
	wg.Wait()

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "[gasoline-bridge] ERROR: stdin scanner error: %v\n", err)
	}

	// EXIT GATE: Wait for at least one response to be sent before allowing exit
	// This prevents the process from exiting before the parent reads stdout
	select {
	case <-responseSent:
		// At least one response was sent and flushed - safe to exit
	case <-time.After(5 * time.Second):
		// Timeout fallback - exit anyway to avoid hanging forever
		fmt.Fprintf(os.Stderr, "[gasoline-bridge] WARNING: No response sent within 5 seconds\n")
	}

	// CRITICAL: Final flush and give OS time to send buffered data to parent process
	os.Stdout.Sync()
	time.Sleep(100 * time.Millisecond)  // Allow OS to flush pipe to parent

	// Quiet mode: Bridge shutdown is silent (normal operation, not an error)
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
	respJSON, _ := json.Marshal(errResp)
	fmt.Println(string(respJSON))
}
