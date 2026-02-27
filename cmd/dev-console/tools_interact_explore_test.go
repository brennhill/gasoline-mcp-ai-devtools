// Purpose: Validate interact(what="explore_page") compound action.
// Why: Prevents regressions in the single-call page exploration that combines
// screenshot, interactive elements, page metadata, readable text, and navigation links.
// Docs: docs/features/feature/interact-explore/index.md

// tools_interact_explore_test.go — Tests for interact(what="explore_page") action.
package main

import (
	"strings"
	"testing"
)

// ============================================
// Dispatch Tests
// ============================================

func TestInteract_ExplorePage_DispatchesPendingQuery(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)

	resp := callInteractRaw(h, `{"what":"explore_page"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("explore_page should queue successfully, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	if data["status"] != "queued" {
		t.Errorf("status = %v, want 'queued'", data["status"])
	}
	corr, _ := data["correlation_id"].(string)
	if !strings.HasPrefix(corr, "explore_page_") {
		t.Errorf("correlation_id should start with 'explore_page_', got: %s", corr)
	}

	pq := cap.GetLastPendingQuery()
	if pq == nil {
		t.Fatal("expected pending query to be created")
	}
	if pq.Type != "explore_page" {
		t.Errorf("pending query type = %q, want 'explore_page'", pq.Type)
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

func TestInteract_ExplorePage_NoURL_UsesCurrentTab(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)

	resp := callInteractRaw(h, `{"what":"explore_page"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("explore_page without URL should succeed, got: %s", result.Content[0].Text)
	}

	pq := cap.GetLastPendingQuery()
	if pq == nil {
		t.Fatal("expected pending query to be created")
	}

	// Params should NOT contain a url field (uses current tab)
	paramsStr := string(pq.Params)
	if strings.Contains(paramsStr, `"url"`) {
		t.Errorf("pending query params should not contain url when not provided, got: %s", paramsStr)
	}
}

func TestInteract_ExplorePage_WithURL_IncludesNavigate(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)

	resp := callInteractRaw(h, `{"what":"explore_page","url":"https://example.com"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("explore_page with URL should succeed, got: %s", result.Content[0].Text)
	}

	pq := cap.GetLastPendingQuery()
	if pq == nil {
		t.Fatal("expected pending query to be created")
	}

	// Params should contain the url field
	paramsStr := string(pq.Params)
	if !strings.Contains(paramsStr, `"url"`) {
		t.Errorf("pending query params should contain url when provided, got: %s", paramsStr)
	}
	if !strings.Contains(paramsStr, "https://example.com") {
		t.Errorf("pending query params should contain the actual URL, got: %s", paramsStr)
	}
}

func TestInteract_ExplorePage_ForwardsParams(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)

	resp := callInteractRaw(h, `{"what":"explore_page","visible_only":true,"limit":50}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("explore_page with params should succeed, got: %s", result.Content[0].Text)
	}

	pq := cap.GetLastPendingQuery()
	if pq == nil {
		t.Fatal("expected pending query to be created")
	}
	if pq.Type != "explore_page" {
		t.Errorf("pending query type = %q, want 'explore_page'", pq.Type)
	}
}

// ============================================
// Mode Spec Tests (via describe_capabilities)
// ============================================

func TestInteract_ExplorePage_ModeSpec_Present(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{"what":"describe_capabilities","tool":"interact","mode":"explore_page"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("describe_capabilities for interact/explore_page should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	if data == nil {
		t.Fatal("describe_capabilities should return non-nil data")
	}
}

// ============================================
// Schema Tests
// ============================================

func TestInteract_ExplorePage_InWhatEnum(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	tools := h.ToolsList()
	var interactSchema map[string]any
	for _, tool := range tools {
		if tool.Name == "interact" {
			interactSchema = tool.InputSchema
			break
		}
	}
	if interactSchema == nil {
		t.Fatal("interact tool not found in ToolsList()")
	}

	props, ok := interactSchema["properties"].(map[string]any)
	if !ok {
		t.Fatal("interact schema missing properties")
	}
	whatProp, ok := props["what"].(map[string]any)
	if !ok {
		t.Fatal("interact schema missing 'what' property")
	}
	enumValues, ok := whatProp["enum"].([]string)
	if !ok {
		t.Fatal("'what' property missing enum")
	}

	found := false
	for _, v := range enumValues {
		if v == "explore_page" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("'what' enum should include 'explore_page', got: %v", enumValues)
	}
}

func TestInteract_ExplorePage_InValidActions(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	validActions := h.getValidInteractActions()
	if !strings.Contains(validActions, "explore_page") {
		t.Errorf("valid interact actions should include 'explore_page': %s", validActions)
	}
}

// ============================================
// Response Structure
// ============================================

func TestInteract_ExplorePage_ResponseStructure(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callInteractRaw(h, `{"what":"explore_page"}`)
	if resp.Result == nil {
		t.Fatal("interact(explore_page) returned nil result")
	}

	result := parseToolResult(t, resp)
	if len(result.Content) == 0 {
		t.Error("interact(explore_page) should return at least one content block")
	}
	if result.Content[0].Type != "text" {
		t.Errorf("content type = %q, want 'text'", result.Content[0].Type)
	}

	assertSnakeCaseFields(t, string(resp.Result))
}
