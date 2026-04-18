// handler_tools_call_usage_test.go — Tests for usage counter wiring in tool call handler.

package main

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/telemetry"
)

// TestToolAliasPrecedence_MatchesDispatcherOrder guards against silent drift between
// the telemetry alias precedence map and each tool's actual dispatcher aliasParams.
// If a tool adds or reorders an alias, telemetry must follow or it will log a
// different mode than the one dispatch picked.
func TestToolAliasPrecedence_MatchesDispatcherOrder(t *testing.T) {
	cases := []struct {
		tool    string
		aliases []modeAlias
	}{
		{"observe", defaultModeActionAliases},
		{"analyze", defaultModeActionAliases},
		{"configure", configureAliasParams},
		{"generate", generateAliasParams},
		{"interact", interactAliasParams},
	}
	for _, tc := range cases {
		t.Run(tc.tool, func(t *testing.T) {
			want := make([]string, len(tc.aliases))
			for i, a := range tc.aliases {
				want[i] = a.JSONField
			}
			got := toolAliasPrecedence[tc.tool]
			if !reflect.DeepEqual(got, want) {
				t.Errorf("toolAliasPrecedence[%q] = %v, want %v (from dispatcher aliasParams)",
					tc.tool, got, want)
			}
		})
	}
}

func TestUsageKey_DeprecatedAliases(t *testing.T) {
	// When callers use deprecated aliases (action/mode/format) instead of "what",
	// dispatch succeeds but we want the dashboard to distinguish these from callers
	// using the canonical field, so clients on the old shape can be identified.
	// Precedence must match each tool's dispatcher order (see toolAliasPrecedence).
	tests := []struct {
		name string
		tool string
		args json.RawMessage
		want string
	}{
		{
			name: "interact: action alias",
			tool: "interact",
			args: json.RawMessage(`{"action":"click"}`),
			want: "legacy_action:click",
		},
		{
			name: "configure: mode alias",
			tool: "configure",
			args: json.RawMessage(`{"mode":"streaming"}`),
			want: "legacy_mode:streaming",
		},
		{
			name: "generate: format alias",
			tool: "generate",
			args: json.RawMessage(`{"format":"playwright"}`),
			want: "legacy_format:playwright",
		},
		{
			name: "what takes precedence over action when both present",
			tool: "interact",
			args: json.RawMessage(`{"what":"click","action":"type"}`),
			want: "click",
		},
		{
			name: "configure: mode takes precedence over action (matches dispatcher order)",
			tool: "configure",
			args: json.RawMessage(`{"action":"navigate","mode":"streaming"}`),
			want: "legacy_mode:streaming",
		},
		{
			name: "generate: format takes precedence over action (matches dispatcher order)",
			tool: "generate",
			args: json.RawMessage(`{"action":"playwright","format":"har"}`),
			want: "legacy_format:har",
		},
		{
			name: "interact ignores mode field (not a declared alias for interact)",
			tool: "interact",
			args: json.RawMessage(`{"mode":"streaming"}`),
			want: "unknown_missing_what",
		},
		{
			name: "interact ignores format field (not a declared alias for interact)",
			tool: "interact",
			args: json.RawMessage(`{"format":"har"}`),
			want: "unknown_missing_what",
		},
		{
			name: "empty what falls through to alias",
			tool: "interact",
			args: json.RawMessage(`{"what":"","action":"click"}`),
			want: "legacy_action:click",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := usageKey(tt.tool, tt.args)
			if got != tt.want {
				t.Errorf("usageKey(%q, %s) = %q, want %q", tt.tool, string(tt.args), got, tt.want)
			}
		})
	}
}

func TestUsageKey_UnknownReasons(t *testing.T) {
	tests := []struct {
		name string
		args json.RawMessage
		want string
	}{
		{
			name: "nil args -> no_args",
			args: nil,
			want: "unknown_no_args",
		},
		{
			name: "empty args -> no_args",
			args: json.RawMessage(``),
			want: "unknown_no_args",
		},
		{
			name: "malformed JSON -> parse_error",
			args: json.RawMessage(`{not valid json`),
			want: "unknown_parse_error",
		},
		{
			name: "empty object -> missing_what",
			args: json.RawMessage(`{}`),
			want: "unknown_missing_what",
		},
		{
			name: "object without what or alias field -> missing_what",
			args: json.RawMessage(`{"unrelated":"value"}`),
			want: "unknown_missing_what",
		},
		{
			name: "what explicitly empty string with no aliases -> missing_what",
			args: json.RawMessage(`{"what":""}`),
			want: "unknown_missing_what",
		},
		{
			name: "JSON null literal -> missing_what",
			args: json.RawMessage(`null`),
			want: "unknown_missing_what",
		},
		{
			name: "what is a number (type mismatch) -> missing_what",
			args: json.RawMessage(`{"what":42}`),
			want: "unknown_missing_what",
		},
		{
			name: "what is an object -> missing_what",
			args: json.RawMessage(`{"what":{"nested":"value"}}`),
			want: "unknown_missing_what",
		},
		{
			name: "top-level JSON array -> parse_error",
			args: json.RawMessage(`[]`),
			want: "unknown_parse_error",
		},
		{
			name: "top-level JSON string -> parse_error",
			args: json.RawMessage(`"hello"`),
			want: "unknown_parse_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := usageKey("interact", tt.args)
			if got != tt.want {
				t.Errorf("usageKey(%s) = %q, want %q", string(tt.args), got, tt.want)
			}
		})
	}
}

func TestUsageKey_CommandResultMapsToOriginalCommand(t *testing.T) {
	tests := []struct {
		name string
		args json.RawMessage
		want string
	}{
		{
			name: "command_result with nav prefix",
			args: json.RawMessage(`{"what":"command_result","correlation_id":"nav_1708300000_123"}`),
			want: "command_result:nav",
		},
		{
			name: "command_result with click prefix",
			args: json.RawMessage(`{"what":"command_result","correlation_id":"click_1708300000_456"}`),
			want: "command_result:click",
		},
		{
			name: "command_result with draw prefix",
			args: json.RawMessage(`{"what":"command_result","correlation_id":"draw_1708300000_789"}`),
			want: "command_result:draw",
		},
		{
			name: "command_result with execute_js prefix",
			args: json.RawMessage(`{"what":"command_result","correlation_id":"execute_js_1708300000_123"}`),
			want: "command_result:execute_js",
		},
		{
			name: "command_result with state_restore prefix",
			args: json.RawMessage(`{"what":"command_result","correlation_id":"state_restore_1708300000_123"}`),
			want: "command_result:state_restore",
		},
		{
			name: "command_result with dom_auto_dismiss_overlays prefix",
			args: json.RawMessage(`{"what":"command_result","correlation_id":"dom_auto_dismiss_overlays_1708300000_123"}`),
			want: "command_result:dom_auto_dismiss_overlays",
		},
		{
			name: "command_result with ann_ prefix",
			args: json.RawMessage(`{"what":"command_result","correlation_id":"ann_1708300000_123"}`),
			want: "command_result:ann",
		},
		{
			name: "command_result without correlation_id",
			args: json.RawMessage(`{"what":"command_result"}`),
			want: "command_result",
		},
		{
			name: "command_result with empty correlation_id",
			args: json.RawMessage(`{"what":"command_result","correlation_id":""}`),
			want: "command_result",
		},
		{
			name: "command_result with no-underscore correlation_id",
			args: json.RawMessage(`{"what":"command_result","correlation_id":"plainid"}`),
			want: "command_result:plainid",
		},
		{
			name: "command_result with nonnumeric underscore suffix keeps full id",
			args: json.RawMessage(`{"what":"command_result","correlation_id":"execute_js_manual_retry"}`),
			want: "command_result:execute_js_manual_retry",
		},
		{
			name: "regular what param unaffected",
			args: json.RawMessage(`{"what":"page","correlation_id":"nav_123"}`),
			want: "page",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := usageKey("observe", tt.args)
			if got != tt.want {
				t.Errorf("usageKey(%s) = %q, want %q", string(tt.args), got, tt.want)
			}
		})
	}
}

func TestGetUsageTracker(t *testing.T) {
	t.Run("happy path returns counter", func(t *testing.T) {
		handler := createTestToolHandler(t)
		counter := telemetry.NewUsageTracker()
		handler.usageTracker = counter

		mcpHandler := NewMCPHandler(nil, "test")
		mcpHandler.SetToolHandler(handler)

		got := mcpHandler.GetUsageTracker()
		if got != counter {
			t.Fatalf("GetUsageTracker() = %p, want %p", got, counter)
		}
	})

	t.Run("nil tool handler returns nil", func(t *testing.T) {
		mcpHandler := NewMCPHandler(nil, "test")
		// toolHandler is nil — should return nil.
		got := mcpHandler.GetUsageTracker()
		if got != nil {
			t.Fatalf("GetUsageTracker() = %p, want nil when toolHandler is nil", got)
		}
	})

	t.Run("test double returns nil", func(t *testing.T) {
		mcpHandler := NewMCPHandler(nil, "test")
		mcpHandler.SetToolHandler(&fakeToolHandlerForMCP{})
		got := mcpHandler.GetUsageTracker()
		if got != nil {
			t.Fatalf("GetUsageTracker() = %p, want nil for test double", got)
		}
	})
}

func TestHandleToolCall_NilUsageTracker(t *testing.T) {
	handler := createTestToolHandler(t)
	// usageTracker is nil by default.

	req := JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		ID:      1,
		Method:  "tools/call",
	}

	// Verify tool dispatch works when usageTracker is nil.
	args := json.RawMessage(`{"what":"errors"}`)
	resp, handled := handler.HandleToolCall(req, "observe", args)
	if !handled {
		t.Fatal("observe tool was not handled with nil usageTracker")
	}

	// Verify the response is a valid JSON-RPC response (tool ran, not a panic).
	if resp.JSONRPC != JSONRPCVersion {
		t.Errorf("response JSONRPC = %q, want %q", resp.JSONRPC, JSONRPCVersion)
	}
}

func TestHandleToolCall_IncrementsUsageTracker(t *testing.T) {
	handler := createTestToolHandler(t)

	counter := telemetry.NewUsageTracker()
	handler.usageTracker = counter

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

	counts := counter.Peek()
	if counts["observe:errors"] != 1 {
		t.Fatalf("observe:errors count = %d, want 1", counts["observe:errors"])
	}
}

func TestHandleToolCall_IncrementsUsageTracker_NoWhatParam(t *testing.T) {
	handler := createTestToolHandler(t)

	counter := telemetry.NewUsageTracker()
	handler.usageTracker = counter

	req := JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		ID:      1,
		Method:  "tools/call",
	}

	// Call configure with no "what" — should increment "configure:unknown_missing_what".
	args := json.RawMessage(`{"key":"value"}`)
	resp, handled := handler.HandleToolCall(req, "configure", args)
	if !handled {
		t.Fatal("configure tool was not handled")
	}
	_ = resp

	counts := counter.Peek()
	if counts["configure:unknown_missing_what"] != 1 {
		t.Fatalf("configure:unknown_missing_what count = %d, want 1", counts["configure:unknown_missing_what"])
	}
}

func TestHandleToolCall_RecordsLatency(t *testing.T) {
	handler := createTestToolHandler(t)

	counter := telemetry.NewUsageTracker()
	handler.usageTracker = counter

	req := JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		ID:      1,
		Method:  "tools/call",
	}

	args := json.RawMessage(`{"what":"errors"}`)
	handler.HandleToolCall(req, "observe", args)

	snapshot := counter.SwapAndReset()
	if snapshot == nil {
		t.Fatal("snapshot is nil")
	}
	found := false
	for _, s := range snapshot.ToolStats {
		if s.Tool == "observe:errors" {
			found = true
			// Latency should be >= 0ms.
			if s.LatencyAvgMs < 0 {
				t.Fatalf("LatencyAvgMs = %d, want >= 0", s.LatencyAvgMs)
			}
		}
	}
	if !found {
		t.Fatal("observe:errors not found in tool stats")
	}
}

func TestHandleToolCall_RecordsErrorRate(t *testing.T) {
	handler := createTestToolHandler(t)

	counter := telemetry.NewUsageTracker()
	handler.usageTracker = counter

	req := JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		ID:      1,
		Method:  "tools/call",
	}

	// Call interact with missing required params — guaranteed to produce an error.
	args := json.RawMessage(`{}`)
	handler.HandleToolCall(req, "interact", args)

	counts := counter.Peek()
	if counts["interact:unknown_missing_what"] != 1 {
		t.Fatalf("interact:unknown_missing_what = %d, want 1", counts["interact:unknown_missing_what"])
	}
	if counts["err:interact:unknown_missing_what"] != 1 {
		t.Fatalf("err:interact:unknown_missing_what = %d, want 1 (missing params = error)", counts["err:interact:unknown_missing_what"])
	}
}

func TestHandleToolCall_TracksSessionDepth(t *testing.T) {
	handler := createTestToolHandler(t)

	counter := telemetry.NewUsageTracker()
	handler.usageTracker = counter

	req := JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		ID:      1,
		Method:  "tools/call",
	}

	// Make 3 tool calls.
	for i := 0; i < 3; i++ {
		handler.HandleToolCall(req, "observe", json.RawMessage(`{"what":"errors"}`))
	}

	if counter.SessionDepth() != 3 {
		t.Fatalf("session depth = %d, want 3", counter.SessionDepth())
	}

	snapshot := counter.SwapAndReset()
	if snapshot == nil {
		t.Fatal("snapshot is nil")
	}
	// SessionDepth is tracked on the tracker (not in snapshot — removed from contract).
	if counter.SessionDepth() != 3 {
		t.Fatalf("session depth = %d, want 3", counter.SessionDepth())
	}
}

func TestHandleToolCall_NilUsageTracker_NoPanic(t *testing.T) {
	handler := createTestToolHandler(t)
	// usageTracker is nil by default — should not panic.

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
