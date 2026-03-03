// Purpose: Implements navigate_and_wait_for workflow that chains navigation + selector wait.
// Why: Isolates navigation workflow orchestration from form and audit workflows.
// Docs: docs/features/feature/form-filling/index.md

package main

import (
	"encoding/json"
	"time"
)

// handleNavigateAndWaitFor navigates to a URL, waits for a CSS selector to appear,
// and optionally returns page content — all in one call.
// Gates (requirePilot, requireExtension, requireTabTracking) are applied by the delegated handlers.
func (h *ToolHandler) handleNavigateAndWaitFor(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		URL            string `json:"url"`
		WaitFor        string `json:"wait_for"`
		TabID          int    `json:"tab_id,omitempty"`
		TimeoutMs      int    `json:"timeout_ms,omitempty"`
		IncludeContent bool   `json:"include_content,omitempty"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}
	if params.URL == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'url' is missing", "Add 'url' to navigate to", withParam("url"))}
	}
	if params.WaitFor == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'wait_for' is missing", "Add a CSS selector to wait for after navigation", withParam("wait_for"))}
	}
	if params.TimeoutMs <= 0 {
		params.TimeoutMs = 15_000
	}

	trace := make([]WorkflowStep, 0, 3)
	workflowStart := time.Now()

	// Step 1: Navigate.
	navArgs, _ := json.Marshal(map[string]any{
		"action": "navigate",
		"url":    params.URL,
		"tab_id": params.TabID,
	})
	stepStart := time.Now()
	navResp := h.handleBrowserActionNavigate(req, navArgs)
	trace = append(trace, WorkflowStep{
		Action:   "navigate",
		Status:   responseStatus(navResp),
		TimingMs: time.Since(stepStart).Milliseconds(),
		Detail:   params.URL,
	})
	if isErrorResponse(navResp) {
		return workflowResult(req, "navigate_and_wait_for", trace, navResp, workflowStart)
	}

	// Step 2: Wait for selector.
	elapsed := time.Since(workflowStart).Milliseconds()
	waitTimeout := params.TimeoutMs - int(elapsed)
	if waitTimeout < 1000 {
		waitTimeout = 1000
	}
	waitArgs, _ := json.Marshal(map[string]any{
		"action":     "wait_for",
		"selector":   params.WaitFor,
		"timeout_ms": waitTimeout,
		"tab_id":     params.TabID,
	})
	stepStart = time.Now()
	waitResp := h.interactAction().handleDOMPrimitive(req, waitArgs, "wait_for")
	trace = append(trace, WorkflowStep{
		Action:   "wait_for",
		Status:   responseStatus(waitResp),
		TimingMs: time.Since(stepStart).Milliseconds(),
		Detail:   params.WaitFor,
	})
	if isErrorResponse(waitResp) {
		return workflowResult(req, "navigate_and_wait_for", trace, waitResp, workflowStart)
	}

	// Step 3: Optional content enrichment.
	if params.IncludeContent {
		stepStart = time.Now()
		navResp = h.enrichNavigateResponse(navResp, req, params.TabID)
		trace = append(trace, WorkflowStep{
			Action:   "get_content",
			Status:   "success",
			TimingMs: time.Since(stepStart).Milliseconds(),
		})
	}

	return workflowResult(req, "navigate_and_wait_for", trace, navResp, workflowStart)
}
