// cli_test.go — Tests for CLI mode: argument parsing, output formatting, and end-to-end flow.
package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// --- IsCLIMode tests ---

func TestIsCLIMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want bool
	}{
		{"observe tool", []string{"observe", "errors"}, true},
		{"analyze tool", []string{"analyze", "dom"}, true},
		{"generate tool", []string{"generate", "har"}, true},
		{"configure tool", []string{"configure", "health"}, true},
		{"interact tool", []string{"interact", "click", "--selector", "#btn"}, true},
		{"flag --version", []string{"--version"}, false},
		{"flag --help", []string{"--help"}, false},
		{"flag --port", []string{"--port", "8080"}, false},
		{"flag --daemon", []string{"--daemon"}, false},
		{"flag --bridge", []string{"--bridge"}, false},
		{"flag --stop", []string{"--stop"}, false},
		{"flag --force", []string{"--force"}, false},
		{"flag --check", []string{"--check"}, false},
		{"flag --doctor", []string{"--doctor"}, false},
		{"flag --fastpath-min-samples", []string{"--fastpath-min-samples", "20"}, false},
		{"flag --fastpath-max-failure-ratio", []string{"--fastpath-max-failure-ratio", "0.05"}, false},
		{"flag --connect", []string{"--connect"}, false},
		{"empty args", []string{}, false},
		{"unknown word", []string{"foobar"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := IsCLIMode(tt.args)
			if got != tt.want {
				t.Errorf("IsCLIMode(%v) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}

// --- resolveCLIConfig tests ---

func TestResolveCLIConfigDefaults(t *testing.T) {
	t.Parallel()

	cfg, remaining := resolveCLIConfig([]string{"observe", "errors"})
	if cfg.Port != defaultPort {
		t.Errorf("expected port %d, got %d", defaultPort, cfg.Port)
	}
	if cfg.Format != "human" {
		t.Errorf("expected format 'human', got %q", cfg.Format)
	}
	if cfg.Timeout != 15000 {
		t.Errorf("expected timeout 15000, got %d", cfg.Timeout)
	}
	if len(remaining) != 2 || remaining[0] != "observe" || remaining[1] != "errors" {
		t.Errorf("expected remaining [observe errors], got %v", remaining)
	}
}

func TestResolveCLIConfigFlagOverrides(t *testing.T) {
	t.Parallel()

	cfg, remaining := resolveCLIConfig([]string{"--port", "9999", "--format", "json", "--timeout", "10000", "observe", "errors"})
	if cfg.Port != 9999 {
		t.Errorf("expected port 9999, got %d", cfg.Port)
	}
	if cfg.Format != "json" {
		t.Errorf("expected format 'json', got %q", cfg.Format)
	}
	if cfg.Timeout != 10000 {
		t.Errorf("expected timeout 10000, got %d", cfg.Timeout)
	}
	if len(remaining) != 2 || remaining[0] != "observe" || remaining[1] != "errors" {
		t.Errorf("expected remaining [observe errors], got %v", remaining)
	}
}

func TestResolveCLIConfigEnvOverrides(t *testing.T) {
	t.Setenv("GASOLINE_PORT", "8888")
	t.Setenv("GASOLINE_FORMAT", "csv")

	cfg, _ := resolveCLIConfig([]string{"observe", "errors"})
	if cfg.Port != 8888 {
		t.Errorf("expected port 8888, got %d", cfg.Port)
	}
	if cfg.Format != "csv" {
		t.Errorf("expected format 'csv', got %q", cfg.Format)
	}
}

func TestResolveCLIConfigFlagBeatsEnv(t *testing.T) {
	t.Setenv("GASOLINE_PORT", "8888")

	cfg, _ := resolveCLIConfig([]string{"--port", "9999", "observe", "errors"})
	if cfg.Port != 9999 {
		t.Errorf("expected port 9999 (flag beats env), got %d", cfg.Port)
	}
}

// --- normalizeAction tests ---

func TestNormalizeAction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{"click", "click"},
		{"get-text", "get_text"},
		{"get-value", "get_value"},
		{"network-waterfall", "network_waterfall"},
		{"network-bodies", "network_bodies"},
		{"noise-rule", "noise_rule"},
		{"execute-js", "execute_js"},
		{"key-press", "key_press"},
		{"scroll-to", "scroll_to"},
		{"wait-for", "wait_for"},
		{"set-attribute", "set_attribute"},
		{"get-attribute", "get_attribute"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got := normalizeAction(tt.input)
			if got != tt.want {
				t.Errorf("normalizeAction(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// --- parseObserveArgs tests ---

func TestParseObserveArgsErrors(t *testing.T) {
	t.Parallel()

	mcpArgs, err := parseObserveArgs("errors", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mcpArgs["what"] != "errors" {
		t.Errorf("expected what 'errors', got %v", mcpArgs["what"])
	}
}

func TestParseObserveArgsWithLimit(t *testing.T) {
	t.Parallel()

	mcpArgs, err := parseObserveArgs("errors", []string{"--limit", "50"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mcpArgs["limit"] != 50 {
		t.Errorf("expected limit 50, got %v", mcpArgs["limit"])
	}
}

func TestParseObserveArgsLogs(t *testing.T) {
	t.Parallel()

	mcpArgs, err := parseObserveArgs("logs", []string{"--min-level", "warn", "--limit", "100"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mcpArgs["what"] != "logs" {
		t.Errorf("expected what 'logs', got %v", mcpArgs["what"])
	}
	if mcpArgs["min_level"] != "warn" {
		t.Errorf("expected min_level 'warn', got %v", mcpArgs["min_level"])
	}
	if mcpArgs["limit"] != 100 {
		t.Errorf("expected limit 100, got %v", mcpArgs["limit"])
	}
}

func TestParseObserveArgsNetworkWaterfall(t *testing.T) {
	t.Parallel()

	mcpArgs, err := parseObserveArgs("network_waterfall", []string{"--url", "api.example.com", "--status-min", "400"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mcpArgs["url"] != "api.example.com" {
		t.Errorf("expected url 'api.example.com', got %v", mcpArgs["url"])
	}
	if mcpArgs["status_min"] != 400 {
		t.Errorf("expected status_min 400, got %v", mcpArgs["status_min"])
	}
}

func TestParseObserveArgsAccessibility(t *testing.T) {
	t.Parallel()

	mcpArgs, err := parseObserveArgs("accessibility", []string{"--scope", "form"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mcpArgs["scope"] != "form" {
		t.Errorf("expected scope 'form', got %v", mcpArgs["scope"])
	}
}

func TestParseObserveArgsWebsocket(t *testing.T) {
	t.Parallel()

	mcpArgs, err := parseObserveArgs("websocket_events", []string{"--connection-id", "ws1", "--direction", "sent"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mcpArgs["connection_id"] != "ws1" {
		t.Errorf("expected connection_id 'ws1', got %v", mcpArgs["connection_id"])
	}
	if mcpArgs["direction"] != "sent" {
		t.Errorf("expected direction 'sent', got %v", mcpArgs["direction"])
	}
}

// --- parseGenerateArgs tests ---

func TestParseGenerateArgsHAR(t *testing.T) {
	t.Parallel()

	mcpArgs, err := parseGenerateArgs("har", []string{"--save-to", "out.har"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mcpArgs["format"] != "har" {
		t.Errorf("expected format 'har', got %v", mcpArgs["format"])
	}
	if mcpArgs["save_to"] != "out.har" {
		t.Errorf("expected save_to 'out.har', got %v", mcpArgs["save_to"])
	}
}

func TestParseGenerateArgsTest(t *testing.T) {
	t.Parallel()

	mcpArgs, err := parseGenerateArgs("test", []string{"--test-name", "my_test", "--assert-network", "--assert-no-errors"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mcpArgs["test_name"] != "my_test" {
		t.Errorf("expected test_name 'my_test', got %v", mcpArgs["test_name"])
	}
	if mcpArgs["assert_network"] != true {
		t.Errorf("expected assert_network true, got %v", mcpArgs["assert_network"])
	}
	if mcpArgs["assert_no_errors"] != true {
		t.Errorf("expected assert_no_errors true, got %v", mcpArgs["assert_no_errors"])
	}
}

func TestParseGenerateArgsCSP(t *testing.T) {
	t.Parallel()

	mcpArgs, err := parseGenerateArgs("csp", []string{"--mode", "strict"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mcpArgs["mode"] != "strict" {
		t.Errorf("expected mode 'strict', got %v", mcpArgs["mode"])
	}
}

func TestParseGenerateArgsReproduction(t *testing.T) {
	t.Parallel()

	mcpArgs, err := parseGenerateArgs("reproduction", []string{"--error-message", "timeout", "--last-n", "5"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mcpArgs["error_message"] != "timeout" {
		t.Errorf("expected error_message 'timeout', got %v", mcpArgs["error_message"])
	}
	if mcpArgs["last_n"] != 5 {
		t.Errorf("expected last_n 5, got %v", mcpArgs["last_n"])
	}
}

// --- parseConfigureArgs tests ---

func TestParseConfigureArgsHealth(t *testing.T) {
	t.Parallel()

	mcpArgs, err := parseConfigureArgs("health", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mcpArgs["action"] != "health" {
		t.Errorf("expected action 'health', got %v", mcpArgs["action"])
	}
}

func TestParseConfigureArgsClear(t *testing.T) {
	t.Parallel()

	mcpArgs, err := parseConfigureArgs("clear", []string{"--buffer", "network"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mcpArgs["buffer"] != "network" {
		t.Errorf("expected buffer 'network', got %v", mcpArgs["buffer"])
	}
}

func TestParseConfigureArgsNoiseRule(t *testing.T) {
	t.Parallel()

	mcpArgs, err := parseConfigureArgs("noise_rule", []string{"--noise-action", "add", "--pattern", "*.png", "--reason", "images"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mcpArgs["noise_action"] != "add" {
		t.Errorf("expected noise_action 'add', got %v", mcpArgs["noise_action"])
	}
	if mcpArgs["pattern"] != "*.png" {
		t.Errorf("expected pattern '*.png', got %v", mcpArgs["pattern"])
	}
}

func TestParseConfigureArgsStore(t *testing.T) {
	t.Parallel()

	mcpArgs, err := parseConfigureArgs("store", []string{"--key", "session", "--data", `{"id":"123"}`})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mcpArgs["key"] != "session" {
		t.Errorf("expected key 'session', got %v", mcpArgs["key"])
	}
	data, ok := mcpArgs["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data to be map, got %T: %v", mcpArgs["data"], mcpArgs["data"])
	}
	if data["id"] != "123" {
		t.Errorf("expected data.id '123', got %v", data["id"])
	}
}

// --- parseInteractArgs tests ---

func TestParseInteractArgsClick(t *testing.T) {
	t.Parallel()

	mcpArgs, err := parseInteractArgs("click", []string{"--selector", "#btn"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mcpArgs["action"] != "click" {
		t.Errorf("expected action 'click', got %v", mcpArgs["action"])
	}
	if mcpArgs["selector"] != "#btn" {
		t.Errorf("expected selector '#btn', got %v", mcpArgs["selector"])
	}
}

func TestParseInteractArgsClickMissingSelector(t *testing.T) {
	t.Parallel()

	_, err := parseInteractArgs("click", nil)
	if err == nil {
		t.Fatal("expected error for missing selector")
	}
	if !strings.Contains(err.Error(), "selector") {
		t.Errorf("expected error about missing selector, got: %v", err)
	}
}

func TestParseInteractArgsNavigate(t *testing.T) {
	t.Parallel()

	mcpArgs, err := parseInteractArgs("navigate", []string{"--url", "https://example.com"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mcpArgs["url"] != "https://example.com" {
		t.Errorf("expected url 'https://example.com', got %v", mcpArgs["url"])
	}
}

func TestParseInteractArgsNavigateMissingURL(t *testing.T) {
	t.Parallel()

	_, err := parseInteractArgs("navigate", nil)
	if err == nil {
		t.Fatal("expected error for missing url")
	}
	if !strings.Contains(err.Error(), "url") {
		t.Errorf("expected error about missing url, got: %v", err)
	}
}

func TestParseInteractArgsType(t *testing.T) {
	t.Parallel()

	mcpArgs, err := parseInteractArgs("type", []string{"--selector", "#input", "--text", "hello", "--clear"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mcpArgs["selector"] != "#input" {
		t.Errorf("expected selector '#input', got %v", mcpArgs["selector"])
	}
	if mcpArgs["text"] != "hello" {
		t.Errorf("expected text 'hello', got %v", mcpArgs["text"])
	}
	if mcpArgs["clear"] != true {
		t.Errorf("expected clear true, got %v", mcpArgs["clear"])
	}
}

func TestParseInteractArgsExecuteJS(t *testing.T) {
	t.Parallel()

	mcpArgs, err := parseInteractArgs("execute_js", []string{"--script", "document.title"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mcpArgs["script"] != "document.title" {
		t.Errorf("expected script 'document.title', got %v", mcpArgs["script"])
	}
}

func TestParseInteractArgsExecuteJSMissingScript(t *testing.T) {
	t.Parallel()

	_, err := parseInteractArgs("execute_js", nil)
	if err == nil {
		t.Fatal("expected error for missing script")
	}
	if !strings.Contains(err.Error(), "script") {
		t.Errorf("expected error about missing script, got: %v", err)
	}
}

func TestParseInteractArgsKeyPressMissingSelector(t *testing.T) {
	t.Parallel()

	_, err := parseInteractArgs("key_press", []string{"--text", "Enter"})
	if err == nil {
		t.Fatal("expected error for missing selector")
	}
	if !strings.Contains(err.Error(), "selector") {
		t.Errorf("expected error about missing selector, got: %v", err)
	}
}

func TestParseInteractArgsKebabCase(t *testing.T) {
	t.Parallel()

	// normalizeAction is called before parseInteractArgs in the real flow,
	// but parseInteractArgs itself receives the already-normalized action.
	// This tests the get-text kebab input being normalized first.
	action := normalizeAction("get-text")
	mcpArgs, err := parseInteractArgs(action, []string{"--selector", ".content"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mcpArgs["action"] != "get_text" {
		t.Errorf("expected action 'get_text', got %v", mcpArgs["action"])
	}
}

// --- parseCLIArgs dispatch tests ---

func TestParseCLIArgsDispatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		tool    string
		action  string
		args    []string
		wantKey string
		wantVal any
		wantErr bool
	}{
		{"observe errors", "observe", "errors", nil, "what", "errors", false},
		{"analyze dom", "analyze", "dom", nil, "what", "dom", false},
		{"generate har", "generate", "har", nil, "format", "har", false},
		{"configure health", "configure", "health", nil, "action", "health", false},
		{"interact click", "interact", "click", []string{"--selector", "#btn"}, "action", "click", false},
		{"unknown tool", "foobar", "x", nil, "", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mcpArgs, err := parseCLIArgs(tt.tool, tt.action, tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if mcpArgs[tt.wantKey] != tt.wantVal {
				t.Errorf("expected %s=%v, got %v", tt.wantKey, tt.wantVal, mcpArgs[tt.wantKey])
			}
		})
	}
}

// --- Format tests ---

func TestFormatHuman(t *testing.T) {
	t.Parallel()

	result := &MCPToolResult{
		Content: []MCPContentBlock{{Type: "text", Text: `{"entries":[],"total":0}`}},
	}
	cliRes := buildCLIResult("observe", "errors", result)

	var buf bytes.Buffer
	err := formatHuman(&buf, cliRes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "[OK]") {
		t.Errorf("expected [OK] in output, got: %s", out)
	}
	if !strings.Contains(out, "observe") {
		t.Errorf("expected 'observe' in output, got: %s", out)
	}
}

func TestFormatHumanError(t *testing.T) {
	t.Parallel()

	result := &MCPToolResult{
		Content: []MCPContentBlock{{Type: "text", Text: "Connection refused"}},
		IsError: true,
	}
	cliRes := buildCLIResult("observe", "errors", result)

	var buf bytes.Buffer
	err := formatHuman(&buf, cliRes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "[Error]") {
		t.Errorf("expected [Error] in output, got: %s", out)
	}
}

func TestFormatJSON(t *testing.T) {
	t.Parallel()

	result := &MCPToolResult{
		Content: []MCPContentBlock{{Type: "text", Text: `{"status":"ok"}`}},
	}
	cliRes := buildCLIResult("configure", "health", result)

	var buf bytes.Buffer
	err := formatJSON(&buf, cliRes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if parsed["success"] != true {
		t.Errorf("expected success=true, got %v", parsed["success"])
	}
	if parsed["tool"] != "configure" {
		t.Errorf("expected tool=configure, got %v", parsed["tool"])
	}
}

func TestFormatCSV(t *testing.T) {
	t.Parallel()

	result := &MCPToolResult{
		Content: []MCPContentBlock{{Type: "text", Text: `{"count":5}`}},
	}
	cliRes := buildCLIResult("observe", "errors", result)

	var buf bytes.Buffer
	err := formatCSV(&buf, cliRes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected header + data row, got %d lines", len(lines))
	}
	// Header should contain success, tool, action
	if !strings.Contains(lines[0], "success") {
		t.Errorf("expected 'success' in CSV header, got: %s", lines[0])
	}
}

// --- End-to-end tests with mock HTTP server ---

func TestEndToEndWithMockServer(t *testing.T) {
	t.Parallel()

	// Mock MCP server that handles tools/call
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req JSONRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", 400)
			return
		}

		// Return a successful tool result
		result := MCPToolResult{
			Content: []MCPContentBlock{{Type: "text", Text: `{"entries":[],"total":0}`}},
		}
		resultJSON, _ := json.Marshal(result)
		resp := JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  resultJSON,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	mcpArgs := map[string]any{"what": "errors"}
	result, err := callTool(server.URL, "observe", mcpArgs, 5000)
	if err != nil {
		t.Fatalf("callTool error: %v", err)
	}
	if result.IsError {
		t.Error("expected success, got isError")
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content blocks")
	}
	if result.Content[0].Text != `{"entries":[],"total":0}` {
		t.Errorf("unexpected text: %s", result.Content[0].Text)
	}
}

func TestEndToEndToolError(t *testing.T) {
	t.Parallel()

	// Mock server that returns a tool error (isError: true)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req JSONRPCRequest
		json.NewDecoder(r.Body).Decode(&req)

		result := MCPToolResult{
			Content: []MCPContentBlock{{Type: "text", Text: "No data available"}},
			IsError: true,
		}
		resultJSON, _ := json.Marshal(result)
		resp := JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  resultJSON,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	mcpArgs := map[string]any{"what": "errors"}
	result, err := callTool(server.URL, "observe", mcpArgs, 5000)
	if err != nil {
		t.Fatalf("callTool error: %v", err)
	}
	if !result.IsError {
		t.Error("expected isError=true")
	}
}

func TestEndToEndJSONRPCError(t *testing.T) {
	t.Parallel()

	// Mock server that returns a JSON-RPC error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req JSONRPCRequest
		json.NewDecoder(r.Body).Decode(&req)

		resp := JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &JSONRPCError{Code: -32601, Message: "Method not found"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	mcpArgs := map[string]any{"what": "errors"}
	_, err := callTool(server.URL, "observe", mcpArgs, 5000)
	if err == nil {
		t.Fatal("expected error for JSON-RPC error response")
	}
	if !strings.Contains(err.Error(), "Method not found") {
		t.Errorf("expected 'Method not found' in error, got: %v", err)
	}
}

func TestEndToEndServerNotRunning(t *testing.T) {
	t.Parallel()

	// Use a URL that can't be reached (port 0 is never bound)
	_, err := callTool("http://127.0.0.1:0", "observe", map[string]any{"what": "errors"}, 1000)
	if err == nil {
		t.Fatal("expected error when server is not running")
	}
}

func TestEndToEndHTTPError(t *testing.T) {
	t.Parallel()

	// Mock server that returns HTTP 500
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", 500)
	}))
	defer server.Close()

	_, err := callTool(server.URL, "observe", map[string]any{"what": "errors"}, 5000)
	if err == nil {
		t.Fatal("expected error for HTTP 500")
	}
}

// --- formatResult exit code tests ---

func TestFormatResultExitCodeSuccess(t *testing.T) {
	result := &MCPToolResult{
		Content: []MCPContentBlock{{Type: "text", Text: "ok"}},
	}
	// Redirect stdout to /dev/null for writing
	oldStdout := os.Stdout
	devNull, _ := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	os.Stdout = devNull
	defer func() {
		os.Stdout = oldStdout
		_ = devNull.Close()
	}()

	code := formatResult("human", "observe", "errors", result)
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
}

func TestFormatResultExitCodeError(t *testing.T) {
	result := &MCPToolResult{
		Content: []MCPContentBlock{{Type: "text", Text: "failed"}},
		IsError: true,
	}
	oldStdout := os.Stdout
	devNull, _ := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	os.Stdout = devNull
	defer func() {
		os.Stdout = oldStdout
		_ = devNull.Close()
	}()

	code := formatResult("human", "observe", "errors", result)
	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
}
