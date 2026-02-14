package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

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
	if len(resourceData.Resources) != 1 || resourceData.Resources[0].URI != "gasoline://guide" {
		t.Fatalf("resources/list result = %+v, want guide resource", resourceData)
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

	templates := h.HandleRequest(JSONRPCRequest{JSONRPC: "2.0", ID: 5, Method: "resources/templates/list"})
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
