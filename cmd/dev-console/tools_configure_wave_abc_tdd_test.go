package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/dev-console/dev-console/internal/capture"
)

func configureSchemaPropertiesForTest(t *testing.T) map[string]any {
	t.Helper()
	server, err := NewServer(t.TempDir()+"/schema-wave-abc.jsonl", 10)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	t.Cleanup(func() { server.Close() })
	cap := capture.NewCapture()
	tools := NewToolHandler(server, cap).toolHandler.ToolsList()
	for _, tool := range tools {
		if tool.Name != "configure" {
			continue
		}
		props, ok := tool.InputSchema["properties"].(map[string]any)
		if !ok {
			t.Fatal("configure tool schema missing properties")
		}
		return props
	}
	t.Fatal("configure tool not found in schema")
	return nil
}

func callHandledTool(t *testing.T, h *ToolHandler, req JSONRPCRequest, name, argsJSON string) JSONRPCResponse {
	t.Helper()
	resp, handled := h.HandleToolCall(req, name, json.RawMessage(argsJSON))
	if !handled {
		t.Fatalf("tool %q was not handled", name)
	}
	return resp
}

func TestWaveA_ConfigureSchema_DiffSessionsURLPropertyPresent(t *testing.T) {
	t.Parallel()

	props := configureSchemaPropertiesForTest(t)
	if _, ok := props["url"]; !ok {
		t.Fatal("configure schema should include 'url' for diff_sessions capture filtering")
	}
}

func TestWaveB_ConfigureSchema_AuditLogOperationPropertyPresent(t *testing.T) {
	t.Parallel()

	props := configureSchemaPropertiesForTest(t)
	opRaw, ok := props["operation"].(map[string]any)
	if !ok {
		t.Fatal("configure schema should include 'operation' for audit_log operation routing")
	}

	enumVals, ok := opRaw["enum"].([]string)
	if !ok {
		t.Fatal("configure.operation enum should be []string")
	}
	got := strings.Join(enumVals, ",")
	for _, want := range []string{"analyze", "report", "clear"} {
		if !strings.Contains(got, want) {
			t.Fatalf("configure.operation enum missing %q: %v", want, enumVals)
		}
	}
}

func TestWaveB_AuditLogOperationAnalyzeAndClear(t *testing.T) {
	t.Parallel()

	server, err := NewServer(t.TempDir()+"/audit-waveb.jsonl", 100)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	t.Cleanup(func() { server.Close() })

	cap := capture.NewCapture()
	mcpHandler := NewToolHandler(server, cap)
	h := mcpHandler.toolHandler.(*ToolHandler)
	req := JSONRPCRequest{JSONRPC: "2.0", ID: 1, ClientID: "wave-b-test"}

	callHandledTool(t, h, req, "configure", `{"action":"health"}`)
	callHandledTool(t, h, req, "observe", `{"what":"logs"}`)

	analyzeResp := callHandledTool(t, h, req, "configure", `{"action":"audit_log","operation":"analyze"}`)
	analyzeResult := parseToolResult(t, analyzeResp)
	if analyzeResult.IsError {
		t.Fatalf("audit_log analyze should succeed, got: %s", analyzeResult.Content[0].Text)
	}
	analyzeData := extractResultJSON(t, analyzeResult)
	if analyzeData["operation"] != "analyze" {
		t.Fatalf("operation = %v, want analyze", analyzeData["operation"])
	}
	if _, ok := analyzeData["summary"]; !ok {
		t.Fatal("audit_log analyze should include summary")
	}

	clearResp := callHandledTool(t, h, req, "configure", `{"action":"audit_log","operation":"clear"}`)
	clearResult := parseToolResult(t, clearResp)
	if clearResult.IsError {
		t.Fatalf("audit_log clear should succeed, got: %s", clearResult.Content[0].Text)
	}
	clearData := extractResultJSON(t, clearResult)
	if clearData["operation"] != "clear" {
		t.Fatalf("operation = %v, want clear", clearData["operation"])
	}
	if _, ok := clearData["cleared"]; !ok {
		t.Fatal("audit_log clear should report cleared count")
	}
}

func TestWaveC_RedactionEngineIsWiredAndApplied(t *testing.T) {
	t.Parallel()

	server, err := NewServer(t.TempDir()+"/redaction-wavec.jsonl", 100)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	t.Cleanup(func() { server.Close() })

	cap := capture.NewCapture()
	h := NewToolHandler(server, cap)
	if h.toolHandler.GetRedactionEngine() == nil {
		t.Fatal("tool handler should provide a redaction engine")
	}

	input := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      1,
		Result:  json.RawMessage(`{"content":[{"type":"text","text":"Authorization: Bearer ghp_1234567890abcdef"}],"isError":false}`),
	}
	output := h.applyToolResponsePostProcessing(input, "wave-c-test", "configure", "")
	result := parseToolResult(t, output)
	if len(result.Content) == 0 {
		t.Fatal("expected content in redacted response")
	}
	if !strings.Contains(result.Content[0].Text, "[REDACTED") {
		t.Fatalf("expected redacted output, got: %q", result.Content[0].Text)
	}
}
