// deps.go — Dependency injection for the toolinteract sub-package.
// Purpose: Declares the external dependencies interact handlers need from the main package.
// Why: Decouples interact handlers from the main package's god object without circular imports.

package toolinteract

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

// GuardCheck mirrors the main package's guardCheck type.
type GuardCheck = func(req mcp.JSONRPCRequest, opts ...func(*mcp.StructuredError)) (mcp.JSONRPCResponse, bool)

// Deps declares all external dependencies interact handlers need from the caller.
// The main package's ToolHandler satisfies this interface via tools_interact_adapter.go.
type Deps interface {
	// -- Gate checks --

	// RequirePilot checks that pilot mode is enabled.
	RequirePilot(req mcp.JSONRPCRequest, opts ...func(*mcp.StructuredError)) (mcp.JSONRPCResponse, bool)

	// RequireExtension checks that the extension is connected.
	RequireExtension(req mcp.JSONRPCRequest, opts ...func(*mcp.StructuredError)) (mcp.JSONRPCResponse, bool)

	// RequireTabTracking checks that tab tracking is active.
	RequireTabTracking(req mcp.JSONRPCRequest, opts ...func(*mcp.StructuredError)) (mcp.JSONRPCResponse, bool)

	// RequireCSPClear checks CSP restrictions for a given world.
	RequireCSPClear(req mcp.JSONRPCRequest, world string) (mcp.JSONRPCResponse, bool)

	// -- Command dispatch --

	// EnqueuePendingQuery queues a command for the extension.
	EnqueuePendingQuery(req mcp.JSONRPCRequest, query queries.PendingQuery, timeout time.Duration) (mcp.JSONRPCResponse, bool)

	// MaybeWaitForCommand waits for a command result or returns queued status.
	MaybeWaitForCommand(req mcp.JSONRPCRequest, correlationID string, args json.RawMessage, queuedSummary string) mcp.JSONRPCResponse

	// -- Capture store --

	// Capture returns the capture store.
	Capture() *capture.Store

	// -- Recording --

	// RecordAIAction records an AI-driven action to the enhanced actions buffer.
	RecordAIAction(action, url string, extra map[string]any)

	// RecordAIEnhancedAction records a fully populated AI-driven action.
	RecordAIEnhancedAction(action capture.EnhancedAction)

	// RecordDOMPrimitiveAction records a DOM primitive action for reproduction.
	RecordDOMPrimitiveAction(action, selector, text, value string)

	// -- Cross-tool dispatch --

	// ToolInteract dispatches an interact request (used by batch for nested calls).
	ToolInteract(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse

	// ToolAnalyze dispatches an analyze request (used by a11y+SARIF workflow).
	ToolAnalyze(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse

	// ToolExportSARIF dispatches a SARIF export request.
	ToolExportSARIF(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse

	// -- Response enrichment --

	// EnrichNavigateResponse appends page content to a navigate response.
	EnrichNavigateResponse(resp mcp.JSONRPCResponse, req mcp.JSONRPCRequest, tabID int) mcp.JSONRPCResponse

	// InjectCSPBlockedActions adds CSP-blocked action warnings to a response.
	InjectCSPBlockedActions(resp mcp.JSONRPCResponse) mcp.JSONRPCResponse

	// -- Screenshot/observe proxies --

	// GetScreenshot captures a screenshot via the observe tool.
	GetScreenshot(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse

	// GetPageInfo returns page info via the observe tool.
	GetPageInfo(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse

	// -- Annotation store --

	// MarkDrawStarted signals that draw mode has been initiated.
	MarkDrawStarted()

	// -- Server info --

	// GetListenPort returns the daemon's listening port.
	GetListenPort() int

	// -- Evidence capture --

	// DefaultEvidenceCapture captures an evidence screenshot.
	DefaultEvidenceCapture(clientID string) EvidenceShot

	// -- Session store --

	// RequireSessionStore checks that the session store is available.
	RequireSessionStore(req mcp.JSONRPCRequest) (mcp.JSONRPCResponse, bool)

	// DiagnosticHint returns a StructuredError option for diagnostic hints.
	DiagnosticHint() func(*mcp.StructuredError)

	// GetRedactionEngine returns the redaction engine (may be nil).
	GetRedactionEngine() MapValueRedactor

	// GetCommandResult retrieves a command result by correlation ID.
	GetCommandResult(correlationID string) (*queries.CommandResult, bool)

	// -- Shared concurrency --

	// GetReplayMu returns the shared mutex for batch/replay serialization.
	GetReplayMu() *sync.Mutex
}

// MapValueRedactor is the narrow redaction interface for toolinteract.
// It intentionally declares only the single method this package needs,
// rather than duplicating the full redaction.Redactor interface.
type MapValueRedactor interface {
	RedactMapValues(m map[string]any) map[string]any
}
