// client.go — JSON-RPC 2.0 client for communicating with gasoline-mcp.
// Sends MCP tool calls over HTTP to the running server instance.
package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
	"time"
)

// Client connects to a running gasoline-mcp server via HTTP.
type Client struct {
	baseURL    string
	httpClient *http.Client
	requestID  atomic.Int64
}

// NewClient creates a new MCP client pointing at the given server URL.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// NewClientWithPort creates a new MCP client for localhost on the given port.
func NewClientWithPort(port int) *Client {
	return NewClient(fmt.Sprintf("http://127.0.0.1:%d", port))
}

// Initialize performs the MCP initialize handshake with the server.
func (c *Client) Initialize() error {
	initReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      c.nextID(),
		Method:  "initialize",
		Params: json.RawMessage(`{
			"protocolVersion": "2024-11-05",
			"capabilities": {},
			"clientInfo": {"name": "gasoline-cmd", "version": "1.0.0"}
		}`),
	}

	body, err := json.Marshal(initReq)
	if err != nil {
		return fmt.Errorf("marshal initialize request: %w", err)
	}

	resp, err := c.doPost(body)
	if err != nil {
		return fmt.Errorf("initialize: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("initialize: HTTP %d", resp.StatusCode)
	}

	var rpcResp JSONRPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return fmt.Errorf("initialize: decode response: %w", err)
	}

	if rpcResp.Error != nil {
		return fmt.Errorf("initialize: server error: %s", rpcResp.Error.Message)
	}

	// Send initialized notification (no response expected)
	notif := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	}
	notifBody, err := json.Marshal(notif)
	if err != nil {
		return fmt.Errorf("marshal initialized notification: %w", err)
	}

	notifResp, err := c.doPost(notifBody)
	if err != nil {
		// Non-fatal: some servers may not handle this
		return nil
	}
	notifResp.Body.Close()

	return nil
}

// CallTool sends a tools/call request to the MCP server.
func (c *Client) CallTool(tool string, arguments map[string]any) (*MCPToolResult, error) {
	body, err := c.buildRequest(tool, arguments)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	resp, err := c.doPost(body)
	if err != nil {
		return nil, fmt.Errorf("call tool %q: %w", tool, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var rpcResp JSONRPCResponse
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("server error [%d]: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	var result MCPToolResult
	if err := json.Unmarshal(rpcResp.Result, &result); err != nil {
		return nil, fmt.Errorf("decode tool result: %w", err)
	}

	return &result, nil
}

// HealthCheck attempts to connect to the server and verify it is responsive.
func (c *Client) HealthCheck() bool {
	req, err := http.NewRequest("GET", c.baseURL+"/health", nil)
	if err != nil {
		return false
	}

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// buildRequest creates the JSON-RPC request body for a tools/call.
func (c *Client) buildRequest(tool string, arguments map[string]any) ([]byte, error) {
	argsJSON, err := json.Marshal(arguments)
	if err != nil {
		return nil, fmt.Errorf("marshal arguments: %w", err)
	}

	params := struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}{
		Name:      tool,
		Arguments: argsJSON,
	}

	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("marshal params: %w", err)
	}

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      c.nextID(),
		Method:  "tools/call",
		Params:  paramsJSON,
	}

	return json.Marshal(req)
}

// doPost sends a POST request to the MCP endpoint.
func (c *Client) doPost(body []byte) (*http.Response, error) {
	url := c.baseURL + "/mcp"
	return c.httpClient.Post(url, "application/json", bytes.NewReader(body))
}

// nextID returns a monotonically increasing request ID.
func (c *Client) nextID() int64 {
	return c.requestID.Add(1)
}
