// cli.go — CLI mode entry point for direct tool invocation.
// Allows: gasoline observe errors --limit 50
// Talks to the daemon over HTTP (same /mcp endpoint as the MCP bridge).
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/dev-console/dev-console/internal/util"
)

// cliToolNames lists valid tool names for CLI mode detection.
var cliToolNames = map[string]bool{
	"observe":   true,
	"generate":  true,
	"configure": true,
	"interact":  true,
}

// cliConfig holds resolved CLI configuration.
type cliConfig struct {
	Port    int
	Format  string
	Timeout int // milliseconds
}

// IsCLIMode returns true if the first argument is a known tool name.
func IsCLIMode(args []string) bool {
	if len(args) == 0 {
		return false
	}
	return cliToolNames[args[0]]
}

// runCLIMode is the main CLI flow. Returns exit code.
func runCLIMode(args []string) int {
	cfg, remaining := resolveCLIConfig(args)

	if len(remaining) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: gasoline <tool> <action> [flags]\n")
		fmt.Fprintf(os.Stderr, "  Tools: observe, generate, configure, interact\n")
		fmt.Fprintf(os.Stderr, "  Example: gasoline observe errors --limit 50\n")
		return 2
	}

	tool := remaining[0]
	action := remaining[1]
	toolArgs := remaining[2:]

	// Parse tool-specific arguments
	mcpArgs, err := parseCLIArgs(tool, action, toolArgs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 2
	}

	// Ensure daemon is running and get base URL
	baseURL, err := ensureDaemon(cfg.Port)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	// Accessibility audits get extended timeout
	timeout := cfg.Timeout
	if tool == "observe" && normalizeAction(action) == "accessibility" {
		timeout = 35000
	}

	// Call the tool
	result, err := callTool(baseURL, tool, mcpArgs, timeout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	return formatResult(cfg.Format, tool, normalizeAction(action), result)
}

// resolveCLIConfig resolves config from defaults < env < flags, stripping global flags.
func resolveCLIConfig(args []string) (cliConfig, []string) {
	cfg := cliConfig{
		Port:    defaultPort,
		Format:  "human",
		Timeout: 5000,
	}

	applyCLIEnvOverrides(&cfg)

	remaining := applyCLIFlagOverrides(args, &cfg)

	return cfg, remaining
}

// applyCLIEnvOverrides applies GASOLINE_PORT and GASOLINE_FORMAT environment variables.
func applyCLIEnvOverrides(cfg *cliConfig) {
	if envPort := os.Getenv("GASOLINE_PORT"); envPort != "" {
		if p, err := strconv.Atoi(envPort); err == nil {
			cfg.Port = p
		}
	}
	if envFormat := os.Getenv("GASOLINE_FORMAT"); envFormat != "" {
		cfg.Format = envFormat
	}
}

// applyCLIFlagOverrides strips --port, --format, --timeout flags and applies their values.
func applyCLIFlagOverrides(args []string, cfg *cliConfig) []string {
	remaining := args

	var portStr string
	portStr, remaining = cliParseFlag(remaining, "--port")
	if portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil {
			cfg.Port = p
		}
	}

	var format string
	format, remaining = cliParseFlag(remaining, "--format")
	if format != "" {
		cfg.Format = format
	}

	var timeoutStr string
	timeoutStr, remaining = cliParseFlag(remaining, "--timeout")
	if timeoutStr != "" {
		if t, err := strconv.Atoi(timeoutStr); err == nil {
			cfg.Timeout = t
		}
	}

	return remaining
}

// ensureDaemon checks if the server is running and spawns it if needed.
// Returns the base URL (e.g., "http://127.0.0.1:7890").
func ensureDaemon(port int) (string, error) {
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)

	if isServerRunning(port) {
		return baseURL, nil
	}

	// Spawn daemon
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("cannot find executable: %w", err)
	}

	cmd := exec.Command(exe, "--daemon", "--port", fmt.Sprintf("%d", port)) // #nosec G204,G702 -- exe is our own binary path from os.Executable with fixed flags // nosemgrep: go.lang.security.audit.dangerous-exec-command.dangerous-exec-command, go_subproc_rule-subproc -- CLI opens browser with known URL
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	util.SetDetachedProcess(cmd)

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start daemon: %w", err)
	}

	// Wait for daemon to become ready
	if !waitForServer(port, 4*time.Second) {
		return "", fmt.Errorf("daemon started but not responding on port %d after 4s", port)
	}

	return baseURL, nil
}

// callTool builds a JSON-RPC tools/call request, POSTs to /mcp, and parses the response.
func callTool(baseURL, toolName string, mcpArgs map[string]any, timeoutMs int) (*MCPToolResult, error) {
	body, err := buildToolCallBody(toolName, mcpArgs)
	if err != nil {
		return nil, err
	}

	respBody, err := postToolCall(baseURL+"/mcp", body, timeoutMs)
	if err != nil {
		return nil, err
	}

	return parseToolCallResponse(respBody)
}

// buildToolCallBody creates the JSON-RPC request body for a tools/call.
func buildToolCallBody(toolName string, mcpArgs map[string]any) ([]byte, error) {
	params := map[string]any{"name": toolName, "arguments": mcpArgs}
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal params: %w", err)
	}

	rpcReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      "cli-1",
		Method:  "tools/call",
		Params:  paramsJSON,
	}
	body, err := json.Marshal(rpcReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	return body, nil
}

// postToolCall sends a JSON-RPC request to the MCP endpoint and returns the raw response body.
func postToolCall(endpoint string, body []byte, timeoutMs int) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(httpReq) // #nosec G704 -- endpoint comes from ensureDaemon() and is localhost-only
	if err != nil {
		return nil, fmt.Errorf("server connection error: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxPostBodySize))
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	return respBody, nil
}

// parseToolCallResponse parses a JSON-RPC response into an MCPToolResult.
func parseToolCallResponse(respBody []byte) (*MCPToolResult, error) {
	var rpcResp JSONRPCResponse
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	if rpcResp.Error != nil {
		return nil, fmt.Errorf("server error (%d): %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	var toolResult MCPToolResult
	if err := json.Unmarshal(rpcResp.Result, &toolResult); err != nil {
		return nil, fmt.Errorf("failed to parse tool result: %w", err)
	}
	return &toolResult, nil
}
