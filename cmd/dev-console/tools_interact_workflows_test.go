// tools_interact_workflows_test.go — Tests for high-level workflow primitives.
package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// ============================================
// Helper response tests
// ============================================

func TestIsErrorResponse_NilError(t *testing.T) {
	t.Parallel()
	resp := JSONRPCResponse{JSONRPC: "2.0"}
	// No error field, no result — should not panic
	if isErrorResponse(resp) {
		t.Error("expected non-error for empty response with no error field")
	}
}

func TestIsErrorResponse_WithJSONRPCError(t *testing.T) {
	t.Parallel()
	resp := JSONRPCResponse{JSONRPC: "2.0", Error: &JSONRPCError{Code: -32600, Message: "bad"}}
	if !isErrorResponse(resp) {
		t.Error("expected error response when Error field is set")
	}
}

func TestIsErrorResponse_WithMCPToolError(t *testing.T) {
	t.Parallel()
	result := MCPToolResult{
		Content: []MCPContentBlock{{Type: "text", Text: "fail"}},
		IsError: true,
	}
	raw, _ := json.Marshal(result)
	resp := JSONRPCResponse{JSONRPC: "2.0", Result: raw}
	if !isErrorResponse(resp) {
		t.Error("expected error when isError=true in MCPToolResult")
	}
}

func TestResponseStatus(t *testing.T) {
	t.Parallel()
	ok := JSONRPCResponse{JSONRPC: "2.0"}
	if responseStatus(ok) != "success" {
		t.Error("expected success for non-error response")
	}

	err := JSONRPCResponse{JSONRPC: "2.0", Error: &JSONRPCError{Code: -1, Message: "x"}}
	if responseStatus(err) != "error" {
		t.Error("expected error for error response")
	}
}

// ============================================
// navigate_and_wait_for validation tests
// ============================================

func TestNavigateAndWaitFor_MissingURL(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()
	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args, _ := json.Marshal(map[string]any{
		"wait_for": ".content",
	})
	resp := h.handleNavigateAndWaitFor(req, args)
	assertIsError(t, resp, "url")
}

func TestNavigateAndWaitFor_MissingWaitFor(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()
	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args, _ := json.Marshal(map[string]any{
		"url": "https://example.com",
	})
	resp := h.handleNavigateAndWaitFor(req, args)
	assertIsError(t, resp, "wait_for")
}

func TestNavigateAndWaitFor_InvalidJSON(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()
	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp := h.handleNavigateAndWaitFor(req, json.RawMessage(`{bad`))
	assertIsError(t, resp, "JSON")
}

// ============================================
// fill_form_and_submit validation tests
// ============================================

func TestFillFormAndSubmit_EmptyFields(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()
	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args, _ := json.Marshal(map[string]any{
		"fields":          []any{},
		"submit_selector": "button[type=submit]",
	})
	resp := h.handleFillFormAndSubmit(req, args)
	assertIsError(t, resp, "fields")
}

func TestFillFormAndSubmit_MissingSubmit(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()
	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args, _ := json.Marshal(map[string]any{
		"fields": []map[string]string{
			{"selector": "#email", "value": "test@example.com"},
		},
	})
	resp := h.handleFillFormAndSubmit(req, args)
	assertIsError(t, resp, "submit_selector")
}

func TestFillFormAndSubmit_FieldMissingSelectorAndIndex(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()
	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args, _ := json.Marshal(map[string]any{
		"fields": []map[string]string{
			{"value": "test@example.com"},
		},
		"submit_selector": "button",
	})
	resp := h.handleFillFormAndSubmit(req, args)
	assertIsError(t, resp, "selector")
}

func TestFillFormAndSubmit_InvalidJSON(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()
	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp := h.handleFillFormAndSubmit(req, json.RawMessage(`{bad`))
	assertIsError(t, resp, "JSON")
}

func TestRunA11yAndExportSARIF_InvalidJSON(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()
	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp := h.handleRunA11yAndExportSARIF(req, json.RawMessage(`{bad`))
	assertIsError(t, resp, "JSON")
}

// ============================================
// run_a11y_and_export_sarif tests
// ============================================

func TestRunA11yAndExportSARIF_ValidParams(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()
	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args, _ := json.Marshal(map[string]any{
		"scope": "page",
	})
	// This will fail due to no extension connected, but should not panic
	// and should return a structured error/workflow result
	resp := h.handleRunA11yAndExportSARIF(req, args)
	if resp.JSONRPC != "2.0" {
		t.Error("expected valid JSON-RPC response")
	}
	// The workflow should have a trace in the result
	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}
}

// ============================================
// workflowResult tests
// ============================================

func TestWorkflowResult_AllSuccess(t *testing.T) {
	t.Parallel()
	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	trace := []WorkflowStep{
		{Action: "step1", Status: "success", TimingMs: 10},
		{Action: "step2", Status: "success", TimingMs: 20},
	}
	okResp := JSONRPCResponse{JSONRPC: "2.0"}
	resp := workflowResult(req, "test_workflow", trace, okResp, time.Now())

	// mcpJSONResponse puts data as JSON text in Content[0].Text
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal MCPToolResult: %v", err)
	}
	if result.IsError {
		t.Fatal("expected non-error result")
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content blocks")
	}
	text := result.Content[0].Text
	if !strings.Contains(text, `"status":"success"`) {
		t.Errorf("expected status=success in text, got: %s", text)
	}
	if !strings.Contains(text, `"successful":2`) {
		t.Errorf("expected successful=2 in text, got: %s", text)
	}
	if !strings.Contains(text, "test_workflow completed") {
		t.Errorf("expected summary line, got: %s", text)
	}
}

func TestWorkflowResult_Failure(t *testing.T) {
	t.Parallel()
	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	trace := []WorkflowStep{
		{Action: "step1", Status: "success", TimingMs: 10},
		{Action: "step2", Status: "error", TimingMs: 5},
	}
	errResp := JSONRPCResponse{JSONRPC: "2.0", Error: &JSONRPCError{Code: -1, Message: "fail"}}
	resp := workflowResult(req, "test_workflow", trace, errResp, time.Now())

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !result.IsError {
		t.Error("expected isError=true for failed workflow")
	}
	text := result.Content[0].Text
	if !strings.Contains(text, `"status":"failed"`) {
		t.Errorf("expected status=failed, got: %s", text)
	}
	if !strings.Contains(text, `"successful":1`) {
		t.Errorf("expected successful=1, got: %s", text)
	}
	if !strings.Contains(text, `"error_detail"`) {
		t.Errorf("expected error_detail in failed workflow, got: %s", text)
	}
}

func TestWorkflowResult_FailureWithMCPError(t *testing.T) {
	t.Parallel()
	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	trace := []WorkflowStep{
		{Action: "step1", Status: "error", TimingMs: 5},
	}

	// MCP-level error (isError in result)
	mcpErr := MCPToolResult{
		Content: []MCPContentBlock{{Type: "text", Text: "pilot disabled"}},
		IsError: true,
	}
	raw, _ := json.Marshal(mcpErr)
	errResp := JSONRPCResponse{JSONRPC: "2.0", Result: raw}
	resp := workflowResult(req, "test_workflow", trace, errResp, time.Now())

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !result.IsError {
		t.Error("expected isError=true")
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "pilot disabled") {
		t.Errorf("expected error detail from MCP error, got: %s", text)
	}
}

