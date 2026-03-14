// native_install_config_test.go — Tests for mergeJSONConfig safety: backup, invalid-JSON refusal, legacy key removal.

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestMergeJSONConfig_PreservesExistingServers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")

	existing := map[string]any{
		"mcpServers": map[string]any{
			"other-server": map[string]any{
				"command": "/usr/bin/other",
				"args":    []string{"--flag"},
			},
		},
	}
	data, _ := json.MarshalIndent(existing, "", "  ")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}

	if err := mergeJSONConfig(path, "mcpServers", "/usr/bin/gasoline", false); err != nil {
		t.Fatalf("mergeJSONConfig failed: %v", err)
	}

	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var result map[string]any
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	servers, ok := result["mcpServers"].(map[string]any)
	if !ok {
		t.Fatal("mcpServers key missing or wrong type")
	}

	if _, ok := servers["other-server"]; !ok {
		t.Error("existing server 'other-server' was deleted")
	}
	if _, ok := servers[mcpServerName]; !ok {
		t.Errorf("canonical server %q was not added", mcpServerName)
	}
}

func TestMergeJSONConfig_RefusesToOverwriteInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")

	if err := os.WriteFile(path, []byte("{invalid json}"), 0600); err != nil {
		t.Fatal(err)
	}

	err := mergeJSONConfig(path, "mcpServers", "/usr/bin/gasoline", false)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}

	// Verify file was NOT overwritten.
	content, _ := os.ReadFile(path)
	if string(content) != "{invalid json}" {
		t.Errorf("file was modified despite invalid JSON; got: %s", content)
	}
}

func TestMergeJSONConfig_CreatesBackup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")

	original := `{"mcpServers":{}}`
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}

	if err := mergeJSONConfig(path, "mcpServers", "/usr/bin/gasoline", false); err != nil {
		t.Fatalf("mergeJSONConfig failed: %v", err)
	}

	bakPath := path + ".bak"
	bak, err := os.ReadFile(bakPath)
	if err != nil {
		t.Fatalf("backup file not created: %v", err)
	}
	if string(bak) != original {
		t.Errorf("backup content mismatch: got %q, want %q", bak, original)
	}
}

func TestMergeJSONConfig_RemovesLegacyKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")

	existing := map[string]any{
		"mcpServers": map[string]any{
			"gasoline":                map[string]any{"command": "/old"},
			"gasoline-agentic-browser": map[string]any{"command": "/old2"},
			"keep-this":               map[string]any{"command": "/keep"},
		},
	}
	data, _ := json.MarshalIndent(existing, "", "  ")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}

	if err := mergeJSONConfig(path, "mcpServers", "/usr/bin/gasoline", false); err != nil {
		t.Fatalf("mergeJSONConfig failed: %v", err)
	}

	out, _ := os.ReadFile(path)
	var result map[string]any
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	servers := result["mcpServers"].(map[string]any)
	for _, legacy := range installerLegacyServerKeys {
		if _, ok := servers[legacy]; ok {
			t.Errorf("legacy key %q was not removed", legacy)
		}
	}
	if _, ok := servers["keep-this"]; !ok {
		t.Error("non-legacy server 'keep-this' was incorrectly removed")
	}
}

func TestMergeJSONConfig_EmptyFileCreatesNew(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")

	if err := os.WriteFile(path, []byte(""), 0600); err != nil {
		t.Fatal(err)
	}

	if err := mergeJSONConfig(path, "mcpServers", "/usr/bin/gasoline", false); err != nil {
		t.Fatalf("mergeJSONConfig failed: %v", err)
	}

	out, _ := os.ReadFile(path)
	var result map[string]any
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	servers, ok := result["mcpServers"].(map[string]any)
	if !ok {
		t.Fatal("mcpServers key missing")
	}
	if _, ok := servers[mcpServerName]; !ok {
		t.Errorf("canonical server %q was not added", mcpServerName)
	}
}

func TestMergeJSONConfig_MissingFileCreatesNew(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")

	if err := mergeJSONConfig(path, "mcpServers", "/usr/bin/gasoline", false); err != nil {
		t.Fatalf("mergeJSONConfig failed: %v", err)
	}

	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("file was not created: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	servers, ok := result["mcpServers"].(map[string]any)
	if !ok {
		t.Fatal("mcpServers key missing")
	}
	if _, ok := servers[mcpServerName]; !ok {
		t.Errorf("canonical server %q was not added", mcpServerName)
	}
}
