// test_helpers_test.go — Test support for toolinteract package tests.
package toolinteract

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

// stubDeps is a minimal Deps implementation for unit tests.
type stubDeps struct{}

func (s *stubDeps) RequirePilot(_ mcp.JSONRPCRequest, _ ...func(*mcp.StructuredError)) (mcp.JSONRPCResponse, bool) {
	return mcp.JSONRPCResponse{}, false
}
func (s *stubDeps) RequireExtension(_ mcp.JSONRPCRequest, _ ...func(*mcp.StructuredError)) (mcp.JSONRPCResponse, bool) {
	return mcp.JSONRPCResponse{}, false
}
func (s *stubDeps) RequireTabTracking(_ mcp.JSONRPCRequest, _ ...func(*mcp.StructuredError)) (mcp.JSONRPCResponse, bool) {
	return mcp.JSONRPCResponse{}, false
}
func (s *stubDeps) RequireCSPClear(_ mcp.JSONRPCRequest, _ string) (mcp.JSONRPCResponse, bool) {
	return mcp.JSONRPCResponse{}, false
}
func (s *stubDeps) EnqueuePendingQuery(_ mcp.JSONRPCRequest, _ queries.PendingQuery, _ time.Duration) (mcp.JSONRPCResponse, bool) {
	return mcp.JSONRPCResponse{}, false
}
func (s *stubDeps) MaybeWaitForCommand(_ mcp.JSONRPCRequest, _ string, _ json.RawMessage, _ string) mcp.JSONRPCResponse {
	return mcp.JSONRPCResponse{}
}
func (s *stubDeps) Capture() *capture.Store                                     { return nil }
func (s *stubDeps) RecordAIAction(_, _ string, _ map[string]any)                {}
func (s *stubDeps) RecordAIEnhancedAction(_ capture.EnhancedAction)             {}
func (s *stubDeps) RecordDOMPrimitiveAction(_, _, _, _ string)                  {}
func (s *stubDeps) ToolInteract(_ mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
	return mcp.JSONRPCResponse{}
}
func (s *stubDeps) ToolAnalyze(_ mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
	return mcp.JSONRPCResponse{}
}
func (s *stubDeps) ToolExportSARIF(_ mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
	return mcp.JSONRPCResponse{}
}
func (s *stubDeps) EnrichNavigateResponse(resp mcp.JSONRPCResponse, _ mcp.JSONRPCRequest, _ int) mcp.JSONRPCResponse {
	return resp
}
func (s *stubDeps) InjectCSPBlockedActions(resp mcp.JSONRPCResponse) mcp.JSONRPCResponse {
	return resp
}
func (s *stubDeps) GetScreenshot(_ mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
	return mcp.JSONRPCResponse{}
}
func (s *stubDeps) GetPageInfo(_ mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
	return mcp.JSONRPCResponse{}
}
func (s *stubDeps) MarkDrawStarted()                              {}
func (s *stubDeps) GetListenPort() int                            { return 0 }
func (s *stubDeps) DefaultEvidenceCapture(_ string) EvidenceShot  { return EvidenceShot{Error: "stub"} }
func (s *stubDeps) RequireSessionStore(_ mcp.JSONRPCRequest) (mcp.JSONRPCResponse, bool) {
	return mcp.JSONRPCResponse{}, false
}
func (s *stubDeps) DiagnosticHint() func(*mcp.StructuredError) { return nil }
func (s *stubDeps) GetRedactionEngine() MapValueRedactor       { return nil }
func (s *stubDeps) GetCommandResult(_ string) (*queries.CommandResult, bool) {
	return nil, false
}
func (s *stubDeps) GetReplayMu() *sync.Mutex { return &sync.Mutex{} }

// newTestHandler creates a minimal InteractActionHandler for unit tests.
func newTestHandler() *InteractActionHandler {
	return NewInteractActionHandler(&stubDeps{})
}

// newTestToolHandler wraps newTestHandler for compatibility with moved tests.
func newTestToolHandler() *testToolHandlerShim {
	return &testToolHandlerShim{h: newTestHandler()}
}

// testToolHandlerShim wraps InteractActionHandler to provide the interactAction() pattern
// that existing tests expect.
type testToolHandlerShim struct {
	h *InteractActionHandler
}

func (s *testToolHandlerShim) interactAction() *InteractActionHandler {
	return s.h
}

// parseToolResult is a test helper that unmarshals a mcp.JSONRPCResponse result into mcp.MCPToolResult.
func parseToolResult(t *testing.T, resp mcp.JSONRPCResponse) mcp.MCPToolResult {
	t.Helper()
	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to parse tool result: %v", err)
	}
	return result
}

// firstText extracts the first text content from a mcp.MCPToolResult.
func firstText(result mcp.MCPToolResult) string {
	for _, c := range result.Content {
		if c.Type == "text" {
			return c.Text
		}
	}
	return ""
}
