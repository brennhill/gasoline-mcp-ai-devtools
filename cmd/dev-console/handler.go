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

// serverInstructions is sent once per session in the initialize response.
// It provides workflow guidance so tool descriptions can stay minimal.
const serverInstructions = `Gasoline provides real-time browser telemetry via 5 tools.

Workflow:
- observe: read passive buffers (errors, logs, network, actions, etc.)
- analyze: trigger active analysis (accessibility, security, performance, DOM queries)
- generate: create artifacts from captured data (Playwright tests, reproductions, HAR, CSP, SARIF)
- configure: session settings (noise rules, storage, streaming, clear buffers, health)
- interact: browser automation (click, type, navigate, execute JS, record) — requires AI Web Pilot extension

Key patterns:
- Pagination: observe returns after_cursor/before_cursor in metadata. Pass them back for next page. Use restart_on_eviction=true if cursor expired.
- Async analysis: analyze dispatches to the extension; poll results with observe(what="command_result", correlation_id=...).
- Error debugging: start with observe(what="error_bundles") for pre-assembled context per error (error + network + actions + logs).
- Performance: interact(action="navigate"|"refresh") auto-includes perf_diff. Add analyze=true to any interact action for profiling.
- Noise filtering: use configure(action="noise_rule", noise_action="auto_detect") to suppress recurring noise.
- For detailed docs, read the gasoline://guide resource.`

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

// httpRequestContext collects metadata from an HTTP request for debug logging.
type httpRequestContext struct {
	startTime time.Time
	sessionID string
	clientID  string
	headers   map[string]string
}

// newHTTPRequestContext extracts metadata from the request headers.
func newHTTPRequestContext(r *http.Request, serverVersion string) httpRequestContext {
	ctx := httpRequestContext{
		startTime: time.Now(),
		sessionID: r.Header.Get("X-Gasoline-Session"),
		clientID:  r.Header.Get("X-Gasoline-Client"),
	}

	ctx.headers = make(map[string]string)
	for name, values := range r.Header {
		lower := strings.ToLower(name)
		if strings.Contains(lower, "auth") || strings.Contains(lower, "token") {
			ctx.headers[name] = "[REDACTED]"
		} else if len(values) > 0 {
			ctx.headers[name] = values[0]
		}
	}

	if extVer := r.Header.Get("X-Gasoline-Extension-Version"); extVer != "" && extVer != serverVersion {
		fmt.Fprintf(os.Stderr, "[gasoline] Version mismatch: server=%s extension=%s\n", serverVersion, extVer)
	}

	return ctx
}

// logDebugEntry logs an HTTP debug entry if capture is available.
func (h *MCPHandler) logDebugEntry(ctx httpRequestContext, requestBody string, status int, responseBody string, errMsg string) {
	if h.toolHandler == nil {
		return
	}
	cap := h.toolHandler.GetCapture()
	if cap == nil {
		return
	}
	entry := capture.HTTPDebugEntry{
		Timestamp:      ctx.startTime,
		Endpoint:       "/mcp",
		Method:         "POST",
		SessionID:      ctx.sessionID,
		ClientID:       ctx.clientID,
		Headers:        ctx.headers,
		RequestBody:    requestBody,
		ResponseStatus: status,
		ResponseBody:   responseBody,
		DurationMs:     time.Since(ctx.startTime).Milliseconds(),
		Error:          errMsg,
	}
	cap.LogHTTPDebugEntry(entry)
}

// truncatePreview returns s truncated to 1000 characters with "..." suffix.
func truncatePreview(s string) string {
	if len(s) > 1000 {
		return s[:1000] + "..."
	}
	return s
}

// HandleHTTP handles MCP requests over HTTP (POST /mcp)
func (h *MCPHandler) HandleHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := newHTTPRequestContext(r, h.version)

	if r.Method != "POST" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxPostBodySize)

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		h.logDebugEntry(ctx, "", http.StatusBadRequest, "", fmt.Sprintf("Could not read body: %v", err))
		h.writeJSONRPCError(w, "error", -32700, "Read error: "+err.Error())
		return
	}

	requestPreview := truncatePreview(string(bodyBytes))

	var req JSONRPCRequest
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		h.logDebugEntry(ctx, requestPreview, http.StatusBadRequest, "", fmt.Sprintf("Parse error: %v", err))
		errorID := extractJSONRPCID(bodyBytes)
		h.writeJSONRPCError(w, errorID, -32700, "Parse error: "+err.Error())
		return
	}

	req.ClientID = ctx.clientID
	resp := h.HandleRequest(req)

	if resp == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Error impossible: simple struct with no circular refs or unsupported types
	responseJSON, _ := json.Marshal(resp)
	h.logDebugEntry(ctx, requestPreview, http.StatusOK, truncatePreview(string(responseJSON)), "")

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// writeJSONRPCError writes a JSON-RPC error response to the HTTP response writer.
func (h *MCPHandler) writeJSONRPCError(w http.ResponseWriter, id any, code int, message string) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &JSONRPCError{Code: code, Message: message},
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// extractJSONRPCID attempts to extract the "id" field from JSON bytes.
// Returns the extracted ID or "error" as fallback (never null - Cursor rejects it).
func extractJSONRPCID(bodyBytes []byte) any {
	var partial map[string]any
	var errorID any = "error"
	if json.Unmarshal(bodyBytes, &partial) == nil {
		if id, ok := partial["id"]; ok && id != nil {
			errorID = id
		}
	}
	return errorID
}

// mcpMethodHandler is a function that handles a specific MCP method.
type mcpMethodHandler func(h *MCPHandler, req JSONRPCRequest) JSONRPCResponse

// mcpMethodHandlers maps MCP method names to their handlers.
var mcpMethodHandlers = map[string]mcpMethodHandler{
	"initialize":               func(h *MCPHandler, req JSONRPCRequest) JSONRPCResponse { return h.handleInitialize(req) },
	"tools/list":               func(h *MCPHandler, req JSONRPCRequest) JSONRPCResponse { return h.handleToolsList(req) },
	"tools/call":               func(h *MCPHandler, req JSONRPCRequest) JSONRPCResponse { return h.handleToolsCall(req) },
	"resources/list":           func(h *MCPHandler, req JSONRPCRequest) JSONRPCResponse { return h.handleResourcesList(req) },
	"resources/read":           func(h *MCPHandler, req JSONRPCRequest) JSONRPCResponse { return h.handleResourcesRead(req) },
	"resources/templates/list": func(h *MCPHandler, req JSONRPCRequest) JSONRPCResponse { return h.handleResourcesTemplatesList(req) },
}

// mcpStaticResponses maps MCP methods to static JSON result bodies.
var mcpStaticResponses = map[string]string{
	"initialized":  `{}`,
	"ping":         `{}`,
	"prompts/list": `{"prompts":[]}`,
}

// HandleRequest processes an MCP request and returns a response.
// Returns nil for notifications (which should not receive a response).
func (h *MCPHandler) HandleRequest(req JSONRPCRequest) *JSONRPCResponse {
	// CRITICAL: Notifications do NOT get responses per JSON-RPC 2.0 spec
	if req.ID == nil || strings.HasPrefix(req.Method, "notifications/") {
		return nil
	}

	if handler, ok := mcpMethodHandlers[req.Method]; ok {
		resp := handler(h, req)
		return &resp
	}

	if staticResult, ok := mcpStaticResponses[req.Method]; ok {
		resp := JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(staticResult)}
		return &resp
	}

	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Error:   &JSONRPCError{Code: -32601, Message: "Method not found: " + req.Method},
	}
	return &resp
}

func (h *MCPHandler) handleInitialize(req JSONRPCRequest) JSONRPCResponse {
	const supportedVersion = "2024-11-05"

	// Parse client's requested protocol version (best-effort; missing/empty is fine)
	var initParams struct {
		ProtocolVersion string `json:"protocolVersion"` // SPEC:MCP
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
		Instructions: serverInstructions,
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

Browser observability for AI coding agents. 5 tools for real-time browser telemetry.

## Quick Reference

| Tool | Purpose | Key Parameters |
|------|---------|----------------|
| observe | Read passive browser buffers | what: errors, logs, extension_logs, network_waterfall, network_bodies, websocket_events, websocket_status, actions, vitals, page, tabs, pilot, timeline, error_bundles, screenshot, command_result, pending_commands, failed_commands, saved_videos, recordings, recording_actions, log_diff_report |
| analyze | Trigger active analysis (async) | what: dom, accessibility, performance, security_audit, third_party_audit, link_health, link_validation, error_clusters, history, api_validation, annotations, annotation_detail, draw_history, draw_session |
| generate | Create artifacts from captured data | format: test, reproduction, pr_summary, sarif, har, csp, sri, visual_test, annotation_report, annotation_issues, test_from_context, test_heal, test_classify |
| configure | Session settings and utilities | action: health, store, load, noise_rule, clear, streaming, test_boundary_start, test_boundary_end, recording_start, recording_stop, playback, log_diff |
| interact | Browser automation (needs AI Web Pilot) | action: click, type, select, check, navigate, refresh, execute_js, highlight, subtitle, key_press, scroll_to, wait_for, get_text, get_value, get_attribute, set_attribute, focus, list_interactive, save_state, load_state, list_states, delete_state, record_start, record_stop, upload, draw_mode_start, back, forward, new_tab |

## Key Patterns

### Check Extension Status First
Always verify the extension is connected before debugging:
  {"tool":"configure","arguments":{"action":"health"}}
If extension_connected is false, ask the user to click "Track This Tab" in the extension popup.

### Async Commands (analyze tool)
analyze dispatches queries to the extension asynchronously. Poll for results:
  1. {"tool":"analyze","arguments":{"what":"accessibility"}}  -> returns correlation_id
  2. {"tool":"observe","arguments":{"what":"command_result","correlation_id":"..."}}

### Pagination (observe tool)
Responses include cursors in metadata. Pass back for next page:
  {"tool":"observe","arguments":{"what":"logs","after_cursor":"...","limit":50}}
Use restart_on_eviction=true if a cursor expires.

## Common Workflows

  // See errors with surrounding context (network + actions + logs)
  {"tool":"observe","arguments":{"what":"error_bundles"}}

  // Check failed network requests
  {"tool":"observe","arguments":{"what":"network_waterfall","status_min":400}}

  // Run accessibility audit (async)
  {"tool":"analyze","arguments":{"what":"accessibility"}}

  // Query DOM elements (async)
  {"tool":"analyze","arguments":{"what":"dom","selector":".error-message"}}

  // Generate Playwright test from session
  {"tool":"generate","arguments":{"format":"test","test_name":"user_login"}}

  // Check Web Vitals (LCP, CLS, INP, FCP)
  {"tool":"observe","arguments":{"what":"vitals"}}

  // Navigate and measure performance (auto perf_diff)
  {"tool":"interact","arguments":{"action":"navigate","url":"https://example.com"}}

  // Suppress noisy console errors
  {"tool":"configure","arguments":{"action":"noise_rule","noise_action":"auto_detect"}}

## Tips

- Start with configure(action:"health") to verify extension is connected
- Use observe(what:"error_bundles") instead of raw errors — includes surrounding context
- Use observe(what:"page") to confirm which URL the browser is on
- interact actions require the AI Web Pilot extension feature to be enabled
- interact navigate and refresh automatically include performance diff metrics
- Data comes from the active tracked browser tab
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
			JSONRPC: "2.0", ID: req.ID,
			Error: &JSONRPCError{Code: -32602, Message: "Invalid params: " + err.Error()},
		}
	}

	if h.toolHandler == nil {
		return JSONRPCResponse{
			JSONRPC: "2.0", ID: req.ID,
			Error: &JSONRPCError{Code: -32601, Message: "Unknown tool: " + params.Name},
		}
	}

	if err := h.checkToolRateLimit(); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Error: err}
	}

	resp, handled := h.toolHandler.HandleToolCall(req, params.Name, params.Arguments)
	if !handled {
		return JSONRPCResponse{
			JSONRPC: "2.0", ID: req.ID,
			Error: &JSONRPCError{Code: -32601, Message: "Unknown tool: " + params.Name},
		}
	}

	resp = h.applyToolResponsePostProcessing(resp)
	return resp
}

// checkToolRateLimit returns a JSON-RPC error if the rate limit is exceeded.
func (h *MCPHandler) checkToolRateLimit() *JSONRPCError {
	limiter := h.toolHandler.GetToolCallLimiter()
	if limiter != nil && !limiter.Allow() {
		return &JSONRPCError{
			Code:    -32603,
			Message: "Tool call rate limit exceeded (500 calls/minute). Please wait before retrying.",
		}
	}
	return nil
}

// applyToolResponsePostProcessing applies redaction and version warnings to a tool response.
func (h *MCPHandler) applyToolResponsePostProcessing(resp JSONRPCResponse) JSONRPCResponse {
	redactor := h.toolHandler.GetRedactionEngine()
	if redactor != nil && resp.Result != nil {
		resp.Result = redactor.RedactJSON(resp.Result)
	}
	return h.maybeAddVersionWarning(resp)
}

// maybeAddVersionWarning prepends a version mismatch warning to the tool response
// when the extension and server versions differ in major.minor.
func (h *MCPHandler) maybeAddVersionWarning(resp JSONRPCResponse) JSONRPCResponse {
	if h.toolHandler == nil || resp.Result == nil {
		return resp
	}
	cap := h.toolHandler.GetCapture()
	if cap == nil {
		return resp
	}
	extVer, srvVer, mismatch := cap.GetVersionMismatch()
	if !mismatch {
		return resp
	}

	warning := fmt.Sprintf("WARNING: Version mismatch detected — server v%s, extension v%s. Update your extension to avoid issues.\n\n", srvVer, extVer)

	// Parse existing result, prepend warning to first text content block
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return resp
	}
	if len(result.Content) > 0 && result.Content[0].Type == "text" {
		result.Content[0].Text = warning + result.Content[0].Text
	} else {
		// Insert warning as new first content block
		result.Content = append([]MCPContentBlock{{Type: "text", Text: warning}}, result.Content...)
	}
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return resp
	}
	resp.Result = json.RawMessage(resultJSON)
	return resp
}

// jsonResponse is a JSON response helper
func jsonResponse(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		fmt.Fprintf(os.Stderr, "[gasoline] Error encoding JSON response: %v\n", err)
	}
}
