// Purpose: Implements save_state, load_state, list_states, and delete_state with form, storage, and cookie capture/restore.
// Why: Enables agents to checkpoint and restore full page state (DOM + storage + cookies) for deterministic test scenarios.
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

func (h *ToolHandler) handleStateSave(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		SnapshotName string `json:"snapshot_name"`
		Name         string `json:"name"` // backward-compatible alias
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}

	snapshotName := params.SnapshotName
	if snapshotName == "" {
		snapshotName = params.Name
	}
	if snapshotName == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'snapshot_name' is missing", "Add the 'snapshot_name' parameter (legacy alias: 'name')", withParam("snapshot_name"))}
	}

	if h.sessionStoreImpl == nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrNotInitialized, "Session store not initialized", "Internal error — do not retry")}
	}

	_, tabID, tabURL := h.capture.GetTrackingStatus()
	tabTitle := h.capture.GetTrackedTabTitle()

	stateData := map[string]any{
		"url":      tabURL,
		"title":    tabTitle,
		"tab_id":   tabID,
		"saved_at": time.Now().Format(time.RFC3339),
	}

	// State capture — always produces a status for the response
	capture := h.captureState(req)
	if capture.Status == act.StateCaptureStatusCaptured && capture.Data != nil {
		for _, field := range act.StateDataFields {
			if v, ok := capture.Data[field]; ok {
				stateData[field] = v
			}
		}
	}

	// Server-side redaction: scrub sensitive values before persisting to disk (#132)
	if re := h.GetRedactionEngine(); re != nil {
		stateData = re.RedactMapValues(stateData)
	}

	data, err := json.Marshal(stateData)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInternal, "Failed to serialize state: "+err.Error(), "Internal error — do not retry")}
	}

	if err := h.sessionStoreImpl.Save(act.StateNamespace, snapshotName, data); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInternal, "Failed to save state: "+err.Error(), "Internal error — check storage")}
	}

	h.recordAIAction("save_state", tabURL, map[string]any{"snapshot_name": snapshotName})

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("State saved", map[string]any{
		"status":        "saved",
		"snapshot_name": snapshotName,
		"state_capture": capture.Status,
		"state": map[string]any{
			"url":   tabURL,
			"title": tabTitle,
		},
	})}
}

func (h *ToolHandler) handleStateLoad(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		SnapshotName string `json:"snapshot_name"`
		Name         string `json:"name"` // backward-compatible alias
		IncludeURL   bool   `json:"include_url,omitempty"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}

	snapshotName := params.SnapshotName
	if snapshotName == "" {
		snapshotName = params.Name
	}
	if snapshotName == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'snapshot_name' is missing", "Add the 'snapshot_name' parameter (legacy alias: 'name')", withParam("snapshot_name"))}
	}

	if h.sessionStoreImpl == nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrNotInitialized, "Session store not initialized", "Internal error — do not retry")}
	}

	data, err := h.sessionStoreImpl.Load(act.StateNamespace, snapshotName)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrNoData, "State not found: "+snapshotName, "Use interact with action='list_states' to see available snapshots", h.diagnosticHint())}
	}

	var stateData map[string]any
	if err := json.Unmarshal(data, &stateData); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInternal, "Failed to parse state data", "Internal error — state may be corrupted")}
	}

	if params.IncludeURL {
		h.queueStateNavigation(req, stateData)
	}

	responseData := map[string]any{
		"status":        "loaded",
		"snapshot_name": snapshotName,
		"state":         stateData,
	}

	formValues, _ := stateData["form_values"].(map[string]any)
	scrollPos, _ := stateData["scroll_position"].(map[string]any)
	localStorage, _ := stateData["local_storage"].(map[string]any)
	sessionStorage, _ := stateData["session_storage"].(map[string]any)
	cookies, _ := stateData["cookies"].(map[string]any)

	hasData := len(formValues) > 0 || len(localStorage) > 0 || len(sessionStorage) > 0 || len(cookies) > 0

	if !hasData {
		responseData["state_restore"] = act.StateRestoreStatusNoData
	} else if !h.capture.IsPilotActionAllowed() {
		responseData["state_restore"] = act.StateRestoreStatusPilotDisabled
	} else if !h.capture.IsExtensionConnected() {
		responseData["state_restore"] = act.StateRestoreStatusExtensionDown
	} else {
		restoreCorrelationID := h.queueStateRestore(req, formValues, scrollPos, localStorage, sessionStorage, cookies)
		responseData["state_restore"] = act.StateRestoreStatusQueued
		responseData["restore_correlation_id"] = restoreCorrelationID
	}

	h.recordAIAction("load_state", "", map[string]any{"snapshot_name": snapshotName})

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("State loaded", responseData)}
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

func (h *ToolHandler) handleStateList(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	if h.sessionStoreImpl == nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrNotInitialized, "Session store not initialized", "Internal error — do not retry")}
	}

	keys, err := h.sessionStoreImpl.List(act.StateNamespace)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInternal, "Failed to list states: "+err.Error(), "Internal error — do not retry")}
	}

	states := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		states = append(states, h.buildStateEntry(key))
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("States listed", map[string]any{
		"states": states,
		"count":  len(states),
	})}
}

// buildStateEntry loads metadata for a single saved state key and returns an entry map.
func (h *ToolHandler) buildStateEntry(key string) map[string]any {
	entry := map[string]any{"name": key}
	data, err := h.sessionStoreImpl.Load(act.StateNamespace, key)
	if err != nil {
		return entry
	}
	var stateData map[string]any
	if json.Unmarshal(data, &stateData) != nil {
		return entry
	}
	for _, field := range []string{"url", "title", "saved_at"} {
		if v, ok := stateData[field].(string); ok {
			entry[field] = v
		}
	}
	return entry
}

func (h *ToolHandler) handleStateDelete(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		SnapshotName string `json:"snapshot_name"`
		Name         string `json:"name"` // backward-compatible alias
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}

	snapshotName := params.SnapshotName
	if snapshotName == "" {
		snapshotName = params.Name
	}
	if snapshotName == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'snapshot_name' is missing", "Add the 'snapshot_name' parameter (legacy alias: 'name')", withParam("snapshot_name"))}
	}

	if h.sessionStoreImpl == nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrNotInitialized, "Session store not initialized", "Internal error — do not retry")}
	}

	if err := h.sessionStoreImpl.Delete(act.StateNamespace, snapshotName); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrNoData, "State not found: "+snapshotName, "Use interact with action='list_states' to see available snapshots", h.diagnosticHint())}
	}

	h.recordAIAction("delete_state", "", map[string]any{"snapshot_name": snapshotName})

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("State deleted", map[string]any{
		"status":        "deleted",
		"snapshot_name": snapshotName,
	})}
}
