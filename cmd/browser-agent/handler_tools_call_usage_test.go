// handler_tools_call_usage_test.go — Tests for usage counter wiring in tool call handler.

package main

import (
	"encoding/json"
	"testing"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/telemetry"
)

func TestHandleToolCall_IncrementsUsageCounter(t *testing.T) {
	handler := createTestToolHandler(t)

	counter := telemetry.NewUsageCounter()
	handler.usageCounter = counter

	req := JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		ID:      1,
		Method:  "tools/call",
	}

	// Call observe with what=errors — should increment "observe:errors".
	args := json.RawMessage(`{"what":"errors"}`)
	resp, handled := handler.HandleToolCall(req, "observe", args)
	if !handled {
		t.Fatal("observe tool was not handled")
	}
	// The tool may return an error (no extension) but the counter should still increment.
	_ = resp

	counts := counter.SwapAndReset()
	if counts["observe:errors"] != 1 {
		t.Fatalf("observe:errors count = %d, want 1", counts["observe:errors"])
	}
}

func TestHandleToolCall_IncrementsUsageCounter_NoWhatParam(t *testing.T) {
	handler := createTestToolHandler(t)

	counter := telemetry.NewUsageCounter()
	handler.usageCounter = counter

	req := JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		ID:      1,
		Method:  "tools/call",
	}

	// Call configure with no "what" — should increment "configure:unknown".
	args := json.RawMessage(`{"key":"value"}`)
	resp, handled := handler.HandleToolCall(req, "configure", args)
	if !handled {
		t.Fatal("configure tool was not handled")
	}
	_ = resp

	counts := counter.SwapAndReset()
	if counts["configure:unknown"] != 1 {
		t.Fatalf("configure:unknown count = %d, want 1", counts["configure:unknown"])
	}
}

func TestHandleToolCall_NilUsageCounter_NoPanic(t *testing.T) {
	handler := createTestToolHandler(t)
	// usageCounter is nil by default — should not panic.

	req := JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		ID:      1,
		Method:  "tools/call",
	}

	args := json.RawMessage(`{"what":"errors"}`)
	_, handled := handler.HandleToolCall(req, "observe", args)
	if !handled {
		t.Fatal("observe tool was not handled")
	}
	// If we got here without panic, the test passes.
}
