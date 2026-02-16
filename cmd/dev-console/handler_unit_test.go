package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dev-console/dev-console/internal/capture"
)

type testLimiter struct {
	allowed bool
}

func (l testLimiter) Allow() bool { return l.allowed }

type testRedactor struct {
	replacement json.RawMessage
}

func (r testRedactor) RedactJSON(_ json.RawMessage) json.RawMessage {
	return r.replacement
}

type fakeToolHandlerForMCP struct {
	cap      *capture.Capture
	limiter  RateLimiter
	redactor RedactionEngine
	tools    []MCPTool
	handleFn func(req JSONRPCRequest, name string, arguments json.RawMessage) (JSONRPCResponse, bool)
}

func (f *fakeToolHandlerForMCP) GetCapture() *capture.Capture { return f.cap }
func (f *fakeToolHandlerForMCP) GetToolCallLimiter() RateLimiter {
	return f.limiter
}
func (f *fakeToolHandlerForMCP) GetRedactionEngine() RedactionEngine {
	return f.redactor
}
func (f *fakeToolHandlerForMCP) ToolsList() []MCPTool { return f.tools }
func (f *fakeToolHandlerForMCP) HandleToolCall(req JSONRPCRequest, name string, arguments json.RawMessage) (JSONRPCResponse, bool) {
	if f.handleFn == nil {
		return JSONRPCResponse{}, false
	}
	return f.handleFn(req, name, arguments)
}

type failReader struct{}

func (failReader) Read(_ []byte) (int, error) { return 0, errors.New("boom") }

func mustDecodeJSON[T any](t *testing.T, raw json.RawMessage) T {
	t.Helper()
	var out T
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	return out
}

func mustTelemetryMetadata(t *testing.T, raw json.RawMessage) map[string]any {
	t.Helper()
	var result MCPToolResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("json.Unmarshal(MCPToolResult) error = %v", err)
	}
	if result.Metadata == nil {
		t.Fatal("result metadata missing")
	}
	return result.Metadata
}

func mustTelemetrySummary(t *testing.T, raw json.RawMessage) map[string]any {
	t.Helper()
	metadata := mustTelemetryMetadata(t, raw)
	summary, ok := metadata["telemetry_summary"].(map[string]any)
	if !ok {
		t.Fatalf("telemetry_summary missing or wrong type: %#v", metadata["telemetry_summary"])
	}
	return summary
}

func telemetrySummaryIfPresent(t *testing.T, raw json.RawMessage) (map[string]any, bool) {
	t.Helper()
	metadata := mustTelemetryMetadata(t, raw)
	summary, ok := metadata["telemetry_summary"].(map[string]any)
	if !ok {
		return nil, false
	}
	return summary, true
}

func mustTelemetryInt(t *testing.T, summary map[string]any, key string) int64 {
	t.Helper()
	v, ok := summary[key]
	if !ok {
		t.Fatalf("telemetry_summary[%q] missing", key)
	}
	f, ok := v.(float64)
	if !ok {
		t.Fatalf("telemetry_summary[%q] type = %T, want number", key, v)
	}
	return int64(f)
}

func mustTelemetryChanged(t *testing.T, raw json.RawMessage) bool {
	t.Helper()
	metadata := mustTelemetryMetadata(t, raw)
	v, ok := metadata["telemetry_changed"]
	if !ok {
		t.Fatal("telemetry_changed missing")
	}
	changed, ok := v.(bool)
	if !ok {
		t.Fatalf("telemetry_changed type = %T, want bool", v)
	}
	return changed
}

func TestMCPHandlerHandleRequestCorePaths(t *testing.T) {
	t.Parallel()

	h := NewMCPHandler(nil, "v1.2.3")

	if resp := h.HandleRequest(JSONRPCRequest{JSONRPC: "2.0", Method: "ping"}); resp != nil {
		t.Fatalf("notification without ID should return nil, got %+v", resp)
	}
	if resp := h.HandleRequest(JSONRPCRequest{JSONRPC: "2.0", ID: 1, Method: "notifications/initialized"}); resp != nil {
		t.Fatalf("notifications/* request should return nil, got %+v", resp)
	}

	unknown := h.HandleRequest(JSONRPCRequest{JSONRPC: "2.0", ID: 1, Method: "not/method"})
	if unknown == nil || unknown.Error == nil || unknown.Error.Code != -32601 {
		t.Fatalf("unknown method response = %+v, want method-not-found error", unknown)
	}

	initReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "initialize",
		Params:  json.RawMessage(`{"protocolVersion":"2024-11-05"}`),
	}
	initResp := h.HandleRequest(initReq)
	if initResp == nil || initResp.Error != nil {
		t.Fatalf("initialize response = %+v, want success", initResp)
	}
	initData := mustDecodeJSON[MCPInitializeResult](t, initResp.Result)
	if initData.ProtocolVersion != "2024-11-05" {
		t.Fatalf("ProtocolVersion = %q, want %q", initData.ProtocolVersion, "2024-11-05")
	}
	if initData.ServerInfo.Version != "v1.2.3" {
		t.Fatalf("server version = %q, want v1.2.3", initData.ServerInfo.Version)
	}

	initFallback := h.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "initialize",
		Params:  json.RawMessage(`{"protocolVersion":"2023-01-01"}`),
	})
	initFallbackData := mustDecodeJSON[MCPInitializeResult](t, initFallback.Result)
	if initFallbackData.ProtocolVersion != "2024-11-05" {
		t.Fatalf("fallback protocol = %q, want supported version", initFallbackData.ProtocolVersion)
	}
}

func TestMCPHandlerResourceAndToolMethods(t *testing.T) {
	t.Parallel()

	h := NewMCPHandler(nil, "v-test")
	th := &fakeToolHandlerForMCP{
		cap:     capture.NewCapture(),
		limiter: testLimiter{allowed: true},
		tools: []MCPTool{
			{Name: "observe"},
		},
		handleFn: func(req JSONRPCRequest, name string, _ json.RawMessage) (JSONRPCResponse, bool) {
			if name != "observe" {
				return JSONRPCResponse{}, false
			}
			return JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  json.RawMessage(`{"ok":true,"secret":"value"}`),
			}, true
		},
		redactor: testRedactor{replacement: json.RawMessage(`{"ok":true,"secret":"[REDACTED]"}`)},
	}
	h.SetToolHandler(th)

	resources := h.HandleRequest(JSONRPCRequest{JSONRPC: "2.0", ID: 1, Method: "resources/list"})
	if resources == nil || resources.Error != nil {
		t.Fatalf("resources/list response = %+v, want success", resources)
	}
	resourceData := mustDecodeJSON[MCPResourcesListResult](t, resources.Result)
	if len(resourceData.Resources) != 2 {
		t.Fatalf("resources/list result = %+v, want 2 resources", resourceData)
	}
	if resourceData.Resources[0].URI != "gasoline://guide" {
		t.Fatalf("resources/list first resource = %q, want gasoline://guide", resourceData.Resources[0].URI)
	}
	if resourceData.Resources[1].URI != "gasoline://quickstart" {
		t.Fatalf("resources/list second resource = %q, want gasoline://quickstart", resourceData.Resources[1].URI)
	}

	readInvalid := h.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "resources/read",
		Params:  json.RawMessage(`{`),
	})
	if readInvalid == nil || readInvalid.Error == nil || readInvalid.Error.Code != -32602 {
		t.Fatalf("resources/read invalid params response = %+v", readInvalid)
	}

	readNotFound := h.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "resources/read",
		Params:  json.RawMessage(`{"uri":"gasoline://missing"}`),
	})
	if readNotFound == nil || readNotFound.Error == nil || readNotFound.Error.Code != -32002 {
		t.Fatalf("resources/read not-found response = %+v", readNotFound)
	}

	readOK := h.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      4,
		Method:  "resources/read",
		Params:  json.RawMessage(`{"uri":"gasoline://guide"}`),
	})
	if readOK == nil || readOK.Error != nil {
		t.Fatalf("resources/read response = %+v, want success", readOK)
	}
	readData := mustDecodeJSON[MCPResourcesReadResult](t, readOK.Result)
	if len(readData.Contents) != 1 || readData.Contents[0].URI != "gasoline://guide" {
		t.Fatalf("resources/read result = %+v, want one guide content entry", readData)
	}

	readQuickstart := h.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      5,
		Method:  "resources/read",
		Params:  json.RawMessage(`{"uri":"gasoline://quickstart"}`),
	})
	if readQuickstart == nil || readQuickstart.Error != nil {
		t.Fatalf("resources/read quickstart response = %+v, want success", readQuickstart)
	}
	readQuickData := mustDecodeJSON[MCPResourcesReadResult](t, readQuickstart.Result)
	if len(readQuickData.Contents) != 1 || readQuickData.Contents[0].URI != "gasoline://quickstart" {
		t.Fatalf("resources/read quickstart result = %+v, want one quickstart content entry", readQuickData)
	}

	readDemo := h.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      6,
		Method:  "resources/read",
		Params:  json.RawMessage(`{"uri":"gasoline://demo/ws"}`),
	})
	if readDemo == nil || readDemo.Error != nil {
		t.Fatalf("resources/read demo response = %+v, want success", readDemo)
	}
	readDemoData := mustDecodeJSON[MCPResourcesReadResult](t, readDemo.Result)
	if len(readDemoData.Contents) != 1 || readDemoData.Contents[0].URI != "gasoline://demo/ws" {
		t.Fatalf("resources/read demo result = %+v, want demo content entry", readDemoData)
	}

	templates := h.HandleRequest(JSONRPCRequest{JSONRPC: "2.0", ID: 7, Method: "resources/templates/list"})
	if templates == nil || templates.Error != nil {
		t.Fatalf("resources/templates/list response = %+v, want success", templates)
	}

	toolsList := h.HandleRequest(JSONRPCRequest{JSONRPC: "2.0", ID: 6, Method: "tools/list"})
	if toolsList == nil || toolsList.Error != nil {
		t.Fatalf("tools/list response = %+v, want success", toolsList)
	}
	toolsData := mustDecodeJSON[MCPToolsListResult](t, toolsList.Result)
	if len(toolsData.Tools) != 1 || toolsData.Tools[0].Name != "observe" {
		t.Fatalf("tools/list result = %+v, want observe tool", toolsData)
	}

	callInvalid := h.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      7,
		Method:  "tools/call",
		Params:  json.RawMessage(`{`),
	})
	if callInvalid == nil || callInvalid.Error == nil || callInvalid.Error.Code != -32602 {
		t.Fatalf("tools/call invalid params response = %+v", callInvalid)
	}

	callUnknown := h.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      8,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"missing","arguments":{}}`),
	})
	if callUnknown == nil || callUnknown.Error == nil || callUnknown.Error.Code != -32601 {
		t.Fatalf("tools/call unknown response = %+v", callUnknown)
	}

	callObserve := h.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      9,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"observe","arguments":{"what":"errors"}}`),
	})
	if callObserve == nil || callObserve.Error != nil {
		t.Fatalf("tools/call handled response = %+v, want success", callObserve)
	}
	var redacted map[string]any
	if err := json.Unmarshal(callObserve.Result, &redacted); err != nil {
		t.Fatalf("json.Unmarshal(result) error = %v", err)
	}
	if redacted["secret"] != "[REDACTED]" {
		t.Fatalf("redacted result = %+v, expected secret to be redacted", redacted)
	}
}

func TestMCPHandler_AppendsServerWarningsToToolResponse(t *testing.T) {
	t.Parallel()

	logFile := filepath.Join(t.TempDir(), "warnings.jsonl")
	srv, err := NewServer(logFile, 100)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	srv.AddWarning("state_dir_not_writable: test warning")

	h := NewMCPHandler(srv, "v-test")
	h.SetToolHandler(&fakeToolHandlerForMCP{
		cap:     capture.NewCapture(),
		limiter: testLimiter{allowed: true},
		handleFn: func(req JSONRPCRequest, name string, _ json.RawMessage) (JSONRPCResponse, bool) {
			if name != "observe" {
				return JSONRPCResponse{}, false
			}
			return JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  mcpTextResponse("ok"),
			}, true
		},
	})

	resp := h.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"observe","arguments":{"what":"errors"}}`),
	})
	if resp == nil || resp.Error != nil {
		t.Fatalf("tools/call response = %+v, want success", resp)
	}
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("result unmarshal error = %v", err)
	}
	if len(result.Content) < 2 {
		t.Fatalf("expected warnings content block, got %d blocks", len(result.Content))
	}
	last := result.Content[len(result.Content)-1].Text
	if !strings.Contains(last, "_warnings:") {
		t.Fatalf("expected warnings content block, got %q", last)
	}

	// Warning should be one-shot.
	resp2 := h.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"observe","arguments":{"what":"errors"}}`),
	})
	if resp2 == nil || resp2.Error != nil {
		t.Fatalf("second tools/call response = %+v, want success", resp2)
	}
	var result2 MCPToolResult
	if err := json.Unmarshal(resp2.Result, &result2); err != nil {
		t.Fatalf("second result unmarshal error = %v", err)
	}
	for _, block := range result2.Content {
		if strings.Contains(block.Text, "_warnings:") {
			t.Fatalf("warning should be one-shot, got %q", block.Text)
		}
	}
}

func TestMCPHandlerToolRateLimit(t *testing.T) {
	t.Parallel()

	h := NewMCPHandler(nil, "v")
	h.SetToolHandler(&fakeToolHandlerForMCP{
		cap:     capture.NewCapture(),
		limiter: testLimiter{allowed: false},
	})

	resp := h.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"observe","arguments":{}}`),
	})
	if resp == nil || resp.Error == nil || resp.Error.Code != -32603 {
		t.Fatalf("rate-limited response = %+v, want internal error with rate-limit message", resp)
	}
}

func TestMCPHandler_PassiveTelemetrySummaryDeltas(t *testing.T) {
	t.Parallel()

	logFile := filepath.Join(t.TempDir(), "telemetry.jsonl")
	srv, err := NewServer(logFile, 100)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	t.Cleanup(srv.Close)
	cap := capture.NewCapture()
	cap.SetTrackingStatusForTest(42, "https://tracked.test")

	// Seed baseline data before first call; first response should still report zero deltas.
	srv.addEntries([]LogEntry{{"level": "error", "message": "baseline error"}})
	cap.AddNetworkBodies([]capture.NetworkBody{
		{Method: "GET", URL: "https://api.test/ok", Status: 200},
		{Method: "GET", URL: "https://api.test/fail", Status: 500},
	})
	cap.AddWebSocketEvents([]capture.WebSocketEvent{{Event: "message", ID: "ws-1"}})
	cap.AddEnhancedActions([]capture.EnhancedAction{{Type: "click", Timestamp: time.Now().UnixMilli()}})

	h := NewMCPHandler(srv, "v-test")
	h.SetToolHandler(&fakeToolHandlerForMCP{
		cap:     cap,
		limiter: testLimiter{allowed: true},
		handleFn: func(req JSONRPCRequest, name string, _ json.RawMessage) (JSONRPCResponse, bool) {
			if name != "observe" {
				return JSONRPCResponse{}, false
			}
			return JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  mcpTextResponse("ok"),
			}, true
		},
	})

	req := JSONRPCRequest{
		JSONRPC:  "2.0",
		ID:       1,
		Method:   "tools/call",
		ClientID: "client-a",
		Params:   json.RawMessage(`{"name":"observe","arguments":{"what":"logs"}}`),
	}
	resp1 := h.HandleRequest(req)
	if resp1 == nil || resp1.Error != nil {
		t.Fatalf("first tools/call response = %+v, want success", resp1)
	}
	if changed := mustTelemetryChanged(t, resp1.Result); changed {
		t.Fatal("first call telemetry_changed = true, want false")
	}
	if _, ok := telemetrySummaryIfPresent(t, resp1.Result); ok {
		t.Fatal("first call should omit telemetry_summary in auto mode when nothing changed")
	}

	// Add new activity between calls; second response should report these deltas.
	srv.addEntries([]LogEntry{
		{"level": "error", "message": "TypeError"},
		{"level": "info", "message": "noise"},
	})
	cap.AddNetworkBodies([]capture.NetworkBody{
		{Method: "GET", URL: "https://api.test/ok2", Status: 204},
		{Method: "GET", URL: "https://api.test/fail2", Status: 503},
	})
	cap.AddWebSocketEvents([]capture.WebSocketEvent{
		{Event: "message", ID: "ws-2"},
		{Event: "message", ID: "ws-3"},
	})
	cap.AddEnhancedActions([]capture.EnhancedAction{{Type: "type", Timestamp: time.Now().UnixMilli()}})

	req.ID = 2
	resp2 := h.HandleRequest(req)
	if resp2 == nil || resp2.Error != nil {
		t.Fatalf("second tools/call response = %+v, want success", resp2)
	}
	if changed := mustTelemetryChanged(t, resp2.Result); !changed {
		t.Fatal("second call telemetry_changed = false, want true")
	}
	summary2 := mustTelemetrySummary(t, resp2.Result)
	if got := mustTelemetryInt(t, summary2, "new_errors_since_last_call"); got != 1 {
		t.Fatalf("second call new_errors_since_last_call = %d, want 1", got)
	}
	if got := mustTelemetryInt(t, summary2, "new_network_requests_since_last_call"); got != 2 {
		t.Fatalf("second call new_network_requests_since_last_call = %d, want 2", got)
	}
	if got := mustTelemetryInt(t, summary2, "new_network_errors_since_last_call"); got != 1 {
		t.Fatalf("second call new_network_errors_since_last_call = %d, want 1", got)
	}
	if got := mustTelemetryInt(t, summary2, "new_websocket_events_since_last_call"); got != 2 {
		t.Fatalf("second call new_websocket_events_since_last_call = %d, want 2", got)
	}
	if got := mustTelemetryInt(t, summary2, "new_actions_since_last_call"); got != 1 {
		t.Fatalf("second call new_actions_since_last_call = %d, want 1", got)
	}
	if got, _ := summary2["trigger_tool"].(string); got != "observe" {
		t.Fatalf("trigger_tool = %q, want observe", got)
	}
}

func TestMCPHandler_PassiveTelemetryIsPerClient(t *testing.T) {
	t.Parallel()

	logFile := filepath.Join(t.TempDir(), "telemetry-per-client.jsonl")
	srv, err := NewServer(logFile, 100)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	t.Cleanup(srv.Close)
	cap := capture.NewCapture()

	h := NewMCPHandler(srv, "v-test")
	h.SetToolHandler(&fakeToolHandlerForMCP{
		cap:     cap,
		limiter: testLimiter{allowed: true},
		handleFn: func(req JSONRPCRequest, name string, _ json.RawMessage) (JSONRPCResponse, bool) {
			if name != "observe" {
				return JSONRPCResponse{}, false
			}
			return JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  mcpTextResponse("ok"),
			}, true
		},
	})

	reqA := JSONRPCRequest{
		JSONRPC:  "2.0",
		ID:       1,
		Method:   "tools/call",
		ClientID: "client-a",
		Params:   json.RawMessage(`{"name":"observe","arguments":{"what":"logs"}}`),
	}
	respA1 := h.HandleRequest(reqA)
	if respA1 == nil || respA1.Error != nil {
		t.Fatalf("client-a first call response = %+v, want success", respA1)
	}
	if changed := mustTelemetryChanged(t, respA1.Result); changed {
		t.Fatal("client-a first call telemetry_changed = true, want false")
	}
	if _, ok := telemetrySummaryIfPresent(t, respA1.Result); ok {
		t.Fatal("client-a first call should omit telemetry_summary in auto mode")
	}

	srv.addEntries([]LogEntry{{"level": "error", "message": "new error"}})

	reqA.ID = 2
	respA2 := h.HandleRequest(reqA)
	if respA2 == nil || respA2.Error != nil {
		t.Fatalf("client-a second call response = %+v, want success", respA2)
	}
	if changed := mustTelemetryChanged(t, respA2.Result); !changed {
		t.Fatal("client-a second call telemetry_changed = false, want true")
	}
	summaryA2 := mustTelemetrySummary(t, respA2.Result)
	if got := mustTelemetryInt(t, summaryA2, "new_errors_since_last_call"); got != 1 {
		t.Fatalf("client-a new_errors_since_last_call = %d, want 1", got)
	}

	reqB := JSONRPCRequest{
		JSONRPC:  "2.0",
		ID:       3,
		Method:   "tools/call",
		ClientID: "client-b",
		Params:   json.RawMessage(`{"name":"observe","arguments":{"what":"logs"}}`),
	}
	respB1 := h.HandleRequest(reqB)
	if respB1 == nil || respB1.Error != nil {
		t.Fatalf("client-b first call response = %+v, want success", respB1)
	}
	if changed := mustTelemetryChanged(t, respB1.Result); changed {
		t.Fatal("client-b first call telemetry_changed = true, want false")
	}
	if _, ok := telemetrySummaryIfPresent(t, respB1.Result); ok {
		t.Fatal("client-b first call should omit telemetry_summary in auto mode")
	}
}

func TestMCPHandler_PassiveTelemetryModeFullIncludesSummaryWithoutChanges(t *testing.T) {
	t.Parallel()

	logFile := filepath.Join(t.TempDir(), "telemetry-full.jsonl")
	srv, err := NewServer(logFile, 100)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	t.Cleanup(srv.Close)
	srv.setTelemetryMode(telemetryModeFull)

	h := NewMCPHandler(srv, "v-test")
	h.SetToolHandler(&fakeToolHandlerForMCP{
		cap:     capture.NewCapture(),
		limiter: testLimiter{allowed: true},
		handleFn: func(req JSONRPCRequest, name string, _ json.RawMessage) (JSONRPCResponse, bool) {
			if name != "observe" {
				return JSONRPCResponse{}, false
			}
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpTextResponse("ok")}, true
		},
	})

	resp := h.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"observe","arguments":{"what":"logs"}}`),
	})
	if resp == nil || resp.Error != nil {
		t.Fatalf("tools/call response = %+v, want success", resp)
	}
	if changed := mustTelemetryChanged(t, resp.Result); changed {
		t.Fatal("telemetry_changed = true, want false on first call")
	}
	summary := mustTelemetrySummary(t, resp.Result)
	if got := mustTelemetryInt(t, summary, "new_errors_since_last_call"); got != 0 {
		t.Fatalf("new_errors_since_last_call = %d, want 0", got)
	}
}

func TestMCPHandler_PassiveTelemetryModeOffSuppressesTelemetryMetadata(t *testing.T) {
	t.Parallel()

	logFile := filepath.Join(t.TempDir(), "telemetry-off.jsonl")
	srv, err := NewServer(logFile, 100)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	t.Cleanup(srv.Close)
	srv.setTelemetryMode(telemetryModeOff)

	h := NewMCPHandler(srv, "v-test")
	h.SetToolHandler(&fakeToolHandlerForMCP{
		cap:     capture.NewCapture(),
		limiter: testLimiter{allowed: true},
		handleFn: func(req JSONRPCRequest, name string, _ json.RawMessage) (JSONRPCResponse, bool) {
			if name != "observe" {
				return JSONRPCResponse{}, false
			}
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpTextResponse("ok")}, true
		},
	})

	resp := h.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"observe","arguments":{"what":"logs"}}`),
	})
	if resp == nil || resp.Error != nil {
		t.Fatalf("tools/call response = %+v, want success", resp)
	}

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("json.Unmarshal(MCPToolResult) error = %v", err)
	}
	if result.Metadata != nil {
		if _, ok := result.Metadata["telemetry_changed"]; ok {
			t.Fatal("telemetry_changed should be omitted in off mode")
		}
		if _, ok := result.Metadata["telemetry_summary"]; ok {
			t.Fatal("telemetry_summary should be omitted in off mode")
		}
	}
}

func TestMCPHandler_PassiveTelemetryModePerCallOverride(t *testing.T) {
	t.Parallel()

	logFile := filepath.Join(t.TempDir(), "telemetry-override.jsonl")
	srv, err := NewServer(logFile, 100)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	t.Cleanup(srv.Close)
	srv.setTelemetryMode(telemetryModeFull)

	h := NewMCPHandler(srv, "v-test")
	h.SetToolHandler(&fakeToolHandlerForMCP{
		cap:     capture.NewCapture(),
		limiter: testLimiter{allowed: true},
		handleFn: func(req JSONRPCRequest, name string, _ json.RawMessage) (JSONRPCResponse, bool) {
			if name != "observe" {
				return JSONRPCResponse{}, false
			}
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpTextResponse("ok")}, true
		},
	})

	resp := h.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"observe","arguments":{"what":"logs","telemetry_mode":"off"}}`),
	})
	if resp == nil || resp.Error != nil {
		t.Fatalf("tools/call response = %+v, want success", resp)
	}

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("json.Unmarshal(MCPToolResult) error = %v", err)
	}
	if result.Metadata != nil {
		if _, ok := result.Metadata["telemetry_summary"]; ok {
			t.Fatal("telemetry_summary should be suppressed by per-call telemetry_mode=off")
		}
		if _, ok := result.Metadata["telemetry_changed"]; ok {
			t.Fatal("telemetry_changed should be suppressed by per-call telemetry_mode=off")
		}
	}
}

func TestMCPHandlerHandleHTTP(t *testing.T) {
	t.Parallel()

	h := NewMCPHandler(nil, "v-http")
	h.SetToolHandler(&fakeToolHandlerForMCP{
		cap:     capture.NewCapture(),
		limiter: testLimiter{allowed: true},
		handleFn: func(req JSONRPCRequest, name string, _ json.RawMessage) (JSONRPCResponse, bool) {
			if name != "observe" {
				return JSONRPCResponse{}, false
			}
			return JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  json.RawMessage(`{"ok":true}`),
			}, true
		},
	})

	getReq := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	getRR := httptest.NewRecorder()
	h.HandleHTTP(getRR, getReq)
	if getRR.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /mcp status = %d, want %d", getRR.Code, http.StatusMethodNotAllowed)
	}

	parseReq := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,`))
	parseRR := httptest.NewRecorder()
	h.HandleHTTP(parseRR, parseReq)
	var parseResp JSONRPCResponse
	if err := json.Unmarshal(parseRR.Body.Bytes(), &parseResp); err != nil {
		t.Fatalf("json.Unmarshal(parse error response) error = %v", err)
	}
	if parseResp.Error == nil || parseResp.Error.Code != -32700 {
		t.Fatalf("parse error response = %+v, want parse error code -32700", parseResp)
	}

	notifyReq := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(`{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}`))
	notifyRR := httptest.NewRecorder()
	h.HandleHTTP(notifyRR, notifyReq)
	if notifyRR.Code != http.StatusNoContent {
		t.Fatalf("notification status = %d, want %d", notifyRR.Code, http.StatusNoContent)
	}

	callReq := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(`{"jsonrpc":"2.0","id":99,"method":"tools/call","params":{"name":"observe","arguments":{"what":"errors"}}}`))
	callRR := httptest.NewRecorder()
	h.HandleHTTP(callRR, callReq)
	var callResp JSONRPCResponse
	if err := json.Unmarshal(callRR.Body.Bytes(), &callResp); err != nil {
		t.Fatalf("json.Unmarshal(call response) error = %v", err)
	}
	if callResp.Error != nil {
		t.Fatalf("tools/call HTTP response has error: %+v", callResp.Error)
	}

	readErrReq := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBuffer(nil))
	readErrReq.Body = ioNopCloser{Reader: failReader{}}
	readErrRR := httptest.NewRecorder()
	h.HandleHTTP(readErrRR, readErrReq)
	var readErrResp JSONRPCResponse
	if err := json.Unmarshal(readErrRR.Body.Bytes(), &readErrResp); err != nil {
		t.Fatalf("json.Unmarshal(read error response) error = %v", err)
	}
	if readErrResp.Error == nil || readErrResp.Error.Code != -32700 {
		t.Fatalf("read error response = %+v, want code -32700", readErrResp)
	}
}

// ioNopCloser is a local substitute for io.NopCloser to avoid pulling in io for one test path.
type ioNopCloser struct {
	Reader interface {
		Read([]byte) (int, error)
	}
}

func (n ioNopCloser) Read(p []byte) (int, error) { return n.Reader.Read(p) }
func (n ioNopCloser) Close() error               { return nil }
