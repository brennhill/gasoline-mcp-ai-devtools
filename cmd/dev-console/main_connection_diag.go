// main_connection_diag.go — Connection diagnostics: health checks, port scanning, and failure analysis.
// Docs: docs/features/feature/observe/index.md
package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// gatherConnectionDiagnostics collects detailed information about why connection failed.
// Returns a map with diagnostic data for debug logging and user error messages.
func gatherConnectionDiagnostics(port int, serverURL string, healthURL string) map[string]interface{} {
	diagnostics := make(map[string]interface{})

	diagnosePortStatus(diagnostics, port)
	diagnoseProcessOnPort(diagnostics, port)
	diagnoseHealthEndpoint(diagnostics, healthURL)
	diagnoseMCPEndpoint(diagnostics, serverURL)
	summarizeDiagnosis(diagnostics, port)

	return diagnostics
}

// diagnosePortStatus checks whether the port is accepting TCP connections.
func diagnosePortStatus(diagnostics map[string]interface{}, port int) {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 500*time.Millisecond)
	if err != nil {
		diagnostics["port_status"] = "not listening"
		diagnostics["port_error"] = err.Error()
		return
	}
	_ = conn.Close()
	diagnostics["port_status"] = "listening"
}

// diagnoseProcessOnPort identifies what process is using the port.
func diagnoseProcessOnPort(diagnostics map[string]interface{}, port int) {
	pids, err := findProcessOnPort(port)
	if err != nil || len(pids) == 0 {
		diagnostics["process_info"] = "no process found on port"
		return
	}

	pidStrs := make([]string, len(pids))
	for i, p := range pids {
		pidStrs[i] = strconv.Itoa(p)
	}
	diagnostics["process_pids"] = strings.Join(pidStrs, "\n")

	cmdLine := getProcessCommand(pids[0])
	if cmdLine == "" {
		return
	}
	diagnostics["process_command"] = cmdLine
	if strings.Contains(strings.ToLower(cmdLine), "gasoline") {
		diagnostics["process_type"] = "gasoline (correct)"
	} else {
		diagnostics["process_type"] = "NOT gasoline (conflict)"
		diagnostics["process_info"] = fmt.Sprintf("Port %d is occupied by: %s", port, cmdLine)
	}
}

// diagnoseHealthEndpoint probes the /health endpoint and classifies the failure mode.
func diagnoseHealthEndpoint(diagnostics map[string]interface{}, healthURL string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
	resp, err := http.DefaultClient.Do(req) // #nosec G704 -- healthURL is localhost-only from trusted port
	if err != nil {
		diagnostics["health_check"] = "failed"
		diagnostics["health_error"] = err.Error()
		diagnostics["health_diagnosis"] = classifyHealthError(err)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	diagnostics["health_status_code"] = resp.StatusCode
	if resp.StatusCode != http.StatusOK {
		diagnostics["health_check"] = fmt.Sprintf("unexpected status %d", resp.StatusCode)
		return
	}

	diagnostics["health_check"] = "passed"
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxPostBodySize))
	if err != nil || len(body) == 0 {
		return
	}
	bodyStr := string(body)
	if strings.Contains(bodyStr, "gasoline") || strings.Contains(bodyStr, "status") {
		diagnostics["health_response"] = "valid gasoline response"
	} else {
		diagnostics["health_response"] = "unexpected response format"
		previewLen := 100
		if len(bodyStr) < previewLen {
			previewLen = len(bodyStr)
		}
		diagnostics["health_body_preview"] = bodyStr[:previewLen]
	}
}

// classifyHealthError maps a health check error to a human-readable diagnosis.
func classifyHealthError(err error) string {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "connection refused"):
		return "port not accepting connections"
	case strings.Contains(msg, "timeout"):
		return "server not responding (may be overloaded)"
	case strings.Contains(msg, "no route to host"):
		return "network/firewall issue"
	default:
		return "unknown connection error"
	}
}

// diagnoseMCPEndpoint probes the /mcp endpoint with a minimal JSON-RPC initialize request.
func diagnoseMCPEndpoint(diagnostics map[string]interface{}, serverURL string) {
	mcpURL := serverURL + "/mcp"
	mcpReq := `{"jsonrpc":"2.0","id":0,"method":"initialize","params":{}}`
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	httpReq, _ := http.NewRequestWithContext(ctx, "POST", mcpURL, strings.NewReader(mcpReq))
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(httpReq) // #nosec G704 -- mcpURL is localhost-only from trusted port
	if err != nil {
		diagnostics["mcp_endpoint"] = "unreachable"
		diagnostics["mcp_error"] = err.Error()
		return
	}
	defer func() { _ = resp.Body.Close() }()

	diagnostics["mcp_endpoint"] = fmt.Sprintf("status %d", resp.StatusCode)
	if resp.StatusCode == http.StatusOK {
		diagnostics["mcp_status"] = "responsive"
	}
}

// summarizeDiagnosis produces a top-level diagnosis and recommended action from gathered data.
func summarizeDiagnosis(diagnostics map[string]interface{}, port int) {
	switch {
	case diagnostics["port_status"] == "not listening":
		diagnostics["diagnosis"] = "No server running on port"
		diagnostics["recommended_action"] = "Server should auto-spawn but didn't - check logs"
	case diagnostics["process_type"] == "NOT gasoline (conflict)":
		diagnostics["diagnosis"] = "Port occupied by different service"
		diagnostics["recommended_action"] = fmt.Sprintf("Kill process or use different port: --port %d", port+1)
	case diagnostics["health_check"] == "failed":
		diagnostics["diagnosis"] = "Gasoline process exists but not responding"
		diagnostics["recommended_action"] = "Process may be hung or crashed - will attempt recovery"
	case diagnostics["health_check"] == "passed" && diagnostics["mcp_endpoint"] == "unreachable":
		diagnostics["diagnosis"] = "Health endpoint works but MCP endpoint doesn't"
		diagnostics["recommended_action"] = "Server partially initialized - will attempt recovery"
	default:
		diagnostics["diagnosis"] = "Unknown connection failure"
		diagnostics["recommended_action"] = "Check debug log for details"
	}
}
