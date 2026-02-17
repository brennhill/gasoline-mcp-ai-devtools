// tools_interact_workflows.go — High-level workflow primitives for interact tool.
// Implements compound actions that chain existing handlers to reduce agent call overhead.
package main

import (
	"encoding/json"
	"fmt"
	"time"
)

// WorkflowStep records a single step's outcome within a workflow trace.
type WorkflowStep struct {
	Action        string `json:"action"`
	CorrelationID string `json:"correlation_id,omitempty"`
	Status        string `json:"status"` // "success", "error", "skipped"
	TimingMs      int64  `json:"timing_ms"`
	Detail        string `json:"detail,omitempty"`
}

// handleNavigateAndWaitFor navigates to a URL, waits for a CSS selector to appear,
// and optionally returns page content — all in one call.
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

	// Step 1: Navigate
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

	// Step 2: Wait for selector
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
	waitResp := h.handleDOMPrimitive(req, waitArgs, "wait_for")
	trace = append(trace, WorkflowStep{
		Action:   "wait_for",
		Status:   responseStatus(waitResp),
		TimingMs: time.Since(stepStart).Milliseconds(),
		Detail:   params.WaitFor,
	})
	if isErrorResponse(waitResp) {
		return workflowResult(req, "navigate_and_wait_for", trace, waitResp, workflowStart)
	}

	// Step 3: Optional content enrichment
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

// FormField represents a single field to fill in a form workflow.
type FormField struct {
	Selector string `json:"selector"`
	Value    string `json:"value"`
	Index    *int   `json:"index,omitempty"`
}

// handleFillFormAndSubmit fills multiple form fields and clicks a submit button.
func (h *ToolHandler) handleFillFormAndSubmit(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Fields          []FormField `json:"fields"`
		SubmitSelector  string      `json:"submit_selector"`
		SubmitIndex     *int        `json:"submit_index,omitempty"`
		TabID           int         `json:"tab_id,omitempty"`
		TimeoutMs       int         `json:"timeout_ms,omitempty"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}
	if len(params.Fields) == 0 {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'fields' is empty", "Provide at least one {selector, value} field entry", withParam("fields"))}
	}
	if params.SubmitSelector == "" && params.SubmitIndex == nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'submit_selector' or 'submit_index' is missing", "Add the selector or index of the submit button", withParam("submit_selector"))}
	}
	if params.TimeoutMs <= 0 {
		params.TimeoutMs = 15_000
	}

	trace := make([]WorkflowStep, 0, len(params.Fields)+1)
	workflowStart := time.Now()

	// Fill each field
	for i, field := range params.Fields {
		if field.Selector == "" && field.Index == nil {
			trace = append(trace, WorkflowStep{
				Action: fmt.Sprintf("type[%d]", i),
				Status: "error",
				Detail: "Missing selector and index",
			})
			return workflowResult(req, "fill_form_and_submit", trace,
				JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
					ErrMissingParam,
					fmt.Sprintf("Field %d missing 'selector' or 'index'", i),
					"Each field needs a 'selector' or 'index'",
					withParam("fields"),
				)}, workflowStart)
		}

		typeArgs := map[string]any{
			"action":   "type",
			"text":     field.Value,
			"clear":    true,
			"tab_id":   params.TabID,
		}
		if field.Index != nil {
			typeArgs["index"] = *field.Index
		} else {
			typeArgs["selector"] = field.Selector
		}
		argsJSON, _ := json.Marshal(typeArgs)

		stepStart := time.Now()
		typeResp := h.handleDOMPrimitive(req, argsJSON, "type")
		selectorLabel := field.Selector
		if field.Index != nil {
			selectorLabel = fmt.Sprintf("index:%d", *field.Index)
		}
		trace = append(trace, WorkflowStep{
			Action:   fmt.Sprintf("type[%d]", i),
			Status:   responseStatus(typeResp),
			TimingMs: time.Since(stepStart).Milliseconds(),
			Detail:   selectorLabel,
		})
		if isErrorResponse(typeResp) {
			return workflowResult(req, "fill_form_and_submit", trace, typeResp, workflowStart)
		}
	}

	// Click submit
	clickArgs := map[string]any{
		"action": "click",
		"tab_id": params.TabID,
	}
	if params.SubmitIndex != nil {
		clickArgs["index"] = *params.SubmitIndex
	} else {
		clickArgs["selector"] = params.SubmitSelector
	}
	clickJSON, _ := json.Marshal(clickArgs)

	stepStart := time.Now()
	clickResp := h.handleDOMPrimitive(req, clickJSON, "click")
	trace = append(trace, WorkflowStep{
		Action:   "click_submit",
		Status:   responseStatus(clickResp),
		TimingMs: time.Since(stepStart).Milliseconds(),
		Detail:   params.SubmitSelector,
	})

	return workflowResult(req, "fill_form_and_submit", trace, clickResp, workflowStart)
}

// handleRunA11yAndExportSARIF runs accessibility audit then exports SARIF in one call.
func (h *ToolHandler) handleRunA11yAndExportSARIF(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Scope  string `json:"scope,omitempty"`
		SaveTo string `json:"save_to,omitempty"`
		TabID  int    `json:"tab_id,omitempty"`
	}
	lenientUnmarshal(args, &params)

	trace := make([]WorkflowStep, 0, 2)
	workflowStart := time.Now()

	// Step 1: Run accessibility audit
	a11yArgs, _ := json.Marshal(map[string]any{
		"what":   "accessibility",
		"scope":  params.Scope,
		"tab_id": params.TabID,
	})
	stepStart := time.Now()
	a11yResp := h.toolAnalyze(req, a11yArgs)
	trace = append(trace, WorkflowStep{
		Action:   "analyze_accessibility",
		Status:   responseStatus(a11yResp),
		TimingMs: time.Since(stepStart).Milliseconds(),
	})
	if isErrorResponse(a11yResp) {
		return workflowResult(req, "run_a11y_and_export_sarif", trace, a11yResp, workflowStart)
	}

	// Step 2: Export as SARIF
	sarifArgs, _ := json.Marshal(map[string]any{
		"format":  "sarif",
		"scope":   params.Scope,
		"save_to": params.SaveTo,
	})
	stepStart = time.Now()
	sarifResp := h.toolGenerate(req, sarifArgs)
	trace = append(trace, WorkflowStep{
		Action:   "generate_sarif",
		Status:   responseStatus(sarifResp),
		TimingMs: time.Since(stepStart).Milliseconds(),
	})

	return workflowResult(req, "run_a11y_and_export_sarif", trace, sarifResp, workflowStart)
}

// ---- Workflow helpers ----

// isErrorResponse checks if a JSONRPCResponse represents an error.
func isErrorResponse(resp JSONRPCResponse) bool {
	if resp.Error != nil {
		return true
	}
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err == nil {
		return result.IsError
	}
	return false
}

// responseStatus returns "success" or "error" based on the response.
func responseStatus(resp JSONRPCResponse) string {
	if isErrorResponse(resp) {
		return "error"
	}
	return "success"
}

// workflowResult wraps the final step's response with workflow metadata (trace + timing).
func workflowResult(req JSONRPCRequest, workflow string, trace []WorkflowStep, lastResp JSONRPCResponse, start time.Time) JSONRPCResponse {
	totalMs := time.Since(start).Milliseconds()

	// Count steps by status
	successCount := 0
	for _, s := range trace {
		if s.Status == "success" {
			successCount++
		}
	}
	allSuccess := successCount == len(trace)

	status := "success"
	if !allSuccess {
		status = "partial_failure"
	}

	summary := fmt.Sprintf("%s completed (%d/%d steps succeeded, %dms)", workflow, successCount, len(trace), totalMs)

	// If the last response was an error, use it as the base but add workflow context
	if isErrorResponse(lastResp) {
		status = "failed"
		summary = fmt.Sprintf("%s failed at step %d/%d (%dms)", workflow, len(trace), len(trace), totalMs)
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse(summary, map[string]any{
		"workflow":   workflow,
		"status":     status,
		"trace":      trace,
		"total_ms":   totalMs,
		"steps":      len(trace),
		"successful": successCount,
	})}
}
