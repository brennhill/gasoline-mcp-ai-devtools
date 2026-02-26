// Purpose: Validate tools_interact_gate_test.go behavior and guard against regressions.
// Why: Prevents silent regressions in critical behavior paths.
// Docs: docs/features/feature/interact-explore/index.md
//
// tools_interact_gate_test.go — Tests for pre-dispatch fast-fail gates on interact commands.
// Verifies that extension-disconnect and CSP-restricted states return immediate structured errors
// instead of queuing commands destined to time out.
//
// Run: go test ./cmd/dev-console -run "TestRequireExtension|TestRequireCSP|TestGateOrder|TestDiagnosticHint|TestNavigate_Ext|TestExecuteJS_CSP|TestClick_Ext|TestSubtitle_No" -v -count=1
package main

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dev-console/dev-console/internal/capture"
)

// ============================================
// Test Infrastructure
// ============================================

type gateTestEnv struct {
	handler *ToolHandler
	server  *Server
	capture *capture.Capture
}

// newGateTestEnv creates a test env WITHOUT extension connection (for disconnect tests).
func newGateTestEnv(t *testing.T) *gateTestEnv {
	t.Helper()
	logFile := filepath.Join(t.TempDir(), "test-gate.jsonl")
	server, err := NewServer(logFile, 100)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	t.Cleanup(func() { server.Close() })
	cap := capture.NewCapture()
	mcpHandler := NewToolHandler(server, cap)
	handler := mcpHandler.toolHandler.(*ToolHandler)
	return &gateTestEnv{handler: handler, server: server, capture: cap}
}

// simulateConnection sends a /sync POST to mark extension as connected.
func (e *gateTestEnv) simulateConnection(t *testing.T) {
	t.Helper()
	httpReq := httptest.NewRequest("POST", "/sync", strings.NewReader(`{"ext_session_id":"test"}`))
	httpReq.Header.Set("X-Gasoline-Client", "test-client")
	e.capture.HandleSync(httptest.NewRecorder(), httpReq)
}

// enablePilot turns on pilot for tests that need it.
func (e *gateTestEnv) enablePilot(t *testing.T) {
	t.Helper()
	e.capture.SetPilotEnabled(true)
}

// extractErrorCode parses the structured error code from a JSONRPCResponse result.
func extractErrorCode(t *testing.T, resp JSONRPCResponse) string {
	t.Helper()
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error response, got success")
	}
	if len(result.Content) == 0 {
		t.Fatal("error response has no content blocks")
	}
	text := result.Content[0].Text
	// The text may have a human-readable prefix before the JSON object.
	idx := strings.Index(text, "{")
	if idx < 0 {
		t.Fatalf("no JSON found in error text: %s", text)
	}
	var se StructuredError
	if err := json.Unmarshal([]byte(text[idx:]), &se); err != nil {
		t.Fatalf("unmarshal structured error: %v\nraw: %s", err, text[idx:])
	}
	return se.ErrorCode
}

// isSuccessOrQueued returns true if the response is not a structured error.
func isSuccessOrQueued(t *testing.T, resp JSONRPCResponse) bool {
	t.Helper()
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	return !result.IsError
}

// ============================================
// Gate unit tests: requireExtension
// ============================================

func TestRequireExtension_Disconnected(t *testing.T) {
	t.Parallel()
	env := newGateTestEnv(t)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp, blocked := env.handler.requireExtension(req)
	if !blocked {
		t.Fatal("expected requireExtension to block when extension is disconnected")
	}
	code := extractErrorCode(t, resp)
	if code != ErrNoData {
		t.Fatalf("expected error code %q, got %q", ErrNoData, code)
	}
}

func TestRequireExtension_Connected(t *testing.T) {
	t.Parallel()
	env := newGateTestEnv(t)
	env.simulateConnection(t)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp, blocked := env.handler.requireExtension(req)
	if blocked {
		t.Fatalf("expected requireExtension to pass when connected, got blocked with: %v", resp)
	}
}

// ============================================
// Gate unit tests: requireCSPClear
// ============================================

func TestRequireCSPClear_MainWorldBlocked(t *testing.T) {
	t.Parallel()
	env := newGateTestEnv(t)
	env.capture.SetCSPStatusForTest(true, "script_exec")

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp, blocked := env.handler.requireCSPClear(req, "main")
	if !blocked {
		t.Fatal("expected requireCSPClear to block world=main when CSP restricts script_exec")
	}
	code := extractErrorCode(t, resp)
	if code != ErrExtError {
		t.Fatalf("expected error code %q, got %q", ErrExtError, code)
	}
}

func TestRequireCSPClear_AutoWorldPasses(t *testing.T) {
	t.Parallel()
	env := newGateTestEnv(t)
	env.capture.SetCSPStatusForTest(true, "script_exec")

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	_, blocked := env.handler.requireCSPClear(req, "auto")
	if blocked {
		t.Fatal("expected requireCSPClear to pass for world=auto (extension handles fallback)")
	}
}

func TestRequireCSPClear_IsolatedWorldPasses(t *testing.T) {
	t.Parallel()
	env := newGateTestEnv(t)
	env.capture.SetCSPStatusForTest(true, "script_exec")

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	_, blocked := env.handler.requireCSPClear(req, "isolated")
	if blocked {
		t.Fatal("expected requireCSPClear to pass for world=isolated (bypasses page CSP)")
	}
}

func TestRequireCSPClear_PageBlocked(t *testing.T) {
	t.Parallel()
	env := newGateTestEnv(t)
	env.capture.SetCSPStatusForTest(true, "page_blocked")

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	_, blocked := env.handler.requireCSPClear(req, "main")
	if !blocked {
		t.Fatal("expected requireCSPClear to block world=main when CSP level is page_blocked")
	}
}

func TestRequireCSPClear_None(t *testing.T) {
	t.Parallel()
	env := newGateTestEnv(t)
	env.capture.SetCSPStatusForTest(false, "none")

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	_, blocked := env.handler.requireCSPClear(req, "main")
	if blocked {
		t.Fatal("expected requireCSPClear to pass when CSP is not restricted")
	}
}

func TestRequireCSPClear_NotRestricted(t *testing.T) {
	t.Parallel()
	env := newGateTestEnv(t)
	// restricted=false takes precedence even with a non-none level
	env.capture.SetCSPStatusForTest(false, "script_exec")

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	_, blocked := env.handler.requireCSPClear(req, "main")
	if blocked {
		t.Fatal("expected requireCSPClear to pass when restricted flag is false (flag wins)")
	}
}

// ============================================
// Integration tests through handlers
// ============================================

func TestNavigate_ExtDisconnected_FastFail(t *testing.T) {
	t.Parallel()
	env := newGateTestEnv(t)
	env.enablePilot(t)
	// Extension NOT connected — no simulateConnection call

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args := json.RawMessage(`{"what":"navigate","url":"https://example.com","sync":false}`)
	resp := env.handler.handleBrowserActionNavigate(req, args)

	code := extractErrorCode(t, resp)
	if code != ErrNoData {
		t.Fatalf("expected %q error for navigate with ext disconnected, got %q", ErrNoData, code)
	}
}

func TestExecuteJS_CSP_MainWorld_FastFail(t *testing.T) {
	t.Parallel()
	env := newGateTestEnv(t)
	env.enablePilot(t)
	env.simulateConnection(t)
	env.capture.SetCSPStatusForTest(true, "script_exec")

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args := json.RawMessage(`{"what":"execute_js","script":"return 1","world":"main","sync":false}`)
	resp := env.handler.handlePilotExecuteJS(req, args)

	code := extractErrorCode(t, resp)
	if code != ErrExtError {
		t.Fatalf("expected %q error for execute_js world=main with CSP restricted, got %q", ErrExtError, code)
	}
}

func TestExecuteJS_CSP_AutoWorld_PassesThrough(t *testing.T) {
	t.Parallel()
	env := newGateTestEnv(t)
	env.enablePilot(t)
	env.simulateConnection(t)
	env.capture.SetCSPStatusForTest(true, "script_exec")

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args := json.RawMessage(`{"what":"execute_js","script":"return 1","sync":false}`)
	resp := env.handler.handlePilotExecuteJS(req, args)

	// world=auto (default) passes through gate — extension handles CSP fallback
	if !isSuccessOrQueued(t, resp) {
		t.Fatal("expected execute_js with world=auto to pass through CSP gate, got error")
	}
}

func TestClick_ExtDisconnected_FastFail(t *testing.T) {
	t.Parallel()
	env := newGateTestEnv(t)
	env.enablePilot(t)
	// Extension NOT connected

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args := json.RawMessage(`{"what":"click","selector":"#btn","sync":false}`)
	resp := env.handler.handleDOMPrimitive(req, args, "click")

	code := extractErrorCode(t, resp)
	if code != ErrNoData {
		t.Fatalf("expected %q error for click with ext disconnected, got %q", ErrNoData, code)
	}
}

func TestSubtitle_NoExtensionGate(t *testing.T) {
	t.Parallel()
	env := newGateTestEnv(t)
	// Extension NOT connected, pilot not needed for subtitle

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args := json.RawMessage(`{"what":"subtitle","text":"hello","sync":false}`)
	resp := env.handler.handleSubtitle(req, args)

	// Subtitle should succeed (queued) even without extension
	if !isSuccessOrQueued(t, resp) {
		t.Fatal("expected subtitle to succeed without extension gate, got error")
	}
}

// ============================================
// Gate ordering tests
// ============================================

func TestGateOrder_ParamValidation_BeforeExtension(t *testing.T) {
	t.Parallel()
	env := newGateTestEnv(t)
	env.enablePilot(t)
	// Extension NOT connected + missing required 'script' param

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args := json.RawMessage(`{"what":"execute_js","sync":false}`)
	resp := env.handler.handlePilotExecuteJS(req, args)

	code := extractErrorCode(t, resp)
	if code != ErrMissingParam {
		t.Fatalf("expected %q (param validation first), got %q", ErrMissingParam, code)
	}
}

func TestGateOrder_Pilot_BeforeExtension(t *testing.T) {
	t.Parallel()
	env := newGateTestEnv(t)
	env.capture.SetPilotEnabled(false)
	// Extension NOT connected + pilot disabled

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args := json.RawMessage(`{"what":"navigate","url":"https://example.com","sync":false}`)
	resp := env.handler.handleBrowserActionNavigate(req, args)

	code := extractErrorCode(t, resp)
	if code != ErrCodePilotDisabled {
		t.Fatalf("expected %q (pilot before extension), got %q", ErrCodePilotDisabled, code)
	}
}

func TestGateOrder_Extension_BeforeCSP(t *testing.T) {
	t.Parallel()
	env := newGateTestEnv(t)
	env.enablePilot(t)
	// Extension NOT connected + CSP restricted
	env.capture.SetCSPStatusForTest(true, "script_exec")

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	args := json.RawMessage(`{"what":"execute_js","script":"return 1","world":"main","sync":false}`)
	resp := env.handler.handlePilotExecuteJS(req, args)

	code := extractErrorCode(t, resp)
	if code != ErrNoData {
		t.Fatalf("expected %q (extension before CSP), got %q", ErrNoData, code)
	}
}

// ============================================
// Diagnostic hint test
// ============================================

func TestDiagnosticHint_IncludesCSP(t *testing.T) {
	t.Parallel()
	env := newGateTestEnv(t)
	env.capture.SetCSPStatusForTest(true, "script_exec")

	hint := env.handler.DiagnosticHintString()
	if !strings.Contains(hint, "csp=") {
		t.Fatalf("expected diagnostic hint to include CSP status, got: %s", hint)
	}
	if !strings.Contains(hint, "script_exec") {
		t.Fatalf("expected diagnostic hint to include CSP level 'script_exec', got: %s", hint)
	}
}
