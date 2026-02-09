// commands_test.go — Tests for command argument parsing and MCP argument building.
package commands

import (
	"encoding/json"
	"strings"
	"testing"
)

// --- Interact tests ---

func TestInteractParseClickArgs(t *testing.T) {
	t.Parallel()

	args := []string{"--selector", "#submit-btn"}
	mcpArgs, err := InteractArgs("click", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mcpArgs["action"] != "click" {
		t.Errorf("expected action 'click', got %v", mcpArgs["action"])
	}
	if mcpArgs["selector"] != "#submit-btn" {
		t.Errorf("expected selector '#submit-btn', got %v", mcpArgs["selector"])
	}
}

func TestInteractParseTypeArgs(t *testing.T) {
	t.Parallel()

	args := []string{"--selector", "#title", "--text", "My Video"}
	mcpArgs, err := InteractArgs("type", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mcpArgs["action"] != "type" {
		t.Errorf("expected action 'type', got %v", mcpArgs["action"])
	}
	if mcpArgs["selector"] != "#title" {
		t.Errorf("expected selector '#title', got %v", mcpArgs["selector"])
	}
	if mcpArgs["text"] != "My Video" {
		t.Errorf("expected text 'My Video', got %v", mcpArgs["text"])
	}
}

func TestInteractParseNavigateArgs(t *testing.T) {
	t.Parallel()

	args := []string{"--url", "https://example.com"}
	mcpArgs, err := InteractArgs("navigate", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mcpArgs["action"] != "navigate" {
		t.Errorf("expected action 'navigate', got %v", mcpArgs["action"])
	}
	if mcpArgs["url"] != "https://example.com" {
		t.Errorf("expected url 'https://example.com', got %v", mcpArgs["url"])
	}
}

func TestInteractParseGetAttributeArgs(t *testing.T) {
	t.Parallel()

	args := []string{"--selector", "a.link", "--name", "href"}
	mcpArgs, err := InteractArgs("get_attribute", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mcpArgs["action"] != "get_attribute" {
		t.Errorf("expected action 'get_attribute', got %v", mcpArgs["action"])
	}
	if mcpArgs["name"] != "href" {
		t.Errorf("expected name 'href', got %v", mcpArgs["name"])
	}
}

func TestInteractParseSetAttributeArgs(t *testing.T) {
	t.Parallel()

	args := []string{"--selector", "input", "--name", "disabled", "--value", "true"}
	mcpArgs, err := InteractArgs("set_attribute", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mcpArgs["value"] != "true" {
		t.Errorf("expected value 'true', got %v", mcpArgs["value"])
	}
}

func TestInteractParseWaitForArgs(t *testing.T) {
	t.Parallel()

	args := []string{"--selector", ".loading", "--timeout-ms", "5000"}
	mcpArgs, err := InteractArgs("wait_for", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mcpArgs["timeout_ms"] != 5000 {
		t.Errorf("expected timeout_ms 5000, got %v", mcpArgs["timeout_ms"])
	}
}

func TestInteractParseKeyPressArgs(t *testing.T) {
	t.Parallel()

	args := []string{"--text", "Enter"}
	mcpArgs, err := InteractArgs("key_press", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mcpArgs["text"] != "Enter" {
		t.Errorf("expected text 'Enter', got %v", mcpArgs["text"])
	}
}

func TestInteractParseExecuteJSArgs(t *testing.T) {
	t.Parallel()

	args := []string{"--script", "document.title"}
	mcpArgs, err := InteractArgs("execute_js", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mcpArgs["script"] != "document.title" {
		t.Errorf("expected script 'document.title', got %v", mcpArgs["script"])
	}
}

func TestInteractMissingSelectorForClick(t *testing.T) {
	t.Parallel()

	args := []string{}
	_, err := InteractArgs("click", args)
	if err == nil {
		t.Fatal("expected error for missing selector")
	}
	if !strings.Contains(err.Error(), "selector") {
		t.Errorf("expected error about missing selector, got: %v", err)
	}
}

func TestInteractFilePathMadeAbsolute(t *testing.T) {
	t.Parallel()

	args := []string{"--selector", "#file", "--file-path", "video.mp4"}
	mcpArgs, err := InteractArgs("upload", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	path, ok := mcpArgs["file_path"].(string)
	if !ok {
		t.Fatal("file_path not a string")
	}
	if !strings.HasPrefix(path, "/") {
		t.Errorf("expected absolute path, got: %s", path)
	}
}

// --- Observe tests ---

func TestObserveParseLogsArgs(t *testing.T) {
	t.Parallel()

	args := []string{"--limit", "50", "--min-level", "warn"}
	mcpArgs, err := ObserveArgs("logs", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mcpArgs["what"] != "logs" {
		t.Errorf("expected what 'logs', got %v", mcpArgs["what"])
	}
	if mcpArgs["limit"] != 50 {
		t.Errorf("expected limit 50, got %v", mcpArgs["limit"])
	}
	if mcpArgs["min_level"] != "warn" {
		t.Errorf("expected min_level 'warn', got %v", mcpArgs["min_level"])
	}
}

func TestObserveParseNetworkArgs(t *testing.T) {
	t.Parallel()

	args := []string{"--url", "api.example.com", "--status-min", "400", "--status-max", "599"}
	mcpArgs, err := ObserveArgs("network_waterfall", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mcpArgs["url"] != "api.example.com" {
		t.Errorf("expected url 'api.example.com', got %v", mcpArgs["url"])
	}
	if mcpArgs["status_min"] != 400 {
		t.Errorf("expected status_min 400, got %v", mcpArgs["status_min"])
	}
	if mcpArgs["status_max"] != 599 {
		t.Errorf("expected status_max 599, got %v", mcpArgs["status_max"])
	}
}

func TestObserveParseErrorsArgs(t *testing.T) {
	t.Parallel()

	args := []string{"--limit", "20"}
	mcpArgs, err := ObserveArgs("errors", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mcpArgs["what"] != "errors" {
		t.Errorf("expected what 'errors', got %v", mcpArgs["what"])
	}
}

func TestObserveParseAccessibilityArgs(t *testing.T) {
	t.Parallel()

	args := []string{"--scope", "form"}
	mcpArgs, err := ObserveArgs("accessibility", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mcpArgs["scope"] != "form" {
		t.Errorf("expected scope 'form', got %v", mcpArgs["scope"])
	}
}

func TestObserveParsePerformanceArgs(t *testing.T) {
	t.Parallel()

	args := []string{"--limit", "5"}
	mcpArgs, err := ObserveArgs("performance", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mcpArgs["what"] != "performance" {
		t.Errorf("expected what 'performance', got %v", mcpArgs["what"])
	}
}

// --- Configure tests ---

func TestConfigureParseNoiseRuleArgs(t *testing.T) {
	t.Parallel()

	args := []string{"--pattern", "favicon.ico", "--reason", "noise"}
	mcpArgs, err := ConfigureArgs("noise_rule", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mcpArgs["action"] != "noise_rule" {
		t.Errorf("expected action 'noise_rule', got %v", mcpArgs["action"])
	}
	if mcpArgs["pattern"] != "favicon.ico" {
		t.Errorf("expected pattern 'favicon.ico', got %v", mcpArgs["pattern"])
	}
	if mcpArgs["reason"] != "noise" {
		t.Errorf("expected reason 'noise', got %v", mcpArgs["reason"])
	}
}

func TestConfigureParseStoreArgs(t *testing.T) {
	t.Parallel()

	args := []string{"--key", "session", "--data", `{"id":"123"}`}
	mcpArgs, err := ConfigureArgs("store", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mcpArgs["action"] != "store" {
		t.Errorf("expected action 'store', got %v", mcpArgs["action"])
	}
	if mcpArgs["key"] != "session" {
		t.Errorf("expected key 'session', got %v", mcpArgs["key"])
	}

	// Data should be parsed JSON object
	data, ok := mcpArgs["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data to be map, got %T: %v", mcpArgs["data"], mcpArgs["data"])
	}
	if data["id"] != "123" {
		t.Errorf("expected data.id '123', got %v", data["id"])
	}
}

func TestConfigureParseLoadArgs(t *testing.T) {
	t.Parallel()

	args := []string{"--key", "session"}
	mcpArgs, err := ConfigureArgs("load", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mcpArgs["key"] != "session" {
		t.Errorf("expected key 'session', got %v", mcpArgs["key"])
	}
}

func TestConfigureParseClearArgs(t *testing.T) {
	t.Parallel()

	args := []string{"--buffer", "network"}
	mcpArgs, err := ConfigureArgs("clear", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mcpArgs["action"] != "clear" {
		t.Errorf("expected action 'clear', got %v", mcpArgs["action"])
	}
	if mcpArgs["buffer"] != "network" {
		t.Errorf("expected buffer 'network', got %v", mcpArgs["buffer"])
	}
}

func TestConfigureParseHealthArgs(t *testing.T) {
	t.Parallel()

	args := []string{}
	mcpArgs, err := ConfigureArgs("health", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mcpArgs["action"] != "health" {
		t.Errorf("expected action 'health', got %v", mcpArgs["action"])
	}
}

func TestConfigureParseNoiseRuleAdd(t *testing.T) {
	t.Parallel()

	args := []string{"--noise-action", "add", "--pattern", "*.png", "--reason", "images"}
	mcpArgs, err := ConfigureArgs("noise_rule", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mcpArgs["noise_action"] != "add" {
		t.Errorf("expected noise_action 'add', got %v", mcpArgs["noise_action"])
	}
}

// --- Generate tests ---

func TestGenerateParseTestArgs(t *testing.T) {
	t.Parallel()

	args := []string{"--test-name", "upload_test"}
	mcpArgs, err := GenerateArgs("test", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mcpArgs["format"] != "test" {
		t.Errorf("expected format 'test', got %v", mcpArgs["format"])
	}
	if mcpArgs["test_name"] != "upload_test" {
		t.Errorf("expected test_name 'upload_test', got %v", mcpArgs["test_name"])
	}
}

func TestGenerateParseReproductionArgs(t *testing.T) {
	t.Parallel()

	args := []string{"--error-message", "timeout after 5000ms"}
	mcpArgs, err := GenerateArgs("reproduction", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mcpArgs["format"] != "reproduction" {
		t.Errorf("expected format 'reproduction', got %v", mcpArgs["format"])
	}
	if mcpArgs["error_message"] != "timeout after 5000ms" {
		t.Errorf("expected error_message, got %v", mcpArgs["error_message"])
	}
}

func TestGenerateParseHARArgs(t *testing.T) {
	t.Parallel()

	args := []string{"--url", "api.example.com", "--method", "POST"}
	mcpArgs, err := GenerateArgs("har", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mcpArgs["url"] != "api.example.com" {
		t.Errorf("expected url 'api.example.com', got %v", mcpArgs["url"])
	}
	if mcpArgs["method"] != "POST" {
		t.Errorf("expected method 'POST', got %v", mcpArgs["method"])
	}
}

func TestGenerateParseCSPArgs(t *testing.T) {
	t.Parallel()

	args := []string{"--mode", "strict"}
	mcpArgs, err := GenerateArgs("csp", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mcpArgs["mode"] != "strict" {
		t.Errorf("expected mode 'strict', got %v", mcpArgs["mode"])
	}
}

func TestGenerateParseSARIFArgs(t *testing.T) {
	t.Parallel()

	args := []string{"--save-to", "/tmp/report.sarif"}
	mcpArgs, err := GenerateArgs("sarif", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mcpArgs["save_to"] != "/tmp/report.sarif" {
		t.Errorf("expected save_to '/tmp/report.sarif', got %v", mcpArgs["save_to"])
	}
}

// --- Action normalization tests ---

func TestNormalizeAction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected string
	}{
		{"click", "click"},
		{"get-text", "get_text"},
		{"get-value", "get_value"},
		{"get-attribute", "get_attribute"},
		{"set-attribute", "set_attribute"},
		{"wait-for", "wait_for"},
		{"key-press", "key_press"},
		{"scroll-to", "scroll_to"},
		{"execute-js", "execute_js"},
		{"navigate", "navigate"},
		{"network-waterfall", "network_waterfall"},
		{"network-bodies", "network_bodies"},
		{"noise-rule", "noise_rule"},
	}

	for _, tt := range tests {
		got := NormalizeAction(tt.input)
		if got != tt.expected {
			t.Errorf("NormalizeAction(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

// --- Result building tests ---

func TestBuildResultFromToolResponse(t *testing.T) {
	t.Parallel()

	textContent := `{"entries":[{"level":"error","message":"test"}],"total":1}`
	result := BuildResult("observe", "logs", textContent, false)

	if !result.Success {
		t.Error("expected success")
	}
	if result.Tool != "observe" {
		t.Errorf("expected tool 'observe', got %q", result.Tool)
	}
	if result.TextContent != textContent {
		t.Errorf("expected text content, got %q", result.TextContent)
	}
}

func TestBuildResultFromToolResponseError(t *testing.T) {
	t.Parallel()

	result := BuildResult("interact", "click", "Element not found", true)

	if result.Success {
		t.Error("expected failure")
	}
	if result.Error != "Element not found" {
		t.Errorf("expected error message, got %q", result.Error)
	}
}

func TestBuildResultParsesJSONContent(t *testing.T) {
	t.Parallel()

	textContent := `{"selector":"#btn","clicked":true}`
	result := BuildResult("interact", "click", textContent, false)

	if result.Data == nil {
		t.Fatal("expected parsed data")
	}
	if result.Data["selector"] != "#btn" {
		t.Errorf("expected selector '#btn', got %v", result.Data["selector"])
	}
}

func TestBuildResultNonJSONContent(t *testing.T) {
	t.Parallel()

	textContent := "This is plain text response"
	result := BuildResult("observe", "logs", textContent, false)

	// Non-JSON should be stored as text content, not data
	if result.TextContent != textContent {
		t.Errorf("expected text content preserved, got %q", result.TextContent)
	}
}

func TestExtractTextFromContentBlocks(t *testing.T) {
	t.Parallel()

	blocks := []json.RawMessage{
		json.RawMessage(`{"type":"text","text":"First block"}`),
		json.RawMessage(`{"type":"text","text":"Second block"}`),
	}

	text := ExtractText(blocks)
	if !strings.Contains(text, "First block") {
		t.Errorf("expected first block, got: %s", text)
	}
	if !strings.Contains(text, "Second block") {
		t.Errorf("expected second block, got: %s", text)
	}
}
