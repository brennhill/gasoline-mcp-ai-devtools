// tools_interact_workflows_test.go — Tests for high-level workflow primitives.
// Pure function tests (isErrorResponse, responseStatus, workflowResult) live in
// internal/tools/interact/workflow_test.go.
package main

import (
	"encoding/json"
	"testing"
)

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


