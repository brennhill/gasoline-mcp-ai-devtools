// tools_interact_elements_test.go — Tests for element index store and resolution.
package main

import (
	"encoding/json"
	"testing"
)

func TestResolveIndexToSelector_Empty(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()
	_, ok := h.resolveIndexToSelector(0, "")
	if ok {
		t.Error("expected not found on empty store")
	}
}

func TestResolveIndexToSelector_AfterBuild(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()

	// Manually populate the store
	h.elementIndexMu.Lock()
	h.elementIndexStore = map[string]map[int]string{
		"": {
			0: "#email",
			1: "#password",
			2: "button[type=submit]",
		},
	}
	h.elementIndexMu.Unlock()

	sel, ok := h.resolveIndexToSelector(1, "")
	if !ok || sel != "#password" {
		t.Errorf("expected #password, got %q (ok=%v)", sel, ok)
	}

	_, ok = h.resolveIndexToSelector(99, "")
	if ok {
		t.Error("expected not found for missing index")
	}
}

func TestBuildElementIndexFromResponse_ValidElements(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()

	// Simulate a list_interactive response with elements in the JSON
	elemData := map[string]any{
		"elements": []any{
			map[string]any{"index": float64(0), "selector": "#name", "tag": "input"},
			map[string]any{"index": float64(1), "selector": ".btn-submit", "tag": "button"},
			map[string]any{"index": float64(2), "selector": "", "tag": "div"}, // empty selector, should be skipped
		},
	}
	elemJSON, _ := json.Marshal(elemData)

	result := MCPToolResult{
		Content: []MCPContentBlock{{Type: "text", Text: "list_interactive results\n" + string(elemJSON)}},
	}
	resultJSON, _ := json.Marshal(result)
	resp := JSONRPCResponse{JSONRPC: "2.0", Result: resultJSON}

	h.buildElementIndexFromResponse(resp, "")

	sel, ok := h.resolveIndexToSelector(0, "")
	if !ok || sel != "#name" {
		t.Errorf("index 0: expected #name, got %q (ok=%v)", sel, ok)
	}

	sel, ok = h.resolveIndexToSelector(1, "")
	if !ok || sel != ".btn-submit" {
		t.Errorf("index 1: expected .btn-submit, got %q (ok=%v)", sel, ok)
	}

	// Index 2 had empty selector, should not be stored
	_, ok = h.resolveIndexToSelector(2, "")
	if ok {
		t.Error("index 2 with empty selector should not be stored")
	}
}

func TestBuildElementIndexFromResponse_NestedResult(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()

	// Elements nested under result.result.elements
	nestedData := map[string]any{
		"result": map[string]any{
			"result": map[string]any{
				"elements": []any{
					map[string]any{"index": float64(0), "selector": "a.link"},
				},
			},
		},
	}
	nestedJSON, _ := json.Marshal(nestedData)

	result := MCPToolResult{
		Content: []MCPContentBlock{{Type: "text", Text: string(nestedJSON)}},
	}
	resultJSON, _ := json.Marshal(result)
	resp := JSONRPCResponse{JSONRPC: "2.0", Result: resultJSON}

	h.buildElementIndexFromResponse(resp, "")

	sel, ok := h.resolveIndexToSelector(0, "")
	if !ok || sel != "a.link" {
		t.Errorf("expected a.link from nested result, got %q (ok=%v)", sel, ok)
	}
}

func TestBuildElementIndexFromResponse_ErrorResponse(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()

	// Pre-populate store
	h.elementIndexMu.Lock()
	h.elementIndexStore = map[string]map[int]string{"": {0: "old"}}
	h.elementIndexMu.Unlock()

	// Error response should not clear the store
	result := MCPToolResult{
		Content: []MCPContentBlock{{Type: "text", Text: "error"}},
		IsError: true,
	}
	resultJSON, _ := json.Marshal(result)
	resp := JSONRPCResponse{JSONRPC: "2.0", Result: resultJSON}

	h.buildElementIndexFromResponse(resp, "")

	sel, ok := h.resolveIndexToSelector(0, "")
	if !ok || sel != "old" {
		t.Errorf("error response should not clear store, got %q (ok=%v)", sel, ok)
	}
}

func TestExtractElementList_Direct(t *testing.T) {
	t.Parallel()
	data := map[string]any{
		"elements": []any{
			map[string]any{"index": float64(0), "selector": "#a"},
		},
	}
	elems := extractElementList(data)
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
}

func TestExtractElementList_Nested(t *testing.T) {
	t.Parallel()
	data := map[string]any{
		"result": map[string]any{
			"elements": []any{
				map[string]any{"index": float64(0), "selector": "#b"},
			},
		},
	}
	elems := extractElementList(data)
	if len(elems) != 1 {
		t.Fatalf("expected 1 element from nested, got %d", len(elems))
	}
}

func TestExtractElementList_NoElements(t *testing.T) {
	t.Parallel()
	data := map[string]any{"foo": "bar"}
	elems := extractElementList(data)
	if elems != nil {
		t.Error("expected nil for data without elements")
	}
}

// ============================================
// CR-3: elementIndexStore must be scoped per client
// ============================================

func TestCR3_ElementIndexStore_ClientIsolation(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()

	// Client A populates store
	h.elementIndexMu.Lock()
	if h.elementIndexStore == nil {
		h.elementIndexStore = make(map[string]map[int]string)
	}
	h.elementIndexStore["client-a"] = map[int]string{0: "#email-a", 1: "#password-a"}
	h.elementIndexMu.Unlock()

	// Client B populates store — should NOT overwrite A's
	h.elementIndexMu.Lock()
	h.elementIndexStore["client-b"] = map[int]string{0: "#email-b"}
	h.elementIndexMu.Unlock()

	// Client A resolves — should get A's selector
	sel, ok := h.resolveIndexToSelector(0, "client-a")
	if !ok || sel != "#email-a" {
		t.Errorf("client-a index 0: expected #email-a, got %q (ok=%v)", sel, ok)
	}

	// Client B resolves — should get B's selector
	sel, ok = h.resolveIndexToSelector(0, "client-b")
	if !ok || sel != "#email-b" {
		t.Errorf("client-b index 0: expected #email-b, got %q (ok=%v)", sel, ok)
	}

	// Client A index 1 should still exist
	sel, ok = h.resolveIndexToSelector(1, "client-a")
	if !ok || sel != "#password-a" {
		t.Errorf("client-a index 1: expected #password-a, got %q (ok=%v)", sel, ok)
	}

	// Client B index 1 should not exist
	_, ok = h.resolveIndexToSelector(1, "client-b")
	if ok {
		t.Error("client-b index 1 should not exist")
	}
}

func TestCR3_ResolveIndexToSelector_EmptyClientFallback(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()

	// Single-client mode: empty clientID should still work
	h.elementIndexMu.Lock()
	if h.elementIndexStore == nil {
		h.elementIndexStore = make(map[string]map[int]string)
	}
	h.elementIndexStore[""] = map[int]string{0: "#solo"}
	h.elementIndexMu.Unlock()

	sel, ok := h.resolveIndexToSelector(0, "")
	if !ok || sel != "#solo" {
		t.Errorf("empty client: expected #solo, got %q (ok=%v)", sel, ok)
	}
}
