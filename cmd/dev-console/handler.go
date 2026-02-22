// handler.go — MCP protocol handler for JSON-RPC 2.0 requests.
// Docs: docs/features/feature/observe/index.md
// Contains MCPHandler type and HTTP/stdio transport handling.
// Extracted from cmd/gasoline/main.go during Phase 4 refactoring.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
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
- configure: session settings (noise rules, storage, streaming, clear buffers, health, restart)
- interact: browser automation (click, type, navigate, execute JS, record) — requires AI Web Pilot extension

Key patterns:
- Pagination: observe returns after_cursor/before_cursor in metadata. Pass them back for next page. Use restart_on_eviction=true if cursor expired.
- Async analysis: analyze dispatches to the extension; poll results with observe(what="command_result", correlation_id=...).
- Error debugging: start with observe(what="error_bundles") for pre-assembled context per error (error + network + actions + logs).
- Performance: interact(action="navigate"|"refresh") auto-includes perf_diff. Add analyze=true to any interact action for profiling.
- Noise filtering: use configure(action="noise_rule", noise_action="auto_detect") to suppress recurring noise.
- Recovery: if tools return repeated connection errors or timeouts, use configure(action="restart") to force-restart the daemon. This works even when the daemon is completely unresponsive.
- For routing help, read gasoline://capabilities. For detailed docs, read gasoline://guide. For quick examples, read gasoline://quickstart.`

// MCPHandler handles MCP protocol messages
type MCPHandler struct {
	server      *Server
	toolHandler ToolHandlerInterface
	version     string

	telemetryMu      sync.Mutex
	telemetryCursors map[string]passiveTelemetryCursor
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
	RedactMapValues(data map[string]any) map[string]any
}

// NewMCPHandler creates a new MCP handler
func NewMCPHandler(server *Server, version string) *MCPHandler {
	return &MCPHandler{
		server:           server,
		version:          version,
		telemetryCursors: make(map[string]passiveTelemetryCursor),
	}
}

// SetToolHandler sets the tool handler (called after construction)
func (h *MCPHandler) SetToolHandler(th ToolHandlerInterface) {
	h.toolHandler = th
}

// httpRequestContext collects metadata from an HTTP request for debug logging.
type httpRequestContext struct {
	startTime    time.Time
	extSessionID string
	clientID     string
	headers      map[string]string
}

// newHTTPRequestContext extracts metadata from the request headers.
func newHTTPRequestContext(r *http.Request, serverVersion string) httpRequestContext {
	ctx := httpRequestContext{
		startTime:    time.Now(),
		extSessionID: r.Header.Get("X-Gasoline-Ext-Session"),
		clientID:     r.Header.Get("X-Gasoline-Client"),
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
		stderrf("[gasoline] Version mismatch: server=%s extension=%s\n", serverVersion, extVer)
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
		ExtSessionID:   ctx.extSessionID,
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

	// Validate Content-Type: must be application/json (or empty for lenient clients)
	if ct := r.Header.Get("Content-Type"); ct != "" && !strings.Contains(ct, "application/json") {
		h.writeJSONRPCError(w, nil, -32700, "Unsupported Content-Type: "+ct)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxPostBodySize)

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		h.logDebugEntry(ctx, "", http.StatusBadRequest, "", fmt.Sprintf("Could not read body: %v", err))
		h.writeJSONRPCError(w, nil, -32700, "Read error: "+err.Error())
		return
	}

	requestPreview := truncatePreview(string(bodyBytes))

	var req JSONRPCRequest
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		h.logDebugEntry(ctx, requestPreview, http.StatusBadRequest, "", fmt.Sprintf("Parse error: %v", err))
		h.writeJSONRPCError(w, nil, -32700, "Parse error: "+err.Error())
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
	if req.HasInvalidID() {
		resp := JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      nil,
			Error: &JSONRPCError{
				Code:    -32600,
				Message: "Invalid Request: id must be string or number when present",
			},
		}
		return &resp
	}

	// Notifications do not get responses per JSON-RPC 2.0.
	if !req.HasID() {
		return nil
	}

	// JSON-RPC 2.0: All requests must include "jsonrpc": "2.0"
	if req.JSONRPC != "2.0" {
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &JSONRPCError{Code: -32600, Message: `Invalid Request: jsonrpc must be "2.0"`},
		}
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
	negotiatedVersion := negotiateProtocolVersion(req.Params)

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
	result := MCPResourcesListResult{Resources: mcpResources()}
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

	canonicalURI, text, ok := resolveResourceContent(params.URI)
	if !ok {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &JSONRPCError{
				Code:    -32002,
				Message: "Resource not found: " + params.URI,
			},
		}
	}

	result := MCPResourcesReadResult{Contents: []MCPResourceContent{
		{URI: canonicalURI, MimeType: "text/markdown", Text: text},
	}}
	// Error impossible: MCPResourceContentResult is a simple struct with no circular refs or unsupported types
	resultJSON, _ := json.Marshal(result)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: resultJSON}
}

func (h *MCPHandler) handleResourcesTemplatesList(req JSONRPCRequest) JSONRPCResponse {
	result := MCPResourceTemplatesListResult{ResourceTemplates: mcpResourceTemplates()}
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

	h.warnUnknownToolArguments(params.Name, params.Arguments)

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

	telemetryModeOverride := parseTelemetryModeOverride(params.Arguments)
	resp = h.applyToolResponsePostProcessing(resp, req.ClientID, params.Name, telemetryModeOverride)
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

func (h *MCPHandler) warnUnknownToolArguments(toolName string, args json.RawMessage) {
	if h.server == nil || h.toolHandler == nil || len(args) == 0 {
		return
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(args, &raw); err != nil {
		return
	}
	if len(raw) == 0 {
		return
	}

	allowed := h.allowedToolArgumentKeys(toolName, raw)
	if len(allowed) == 0 {
		return
	}

	unknown := make([]string, 0)
	for k := range raw {
		if _, ok := allowed[k]; !ok {
			unknown = append(unknown, k)
		}
	}
	sort.Strings(unknown)
	for _, k := range unknown {
		h.server.AddWarning(fmt.Sprintf("unknown parameter '%s' for tool '%s' (ignored)", k, toolName))
	}
}

func (h *MCPHandler) allowedToolArgumentKeys(toolName string, rawArgs map[string]json.RawMessage) map[string]struct{} {
	tools := h.toolHandler.ToolsList()
	for _, tool := range tools {
		if tool.Name != toolName {
			continue
		}

		keys := make(map[string]struct{})
		props, ok := tool.InputSchema["properties"].(map[string]any)
		if !ok {
			return keys
		}
		for k := range props {
			keys[k] = struct{}{}
		}
		return keys
	}
	return nil
}

// applyToolResponsePostProcessing applies redaction and version warnings to a tool response.
func (h *MCPHandler) applyToolResponsePostProcessing(resp JSONRPCResponse, clientID, toolName, telemetryModeOverride string) JSONRPCResponse {
	redactor := h.toolHandler.GetRedactionEngine()
	if redactor != nil && resp.Result != nil {
		resp.Result = redactor.RedactJSON(resp.Result)
	}
	if h.server != nil {
		resp = appendWarningsToResponse(resp, h.server.TakeWarnings())
	}
	resp = h.maybeAddSecurityModeWarning(resp)
	resp = h.maybeAddVersionWarning(resp)
	resp = maybeAddUpdateAvailableWarning(resp)
	resp = maybeAddUpgradeWarning(resp)
	return h.maybeAddTelemetrySummary(resp, clientID, toolName, telemetryModeOverride)
}

func (h *MCPHandler) maybeAddSecurityModeWarning(resp JSONRPCResponse) JSONRPCResponse {
	if h.toolHandler == nil || resp.Result == nil {
		return resp
	}
	cap := h.toolHandler.GetCapture()
	if cap == nil {
		return resp
	}

	mode, productionParity, rewrites := cap.GetSecurityMode()
	if mode == capture.SecurityModeNormal {
		return resp
	}

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return resp
	}

	warning := "[ALTERED ENVIRONMENT] security_mode=insecure_proxy; production_parity=false. CSP headers are rewritten for debugging.\n\n"
	if len(result.Content) > 0 && result.Content[0].Type == "text" {
		result.Content[0].Text = warning + result.Content[0].Text
	} else {
		result.Content = append([]MCPContentBlock{{Type: "text", Text: warning}}, result.Content...)
	}

	if result.Metadata == nil {
		result.Metadata = make(map[string]any)
	}
	result.Metadata["security_mode"] = mode
	result.Metadata["production_parity"] = productionParity
	result.Metadata["insecure_rewrites_applied"] = rewrites

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return resp
	}
	resp.Result = json.RawMessage(resultJSON)
	return resp
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

// updateNotifyLastShown tracks when the "update available" warning was last shown.
// Used to enforce a daily cooldown so we don't nag on every tool call.
var updateNotifyLastShown time.Time

// maybeAddUpdateAvailableWarning prepends an "update available" notice when the
// GitHub version check has found a newer release than the running daemon.
// Shows at most once per day (24h cooldown). Skipped when a binary upgrade
// is already pending (that warning is more actionable).
func maybeAddUpdateAvailableWarning(resp JSONRPCResponse) JSONRPCResponse {
	if resp.Result == nil {
		return resp
	}

	// Skip if a binary upgrade is already pending — that's more actionable.
	if binaryUpgradeState != nil {
		if pending, _, _ := binaryUpgradeState.UpgradeInfo(); pending {
			return resp
		}
	}

	versionCheckMu.Lock()
	availVer := availableVersion
	versionCheckMu.Unlock()

	if availVer == "" || !isNewerVersion(availVer, version) {
		return resp
	}

	// Daily cooldown: don't nag more than once per 24h.
	if !updateNotifyLastShown.IsZero() && time.Since(updateNotifyLastShown) < 24*time.Hour {
		return resp
	}
	updateNotifyLastShown = time.Now()

	warning := fmt.Sprintf("UPDATE AVAILABLE: Gasoline v%s is available (current: v%s). Run: npm install -g gasoline-mcp@latest\n\n", availVer, version)

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return resp
	}
	if len(result.Content) > 0 && result.Content[0].Type == "text" {
		result.Content[0].Text = warning + result.Content[0].Text
	} else {
		result.Content = append([]MCPContentBlock{{Type: "text", Text: warning}}, result.Content...)
	}
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return resp
	}
	resp.Result = json.RawMessage(resultJSON)
	return resp
}

// maybeAddUpgradeWarning prepends a binary upgrade notice to the tool response
// when a newer binary has been detected on disk (pending auto-restart).
func maybeAddUpgradeWarning(resp JSONRPCResponse) JSONRPCResponse {
	if binaryUpgradeState == nil || resp.Result == nil {
		return resp
	}
	pending, newVer, detectedAt := binaryUpgradeState.UpgradeInfo()
	if !pending {
		return resp
	}

	elapsed := time.Since(detectedAt).Truncate(time.Second)
	warning := fmt.Sprintf("NOTICE: Gasoline v%s detected on disk (current: v%s, detected %s ago). Auto-restart imminent. Your next tool call will use the new version.\n\n", newVer, version, elapsed)

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return resp
	}
	if len(result.Content) > 0 && result.Content[0].Type == "text" {
		result.Content[0].Text = warning + result.Content[0].Text
	} else {
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
		stderrf("[gasoline] Error encoding JSON response: %v\n", err)
	}
}
