// tools_interact_state.go — MCP interact state management handlers.
// Implements save_state, load_state, list_states, delete_state with optional form capture.
//
// JSON CONVENTION: All fields MUST use snake_case. See .claude/refs/api-naming-standards.md
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dev-console/dev-console/internal/queries"
)

const stateNamespace = "saved_states"

// Form capture status values — always present in save_state response as "form_capture".
const (
	formCaptureStatusCaptured              = "captured"
	formCaptureStatusPilotDisabled         = "skipped_pilot_disabled"
	formCaptureStatusExtensionDisconnected = "skipped_extension_disconnected"
	formCaptureStatusTimeout               = "skipped_timeout"
	formCaptureStatusError                 = "skipped_error"
)

// Form restore status values — always present in load_state response as "form_restore".
const (
	formRestoreStatusQueued        = "queued"
	formRestoreStatusPilotDisabled = "skipped_pilot_disabled"
	formRestoreStatusExtensionDown = "skipped_extension_disconnected"
	formRestoreStatusNoFormData    = "skipped_no_form_data"
)

// formCaptureScript is the JS executed in the browser to capture form values and scroll position.
const formCaptureScript = `(() => {
  const sensitiveKeyPattern = /(pass(word)?|token|secret|api[_-]?key|auth|cookie|session|bearer|credential|otp|ssn|credit|card|cvv|cvc)/i;
  const sensitiveAutocompletePattern = /(password|one-time-code|cc-|credit-card|csc|cvv)/i;
  const sensitiveValuePattern = /^(eyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}|sk-[A-Za-z0-9_-]{16,}|gh[pousr]_[A-Za-z0-9_]{16,}|xox[baprs]-[A-Za-z0-9-]{10,})$/;

  function isSensitive(el, key, value) {
    const type = (el.type || '').toLowerCase();
    if (type === 'password' || type === 'hidden' || type === 'file') return true;

    const autocomplete = (el.getAttribute('autocomplete') || '').toLowerCase();
    if (sensitiveAutocompletePattern.test(autocomplete)) return true;

    const keyProbe = [key, el.name, el.id, el.getAttribute('aria-label'), el.getAttribute('placeholder')]
      .filter(Boolean)
      .join(' ');
    if (sensitiveKeyPattern.test(keyProbe)) return true;

    if (typeof value === 'string' && value.length >= 12 && sensitiveValuePattern.test(value.trim())) return true;
    return false;
  }

  const forms = {};
  document.querySelectorAll('input, textarea, select').forEach(el => {
    const key = el.id || el.name;
    if (!key) return;

    const rawValue = (el.type === 'checkbox' || el.type === 'radio') ? !!el.checked : String(el.value ?? '');
    if (isSensitive(el, key, rawValue)) return;

    if (el.type === 'checkbox' || el.type === 'radio') {
      forms[key] = el.checked;
    } else {
      forms[key] = el.value;
    }
  });
  return {
    form_values: forms,
    scroll_position: { x: window.scrollX, y: window.scrollY }
  };
})()`

// formCaptureResult holds the outcome of a form capture attempt.
type formCaptureResult struct {
	Status string         // one of formCaptureStatus* constants
	Data   map[string]any // non-nil only when Status == "captured"
}

func parseCapturedFormPayload(raw json.RawMessage) (map[string]any, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return nil, errors.New("empty form capture payload")
	}

	var envelope map[string]any
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, err
	}

	if successVal, hasSuccess := envelope["success"]; hasSuccess {
		success, _ := successVal.(bool)
		if !success {
			if msg, ok := envelope["message"].(string); ok && msg != "" {
				return nil, errors.New(msg)
			}
			if code, ok := envelope["error"].(string); ok && code != "" {
				return nil, errors.New(code)
			}
			return nil, errors.New("execute_js failed")
		}

		if resultObj, ok := envelope["result"].(map[string]any); ok {
			return resultObj, nil
		}
		if _, ok := envelope["form_values"]; ok {
			return envelope, nil
		}
		return nil, errors.New("execute_js result missing payload")
	}

	if _, ok := envelope["form_values"]; ok {
		return envelope, nil
	}
	return nil, errors.New("form capture payload missing form_values")
}

// buildFormRestoreScript builds a JS script that restores form values and scroll position.
// The form values and scroll position are embedded directly in the script as JSON literals.
func buildFormRestoreScript(formValues map[string]any, scrollPos map[string]any) string {
	formJSON, _ := json.Marshal(formValues)
	if scrollPos == nil {
		scrollPos = map[string]any{}
	}
	scrollJSON, _ := json.Marshal(scrollPos)
	return fmt.Sprintf(`(() => {
  const formValues = %s;
  const scrollPos = %s;
  Object.entries(formValues).forEach(([key, val]) => {
    const el = document.getElementById(key) || document.querySelector('[name="' + key + '"]');
    if (!el) return;
    if (el.type === 'checkbox' || el.type === 'radio') {
      el.checked = !!val;
    } else {
      el.value = String(val);
      el.dispatchEvent(new Event('input', {bubbles: true}));
    }
  });
  if (scrollPos && scrollPos.x !== undefined) {
    window.scrollTo(scrollPos.x, scrollPos.y);
  }
  return { restored: Object.keys(formValues).length };
})()`, string(formJSON), string(scrollJSON))
}

// captureFormState attempts to capture form values and scroll position from the browser.
// Always returns a formCaptureResult with an explicit Status the caller can surface to the LLM.
func (h *ToolHandler) captureFormState(req JSONRPCRequest) formCaptureResult {
	if !h.capture.IsPilotEnabled() {
		return formCaptureResult{Status: formCaptureStatusPilotDisabled}
	}
	if !h.capture.IsExtensionConnected() {
		return formCaptureResult{Status: formCaptureStatusExtensionDisconnected}
	}

	correlationID := fmt.Sprintf("state_capture_%d_%d", time.Now().UnixNano(), randomInt63())

	scriptArgs, _ := json.Marshal(map[string]any{
		"action": "execute_js",
		"script": formCaptureScript,
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
		return formCaptureResult{Status: formCaptureStatusTimeout}
	}
	if cmd.Error != "" {
		return formCaptureResult{Status: formCaptureStatusError}
	}
	if cmd.Status != "complete" || len(cmd.Result) == 0 {
		return formCaptureResult{Status: formCaptureStatusError}
	}

	captureData, err := parseCapturedFormPayload(cmd.Result)
	if err != nil {
		return formCaptureResult{Status: formCaptureStatusError}
	}

	return formCaptureResult{Status: formCaptureStatusCaptured, Data: captureData}
}

// queueFormRestore queues a JS execute command to restore form values and scroll position.
// This is fire-and-forget: the caller does not wait for the result.
func (h *ToolHandler) queueFormRestore(req JSONRPCRequest, formValues map[string]any, scrollPos map[string]any) string {
	correlationID := fmt.Sprintf("state_restore_%d_%d", time.Now().UnixNano(), randomInt63())

	script := buildFormRestoreScript(formValues, scrollPos)
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

func (h *ToolHandler) handlePilotManageStateSave(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		SnapshotName string `json:"snapshot_name"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}

	if params.SnapshotName == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'snapshot_name' is missing", "Add the 'snapshot_name' parameter", withParam("snapshot_name"))}
	}

	// Ensure session store is initialized
	if h.sessionStoreImpl == nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrNotInitialized, "Session store not initialized", "Internal error — do not retry")}
	}

	// Capture current state from the tracked tab
	_, tabID, tabURL := h.capture.GetTrackingStatus()
	tabTitle := h.capture.GetTrackedTabTitle()

	stateData := map[string]any{
		"url":      tabURL,
		"title":    tabTitle,
		"tab_id":   tabID,
		"saved_at": time.Now().Format(time.RFC3339),
	}

	// Form capture — always produces a status for the response
	capture := h.captureFormState(req)
	if capture.Status == formCaptureStatusCaptured && capture.Data != nil {
		if fv, ok := capture.Data["form_values"]; ok {
			stateData["form_values"] = fv
		}
		if sp, ok := capture.Data["scroll_position"]; ok {
			stateData["scroll_position"] = sp
		}
	}

	// Serialize and save
	data, err := json.Marshal(stateData)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInternal, "Failed to serialize state: "+err.Error(), "Internal error — do not retry")}
	}

	if err := h.sessionStoreImpl.Save(stateNamespace, params.SnapshotName, data); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInternal, "Failed to save state: "+err.Error(), "Internal error — check storage")}
	}

	// Record AI action
	h.recordAIAction("save_state", tabURL, map[string]any{"snapshot_name": params.SnapshotName})

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("State saved", map[string]any{
		"status":        "saved",
		"snapshot_name": params.SnapshotName,
		"form_capture":  capture.Status,
		"state": map[string]any{
			"url":   tabURL,
			"title": tabTitle,
		},
	})}
}

func (h *ToolHandler) handlePilotManageStateLoad(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		SnapshotName string `json:"snapshot_name"`
		IncludeURL   bool   `json:"include_url,omitempty"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}

	if params.SnapshotName == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'snapshot_name' is missing", "Add the 'snapshot_name' parameter", withParam("snapshot_name"))}
	}

	if h.sessionStoreImpl == nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrNotInitialized, "Session store not initialized", "Internal error — do not retry")}
	}

	data, err := h.sessionStoreImpl.Load(stateNamespace, params.SnapshotName)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrNoData, "State not found: "+params.SnapshotName, "Use interact with action='list_states' to see available snapshots", h.diagnosticHint())}
	}

	var stateData map[string]any
	if err := json.Unmarshal(data, &stateData); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInternal, "Failed to parse state data", "Internal error — state may be corrupted")}
	}

	if params.IncludeURL {
		h.queueStateNavigation(req, stateData)
	}

	// Determine form restore status
	responseData := map[string]any{
		"status":        "loaded",
		"snapshot_name": params.SnapshotName,
		"state":         stateData,
	}

	formValues, hasFormData := stateData["form_values"].(map[string]any)
	if !hasFormData {
		responseData["form_restore"] = formRestoreStatusNoFormData
	} else if !h.capture.IsPilotEnabled() {
		responseData["form_restore"] = formRestoreStatusPilotDisabled
	} else if !h.capture.IsExtensionConnected() {
		responseData["form_restore"] = formRestoreStatusExtensionDown
	} else {
		scrollPos, _ := stateData["scroll_position"].(map[string]any)
		restoreCorrelationID := h.queueFormRestore(req, formValues, scrollPos)
		responseData["form_restore"] = formRestoreStatusQueued
		responseData["restore_correlation_id"] = restoreCorrelationID
	}

	h.recordAIAction("load_state", "", map[string]any{"snapshot_name": params.SnapshotName})

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("State loaded", responseData)}
}

// queueStateNavigation queues a navigation to the saved URL if pilot is enabled
// and the state contains a non-empty URL. Mutates stateData to add tracking fields.
func (h *ToolHandler) queueStateNavigation(req JSONRPCRequest, stateData map[string]any) {
	savedURL, ok := stateData["url"].(string)
	if !ok || savedURL == "" || !h.capture.IsPilotEnabled() {
		return
	}
	correlationID := fmt.Sprintf("nav_%d_%d", time.Now().UnixNano(), randomInt63())
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

func (h *ToolHandler) handlePilotManageStateList(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	if h.sessionStoreImpl == nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrNotInitialized, "Session store not initialized", "Internal error — do not retry")}
	}

	keys, err := h.sessionStoreImpl.List(stateNamespace)
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
	data, err := h.sessionStoreImpl.Load(stateNamespace, key)
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

func (h *ToolHandler) handlePilotManageStateDelete(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		SnapshotName string `json:"snapshot_name"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}

	if params.SnapshotName == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'snapshot_name' is missing", "Add the 'snapshot_name' parameter", withParam("snapshot_name"))}
	}

	// Ensure session store is initialized
	if h.sessionStoreImpl == nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrNotInitialized, "Session store not initialized", "Internal error — do not retry")}
	}

	// Delete the state
	if err := h.sessionStoreImpl.Delete(stateNamespace, params.SnapshotName); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrNoData, "State not found: "+params.SnapshotName, "Use interact with action='list_states' to see available snapshots", h.diagnosticHint())}
	}

	// Record AI action
	h.recordAIAction("delete_state", "", map[string]any{"snapshot_name": params.SnapshotName})

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("State deleted", map[string]any{
		"status":        "deleted",
		"snapshot_name": params.SnapshotName,
	})}
}
