// tools_configure_handler_test.go — Comprehensive unit tests for configure tool dispatch and response fields.
// Validates all response fields, snake_case JSON convention, and state changes.
package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// ============================================
// Dispatch Tests
// ============================================

func TestToolsConfigureDispatch_InvalidJSON(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{bad json`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("invalid JSON should return isError:true")
	}
	if !strings.Contains(result.Content[0].Text, "invalid_json") {
		t.Errorf("error code should be 'invalid_json', got: %s", result.Content[0].Text)
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsConfigureDispatch_MissingAction(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("missing 'action' should return isError:true")
	}
	if !strings.Contains(result.Content[0].Text, "missing_param") {
		t.Errorf("error code should be 'missing_param', got: %s", result.Content[0].Text)
	}
	// Verify hint lists valid actions
	text := result.Content[0].Text
	for _, action := range []string{"clear", "health", "noise_rule", "store"} {
		if !strings.Contains(text, action) {
			t.Errorf("hint should list valid action %q", action)
		}
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsConfigureDispatch_UnknownAction(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{"action":"nonexistent_action"}`)
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

func TestToolsConfigureDispatch_EmptyArgs(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := h.toolConfigure(req, nil)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("nil args (no 'action') should return isError:true")
	}
}

// ============================================
// getValidConfigureActions Tests
// ============================================

func TestToolsConfigure_GetValidConfigureActions(t *testing.T) {
	t.Parallel()

	actions := getValidConfigureActions()
	actionList := strings.Split(actions, ", ")
	for i := 1; i < len(actionList); i++ {
		if actionList[i-1] > actionList[i] {
			t.Errorf("actions not sorted: %q > %q", actionList[i-1], actionList[i])
		}
	}

	for _, required := range []string{"clear", "health", "noise_rule", "store", "load", "streaming"} {
		if !strings.Contains(actions, required) {
			t.Errorf("valid actions missing %q: %s", required, actions)
		}
	}
}

// ============================================
// configure(action:"health") — Response Fields
// ============================================

func TestToolsConfigureHealth_ResponseFields(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{"action":"health"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("health should succeed, got: %s", result.Content[0].Text)
	}

	// Health response should have content
	if len(result.Content) == 0 {
		t.Fatal("health should return content block")
	}
	if result.Content[0].Type != "text" {
		t.Errorf("content type = %q, want 'text'", result.Content[0].Type)
	}

	// Health response text should contain status info
	text := result.Content[0].Text
	if !strings.Contains(strings.ToLower(text), "status") &&
		!strings.Contains(strings.ToLower(text), "health") {
		t.Errorf("health response should mention status/health, got: %s", text)
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsConfigureTelemetry_DefaultStatus(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{"action":"telemetry"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("telemetry status should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	if data["status"] != "ok" {
		t.Errorf("status = %v, want 'ok'", data["status"])
	}
	if mode, _ := data["telemetry_mode"].(string); mode != telemetryModeAuto {
		t.Errorf("telemetry_mode = %q, want %q", mode, telemetryModeAuto)
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsConfigureTelemetry_SetMode(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{"action":"telemetry","telemetry_mode":"full"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("telemetry set mode should succeed, got: %s", result.Content[0].Text)
	}
	data := extractResultJSON(t, result)
	if mode, _ := data["telemetry_mode"].(string); mode != telemetryModeFull {
		t.Errorf("telemetry_mode = %q, want %q", mode, telemetryModeFull)
	}
}

func TestToolsConfigureTelemetry_InvalidMode(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{"action":"telemetry","telemetry_mode":"verbose"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("invalid telemetry mode should return isError:true")
	}
	if !strings.Contains(result.Content[0].Text, "invalid_param") {
		t.Errorf("error code should be 'invalid_param', got: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "telemetry_mode") {
		t.Errorf("error should mention telemetry_mode, got: %s", result.Content[0].Text)
	}
}

// ============================================
// configure(action:"clear") — Response Fields & State Changes
// ============================================

func TestToolsConfigureClear_AllBuffers_ResponseFields(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{"action":"clear","buffer":"all"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("clear all should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	if data["status"] != "ok" {
		t.Errorf("status = %v, want 'ok'", data["status"])
	}
	if data["buffer"] != "all" {
		t.Errorf("buffer = %v, want 'all'", data["buffer"])
	}
	if _, ok := data["cleared"]; !ok {
		t.Error("response missing 'cleared' field")
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsConfigureClear_DefaultsToAll(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{"action":"clear"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("clear default should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	if data["buffer"] != "all" {
		t.Errorf("default buffer = %v, want 'all'", data["buffer"])
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsConfigureClear_SpecificBuffers(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	buffers := []string{"network", "websocket", "actions", "logs"}
	for _, buffer := range buffers {
		t.Run(buffer, func(t *testing.T) {
			resp := callConfigureRaw(h, `{"action":"clear","buffer":"`+buffer+`"}`)
			result := parseToolResult(t, resp)
			if result.IsError {
				t.Fatalf("clear %s should succeed, got: %s", buffer, result.Content[0].Text)
			}

			data := extractResultJSON(t, result)
			if data["status"] != "ok" {
				t.Errorf("status = %v, want 'ok'", data["status"])
			}
			if data["buffer"] != buffer {
				t.Errorf("buffer = %v, want %q", data["buffer"], buffer)
			}

			assertSnakeCaseFields(t, string(resp.Result))
		})
	}
}

func TestToolsConfigureClear_UnknownBuffer(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{"action":"clear","buffer":"invalid_buf"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("clear invalid buffer should return isError:true")
	}
	if !strings.Contains(result.Content[0].Text, "invalid_buf") {
		t.Error("error should mention the invalid buffer name")
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsConfigureClear_InvalidJSON(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := h.toolConfigureClear(req, json.RawMessage(`{bad}`))
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("invalid JSON should return isError:true")
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

// ============================================
// configure(action:"noise_rule") — Response Fields
// ============================================

func TestToolsConfigureNoiseRule_ListAction(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{"action":"noise_rule","noise_action":"list"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("noise_rule list should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	if _, ok := data["rules"]; !ok {
		t.Error("response missing 'rules' field")
	}
	if _, ok := data["statistics"]; !ok {
		t.Error("response missing 'statistics' field")
	}

	// Verify statistics fields
	stats, _ := data["statistics"].(map[string]any)
	if stats == nil {
		t.Fatal("statistics should be a map")
	}
	for _, field := range []string{"total_filtered", "per_rule", "last_signal_at", "last_noise_at"} {
		if _, ok := stats[field]; !ok {
			t.Errorf("statistics missing field %q", field)
		}
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsConfigureNoiseRule_DefaultAction(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	// No noise_action should default to "list"
	resp := callConfigureRaw(h, `{"action":"noise_rule"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("noise_rule default should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	if _, ok := data["rules"]; !ok {
		t.Error("default action should return rules (list)")
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsConfigureNoiseRule_ResetAction(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{"action":"noise_rule","noise_action":"reset"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("noise_rule reset should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	if data["status"] != "ok" {
		t.Errorf("status = %v, want 'ok'", data["status"])
	}
	if _, ok := data["total_rules"]; !ok {
		t.Error("response missing 'total_rules' field")
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsConfigureNoiseRule_RemoveMissingRuleID(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{"action":"noise_rule","noise_action":"remove"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("noise_rule remove without rule_id should return isError:true")
	}
	if !strings.Contains(result.Content[0].Text, "rule_id") {
		t.Error("error should mention 'rule_id' parameter")
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsConfigureNoiseRule_UnknownSubAction(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{"action":"noise_rule","noise_action":"invalid_action"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("noise_rule with unknown sub-action should return isError:true")
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsConfigureNoiseRule_AutoDetect(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{"action":"noise_rule","noise_action":"auto_detect"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("noise_rule auto_detect should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	for _, field := range []string{"proposals", "total_rules", "proposals_count", "message"} {
		if _, ok := data[field]; !ok {
			t.Errorf("auto_detect response missing field %q", field)
		}
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

// ============================================
// configure(action:"store") — Response Fields
// ============================================

func TestToolsConfigureStore_ListDefault(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	// store with no sub-action defaults to "list"; namespace is required for list
	resp := callConfigureRaw(h, `{"action":"store","namespace":"test_ns"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("store default should succeed, got: %s", result.Content[0].Text)
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsConfigureStore_InvalidJSON(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := h.toolConfigureStore(req, json.RawMessage(`{bad}`))
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("store invalid JSON should return isError:true")
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

// ============================================
// configure(action:"load") — Response Fields
// ============================================

func TestToolsConfigureLoad_ResponseFields(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{"action":"load"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("load should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	if data["status"] != "ok" {
		t.Errorf("status = %v, want 'ok'", data["status"])
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

// ============================================
// configure(action:"test_boundary_start") — Response Fields
// ============================================

func TestToolsConfigureTestBoundaryStart_ResponseFields(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{"action":"test_boundary_start","test_id":"test-123","label":"My Test"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("test_boundary_start should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	for _, field := range []string{"status", "test_id", "label", "message"} {
		if _, ok := data[field]; !ok {
			t.Errorf("test_boundary_start response missing field %q", field)
		}
	}
	if data["status"] != "ok" {
		t.Errorf("status = %v, want 'ok'", data["status"])
	}
	if data["test_id"] != "test-123" {
		t.Errorf("test_id = %v, want 'test-123'", data["test_id"])
	}
	if data["label"] != "My Test" {
		t.Errorf("label = %v, want 'My Test'", data["label"])
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsConfigureTestBoundaryStart_MissingTestID(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{"action":"test_boundary_start"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("test_boundary_start without test_id should return isError:true")
	}
	if !strings.Contains(result.Content[0].Text, "test_id") {
		t.Error("error should mention 'test_id' parameter")
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsConfigureTestBoundaryStart_DefaultLabel(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{"action":"test_boundary_start","test_id":"abc"}`)
	result := parseToolResult(t, resp)
	data := extractResultJSON(t, result)

	label, _ := data["label"].(string)
	if !strings.Contains(label, "abc") {
		t.Errorf("default label should contain test_id, got: %q", label)
	}
}

// ============================================
// configure(action:"test_boundary_end") — Response Fields
// ============================================

func TestToolsConfigureTestBoundaryEnd_ResponseFields(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{"action":"test_boundary_end","test_id":"test-123"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("test_boundary_end should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	for _, field := range []string{"status", "test_id", "was_active", "message"} {
		if _, ok := data[field]; !ok {
			t.Errorf("test_boundary_end response missing field %q", field)
		}
	}
	if data["status"] != "ok" {
		t.Errorf("status = %v, want 'ok'", data["status"])
	}
	if data["test_id"] != "test-123" {
		t.Errorf("test_id = %v, want 'test-123'", data["test_id"])
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsConfigureTestBoundaryEnd_MissingTestID(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{"action":"test_boundary_end"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("test_boundary_end without test_id should return isError:true")
	}
	if !strings.Contains(result.Content[0].Text, "test_id") {
		t.Error("error should mention 'test_id' parameter")
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

// ============================================
// configure(action:"audit_log") — Response Fields
// ============================================

func TestToolsConfigureAuditLog_ResponseFields(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{"action":"audit_log"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("audit_log should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	if data["status"] != "ok" {
		t.Errorf("status = %v, want 'ok'", data["status"])
	}
	if _, ok := data["entries"]; !ok {
		t.Error("response missing 'entries' field")
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

// ============================================
// configure(action:"diff_sessions") — Response Fields
// ============================================

func TestToolsConfigureDiffSessions_ResponseFields(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{"action":"diff_sessions"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("diff_sessions should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	if data["status"] != "ok" {
		t.Errorf("status = %v, want 'ok'", data["status"])
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

// ============================================
// All configure actions safety net
// ============================================

func TestToolsConfigure_AllActions_ResponseStructure(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	actions := []struct {
		action string
		args   string
	}{
		{"health", `{"action":"health"}`},
		{"telemetry", `{"action":"telemetry"}`},
		{"clear", `{"action":"clear"}`},
		{"noise_rule", `{"action":"noise_rule","noise_action":"list"}`},
		{"load", `{"action":"load"}`},
		{"audit_log", `{"action":"audit_log"}`},
		{"diff_sessions", `{"action":"diff_sessions"}`},
		{"test_boundary_start", `{"action":"test_boundary_start","test_id":"test"}`},
		{"test_boundary_end", `{"action":"test_boundary_end","test_id":"test"}`},
	}

	for _, tc := range actions {
		t.Run(tc.action, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("configure(%s) PANICKED: %v", tc.action, r)
				}
			}()

			resp := callConfigureRaw(h, tc.args)
			if resp.Result == nil {
				t.Fatalf("configure(%s) returned nil result", tc.action)
			}

			result := parseToolResult(t, resp)
			if len(result.Content) == 0 {
				t.Errorf("configure(%s) should return at least one content block", tc.action)
			}
			if result.Content[0].Type != "text" {
				t.Errorf("configure(%s) content type = %q, want 'text'", tc.action, result.Content[0].Type)
			}

			if !result.IsError {
				assertSnakeCaseFields(t, string(resp.Result))
			}
		})
	}
}
