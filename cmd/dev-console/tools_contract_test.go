// tools_contract_test.go — Response shape contracts for configure, generate, and interact tools.
// Lighter coverage than observe contracts — focuses on key actions that return JSON.
//
// Run: go test ./cmd/dev-console -run "TestContract" -v
package main

import (
	"encoding/json"
	"testing"
)

// ============================================
// Configure Tool Contracts
// ============================================

// configureContractEnv wraps configureTestEnv with contract assertions.
type configureContractEnv struct {
	*configureTestEnv
}

func newConfigureContractEnv(t *testing.T) *configureContractEnv {
	t.Helper()
	return &configureContractEnv{configureTestEnv: newConfigureTestEnv(t)}
}

func TestContractConfigure_Health(t *testing.T) {
	env := newConfigureContractEnv(t)
	result, ok := env.callConfigure(t, `{"action":"health"}`)
	if !ok {
		t.Fatal("configure health: no result")
	}

	data := parseResponseJSON(t, result)
	assertObjectShape(t, "health", data, []fieldSpec{
		required("server", "object"),
		required("buffers", "object"),
		required("memory", "object"),
	})
}

func TestContractConfigure_Clear(t *testing.T) {
	env := newConfigureContractEnv(t)
	result, ok := env.callConfigure(t, `{"action":"clear","buffer":"all"}`)
	if !ok {
		t.Fatal("configure clear: no result")
	}

	data := parseResponseJSON(t, result)
	assertObjectShape(t, "clear", data, []fieldSpec{
		required("status", "string"),
		required("buffer", "string"),
	})
}

func TestContractConfigure_NoiseRule_List(t *testing.T) {
	env := newConfigureContractEnv(t)
	result, ok := env.callConfigure(t, `{"action":"noise_rule","noise_action":"list"}`)
	if !ok {
		t.Fatal("configure noise_rule list: no result")
	}

	data := parseResponseJSON(t, result)
	assertObjectShape(t, "noise_rule_list", data, []fieldSpec{
		required("rules", "array"),
		required("statistics", "object"),
	})
}

func TestContractConfigure_Streaming_Status(t *testing.T) {
	env := newConfigureContractEnv(t)
	result, ok := env.callConfigure(t, `{"action":"streaming","streaming_action":"status"}`)
	if !ok {
		t.Fatal("configure streaming status: no result")
	}

	data := parseResponseJSON(t, result)
	assertObjectShape(t, "streaming_status", data, []fieldSpec{
		required("config", "object"),
		required("notify_count", "number"),
		required("pending", "number"),
	})
}

func TestContractConfigure_UnknownAction_Error(t *testing.T) {
	env := newConfigureContractEnv(t)
	result, ok := env.callConfigure(t, `{"action":"invalid_action_xyz"}`)
	if !ok {
		t.Fatal("configure unknown action: no result")
	}
	assertStructuredError(t, "configure (unknown action)", result)
}

func TestContractConfigure_MissingAction_Error(t *testing.T) {
	env := newConfigureContractEnv(t)
	result, ok := env.callConfigure(t, `{}`)
	if !ok {
		t.Fatal("configure missing action: no result")
	}
	assertStructuredError(t, "configure (missing action)", result)
}

// ============================================
// Generate Tool Contracts
// ============================================

// generateContractEnv wraps generateTestEnv with contract assertions.
type generateContractEnv struct {
	*generateTestEnv
}

func newGenerateContractEnv(t *testing.T) *generateContractEnv {
	t.Helper()
	return &generateContractEnv{generateTestEnv: newGenerateTestEnv(t)}
}

func TestContractGenerate_Reproduction(t *testing.T) {
	env := newGenerateContractEnv(t)
	result, ok := env.callGenerate(t, `{"format":"reproduction"}`)
	if !ok {
		t.Fatal("generate reproduction: no result")
	}
	// Reproduction returns text content, not JSON
	assertNonErrorResponse(t, "reproduction", result)
}

func TestContractGenerate_Test(t *testing.T) {
	env := newGenerateContractEnv(t)
	result, ok := env.callGenerate(t, `{"format":"test"}`)
	if !ok {
		t.Fatal("generate test: no result")
	}
	assertNonErrorResponse(t, "test", result)
}

func TestContractGenerate_PRSummary(t *testing.T) {
	env := newGenerateContractEnv(t)
	result, ok := env.callGenerate(t, `{"format":"pr_summary"}`)
	if !ok {
		t.Fatal("generate pr_summary: no result")
	}
	assertNonErrorResponse(t, "pr_summary", result)
}

func TestContractGenerate_HAR(t *testing.T) {
	env := newGenerateContractEnv(t)
	result, ok := env.callGenerate(t, `{"format":"har"}`)
	if !ok {
		t.Fatal("generate har: no result")
	}
	assertNonErrorResponse(t, "har", result)
}

func TestContractGenerate_CSP(t *testing.T) {
	env := newGenerateContractEnv(t)
	result, ok := env.callGenerate(t, `{"format":"csp"}`)
	if !ok {
		t.Fatal("generate csp: no result")
	}
	assertNonErrorResponse(t, "csp", result)
}

func TestContractGenerate_SRI(t *testing.T) {
	env := newGenerateContractEnv(t)
	result, ok := env.callGenerate(t, `{"format":"sri"}`)
	if !ok {
		t.Fatal("generate sri: no result")
	}
	assertNonErrorResponse(t, "sri", result)
}

func TestContractGenerate_SARIF(t *testing.T) {
	env := newGenerateContractEnv(t)
	result, ok := env.callGenerate(t, `{"format":"sarif"}`)
	if !ok {
		t.Fatal("generate sarif: no result")
	}
	assertNonErrorResponse(t, "sarif", result)
}

func TestContractGenerate_UnknownFormat_Error(t *testing.T) {
	env := newGenerateContractEnv(t)
	result, ok := env.callGenerate(t, `{"format":"invalid_format_xyz"}`)
	if !ok {
		t.Fatal("generate unknown format: no result")
	}
	assertStructuredError(t, "generate (unknown format)", result)
}

func TestContractGenerate_MissingFormat_Error(t *testing.T) {
	env := newGenerateContractEnv(t)
	result, ok := env.callGenerate(t, `{}`)
	if !ok {
		t.Fatal("generate missing format: no result")
	}
	assertStructuredError(t, "generate (missing format)", result)
}

// ============================================
// Interact Tool Contracts
// ============================================

// interactContractEnv wraps interactTestEnv with contract assertions.
type interactContractEnv struct {
	*interactTestEnv
}

func newInteractContractEnv(t *testing.T) *interactContractEnv {
	t.Helper()
	return &interactContractEnv{interactTestEnv: newInteractTestEnv(t)}
}

func TestContractInteract_ListStates(t *testing.T) {
	env := newInteractContractEnv(t)
	result, ok := env.callInteract(t, `{"action":"list_states"}`)
	if !ok {
		t.Fatal("interact list_states: no result")
	}

	data := parseResponseJSON(t, result)
	assertObjectShape(t, "list_states", data, []fieldSpec{
		required("states", "array"),
		required("count", "number"),
	})
}

func TestContractInteract_SaveState_MissingName(t *testing.T) {
	env := newInteractContractEnv(t)
	result, ok := env.callInteract(t, `{"action":"save_state"}`)
	if !ok {
		t.Fatal("interact save_state: no result")
	}
	assertStructuredError(t, "save_state (missing snapshot_name)", result)
}

func TestContractInteract_UnknownAction_Error(t *testing.T) {
	env := newInteractContractEnv(t)
	result, ok := env.callInteract(t, `{"action":"invalid_action_xyz"}`)
	if !ok {
		t.Fatal("interact unknown action: no result")
	}
	assertStructuredError(t, "interact (unknown action)", result)
}

func TestContractInteract_MissingAction_Error(t *testing.T) {
	env := newInteractContractEnv(t)
	result, ok := env.callInteract(t, `{}`)
	if !ok {
		t.Fatal("interact missing action: no result")
	}
	assertStructuredError(t, "interact (missing action)", result)
}

// ============================================
// Bad Path Contracts — Invalid JSON
// ============================================

func TestContractBadPath_Observe_InvalidJSON(t *testing.T) {
	s := newScenario(t)
	result, ok := s.callObserveWithArgs(t, `{invalid json`)
	if !ok {
		t.Fatal("observe invalid JSON: no result")
	}
	assertStructuredErrorCode(t, "observe (invalid JSON)", result, "invalid_json")
}

func TestContractBadPath_Configure_InvalidJSON(t *testing.T) {
	env := newConfigureContractEnv(t)
	result, ok := env.callConfigure(t, `{invalid json`)
	if !ok {
		t.Fatal("configure invalid JSON: no result")
	}
	assertStructuredErrorCode(t, "configure (invalid JSON)", result, "invalid_json")
}

func TestContractBadPath_Generate_InvalidJSON(t *testing.T) {
	env := newGenerateContractEnv(t)
	result, ok := env.callGenerate(t, `{invalid json`)
	if !ok {
		t.Fatal("generate invalid JSON: no result")
	}
	assertStructuredErrorCode(t, "generate (invalid JSON)", result, "invalid_json")
}

func TestContractBadPath_Interact_InvalidJSON(t *testing.T) {
	env := newInteractContractEnv(t)
	result, ok := env.callInteract(t, `{invalid json`)
	if !ok {
		t.Fatal("interact invalid JSON: no result")
	}
	assertStructuredErrorCode(t, "interact (invalid JSON)", result, "invalid_json")
}

// ============================================
// Bad Path Contracts — Missing Required Params
// ============================================

func TestContractBadPath_Configure_QueryDOM_MissingSelector(t *testing.T) {
	env := newConfigureContractEnv(t)
	result, ok := env.callConfigure(t, `{"action":"query_dom"}`)
	if !ok {
		t.Fatal("configure query_dom: no result")
	}
	assertStructuredErrorCode(t, "query_dom (missing selector)", result, "missing_param")
}

func TestContractBadPath_Configure_TestBoundary_MissingTestID(t *testing.T) {
	env := newConfigureContractEnv(t)
	result, ok := env.callConfigure(t, `{"action":"test_boundary_start"}`)
	if !ok {
		t.Fatal("configure test_boundary_start: no result")
	}
	assertStructuredErrorCode(t, "test_boundary_start (missing test_id)", result, "missing_param")
}

func TestContractBadPath_Configure_NoiseRule_Remove_MissingRuleID(t *testing.T) {
	env := newConfigureContractEnv(t)
	result, ok := env.callConfigure(t, `{"action":"noise_rule","noise_action":"remove"}`)
	if !ok {
		t.Fatal("configure noise_rule remove: no result")
	}
	assertStructuredErrorCode(t, "noise_rule remove (missing rule_id)", result, "missing_param")
}

func TestContractBadPath_Interact_Highlight_MissingSelector(t *testing.T) {
	env := newInteractContractEnv(t)
	result, ok := env.callInteract(t, `{"action":"highlight"}`)
	if !ok {
		t.Fatal("interact highlight: no result")
	}
	assertStructuredErrorCode(t, "highlight (missing selector)", result, "missing_param")
}

func TestContractBadPath_Interact_ExecuteJS_MissingScript(t *testing.T) {
	env := newInteractContractEnv(t)
	result, ok := env.callInteract(t, `{"action":"execute_js"}`)
	if !ok {
		t.Fatal("interact execute_js: no result")
	}
	assertStructuredErrorCode(t, "execute_js (missing script)", result, "missing_param")
}

func TestContractBadPath_Interact_Navigate_MissingURL(t *testing.T) {
	env := newInteractContractEnv(t)
	result, ok := env.callInteract(t, `{"action":"navigate"}`)
	if !ok {
		t.Fatal("interact navigate: no result")
	}
	assertStructuredErrorCode(t, "navigate (missing url)", result, "missing_param")
}

func TestContractBadPath_Interact_LoadState_MissingName(t *testing.T) {
	env := newInteractContractEnv(t)
	result, ok := env.callInteract(t, `{"action":"load_state"}`)
	if !ok {
		t.Fatal("interact load_state: no result")
	}
	assertStructuredErrorCode(t, "load_state (missing snapshot_name)", result, "missing_param")
}

func TestContractBadPath_Interact_DeleteState_MissingName(t *testing.T) {
	env := newInteractContractEnv(t)
	result, ok := env.callInteract(t, `{"action":"delete_state"}`)
	if !ok {
		t.Fatal("interact delete_state: no result")
	}
	assertStructuredErrorCode(t, "delete_state (missing snapshot_name)", result, "missing_param")
}

// ============================================
// Bad Path Contracts — Invalid Param Values
// ============================================

func TestContractBadPath_Configure_Clear_InvalidBuffer(t *testing.T) {
	env := newConfigureContractEnv(t)
	result, ok := env.callConfigure(t, `{"action":"clear","buffer":"nonexistent_buffer"}`)
	if !ok {
		t.Fatal("configure clear invalid buffer: no result")
	}
	assertStructuredErrorCode(t, "clear (invalid buffer)", result, "invalid_param")
}

func TestContractBadPath_Configure_NoiseRule_UnknownAction(t *testing.T) {
	env := newConfigureContractEnv(t)
	result, ok := env.callConfigure(t, `{"action":"noise_rule","noise_action":"invalid_xyz"}`)
	if !ok {
		t.Fatal("configure noise_rule unknown action: no result")
	}
	assertStructuredErrorCode(t, "noise_rule (unknown noise_action)", result, "unknown_mode")
}

// ============================================
// Bad Path Contracts — Pilot Disabled
// ============================================

func TestContractBadPath_Interact_Navigate_PilotDisabled(t *testing.T) {
	env := newInteractContractEnv(t)
	// Pilot is disabled by default in test env
	result, ok := env.callInteract(t, `{"action":"navigate","url":"https://example.com"}`)
	if !ok {
		t.Fatal("interact navigate pilot disabled: no result")
	}
	assertStructuredErrorCode(t, "navigate (pilot disabled)", result, "pilot_disabled")
}

func TestContractBadPath_Interact_ExecuteJS_PilotDisabled(t *testing.T) {
	env := newInteractContractEnv(t)
	result, ok := env.callInteract(t, `{"action":"execute_js","script":"1+1"}`)
	if !ok {
		t.Fatal("interact execute_js pilot disabled: no result")
	}
	assertStructuredErrorCode(t, "execute_js (pilot disabled)", result, "pilot_disabled")
}

func TestContractBadPath_Interact_Refresh_PilotDisabled(t *testing.T) {
	env := newInteractContractEnv(t)
	result, ok := env.callInteract(t, `{"action":"refresh"}`)
	if !ok {
		t.Fatal("interact refresh pilot disabled: no result")
	}
	assertStructuredErrorCode(t, "refresh (pilot disabled)", result, "pilot_disabled")
}

func TestContractBadPath_Interact_Back_PilotDisabled(t *testing.T) {
	env := newInteractContractEnv(t)
	result, ok := env.callInteract(t, `{"action":"back"}`)
	if !ok {
		t.Fatal("interact back pilot disabled: no result")
	}
	assertStructuredErrorCode(t, "back (pilot disabled)", result, "pilot_disabled")
}

func TestContractBadPath_Interact_NewTab_PilotDisabled(t *testing.T) {
	env := newInteractContractEnv(t)
	result, ok := env.callInteract(t, `{"action":"new_tab","url":"https://example.com"}`)
	if !ok {
		t.Fatal("interact new_tab pilot disabled: no result")
	}
	assertStructuredErrorCode(t, "new_tab (pilot disabled)", result, "pilot_disabled")
}

func TestContractBadPath_Interact_Forward_PilotDisabled(t *testing.T) {
	env := newInteractContractEnv(t)
	result, ok := env.callInteract(t, `{"action":"forward"}`)
	if !ok {
		t.Fatal("interact forward pilot disabled: no result")
	}
	assertStructuredErrorCode(t, "forward (pilot disabled)", result, "pilot_disabled")
}

func TestContractBadPath_Interact_Highlight_PilotDisabled(t *testing.T) {
	env := newInteractContractEnv(t)
	result, ok := env.callInteract(t, `{"action":"highlight","selector":"#test"}`)
	if !ok {
		t.Fatal("interact highlight pilot disabled: no result")
	}
	assertStructuredErrorCode(t, "highlight (pilot disabled)", result, "pilot_disabled")
}

// ============================================
// Bad Path Contracts — No Data
// ============================================

func TestContractBadPath_Interact_LoadState_NotFound(t *testing.T) {
	env := newInteractContractEnv(t)
	result, ok := env.callInteract(t, `{"action":"load_state","snapshot_name":"nonexistent_state_xyz"}`)
	if !ok {
		t.Fatal("interact load_state not found: no result")
	}
	assertStructuredErrorCode(t, "load_state (not found)", result, "no_data")
}

func TestContractBadPath_Observe_CommandResult_NotFound(t *testing.T) {
	s := newScenario(t)
	result, ok := s.callObserveWithArgs(t, `{"what":"command_result","correlation_id":"nonexistent_corr_id"}`)
	if !ok {
		t.Fatal("observe command_result not found: no result")
	}
	assertStructuredErrorCode(t, "command_result (not found)", result, "no_data")
}

// ============================================
// Helpers
// ============================================

// assertNonErrorResponse verifies a result has content and is not an error.
func assertNonErrorResponse(t *testing.T, label string, result MCPToolResult) {
	t.Helper()
	if result.IsError {
		t.Errorf("%s: unexpected error response: %s", label, firstText(result))
		return
	}
	if len(result.Content) == 0 {
		t.Errorf("%s: no content blocks", label)
		return
	}
	if result.Content[0].Text == "" {
		t.Errorf("%s: empty text content", label)
	}
}

// firstText extracts the first text block from a result, or "".
func firstText(result MCPToolResult) string {
	if len(result.Content) > 0 {
		return result.Content[0].Text
	}
	return ""
}

// callObserveWithArgs is a helper to call observe with custom JSON args.
func (s *scenario) callObserveWithArgs(t *testing.T, argsJSON string) (MCPToolResult, bool) {
	t.Helper()
	args := json.RawMessage(argsJSON)
	req := JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := s.handler.toolObserve(req, args)
	if resp.Result == nil {
		return MCPToolResult{}, false
	}
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	return result, true
}
