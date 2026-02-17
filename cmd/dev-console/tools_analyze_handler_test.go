// tools_analyze_handler_test.go — Comprehensive unit tests for analyze tool dispatch and response fields.
// Validates all response fields, snake_case JSON convention, and dispatch logic.
package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/dev-console/dev-console/internal/capture"
)

// ============================================
// Test Helpers
// ============================================

func makeAnalyzeToolHandler(t *testing.T) (*ToolHandler, *Server, *capture.Capture) {
	t.Helper()
	server, err := NewServer(t.TempDir()+"/test.jsonl", 100)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	t.Cleanup(func() { server.Close() })
	cap := capture.NewCapture()
	mcpHandler := NewToolHandler(server, cap)
	handler := mcpHandler.toolHandler.(*ToolHandler)
	return handler, server, cap
}

func callAnalyzeRaw(h *ToolHandler, argsJSON string) JSONRPCResponse {
	req := JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	return h.toolAnalyze(req, json.RawMessage(argsJSON))
}

// ============================================
// Dispatch Tests
// ============================================

func TestToolsAnalyzeDispatch_InvalidJSON(t *testing.T) {
	t.Parallel()
	h, _, _ := makeAnalyzeToolHandler(t)

	resp := callAnalyzeRaw(h, `{bad json`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("invalid JSON should return isError:true")
	}
	if !strings.Contains(result.Content[0].Text, "invalid_json") {
		t.Errorf("error code should be 'invalid_json', got: %s", result.Content[0].Text)
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsAnalyzeDispatch_MissingWhat(t *testing.T) {
	t.Parallel()
	h, _, _ := makeAnalyzeToolHandler(t)

	resp := callAnalyzeRaw(h, `{}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("missing 'what' should return isError:true")
	}
	if !strings.Contains(result.Content[0].Text, "missing_param") {
		t.Errorf("error code should be 'missing_param', got: %s", result.Content[0].Text)
	}
	// Verify hint lists valid modes
	text := result.Content[0].Text
	for _, mode := range []string{"dom", "performance", "accessibility"} {
		if !strings.Contains(text, mode) {
			t.Errorf("hint should list valid mode %q", mode)
		}
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsAnalyzeDispatch_UnknownMode(t *testing.T) {
	t.Parallel()
	h, _, _ := makeAnalyzeToolHandler(t)

	resp := callAnalyzeRaw(h, `{"what":"nonexistent_mode"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("unknown mode should return isError:true")
	}
	if !strings.Contains(result.Content[0].Text, "unknown_mode") {
		t.Errorf("error code should be 'unknown_mode', got: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "nonexistent_mode") {
		t.Error("error should mention the invalid mode name")
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsAnalyzeDispatch_EmptyArgs(t *testing.T) {
	t.Parallel()
	h, _, _ := makeAnalyzeToolHandler(t)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := h.toolAnalyze(req, nil)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("nil args (no 'what') should return isError:true")
	}
}

// ============================================
// getValidAnalyzeModes Tests
// ============================================

func TestToolsAnalyze_GetValidAnalyzeModes(t *testing.T) {
	t.Parallel()

	modes := getValidAnalyzeModes()
	modeList := strings.Split(modes, ", ")
	for i := 1; i < len(modeList); i++ {
		if modeList[i-1] > modeList[i] {
			t.Errorf("modes not sorted: %q > %q", modeList[i-1], modeList[i])
		}
	}

	for _, required := range []string{"dom", "performance", "accessibility", "api_validation", "link_health", "page_summary"} {
		if !strings.Contains(modes, required) {
			t.Errorf("valid modes missing %q: %s", required, modes)
		}
	}
}

func TestToolsAnalyzeSchema_HasFrameParam(t *testing.T) {
	t.Parallel()
	h, _, _ := makeAnalyzeToolHandler(t)

	tools := h.ToolsList()
	var analyzeSchema map[string]any
	for _, tool := range tools {
		if tool.Name == "analyze" {
			analyzeSchema = tool.InputSchema
			break
		}
	}
	if analyzeSchema == nil {
		t.Fatal("analyze tool not found in ToolsList()")
	}

	props, ok := analyzeSchema["properties"].(map[string]any)
	if !ok {
		t.Fatal("analyze schema missing properties")
	}
	frameParam, exists := props["frame"]
	if !exists {
		t.Fatal("analyze schema missing 'frame' property")
	}
	frameMap, ok := frameParam.(map[string]any)
	if !ok {
		t.Fatal("frame property is not an object")
	}
	if _, ok := frameMap["oneOf"]; !ok {
		t.Fatal("frame property should declare oneOf (string | number)")
	}
}

func TestToolsAnalyzeSchema_HasPierceShadowParam(t *testing.T) {
	t.Parallel()
	h, _, _ := makeAnalyzeToolHandler(t)

	tools := h.ToolsList()
	var analyzeSchema map[string]any
	for _, tool := range tools {
		if tool.Name == "analyze" {
			analyzeSchema = tool.InputSchema
			break
		}
	}
	if analyzeSchema == nil {
		t.Fatal("analyze tool not found in ToolsList()")
	}

	props, ok := analyzeSchema["properties"].(map[string]any)
	if !ok {
		t.Fatal("analyze schema missing properties")
	}
	pierceParam, exists := props["pierce_shadow"]
	if !exists {
		t.Fatal("analyze schema missing 'pierce_shadow' property")
	}
	pierceMap, ok := pierceParam.(map[string]any)
	if !ok {
		t.Fatal("pierce_shadow property is not an object")
	}
	// ToolsList() returns native Go types ([]map[string]any), not JSON-unmarshaled ([]any).
	// Try native type first, then fall back to JSON-unmarshaled type.
	var items []map[string]any
	if nativeOneOf, ok := pierceMap["oneOf"].([]map[string]any); ok {
		items = nativeOneOf
	} else if jsonOneOf, ok := pierceMap["oneOf"].([]any); ok {
		for _, raw := range jsonOneOf {
			if m, ok := raw.(map[string]any); ok {
				items = append(items, m)
			}
		}
	}
	if len(items) != 2 {
		t.Fatalf("pierce_shadow.oneOf should have 2 variants, got %#v", pierceMap["oneOf"])
	}

	hasBoolean := false
	hasAutoString := false
	for _, item := range items {
		typeStr, _ := item["type"].(string)
		if typeStr == "boolean" {
			hasBoolean = true
		}
		if typeStr == "string" {
			// enum can be []string (native Go) or []any (JSON-unmarshaled)
			if nativeEnum, ok := item["enum"].([]string); ok && len(nativeEnum) == 1 && nativeEnum[0] == "auto" {
				hasAutoString = true
			} else if jsonEnum, ok := item["enum"].([]any); ok && len(jsonEnum) == 1 {
				if s, ok := jsonEnum[0].(string); ok && s == "auto" {
					hasAutoString = true
				}
			}
		}
	}
	if !hasBoolean {
		t.Fatal("pierce_shadow.oneOf missing boolean variant")
	}
	if !hasAutoString {
		t.Fatal("pierce_shadow.oneOf missing string enum ['auto'] variant")
	}
}

func TestToolsAnalyzePageSummary_QueuedAsync(t *testing.T) {
	t.Parallel()
	h, _, _ := makeAnalyzeToolHandler(t)

	resp := callAnalyzeRaw(h, `{"what":"page_summary","sync":false}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("page_summary with sync=false should queue, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	if data["status"] != "queued" {
		t.Errorf("status = %v, want 'queued'", data["status"])
	}
	corr, _ := data["correlation_id"].(string)
	if !strings.HasPrefix(corr, "page_summary_") {
		t.Errorf("correlation_id should start with 'page_summary_', got: %s", corr)
	}
}

func TestToolsAnalyzePageSummary_InvalidWorld(t *testing.T) {
	t.Parallel()
	h, _, _ := makeAnalyzeToolHandler(t)

	resp := callAnalyzeRaw(h, `{"what":"page_summary","sync":false,"world":"bad_world"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("invalid world should return isError:true")
	}
	if !strings.Contains(result.Content[0].Text, "world") {
		t.Errorf("error should mention 'world', got: %s", result.Content[0].Text)
	}
}

// ============================================
// analyze(what:"dom") — Response Fields
// ============================================

func TestToolsAnalyzeDOM_MissingSelector(t *testing.T) {
	t.Parallel()
	h, _, _ := makeAnalyzeToolHandler(t)

	resp := callAnalyzeRaw(h, `{"what":"dom"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("dom without selector should return isError:true")
	}
	if !strings.Contains(result.Content[0].Text, "selector") {
		t.Error("error should mention 'selector' parameter")
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsAnalyzeDOM_Success(t *testing.T) {
	t.Parallel()
	h, _, _ := makeAnalyzeToolHandler(t)

	resp := callAnalyzeRaw(h, `{"what":"dom","selector":"#main"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("dom should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	for _, field := range []string{"status", "correlation_id", "selector", "hint"} {
		if _, ok := data[field]; !ok {
			t.Errorf("dom response missing field %q", field)
		}
	}
	if data["status"] != "queued" {
		t.Errorf("status = %v, want 'queued'", data["status"])
	}
	if data["selector"] != "#main" {
		t.Errorf("selector = %v, want '#main'", data["selector"])
	}
	corr, _ := data["correlation_id"].(string)
	if !strings.HasPrefix(corr, "dom_") {
		t.Errorf("correlation_id should start with 'dom_', got: %s", corr)
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsAnalyzeDOM_FrameSelectorForwardedInPendingQuery(t *testing.T) {
	t.Parallel()
	h, _, cap := makeAnalyzeToolHandler(t)

	resp := callAnalyzeRaw(h, `{"what":"dom","selector":"#main","frame":"iframe.editor","sync":false}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("dom with frame selector should succeed, got: %s", result.Content[0].Text)
	}

	pq := cap.GetLastPendingQuery()
	if pq == nil {
		t.Fatal("expected pending query to be created")
	}

	var params map[string]any
	if err := json.Unmarshal(pq.Params, &params); err != nil {
		t.Fatalf("failed to parse pending query params: %v", err)
	}

	if got, ok := params["frame"].(string); !ok || got != "iframe.editor" {
		t.Fatalf("frame selector not forwarded correctly, got %#v", params["frame"])
	}
}

func TestToolsAnalyzeDOM_FrameIndexForwardedInPendingQuery(t *testing.T) {
	t.Parallel()
	h, _, cap := makeAnalyzeToolHandler(t)

	resp := callAnalyzeRaw(h, `{"what":"dom","selector":"#main","frame":0,"sync":false}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("dom with frame index should succeed, got: %s", result.Content[0].Text)
	}

	pq := cap.GetLastPendingQuery()
	if pq == nil {
		t.Fatal("expected pending query to be created")
	}

	var params map[string]any
	if err := json.Unmarshal(pq.Params, &params); err != nil {
		t.Fatalf("failed to parse pending query params: %v", err)
	}

	if got, ok := params["frame"].(float64); !ok || got != 0 {
		t.Fatalf("frame index not forwarded correctly, got %#v", params["frame"])
	}
}

func TestToolsAnalyzeDOM_PierceShadowTrueForwardedInPendingQuery(t *testing.T) {
	t.Parallel()
	h, _, cap := makeAnalyzeToolHandler(t)

	resp := callAnalyzeRaw(h, `{"what":"dom","selector":"#main","pierce_shadow":true,"sync":false}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("dom with pierce_shadow=true should succeed, got: %s", result.Content[0].Text)
	}

	pq := cap.GetLastPendingQuery()
	if pq == nil {
		t.Fatal("expected pending query to be created")
	}

	var params map[string]any
	if err := json.Unmarshal(pq.Params, &params); err != nil {
		t.Fatalf("failed to parse pending query params: %v", err)
	}

	if got, ok := params["pierce_shadow"].(bool); !ok || !got {
		t.Fatalf("pierce_shadow=true not forwarded correctly, got %#v", params["pierce_shadow"])
	}
}

func TestToolsAnalyzeDOM_PierceShadowFalseForwardedInPendingQuery(t *testing.T) {
	t.Parallel()
	h, _, cap := makeAnalyzeToolHandler(t)

	resp := callAnalyzeRaw(h, `{"what":"dom","selector":"#main","pierce_shadow":false,"sync":false}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("dom with pierce_shadow=false should succeed, got: %s", result.Content[0].Text)
	}

	pq := cap.GetLastPendingQuery()
	if pq == nil {
		t.Fatal("expected pending query to be created")
	}

	var params map[string]any
	if err := json.Unmarshal(pq.Params, &params); err != nil {
		t.Fatalf("failed to parse pending query params: %v", err)
	}

	if got, ok := params["pierce_shadow"].(bool); !ok || got {
		t.Fatalf("pierce_shadow=false not forwarded correctly, got %#v", params["pierce_shadow"])
	}
}

func TestToolsAnalyzeDOM_PierceShadowAutoForwardedInPendingQuery(t *testing.T) {
	t.Parallel()
	h, _, cap := makeAnalyzeToolHandler(t)

	resp := callAnalyzeRaw(h, `{"what":"dom","selector":"#main","pierce_shadow":"auto","sync":false}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("dom with pierce_shadow='auto' should succeed, got: %s", result.Content[0].Text)
	}

	pq := cap.GetLastPendingQuery()
	if pq == nil {
		t.Fatal("expected pending query to be created")
	}

	var params map[string]any
	if err := json.Unmarshal(pq.Params, &params); err != nil {
		t.Fatalf("failed to parse pending query params: %v", err)
	}

	if got, ok := params["pierce_shadow"].(string); !ok || got != "auto" {
		t.Fatalf("pierce_shadow='auto' not forwarded correctly, got %#v", params["pierce_shadow"])
	}
}

func TestToolsAnalyzeDOM_InvalidPierceShadowStringRejected(t *testing.T) {
	t.Parallel()
	h, _, _ := makeAnalyzeToolHandler(t)

	resp := callAnalyzeRaw(h, `{"what":"dom","selector":"#main","pierce_shadow":"sometimes","sync":false}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("invalid pierce_shadow value should return isError:true")
	}
	text := strings.ToLower(result.Content[0].Text)
	if !strings.Contains(text, "pierce_shadow") {
		t.Fatalf("error should mention pierce_shadow, got: %s", result.Content[0].Text)
	}
	if !strings.Contains(text, "invalid_param") {
		t.Fatalf("error should use invalid_param code, got: %s", result.Content[0].Text)
	}
}

func TestToolsAnalyzeDOM_InvalidPierceShadowTypeRejected(t *testing.T) {
	t.Parallel()
	h, _, _ := makeAnalyzeToolHandler(t)

	resp := callAnalyzeRaw(h, `{"what":"dom","selector":"#main","pierce_shadow":7,"sync":false}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("numeric pierce_shadow should return isError:true")
	}
	text := strings.ToLower(result.Content[0].Text)
	if !strings.Contains(text, "pierce_shadow") {
		t.Fatalf("error should mention pierce_shadow, got: %s", result.Content[0].Text)
	}
	if !strings.Contains(text, "invalid_param") {
		t.Fatalf("error should use invalid_param code, got: %s", result.Content[0].Text)
	}
}

// ============================================
// analyze(what:"api_validation") — Response Fields
// ============================================

func TestToolsAnalyzeAPIValidation_Analyze(t *testing.T) {
	t.Parallel()
	h, _, _ := makeAnalyzeToolHandler(t)

	resp := callAnalyzeRaw(h, `{"what":"api_validation","operation":"analyze"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("api_validation analyze should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	if data["status"] != "ok" {
		t.Errorf("status = %v, want 'ok'", data["status"])
	}
	if data["operation"] != "analyze" {
		t.Errorf("operation = %v, want 'analyze'", data["operation"])
	}
	if _, ok := data["violations"]; !ok {
		t.Error("response missing 'violations' field")
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsAnalyzeAPIValidation_Report(t *testing.T) {
	t.Parallel()
	h, _, _ := makeAnalyzeToolHandler(t)

	resp := callAnalyzeRaw(h, `{"what":"api_validation","operation":"report"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("api_validation report should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	if data["status"] != "ok" {
		t.Errorf("status = %v, want 'ok'", data["status"])
	}
	if data["operation"] != "report" {
		t.Errorf("operation = %v, want 'report'", data["operation"])
	}
	if _, ok := data["endpoints"]; !ok {
		t.Error("response missing 'endpoints' field")
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsAnalyzeAPIValidation_Clear(t *testing.T) {
	t.Parallel()
	h, _, _ := makeAnalyzeToolHandler(t)

	resp := callAnalyzeRaw(h, `{"what":"api_validation","operation":"clear"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("api_validation clear should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	if data["action"] != "cleared" {
		t.Errorf("action = %v, want 'cleared'", data["action"])
	}
	if data["status"] != "ok" {
		t.Errorf("status = %v, want 'ok'", data["status"])
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsAnalyzeAPIValidation_InvalidOperation(t *testing.T) {
	t.Parallel()
	h, _, _ := makeAnalyzeToolHandler(t)

	resp := callAnalyzeRaw(h, `{"what":"api_validation","operation":"invalid_op"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("api_validation with invalid operation should return isError:true")
	}
	if !strings.Contains(result.Content[0].Text, "operation") {
		t.Error("error should mention 'operation' parameter")
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsAnalyzeAPIValidation_DefaultOperation(t *testing.T) {
	t.Parallel()
	h, _, _ := makeAnalyzeToolHandler(t)

	// No operation param defaults to empty string, which is invalid
	resp := callAnalyzeRaw(h, `{"what":"api_validation"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("api_validation without operation should return isError:true")
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

// ============================================
// analyze(what:"performance") — Response Fields
// ============================================

func TestToolsAnalyzePerformance_Empty(t *testing.T) {
	t.Parallel()
	h, _, _ := makeAnalyzeToolHandler(t)

	resp := callAnalyzeRaw(h, `{"what":"performance"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("performance should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	for _, field := range []string{"snapshots", "count"} {
		if _, ok := data[field]; !ok {
			t.Errorf("performance response missing field %q", field)
		}
	}
	count, _ := data["count"].(float64)
	if count != 0 {
		t.Errorf("count = %v, want 0 (empty)", count)
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

// ============================================
// analyze(what:"link_health") — Response Fields
// ============================================

func TestToolsAnalyzeLinkHealth_ResponseFields(t *testing.T) {
	t.Parallel()
	h, _, _ := makeAnalyzeToolHandler(t)

	resp := callAnalyzeRaw(h, `{"what":"link_health"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("link_health should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	if data["status"] != "queued" {
		t.Errorf("status = %v, want 'queued'", data["status"])
	}
	corr, _ := data["correlation_id"].(string)
	if !strings.HasPrefix(corr, "link_health_") {
		t.Errorf("correlation_id should start with 'link_health_', got: %s", corr)
	}
	if _, ok := data["hint"]; !ok {
		t.Error("response missing 'hint' field")
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

// ============================================
// analyze(what:"link_validation") — Response Fields
// ============================================

func TestToolsAnalyzeLinkValidation_MissingURLs(t *testing.T) {
	t.Parallel()
	h, _, _ := makeAnalyzeToolHandler(t)

	resp := callAnalyzeRaw(h, `{"what":"link_validation"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("link_validation without urls should return isError:true")
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsAnalyzeLinkValidation_EmptyURLs(t *testing.T) {
	t.Parallel()
	h, _, _ := makeAnalyzeToolHandler(t)

	resp := callAnalyzeRaw(h, `{"what":"link_validation","urls":[]}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("link_validation with empty urls should return isError:true")
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsAnalyzeLinkValidation_NonHTTPURLs(t *testing.T) {
	t.Parallel()
	h, _, _ := makeAnalyzeToolHandler(t)

	resp := callAnalyzeRaw(h, `{"what":"link_validation","urls":["ftp://files.example.com","mailto:test@test.com"]}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("link_validation with non-HTTP urls should return isError:true")
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

// ============================================
// Helper function tests
// ============================================

func TestToolsAnalyze_ClampInt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		v, def, min, max, want int
	}{
		{0, 10, 1, 100, 10},    // zero uses default
		{5, 10, 1, 100, 5},     // in range
		{-1, 10, 1, 100, 1},    // below min
		{200, 10, 1, 100, 100}, // above max
		{50, 10, 1, 100, 50},   // in range
	}

	for _, tc := range tests {
		got := clampInt(tc.v, tc.def, tc.min, tc.max)
		if got != tc.want {
			t.Errorf("clampInt(%d, %d, %d, %d) = %d, want %d",
				tc.v, tc.def, tc.min, tc.max, got, tc.want)
		}
	}
}

func TestToolsAnalyze_FilterHTTPURLs(t *testing.T) {
	t.Parallel()

	urls := []string{
		"https://example.com",
		"http://example.com",
		"ftp://example.com",
		"mailto:test@example.com",
		"javascript:void(0)",
		"https://other.com/path",
	}

	filtered := filterHTTPURLs(urls)
	if len(filtered) != 3 {
		t.Errorf("filterHTTPURLs len = %d, want 3", len(filtered))
	}
	for _, u := range filtered {
		if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
			t.Errorf("filtered URL %q should start with http:// or https://", u)
		}
	}
}

func TestToolsAnalyze_ClassifyHTTPStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status int
		want   string
	}{
		{200, "ok"},
		{201, "ok"},
		{299, "ok"},
		{301, "redirect"},
		{302, "redirect"},
		{399, "redirect"},
		{401, "requires_auth"},
		{403, "requires_auth"},
		{404, "broken"},
		{500, "broken"},
		{100, "broken"},
	}

	for _, tc := range tests {
		got := classifyHTTPStatus(tc.status)
		if got != tc.want {
			t.Errorf("classifyHTTPStatus(%d) = %q, want %q", tc.status, got, tc.want)
		}
	}
}

// ============================================
// All analyze modes safety net
// ============================================

func TestToolsAnalyze_AllModes_ResponseStructure(t *testing.T) {
	t.Parallel()
	h, _, _ := makeAnalyzeToolHandler(t)

	// All modes from analyzeHandlers that can run without extension
	modes := []struct {
		what string
		args string
	}{
		{"dom", `{"what":"dom","selector":"div"}`},
		{"api_validation", `{"what":"api_validation","operation":"analyze"}`},
		{"performance", `{"what":"performance"}`},
		{"link_health", `{"what":"link_health"}`},
		{"page_summary", `{"what":"page_summary","sync":false}`},
	}

	for _, tc := range modes {
		t.Run(tc.what, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("analyze(%s) PANICKED: %v", tc.what, r)
				}
			}()

			resp := callAnalyzeRaw(h, tc.args)
			if resp.Result == nil {
				t.Fatalf("analyze(%s) returned nil result", tc.what)
			}

			result := parseToolResult(t, resp)
			if len(result.Content) == 0 {
				t.Errorf("analyze(%s) should return at least one content block", tc.what)
			}

			// Verify content type is "text"
			if result.Content[0].Type != "text" {
				t.Errorf("analyze(%s) content type = %q, want 'text'", tc.what, result.Content[0].Type)
			}

			assertSnakeCaseFields(t, string(resp.Result))
		})
	}
}
