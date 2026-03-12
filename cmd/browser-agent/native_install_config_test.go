// native_install_config_test.go — Tests for mergeJSONConfig safety guarantees.

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
			"github":    map[string]any{"command": "github-mcp"},
			"atlassian": map[string]any{"command": "atlassian-mcp"},
		},
	}
	data, _ := json.MarshalIndent(existing, "", "  ")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}

	if err := mergeJSONConfig(path, "mcpServers", "/usr/local/bin/gasoline", false); err != nil {
		t.Fatalf("mergeJSONConfig failed: %v", err)
	}

	result := readJSONFile(t, path)
	servers := result["mcpServers"].(map[string]any)

	if _, ok := servers["github"]; !ok {
		t.Error("github server was deleted")
	}
	if _, ok := servers["atlassian"]; !ok {
		t.Error("atlassian server was deleted")
	}
	if _, ok := servers[mcpServerName]; !ok {
		t.Errorf("%s server was not added", mcpServerName)
	}
}

func TestMergeJSONConfig_RefusesToOverwriteInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")

	if err := os.WriteFile(path, []byte(`{not valid json`), 0600); err != nil {
		t.Fatal(err)
	}

	err := mergeJSONConfig(path, "mcpServers", "/usr/local/bin/gasoline", false)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}

	// Verify the original file was NOT overwritten.
	content, _ := os.ReadFile(path)
	if string(content) != `{not valid json` {
		t.Errorf("original file was modified: %s", content)
	}
}

func TestMergeJSONConfig_CreatesBackup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")

	original := `{"mcpServers": {"other": {"command": "other-mcp"}}}`
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}

	if err := mergeJSONConfig(path, "mcpServers", "/usr/local/bin/gasoline", false); err != nil {
		t.Fatalf("mergeJSONConfig failed: %v", err)
	}

	bakPath := path + ".bak"
	bakContent, err := os.ReadFile(bakPath)
	if err != nil {
		t.Fatalf("backup file not created: %v", err)
	}
	if string(bakContent) != original {
		t.Errorf("backup content mismatch: got %s", bakContent)
	}
}

func TestMergeJSONConfig_RemovesLegacyKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")

	existing := map[string]any{
		"mcpServers": map[string]any{
			"gasoline":                 map[string]any{"command": "old"},
			"gasoline-agentic-browser": map[string]any{"command": "older"},
			"github":                   map[string]any{"command": "github-mcp"},
		},
	}
	data, _ := json.MarshalIndent(existing, "", "  ")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}

	if err := mergeJSONConfig(path, "mcpServers", "/usr/local/bin/gasoline", false); err != nil {
		t.Fatalf("mergeJSONConfig failed: %v", err)
	}

	result := readJSONFile(t, path)
	servers := result["mcpServers"].(map[string]any)

	for _, legacy := range installerLegacyServerKeys {
		if _, ok := servers[legacy]; ok {
			t.Errorf("legacy key %q was not removed", legacy)
		}
	}
	if _, ok := servers["github"]; !ok {
		t.Error("github server was deleted")
	}
	if _, ok := servers[mcpServerName]; !ok {
		t.Errorf("%s server was not added", mcpServerName)
	}
}

func TestMergeJSONConfig_EmptyFileCreatesNew(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")

	if err := os.WriteFile(path, []byte{}, 0600); err != nil {
		t.Fatal(err)
	}

	if err := mergeJSONConfig(path, "mcpServers", "/usr/local/bin/gasoline", false); err != nil {
		t.Fatalf("mergeJSONConfig failed: %v", err)
	}

	result := readJSONFile(t, path)
	servers := result["mcpServers"].(map[string]any)
	if _, ok := servers[mcpServerName]; !ok {
		t.Errorf("%s server was not added", mcpServerName)
	}
}

func TestMergeJSONConfig_MissingFileCreatesNew(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")

	if err := mergeJSONConfig(path, "mcpServers", "/usr/local/bin/gasoline", false); err != nil {
		t.Fatalf("mergeJSONConfig failed: %v", err)
	}

	result := readJSONFile(t, path)
	servers := result["mcpServers"].(map[string]any)
	if _, ok := servers[mcpServerName]; !ok {
		t.Errorf("%s server was not added", mcpServerName)
	}
}

// readJSONFile is a test helper that reads and parses a JSON file.
func readJSONFile(t *testing.T, path string) map[string]any {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}
	var result map[string]any
	if err := json.Unmarshal(content, &result); err != nil {
		t.Fatalf("failed to parse %s: %v", path, err)
	}
	return result
}
