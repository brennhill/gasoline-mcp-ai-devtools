// Purpose: Implements browser-side state capture and restore command queueing.
// Why: Isolates extension command orchestration from MCP request/response handlers.
// Docs: docs/features/feature/state-time-travel/index.md

package main

import (
	"encoding/json"
	"time"

	"github.com/dev-console/dev-console/internal/queries"
	act "github.com/dev-console/dev-console/internal/tools/interact"
)

// stateCaptureResult — type alias delegated to internal/tools/interact package.
type stateCaptureResult = act.StateCaptureResult

// captureState attempts to capture form values, scroll position, and web storage from the browser.
// Always returns a stateCaptureResult with an explicit Status the caller can surface to the LLM.
func (h *ToolHandler) captureState(req JSONRPCRequest) stateCaptureResult {
	if !h.capture.IsPilotActionAllowed() {
		return stateCaptureResult{Status: act.StateCaptureStatusPilotDisabled}
	}
	if !h.capture.IsExtensionConnected() {
		return stateCaptureResult{Status: act.StateCaptureStatusExtensionDisconnected}
	}

	correlationID := newCorrelationID("state_capture")

	scriptArgs, _ := json.Marshal(map[string]any{
		"action": "execute_js",
		"script": act.StateCaptureScript,
		"world":  "main",
	})

	query := queries.PendingQuery{
		Type:          "execute",
		Params:        scriptArgs,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	cmd, found := h.capture.WaitForCommand(correlationID, 5*time.Second)
	if !found || cmd.Status == "pending" {
		return stateCaptureResult{Status: act.StateCaptureStatusTimeout}
	}
	if cmd.Error != "" {
		return stateCaptureResult{Status: act.StateCaptureStatusError}
	}
	if cmd.Status != "complete" || len(cmd.Result) == 0 {
		return stateCaptureResult{Status: act.StateCaptureStatusError}
	}

	captureData, err := act.ParseCapturedStatePayload(cmd.Result)
	if err != nil {
		return stateCaptureResult{Status: act.StateCaptureStatusError}
	}

	return stateCaptureResult{Status: act.StateCaptureStatusCaptured, Data: captureData}
}

// queueStateRestore queues a JS execute command to restore form values, scroll position,
// localStorage, sessionStorage, and cookies. This is fire-and-forget.
func (h *ToolHandler) queueStateRestore(req JSONRPCRequest, formValues, scrollPos, localStorage, sessionStorage, cookies map[string]any) string {
	correlationID := newCorrelationID("state_restore")

	script := act.BuildStateRestoreScript(formValues, scrollPos, localStorage, sessionStorage, cookies)
	scriptArgs, _ := json.Marshal(map[string]any{
		"action": "execute_js",
		"script": script,
		"world":  "main",
	})

	query := queries.PendingQuery{
		Type:          "execute",
		Params:        scriptArgs,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	return correlationID
}

// queueStateNavigation queues a navigation to the saved URL if pilot is enabled
// and the state contains a non-empty URL. Mutates stateData to add tracking fields.
func (h *ToolHandler) queueStateNavigation(req JSONRPCRequest, stateData map[string]any) {
	savedURL, ok := stateData["url"].(string)
	if !ok || savedURL == "" || !h.capture.IsPilotActionAllowed() || !h.capture.IsExtensionConnected() {
		return
	}
	correlationID := newCorrelationID("nav")
	// Error impossible: map contains only string values
	navArgs, _ := json.Marshal(map[string]any{"action": "navigate", "url": savedURL})
	query := queries.PendingQuery{
		Type:          "browser_action",
		Params:        navArgs,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)
	stateData["navigation_queued"] = true
	stateData["correlation_id"] = correlationID
}
