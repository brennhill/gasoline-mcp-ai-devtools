// Purpose: Handles navigate, refresh, back, forward, new_tab, switch_tab, and close_tab browser actions with perf_diff capture.
// Why: Groups all browser navigation and tab management actions into a single handler file.
// Docs: docs/features/feature/interact-explore/index.md

package main

import (
	"encoding/json"
)

func (h *ToolHandler) handleHighlight(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return h.interactAction().handleHighlightImpl(req, args)
}

func (h *ToolHandler) handleExecuteJS(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return h.interactAction().handleExecuteJSImpl(req, args)
}

func (h *ToolHandler) handleBrowserActionNavigate(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return h.interactAction().handleBrowserActionNavigateImpl(req, args)
}

// enrichNavigateResponse moved to tools_interact_content.go

func (h *ToolHandler) handleBrowserActionRefresh(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return h.interactAction().handleBrowserActionRefreshImpl(req, args)
}

// stashPerfSnapshot saves the current performance snapshot as a "before" baseline
// for perf_diff computation, keyed by correlation ID.
func (h *ToolHandler) stashPerfSnapshot(correlationID string) {
	h.interactAction().stashPerfSnapshotImpl(correlationID)
}

func (h *ToolHandler) handleBrowserActionBack(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return h.interactAction().handleBrowserActionBackImpl(req, args)
}

func (h *ToolHandler) handleBrowserActionForward(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return h.interactAction().handleBrowserActionForwardImpl(req, args)
}

func (h *ToolHandler) resolveNavigateURL(rawURL string) (string, error) {
	return h.interactAction().resolveNavigateURLImpl(rawURL)
}

// handleScreenshotAlias provides backward compatibility for clients that call
// interact({action:"screenshot"}). The canonical API remains observe({what:"screenshot"}).
func (h *ToolHandler) handleScreenshotAlias(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return h.interactAction().handleScreenshotAliasImpl(req, args)
}

func (h *ToolHandler) handleSubtitle(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return h.interactAction().handleSubtitleImpl(req, args)
}
