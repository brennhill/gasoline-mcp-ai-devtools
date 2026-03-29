// handler_tools_call_usage_test.go — Tests for usage counter wiring in tool call handler.

package main

import (
	"encoding/json"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/telemetry"
)

func TestExtractWhatParam(t *testing.T) {
	tests := []struct {
		name string
		args json.RawMessage
		want string
	}{
		{
			name: "valid args with what=errors",
			args: json.RawMessage(`{"what":"errors"}`),
			want: "errors",
		},
		{
			name: "missing what key",
			args: json.RawMessage(`{"key":"value"}`),
			want: "",
		},
		{
			name: "empty args object",
			args: json.RawMessage(`{}`),
			want: "",
		},
		{
			name: "malformed JSON",
			args: json.RawMessage(`{not valid json`),
			want: "",
		},
		{
			name: "nil args",
			args: nil,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractWhatParam(tt.args)
			if got != tt.want {
				t.Errorf("extractWhatParam(%s) = %q, want %q", string(tt.args), got, tt.want)
			}
		})
	}
}

func TestGetUsageCounter(t *testing.T) {
	t.Run("happy path returns counter", func(t *testing.T) {
		handler := createTestToolHandler(t)
		counter := telemetry.NewUsageCounter()
		handler.usageCounter = counter

		mcpHandler := NewMCPHandler(nil, "test")
		mcpHandler.SetToolHandler(handler)

		got := mcpHandler.GetUsageCounter()
		if got != counter {
			t.Fatalf("GetUsageCounter() = %p, want %p", got, counter)
		}
	})

	t.Run("nil tool handler returns nil", func(t *testing.T) {
		mcpHandler := NewMCPHandler(nil, "test")
		// toolHandler is nil — should return nil.
		got := mcpHandler.GetUsageCounter()
		if got != nil {
			t.Fatalf("GetUsageCounter() = %p, want nil when toolHandler is nil", got)
		}
	})

	t.Run("test double returns nil", func(t *testing.T) {
		mcpHandler := NewMCPHandler(nil, "test")
		mcpHandler.SetToolHandler(&fakeToolHandlerForMCP{})
		got := mcpHandler.GetUsageCounter()
		if got != nil {
			t.Fatalf("GetUsageCounter() = %p, want nil for test double", got)
		}
	})
}

func TestHandleToolCall_NilUsageCounter(t *testing.T) {
	handler := createTestToolHandler(t)
	// usageCounter is nil by default.

	req := JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		ID:      1,
		Method:  "tools/call",
	}

	// Verify tool dispatch works when usageCounter is nil.
	args := json.RawMessage(`{"what":"errors"}`)
	resp, handled := handler.HandleToolCall(req, "observe", args)
	if !handled {
		t.Fatal("observe tool was not handled with nil usageCounter")
	}

	// Verify the response is a valid JSON-RPC response (tool ran, not a panic).
	if resp.JSONRPC != JSONRPCVersion {
		t.Errorf("response JSONRPC = %q, want %q", resp.JSONRPC, JSONRPCVersion)
	}
}

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
