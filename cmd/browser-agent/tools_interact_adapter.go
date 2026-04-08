// tools_interact_adapter.go — Bridges the toolinteract package to the main ToolHandler.
// Purpose: Implements toolinteract.Deps interface by delegating to *ToolHandler methods.
// Why: Keeps the toolinteract package decoupled from the main package's god object.

package main

import (
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolinteract"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe"
)

// interactDepsAdapter implements toolinteract.Deps by delegating to *ToolHandler.
type interactDepsAdapter struct {
	h *ToolHandler
}

// Compile-time check that interactDepsAdapter satisfies toolinteract.Deps.
var _ toolinteract.Deps = (*interactDepsAdapter)(nil)

// buildInteractDeps constructs a toolinteract.Deps wired to the given ToolHandler.
func buildInteractDeps(h *ToolHandler) toolinteract.Deps {
	return &interactDepsAdapter{h: h}
}

// -- Gate checks --

func (a *interactDepsAdapter) RequirePilot(req mcp.JSONRPCRequest, opts ...func(*mcp.StructuredError)) (mcp.JSONRPCResponse, bool) {
	return a.h.requirePilot(req, opts...)
}

func (a *interactDepsAdapter) RequireExtension(req mcp.JSONRPCRequest, opts ...func(*mcp.StructuredError)) (mcp.JSONRPCResponse, bool) {
	return a.h.requireExtension(req, opts...)
}

func (a *interactDepsAdapter) RequireTabTracking(req mcp.JSONRPCRequest, opts ...func(*mcp.StructuredError)) (mcp.JSONRPCResponse, bool) {
	return a.h.requireTabTracking(req, opts...)
}

func (a *interactDepsAdapter) RequireCSPClear(req mcp.JSONRPCRequest, world string) (mcp.JSONRPCResponse, bool) {
	return a.h.requireCSPClear(req, world)
}

// -- Command dispatch --

func (a *interactDepsAdapter) EnqueuePendingQuery(req mcp.JSONRPCRequest, query queries.PendingQuery, timeout time.Duration) (mcp.JSONRPCResponse, bool) {
	return a.h.EnqueuePendingQuery(req, query, timeout)
}

func (a *interactDepsAdapter) MaybeWaitForCommand(req mcp.JSONRPCRequest, correlationID string, args json.RawMessage, queuedSummary string) mcp.JSONRPCResponse {
	return a.h.MaybeWaitForCommand(req, correlationID, args, queuedSummary)
}

// -- Capture store --

func (a *interactDepsAdapter) Capture() *capture.Store {
	return a.h.capture
}

// -- Recording --

func (a *interactDepsAdapter) RecordAIAction(action, url string, extra map[string]any) {
	a.h.recordAIAction(action, url, extra)
}

func (a *interactDepsAdapter) RecordAIEnhancedAction(action capture.EnhancedAction) {
	a.h.recordAIEnhancedAction(action)
}

func (a *interactDepsAdapter) RecordDOMPrimitiveAction(action, selector, text, value string) {
	a.h.recordDOMPrimitiveAction(action, selector, text, value)
}

// -- Cross-tool dispatch --

func (a *interactDepsAdapter) ToolInteract(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	return a.h.toolInteract(req, args)
}

func (a *interactDepsAdapter) ToolAnalyze(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	return a.h.toolAnalyze(req, args)
}

func (a *interactDepsAdapter) ToolExportSARIF(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	return a.h.toolExportSARIF(req, args)
}

// -- Response enrichment --

func (a *interactDepsAdapter) EnrichNavigateResponse(resp mcp.JSONRPCResponse, req mcp.JSONRPCRequest, tabID int) mcp.JSONRPCResponse {
	return a.h.enrichNavigateResponse(resp, req, tabID)
}

func (a *interactDepsAdapter) InjectCSPBlockedActions(resp mcp.JSONRPCResponse) mcp.JSONRPCResponse {
	return a.h.injectCSPBlockedActions(resp)
}

// -- Screenshot/observe proxies --

func (a *interactDepsAdapter) GetScreenshot(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	return observe.GetScreenshot(a.h, req, args)
}

func (a *interactDepsAdapter) GetPageInfo(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	return observe.GetPageInfo(a.h, req, args)
}

// -- Annotation store --

func (a *interactDepsAdapter) MarkDrawStarted() {
	if a.h.annotationStore != nil {
		a.h.annotationStore.MarkDrawStarted()
	}
}

// -- Server info --

func (a *interactDepsAdapter) GetListenPort() int {
	if a.h.server != nil {
		return a.h.server.getListenPort()
	}
	return defaultPort
}

// -- Evidence capture --

func (a *interactDepsAdapter) DefaultEvidenceCapture(clientID string) toolinteract.EvidenceShot {
	return defaultEvidenceCaptureImpl(a.h, clientID)
}

// -- Session store --

func (a *interactDepsAdapter) RequireSessionStore(req mcp.JSONRPCRequest) (mcp.JSONRPCResponse, bool) {
	return a.h.requireSessionStore(req)
}

func (a *interactDepsAdapter) DiagnosticHint() func(*mcp.StructuredError) {
	return a.h.diagnosticHint()
}

func (a *interactDepsAdapter) GetRedactionEngine() toolinteract.MapValueRedactor {
	return a.h.GetRedactionEngine()
}

func (a *interactDepsAdapter) GetCommandResult(correlationID string) (*queries.CommandResult, bool) {
	return a.h.capture.GetCommandResult(correlationID)
}

// -- Shared concurrency --

func (a *interactDepsAdapter) GetReplayMu() *sync.Mutex {
	return &replayMu
}

// interactAction returns the interact action handler from the toolinteract package.
func (h *ToolHandler) interactAction() *toolinteract.InteractActionHandler {
	return h.interactActionHandler
}

// stateInteract returns the state interact handler from the toolinteract package.
func (h *ToolHandler) stateInteract() *toolinteract.StateInteractHandler {
	return h.stateInteractHandler
}

// defaultEvidenceCaptureImpl captures an evidence screenshot using the ToolHandler's capture store.
func defaultEvidenceCaptureImpl(h *ToolHandler, clientID string) toolinteract.EvidenceShot {
	if h == nil || h.capture == nil {
		return toolinteract.EvidenceShot{Error: "capture_not_initialized"}
	}
	enabled, _, _ := h.capture.GetTrackingStatus()
	if !enabled {
		return toolinteract.EvidenceShot{Error: "no_tracked_tab"}
	}

	queryID, qerr := h.capture.CreatePendingQueryWithTimeout(
		queries.PendingQuery{
			Type:   "screenshot",
			Params: json.RawMessage(`{}`),
		},
		12*time.Second,
		clientID,
	)
	if qerr != nil {
		return toolinteract.EvidenceShot{Error: "queue_full: " + qerr.Error()}
	}

	raw, err := h.capture.WaitForResult(queryID, 12*time.Second)
	if err != nil {
		return toolinteract.EvidenceShot{Error: "screenshot_timeout: " + err.Error()}
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return toolinteract.EvidenceShot{Error: "screenshot_parse_error: " + err.Error()}
	}

	if errMsg, ok := payload["error"].(string); ok && strings.TrimSpace(errMsg) != "" {
		return toolinteract.EvidenceShot{Error: strings.TrimSpace(errMsg)}
	}

	path, _ := payload["path"].(string)
	filename, _ := payload["filename"].(string)
	path = strings.TrimSpace(path)
	filename = strings.TrimSpace(filename)
	if path == "" {
		return toolinteract.EvidenceShot{
			Filename: filename,
			Error:    "screenshot_missing_path",
		}
	}

	return toolinteract.EvidenceShot{
		Path:     path,
		Filename: filename,
	}
}
