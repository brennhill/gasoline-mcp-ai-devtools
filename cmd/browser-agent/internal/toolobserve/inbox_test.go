// inbox_test.go — Unit tests for HandleInbox draining behavior.

package toolobserve

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/push"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

// stubDeps is a minimal Deps implementation for inbox tests.
// Only PushInbox is exercised; the async/enqueuer methods are unused here.
type stubDeps struct {
	inbox *push.PushInbox
}

func (s *stubDeps) PushInbox() *push.PushInbox { return s.inbox }
func (s *stubDeps) IsExtensionConnected() bool { return true }
func (s *stubDeps) EnqueuePendingQuery(mcp.JSONRPCRequest, queries.PendingQuery, time.Duration) (mcp.JSONRPCResponse, bool) {
	return mcp.JSONRPCResponse{}, false
}
func (s *stubDeps) MaybeWaitForCommand(mcp.JSONRPCRequest, string, json.RawMessage, string) mcp.JSONRPCResponse {
	return mcp.JSONRPCResponse{}
}

func decodeInboxResult(t *testing.T, resp mcp.JSONRPCResponse) map[string]any {
	t.Helper()
	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected at least one content block")
	}
	// The summary block is text with a JSON payload appended. Find the JSON.
	text := result.Content[0].Text
	jsonStart := -1
	for i, r := range text {
		if r == '{' {
			jsonStart = i
			break
		}
	}
	if jsonStart < 0 {
		t.Fatalf("no JSON payload in response text: %q", text)
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(text[jsonStart:]), &data); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	return data
}

func TestHandleInbox_Empty(t *testing.T) {
	d := &stubDeps{inbox: push.NewPushInbox(10)}
	resp := HandleInbox(d, mcp.JSONRPCRequest{ID: json.RawMessage(`1`)}, nil)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
	data := decodeInboxResult(t, resp)
	if count, _ := data["count"].(float64); count != 0 {
		t.Errorf("count = %v, want 0", data["count"])
	}
}

func TestHandleInbox_WithEvents(t *testing.T) {
	inbox := push.NewPushInbox(10)
	inbox.Enqueue(push.PushEvent{ID: "a", Type: "chat", Message: "hello", Timestamp: time.Now()})
	inbox.Enqueue(push.PushEvent{ID: "b", Type: "screenshot", Timestamp: time.Now()})

	d := &stubDeps{inbox: inbox}
	resp := HandleInbox(d, mcp.JSONRPCRequest{ID: json.RawMessage(`2`)}, nil)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
	if inbox.Len() != 0 {
		t.Errorf("inbox should be drained, got Len() = %d", inbox.Len())
	}
	data := decodeInboxResult(t, resp)
	if count, _ := data["count"].(float64); count != 2 {
		t.Errorf("count = %v, want 2", data["count"])
	}
}

func TestHandleInbox_NilInbox(t *testing.T) {
	d := &stubDeps{inbox: nil}
	resp := HandleInbox(d, mcp.JSONRPCRequest{ID: json.RawMessage(`3`)}, nil)
	if resp.Error != nil {
		t.Fatalf("nil inbox should not error, got %v", resp.Error)
	}
	data := decodeInboxResult(t, resp)
	if count, _ := data["count"].(float64); count != 0 {
		t.Errorf("count = %v, want 0", data["count"])
	}
}
