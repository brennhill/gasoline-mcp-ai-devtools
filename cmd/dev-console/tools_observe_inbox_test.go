// tools_observe_inbox_test.go — Tests for observe inbox handler and push piggyback.
package main

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/dev-console/dev-console/internal/push"
)

func newPushTestToolHandler(inbox *push.PushInbox) *ToolHandler {
	s := &Server{pushInbox: inbox}
	mcp := &MCPHandler{server: s}
	return &ToolHandler{MCPHandler: mcp}
}

func TestToolObserveInbox_Empty(t *testing.T) {
	h := newPushTestToolHandler(push.NewPushInbox(10))

	resp := h.toolObserveInbox(JSONRPCRequest{ID: json.RawMessage(`1`)}, nil)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected at least one content block")
	}
}

func TestToolObserveInbox_WithEvents(t *testing.T) {
	inbox := push.NewPushInbox(10)
	inbox.Enqueue(push.PushEvent{
		ID:        "test-1",
		Type:      "chat",
		Message:   "hello",
		Timestamp: time.Now(),
	})
	inbox.Enqueue(push.PushEvent{
		ID:        "test-2",
		Type:      "screenshot",
		Timestamp: time.Now(),
	})

	h := newPushTestToolHandler(inbox)
	resp := h.toolObserveInbox(JSONRPCRequest{ID: json.RawMessage(`2`)}, nil)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	// After drain, inbox should be empty
	if inbox.Len() != 0 {
		t.Fatal("inbox should be empty after drain")
	}
}

func TestToolObserveInbox_NilInbox(t *testing.T) {
	h := newPushTestToolHandler(nil)
	resp := h.toolObserveInbox(JSONRPCRequest{ID: json.RawMessage(`3`)}, nil)
	if resp.Error != nil {
		t.Fatal("nil inbox should not error")
	}
}

func TestAppendPushPiggyback_Empty(t *testing.T) {
	h := newPushTestToolHandler(push.NewPushInbox(10))

	result := MCPToolResult{Content: []MCPContentBlock{{Type: "text", Text: "hello"}}}
	resultJSON, _ := json.Marshal(result)
	resp := JSONRPCResponse{JSONRPC: "2.0", Result: json.RawMessage(resultJSON)}

	out := h.appendPushPiggyback(resp)
	var outResult MCPToolResult
	if err := json.Unmarshal(out.Result, &outResult); err != nil {
		t.Fatal(err)
	}
	// Empty inbox should not add piggyback
	if len(outResult.Content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(outResult.Content))
	}
}

func TestAppendPushPiggyback_WithEvents(t *testing.T) {
	inbox := push.NewPushInbox(10)
	inbox.Enqueue(push.PushEvent{ID: "1", Type: "chat", Message: "pending"})

	h := newPushTestToolHandler(inbox)

	result := MCPToolResult{Content: []MCPContentBlock{{Type: "text", Text: "hello"}}}
	resultJSON, _ := json.Marshal(result)
	resp := JSONRPCResponse{JSONRPC: "2.0", Result: json.RawMessage(resultJSON)}

	out := h.appendPushPiggyback(resp)
	var outResult MCPToolResult
	if err := json.Unmarshal(out.Result, &outResult); err != nil {
		t.Fatal(err)
	}
	if len(outResult.Content) != 2 {
		t.Fatalf("expected 2 content blocks (original + piggyback), got %d", len(outResult.Content))
	}
}

func TestAppendPushPiggyback_NilInbox(t *testing.T) {
	h := newPushTestToolHandler(nil)

	resp := JSONRPCResponse{JSONRPC: "2.0", Result: json.RawMessage(`{}`)}
	out := h.appendPushPiggyback(resp)
	if string(out.Result) != `{}` {
		t.Fatal("nil inbox piggyback should be no-op")
	}
}
