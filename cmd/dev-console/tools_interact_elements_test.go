// tools_interact_elements_test.go — Tests for element index store and resolution.
package main

import (
	"encoding/json"
	"testing"
)

func TestResolveIndexToSelector_Empty(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()
	_, ok := h.resolveIndexToSelector(0)
	if ok {
		t.Error("expected not found on empty store")
	}
}

func TestResolveIndexToSelector_AfterBuild(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()

	// Manually populate the store
	h.elementIndexMu.Lock()
	h.elementIndexStore = map[int]string{
		0: "#email",
		1: "#password",
		2: "button[type=submit]",
	}
	h.elementIndexMu.Unlock()

	sel, ok := h.resolveIndexToSelector(1)
	if !ok || sel != "#password" {
		t.Errorf("expected #password, got %q (ok=%v)", sel, ok)
	}

	_, ok = h.resolveIndexToSelector(99)
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

	h.buildElementIndexFromResponse(resp)

	sel, ok := h.resolveIndexToSelector(0)
	if !ok || sel != "#name" {
		t.Errorf("index 0: expected #name, got %q (ok=%v)", sel, ok)
	}

	sel, ok = h.resolveIndexToSelector(1)
	if !ok || sel != ".btn-submit" {
		t.Errorf("index 1: expected .btn-submit, got %q (ok=%v)", sel, ok)
	}

	// Index 2 had empty selector, should not be stored
	_, ok = h.resolveIndexToSelector(2)
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

	h.buildElementIndexFromResponse(resp)

	sel, ok := h.resolveIndexToSelector(0)
	if !ok || sel != "a.link" {
		t.Errorf("expected a.link from nested result, got %q (ok=%v)", sel, ok)
	}
}

func TestBuildElementIndexFromResponse_ErrorResponse(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()

	// Pre-populate store
	h.elementIndexMu.Lock()
	h.elementIndexStore = map[int]string{0: "old"}
	h.elementIndexMu.Unlock()

	// Error response should not clear the store
	result := MCPToolResult{
		Content: []MCPContentBlock{{Type: "text", Text: "error"}},
		IsError: true,
	}
	resultJSON, _ := json.Marshal(result)
	resp := JSONRPCResponse{JSONRPC: "2.0", Result: resultJSON}

	h.buildElementIndexFromResponse(resp)

	sel, ok := h.resolveIndexToSelector(0)
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
