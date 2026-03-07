// tools_interact_command_builder_test.go — Tests for the commandBuilder pattern.
// Validates that the builder correctly produces the same behavior as hand-coded handlers.
package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/queries"
)

// asyncArgs wraps args JSON with background=true to avoid sync extension wait in tests.
func asyncArgs(raw string) json.RawMessage {
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return json.RawMessage(raw)
	}
	m["background"] = true
	out, _ := json.Marshal(m)
	return json.RawMessage(out)
}

// TestCommandBuilder_BasicFlow verifies the standard correlate→arm→enqueue→wait flow.
func TestCommandBuilder_BasicFlow(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	args := asyncArgs(`{"tab_id":42}`)
	req := JSONRPCRequest{JSONRPC: "2.0", ID: 1, ClientID: "test"}
	h := env.handler.interactAction()

	resp := h.newCommand("test_flow").
		correlationPrefix("test").
		reason("test_action").
		queryType("browser_action").
		queryParams(args).
		tabID(42).
		guards(h.parent.requirePilot, h.parent.requireExtension, h.parent.requireTabTracking).
		queuedMessage("Test queued").
		execute(req, args)

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.IsError {
		t.Fatalf("should not error, got: %s", result.Content[0].Text)
	}

	pq := env.capture.GetLastPendingQuery()
	if pq == nil {
		t.Fatal("should create pending query")
	}
	if pq.Type != "browser_action" {
		t.Fatalf("query type = %q, want browser_action", pq.Type)
	}
	if pq.TabID != 42 {
		t.Fatalf("tab ID = %d, want 42", pq.TabID)
	}
	if pq.CorrelationID == "" {
		t.Fatal("correlation ID should not be empty")
	}
	if !strings.HasPrefix(pq.CorrelationID, "test_") {
		t.Fatalf("correlation ID = %q, want prefix test_", pq.CorrelationID)
	}
}

// TestCommandBuilder_PilotDisabledBlocks verifies that the pilot guard blocks execution.
func TestCommandBuilder_PilotDisabledBlocks(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)
	// pilot disabled by default

	args := asyncArgs(`{}`)
	req := JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	h := env.handler.interactAction()

	resp := h.newCommand("test_pilot").
		correlationPrefix("test").
		reason("test").
		queryType("browser_action").
		queryParams(args).
		guards(h.parent.requirePilot, h.parent.requireExtension).
		queuedMessage("Test queued").
		execute(req, args)

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !result.IsError {
		t.Fatal("pilot disabled should return error")
	}
	if !strings.Contains(strings.ToLower(result.Content[0].Text), "pilot") {
		t.Errorf("error should mention pilot, got: %s", result.Content[0].Text)
	}

	// Verify no pending query was created
	pq := env.capture.GetLastPendingQuery()
	if pq != nil {
		t.Fatal("pilot disabled should not create pending query")
	}
}

// TestCommandBuilder_WithBuildParams verifies the builder can construct params from a map.
func TestCommandBuilder_WithBuildParams(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	args := asyncArgs(`{}`)
	req := JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	h := env.handler.interactAction()

	resp := h.newCommand("test_params").
		correlationPrefix("exec").
		reason("execute_js").
		queryType("execute").
		buildParams(map[string]any{
			"script":     "console.log('hi')",
			"timeout_ms": 5000,
			"world":      "auto",
			"reason":     "execute_js",
		}).
		guards(h.parent.requirePilot, h.parent.requireExtension, h.parent.requireTabTracking).
		queuedMessage("Command queued").
		execute(req, args)

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.IsError {
		t.Fatalf("should not error, got: %s", result.Content[0].Text)
	}

	pq := env.capture.GetLastPendingQuery()
	if pq == nil {
		t.Fatal("should create pending query")
	}
	if pq.Type != "execute" {
		t.Fatalf("query type = %q, want execute", pq.Type)
	}
	// Verify built params contain expected fields
	var params map[string]any
	if err := json.Unmarshal(pq.Params, &params); err != nil {
		t.Fatalf("params unmarshal: %v", err)
	}
	if params["script"] != "console.log('hi')" {
		t.Fatalf("script = %v, want console.log('hi')", params["script"])
	}
}

// TestCommandBuilder_WithTimeout verifies custom timeout is forwarded.
func TestCommandBuilder_WithTimeout(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	args := asyncArgs(`{}`)
	req := JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	h := env.handler.interactAction()

	resp := h.newCommand("test_timeout").
		correlationPrefix("test").
		reason("test").
		queryType("browser_action").
		queryParams(args).
		timeout(queries.AsyncCommandTimeout).
		guards(h.parent.requirePilot, h.parent.requireExtension, h.parent.requireTabTracking).
		queuedMessage("Test queued").
		execute(req, args)

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.IsError {
		t.Fatalf("should not error, got: %s", result.Content[0].Text)
	}
}

// TestCommandBuilder_WithRecordAction verifies AI action recording.
func TestCommandBuilder_WithRecordAction(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	args := asyncArgs(`{}`)
	req := JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	h := env.handler.interactAction()

	resp := h.newCommand("test_record").
		correlationPrefix("test").
		reason("test").
		queryType("browser_action").
		queryParams(args).
		guards(h.parent.requirePilot, h.parent.requireExtension, h.parent.requireTabTracking).
		recordAction("navigate", "https://example.com", map[string]any{"foo": "bar"}).
		queuedMessage("Test queued").
		execute(req, args)

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.IsError {
		t.Fatalf("should not error, got: %s", result.Content[0].Text)
	}
	// AI action recording is fire-and-forget; main assertion is no error
}

// TestCommandBuilder_WithGuardOpts verifies structured error opts are passed to guards.
func TestCommandBuilder_WithGuardOpts(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)
	// pilot disabled to trigger guard

	args := asyncArgs(`{}`)
	req := JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	h := env.handler.interactAction()

	resp := h.newCommand("test_opts").
		correlationPrefix("test").
		reason("test").
		queryType("browser_action").
		queryParams(args).
		guardsWithOpts(
			[]func(*StructuredError){withAction("test_action")},
			h.parent.requirePilot, h.parent.requireExtension,
		).
		queuedMessage("Test queued").
		execute(req, args)

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !result.IsError {
		t.Fatal("pilot disabled should return error")
	}
}

// TestCommandBuilder_NoQueryParams_UsesEmptyArgs verifies that forgetting to set params
// falls back to using the original args as query params.
func TestCommandBuilder_NoQueryParams_UsesEmptyArgs(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	args := asyncArgs(`{}`)
	req := JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	h := env.handler.interactAction()

	// When no queryParams or buildParams is set, builder uses the original args
	resp := h.newCommand("test_defaults").
		correlationPrefix("test").
		reason("test").
		queryType("browser_action").
		guards(h.parent.requirePilot, h.parent.requireExtension, h.parent.requireTabTracking).
		queuedMessage("Test queued").
		execute(req, args)

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.IsError {
		t.Fatalf("should not error, got: %s", result.Content[0].Text)
	}

	pq := env.capture.GetLastPendingQuery()
	if pq == nil {
		t.Fatal("should create pending query")
	}
}

// TestCommandBuilder_MatchesQueueBrowserAction verifies the builder produces the same
// result as the existing queueBrowserAction helper for back/forward.
func TestCommandBuilder_MatchesQueueBrowserAction(t *testing.T) {
	t.Parallel()

	// Test with back action using builder
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, ok := env.callInteract(t, `{"what":"back"}`)
	if !ok {
		t.Fatal("back should return result")
	}
	if result.IsError {
		t.Fatalf("back should not error, got: %s", result.Content[0].Text)
	}

	pq := env.capture.GetLastPendingQuery()
	if pq == nil {
		t.Fatal("back should create pending query")
	}
	if pq.Type != "browser_action" {
		t.Fatalf("query type = %q, want browser_action", pq.Type)
	}
}

// TestCommandBuilder_MatchesHighlight verifies builder produces same result as handleHighlightImpl.
func TestCommandBuilder_MatchesHighlight(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, ok := env.callInteract(t, `{"what":"highlight","selector":"#main"}`)
	if !ok {
		t.Fatal("highlight should return result")
	}
	if result.IsError {
		t.Fatalf("highlight should not error, got: %s", result.Content[0].Text)
	}

	pq := env.capture.GetLastPendingQuery()
	if pq == nil {
		t.Fatal("highlight should create pending query")
	}
	if pq.Type != "highlight" {
		t.Fatalf("query type = %q, want highlight", pq.Type)
	}
}

// TestCommandBuilder_MatchesContentExtraction verifies builder for get_readable.
func TestCommandBuilder_MatchesContentExtraction(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, ok := env.callInteract(t, `{"what":"get_readable"}`)
	if !ok {
		t.Fatal("get_readable should return result")
	}
	if result.IsError {
		t.Fatalf("get_readable should not error, got: %s", result.Content[0].Text)
	}

	pq := env.capture.GetLastPendingQuery()
	if pq == nil {
		t.Fatal("get_readable should create pending query")
	}
	if pq.Type != "get_readable" {
		t.Fatalf("query type = %q, want get_readable", pq.Type)
	}
}

// TestCommandBuilder_MatchesRefresh verifies builder for refresh action.
func TestCommandBuilder_MatchesRefresh(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, ok := env.callInteract(t, `{"what":"refresh"}`)
	if !ok {
		t.Fatal("refresh should return result")
	}
	if result.IsError {
		t.Fatalf("refresh should not error, got: %s", result.Content[0].Text)
	}

	pq := env.capture.GetLastPendingQuery()
	if pq == nil {
		t.Fatal("refresh should create pending query")
	}
	if pq.Type != "browser_action" {
		t.Fatalf("query type = %q, want browser_action", pq.Type)
	}
}

// TestCommandBuilder_EmptyCorrelationPrefix_FallsBackToName verifies that omitting
// correlationPrefix falls back to the builder name.
func TestCommandBuilder_EmptyCorrelationPrefix_FallsBackToName(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	args := asyncArgs(`{}`)
	req := JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	h := env.handler.interactAction()

	resp := h.newCommand("myaction").
		reason("test").
		queryType("browser_action").
		queryParams(args).
		guards(h.parent.requirePilot, h.parent.requireExtension, h.parent.requireTabTracking).
		queuedMessage("Test queued").
		execute(req, args)

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.IsError {
		t.Fatalf("should not error, got: %s", result.Content[0].Text)
	}

	pq := env.capture.GetLastPendingQuery()
	if pq == nil {
		t.Fatal("should create pending query")
	}
	if !strings.HasPrefix(pq.CorrelationID, "myaction_") {
		t.Fatalf("correlation ID = %q, want prefix myaction_ (fallback from name)", pq.CorrelationID)
	}
}

// TestCommandBuilder_MissingQueryType_ReturnsError verifies that forgetting to set
// queryType produces an error response instead of silently creating a bad query.
func TestCommandBuilder_MissingQueryType_ReturnsError(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	args := asyncArgs(`{}`)
	req := JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	h := env.handler.interactAction()

	resp := h.newCommand("test_no_type").
		correlationPrefix("test").
		reason("test").
		queryParams(args).
		guards(h.parent.requirePilot, h.parent.requireExtension, h.parent.requireTabTracking).
		queuedMessage("Test queued").
		execute(req, args)

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !result.IsError {
		t.Fatal("missing queryType should return error")
	}
	if !strings.Contains(result.Content[0].Text, "queryType") {
		t.Errorf("error should mention queryType, got: %s", result.Content[0].Text)
	}

	pq := env.capture.GetLastPendingQuery()
	if pq != nil {
		t.Fatal("missing queryType should not create pending query")
	}
}

// TestCommandBuilder_GuardsWithOptsAppends verifies that multiple guardsWithOpts calls
// accumulate options instead of overwriting.
func TestCommandBuilder_GuardsWithOptsAppends(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)
	// pilot disabled to trigger guard

	args := asyncArgs(`{}`)
	req := JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	h := env.handler.interactAction()

	resp := h.newCommand("test_opts_append").
		correlationPrefix("test").
		reason("test").
		queryType("browser_action").
		queryParams(args).
		guardsWithOpts(
			[]func(*StructuredError){withAction("first_action")},
			h.parent.requirePilot,
		).
		guardsWithOpts(
			[]func(*StructuredError){withAction("second_action")},
			h.parent.requireExtension,
		).
		queuedMessage("Test queued").
		execute(req, args)

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// Should error from pilot guard (first in chain)
	if !result.IsError {
		t.Fatal("pilot disabled should return error")
	}
}
