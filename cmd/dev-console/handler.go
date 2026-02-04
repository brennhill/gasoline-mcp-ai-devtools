// handler.go — MCP protocol handler for JSON-RPC 2.0 requests.
// Contains MCPHandler type and HTTP/stdio transport handling.
// Extracted from cmd/gasoline/main.go during Phase 4 refactoring.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/dev-console/dev-console/internal/capture"
)

// MCPHandler handles MCP protocol messages
type MCPHandler struct {
	server      *Server
	toolHandler ToolHandlerInterface
	version     string
}

// ToolHandlerInterface defines the minimal tool handler interface
type ToolHandlerInterface interface {
	GetCapture() *capture.Capture
	GetToolCallLimiter() RateLimiter
	GetRedactionEngine() RedactionEngine
	ToolsList() []MCPTool
	HandleToolCall(req JSONRPCRequest, name string, arguments json.RawMessage) (JSONRPCResponse, bool)
}

// RateLimiter interface for tool call rate limiting
type RateLimiter interface {
	Allow() bool
}

// RedactionEngine interface for response redaction
type RedactionEngine interface {
	RedactJSON(data json.RawMessage) json.RawMessage
}

// NewMCPHandler creates a new MCP handler
func NewMCPHandler(server *Server, version string) *MCPHandler {
	return &MCPHandler{
		server:  server,
		version: version,
	}
}

// SetToolHandler sets the tool handler (called after construction)
func (h *MCPHandler) SetToolHandler(th ToolHandlerInterface) {
	h.toolHandler = th
}

// HandleHTTP handles MCP requests over HTTP (POST /mcp)
func (h *MCPHandler) HandleHTTP(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	sessionID := r.Header.Get("X-Gasoline-Session")
	clientID := r.Header.Get("X-Gasoline-Client")
	extensionVersion := r.Header.Get("X-Gasoline-Extension-Version")

	// Collect all headers for debug logging (redact auth)
	headers := make(map[string]string)
	for name, values := range r.Header {
		if strings.Contains(strings.ToLower(name), "auth") || strings.Contains(strings.ToLower(name), "token") {
			headers[name] = "[REDACTED]"
		} else if len(values) > 0 {
			headers[name] = values[0]
		}
	}

	// Log version mismatch if detected
	if extensionVersion != "" && extensionVersion != h.version {
		fmt.Fprintf(os.Stderr, "[gasoline] Version mismatch: server=%s extension=%s\n", h.version, extensionVersion)
	}

	if r.Method != "POST" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	// Read body for logging
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		if h.toolHandler != nil {
			cap := h.toolHandler.GetCapture()
			if cap != nil {
				duration := time.Since(startTime)
				debugEntry := capture.HTTPDebugEntry{
					Timestamp:      startTime,
					Endpoint:       "/mcp",
					Method:         "POST",
					SessionID:      sessionID,
					ClientID:       clientID,
					Headers:        headers,
					ResponseStatus: http.StatusBadRequest,
					DurationMs:     duration.Milliseconds(),
					Error:          fmt.Sprintf("Could not read body: %v", err),
				}
				cap.LogHTTPDebugEntry(debugEntry)
				capture.PrintHTTPDebug(debugEntry)
			}
		}
		resp := JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      "error",  // Fallback ID (never null - Cursor rejects it)
			Error: &JSONRPCError{
				Code:    -32700,
				Message: "Read error: " + err.Error(),
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	requestPreview := string(bodyBytes)
	if len(requestPreview) > 1000 {
		requestPreview = requestPreview[:1000] + "..."
	}

	var req JSONRPCRequest
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		if h.toolHandler != nil {
			cap := h.toolHandler.GetCapture()
			if cap != nil {
				duration := time.Since(startTime)
				debugEntry := capture.HTTPDebugEntry{
					Timestamp:      startTime,
					Endpoint:       "/mcp",
					Method:         "POST",
					SessionID:      sessionID,
					ClientID:       clientID,
					Headers:        headers,
					RequestBody:    requestPreview,
					ResponseStatus: http.StatusBadRequest,
					DurationMs:     duration.Milliseconds(),
					Error:          fmt.Sprintf("Parse error: %v", err),
				}
				cap.LogHTTPDebugEntry(debugEntry)
				capture.PrintHTTPDebug(debugEntry)
			}
		}
		// Try to extract ID from malformed JSON
		var partial map[string]any
		var errorID any = "error"  // Fallback ID (never null - Cursor rejects it)
		if json.Unmarshal(bodyBytes, &partial) == nil {
			if id, ok := partial["id"]; ok && id != nil {
				errorID = id
			}
		}

		resp := JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      errorID,
			Error: &JSONRPCError{
				Code:    -32700,
				Message: "Parse error: " + err.Error(),
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	// Extract client ID for multi-client isolation (stored on the request, not the handler)
	req.ClientID = clientID

	resp := h.HandleRequest(req)

	// Notifications return nil - do NOT send a response
	if resp == nil {
		// For HTTP, we still need to send *something* - send 204 No Content
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Log debug entry
	if h.toolHandler != nil {
		cap := h.toolHandler.GetCapture()
		if cap != nil {
			duration := time.Since(startTime)
			responseJSON, _ := json.Marshal(resp)
			responsePreview := string(responseJSON)
			if len(responsePreview) > 1000 {
				responsePreview = responsePreview[:1000] + "..."
			}

			debugEntry := capture.HTTPDebugEntry{
				Timestamp:      startTime,
				Endpoint:       "/mcp",
				Method:         "POST",
				SessionID:      sessionID,
				ClientID:       clientID,
				Headers:        headers,
				RequestBody:    requestPreview,
				ResponseStatus: http.StatusOK,
				ResponseBody:   responsePreview,
				DurationMs:     duration.Milliseconds(),
			}
			cap.LogHTTPDebugEntry(debugEntry)
			capture.PrintHTTPDebug(debugEntry)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// HandleRequest processes an MCP request and returns a response.
// Returns nil for notifications (which should not receive a response).
func (h *MCPHandler) HandleRequest(req JSONRPCRequest) *JSONRPCResponse {
	// CRITICAL: Notifications do NOT get responses per JSON-RPC 2.0 spec
	// Notifications are identified by: no "id" field OR method starting with "notifications/"
	if req.ID == nil || strings.HasPrefix(req.Method, "notifications/") {
		// This is a notification - do NOT send a response
		return nil
	}

	switch req.Method {
	case "initialize":
		resp := h.handleInitialize(req)
		return &resp
	case "initialized":
		// Legacy: some clients send "initialized" as a request (with id)
		resp := JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(`{}`)}
		return &resp
	case "tools/list":
		resp := h.handleToolsList(req)
		return &resp
	case "tools/call":
		resp := h.handleToolsCall(req)
		return &resp
	case "resources/list":
		resp := h.handleResourcesList(req)
		return &resp
	case "resources/read":
		resp := h.handleResourcesRead(req)
		return &resp
	case "resources/templates/list":
		resp := h.handleResourcesTemplatesList(req)
		return &resp
	case "prompts/list":
		// Return empty prompts list (we don't support prompts yet)
		resp := JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(`{"prompts":[]}`)}
		return &resp
	case "ping":
		resp := JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(`{}`)}
		return &resp
	default:
		resp := JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &JSONRPCError{
				Code:    -32601,
				Message: "Method not found: " + req.Method,
			},
		}
		return &resp
	}
}

func (h *MCPHandler) handleInitialize(req JSONRPCRequest) JSONRPCResponse {
	const supportedVersion = "2024-11-05"

	// Parse client's requested protocol version (best-effort; missing/empty is fine)
	var initParams struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	if len(req.Params) > 0 {
		_ = json.Unmarshal(req.Params, &initParams)
	}

	// Negotiate: echo client's version if supported, otherwise respond with our latest
	negotiatedVersion := supportedVersion
	if initParams.ProtocolVersion == supportedVersion {
		negotiatedVersion = initParams.ProtocolVersion
	}

	result := MCPInitializeResult{
		ProtocolVersion: negotiatedVersion,
		ServerInfo: MCPServerInfo{
			Name:    "gasoline",
			Version: h.version,
		},
		Capabilities: MCPCapabilities{
			Tools:     MCPToolsCapability{},
			Resources: MCPResourcesCapability{},
		},
	}

	// Error impossible: MCPInitResult is a simple struct with no circular refs or unsupported types
	resultJSON, _ := json.Marshal(result)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: resultJSON}
}

func (h *MCPHandler) handleResourcesList(req JSONRPCRequest) JSONRPCResponse {
	resources := []MCPResource{
		{
			URI:         "gasoline://guide",
			Name:        "Gasoline Usage Guide",
			Description: "How to use Gasoline MCP tools for browser debugging",
			MimeType:    "text/markdown",
		},
	}
	result := MCPResourcesListResult{Resources: resources}
	// Error impossible: MCPResourcesListResult is a simple struct with no circular refs or unsupported types
	resultJSON, _ := json.Marshal(result)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: resultJSON}
}

func (h *MCPHandler) handleResourcesRead(req JSONRPCRequest) JSONRPCResponse {
	var params struct {
		URI string `json:"uri"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &JSONRPCError{
				Code:    -32602,
				Message: "Invalid params: " + err.Error(),
			},
		}
	}

	if params.URI != "gasoline://guide" {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &JSONRPCError{
				Code:    -32002,
				Message: "Resource not found: " + params.URI,
			},
		}
	}

	guide := `# Gasoline MCP Tools

Browser observability for AI coding agents. See console errors, network failures, DOM state, and more.

## Quick Reference

| Tool | Purpose | Key Parameter |
|------|---------|---------------|
| ` + "`observe`" + ` | Read browser state & analyze | ` + "`what`" + `: errors, logs, network, vitals, page, performance, accessibility, api, changes, timeline, security_audit |
| ` + "`generate`" + ` | Create artifacts | ` + "`format`" + `: test, reproduction, pr_summary, sarif, har, csp, sri |
| ` + "`configure`" + ` | Manage session & settings | ` + "`action`" + `: store, noise_rule, dismiss, clear, query_dom, health |
| ` + "`interact`" + ` | Control the browser | ` + "`action`" + `: highlight, save_state, load_state, execute_js, navigate, refresh |

## Common Workflows

### See browser errors
` + "```" + `json
{ "tool": "observe", "arguments": { "what": "errors" } }
` + "```" + `

### Check failed network requests
` + "```" + `json
{ "tool": "observe", "arguments": { "what": "network", "status_min": 400 } }
` + "```" + `

### Run accessibility audit
` + "```" + `json
{ "tool": "observe", "arguments": { "what": "accessibility" } }
` + "```" + `

### Query DOM element
` + "```" + `json
{ "tool": "configure", "arguments": { "action": "query_dom", "selector": ".error-message" } }
` + "```" + `

### Generate Playwright test from session
` + "```" + `json
{ "tool": "generate", "arguments": { "format": "test", "test_name": "user_login" } }
` + "```" + `

### Check Web Vitals (LCP, CLS, INP, FCP)
` + "```" + `json
{ "tool": "observe", "arguments": { "what": "vitals" } }
` + "```" + `

## Tips

- Start with ` + "`observe`" + ` ` + "`what: \"errors\"`" + ` to see what's broken
- Use ` + "`what: \"page\"`" + ` to confirm which URL the browser is on
- The browser extension must show "Connected" for tools to work
- Data comes from the active browser tab
`

	result := MCPResourcesReadResult{
		Contents: []MCPResourceContent{
			{
				URI:      "gasoline://guide",
				MimeType: "text/markdown",
				Text:     guide,
			},
		},
	}
	// Error impossible: MCPResourceContentResult is a simple struct with no circular refs or unsupported types
	resultJSON, _ := json.Marshal(result)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: resultJSON}
}

func (h *MCPHandler) handleResourcesTemplatesList(req JSONRPCRequest) JSONRPCResponse {
	result := MCPResourceTemplatesListResult{ResourceTemplates: []any{}}
	// Error impossible: MCPResourceTemplatesListResult is a simple struct with no circular refs or unsupported types
	resultJSON, _ := json.Marshal(result)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: resultJSON}
}

func (h *MCPHandler) handleToolsList(req JSONRPCRequest) JSONRPCResponse {
	var tools []MCPTool
	if h.toolHandler != nil {
		tools = h.toolHandler.ToolsList()
	}

	result := MCPToolsListResult{Tools: tools}
	// Error impossible: MCPToolsListResult is a simple struct with no circular refs or unsupported types
	resultJSON, _ := json.Marshal(result)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: resultJSON}
}

func (h *MCPHandler) handleToolsCall(req JSONRPCRequest) JSONRPCResponse {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}

	if err := json.Unmarshal(req.Params, &params); err != nil {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &JSONRPCError{
				Code:    -32602,
				Message: "Invalid params: " + err.Error(),
			},
		}
	}

	// Check tool call rate limit before dispatch
	if h.toolHandler != nil {
		limiter := h.toolHandler.GetToolCallLimiter()
		if limiter != nil && !limiter.Allow() {
			return JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &JSONRPCError{
					Code:    -32603,
					Message: "Tool call rate limit exceeded (500 calls/minute). Please wait before retrying.",
				},
			}
		}
	}

	if h.toolHandler != nil {
		if resp, handled := h.toolHandler.HandleToolCall(req, params.Name, params.Arguments); handled {
			// Apply redaction to tool response before returning to AI client
			redactor := h.toolHandler.GetRedactionEngine()
			if redactor != nil && resp.Result != nil {
				resp.Result = redactor.RedactJSON(resp.Result)
			}
			return resp
		}
	}

	return JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Error: &JSONRPCError{
			Code:    -32601,
			Message: "Unknown tool: " + params.Name,
		},
	}
}

// jsonResponse is a JSON response helper
func jsonResponse(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		fmt.Fprintf(os.Stderr, "[gasoline] Error encoding JSON response: %v\n", err)
	}
}
