// tools_interact_state.go — MCP interact state management handlers.
// Implements save_state, load_state, list_states, delete_state with form, storage, and cookie capture.
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

// State capture status values — always present in save_state response as "state_capture".
const (
	stateCaptureStatusCaptured              = "captured"
	stateCaptureStatusPilotDisabled         = "skipped_pilot_disabled"
	stateCaptureStatusExtensionDisconnected = "skipped_extension_disconnected"
	stateCaptureStatusTimeout               = "skipped_timeout"
	stateCaptureStatusError                 = "skipped_error"
)

// State restore status values — always present in load_state response as "state_restore".
const (
	stateRestoreStatusQueued        = "queued"
	stateRestoreStatusPilotDisabled = "skipped_pilot_disabled"
	stateRestoreStatusExtensionDown = "skipped_extension_disconnected"
	stateRestoreStatusNoData        = "skipped_no_state_data"
)

// stateCaptureScript is the JS executed in the browser to capture form values,
// scroll position, localStorage, sessionStorage, and cookies.
const stateCaptureScript = `(() => {
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

    const type = (el.type || '').toLowerCase();
    const rawValue = (type === 'checkbox' || type === 'radio') ? !!el.checked : String(el.value ?? '');
    if (isSensitive(el, key, rawValue)) return;

    if (type === 'radio') {
      const groupName = el.name || key;
      if (el.checked) {
        forms['radio::' + groupName] = {
          kind: 'radio',
          name: groupName,
          value: String(el.value ?? '')
        };
      }
      return;
    }

    if (type === 'checkbox') {
      forms[key] = el.checked;
    } else {
      forms[key] = el.value;
    }
  });

  const ls = {};
  try {
    for (let i = 0; i < localStorage.length; i++) {
      const k = localStorage.key(i);
      if (!sensitiveKeyPattern.test(k)) {
        const v = localStorage.getItem(k);
        if (v !== null && !(v.length >= 12 && sensitiveValuePattern.test(v.trim()))) {
          ls[k] = v;
        }
      }
    }
  } catch(e) {}

  const ss = {};
  try {
    for (let i = 0; i < sessionStorage.length; i++) {
      const k = sessionStorage.key(i);
      if (!sensitiveKeyPattern.test(k)) {
        const v = sessionStorage.getItem(k);
        if (v !== null && !(v.length >= 12 && sensitiveValuePattern.test(v.trim()))) {
          ss[k] = v;
        }
      }
    }
  } catch(e) {}

  const cookies = {};
  try {
    document.cookie.split(';').forEach(c => {
      const [k, ...rest] = c.trim().split('=');
      if (k && !sensitiveKeyPattern.test(k)) {
        const v = rest.join('=');
        if (!(v.length >= 12 && sensitiveValuePattern.test(v.trim()))) {
          cookies[k] = v;
        }
      }
    });
  } catch(e) {}

  return {
    form_values: forms,
    scroll_position: { x: window.scrollX, y: window.scrollY },
    local_storage: ls,
    session_storage: ss,
    cookies: cookies
  };
})()`

// stateCaptureResult holds the outcome of a state capture attempt.
type stateCaptureResult struct {
	Status string         // one of stateCaptureStatus* constants
	Data   map[string]any // non-nil only when Status == "captured"
}

func parseCapturedStatePayload(raw json.RawMessage) (map[string]any, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return nil, errors.New("empty state capture payload")
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

	// Direct result (no success envelope) — accept if any known field present
	for _, key := range []string{"form_values", "local_storage", "session_storage", "cookies"} {
		if _, ok := envelope[key]; ok {
			return envelope, nil
		}
	}
	return nil, errors.New("state capture payload missing expected fields")
}

// stateDataFields lists the fields extracted from capture data into persisted state.
var stateDataFields = []string{"form_values", "scroll_position", "local_storage", "session_storage", "cookies"}

// buildStateRestoreScript builds a JS script that restores form values, scroll position,
// localStorage, sessionStorage, and cookies. All data is embedded as JSON literals.
func buildStateRestoreScript(formValues, scrollPos, localStorage, sessionStorage, cookies map[string]any) string {
	if formValues == nil {
		formValues = map[string]any{}
	}
	if scrollPos == nil {
		scrollPos = map[string]any{}
	}
	if localStorage == nil {
		localStorage = map[string]any{}
	}
	if sessionStorage == nil {
		sessionStorage = map[string]any{}
	}
	if cookies == nil {
		cookies = map[string]any{}
	}
	formJSON, _ := json.Marshal(formValues)
	scrollJSON, _ := json.Marshal(scrollPos)
	lsJSON, _ := json.Marshal(localStorage)
	ssJSON, _ := json.Marshal(sessionStorage)
	cookiesJSON, _ := json.Marshal(cookies)

	return fmt.Sprintf(`(() => {
  const formValues = %s;
  const scrollPos = %s;
  const lsData = %s;
  const ssData = %s;
  const cookieData = %s;
  const escapeCSS = (value) => {
    if (typeof CSS !== 'undefined' && CSS && typeof CSS.escape === 'function') {
      return CSS.escape(String(value));
    }
    return String(value).replace(/\\/g, '\\\\').replace(/"/g, '\\"');
  };

  Object.entries(formValues).forEach(([key, val]) => {
    if (val && typeof val === 'object' && val.kind === 'radio' && val.name) {
      const escapedName = escapeCSS(val.name);
      const escapedValue = escapeCSS(val.value ?? '');
      const radio = document.querySelector('input[type="radio"][name="' + escapedName + '"][value="' + escapedValue + '"]');
      if (radio) {
        radio.checked = true;
      }
      return;
    }

    const escapedKey = escapeCSS(key);
    const el = document.getElementById(key) || document.querySelector('[name="' + escapedKey + '"]');
    if (!el) return;
    if (el.type === 'checkbox' || el.type === 'radio') {
      el.checked = !!val;
    } else {
      el.value = String(val);
      el.dispatchEvent(new Event('input', {bubbles: true}));
    }
  });

  try { Object.entries(lsData).forEach(([k, v]) => { localStorage.setItem(k, v); }); } catch(e) {}
  try { Object.entries(ssData).forEach(([k, v]) => { sessionStorage.setItem(k, v); }); } catch(e) {}
  try { Object.entries(cookieData).forEach(([k, v]) => { document.cookie = k + '=' + v; }); } catch(e) {}

  if (scrollPos && scrollPos.x !== undefined) {
    window.scrollTo(scrollPos.x, scrollPos.y);
  }
  return {
    restored_forms: Object.keys(formValues).length,
    restored_local_storage: Object.keys(lsData).length,
    restored_session_storage: Object.keys(ssData).length,
    restored_cookies: Object.keys(cookieData).length
  };
})()`, string(formJSON), string(scrollJSON), string(lsJSON), string(ssJSON), string(cookiesJSON))
}

// captureState attempts to capture form values, scroll position, and web storage from the browser.
// Always returns a stateCaptureResult with an explicit Status the caller can surface to the LLM.
func (h *ToolHandler) captureState(req JSONRPCRequest) stateCaptureResult {
	if !h.capture.IsPilotEnabled() {
		return stateCaptureResult{Status: stateCaptureStatusPilotDisabled}
	}
	if !h.capture.IsExtensionConnected() {
		return stateCaptureResult{Status: stateCaptureStatusExtensionDisconnected}
	}

	correlationID := fmt.Sprintf("state_capture_%d_%d", time.Now().UnixNano(), randomInt63())

	scriptArgs, _ := json.Marshal(map[string]any{
		"action": "execute_js",
		"script": stateCaptureScript,
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
		return stateCaptureResult{Status: stateCaptureStatusTimeout}
	}
	if cmd.Error != "" {
		return stateCaptureResult{Status: stateCaptureStatusError}
	}
	if cmd.Status != "complete" || len(cmd.Result) == 0 {
		return stateCaptureResult{Status: stateCaptureStatusError}
	}

	captureData, err := parseCapturedStatePayload(cmd.Result)
	if err != nil {
		return stateCaptureResult{Status: stateCaptureStatusError}
	}

	return stateCaptureResult{Status: stateCaptureStatusCaptured, Data: captureData}
}

// queueStateRestore queues a JS execute command to restore form values, scroll position,
// localStorage, sessionStorage, and cookies. This is fire-and-forget.
func (h *ToolHandler) queueStateRestore(req JSONRPCRequest, formValues, scrollPos, localStorage, sessionStorage, cookies map[string]any) string {
	correlationID := fmt.Sprintf("state_restore_%d_%d", time.Now().UnixNano(), randomInt63())

	script := buildStateRestoreScript(formValues, scrollPos, localStorage, sessionStorage, cookies)
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
	if capture.Status == stateCaptureStatusCaptured && capture.Data != nil {
		for _, field := range stateDataFields {
			if v, ok := capture.Data[field]; ok {
				stateData[field] = v
			}
		}
	}

	data, err := json.Marshal(stateData)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInternal, "Failed to serialize state: "+err.Error(), "Internal error — do not retry")}
	}

	if err := h.sessionStoreImpl.Save(stateNamespace, params.SnapshotName, data); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInternal, "Failed to save state: "+err.Error(), "Internal error — check storage")}
	}

	h.recordAIAction("save_state", tabURL, map[string]any{"snapshot_name": params.SnapshotName})

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("State saved", map[string]any{
		"status":        "saved",
		"snapshot_name": params.SnapshotName,
		"state_capture": capture.Status,
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

	responseData := map[string]any{
		"status":        "loaded",
		"snapshot_name": params.SnapshotName,
		"state":         stateData,
	}

	formValues, _ := stateData["form_values"].(map[string]any)
	scrollPos, _ := stateData["scroll_position"].(map[string]any)
	localStorage, _ := stateData["local_storage"].(map[string]any)
	sessionStorage, _ := stateData["session_storage"].(map[string]any)
	cookies, _ := stateData["cookies"].(map[string]any)

	hasData := len(formValues) > 0 || len(localStorage) > 0 || len(sessionStorage) > 0 || len(cookies) > 0

	if !hasData {
		responseData["state_restore"] = stateRestoreStatusNoData
	} else if !h.capture.IsPilotEnabled() {
		responseData["state_restore"] = stateRestoreStatusPilotDisabled
	} else if !h.capture.IsExtensionConnected() {
		responseData["state_restore"] = stateRestoreStatusExtensionDown
	} else {
		restoreCorrelationID := h.queueStateRestore(req, formValues, scrollPos, localStorage, sessionStorage, cookies)
		responseData["state_restore"] = stateRestoreStatusQueued
		responseData["restore_correlation_id"] = restoreCorrelationID
	}

	h.recordAIAction("load_state", "", map[string]any{"snapshot_name": params.SnapshotName})

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("State loaded", responseData)}
}

// queueStateNavigation queues a navigation to the saved URL if pilot is enabled
// and the state contains a non-empty URL. Mutates stateData to add tracking fields.
func (h *ToolHandler) queueStateNavigation(req JSONRPCRequest, stateData map[string]any) {
	savedURL, ok := stateData["url"].(string)
	if !ok || savedURL == "" || !h.capture.IsPilotEnabled() || !h.capture.IsExtensionConnected() {
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

	if h.sessionStoreImpl == nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrNotInitialized, "Session store not initialized", "Internal error — do not retry")}
	}

	if err := h.sessionStoreImpl.Delete(stateNamespace, params.SnapshotName); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrNoData, "State not found: "+params.SnapshotName, "Use interact with action='list_states' to see available snapshots", h.diagnosticHint())}
	}

	h.recordAIAction("delete_state", "", map[string]any{"snapshot_name": params.SnapshotName})

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("State deleted", map[string]any{
		"status":        "deleted",
		"snapshot_name": params.SnapshotName,
	})}
}
