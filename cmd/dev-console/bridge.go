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
	"strings"
	"sync"
	"time"
)

// daemonState tracks the state of daemon startup for fast-start mode
type daemonState struct {
	ready    bool
	failed   bool
	err      string
	mu       sync.Mutex
	readyCh  chan struct{}
	failedCh chan struct{}
}

// flushStdout syncs stdout and logs any errors (best-effort)
func flushStdout() {
	if err := os.Stdout.Sync(); err != nil {
		fmt.Fprintf(os.Stderr, "[gasoline] warning: stdout.Sync failed: %v\n", err)
	}
}

// runBridgeMode bridges stdio (from MCP client) to HTTP (to persistent server)
// Uses fast-start: responds to initialize/tools/list immediately while spawning daemon async.
func runBridgeMode(port int) {
	serverURL := fmt.Sprintf("http://127.0.0.1:%d", port)

	// Track daemon state with proper failure handling
	state := &daemonState{
		readyCh:  make(chan struct{}),
		failedCh: make(chan struct{}),
	}

	// Check if server is already running
	if isServerRunning(port) {
		state.ready = true
		close(state.readyCh)
	} else {
		// Spawn daemon in background (don't block on it)
		go func() {
			exe, err := os.Executable()
			if err != nil {
				state.mu.Lock()
				state.failed = true
				state.err = "Cannot find executable: " + err.Error()
				state.mu.Unlock()
				close(state.failedCh)
				return
			}

			cmd := exec.Command(exe, "--daemon", "--port", fmt.Sprintf("%d", port))
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
		}()
	}

	// Bridge stdio <-> HTTP with fast-start support
	bridgeStdioToHTTPFast(serverURL+"/mcp", state, port)
}

// isServerRunning checks if a server is healthy on the given port via HTTP health check.
// This catches zombie servers that accept TCP connections but don't respond to HTTP.
func isServerRunning(port int) bool {
	client := &http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/health", port))
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }() //nolint:errcheck -- best-effort cleanup
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
				_ = resp.Body.Close() //nolint:errcheck -- best-effort cleanup
				return true
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

// bridgeStdioToHTTPFast forwards JSON-RPC with fast-start: responds to initialize/tools/list
// immediately while daemon starts in background. Only blocks on tools/call.
func bridgeStdioToHTTPFast(endpoint string, state *daemonState, port int) {
	scanner := bufio.NewScanner(os.Stdin)

	const maxScanTokenSize = 10 * 1024 * 1024
	buf := make([]byte, maxScanTokenSize)
	scanner.Buffer(buf, maxScanTokenSize)

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	var wg sync.WaitGroup
	responseSent := make(chan bool, 1)
	var responseOnce sync.Once
	signalResponseSent := func() {
		responseOnce.Do(func() {
			responseSent <- true
		})
	}

	// Get static tools list for fast response (ToolsList doesn't use receiver fields)
	toolsHandler := &ToolHandler{}
	toolsList := toolsHandler.ToolsList()

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req JSONRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
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
			respJSON, _ := json.Marshal(errResp)
			fmt.Println(string(respJSON))
			flushStdout()
			signalResponseSent()
			continue
		}

		// FAST PATH: Handle initialize and tools/list directly (no daemon needed)
		switch req.Method {
		case "initialize":
			// Respond immediately with capabilities
			result := map[string]any{
				"protocolVersion": "2024-11-05",
				"serverInfo":      map[string]any{"name": "gasoline", "version": version},
				"capabilities":    map[string]any{"tools": map[string]any{}, "resources": map[string]any{}},
			}
			resultJSON, _ := json.Marshal(result)
			resp := JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: resultJSON}
			respJSON, _ := json.Marshal(resp)
			fmt.Println(string(respJSON))
			flushStdout()
			signalResponseSent()
			continue

		case "initialized":
			// Notification - no response needed, but some clients send with ID
			if req.ID != nil {
				resp := JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(`{}`)}
				respJSON, _ := json.Marshal(resp)
				fmt.Println(string(respJSON))
				flushStdout()
				signalResponseSent()
			}
			continue

		case "tools/list":
			// Respond immediately with static tools schema
			result := map[string]any{"tools": toolsList}
			resultJSON, _ := json.Marshal(result)
			resp := JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: resultJSON}
			respJSON, _ := json.Marshal(resp)
			fmt.Println(string(respJSON))
			flushStdout()
			signalResponseSent()
			continue

		case "ping":
			resp := JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(`{}`)}
			respJSON, _ := json.Marshal(resp)
			fmt.Println(string(respJSON))
			flushStdout()
			signalResponseSent()
			continue

		case "prompts/list":
			resp := JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(`{"prompts":[]}`)}
			respJSON, _ := json.Marshal(resp)
			fmt.Println(string(respJSON))
			flushStdout()
			signalResponseSent()
			continue

		case "resources/list":
			resp := JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(`{"resources":[]}`)}
			respJSON, _ := json.Marshal(resp)
			fmt.Println(string(respJSON))
			flushStdout()
			signalResponseSent()
			continue

		case "resources/templates/list":
			resp := JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(`{"resourceTemplates":[]}`)}
			respJSON, _ := json.Marshal(resp)
			fmt.Println(string(respJSON))
			flushStdout()
			signalResponseSent()
			continue
		}

		// SLOW PATH: Check daemon status for tools/call and other methods
		state.mu.Lock()
		isReady := state.ready
		isFailed := state.failed
		failErr := state.err
		state.mu.Unlock()

		if isFailed {
			// Daemon failed to start - return tool error with details and suggestions
			suggestion := fmt.Sprintf("Server failed to start: %s. ", failErr)
			if strings.Contains(failErr, "port") || strings.Contains(failErr, "bind") || strings.Contains(failErr, "address") {
				suggestion += fmt.Sprintf("Port may be in use. Try: npx gasoline-mcp --port %d", port+1)
			} else {
				suggestion += "Try: npx gasoline-mcp --doctor"
			}
			sendToolError(req.ID, suggestion)
			flushStdout()
			signalResponseSent()
			continue
		}

		if !isReady {
			// Server is still starting - tell LLM to retry
			sendToolError(req.ID, "Server is starting up. Please retry this tool call in 2 seconds.")
			flushStdout()
			signalResponseSent()
			continue
		}

		// Forward to HTTP server
		wg.Add(1)
		httpReq, err := http.NewRequest("POST", endpoint, bytes.NewReader(line))
		if err != nil {
			sendBridgeError(req.ID, -32603, "Bridge error: "+err.Error())
			flushStdout()
			signalResponseSent()
			wg.Done()
			continue
		}
		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(httpReq)
		if err != nil {
			sendBridgeError(req.ID, -32603, "Server connection error: "+err.Error())
			flushStdout()
			signalResponseSent()
			wg.Done()
			continue
		}

		body, err := io.ReadAll(io.LimitReader(resp.Body, maxPostBodySize))
		_ = resp.Body.Close() //nolint:errcheck -- best-effort cleanup
		if err != nil {
			sendBridgeError(req.ID, -32603, "Failed to read response: "+err.Error())
			flushStdout()
			signalResponseSent()
			wg.Done()
			continue
		}

		if resp.StatusCode == 204 {
			wg.Done()
			continue
		}

		if resp.StatusCode != 200 {
			sendBridgeError(req.ID, -32603, fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body)))
			flushStdout()
			signalResponseSent()
			wg.Done()
			continue
		}

		fmt.Print(string(body))
		flushStdout()
		signalResponseSent()
		wg.Done()
	}

	wg.Wait()
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "[gasoline-bridge] ERROR: stdin scanner error: %v\n", err)
	}

	select {
	case <-responseSent:
	case <-time.After(5 * time.Second):
	}

	flushStdout()
	time.Sleep(100 * time.Millisecond)
}

// bridgeStdioToHTTP forwards JSON-RPC messages between stdin/stdout and HTTP endpoint
func bridgeStdioToHTTP(endpoint string) {
	scanner := bufio.NewScanner(os.Stdin)

	// Increase buffer size for large messages (screenshots, etc.)
	const maxScanTokenSize = 10 * 1024 * 1024 // 10MB
	buf := make([]byte, maxScanTokenSize)
	scanner.Buffer(buf, maxScanTokenSize)

	client := &http.Client{
		Timeout: 35 * time.Second, // Must exceed longest handler wait (a11y: 30s)
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
			flushStdout()
			signalResponseSent()
			continue
		}

		// Process request in current goroutine to maintain order
		wg.Add(1)

		// Forward to HTTP server
		httpReq, err := http.NewRequest("POST", endpoint, bytes.NewReader(line))
		if err != nil {
			sendBridgeError(req.ID, -32603, "Bridge error: "+err.Error())
			flushStdout()
			signalResponseSent()
			wg.Done()
			continue
		}

		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(httpReq)
		if err != nil {
			sendBridgeError(req.ID, -32603, "Server connection error: "+err.Error())
			flushStdout()
			signalResponseSent()
			wg.Done()
			continue
		}

		// Read response (limit size to prevent memory exhaustion)
		body, err := io.ReadAll(io.LimitReader(resp.Body, maxPostBodySize))
		_ = resp.Body.Close() //nolint:errcheck -- best-effort cleanup

		if err != nil {
			sendBridgeError(req.ID, -32603, "Failed to read response: "+err.Error())
			flushStdout()
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
			flushStdout()
			signalResponseSent()
			wg.Done()
			continue
		}

		// Forward response to stdout
		// Use Print not Println - HTTP response already has trailing newline from json.Encoder.Encode()
		fmt.Print(string(body))
		flushStdout()  // Flush immediately
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
	flushStdout()
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

// sendToolError sends a tool result with isError: true (soft error, not protocol error)
// This tells the LLM the tool ran but returned an error, allowing it to retry.
func sendToolError(id any, message string) {
	result := map[string]any{
		"content": []map[string]any{
			{"type": "text", "text": message},
		},
		"isError": true,
	}
	resultJSON, _ := json.Marshal(result)
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  resultJSON,
	}
	respJSON, _ := json.Marshal(resp)
	fmt.Println(string(respJSON))
}
