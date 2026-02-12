// tools_configure_noise_test.go — Coverage tests for noise and analyze(api_validation) handlers.
package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// ============================================
// toolConfigureNoise — 0% → 80%+
// ============================================

func TestToolConfigureNoise_List(t *testing.T) {
	t.Parallel()
	env := newConfigureTestEnv(t)

	result, ok := env.callConfigure(t, `{"action":"noise_rule","noise_action":"list"}`)
	if !ok {
		t.Fatal("noise list should return result")
	}
	if result.IsError {
		t.Fatalf("noise list should not error, got: %s", result.Content[0].Text)
	}

	data := parseResponseJSON(t, result)
	if _, ok := data["rules"]; !ok {
		t.Error("response should contain rules")
	}
	if _, ok := data["statistics"]; !ok {
		t.Error("response should contain statistics")
	}
}

func TestToolConfigureNoise_Reset(t *testing.T) {
	t.Parallel()
	env := newConfigureTestEnv(t)

	result, ok := env.callConfigure(t, `{"action":"noise_rule","noise_action":"reset"}`)
	if !ok {
		t.Fatal("noise reset should return result")
	}
	if result.IsError {
		t.Fatalf("noise reset should not error, got: %s", result.Content[0].Text)
	}

	data := parseResponseJSON(t, result)
	if status, _ := data["status"].(string); status != "ok" {
		t.Fatalf("status = %q, want ok", status)
	}
}

func TestToolConfigureNoise_AddRule(t *testing.T) {
	t.Parallel()
	env := newConfigureTestEnv(t)

	result, ok := env.callConfigure(t, `{"action":"noise_rule","noise_action":"add","rules":[{"category":"console","classification":"noisy","match_spec":{"message_regex":"test.*pattern"}}]}`)
	if !ok {
		t.Fatal("noise add should return result")
	}
	if result.IsError {
		t.Fatalf("noise add should not error, got: %s", result.Content[0].Text)
	}

	data := parseResponseJSON(t, result)
	rulesAdded, _ := data["rules_added"].(float64)
	if rulesAdded != 1 {
		t.Fatalf("rules_added = %v, want 1", rulesAdded)
	}
}

func TestToolConfigureNoise_RemoveMissingRuleID(t *testing.T) {
	t.Parallel()
	env := newConfigureTestEnv(t)

	result, ok := env.callConfigure(t, `{"action":"noise_rule","noise_action":"remove"}`)
	if !ok {
		t.Fatal("noise remove should return result")
	}
	if !result.IsError {
		t.Fatal("noise remove without rule_id should return error")
	}
	text := result.Content[0].Text
	if !strings.Contains(strings.ToLower(text), "rule_id") {
		t.Errorf("error should mention rule_id, got: %s", text)
	}
}

func TestToolConfigureNoise_AutoDetect(t *testing.T) {
	t.Parallel()
	env := newConfigureTestEnv(t)

	result, ok := env.callConfigure(t, `{"action":"noise_rule","noise_action":"auto_detect"}`)
	if !ok {
		t.Fatal("noise auto_detect should return result")
	}
	if result.IsError {
		t.Fatalf("noise auto_detect should not error, got: %s", result.Content[0].Text)
	}

	data := parseResponseJSON(t, result)
	if _, ok := data["proposals"]; !ok {
		t.Error("response should contain proposals")
	}
}

func TestToolConfigureNoise_UnknownAction(t *testing.T) {
	t.Parallel()
	env := newConfigureTestEnv(t)

	result, ok := env.callConfigure(t, `{"action":"noise_rule","noise_action":"invalid_action"}`)
	if !ok {
		t.Fatal("noise unknown action should return result")
	}
	if !result.IsError {
		t.Fatal("noise unknown action should return error")
	}
}

func TestToolConfigureNoise_InvalidJSON(t *testing.T) {
	t.Parallel()
	env := newConfigureTestEnv(t)

	args := json.RawMessage(`{bad json}`)
	req := JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := env.handler.toolConfigureNoise(req, args)

	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("invalid JSON should return error")
	}
}

// ============================================
// toolValidateAPI — 0% → 100%
// Routed via analyze({what: "api_validation"})
// ============================================

func TestToolValidateAPI_Analyze(t *testing.T) {
	t.Parallel()
	env := newConfigureTestEnv(t)

	args := json.RawMessage(`{"operation":"analyze"}`)
	req := JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := env.handler.toolValidateAPI(req, args)

	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("validate_api analyze should not error, got: %s", result.Content[0].Text)
	}

	data := parseResponseJSON(t, result)
	if status, _ := data["status"].(string); status != "ok" {
		t.Fatalf("status = %q, want ok", status)
	}
	if op, _ := data["operation"].(string); op != "analyze" {
		t.Fatalf("operation = %q, want analyze", op)
	}
}

func TestToolValidateAPI_Report(t *testing.T) {
	t.Parallel()
	env := newConfigureTestEnv(t)

	args := json.RawMessage(`{"operation":"report"}`)
	req := JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := env.handler.toolValidateAPI(req, args)

	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("validate_api report should not error, got: %s", result.Content[0].Text)
	}
}

func TestToolValidateAPI_Clear(t *testing.T) {
	t.Parallel()
	env := newConfigureTestEnv(t)

	args := json.RawMessage(`{"operation":"clear"}`)
	req := JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := env.handler.toolValidateAPI(req, args)

	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("validate_api clear should not error, got: %s", result.Content[0].Text)
	}
}

func TestToolValidateAPI_UnknownOperation(t *testing.T) {
	t.Parallel()
	env := newConfigureTestEnv(t)

	args := json.RawMessage(`{"operation":"invalid"}`)
	req := JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := env.handler.toolValidateAPI(req, args)

	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("validate_api with invalid operation should return error")
	}
	text := result.Content[0].Text
	if !strings.Contains(strings.ToLower(text), "operation") {
		t.Errorf("error should mention operation, got: %s", text)
	}
}

func TestToolValidateAPI_InvalidJSON(t *testing.T) {
	t.Parallel()
	env := newConfigureTestEnv(t)

	args := json.RawMessage(`{bad json}`)
	req := JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := env.handler.toolValidateAPI(req, args)

	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("invalid JSON should return error")
	}
}
