// tools_interact_rich_cmdresult_test.go — TDD tests for Rich Action command result handling.
// Tests failed command visibility (IsError signaling), queued/final markers,
// effective context surfacing, subtitle correlation IDs, and diagnostic hints.
//
// Run: go test ./cmd/dev-console -run "TestCommandResult|TestQueuedResponse|TestSubtitle" -v
package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/dev-console/dev-console/internal/queries"
)

// ============================================
// Failed command visibility: IsError signaling
// ============================================

func TestCommandResult_ExpiredSetsIsError(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	// Queue a command
	result, ok := env.callInteract(t, `{"what":"click","selector":"#btn","background":true}`)
	if !ok || result.IsError {
		t.Fatal("click should succeed")
	}

	pq := env.capture.GetLastPendingQuery()
	corrID := pq.CorrelationID

	// Simulate expiry — extension never responded
	env.capture.ExpireCommand(corrID)

	// Observe the expired command
	req := JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.toolObserveCommandResult(req, args)

	var observeResult MCPToolResult
	if err := json.Unmarshal(resp.Result, &observeResult); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	if !observeResult.IsError {
		t.Error("Expired command MUST set IsError=true so LLMs recognize failure")
	}

	text := observeResult.Content[0].Text
	if !strings.Contains(text, "extension_timeout") {
		t.Errorf("Expired command should include error code 'extension_timeout', got: %s", text)
	}
	if !strings.Contains(text, "retry") {
		t.Errorf("Expired command should include retry instructions, got: %s", text)
	}
}

func TestCommandResult_CompleteWithErrorSetsIsError(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	// Queue a command
	result, ok := env.callInteract(t, `{"what":"click","selector":"#btn","background":true}`)
	if !ok || result.IsError {
		t.Fatal("click should succeed")
	}

	pq := env.capture.GetLastPendingQuery()
	corrID := pq.CorrelationID

	// Simulate extension completing with an error
	env.capture.CompleteCommand(corrID, json.RawMessage(`null`), "Element not found: #btn")

	// Observe the failed command
	req := JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.toolObserveCommandResult(req, args)

	var observeResult MCPToolResult
	if err := json.Unmarshal(resp.Result, &observeResult); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	if !observeResult.IsError {
		t.Error("Command completed with error MUST set IsError=true so LLMs recognize failure")
	}

	text := observeResult.Content[0].Text
	if !strings.Contains(text, "FAILED") {
		t.Errorf("Failed command summary should include 'FAILED', got: %s", text)
	}
	if !strings.Contains(text, "Element not found") {
		t.Errorf("Failed command should include error message, got: %s", text)
	}
}

func TestCommandResult_EmbeddedFailureSetsIsError(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, ok := env.callInteract(t, `{"what":"click","selector":"#btn","background":true}`)
	if !ok || result.IsError {
		t.Fatal("click should succeed")
	}

	pq := env.capture.GetLastPendingQuery()
	corrID := pq.CorrelationID

	// Extension reported failure inside the result payload without setting command error.
	env.capture.CompleteCommand(corrID, json.RawMessage(`{"success":false,"error":"selector_not_found","message":"#btn not found"}`), "")

	req := JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.toolObserveCommandResult(req, args)

	var observeResult MCPToolResult
	if err := json.Unmarshal(resp.Result, &observeResult); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	if !observeResult.IsError {
		t.Fatal("Embedded success=false MUST set IsError=true")
	}

	text := observeResult.Content[0].Text
	if !strings.Contains(text, "FAILED") {
		t.Fatalf("Expected FAILED summary, got: %s", text)
	}
	if !strings.Contains(text, "selector_not_found") {
		t.Fatalf("Expected embedded error to surface, got: %s", text)
	}
}

func TestCommandResult_EmbeddedCSPFailureAddsCSPMarkers(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, ok := env.callInteract(t, `{"what":"execute_js","script":"(() => 1)()","background":true}`)
	if !ok || result.IsError {
		t.Fatal("execute_js should queue successfully")
	}

	pq := env.capture.GetLastPendingQuery()
	corrID := pq.CorrelationID

	env.capture.CompleteCommand(corrID, json.RawMessage(`{"success":false,"error":"csp_blocked_all_worlds","message":"Page CSP blocks dynamic script execution"}`), "")

	req := JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.toolObserveCommandResult(req, args)

	var observeResult MCPToolResult
	if err := json.Unmarshal(resp.Result, &observeResult); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}
	if !observeResult.IsError {
		t.Fatal("CSP failure must set IsError=true")
	}

	var responseData map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(observeResult.Content[0].Text)), &responseData); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}
	if blocked, _ := responseData["csp_blocked"].(bool); !blocked {
		t.Fatalf("csp_blocked = %v, want true", responseData["csp_blocked"])
	}
	if responseData["failure_cause"] != "csp" {
		t.Fatalf("failure_cause = %v, want csp", responseData["failure_cause"])
	}
}

func TestCommandResult_ErrorStatusCSPFailureIncludesRetryHint(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, ok := env.callInteract(t, `{"what":"navigate","url":"https://example.com","background":true}`)
	if !ok || result.IsError {
		t.Fatal("navigate should queue successfully")
	}

	pq := env.capture.GetLastPendingQuery()
	corrID := pq.CorrelationID
	env.capture.ApplyCommandResult(corrID, "error", json.RawMessage(`{"success":false,"error":"csp_blocked_page","message":"This page blocks extension script execution.","csp_blocked":true,"failure_cause":"csp"}`), "csp_blocked_page")

	req := JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.toolObserveCommandResult(req, args)

	var observeResult MCPToolResult
	if err := json.Unmarshal(resp.Result, &observeResult); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}
	if !observeResult.IsError {
		t.Fatal("CSP error status must set IsError=true")
	}

	var responseData map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(observeResult.Content[0].Text)), &responseData); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}
	if blocked, _ := responseData["csp_blocked"].(bool); !blocked {
		t.Fatalf("csp_blocked = %v, want true", responseData["csp_blocked"])
	}
	if responseData["failure_cause"] != "csp" {
		t.Fatalf("failure_cause = %v, want csp", responseData["failure_cause"])
	}
	retry, _ := responseData["retry"].(string)
	if !strings.Contains(strings.ToLower(retry), "navigate") {
		t.Fatalf("retry hint should include navigation guidance, got: %q", retry)
	}
}

func TestCommandResult_SuccessDoesNotSetIsError(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	// Queue a command
	result, ok := env.callInteract(t, `{"what":"click","selector":"#btn","background":true}`)
	if !ok || result.IsError {
		t.Fatal("click should succeed")
	}

	pq := env.capture.GetLastPendingQuery()
	corrID := pq.CorrelationID

	// Simulate successful completion
	env.capture.CompleteCommand(corrID, json.RawMessage(`{"success":true}`), "")

	// Observe the successful command
	req := JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.toolObserveCommandResult(req, args)

	var observeResult MCPToolResult
	if err := json.Unmarshal(resp.Result, &observeResult); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	if observeResult.IsError {
		t.Error("Successful command should NOT set IsError")
	}

	text := observeResult.Content[0].Text
	if strings.Contains(text, "FAILED") {
		t.Errorf("Successful command should not contain 'FAILED', got: %s", text)
	}
}

// ============================================
// Issue #92: queued/final markers on async responses
// ============================================

func TestQueuedResponse_HasQueuedAndFinalMarkers(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, _ := env.callInteract(t, `{"what":"click","selector":"#btn","background":true}`)
	var responseData map[string]any
	_ = json.Unmarshal([]byte(extractJSONFromText(result.Content[0].Text)), &responseData)

	if responseData["status"] != "queued" {
		t.Fatalf("status = %v, want queued", responseData["status"])
	}
	if queued, _ := responseData["queued"].(bool); !queued {
		t.Fatalf("queued response should have queued=true, got %v", responseData["queued"])
	}
	if final, _ := responseData["final"].(bool); final {
		t.Fatalf("queued response should have final=false, got %v", responseData["final"])
	}
}

func TestCommandResult_CompleteHasFinalTrue(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	// Queue async to avoid sync-wait-for-extension
	result, _ := env.callInteract(t, `{"what":"click","selector":"#btn","background":true}`)
	var resultData map[string]any
	_ = json.Unmarshal([]byte(extractJSONFromText(result.Content[0].Text)), &resultData)
	corrID := resultData["correlation_id"].(string)

	env.capture.CompleteCommand(corrID, json.RawMessage(`{"success":true}`), "")

	req := JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.toolObserveCommandResult(req, args)

	var observeResult MCPToolResult
	_ = json.Unmarshal(resp.Result, &observeResult)

	var responseData map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(observeResult.Content[0].Text)), &responseData); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	finalVal, ok := responseData["final"].(bool)
	if !ok || !finalVal {
		t.Fatalf("complete command should have final=true, got %v", responseData["final"])
	}
}

func TestCommandResult_ErrorHasFinalTrue(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, _ := env.callInteract(t, `{"what":"click","selector":"#btn","background":true}`)
	var resultData map[string]any
	_ = json.Unmarshal([]byte(extractJSONFromText(result.Content[0].Text)), &resultData)
	corrID := resultData["correlation_id"].(string)

	env.capture.CompleteCommand(corrID, nil, "element_not_found")

	req := JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.toolObserveCommandResult(req, args)

	var observeResult MCPToolResult
	_ = json.Unmarshal(resp.Result, &observeResult)

	var responseData map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(observeResult.Content[0].Text)), &responseData); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	finalVal, ok := responseData["final"].(bool)
	if !ok || !finalVal {
		t.Fatalf("error command should have final=true, got %v", responseData["final"])
	}
}

// ============================================
// Issue #91: effective_tab_id and effective_url surfaced
// ============================================

func TestCommandResult_EffectiveContextSurfaced(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, _ := env.callInteract(t, `{"what":"click","selector":"#btn","tab_id":42,"background":true}`)
	var resultData map[string]any
	_ = json.Unmarshal([]byte(extractJSONFromText(result.Content[0].Text)), &resultData)
	corrID := resultData["correlation_id"].(string)

	extensionResult := json.RawMessage(`{
		"success": true,
		"action": "click",
		"resolved_tab_id": 42,
		"resolved_url": "https://example.com/page1",
		"effective_tab_id": 42,
		"effective_url": "https://example.com/page2"
	}`)
	env.capture.CompleteCommand(corrID, extensionResult, "")

	req := JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.toolObserveCommandResult(req, args)

	var observeResult MCPToolResult
	_ = json.Unmarshal(resp.Result, &observeResult)

	var responseData map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(observeResult.Content[0].Text)), &responseData); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	if responseData["effective_tab_id"] != float64(42) {
		t.Fatalf("effective_tab_id = %v, want 42", responseData["effective_tab_id"])
	}
	if responseData["effective_url"] != "https://example.com/page2" {
		t.Fatalf("effective_url = %v, want https://example.com/page2", responseData["effective_url"])
	}
	if responseData["resolved_url"] != "https://example.com/page1" {
		t.Fatalf("resolved_url = %v, want https://example.com/page1", responseData["resolved_url"])
	}
}

func TestCommandResult_WorldFallbackMetadataSurfaced(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, _ := env.callInteract(t, `{"what":"click","selector":"#btn","background":true}`)
	var resultData map[string]any
	_ = json.Unmarshal([]byte(extractJSONFromText(result.Content[0].Text)), &resultData)
	corrID := resultData["correlation_id"].(string)

	extensionResult := json.RawMessage(`{
		"success": true,
		"action": "click",
		"execution_world": "isolated",
		"fallback_attempted": true,
		"main_world_status": "error",
		"isolated_world_status": "success",
		"fallback_summary": "Error: MAIN world execution FAILED. Fallback in ISOLATED is SUCCESS."
	}`)
	env.capture.CompleteCommand(corrID, extensionResult, "")

	req := JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.toolObserveCommandResult(req, args)

	var observeResult MCPToolResult
	_ = json.Unmarshal(resp.Result, &observeResult)

	var responseData map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(observeResult.Content[0].Text)), &responseData); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	if responseData["execution_world"] != "isolated" {
		t.Fatalf("execution_world = %v, want isolated", responseData["execution_world"])
	}
	if attempted, _ := responseData["fallback_attempted"].(bool); !attempted {
		t.Fatalf("fallback_attempted = %v, want true", responseData["fallback_attempted"])
	}
	if responseData["main_world_status"] != "error" {
		t.Fatalf("main_world_status = %v, want error", responseData["main_world_status"])
	}
	if responseData["isolated_world_status"] != "success" {
		t.Fatalf("isolated_world_status = %v, want success", responseData["isolated_world_status"])
	}
	if responseData["fallback_summary"] != "Error: MAIN world execution FAILED. Fallback in ISOLATED is SUCCESS." {
		t.Fatalf("fallback_summary = %v, want SUCCESS summary", responseData["fallback_summary"])
	}
}

func TestCommandResult_WorldFallbackErrorMetadataSurfaced(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, _ := env.callInteract(t, `{"what":"click","selector":"#btn","background":true}`)
	var resultData map[string]any
	_ = json.Unmarshal([]byte(extractJSONFromText(result.Content[0].Text)), &resultData)
	corrID := resultData["correlation_id"].(string)

	extensionResult := json.RawMessage(`{
		"success": false,
		"action": "click",
		"error": "dom_world_fallback_failed",
		"message": "Error: MAIN world execution FAILED. Fallback in ISOLATED is ERROR.",
		"execution_world": "isolated",
		"fallback_attempted": true,
		"main_world_status": "error",
		"isolated_world_status": "error",
		"fallback_summary": "Error: MAIN world execution FAILED. Fallback in ISOLATED is ERROR."
	}`)
	env.capture.ApplyCommandResult(corrID, "error", extensionResult, "Error: MAIN world execution FAILED. Fallback in ISOLATED is ERROR.")

	req := JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.toolObserveCommandResult(req, args)

	var observeResult MCPToolResult
	_ = json.Unmarshal(resp.Result, &observeResult)
	if !observeResult.IsError {
		t.Fatal("fallback ERROR outcome must set IsError=true")
	}

	var responseData map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(observeResult.Content[0].Text)), &responseData); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	if responseData["execution_world"] != "isolated" {
		t.Fatalf("execution_world = %v, want isolated", responseData["execution_world"])
	}
	if attempted, _ := responseData["fallback_attempted"].(bool); !attempted {
		t.Fatalf("fallback_attempted = %v, want true", responseData["fallback_attempted"])
	}
	if responseData["main_world_status"] != "error" {
		t.Fatalf("main_world_status = %v, want error", responseData["main_world_status"])
	}
	if responseData["isolated_world_status"] != "error" {
		t.Fatalf("isolated_world_status = %v, want error", responseData["isolated_world_status"])
	}
	if responseData["fallback_summary"] != "Error: MAIN world execution FAILED. Fallback in ISOLATED is ERROR." {
		t.Fatalf("fallback_summary = %v, want ERROR summary", responseData["fallback_summary"])
	}
}

// ============================================
// Issue #92 follow-up: queued=false on non-queued responses
// ============================================

func TestCommandResult_CompleteHasQueuedFalse(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, _ := env.callInteract(t, `{"what":"click","selector":"#btn","background":true}`)
	var resultData map[string]any
	_ = json.Unmarshal([]byte(extractJSONFromText(result.Content[0].Text)), &resultData)
	corrID := resultData["correlation_id"].(string)

	env.capture.CompleteCommand(corrID, json.RawMessage(`{"success":true}`), "")

	req := JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.toolObserveCommandResult(req, args)

	var observeResult MCPToolResult
	_ = json.Unmarshal(resp.Result, &observeResult)

	var responseData map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(observeResult.Content[0].Text)), &responseData); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	if queued, ok := responseData["queued"].(bool); !ok || queued {
		t.Fatalf("complete command should have queued=false, got %v", responseData["queued"])
	}
}

// ============================================
// Issue #92 follow-up: final=true on expired/timeout
// ============================================

func TestCommandResult_ExpiredHasFinalTrue(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, _ := env.callInteract(t, `{"what":"click","selector":"#btn","background":true}`)
	var resultData map[string]any
	_ = json.Unmarshal([]byte(extractJSONFromText(result.Content[0].Text)), &resultData)
	corrID := resultData["correlation_id"].(string)

	env.capture.ExpireCommand(corrID)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.toolObserveCommandResult(req, args)

	var observeResult MCPToolResult
	_ = json.Unmarshal(resp.Result, &observeResult)

	var responseData map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(observeResult.Content[0].Text)), &responseData); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	finalVal, ok := responseData["final"].(bool)
	if !ok || !finalVal {
		t.Fatalf("expired command should have final=true, got %v", responseData["final"])
	}
	if queued, ok := responseData["queued"].(bool); !ok || queued {
		t.Fatalf("expired command should have queued=false, got %v", responseData["queued"])
	}
	if responseData["error"] != ErrExtTimeout {
		t.Fatalf("expired command should have error=%s, got %v", ErrExtTimeout, responseData["error"])
	}
}

func TestCommandResult_TimeoutHasFinalTrue(t *testing.T) {
	env := newInteractTestEnv(t)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	cmd := queries.CommandResult{
		CorrelationID: "timeout_cmd_123",
		Status:        "timeout",
		Error:         "extension did not respond",
		CreatedAt:     time.Now().Add(-2 * time.Second),
	}
	resp := env.handler.formatCommandResult(req, cmd, cmd.CorrelationID)

	var observeResult MCPToolResult
	_ = json.Unmarshal(resp.Result, &observeResult)

	var responseData map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(observeResult.Content[0].Text)), &responseData); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	finalVal, ok := responseData["final"].(bool)
	if !ok || !finalVal {
		t.Fatalf("timeout command should have final=true, got %v", responseData["final"])
	}
	if queued, ok := responseData["queued"].(bool); !ok || queued {
		t.Fatalf("timeout command should have queued=false, got %v", responseData["queued"])
	}
	if responseData["error"] != ErrExtTimeout {
		t.Fatalf("timeout command should have error=%s, got %v", ErrExtTimeout, responseData["error"])
	}
}

func TestCommandResult_NotFoundHasFinalTrue(t *testing.T) {
	env := newInteractTestEnv(t)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"missing_corr_123"}`)
	resp := env.handler.toolObserveCommandResult(req, args)

	var observeResult MCPToolResult
	if err := json.Unmarshal(resp.Result, &observeResult); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}
	if !observeResult.IsError {
		t.Fatal("missing command should return isError=true")
	}

	var responseData map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(observeResult.Content[0].Text)), &responseData); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}
	if responseData["error"] != "no_data" {
		t.Fatalf("missing command should return error=no_data, got %v", responseData["error"])
	}
	if finalVal, ok := responseData["final"].(bool); !ok || !finalVal {
		t.Fatalf("missing command should have final=true, got %v", responseData["final"])
	}
}

func TestCommandResult_AnnotationNotFoundHasFinalTrue(t *testing.T) {
	env := newInteractTestEnv(t)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"ann_missing_123"}`)
	resp := env.handler.toolObserveCommandResult(req, args)

	var observeResult MCPToolResult
	if err := json.Unmarshal(resp.Result, &observeResult); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}
	if !observeResult.IsError {
		t.Fatal("missing annotation command should return isError=true")
	}

	var responseData map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(observeResult.Content[0].Text)), &responseData); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}
	if responseData["error"] != "no_data" {
		t.Fatalf("missing annotation command should return error=no_data, got %v", responseData["error"])
	}
	if finalVal, ok := responseData["final"].(bool); !ok || !finalVal {
		t.Fatalf("missing annotation command should have final=true, got %v", responseData["final"])
	}
}

func TestCommandResult_ExpiredIncludesDiagnosticHint(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, ok := env.callInteract(t, `{"what":"click","selector":"#btn","background":true}`)
	if !ok || result.IsError {
		t.Fatal("click should succeed")
	}

	pq := env.capture.GetLastPendingQuery()
	corrID := pq.CorrelationID
	env.capture.ExpireCommand(corrID)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.toolObserveCommandResult(req, args)

	var observeResult MCPToolResult
	if err := json.Unmarshal(resp.Result, &observeResult); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	text := observeResult.Content[0].Text

	// Diagnostic hint must include pilot and tracking state
	if !strings.Contains(text, "pilot=") {
		t.Errorf("Error response must include pilot status in diagnostic hint, got: %s", text)
	}
	if !strings.Contains(text, "tracked_tab=") {
		t.Errorf("Error response must include tracking status in diagnostic hint, got: %s", text)
	}
}

// ============================================
// Subtitle: correlation_id contract
// ============================================
// handleSubtitle() creates a PendingQuery with a correlationID but never
// returns it in the MCP response. This makes it impossible for callers to
// poll observe(command_result) for completion, causing race conditions in
// smoke tests (test 11.1: "Subtitle still visible after clear").

func TestSubtitle_SetResponse_HasCorrelationID(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)

	result, ok := env.callInteract(t, `{"what":"subtitle","text":"hello world"}`)
	if !ok {
		t.Fatal("subtitle set should return a result")
	}
	if result.IsError {
		t.Fatal("subtitle set should not be an error")
	}
	if len(result.Content) == 0 {
		t.Fatal("No content in result")
	}

	text := result.Content[0].Text
	if !strings.Contains(text, "correlation_id") {
		t.Errorf("Subtitle SET response must contain correlation_id so callers can poll for completion.\n"+
			"Without it, callers cannot wait for the extension to process the command.\n"+
			"Every other async handler (click, navigate, execute_js, highlight) returns correlation_id.\n"+
			"Got: %s", text)
	}
	if !strings.Contains(text, "subtitle_") {
		t.Errorf("Subtitle correlation_id should have subtitle_ prefix. Got: %s", text)
	}
}

func TestSubtitle_ClearResponse_HasCorrelationID(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)

	result, ok := env.callInteract(t, `{"what":"subtitle","text":""}`)
	if !ok {
		t.Fatal("subtitle clear should return a result")
	}
	if result.IsError {
		t.Fatal("subtitle clear should not be an error")
	}
	if len(result.Content) == 0 {
		t.Fatal("No content in result")
	}

	text := result.Content[0].Text
	if !strings.Contains(text, "correlation_id") {
		t.Errorf("Subtitle CLEAR response must contain correlation_id so callers can poll for completion.\n"+
			"Without it, the smoke test's interact_and_wait returns immediately and checks DOM\n"+
			"before the extension has processed the clear — causing 'still visible after clear'.\n"+
			"Got: %s", text)
	}
}

func TestSubtitle_CorrelationID_MatchesPendingQuery(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)

	result, ok := env.callInteract(t, `{"what":"subtitle","text":"test"}`)
	if !ok || result.IsError {
		t.Fatal("subtitle should succeed")
	}

	// The PendingQuery IS created with a correlationID
	pq := env.capture.GetLastPendingQuery()
	if pq == nil {
		t.Fatal("No pending query created for subtitle")
	}
	if pq.CorrelationID == "" {
		t.Fatal("PendingQuery has empty correlationID")
	}

	// But the MCP response must also contain it
	text := result.Content[0].Text
	if !strings.Contains(text, pq.CorrelationID) {
		t.Errorf("MCP response must contain the same correlation_id as the PendingQuery.\n"+
			"PendingQuery has: %s\n"+
			"Response text: %s", pq.CorrelationID, text)
	}
}

func TestCommandResult_PilotDisabledIncludesDiagnosticHint(t *testing.T) {
	env := newInteractTestEnv(t)
	// Pilot is disabled by default in test env

	result, ok := env.callInteract(t, `{"what":"click","selector":"#btn","background":true}`)
	if !ok {
		t.Fatal("should return result")
	}
	if !result.IsError {
		t.Fatal("pilot disabled should return error")
	}

	text := result.Content[0].Text
	if !strings.Contains(text, "pilot=DISABLED") {
		t.Errorf("pilot_disabled error must include 'pilot=DISABLED' in hint, got: %s", text)
	}
	if !strings.Contains(text, "tracked_tab=") {
		t.Errorf("pilot_disabled error must include tracking status in hint, got: %s", text)
	}
}
