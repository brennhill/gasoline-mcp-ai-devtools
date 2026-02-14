// golden_test.go — Golden file validation for MCP tool schemas and initialize response.
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"regexp"
	"testing"

	"github.com/dev-console/dev-console/internal/capture"
)

var updateGolden = os.Getenv("UPDATE_GOLDEN") == "1"

// normalizeVersion replaces all "version": "x.y.z" fields with "version": "VERSION"
func normalizeVersion(data []byte) []byte {
	re := regexp.MustCompile(`"version"\s*:\s*"[^"]*"`)
	return re.ReplaceAll(data, []byte(`"version": "VERSION"`))
}

func TestGoldenToolsList(t *testing.T) {
	// Create server with temp log file
	server, err := NewServer("/tmp/test-gasoline-golden-tools.jsonl", 100)
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

	// Normalize version fields
	normalizedData := normalizeVersion(data)

	if updateGolden {
		// Update mode: write normalized data to golden file
		err = os.WriteFile("testdata/mcp-tools-list.golden.json", normalizedData, 0644)
		if err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}
		t.Logf("Updated golden file with %d tools (%d bytes)", len(tools), len(normalizedData))
	} else {
		// Comparison mode: read golden file and compare
		goldenData, err := os.ReadFile("testdata/mcp-tools-list.golden.json")
		if err != nil {
			t.Fatalf("Failed to read golden file: %v", err)
		}

		// Normalize golden data as well
		normalizedGolden := normalizeVersion(goldenData)

		if !bytes.Equal(normalizedData, normalizedGolden) {
			t.Errorf("Golden file mismatch for mcp-tools-list.golden.json")
			t.Errorf("Expected %d bytes, got %d bytes", len(normalizedGolden), len(normalizedData))
			t.Fatalf("Run with UPDATE_GOLDEN=1 to update golden files")
		}

		t.Logf("Golden file validation passed (%d tools, %d bytes)", len(tools), len(normalizedData))
	}
}

func TestGoldenInitialize(t *testing.T) {
	// Create server with temp log file
	server, err := NewServer("/tmp/test-gasoline-golden-init.jsonl", 100)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	cap := capture.NewCapture()

	// Create handler using proper constructor
	mcpHandler := NewToolHandler(server, cap)

	// Create initialize request
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params:  json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0.0"}}`),
	}

	// Call HandleRequest
	resp := mcpHandler.HandleRequest(req)
	if resp == nil {
		t.Fatalf("HandleRequest returned nil response")
	}

	if resp.Error != nil {
		t.Fatalf("HandleRequest returned error: %v", resp.Error.Message)
	}

	// Marshal the result to JSON
	data, err := json.MarshalIndent(json.RawMessage(resp.Result), "", "  ")
	if err != nil {
		t.Fatalf("json.MarshalIndent failed: %v", err)
	}

	// Append a trailing newline for consistency
	data = append(data, '\n')

	// Normalize version fields
	normalizedData := normalizeVersion(data)

	if updateGolden {
		// Update mode: write normalized data to golden file
		err = os.WriteFile("testdata/mcp-initialize.golden.json", normalizedData, 0644)
		if err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}
		t.Logf("Updated initialize golden file (%d bytes)", len(normalizedData))
	} else {
		// Comparison mode: read golden file and compare
		goldenData, err := os.ReadFile("testdata/mcp-initialize.golden.json")
		if err != nil {
			t.Fatalf("Failed to read golden file: %v", err)
		}

		// Normalize golden data as well
		normalizedGolden := normalizeVersion(goldenData)

		if !bytes.Equal(normalizedData, normalizedGolden) {
			t.Errorf("Golden file mismatch for mcp-initialize.golden.json")
			t.Errorf("Expected %d bytes, got %d bytes", len(normalizedGolden), len(normalizedData))
			t.Fatalf("Run with UPDATE_GOLDEN=1 to update golden files")
		}

		t.Logf("Initialize golden file validation passed (%d bytes)", len(normalizedData))
	}
}
