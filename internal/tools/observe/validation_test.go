// Purpose: Tests for enum param validation in observe handlers.
// Why: Ensures invalid min_level, direction, classification, scope values return errors instead of silent empty results.

package observe

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/capture"
	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/mcp"
)

func newValidationDeps() *mockTransientDeps {
	return &mockTransientDeps{cap: capture.NewCapture()}
}

// ============================================
// min_level validation
// ============================================

func TestGetBrowserLogs_InvalidMinLevel_ReturnsError(t *testing.T) {
	t.Parallel()
	deps := newValidationDeps()
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args := json.RawMessage(`{"min_level":"critical"}`)

	resp := GetBrowserLogs(deps, req, args)
	result := extractMCPResult(t, resp)

	if !result.IsError {
		t.Fatal("expected error for invalid min_level")
	}
	if !strings.Contains(result.Content[0].Text, "invalid_param") {
		t.Errorf("expected invalid_param, got: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "min_level") {
		t.Errorf("error should mention min_level param, got: %s", result.Content[0].Text)
	}
}

func TestGetBrowserLogs_ValidMinLevels_NoError(t *testing.T) {
	t.Parallel()
	deps := newValidationDeps()
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}

	for _, level := range []string{"debug", "log", "info", "warn", "error", ""} {
		args, _ := json.Marshal(map[string]any{"min_level": level})
		resp := GetBrowserLogs(deps, req, args)
		result := extractMCPResult(t, resp)
		if result.IsError {
			t.Errorf("min_level=%q should not error, got: %s", level, result.Content[0].Text)
		}
	}
}

// ============================================
// direction validation
// ============================================

func TestGetWSEvents_InvalidDirection_ReturnsError(t *testing.T) {
	t.Parallel()
	deps := newValidationDeps()
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args := json.RawMessage(`{"direction":"both"}`)

	resp := GetWSEvents(deps, req, args)
	result := extractMCPResult(t, resp)

	if !result.IsError {
		t.Fatal("expected error for invalid direction")
	}
	if !strings.Contains(result.Content[0].Text, "invalid_param") {
		t.Errorf("expected invalid_param, got: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "direction") {
		t.Errorf("error should mention direction param, got: %s", result.Content[0].Text)
	}
}

func TestGetWSEvents_ValidDirections_NoError(t *testing.T) {
	t.Parallel()
	deps := newValidationDeps()
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}

	for _, dir := range []string{"incoming", "outgoing", ""} {
		args, _ := json.Marshal(map[string]any{"direction": dir})
		resp := GetWSEvents(deps, req, args)
		result := extractMCPResult(t, resp)
		if result.IsError {
			t.Errorf("direction=%q should not error, got: %s", dir, result.Content[0].Text)
		}
	}
}

// ============================================
// classification validation
// ============================================

func TestGetTransients_InvalidClassification_ReturnsError(t *testing.T) {
	t.Parallel()
	deps := newValidationDeps()
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args := json.RawMessage(`{"classification":"popup"}`)

	resp := GetTransients(deps, req, args)
	result := extractMCPResult(t, resp)

	if !result.IsError {
		t.Fatal("expected error for invalid classification")
	}
	if !strings.Contains(result.Content[0].Text, "invalid_param") {
		t.Errorf("expected invalid_param, got: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "classification") {
		t.Errorf("error should mention classification param, got: %s", result.Content[0].Text)
	}
}

func TestGetTransients_ValidClassifications_NoError(t *testing.T) {
	t.Parallel()
	deps := newValidationDeps()
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}

	for _, cls := range []string{"alert", "toast", "snackbar", "notification", "tooltip", "banner", "flash", ""} {
		args, _ := json.Marshal(map[string]any{"classification": cls})
		resp := GetTransients(deps, req, args)
		result := extractMCPResult(t, resp)
		if result.IsError {
			t.Errorf("classification=%q should not error, got: %s", cls, result.Content[0].Text)
		}
	}
}

// ============================================
// error_bundles scope validation
// ============================================

func TestGetErrorBundles_InvalidScope_ReturnsError(t *testing.T) {
	t.Parallel()
	deps := newValidationDeps()
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args := json.RawMessage(`{"scope":"bogus"}`)

	resp := GetErrorBundles(deps, req, args)
	result := extractMCPResult(t, resp)

	if !result.IsError {
		t.Fatal("expected error for invalid scope in error_bundles")
	}
	if !strings.Contains(result.Content[0].Text, "invalid_param") {
		t.Errorf("expected invalid_param, got: %s", result.Content[0].Text)
	}
}

func TestGetErrorBundles_ValidScopes_NoError(t *testing.T) {
	t.Parallel()
	deps := newValidationDeps()
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}

	for _, scope := range []string{"current_page", "all", ""} {
		args, _ := json.Marshal(map[string]any{"scope": scope})
		resp := GetErrorBundles(deps, req, args)
		result := extractMCPResult(t, resp)
		if result.IsError {
			t.Errorf("scope=%q should not error, got: %s", scope, result.Content[0].Text)
		}
	}
}

// extractMCPResult parses the MCPToolResult from a response.
func extractMCPResult(t *testing.T, resp mcp.JSONRPCResponse) mcp.MCPToolResult {
	t.Helper()
	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	return result
}
