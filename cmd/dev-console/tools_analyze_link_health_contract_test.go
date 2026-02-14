// tools_analyze_link_health_contract_test.go — Response shape contracts for analyze/link_health.
// Each test verifies that link_health mode returns the correct JSON fields with correct types.
// Catches field renames, missing fields, and type changes.
//
// Run: go test ./cmd/dev-console -run "TestAnalyzeLinkHealthContract" -v
package main

import (
	"testing"
)

// ============================================
// Link Health Response Contracts
// ============================================

// TestAnalyzeLinkHealthContract_StartResponse verifies link_health start response shape
func TestAnalyzeLinkHealthContract_StartResponse(t *testing.T) {
	env := newAnalyzeTestEnv(t)

	result, ok := env.callAnalyze(t, `{"what":"link_health"}`)
	if !ok {
		t.Fatal("link_health: no result")
	}

	// Response should not be an error
	if result.IsError {
		t.Errorf("link_health should not error on valid call")
		return
	}

	// Parse the JSON response to check field structure
	data := parseResponseJSON(t, result)

	// Verify required fields in response
	assertObjectShape(t, "link_health_start", data, []fieldSpec{
		required("status", "string"),
		required("correlation_id", "string"),
		optional("hint", "string"),
	})

	// Verify correlation_id format (should be non-empty string)
	correlationID, ok := data["correlation_id"].(string)
	if !ok {
		t.Error("link_health: correlation_id should be string")
	}
	if correlationID == "" {
		t.Error("link_health: correlation_id should not be empty")
	}

	// Verify status value
	status, ok := data["status"].(string)
	if !ok {
		t.Error("link_health: status should be string")
		return
	}
	if status != "queued" && status != "initiated" {
		t.Errorf("link_health: status should be 'queued' or 'initiated', got '%s'", status)
	}
}

// TestAnalyzeLinkHealthContract_ErrorResponse verifies error response shape
func TestAnalyzeLinkHealthContract_ErrorResponse(t *testing.T) {
	env := newAnalyzeTestEnv(t)

	// Test with invalid mode to trigger error response
	result, ok := env.callAnalyze(t, `{"what":"invalid_mode"}`)
	if !ok {
		t.Fatal("analyze with invalid mode: no result")
	}

	// Error response should be structured
	if result.IsError {
		// Errors should have proper structure (text content)
		if len(result.Content) == 0 {
			t.Fatal("error result should have content")
		}
		if result.Content[0].Type != "text" {
			t.Errorf("error content should be text, got %s", result.Content[0].Type)
		}
	}
}

// TestAnalyzeLinkHealthContract_WithParams verifies response structure with optional parameters
func TestAnalyzeLinkHealthContract_WithParams(t *testing.T) {
	env := newAnalyzeTestEnv(t)

	result, ok := env.callAnalyze(t, `{"what":"link_health","timeout_ms":15000,"max_workers":20}`)
	if !ok {
		t.Fatal("link_health with params: no result")
	}

	if result.IsError {
		t.Errorf("link_health should accept optional params: %s", result.Content[0].Text)
		return
	}

	// Response shape should be same regardless of optional params
	data := parseResponseJSON(t, result)
	assertObjectShape(t, "link_health_with_params", data, []fieldSpec{
		required("status", "string"),
		required("correlation_id", "string"),
	})
}

// ============================================
// Dispatcher Response Contracts
// ============================================

// TestAnalyzeContract_MissingWhat verifies error response for missing 'what'
func TestAnalyzeContract_MissingWhat(t *testing.T) {
	env := newAnalyzeTestEnv(t)

	result, ok := env.callAnalyze(t, `{}`)
	if !ok {
		t.Fatal("analyze: no result")
	}

	assertStructuredError(t, "analyze (missing what)", result)
}

// TestAnalyzeContract_InvalidMode verifies error response for invalid mode
func TestAnalyzeContract_InvalidMode(t *testing.T) {
	env := newAnalyzeTestEnv(t)

	result, ok := env.callAnalyze(t, `{"what":"nonexistent_mode_xyz"}`)
	if !ok {
		t.Fatal("analyze: no result")
	}

	assertStructuredError(t, "analyze (invalid mode)", result)
}

// TestAnalyzeContract_InvalidJSON verifies error response for invalid JSON
func TestAnalyzeContract_InvalidJSON(t *testing.T) {
	env := newAnalyzeTestEnv(t)

	result, ok := env.callAnalyze(t, `{not valid json}`)
	if !ok {
		t.Fatal("analyze: no result")
	}

	assertStructuredError(t, "analyze (invalid json)", result)
}

// ============================================
// Mode Integration Contracts
// ============================================

// TestAnalyzeContract_DomMode verifies dom mode still works after migration to analyze
func TestAnalyzeContract_DomMode(t *testing.T) {
	env := newAnalyzeTestEnv(t)

	result, ok := env.callAnalyze(t, `{"what":"dom","selector":"body"}`)
	if !ok {
		t.Fatal("analyze dom: no result")
	}

	// Should either return valid result (if extension connected) or clear error
	// We don't assert success here since extension may not be running in tests
	// but we verify the response is properly structured
	if len(result.Content) == 0 {
		t.Fatal("analyze dom should return content")
	}
}

// TestAnalyzeContract_ApiValidationMode verifies api_validation mode still works
func TestAnalyzeContract_ApiValidationMode(t *testing.T) {
	env := newAnalyzeTestEnv(t)

	result, ok := env.callAnalyze(t, `{"what":"api_validation"}`)
	if !ok {
		t.Fatal("analyze api_validation: no result")
	}

	// Should return properly structured response
	if len(result.Content) == 0 {
		t.Fatal("analyze api_validation should return content")
	}
}

