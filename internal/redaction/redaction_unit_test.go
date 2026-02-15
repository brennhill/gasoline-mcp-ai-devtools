package redaction

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewRedactionEngine_CustomPatterns(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "redaction.json")
	config := `{
		"patterns": [
			{"name":"custom-secret","pattern":"secret_[0-9]+"},
			{"name":"explicit-replacement","pattern":"token=[a-z]+","replacement":"token=[MASKED]"},
			{"name":"invalid-regex","pattern":"[unclosed"}
		]
	}`
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	engine := NewRedactionEngine(configPath)
	got := engine.Redact("secret_42 token=abc")
	if !strings.Contains(got, "[REDACTED:custom-secret]") {
		t.Fatalf("expected default replacement for custom-secret, got %q", got)
	}
	if !strings.Contains(got, "token=[MASKED]") {
		t.Fatalf("expected explicit replacement, got %q", got)
	}
}

func TestRedact_CreditCardLuhnValidation(t *testing.T) {
	t.Parallel()

	engine := NewRedactionEngine("")

	valid := engine.Redact("card 4111 1111 1111 1111")
	if !strings.Contains(valid, "[REDACTED:credit-card]") {
		t.Fatalf("expected valid card to be redacted, got %q", valid)
	}

	invalid := engine.Redact("card 4111 1111 1111 1112")
	if strings.Contains(invalid, "[REDACTED:credit-card]") {
		t.Fatalf("expected invalid card to remain, got %q", invalid)
	}
}

func TestRedactJSON_StructuredBlocksOnly(t *testing.T) {
	t.Parallel()

	engine := NewRedactionEngine("")
	input := json.RawMessage(`{
		"content": [
			{"type":"text","text":"Authorization: Bearer abc123"},
			{"type":"image","text":"Authorization: Bearer keep-me"}
		],
		"isError": false
	}`)

	out := engine.RedactJSON(input)
	var parsed MCPToolResult
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if len(parsed.Content) != 2 {
		t.Fatalf("expected 2 content blocks, got %d", len(parsed.Content))
	}
	if !strings.Contains(parsed.Content[0].Text, "[REDACTED:bearer-token]") {
		t.Fatalf("expected text block redacted, got %q", parsed.Content[0].Text)
	}
	if parsed.Content[1].Text != "Authorization: Bearer keep-me" {
		t.Fatalf("non-text block should be untouched, got %q", parsed.Content[1].Text)
	}
}

func TestRedactJSON_FallbackForMalformedInput(t *testing.T) {
	t.Parallel()

	engine := NewRedactionEngine("")
	input := json.RawMessage(`{"content":[{"type":"text","text":"AKIAIOSFODNN7EXAMPLE"}]`)
	out := engine.RedactJSON(input)
	if !strings.Contains(string(out), "[REDACTED:aws-key]") {
		t.Fatalf("expected fallback raw-string redaction, got %q", string(out))
	}
}

func TestLuhnValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value string
		valid bool
	}{
		{value: "4111111111111111", valid: true},
		{value: "4111-1111-1111-1111", valid: true},
		{value: "4111 1111 1111 1111", valid: true},
		{value: "4111111111111112", valid: false},
		{value: "123456", valid: false},
	}

	for _, tt := range tests {
		if got := luhnValid(tt.value); got != tt.valid {
			t.Fatalf("luhnValid(%q) = %v, want %v", tt.value, got, tt.valid)
		}
	}
}
