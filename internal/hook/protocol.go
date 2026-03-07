// protocol.go — Hook protocol types and helpers for Claude Code, Gemini CLI, and Codex.
// Handles JSON input from stdin and JSON output to stdout for PostToolUse/AfterTool hooks.

package hook

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// Agent identifies which AI coding agent is calling the hook.
type Agent string

const (
	AgentClaude Agent = "claude"
	AgentGemini Agent = "gemini"
	AgentCodex  Agent = "codex"
)

// DetectAgent identifies the calling agent from environment variables.
func DetectAgent() Agent {
	if os.Getenv("GEMINI_SESSION_ID") != "" {
		return AgentGemini
	}
	if os.Getenv("CODEX_SESSION_ID") != "" {
		return AgentCodex
	}
	return AgentClaude
}

// Input is the JSON structure Claude Code sends to PostToolUse hooks via stdin.
type Input struct {
	ToolName     string          `json:"tool_name"`
	ToolInput    json.RawMessage `json:"tool_input"`
	ToolResponse json.RawMessage `json:"tool_response"`
}

// ToolInputFields holds the commonly needed fields from tool_input.
type ToolInputFields struct {
	FilePath  string `json:"file_path"`
	Command   string `json:"command"`
	NewString string `json:"new_string"` // Edit tool: the replacement text
	Content   string `json:"content"`    // Write tool: the full file content
}

// Output is the JSON structure hooks write to stdout.
type Output struct {
	AdditionalContext string `json:"additionalContext"` // SPEC:claude-code-hooks (camelCase per protocol)
}

// ReadInput reads and parses hook input from a reader (typically os.Stdin).
func ReadInput(r io.Reader) (Input, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return Input{}, fmt.Errorf("ReadInput: cannot read stdin. %v", err)
	}
	var input Input
	if err := json.Unmarshal(data, &input); err != nil {
		return Input{}, fmt.Errorf("ReadInput: invalid JSON input. %v", err)
	}
	return input, nil
}

// ParseToolInput extracts common fields from the tool_input JSON.
func (in Input) ParseToolInput() ToolInputFields {
	var fields ToolInputFields
	if len(in.ToolInput) > 0 {
		// Best-effort parse — malformed tool_input falls back to zero-value fields,
		// causing the hook to silently do nothing (correct per hook protocol).
		_ = json.Unmarshal(in.ToolInput, &fields)
	}
	return fields
}

// ResponseText extracts the output text from tool_response.
// Handles both string responses and object responses with output/stdout/content fields.
func (in Input) ResponseText() string {
	if len(in.ToolResponse) == 0 {
		return ""
	}

	// Try as plain string first.
	var s string
	if err := json.Unmarshal(in.ToolResponse, &s); err == nil {
		return s
	}

	// Try as object with output/stdout/content fields.
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(in.ToolResponse, &obj); err != nil {
		return ""
	}

	for _, key := range []string{"output", "stdout", "content"} {
		if raw, ok := obj[key]; ok {
			var val string
			if json.Unmarshal(raw, &val) == nil {
				return val
			}
		}
	}
	return ""
}

// WriteOutput writes the hook output JSON to a writer (typically os.Stdout).
// Auto-detects the calling agent and adapts the JSON format accordingly.
// Returns nil if context is empty (nothing to output).
func WriteOutput(w io.Writer, context string) error {
	if context == "" {
		return nil
	}
	agent := DetectAgent()
	var out any
	switch agent {
	case AgentGemini:
		// SPEC:gemini-cli-hooks — nested under hookSpecificOutput.
		out = map[string]any{
			"hookSpecificOutput": map[string]string{
				"additionalContext": context,
			},
		}
	default:
		// SPEC:claude-code-hooks — flat additionalContext.
		out = Output{AdditionalContext: context}
	}
	return json.NewEncoder(w).Encode(out) //nolint: error returned to caller
}
