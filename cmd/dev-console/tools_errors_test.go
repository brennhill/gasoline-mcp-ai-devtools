// Purpose: Tests for tool error response formatting.
// Why: Prevents silent regressions in critical behavior paths.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// tools_errors_test.go — Tests for structured error retryable field and retry_after_ms.
package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// ============================================
// Retryable Error Field Tests
// ============================================

func TestStructuredError_RetryableErrors_SerializeCorrectly(t *testing.T) {
	t.Parallel()

	result := mcpStructuredError(
		ErrExtTimeout, "Extension timed out", "Retry the command",
		withRetryable(true), withRetryAfterMs(1000),
	)

	se := extractStructuredErrorJSON(t, result)

	retryable, ok := se["retryable"].(bool)
	if !ok {
		t.Fatal("retryable field missing or not a bool")
	}
	if !retryable {
		t.Error("retryable should be true for ErrExtTimeout")
	}

	retryAfterMs, ok := se["retry_after_ms"].(float64)
	if !ok {
		t.Fatal("retry_after_ms field missing or not a number")
	}
	if retryAfterMs != 1000 {
		t.Errorf("retry_after_ms = %v, want 1000", retryAfterMs)
	}
}

func TestStructuredError_NonRetryableErrors_OmitRetryAfterMs(t *testing.T) {
	t.Parallel()

	result := mcpStructuredError(
		ErrInvalidParam, "Bad parameter", "Fix the parameter",
		withRetryable(false),
	)

	se := extractStructuredErrorJSON(t, result)

	retryable, ok := se["retryable"].(bool)
	if !ok {
		t.Fatal("retryable field missing or not a bool")
	}
	if retryable {
		t.Error("retryable should be false for ErrInvalidParam")
	}

	if _, exists := se["retry_after_ms"]; exists {
		t.Error("retry_after_ms should not be present for non-retryable errors")
	}
}

func TestStructuredError_DefaultRetryable_IsFalse(t *testing.T) {
	t.Parallel()

	// No withRetryable option — should default to false
	result := mcpStructuredError(
		ErrInternal, "Internal error", "Do not retry",
	)

	se := extractStructuredErrorJSON(t, result)

	// retryable should still be present (zero value = false)
	retryable, ok := se["retryable"].(bool)
	if !ok {
		t.Fatal("retryable field missing or not a bool")
	}
	if retryable {
		t.Error("retryable should default to false")
	}
}

func TestStructuredError_CanonicalRecoveryContractFields(t *testing.T) {
	t.Parallel()

	result := mcpStructuredError(
		ErrMissingParam, "Missing parameter", "Call interact with what=list_interactive",
	)

	se := extractStructuredErrorJSON(t, result)
	if se["error_code"] != ErrMissingParam {
		t.Fatalf("error_code = %v, want %q", se["error_code"], ErrMissingParam)
	}
	if se["recovery_playbook"] != "Call interact with what=list_interactive" {
		t.Fatalf("recovery_playbook = %v", se["recovery_playbook"])
	}
	if _, exists := se["error"]; exists {
		t.Fatalf("legacy field error should not be present: %v", se["error"])
	}
	if _, exists := se["retry"]; exists {
		t.Fatalf("legacy field retry should not be present: %v", se["retry"])
	}
}

func TestStructuredError_ErrorCodes_RetryableDefaults(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		code      string
		retryable bool
		retryMs   int
	}{
		{ErrExtTimeout, true, 1000},
		{ErrExtError, true, 2000},
		{ErrRateLimited, true, 1000},
		{ErrInvalidParam, false, 0},
		{ErrMissingParam, false, 0},
		{ErrInternal, false, 0},
		{ErrUnknownMode, false, 0},
		{ErrNoData, true, 2000},
	}

	for _, tc := range testCases {
		t.Run(tc.code, func(t *testing.T) {
			opts := retryDefaultsForCode(tc.code)
			result := mcpStructuredError(tc.code, "test", "test", opts...)

			se := extractStructuredErrorJSON(t, result)

			retryable, _ := se["retryable"].(bool)
			if retryable != tc.retryable {
				t.Errorf("code %s: retryable = %v, want %v", tc.code, retryable, tc.retryable)
			}

			if tc.retryMs > 0 {
				retryAfterMs, ok := se["retry_after_ms"].(float64)
				if !ok {
					t.Errorf("code %s: retry_after_ms missing", tc.code)
				} else if int(retryAfterMs) != tc.retryMs {
					t.Errorf("code %s: retry_after_ms = %v, want %v", tc.code, retryAfterMs, tc.retryMs)
				}
			} else {
				if _, exists := se["retry_after_ms"]; exists {
					t.Errorf("code %s: retry_after_ms should not be present", tc.code)
				}
			}
		})
	}
}

// ============================================
// Action/Selector Context Tests
// ============================================

func TestStructuredError_ActionAndSelector_OmittedWhenEmpty(t *testing.T) {
	t.Parallel()

	result := mcpStructuredError(
		ErrExtTimeout, "Extension timed out", "Retry the command",
	)

	se := extractStructuredErrorJSON(t, result)

	if _, exists := se["action"]; exists {
		t.Error("action should be omitted when not set")
	}
	if _, exists := se["selector"]; exists {
		t.Error("selector should be omitted when not set")
	}
}

func TestStructuredError_ActionAndSelector_PresentWhenSet(t *testing.T) {
	t.Parallel()

	result := mcpStructuredError(
		ErrNoData, "Extension not connected", "Check extension",
		withAction("click"), withSelector("#submit-btn"),
	)

	se := extractStructuredErrorJSON(t, result)

	action, ok := se["action"].(string)
	if !ok || action != "click" {
		t.Errorf("action = %v, want 'click'", se["action"])
	}

	selector, ok := se["selector"].(string)
	if !ok || selector != "#submit-btn" {
		t.Errorf("selector = %v, want '#submit-btn'", se["selector"])
	}
}

// ============================================
// Smoke Tests: Stream 4 — Diagnostic Hints in Gate Errors
// ============================================

func TestSmoke_RequireExtension_ErrorContainsDiagnosticHint(t *testing.T) {
	t.Parallel()
	env := newGateTestEnv(t)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp, blocked := env.handler.requireExtension(req)
	if !blocked {
		t.Fatal("expected requireExtension to block when extension is disconnected")
	}

	se := extractStructuredError(t, resp)
	if se.Hint == "" {
		t.Fatal("StructuredError.Hint should not be empty for extension gate error")
	}
	for _, expected := range []string{"extension=DISCONNECTED", "pilot=", "tracked_tab=", "csp="} {
		if !strings.Contains(se.Hint, expected) {
			t.Errorf("hint should contain %q, got: %s", expected, se.Hint)
		}
	}
}

func TestSmoke_RequirePilot_ErrorContainsDiagnosticHint(t *testing.T) {
	t.Parallel()
	env := newGateTestEnv(t)
	env.capture.SetPilotEnabled(false)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp, blocked := env.handler.requirePilot(req)
	if !blocked {
		t.Fatal("expected requirePilot to block when pilot is disabled")
	}

	se := extractStructuredError(t, resp)
	if se.Hint == "" {
		t.Fatal("StructuredError.Hint should not be empty for pilot gate error")
	}
	if !strings.Contains(se.Hint, "pilot=DISABLED") {
		t.Errorf("hint should contain 'pilot=DISABLED', got: %s", se.Hint)
	}
}

func TestSmoke_RequireTabTracking_ErrorContainsDiagnosticHint(t *testing.T) {
	t.Parallel()
	env := newGateTestEnv(t)
	// No tab tracking set

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp, blocked := env.handler.requireTabTracking(req)
	if !blocked {
		t.Fatal("expected requireTabTracking to block when no tab is tracked")
	}

	se := extractStructuredError(t, resp)
	if se.Hint == "" {
		t.Fatal("StructuredError.Hint should not be empty for tab tracking gate error")
	}
	if !strings.Contains(se.Hint, "tracked_tab=NONE") {
		t.Errorf("hint should contain 'tracked_tab=NONE', got: %s", se.Hint)
	}
}

func TestSmoke_RequireCSPClear_ErrorContainsDiagnosticHint(t *testing.T) {
	t.Parallel()
	env := newGateTestEnv(t)
	env.capture.SetCSPStatusForTest(true, "script_exec")

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp, blocked := env.handler.requireCSPClear(req, "main")
	if !blocked {
		t.Fatal("expected requireCSPClear to block world=main when CSP restricts script_exec")
	}

	se := extractStructuredError(t, resp)
	if se.Hint == "" {
		t.Fatal("StructuredError.Hint should not be empty for CSP gate error")
	}
	if !strings.Contains(se.Hint, "csp=RESTRICTED(script_exec)") {
		t.Errorf("hint should contain 'csp=RESTRICTED(script_exec)', got: %s", se.Hint)
	}
}

// Helpers: extractStructuredErrorJSON and extractJSONFromText are in tools_test_helpers_test.go.
