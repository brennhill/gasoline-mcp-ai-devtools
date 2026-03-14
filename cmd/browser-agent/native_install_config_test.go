package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestMergeJSONConfig_PreservesExistingServers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	existing := map[string]any{
		"mcpServers": map[string]any{
			"github": map[string]any{
				"command": "github-mcp",
				"args":    []any{},
			},
			"atlassian": map[string]any{
				"command": "atlassian-mcp",
				"args":    []any{"--token", "xxx"},
			},
		},
	}
	data, _ := json.MarshalIndent(existing, "", "  ")
	os.WriteFile(path, data, 0600)

	err := mergeJSONConfig(path, "mcpServers", "/usr/local/bin/gasoline", false)
	if err != nil {
		t.Fatalf("mergeJSONConfig failed: %v", err)
	}

	raw, _ := os.ReadFile(path)
	var result map[string]any
	json.Unmarshal(raw, &result)

	servers := result["mcpServers"].(map[string]any)
	if _, ok := servers["github"]; !ok {
		t.Fatal("github server was removed")
	}
	if _, ok := servers["atlassian"]; !ok {
		t.Fatal("atlassian server was removed")
	}
	if _, ok := servers[mcpServerName]; !ok {
		t.Fatal("gasoline server was not added")
	}
}

func TestMergeJSONConfig_RefusesToOverwriteInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	// Write invalid JSON (trailing comma)
	os.WriteFile(path, []byte(`{"mcpServers": {"github": {"command": "gh"},}}`), 0600)

	err := mergeJSONConfig(path, "mcpServers", "/usr/local/bin/gasoline", false)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}

	// Verify the original file was NOT overwritten
	raw, _ := os.ReadFile(path)
	if string(raw) != `{"mcpServers": {"github": {"command": "gh"},}}` {
		t.Fatalf("file was modified despite error; got: %s", string(raw))
	}
}

func TestMergeJSONConfig_CreatesBackup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	original := `{"mcpServers": {"github": {"command": "gh"}}}`
	os.WriteFile(path, []byte(original), 0600)

	err := mergeJSONConfig(path, "mcpServers", "/usr/local/bin/gasoline", false)
	if err != nil {
		t.Fatalf("mergeJSONConfig failed: %v", err)
	}

	backup, err := os.ReadFile(path + ".bak")
	if err != nil {
		t.Fatal("backup file was not created")
	}
	if string(backup) != original {
		t.Fatalf("backup content mismatch; got: %s", string(backup))
	}
}

func TestMergeJSONConfig_RemovesLegacyKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	existing := map[string]any{
		"mcpServers": map[string]any{
			"gasoline":                 map[string]any{"command": "old"},
			"gasoline-agentic-browser": map[string]any{"command": "old2"},
			"github":                   map[string]any{"command": "gh"},
		},
	}
	data, _ := json.MarshalIndent(existing, "", "  ")
	os.WriteFile(path, data, 0600)

	mergeJSONConfig(path, "mcpServers", "/usr/local/bin/gasoline", false)

	raw, _ := os.ReadFile(path)
	var result map[string]any
	json.Unmarshal(raw, &result)

	servers := result["mcpServers"].(map[string]any)
	if _, ok := servers["gasoline"]; ok {
		t.Fatal("legacy 'gasoline' key was not removed")
	}
	if _, ok := servers["gasoline-agentic-browser"]; ok {
		t.Fatal("legacy 'gasoline-agentic-browser' key was not removed")
	}
	if _, ok := servers["github"]; !ok {
		t.Fatal("github server was incorrectly removed")
	}
}

func TestMergeJSONConfig_EmptyFileCreatesNew(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	// Empty file is OK — treated as new config
	os.WriteFile(path, []byte{}, 0600)

	err := mergeJSONConfig(path, "mcpServers", "/usr/local/bin/gasoline", false)
	if err != nil {
		t.Fatalf("mergeJSONConfig failed on empty file: %v", err)
	}

	raw, _ := os.ReadFile(path)
	var result map[string]any
	json.Unmarshal(raw, &result)

	servers := result["mcpServers"].(map[string]any)
	if _, ok := servers[mcpServerName]; !ok {
		t.Fatal("gasoline server was not added to empty config")
	}
}

func TestMergeJSONConfig_MissingFileCreatesNew(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	err := mergeJSONConfig(path, "mcpServers", "/usr/local/bin/gasoline", false)
	if err != nil {
		t.Fatalf("mergeJSONConfig failed on missing file: %v", err)
	}

	raw, _ := os.ReadFile(path)
	var result map[string]any
	json.Unmarshal(raw, &result)

	servers := result["mcpServers"].(map[string]any)
	if _, ok := servers[mcpServerName]; !ok {
		t.Fatal("gasoline server was not added")
	}
}
