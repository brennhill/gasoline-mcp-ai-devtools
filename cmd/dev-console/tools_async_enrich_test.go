// tools_async_enrich_test.go — Tests for enrichCommandResponseData tab deduplication and slim response output.
package main

import (
	"encoding/json"
	"testing"

	"github.com/dev-console/dev-console/internal/queries"
)

func TestEnrichCommandResponseData_SameURLOmitsRedundantFields(t *testing.T) {
	t.Parallel()

	result := json.RawMessage(`{
		"success": true,
		"resolved_tab_id": 42,
		"resolved_url": "https://example.com/page",
		"effective_tab_id": 42,
		"effective_url": "https://example.com/page",
		"effective_title": "Example Page",
		"final_url": "https://example.com/page",
		"title": "Example Page",
		"timing": {"duration_ms": 50}
	}`)

	responseData := map[string]any{}
	embeddedErr, hasErr := enrichCommandResponseData(result, responseData)
	if hasErr {
		t.Fatalf("unexpected embedded error: %s", embeddedErr)
	}

	// When URLs match, resolved_* and final_url/title should be omitted
	if _, ok := responseData["resolved_tab_id"]; ok {
		t.Error("resolved_tab_id should be omitted when URLs match")
	}
	if _, ok := responseData["resolved_url"]; ok {
		t.Error("resolved_url should be omitted when URLs match")
	}
	if _, ok := responseData["final_url"]; ok {
		t.Error("final_url should be omitted when same as effective_url")
	}
	if _, ok := responseData["title"]; ok {
		t.Error("title should be omitted when same as effective_title")
	}

	// effective_* should still be present
	if responseData["effective_url"] != "https://example.com/page" {
		t.Errorf("effective_url should be present, got %v", responseData["effective_url"])
	}
	if responseData["effective_title"] != "Example Page" {
		t.Errorf("effective_title should be present, got %v", responseData["effective_title"])
	}

	// tab_changed should not be present
	if _, ok := responseData["tab_changed"]; ok {
		t.Error("tab_changed should not be present when URLs match")
	}
	// navigation_detected should not be present
	if _, ok := responseData["navigation_detected"]; ok {
		t.Error("navigation_detected should not be present when URLs match")
	}
}

func TestEnrichCommandResponseData_DifferentURLIncludesNavFields(t *testing.T) {
	t.Parallel()

	result := json.RawMessage(`{
		"success": true,
		"resolved_tab_id": 42,
		"resolved_url": "https://example.com/old",
		"effective_tab_id": 42,
		"effective_url": "https://example.com/new",
		"effective_title": "New Page",
		"final_url": "https://example.com/new",
		"title": "New Page",
		"timing": {"duration_ms": 50}
	}`)

	responseData := map[string]any{}
	embeddedErr, hasErr := enrichCommandResponseData(result, responseData)
	if hasErr {
		t.Fatalf("unexpected embedded error: %s", embeddedErr)
	}

	// When URLs differ, tab_changed and navigation_detected should be true
	if responseData["tab_changed"] != true {
		t.Error("tab_changed should be true when URLs differ")
	}
	if responseData["navigation_detected"] != true {
		t.Error("navigation_detected should be true when URLs differ")
	}

	// navigation_note should explain the change
	note, ok := responseData["navigation_note"].(string)
	if !ok || note == "" {
		t.Error("navigation_note should be present when URLs differ")
	}

	// All URL fields should be present when they differ
	if responseData["resolved_url"] != "https://example.com/old" {
		t.Errorf("resolved_url should be present when URLs differ, got %v", responseData["resolved_url"])
	}
	if responseData["effective_url"] != "https://example.com/new" {
		t.Errorf("effective_url should be present, got %v", responseData["effective_url"])
	}
}

func TestEnrichCommandResponseData_NoTabFields(t *testing.T) {
	t.Parallel()

	result := json.RawMessage(`{
		"success": true,
		"timing": {"duration_ms": 50}
	}`)

	responseData := map[string]any{}
	embeddedErr, hasErr := enrichCommandResponseData(result, responseData)
	if hasErr {
		t.Fatalf("unexpected embedded error: %s", embeddedErr)
	}

	// No tab-related fields should be present
	if _, ok := responseData["tab_changed"]; ok {
		t.Error("tab_changed should not be present when no tab fields")
	}
	if _, ok := responseData["navigation_detected"]; ok {
		t.Error("navigation_detected should not be present when no tab fields")
	}

	// timing should still come through
	if responseData["timing"] == nil {
		t.Error("timing should still be surfaced")
	}
}

func TestEnrichCommandResponseData_ErrorStillSurfaced(t *testing.T) {
	t.Parallel()

	result := json.RawMessage(`{
		"success": false,
		"error": "element not found",
		"resolved_url": "https://example.com",
		"effective_url": "https://example.com"
	}`)

	responseData := map[string]any{}
	embeddedErr, hasErr := enrichCommandResponseData(result, responseData)
	if !hasErr {
		t.Fatal("expected embedded error")
	}
	if embeddedErr != "element not found" {
		t.Errorf("expected 'element not found', got %q", embeddedErr)
	}
}

func TestEnrichCommandResponseData_ReturnValueSurfaced(t *testing.T) {
	t.Parallel()

	result := json.RawMessage(`{
		"success": true,
		"result": {"links": ["https://a.com", "https://b.com"]},
		"effective_url": "https://example.com"
	}`)

	responseData := map[string]any{}
	embeddedErr, hasErr := enrichCommandResponseData(result, responseData, "exec_test_123")
	if hasErr {
		t.Fatalf("unexpected embedded error: %s", embeddedErr)
	}

	// return_value should be surfaced at top level
	rv, ok := responseData["return_value"]
	if !ok {
		t.Fatal("return_value should be surfaced at top level for execute_js results")
	}
	rvMap, ok := rv.(map[string]any)
	if !ok {
		t.Fatalf("return_value should be a map, got %T", rv)
	}
	links, ok := rvMap["links"].([]any)
	if !ok || len(links) != 2 {
		t.Errorf("return_value.links should have 2 items, got %v", rvMap["links"])
	}
}

func TestEnrichCommandResponseData_ReturnValueNil(t *testing.T) {
	t.Parallel()

	result := json.RawMessage(`{
		"success": true,
		"result": null,
		"effective_url": "https://example.com"
	}`)

	responseData := map[string]any{}
	enrichCommandResponseData(result, responseData, "exec_test_456")

	// return_value should still be surfaced even when null
	if _, ok := responseData["return_value"]; !ok {
		t.Error("return_value should be surfaced even when null")
	}
}

func TestEnrichCommandResponseData_NoResultField(t *testing.T) {
	t.Parallel()

	result := json.RawMessage(`{
		"success": true,
		"action": "click",
		"selector": "#btn"
	}`)

	responseData := map[string]any{}
	enrichCommandResponseData(result, responseData)

	// return_value should NOT be surfaced when there's no "result" field in extension response
	if _, ok := responseData["return_value"]; ok {
		t.Error("return_value should not be surfaced when no result field in extension response")
	}
}

// ============================================
// stripEnrichedFieldsFromResult tests
// ============================================

func TestStripEnrichedFieldsFromResult_RemovesDuplicates(t *testing.T) {
	t.Parallel()

	responseData := map[string]any{
		"effective_url":   "https://example.com",
		"effective_title": "Example",
		"match_count":     float64(1),
		"match_strategy":  "selector",
		"result": json.RawMessage(`{
			"success": true,
			"action": "click",
			"selector": "[data-testid='btn']",
			"timing": {"total_ms": 223},
			"effective_url": "https://example.com",
			"effective_title": "Example",
			"match_count": 1,
			"match_strategy": "selector",
			"target_context": {"source": "tracked_tab"},
			"content_script_status": "loaded"
		}`),
	}

	stripEnrichedFieldsFromResult(responseData)

	var resultMap map[string]any
	if err := json.Unmarshal(responseData["result"].(json.RawMessage), &resultMap); err != nil {
		t.Fatalf("failed to unmarshal stripped result: %v", err)
	}

	// Non-enriched fields must survive
	if resultMap["success"] != true {
		t.Error("success should be preserved in result")
	}
	if resultMap["action"] != "click" {
		t.Error("action should be preserved in result")
	}
	if resultMap["selector"] != "[data-testid='btn']" {
		t.Error("selector should be preserved in result")
	}

	// Enriched fields must be stripped
	for _, key := range []string{"timing", "effective_url", "effective_title", "match_count", "match_strategy", "target_context", "content_script_status"} {
		if _, ok := resultMap[key]; ok {
			t.Errorf("%s should be stripped from result after enrichment", key)
		}
	}
}

func TestStripEnrichedFieldsFromResult_NoResultField(t *testing.T) {
	t.Parallel()
	responseData := map[string]any{"status": "complete"}
	// Should not panic
	stripEnrichedFieldsFromResult(responseData)
}

func TestStripEnrichedFieldsFromResult_EmptyResult(t *testing.T) {
	t.Parallel()
	responseData := map[string]any{"result": json.RawMessage(`{}`)}
	stripEnrichedFieldsFromResult(responseData)

	var resultMap map[string]any
	if err := json.Unmarshal(responseData["result"].(json.RawMessage), &resultMap); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if len(resultMap) != 0 {
		t.Errorf("expected empty result map, got %v", resultMap)
	}
}

func TestStripEnrichedFieldsFromResult_InvalidJSON(t *testing.T) {
	t.Parallel()
	responseData := map[string]any{"result": json.RawMessage(`not json`)}
	// Should not panic, should leave result unchanged
	stripEnrichedFieldsFromResult(responseData)
	if string(responseData["result"].(json.RawMessage)) != "not json" {
		t.Error("invalid JSON result should be left unchanged")
	}
}

// ============================================
// attachTraceSummary slim output tests
// ============================================

func TestAttachTraceSummary_OmitsEvents(t *testing.T) {
	t.Parallel()

	cmd := queries.CommandResult{
		TraceID:       "dom_click_123",
		TraceTimeline: "queued -> sent -> started -> resolved",
		TraceEvents: []queries.CommandTraceEvent{
			{Stage: "queued"},
			{Stage: "sent"},
			{Stage: "started"},
			{Stage: "resolved"},
		},
		QueryID: "q-22",
	}

	responseData := map[string]any{}
	attachTraceSummary(responseData, cmd)

	trace, ok := responseData["trace"].(map[string]any)
	if !ok {
		t.Fatal("trace should be present")
	}

	if trace["trace_id"] != "dom_click_123" {
		t.Errorf("expected trace_id dom_click_123, got %v", trace["trace_id"])
	}
	if trace["timeline"] != "queued -> sent -> started -> resolved" {
		t.Errorf("unexpected timeline: %v", trace["timeline"])
	}
	if trace["query_id"] != "q-22" {
		t.Errorf("expected query_id q-22, got %v", trace["query_id"])
	}
	if trace["last_stage"] != "resolved" {
		t.Errorf("expected last_stage resolved, got %v", trace["last_stage"])
	}

	// events must NOT be present (token savings)
	if _, ok := trace["events"]; ok {
		t.Error("trace.events should be omitted for token efficiency")
	}
}

// ============================================
// stripSuccessOnlyFields tests
// ============================================

func TestStripSuccessOnlyFields_RemovesRoutingFields(t *testing.T) {
	t.Parallel()

	responseData := map[string]any{
		"status":                "complete",
		"target_context":       map[string]any{"source": "tracked_tab"},
		"content_script_status": "loaded",
		"created_at":           "2026-02-26T00:00:00Z",
		"trace_id":             "dom_click_123",
		"effective_url":        "https://example.com",
	}

	stripSuccessOnlyFields(responseData)

	for _, key := range []string{"target_context", "content_script_status", "created_at", "trace_id"} {
		if _, ok := responseData[key]; ok {
			t.Errorf("%s should be stripped from successful response", key)
		}
	}

	// effective_url should survive
	if responseData["effective_url"] != "https://example.com" {
		t.Error("effective_url should not be stripped")
	}
}

func TestStripRetryContextOnSuccess_RemovesAttempt1_Int(t *testing.T) {
	t.Parallel()

	responseData := map[string]any{
		"retry_context": map[string]any{
			"attempt":      1, // int from attachRetryContext
			"max_attempts": 2,
			"reason":       "success",
		},
	}

	stripRetryContextOnSuccess(responseData)

	if _, ok := responseData["retry_context"]; ok {
		t.Error("retry_context should be stripped when attempt=1 on success (int)")
	}
}

func TestStripRetryContextOnSuccess_RemovesAttempt1_Float(t *testing.T) {
	t.Parallel()

	responseData := map[string]any{
		"retry_context": map[string]any{
			"attempt":      float64(1), // float64 from JSON unmarshal
			"max_attempts": float64(2),
			"reason":       "success",
		},
	}

	stripRetryContextOnSuccess(responseData)

	if _, ok := responseData["retry_context"]; ok {
		t.Error("retry_context should be stripped when attempt=1 on success (float64)")
	}
}

func TestStripRetryContextOnSuccess_KeepsAttempt2(t *testing.T) {
	t.Parallel()

	responseData := map[string]any{
		"retry_context": map[string]any{
			"attempt":      2, // int from attachRetryContext
			"max_attempts": 2,
			"reason":       "success",
		},
	}

	stripRetryContextOnSuccess(responseData)

	if _, ok := responseData["retry_context"]; !ok {
		t.Error("retry_context should be kept when attempt > 1")
	}
}

func TestStripRetryContextOnSuccess_NoRetryContext(t *testing.T) {
	t.Parallel()
	responseData := map[string]any{"status": "complete"}
	// Should not panic
	stripRetryContextOnSuccess(responseData)
}

// ============================================
// blocked_by_overlay playbook tests (#319)
// ============================================

func TestLookupInteractFailurePlaybook_BlockedByOverlay(t *testing.T) {
	t.Parallel()
	canonical, playbook, ok := lookupInteractFailurePlaybook("blocked_by_overlay")
	if !ok {
		t.Fatal("blocked_by_overlay playbook should exist")
	}
	if canonical != "blocked_by_overlay" {
		t.Errorf("expected canonical code 'blocked_by_overlay', got %q", canonical)
	}
	if playbook.RetrySuggestion == "" {
		t.Error("blocked_by_overlay should have a retry suggestion")
	}
}

func TestNormalizeInteractFailureCode_OverlayVariants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected string
	}{
		{"blocked_by_overlay", "blocked_by_overlay"},
		{"BLOCKED_BY_OVERLAY", "blocked_by_overlay"},
		{"Element is blocked_by_overlay: click intercepted", "blocked_by_overlay"},
		{"unrelated_error", ""},
	}
	for _, tt := range tests {
		got := normalizeInteractFailureCode(tt.input)
		if got != tt.expected {
			t.Errorf("normalizeInteractFailureCode(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestAnnotateInteractFailureRecovery_BlockedByOverlay(t *testing.T) {
	t.Parallel()

	result := json.RawMessage(`{
		"success": false,
		"error": "blocked_by_overlay",
		"message": "Element is behind a modal overlay. Use dismiss_top_overlay first."
	}`)

	responseData := map[string]any{}
	annotateInteractFailureRecovery(responseData, "blocked_by_overlay", result)

	if responseData["error_code"] != "blocked_by_overlay" {
		t.Errorf("expected error_code 'blocked_by_overlay', got %v", responseData["error_code"])
	}
	if responseData["retryable"] != true {
		t.Error("blocked_by_overlay should be marked retryable")
	}

	retry, ok := responseData["retry"].(string)
	if !ok || retry == "" {
		t.Error("retry suggestion should be present for blocked_by_overlay")
	}

	hint, ok := responseData["hint"].(string)
	if !ok || hint == "" {
		t.Error("hint should be present for blocked_by_overlay")
	}
}
