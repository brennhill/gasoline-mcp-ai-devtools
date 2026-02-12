// golden_test.go — Test to regenerate golden files for MCP tool schemas.
package main

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/dev-console/dev-console/internal/capture"
)

func TestUpdateGoldenToolsList(t *testing.T) {
	// Create server with temp log file
	server, err := NewServer("/tmp/test-gasoline-golden.jsonl", 100)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	cap := capture.NewCapture()

	// Create handler using proper constructor
	mcpHandler := NewToolHandler(server, cap)

	// Extract the ToolHandler from MCPHandler.toolHandler
	toolHandler := mcpHandler.toolHandler.(*ToolHandler)

	tools := toolHandler.ToolsList()
	data, err := json.MarshalIndent(tools, "", "  ")
	if err != nil {
		t.Fatalf("json.MarshalIndent failed: %v", err)
	}

	// Append a trailing newline for consistency
	data = append(data, '\n')

	err = os.WriteFile("testdata/mcp-tools-list.golden.json", data, 0644)
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	t.Logf("Updated golden file with %d tools (%d bytes)", len(tools), len(data))
}
