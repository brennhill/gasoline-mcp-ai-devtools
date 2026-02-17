// tools_interact_state_test.go — Tests for state management handlers with form capture.
// Verifies that form_capture and form_restore status fields are always present and unambiguous.
package main

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// extractResponseData parses the JSON payload from an MCPToolResult's text content.
func extractResponseData(t *testing.T, resp JSONRPCResponse) map[string]any {
	t.Helper()
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal MCPToolResult: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error response: %s", result.Content[0].Text)
	}
	text := result.Content[0].Text
	idx := strings.Index(text, "{")
	if idx < 0 {
		t.Fatalf("no JSON in response text: %s", text)
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(text[idx:]), &data); err != nil {
		t.Fatalf("parse response JSON: %v\nraw: %s", err, text[idx:])
	}
	return data
}

// simulateExtensionConnection sends a /sync request to mark the extension as connected.
func simulateExtensionConnection(t *testing.T, env *interactHelpersTestEnv) {
	t.Helper()
	httpReq := httptest.NewRequest("POST", "/sync", strings.NewReader(`{"session_id":"test"}`))
	httpReq.Header.Set("X-Gasoline-Client", "test-client")
	env.capture.HandleSync(httptest.NewRecorder(), httpReq)
}

func requireSessionStore(t *testing.T, env *interactHelpersTestEnv) {
	t.Helper()
	if env.handler.sessionStoreImpl == nil {
		t.Skip("session store unavailable in this test environment")
	}
}

// ============================================
// captureFormState — explicit status per scenario
// ============================================

func TestCaptureFormState_Status_PilotDisabled(t *testing.T) {
	t.Parallel()
	env := newInteractHelpersTestEnv(t)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	result := env.handler.captureFormState(req)
	if result.Status != formCaptureStatusPilotDisabled {
		t.Errorf("status = %q, want %q", result.Status, formCaptureStatusPilotDisabled)
	}
	if result.Data != nil {
		t.Error("Data should be nil when pilot is disabled")
	}
}

func TestCaptureFormState_Status_ExtensionDisconnected(t *testing.T) {
	t.Parallel()
	env := newInteractHelpersTestEnv(t)
	env.enablePilot(t)
	// No HandleSync → extension disconnected

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	result := env.handler.captureFormState(req)
	if result.Status != formCaptureStatusExtensionDisconnected {
		t.Errorf("status = %q, want %q", result.Status, formCaptureStatusExtensionDisconnected)
	}
	if result.Data != nil {
		t.Error("Data should be nil when extension is disconnected")
	}
}

// ============================================
// save_state — form_capture field always present
// ============================================

func TestSaveState_FormCapture_Captured(t *testing.T) {
	t.Parallel()
	env := newInteractHelpersTestEnv(t)
	env.enablePilot(t)
	simulateExtensionConnection(t, env)
	requireSessionStore(t, env)

	// Complete form capture in background
	go func() {
		time.Sleep(50 * time.Millisecond)
		queries := env.capture.GetPendingQueries()
		for _, q := range queries {
			if q.Type == "execute" && strings.HasPrefix(q.CorrelationID, "state_capture_") {
				result, _ := json.Marshal(map[string]any{
					"success": true,
					"result": map[string]any{
						"form_values":     map[string]any{"username": "john", "remember": true},
						"scroll_position": map[string]any{"x": 0.0, "y": 150.0},
					},
				})
				env.capture.CompleteCommand(q.CorrelationID, result, "")
				return
			}
		}
	}()

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), ClientID: "test-client"}
	resp := env.handler.handlePilotManageStateSave(req, json.RawMessage(`{"snapshot_name":"form_test"}`))
	data := extractResponseData(t, resp)

	if data["form_capture"] != "captured" {
		t.Errorf("form_capture = %v, want \"captured\"", data["form_capture"])
	}
	if data["status"] != "saved" {
		t.Errorf("status = %v, want \"saved\"", data["status"])
	}

	// Verify form_values actually persisted by loading the state
	resp2 := env.handler.handlePilotManageStateLoad(req, json.RawMessage(`{"snapshot_name":"form_test"}`))
	loadData := extractResponseData(t, resp2)
	state, _ := loadData["state"].(map[string]any)
	if _, ok := state["form_values"]; !ok {
		t.Error("form_values should be present in persisted state")
	}
	if _, ok := state["scroll_position"]; !ok {
		t.Error("scroll_position should be present in persisted state")
	}
}

func TestSaveState_FormCapture_SkippedPilotDisabled(t *testing.T) {
	t.Parallel()
	env := newInteractHelpersTestEnv(t)
	// Pilot NOT enabled
	requireSessionStore(t, env)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp := env.handler.handlePilotManageStateSave(req, json.RawMessage(`{"snapshot_name":"no_pilot"}`))
	data := extractResponseData(t, resp)

	if data["form_capture"] != "skipped_pilot_disabled" {
		t.Errorf("form_capture = %v, want \"skipped_pilot_disabled\"", data["form_capture"])
	}
	if data["status"] != "saved" {
		t.Errorf("status = %v, want \"saved\"", data["status"])
	}
}

func TestSaveState_FormCapture_SkippedExtensionDisconnected(t *testing.T) {
	t.Parallel()
	env := newInteractHelpersTestEnv(t)
	env.enablePilot(t)
	// Extension NOT connected
	requireSessionStore(t, env)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp := env.handler.handlePilotManageStateSave(req, json.RawMessage(`{"snapshot_name":"no_ext"}`))
	data := extractResponseData(t, resp)

	if data["form_capture"] != "skipped_extension_disconnected" {
		t.Errorf("form_capture = %v, want \"skipped_extension_disconnected\"", data["form_capture"])
	}
	if data["status"] != "saved" {
		t.Errorf("status = %v, want \"saved\"", data["status"])
	}
}

func TestSaveState_FormCapture_SkippedErrorOnExecuteFailure(t *testing.T) {
	t.Parallel()
	env := newInteractHelpersTestEnv(t)
	env.enablePilot(t)
	simulateExtensionConnection(t, env)
	requireSessionStore(t, env)

	go func() {
		time.Sleep(50 * time.Millisecond)
		queries := env.capture.GetPendingQueries()
		for _, q := range queries {
			if q.Type == "execute" && strings.HasPrefix(q.CorrelationID, "state_capture_") {
				result, _ := json.Marshal(map[string]any{
					"success": false,
					"error":   "execution_error",
					"message": "script failed",
				})
				env.capture.CompleteCommand(q.CorrelationID, result, "")
				return
			}
		}
	}()

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), ClientID: "test-client"}
	resp := env.handler.handlePilotManageStateSave(req, json.RawMessage(`{"snapshot_name":"capture_failure"}`))
	data := extractResponseData(t, resp)

	if data["form_capture"] != "skipped_error" {
		t.Errorf("form_capture = %v, want \"skipped_error\"", data["form_capture"])
	}
}

// ============================================
// load_state — form_restore field always present
// ============================================

func TestLoadState_FormRestore_Queued(t *testing.T) {
	t.Parallel()
	env := newInteractHelpersTestEnv(t)
	env.enablePilot(t)
	simulateExtensionConnection(t, env)
	requireSessionStore(t, env)

	// Inject a state WITH form_values
	stateData := map[string]any{
		"url": "https://example.com/form", "title": "Form Page",
		"saved_at":        time.Now().Format(time.RFC3339),
		"form_values":     map[string]any{"email": "test@test.com"},
		"scroll_position": map[string]any{"x": 0.0, "y": 100.0},
	}
	raw, _ := json.Marshal(stateData)
	if err := env.handler.sessionStoreImpl.Save("saved_states", "with_forms", raw); err != nil {
		t.Fatalf("seed state save failed: %v", err)
	}

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp := env.handler.handlePilotManageStateLoad(req, json.RawMessage(`{"snapshot_name":"with_forms"}`))
	data := extractResponseData(t, resp)

	if data["form_restore"] != "queued" {
		t.Errorf("form_restore = %v, want \"queued\"", data["form_restore"])
	}
	corrID, ok := data["restore_correlation_id"].(string)
	if !ok || corrID == "" {
		t.Error("restore_correlation_id should be present when form_restore=queued")
	}
	if !strings.HasPrefix(corrID, "state_restore_") {
		t.Errorf("restore_correlation_id = %q, want prefix \"state_restore_\"", corrID)
	}

	// Verify the queued execute command contains the form values
	pqs := env.capture.GetPendingQueries()
	found := false
	for _, q := range pqs {
		if q.Type == "execute" && q.CorrelationID == corrID {
			found = true
			var params map[string]any
			json.Unmarshal(q.Params, &params)
			script, _ := params["script"].(string)
			if !strings.Contains(script, "test@test.com") {
				t.Error("restore script should contain the form values")
			}
			break
		}
	}
	if !found {
		t.Error("expected execute query with restore_correlation_id")
	}
}

func TestLoadState_FormRestore_SkippedNoFormData(t *testing.T) {
	t.Parallel()
	env := newInteractHelpersTestEnv(t)
	env.enablePilot(t)
	requireSessionStore(t, env)

	// Inject a state WITHOUT form_values
	stateData := map[string]any{
		"url": "https://example.com/plain", "title": "Plain Page",
		"saved_at": time.Now().Format(time.RFC3339),
	}
	raw, _ := json.Marshal(stateData)
	if err := env.handler.sessionStoreImpl.Save("saved_states", "no_forms", raw); err != nil {
		t.Fatalf("seed state save failed: %v", err)
	}

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp := env.handler.handlePilotManageStateLoad(req, json.RawMessage(`{"snapshot_name":"no_forms"}`))
	data := extractResponseData(t, resp)

	if data["form_restore"] != "skipped_no_form_data" {
		t.Errorf("form_restore = %v, want \"skipped_no_form_data\"", data["form_restore"])
	}
	if _, ok := data["restore_correlation_id"]; ok {
		t.Error("restore_correlation_id should be absent when form_restore != queued")
	}
}

func TestLoadState_FormRestore_SkippedPilotDisabled(t *testing.T) {
	t.Parallel()
	env := newInteractHelpersTestEnv(t)
	// Pilot NOT enabled
	requireSessionStore(t, env)

	// Inject a state WITH form_values
	stateData := map[string]any{
		"url": "https://example.com/form", "title": "Form Page",
		"saved_at":    time.Now().Format(time.RFC3339),
		"form_values": map[string]any{"email": "test@test.com"},
	}
	raw, _ := json.Marshal(stateData)
	if err := env.handler.sessionStoreImpl.Save("saved_states", "has_forms", raw); err != nil {
		t.Fatalf("seed state save failed: %v", err)
	}

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp := env.handler.handlePilotManageStateLoad(req, json.RawMessage(`{"snapshot_name":"has_forms"}`))
	data := extractResponseData(t, resp)

	if data["form_restore"] != "skipped_pilot_disabled" {
		t.Errorf("form_restore = %v, want \"skipped_pilot_disabled\"", data["form_restore"])
	}
	if _, ok := data["restore_correlation_id"]; ok {
		t.Error("restore_correlation_id should be absent when pilot is disabled")
	}
}

func TestLoadState_FormRestore_SkippedExtensionDisconnected(t *testing.T) {
	t.Parallel()
	env := newInteractHelpersTestEnv(t)
	env.enablePilot(t)
	requireSessionStore(t, env)
	// No sync request: extension remains disconnected.

	stateData := map[string]any{
		"url":             "https://example.com/form",
		"title":           "Form Page",
		"saved_at":        time.Now().Format(time.RFC3339),
		"form_values":     map[string]any{"email": "test@test.com"},
		"scroll_position": map[string]any{"x": 0.0, "y": 20.0},
	}
	raw, _ := json.Marshal(stateData)
	if err := env.handler.sessionStoreImpl.Save("saved_states", "has_forms_disconnected", raw); err != nil {
		t.Fatalf("seed state save failed: %v", err)
	}

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp := env.handler.handlePilotManageStateLoad(req, json.RawMessage(`{"snapshot_name":"has_forms_disconnected"}`))
	data := extractResponseData(t, resp)

	if data["form_restore"] != "skipped_extension_disconnected" {
		t.Errorf("form_restore = %v, want \"skipped_extension_disconnected\"", data["form_restore"])
	}
	if _, ok := data["restore_correlation_id"]; ok {
		t.Error("restore_correlation_id should be absent when extension is disconnected")
	}
}

func TestLoadState_IncludeURL_SkipsNavigationWhenExtensionDisconnected(t *testing.T) {
	t.Parallel()
	env := newInteractHelpersTestEnv(t)
	env.enablePilot(t)
	requireSessionStore(t, env)
	// No sync request: extension remains disconnected.

	stateData := map[string]any{
		"url":      "https://example.com/restore-target",
		"title":    "Restore Target",
		"saved_at": time.Now().Format(time.RFC3339),
	}
	raw, _ := json.Marshal(stateData)
	if err := env.handler.sessionStoreImpl.Save("saved_states", "nav_disconnected", raw); err != nil {
		t.Fatalf("seed state save failed: %v", err)
	}

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp := env.handler.handlePilotManageStateLoad(req, json.RawMessage(`{"snapshot_name":"nav_disconnected","include_url":true}`))
	data := extractResponseData(t, resp)

	state, _ := data["state"].(map[string]any)
	if _, ok := state["navigation_queued"]; ok {
		t.Error("navigation_queued should be absent when extension is disconnected")
	}
	if _, ok := state["correlation_id"]; ok {
		t.Error("correlation_id should be absent when extension is disconnected")
	}
}

func TestLoadState_IncludeURL_QueuesNavigationWhenConnected(t *testing.T) {
	t.Parallel()
	env := newInteractHelpersTestEnv(t)
	env.enablePilot(t)
	simulateExtensionConnection(t, env)
	requireSessionStore(t, env)

	stateData := map[string]any{
		"url":      "https://example.com/restore-target",
		"title":    "Restore Target",
		"saved_at": time.Now().Format(time.RFC3339),
	}
	raw, _ := json.Marshal(stateData)
	if err := env.handler.sessionStoreImpl.Save("saved_states", "nav_connected", raw); err != nil {
		t.Fatalf("seed state save failed: %v", err)
	}

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp := env.handler.handlePilotManageStateLoad(req, json.RawMessage(`{"snapshot_name":"nav_connected","include_url":true}`))
	data := extractResponseData(t, resp)

	state, _ := data["state"].(map[string]any)
	if state["navigation_queued"] != true {
		t.Error("navigation_queued should be true when include_url is requested and extension is connected")
	}
	corrID, ok := state["correlation_id"].(string)
	if !ok || !strings.HasPrefix(corrID, "nav_") {
		t.Errorf("correlation_id = %v, want nav_* string", state["correlation_id"])
	}
}

// ============================================
// buildFormRestoreScript
// ============================================

func TestBuildFormRestoreScript_ContainsValues(t *testing.T) {
	t.Parallel()
	formValues := map[string]any{"email": "a@b.com", "name": "Test"}
	scrollPos := map[string]any{"x": 10.0, "y": 200.0}

	script := buildFormRestoreScript(formValues, scrollPos)

	if !strings.Contains(script, "a@b.com") {
		t.Error("script should contain form value 'a@b.com'")
	}
	if !strings.Contains(script, "Test") {
		t.Error("script should contain form value 'Test'")
	}
	if !strings.Contains(script, "200") {
		t.Error("script should contain scroll position")
	}
}

func TestBuildFormRestoreScript_EmptyValues(t *testing.T) {
	t.Parallel()
	script := buildFormRestoreScript(map[string]any{}, nil)
	if script == "" {
		t.Error("should return a valid script even with empty values")
	}
	if !strings.Contains(script, "{}") {
		t.Error("script should contain empty object for form values")
	}
}

func TestBuildFormRestoreScript_HandlesRadioEntriesAndEscaping(t *testing.T) {
	t.Parallel()
	formValues := map[string]any{
		"radio::plan": map[string]any{
			"kind":  "radio",
			"name":  `billing.plan`,
			"value": `pro"monthly`,
		},
	}

	script := buildFormRestoreScript(formValues, nil)

	if !strings.Contains(script, "val.kind === 'radio'") {
		t.Error("script should include the radio restore branch")
	}
	if !strings.Contains(script, "CSS.escape") {
		t.Error("script should use CSS.escape when available")
	}
	if !strings.Contains(script, "input[type=\"radio\"][name=\"") {
		t.Error("script should target radio inputs by name and value")
	}
	if !strings.Contains(script, "\"kind\":\"radio\"") {
		t.Error("script should embed serialized radio metadata")
	}
}

func TestFormCaptureScript_CapturesRadioAsStructuredValue(t *testing.T) {
	t.Parallel()

	if !strings.Contains(formCaptureScript, "forms['radio::' + groupName]") {
		t.Error("form capture script should store radio values under radio::<group>")
	}
	if !strings.Contains(formCaptureScript, "kind: 'radio'") {
		t.Error("form capture script should persist radio metadata kind")
	}
}
