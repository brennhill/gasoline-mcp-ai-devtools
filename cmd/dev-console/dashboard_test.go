// dashboard_test.go — Tests for dashboard helpers.
package main

import (
	"testing"
	"time"

	"github.com/dev-console/dev-console/internal/capture"
)

func TestParseMCPCommand_ToolCalls(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantTool   string
		wantParams string
	}{
		{
			name:       "observe errors",
			body:       `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"observe","arguments":{"what":"errors"}}}`,
			wantTool:   "observe",
			wantParams: "what=errors",
		},
		{
			name:       "interact navigate",
			body:       `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"interact","arguments":{"what":"navigate","url":"https://example.com"}}}`,
			wantTool:   "interact",
			wantParams: "what=navigate url=https://example.com",
		},
		{
			name:       "interact click",
			body:       `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"interact","arguments":{"what":"click","selector":"#btn"}}}`,
			wantTool:   "interact",
			wantParams: "what=click selector=#btn",
		},
		{
			name:       "analyze accessibility",
			body:       `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"analyze","arguments":{"what":"accessibility"}}}`,
			wantTool:   "analyze",
			wantParams: "what=accessibility",
		},
		{
			name:       "generate test",
			body:       `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"generate","arguments":{"what":"test"}}}`,
			wantTool:   "generate",
			wantParams: "what=test",
		},
		{
			name:       "configure clear",
			body:       `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"configure","arguments":{"what":"clear","buffer":"all"}}}`,
			wantTool:   "configure",
			wantParams: "what=clear buffer=all",
		},
		{
			name:       "configure noise rule",
			body:       `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"configure","arguments":{"what":"noise_rule","noise_action":"auto_detect"}}}`,
			wantTool:   "configure",
			wantParams: "what=noise_rule noise_action=auto_detect",
		},
		{
			name:       "empty body",
			body:       "",
			wantTool:   "unknown",
			wantParams: "",
		},
		{
			name:       "invalid json",
			body:       "not json",
			wantTool:   "unknown",
			wantParams: "",
		},
		{
			name:       "non-tool method",
			body:       `{"jsonrpc":"2.0","method":"initialize","params":{}}`,
			wantTool:   "initialize",
			wantParams: "",
		},
		{
			name:       "observe with no arguments",
			body:       `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"observe","arguments":{}}}`,
			wantTool:   "observe",
			wantParams: "",
		},
		{
			name:       "long url truncated",
			body:       `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"interact","arguments":{"what":"navigate","url":"https://example.com/very/long/path/that/exceeds/the/forty/character/limit/and/should/be/truncated"}}}`,
			wantTool:   "interact",
			wantParams: "what=navigate url=https://example.com/very/long/path/th...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool, params := parseMCPCommand(tt.body)
			if tool != tt.wantTool {
				t.Errorf("tool = %q, want %q", tool, tt.wantTool)
			}
			if params != tt.wantParams {
				t.Errorf("params = %q, want %q", params, tt.wantParams)
			}
		})
	}
}

func TestBuildRecentCommands_UsesToolAndParams(t *testing.T) {
	entries := []capture.HTTPDebugEntry{
		{
			Timestamp:      time.Now(),
			Endpoint:       "/mcp",
			Method:         "POST",
			RequestBody:    `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"observe","arguments":{"what":"logs"}}}`,
			ResponseStatus: 200,
			DurationMs:     5,
		},
		{
			Timestamp:      time.Now().Add(-time.Second),
			Endpoint:       "/mcp",
			Method:         "POST",
			RequestBody:    `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"interact","arguments":{"what":"click","selector":"#btn"}}}`,
			ResponseStatus: 200,
			DurationMs:     120,
		},
	}

	cmds := buildRecentCommands(entries)
	if len(cmds) != 2 {
		t.Fatalf("expected 2 commands, got %d", len(cmds))
	}

	// Newest first
	if cmds[0].Tool != "observe" {
		t.Errorf("cmds[0].Tool = %q, want observe", cmds[0].Tool)
	}
	if cmds[0].Params != "what=logs" {
		t.Errorf("cmds[0].Params = %q, want what=logs", cmds[0].Params)
	}
	if cmds[1].Tool != "interact" {
		t.Errorf("cmds[1].Tool = %q, want interact", cmds[1].Tool)
	}
	if cmds[1].Params != "what=click selector=#btn" {
		t.Errorf("cmds[1].Params = %q, want what=click selector=#btn", cmds[1].Params)
	}
}
