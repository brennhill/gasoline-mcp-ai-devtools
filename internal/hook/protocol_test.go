// protocol_test.go — Tests for hook protocol parsing.

package hook

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestReadInput_ValidJSON(t *testing.T) {
	t.Parallel()
	input := `{"tool_name":"Bash","tool_input":{"command":"go test ./..."},"tool_response":"ok"}`
	in, err := ReadInput(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ReadInput: %v", err)
	}
	if in.ToolName != "Bash" {
		t.Errorf("ToolName = %q, want Bash", in.ToolName)
	}
}

func TestReadInput_InvalidJSON(t *testing.T) {
	t.Parallel()
	_, err := ReadInput(strings.NewReader("not json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseToolInput_FilePath(t *testing.T) {
	t.Parallel()
	in := Input{ToolInput: json.RawMessage(`{"file_path":"/tmp/foo.go"}`)}
	fields := in.ParseToolInput()
	if fields.FilePath != "/tmp/foo.go" {
		t.Errorf("FilePath = %q, want /tmp/foo.go", fields.FilePath)
	}
}

func TestParseToolInput_Command(t *testing.T) {
	t.Parallel()
	in := Input{ToolInput: json.RawMessage(`{"command":"go test ./..."}`)}
	fields := in.ParseToolInput()
	if fields.Command != "go test ./..." {
		t.Errorf("Command = %q, want 'go test ./...'", fields.Command)
	}
}

func TestResponseText_String(t *testing.T) {
	t.Parallel()
	in := Input{ToolResponse: json.RawMessage(`"hello world"`)}
	if got := in.ResponseText(); got != "hello world" {
		t.Errorf("ResponseText = %q, want 'hello world'", got)
	}
}

func TestResponseText_ObjectWithOutput(t *testing.T) {
	t.Parallel()
	in := Input{ToolResponse: json.RawMessage(`{"output":"test output"}`)}
	if got := in.ResponseText(); got != "test output" {
		t.Errorf("ResponseText = %q, want 'test output'", got)
	}
}

func TestResponseText_ObjectWithStdout(t *testing.T) {
	t.Parallel()
	in := Input{ToolResponse: json.RawMessage(`{"stdout":"stdout text"}`)}
	if got := in.ResponseText(); got != "stdout text" {
		t.Errorf("ResponseText = %q, want 'stdout text'", got)
	}
}

func TestResponseText_Empty(t *testing.T) {
	t.Parallel()
	in := Input{}
	if got := in.ResponseText(); got != "" {
		t.Errorf("ResponseText = %q, want empty", got)
	}
}

func TestWriteOutput_NonEmpty(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := WriteOutput(&buf, "some context"); err != nil {
		t.Fatalf("WriteOutput: %v", err)
	}
	var out Output
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if out.AdditionalContext != "some context" {
		t.Errorf("AdditionalContext = %q, want 'some context'", out.AdditionalContext)
	}
}

func TestWriteOutput_Empty(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := WriteOutput(&buf, ""); err != nil {
		t.Fatalf("WriteOutput: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no output for empty context, got %q", buf.String())
	}
}

func TestDetectAgent_Default(t *testing.T) {
	// Clear any env vars that might be set.
	t.Setenv("GEMINI_SESSION_ID", "")
	t.Setenv("CODEX_SESSION_ID", "")
	if got := DetectAgent(); got != AgentClaude {
		t.Errorf("DetectAgent() = %q, want %q", got, AgentClaude)
	}
}

func TestDetectAgent_Gemini(t *testing.T) {
	t.Setenv("GEMINI_SESSION_ID", "test-session")
	if got := DetectAgent(); got != AgentGemini {
		t.Errorf("DetectAgent() = %q, want %q", got, AgentGemini)
	}
}

func TestDetectAgent_Codex(t *testing.T) {
	t.Setenv("CODEX_SESSION_ID", "test-session")
	t.Setenv("GEMINI_SESSION_ID", "")
	if got := DetectAgent(); got != AgentCodex {
		t.Errorf("DetectAgent() = %q, want %q", got, AgentCodex)
	}
}

func TestWriteOutput_GeminiFormat(t *testing.T) {
	t.Setenv("GEMINI_SESSION_ID", "test-session")

	var buf bytes.Buffer
	if err := WriteOutput(&buf, "test context"); err != nil {
		t.Fatalf("WriteOutput: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(buf.Bytes(), &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	hso, ok := raw["hookSpecificOutput"]
	if !ok {
		t.Fatal("expected hookSpecificOutput key for Gemini format")
	}

	var inner map[string]string
	if err := json.Unmarshal(hso, &inner); err != nil {
		t.Fatalf("unmarshal inner: %v", err)
	}

	if inner["additionalContext"] != "test context" {
		t.Errorf("additionalContext = %q, want 'test context'", inner["additionalContext"])
	}
}

func TestWriteOutput_ClaudeFormat(t *testing.T) {
	t.Setenv("GEMINI_SESSION_ID", "")
	t.Setenv("CODEX_SESSION_ID", "")

	var buf bytes.Buffer
	if err := WriteOutput(&buf, "test context"); err != nil {
		t.Fatalf("WriteOutput: %v", err)
	}

	var out Output
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if out.AdditionalContext != "test context" {
		t.Errorf("AdditionalContext = %q, want 'test context'", out.AdditionalContext)
	}
}
