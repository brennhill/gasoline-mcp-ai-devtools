// tools_interact_handler_test.go — Comprehensive unit tests for interact tool dispatch and response fields.
// Validates all response fields, snake_case JSON convention, parameter validation, and error handling.
package main

import (
	"strings"
	"testing"
)

// ============================================
// Dispatch Tests
// ============================================

func TestToolsInteractDispatch_InvalidJSON(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callInteractRaw(h, `{bad json`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("invalid JSON should return isError:true")
	}
	if !strings.Contains(result.Content[0].Text, "invalid_json") {
		t.Errorf("error code should be 'invalid_json', got: %s", result.Content[0].Text)
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsInteractDispatch_MissingAction(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callInteractRaw(h, `{}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("missing 'action' should return isError:true")
	}
	if !strings.Contains(result.Content[0].Text, "missing_param") {
		t.Errorf("error code should be 'missing_param', got: %s", result.Content[0].Text)
	}
	// Verify hint lists valid actions
	text := result.Content[0].Text
	for _, action := range []string{"highlight", "navigate", "execute_js", "click"} {
		if !strings.Contains(text, action) {
			t.Errorf("hint should list valid action %q", action)
		}
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsInteractDispatch_UnknownAction(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callInteractRaw(h, `{"what":"nonexistent_action"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("unknown action should return isError:true")
	}
	if !strings.Contains(result.Content[0].Text, "unknown_mode") {
		t.Errorf("error code should be 'unknown_mode', got: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "nonexistent_action") {
		t.Error("error should mention the invalid action name")
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsInteractDispatch_ScreenshotAlias(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callInteractRaw(h, `{"what":"screenshot"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("screenshot alias without tracked tab should return isError:true")
	}
	text := result.Content[0].Text
	if strings.Contains(text, "unknown_mode") {
		t.Fatalf("screenshot alias should not return unknown_mode. Got: %s", text)
	}
	if !strings.Contains(text, "no_data") {
		t.Fatalf("screenshot alias should route to screenshot handler (no_data expected in unit test). Got: %s", text)
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsInteractDispatch_EmptyArgs(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := h.toolInteract(req, nil)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("nil args (no 'action') should return isError:true")
	}
}

// ============================================
// interact(action:"highlight") — Response Fields & Validation
// ============================================

func TestToolsInteractHighlight_MissingSelector(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callInteractRaw(h, `{"what":"highlight"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("highlight without selector should return isError:true")
	}
	if !strings.Contains(result.Content[0].Text, "missing_param") {
		t.Errorf("error code should be 'missing_param', got: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "selector") {
		t.Error("error should mention 'selector' parameter")
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsInteractHighlight_PilotDisabled(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callInteractRaw(h, `{"what":"highlight","selector":"#main"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("highlight with pilot disabled should return isError:true")
	}
	if !strings.Contains(result.Content[0].Text, "pilot_disabled") {
		t.Errorf("error code should be 'pilot_disabled', got: %s", result.Content[0].Text)
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsInteractHighlight_Success(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)
	cap.SetPilotEnabled(true)

	resp := callInteractRaw(h, `{"what":"highlight","selector":".btn"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("highlight should succeed with pilot enabled, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	// Verify response fields
	for _, field := range []string{"status", "correlation_id", "queued", "final"} {
		if _, ok := data[field]; !ok {
			t.Errorf("highlight response missing field %q", field)
		}
	}
	if data["status"] != "queued" {
		t.Errorf("status = %v, want 'queued'", data["status"])
	}
	corr, _ := data["correlation_id"].(string)
	if corr == "" {
		t.Error("correlation_id should be non-empty")
	}
	if !strings.HasPrefix(corr, "highlight_") {
		t.Errorf("correlation_id should start with 'highlight_', got: %s", corr)
	}

	// Verify pending query created
	pq := cap.GetLastPendingQuery()
	if pq == nil {
		t.Fatal("highlight should create a pending query")
	}
	if pq.Type != "highlight" {
		t.Errorf("pending query type = %q, want 'highlight'", pq.Type)
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

// ============================================
// interact(action:"execute_js") — Response Fields & Validation
// ============================================

func TestToolsInteractExecuteJS_MissingScript(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callInteractRaw(h, `{"what":"execute_js"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("execute_js without script should return isError:true")
	}
	if !strings.Contains(result.Content[0].Text, "script") {
		t.Error("error should mention missing 'script' parameter")
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsInteractExecuteJS_InvalidWorld(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callInteractRaw(h, `{"what":"execute_js","script":"1+1","world":"invalid_world"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("invalid world should return isError:true")
	}
	if !strings.Contains(result.Content[0].Text, "invalid_param") {
		t.Errorf("error code should be 'invalid_param', got: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "world") {
		t.Error("error should mention 'world' parameter")
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsInteractExecuteJS_ValidWorlds(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	// All valid worlds should pass world validation (fail at pilot check, not world check)
	for _, world := range []string{"auto", "main", "isolated"} {
		t.Run(world, func(t *testing.T) {
			resp := callInteractRaw(h, `{"what":"execute_js","script":"1+1","world":"`+world+`"}`)
			result := parseToolResult(t, resp)
			// Should fail at pilot check, NOT world validation
			if !result.IsError {
				t.Fatal("should return error (pilot disabled)")
			}
			if strings.Contains(result.Content[0].Text, "world") {
				t.Errorf("world=%q should pass validation, but got world error: %s", world, result.Content[0].Text)
			}
		})
	}
}

func TestToolsInteractExecuteJS_DefaultWorld(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	// Omitting world should default to "auto" and pass validation
	resp := callInteractRaw(h, `{"what":"execute_js","script":"1+1"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("should return error (pilot disabled)")
	}
	// Should NOT contain world error
	if strings.Contains(result.Content[0].Text, "world") {
		t.Errorf("default world should pass validation, got: %s", result.Content[0].Text)
	}
}

func TestToolsInteractExecuteJS_Success(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)
	cap.SetPilotEnabled(true)

	resp := callInteractRaw(h, `{"what":"execute_js","script":"document.title"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("execute_js should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	for _, field := range []string{"status", "correlation_id", "queued", "final"} {
		if _, ok := data[field]; !ok {
			t.Errorf("execute_js response missing field %q", field)
		}
	}
	if data["status"] != "queued" {
		t.Errorf("status = %v, want 'queued'", data["status"])
	}
	corr, _ := data["correlation_id"].(string)
	if !strings.HasPrefix(corr, "exec_") {
		t.Errorf("correlation_id should start with 'exec_', got: %s", corr)
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

// ============================================
// interact(action:"navigate") — Response Fields & Validation
// ============================================

func TestToolsInteractNavigate_MissingURL(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callInteractRaw(h, `{"what":"navigate"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("navigate without url should return isError:true")
	}
	if !strings.Contains(result.Content[0].Text, "url") {
		t.Error("error should mention 'url' parameter")
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsInteractNavigate_Success(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)
	cap.SetPilotEnabled(true)

	resp := callInteractRaw(h, `{"what":"navigate","url":"https://example.com"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("navigate should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	if data["status"] != "queued" {
		t.Errorf("status = %v, want 'queued'", data["status"])
	}
	corr, _ := data["correlation_id"].(string)
	if !strings.HasPrefix(corr, "nav_") {
		t.Errorf("correlation_id should start with 'nav_', got: %s", corr)
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

// ============================================
// interact(action:"refresh/back/forward") — Pilot Check
// ============================================

func TestToolsInteractBrowserActions_PilotRequired(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	actions := []struct {
		name string
		args string
	}{
		{"refresh", `{"what":"refresh"}`},
		{"back", `{"what":"back"}`},
		{"forward", `{"what":"forward"}`},
		{"new_tab", `{"what":"new_tab","url":"https://example.com"}`},
	}

	for _, tc := range actions {
		t.Run(tc.name, func(t *testing.T) {
			resp := callInteractRaw(h, tc.args)
			result := parseToolResult(t, resp)
			if !result.IsError {
				t.Fatalf("%s with pilot disabled should return isError:true", tc.name)
			}
			if !strings.Contains(result.Content[0].Text, "pilot_disabled") {
				t.Errorf("%s error code should be 'pilot_disabled', got: %s", tc.name, result.Content[0].Text)
			}
		})
	}
}

func TestToolsInteractBrowserActions_SuccessWithPilot(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)
	cap.SetPilotEnabled(true)

	actions := []struct {
		name   string
		args   string
		prefix string
	}{
		{"refresh", `{"what":"refresh"}`, "refresh_"},
		{"back", `{"what":"back"}`, "back_"},
		{"forward", `{"what":"forward"}`, "forward_"},
		{"new_tab", `{"what":"new_tab","url":"https://example.com"}`, "newtab_"},
	}

	for _, tc := range actions {
		t.Run(tc.name, func(t *testing.T) {
			resp := callInteractRaw(h, tc.args)
			result := parseToolResult(t, resp)
			if result.IsError {
				t.Fatalf("%s should succeed, got: %s", tc.name, result.Content[0].Text)
			}

			data := extractResultJSON(t, result)
			if data["status"] != "queued" {
				t.Errorf("status = %v, want 'queued'", data["status"])
			}
			corr, _ := data["correlation_id"].(string)
			if !strings.HasPrefix(corr, tc.prefix) {
				t.Errorf("correlation_id should start with %q, got: %s", tc.prefix, corr)
			}
			assertSnakeCaseFields(t, string(resp.Result))
		})
	}
}

// ============================================
// interact(action:"subtitle") — Response Fields
// ============================================

func TestToolsInteractSubtitle_MissingText(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callInteractRaw(h, `{"what":"subtitle"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("subtitle without text should return isError:true")
	}
	if !strings.Contains(result.Content[0].Text, "text") {
		t.Error("error should mention 'text' parameter")
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsInteractSubtitle_SetText(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callInteractRaw(h, `{"what":"subtitle","text":"Hello world"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("subtitle set should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	if data["status"] != "queued" {
		t.Errorf("status = %v, want 'queued'", data["status"])
	}
	if queued, ok := data["queued"].(bool); !ok || !queued {
		t.Errorf("queued = %v, want true", data["queued"])
	}
	if final, ok := data["final"].(bool); !ok || final {
		t.Errorf("final = %v, want false", data["final"])
	}
	corr, _ := data["correlation_id"].(string)
	if !strings.HasPrefix(corr, "subtitle_") {
		t.Errorf("correlation_id should start with 'subtitle_', got: %s", corr)
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsInteractSubtitle_ClearText(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callInteractRaw(h, `{"what":"subtitle","text":""}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("subtitle clear should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	if data["status"] != "queued" {
		t.Errorf("status = %v, want 'queued'", data["status"])
	}
	if queued, ok := data["queued"].(bool); !ok || !queued {
		t.Errorf("queued = %v, want true", data["queued"])
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

// ============================================
// interact DOM primitives — Parameter Validation
// ============================================

func TestToolsInteractDOMPrimitives_MissingSelector(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	actions := []string{"click", "type", "select", "check", "get_text", "get_value",
		"get_attribute", "set_attribute", "focus", "scroll_to", "wait_for", "key_press"}

	for _, action := range actions {
		t.Run(action, func(t *testing.T) {
			resp := callInteractRaw(h, `{"what":"`+action+`"}`)
			result := parseToolResult(t, resp)
			if !result.IsError {
				t.Fatalf("%s without selector should return isError:true", action)
			}
			if !strings.Contains(result.Content[0].Text, "selector") {
				t.Errorf("%s error should mention 'selector', got: %s", action, result.Content[0].Text)
			}
		})
	}
}

func TestToolsInteractDOMPrimitives_IntentActions_NoSelector(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	actions := []string{
		"open_composer",
		"submit_active_composer",
		"confirm_top_dialog",
		"dismiss_top_overlay",
	}

	for _, action := range actions {
		t.Run(action, func(t *testing.T) {
			resp := callInteractRaw(h, `{"what":"`+action+`"}`)
			result := parseToolResult(t, resp)
			if !result.IsError {
				t.Fatalf("%s without selector should still error while pilot is disabled", action)
			}
			if strings.Contains(strings.ToLower(result.Content[0].Text), "selector") {
				t.Errorf("%s should not fail with selector-missing guidance: %s", action, result.Content[0].Text)
			}
		})
	}
}

func TestToolsInteractDOMPrimitives_ActionSpecificParams(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	// Actions that require specific params beyond selector
	cases := []struct {
		action  string
		args    string
		missing string
	}{
		{"type", `{"what":"type","selector":"input"}`, "text"},
		{"select", `{"what":"select","selector":"select"}`, "value"},
		{"get_attribute", `{"what":"get_attribute","selector":"div"}`, "name"},
		{"set_attribute", `{"what":"set_attribute","selector":"div"}`, "name"},
	}

	for _, tc := range cases {
		t.Run(tc.action+"_missing_"+tc.missing, func(t *testing.T) {
			resp := callInteractRaw(h, tc.args)
			result := parseToolResult(t, resp)
			if !result.IsError {
				t.Fatalf("%s without %s should return isError:true", tc.action, tc.missing)
			}
			if !strings.Contains(result.Content[0].Text, tc.missing) {
				t.Errorf("error should mention missing %q param, got: %s", tc.missing, result.Content[0].Text)
			}
		})
	}
}

func TestToolsInteractDOMPrimitives_SuccessWithPilot(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)
	cap.SetPilotEnabled(true)

	cases := []struct {
		action string
		args   string
	}{
		{"click", `{"what":"click","selector":"#btn"}`},
		{"type", `{"what":"type","selector":"input","text":"hello"}`},
		{"select", `{"what":"select","selector":"select","value":"opt1"}`},
		{"check", `{"what":"check","selector":"input[type=checkbox]"}`},
		{"get_text", `{"what":"get_text","selector":"div"}`},
		{"get_value", `{"what":"get_value","selector":"input"}`},
		{"get_attribute", `{"what":"get_attribute","selector":"a","name":"href"}`},
		{"set_attribute", `{"what":"set_attribute","selector":"div","name":"data-test","value":"1"}`},
		{"focus", `{"what":"focus","selector":"input"}`},
		{"scroll_to", `{"what":"scroll_to","selector":"#footer"}`},
		{"wait_for", `{"what":"wait_for","selector":"#spinner"}`},
		{"key_press", `{"what":"key_press","selector":"input","text":"Enter"}`},
		{"open_composer", `{"what":"open_composer"}`},
		{"submit_active_composer", `{"what":"submit_active_composer"}`},
		{"confirm_top_dialog", `{"what":"confirm_top_dialog"}`},
		{"dismiss_top_overlay", `{"what":"dismiss_top_overlay"}`},
	}

	for _, tc := range cases {
		t.Run(tc.action, func(t *testing.T) {
			resp := callInteractRaw(h, tc.args)
			result := parseToolResult(t, resp)
			if result.IsError {
				t.Fatalf("%s should succeed with pilot enabled, got: %s", tc.action, result.Content[0].Text)
			}

			data := extractResultJSON(t, result)
			if data["status"] != "queued" {
				t.Errorf("status = %v, want 'queued'", data["status"])
			}
			corr, _ := data["correlation_id"].(string)
			if !strings.HasPrefix(corr, "dom_") {
				t.Errorf("correlation_id should start with 'dom_', got: %s", corr)
			}
			assertSnakeCaseFields(t, string(resp.Result))
		})
	}
}

// ============================================
// interact(action:"save_state") — Response Fields
// ============================================

func TestToolsInteractSaveState_MissingSnapshotName(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callInteractRaw(h, `{"what":"save_state"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("save_state without snapshot_name should return isError:true")
	}
	if !strings.Contains(result.Content[0].Text, "snapshot_name") {
		t.Error("error should mention 'snapshot_name' parameter")
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsInteractSaveState_Success(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callInteractRaw(h, `{"what":"save_state","snapshot_name":"test_save"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("save_state should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	for _, field := range []string{"status", "snapshot_name", "state"} {
		if _, ok := data[field]; !ok {
			t.Errorf("save_state response missing field %q", field)
		}
	}
	if data["status"] != "saved" {
		t.Errorf("status = %v, want 'saved'", data["status"])
	}
	if data["snapshot_name"] != "test_save" {
		t.Errorf("snapshot_name = %v, want 'test_save'", data["snapshot_name"])
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

// ============================================
// interact(action:"list_states") — Response Fields
// ============================================

func TestToolsInteractListStates_ResponseFields(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callInteractRaw(h, `{"what":"list_states"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("list_states should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	for _, field := range []string{"states", "count"} {
		if _, ok := data[field]; !ok {
			t.Errorf("list_states response missing field %q", field)
		}
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

// ============================================
// interact(action:"list_interactive") — Response Fields
// ============================================

func TestToolsInteractListInteractive_ResponseFields(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)
	cap.SetPilotEnabled(true)

	resp := callInteractRaw(h, `{"what":"list_interactive"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("list_interactive should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	if data["status"] != "queued" {
		t.Errorf("status = %v, want 'queued'", data["status"])
	}
	corr, _ := data["correlation_id"].(string)
	if !strings.HasPrefix(corr, "dom_list_") {
		t.Errorf("correlation_id should start with 'dom_list_', got: %s", corr)
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

// ============================================
// validateDOMActionParams Tests
// ============================================

func TestToolsValidateDOMActionParams(t *testing.T) {
	t.Parallel()

	req := JSONRPCRequest{JSONRPC: "2.0", ID: 1}

	// Actions without special required params should pass
	for _, action := range []string{"click", "check", "focus", "scroll_to", "wait_for", "key_press"} {
		_, failed := validateDOMActionParams(req, action, "", "", "")
		if failed {
			t.Errorf("validateDOMActionParams(%q) should not fail for actions without required params", action)
		}
	}

	// "type" requires "text"
	_, failed := validateDOMActionParams(req, "type", "", "", "")
	if !failed {
		t.Error("type without text should fail validation")
	}
	_, failed = validateDOMActionParams(req, "type", "hello", "", "")
	if failed {
		t.Error("type with text should pass validation")
	}

	// "select" requires "value"
	_, failed = validateDOMActionParams(req, "select", "", "", "")
	if !failed {
		t.Error("select without value should fail validation")
	}
	_, failed = validateDOMActionParams(req, "select", "", "opt1", "")
	if failed {
		t.Error("select with value should pass validation")
	}

	// "get_attribute" requires "name"
	_, failed = validateDOMActionParams(req, "get_attribute", "", "", "")
	if !failed {
		t.Error("get_attribute without name should fail validation")
	}
	_, failed = validateDOMActionParams(req, "get_attribute", "", "", "href")
	if failed {
		t.Error("get_attribute with name should pass validation")
	}
}

// truncateToLen pure function tests live in internal/tools/interact/selector_test.go.
