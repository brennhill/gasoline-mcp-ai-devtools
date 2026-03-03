// Purpose: Implements navigate_and_document workflow for click-driven page transitions.
// Why: Reduces multi-call navigation documentation loops into one action with URL-change + stability guards.
// Docs: docs/features/feature/interact-explore/index.md

package main

import (
	"encoding/json"
	"fmt"
	"time"
)

// handleNavigateAndDocument performs click-based navigation, waits for URL/stability,
// then enriches the response with compact page context (url/title/tab_id).
func (h *interactActionHandler) handleNavigateAndDocument(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		TimeoutMs        int   `json:"timeout_ms,omitempty"`
		StabilityMs      int   `json:"stability_ms,omitempty"`
		TabID            int   `json:"tab_id,omitempty"`
		WaitForURLChange *bool `json:"wait_for_url_change,omitempty"`
		WaitForStable    *bool `json:"wait_for_stable,omitempty"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again"),
		}
	}

	waitForURLChange := true
	if params.WaitForURLChange != nil {
		waitForURLChange = *params.WaitForURLChange
	}
	waitForStable := true
	if params.WaitForStable != nil {
		waitForStable = *params.WaitForStable
	}

	if resp, blocked := h.validateNavigateAndDocumentTab(req, params.TabID); blocked {
		return resp
	}

	workflowStart := time.Now()
	beforeURL := h.currentTrackedURL(req)

	clickArgs := filterNavigateAndDocumentClickArgs(args)
	clickResp := h.handleDOMPrimitive(req, clickArgs, "click")
	if isErrorResponse(clickResp) {
		return clickResp
	}

	if waitForURLChange && beforeURL != "" {
		timeoutMs := params.TimeoutMs
		if params.TimeoutMs > 0 {
			var ok bool
			timeoutMs, ok = remainingNavigateAndDocumentTimeoutMs(workflowStart, params.TimeoutMs)
			if !ok {
				return navigateAndDocumentTimeoutBudgetExceeded(req, "wait_for_url_change")
			}
		} else if timeoutMs <= 0 {
			timeoutMs = 5000
		}
		if _, changed := h.waitForTrackedURLChange(req, beforeURL, timeoutMs); !changed {
			return JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: mcpStructuredError(
					ErrExtTimeout,
					"URL did not change after click within timeout",
					"Increase timeout_ms, disable wait_for_url_change, or verify the click target triggers navigation.",
					withParam("wait_for_url_change"),
				),
			}
		}
	}

	if waitForStable {
		waitArgsMap := map[string]any{
			"action": "wait_for_stable",
		}
		if params.StabilityMs > 0 {
			waitArgsMap["stability_ms"] = params.StabilityMs
		}
		if params.TimeoutMs > 0 {
			timeoutMs, ok := remainingNavigateAndDocumentTimeoutMs(workflowStart, params.TimeoutMs)
			if !ok {
				return navigateAndDocumentTimeoutBudgetExceeded(req, "wait_for_stable")
			}
			waitArgsMap["timeout_ms"] = timeoutMs
		}
		waitArgs, _ := json.Marshal(waitArgsMap)
		waitResp := h.handleWaitForStable(req, waitArgs)
		if isErrorResponse(waitResp) {
			return waitResp
		}
	}

	return h.appendPageContextToResponse(clickResp, req)
}

// filterNavigateAndDocumentClickArgs keeps only click-relevant fields.
func filterNavigateAndDocumentClickArgs(args json.RawMessage) json.RawMessage {
	var raw map[string]any
	if err := json.Unmarshal(args, &raw); err != nil || raw == nil {
		return args
	}

	click := make(map[string]any, 12)
	for _, key := range []string{
		"selector", "scope_selector", "scope_rect", "annotation_rect",
		"element_id", "index", "index_generation", "nth",
		"x", "y",
		"tab_id", "frame", "timeout_ms", "reason",
	} {
		if v, ok := raw[key]; ok {
			click[key] = v
		}
	}
	encoded, err := json.Marshal(click)
	if err != nil {
		return args
	}
	return encoded
}

func (h *interactActionHandler) currentTrackedURL(req JSONRPCRequest) string {
	_, _, trackedURL := h.parent.capture.GetTrackingStatus()
	if trackedURL != "" {
		return trackedURL
	}
	if pageCtx, ok := h.readPageContext(req); ok {
		if url, ok := pageCtx["url"].(string); ok {
			return url
		}
	}
	return ""
}

func (h *interactActionHandler) waitForTrackedURLChange(req JSONRPCRequest, beforeURL string, timeoutMs int) (string, bool) {
	if timeoutMs <= 0 {
		timeoutMs = 5000
	}
	deadline := time.Now().Add(time.Duration(timeoutMs) * time.Millisecond)
	lastURL := beforeURL
	for time.Now().Before(deadline) {
		lastURL = h.currentTrackedURL(req)
		if lastURL != "" && lastURL != beforeURL {
			return lastURL, true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return lastURL, false
}

// validateNavigateAndDocumentTab ensures workflow-level waits and page context are
// scoped to the currently tracked tab. Unlike plain click, this workflow derives
// post-action state from tracked page metadata.
func (h *interactActionHandler) validateNavigateAndDocumentTab(req JSONRPCRequest, tabID int) (JSONRPCResponse, bool) {
	if tabID <= 0 {
		return JSONRPCResponse{}, false
	}

	enabled, trackedTabID, _ := h.parent.capture.GetTrackingStatus()
	if !enabled || trackedTabID <= 0 || trackedTabID == tabID {
		return JSONRPCResponse{}, false
	}

	return JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: mcpStructuredError(
			ErrInvalidParam,
			fmt.Sprintf("navigate_and_document requires tracked tab_id=%d; got tab_id=%d", trackedTabID, tabID),
			"Switch tracking to the target tab first (interact what=switch_tab) or omit tab_id.",
			withParam("tab_id"),
		),
	}, true
}

// remainingNavigateAndDocumentTimeoutMs converts total workflow timeout into
// remaining stage timeout. Returns false when budget is exhausted.
func remainingNavigateAndDocumentTimeoutMs(workflowStart time.Time, totalTimeoutMs int) (int, bool) {
	if totalTimeoutMs <= 0 {
		return 0, false
	}
	remaining := totalTimeoutMs - int(time.Since(workflowStart).Milliseconds())
	if remaining <= 0 {
		return 0, false
	}
	return remaining, true
}

func navigateAndDocumentTimeoutBudgetExceeded(req JSONRPCRequest, stage string) JSONRPCResponse {
	return JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: mcpStructuredError(
			ErrExtTimeout,
			fmt.Sprintf("timeout_ms exhausted before %s stage", stage),
			"Increase timeout_ms or disable one of the workflow wait stages.",
			withParam("timeout_ms"),
		),
	}
}
