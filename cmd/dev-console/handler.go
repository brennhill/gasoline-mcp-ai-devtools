// Purpose: Defines MCP handler core types, interfaces, and bootstrap wiring.
// Why: Keeps shared handler state concise while method behavior lives in focused files.

package main

import (
	"encoding/json"
	"sync"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/capture"
)

// serverInstructions is sent once per session in the initialize response.
// It provides workflow guidance so tool descriptions can stay minimal.
const serverInstructions = `Gasoline Agentic Browser provides real-time browser telemetry and automation via 5 tools.

Workflow:
- observe: read passive buffers (errors, logs, network, screenshots, actions, etc.)
- analyze: trigger active analysis (accessibility, security, performance, DOM queries)
- generate: create artifacts from captured data (Playwright tests, reproductions, HAR, CSP, SARIF)
- configure: session settings (noise rules, storage, streaming, clear buffers, health, restart)
- interact: browser automation (navigate, click, type, fill forms, upload, execute JS, record) — controls any web page

Key patterns:
- Browser automation: use interact to navigate to any URL, click buttons, type text, fill forms, and control the browser. Use observe(what="screenshot") to visually verify page state before and after actions.
- Pagination: observe returns after_cursor/before_cursor in metadata. Pass them back for next page. Use restart_on_eviction=true if cursor expired.
- Async analysis: analyze dispatches to the extension; poll results with observe(what="command_result", correlation_id=...).
- Error debugging: start with observe(what="error_bundles") for pre-assembled context per error (error + network + actions + logs).
- Performance: interact(what="navigate"|"refresh") auto-includes perf_diff. Add analyze=true to any interact action for profiling.
- Noise filtering: use configure(what="noise_rule", noise_action="auto_detect") to suppress recurring noise.
- Recovery: if tools return repeated connection errors or timeouts, use configure(what="restart") to force-restart the daemon. This works even when the daemon is completely unresponsive.
- Token savings: pass summary=true to observe or analyze for compact responses (~60-70% smaller). Set once per session: configure(what="store", store_action="save", namespace="session", key="response_mode", data={"summary":true}). Use limit=N on interact(what="list_interactive") to cap returned elements.
- For routing help, read gasoline://capabilities. For detailed docs, read gasoline://guide. For quick examples, read gasoline://quickstart.`

// MCPHandler owns JSON-RPC request routing and response post-processing for MCP.
//
// Invariants:
// - toolHandler is expected to be set once during bootstrap before serving requests.
// - telemetryCursors is guarded by telemetryMu.
//
// Failure semantics:
// - Unknown methods/tools return JSON-RPC method-not-found errors.
// - Notification requests (no id) intentionally produce no response.
type MCPHandler struct {
	server      *Server
	toolHandler ToolHandlerInterface
	version     string

	telemetryMu      sync.Mutex
	telemetryCursors map[string]passiveTelemetryCursor
}

// ToolHandlerInterface defines the minimal tool handler interface.
type ToolHandlerInterface interface {
	GetCapture() *capture.Capture
	GetToolCallLimiter() RateLimiter
	GetRedactionEngine() RedactionEngine
	ToolsList() []MCPTool
	HandleToolCall(req JSONRPCRequest, name string, arguments json.RawMessage) (JSONRPCResponse, bool)
}

// RateLimiter interface for tool call rate limiting.
type RateLimiter interface {
	Allow() bool
}

// RedactionEngine interface for response redaction.
type RedactionEngine interface {
	RedactJSON(data json.RawMessage) json.RawMessage
	RedactMapValues(data map[string]any) map[string]any
}

// NewMCPHandler creates a new MCP handler.
func NewMCPHandler(server *Server, version string) *MCPHandler {
	return &MCPHandler{
		server:           server,
		version:          version,
		telemetryCursors: make(map[string]passiveTelemetryCursor),
	}
}

// SetToolHandler injects the tool execution backend.
//
// Invariants:
// - Intended for one-time startup wiring; runtime swapping is unsupported.
func (h *MCPHandler) SetToolHandler(th ToolHandlerInterface) {
	h.toolHandler = th
}
