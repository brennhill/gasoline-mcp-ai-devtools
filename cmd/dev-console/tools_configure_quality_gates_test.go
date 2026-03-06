// tools_configure_quality_gates_test.go — Tests for configure(what="setup_quality_gates") handler.

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetupQualityGates_CreatesConfigAndStandards(t *testing.T) {
	t.Parallel()
	h, server, _ := makeToolHandler(t)

	dir := t.TempDir()
	server.SetActiveCodebase(dir)

	resp := callConfigureRaw(h, `{"what":"setup_quality_gates"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("expected success, got error: %s", firstText(result))
	}

	data := extractResultJSON(t, result)

	// Verify .gasoline.json was created.
	configPath := filepath.Join(dir, ".gasoline.json")
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf(".gasoline.json not created: %v", err)
	}

	// Verify gasoline-code-standards.md was created.
	standardsPath := filepath.Join(dir, "gasoline-code-standards.md")
	if _, err := os.Stat(standardsPath); err != nil {
		t.Fatalf("gasoline-code-standards.md not created: %v", err)
	}

	// Verify response contains created paths.
	if data["config_path"] == nil {
		t.Fatal("response missing config_path")
	}
	if data["standards_path"] == nil {
		t.Fatal("response missing standards_path")
	}

	// Verify config content is valid JSON with expected defaults.
	configBytes, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read .gasoline.json: %v", err)
	}
	var config map[string]any
	if err := json.Unmarshal(configBytes, &config); err != nil {
		t.Fatalf(".gasoline.json is not valid JSON: %v", err)
	}
	if config["code_standards"] != "gasoline-code-standards.md" {
		t.Fatalf("code_standards = %v, want gasoline-code-standards.md", config["code_standards"])
	}

	// Verify standards file has content.
	standardsBytes, err := os.ReadFile(standardsPath)
	if err != nil {
		t.Fatalf("failed to read gasoline-code-standards.md: %v", err)
	}
	if len(standardsBytes) < 100 {
		t.Fatal("gasoline-code-standards.md is too short — missing starter content")
	}
}

func TestSetupQualityGates_DoesNotOverwriteExistingConfig(t *testing.T) {
	t.Parallel()
	h, server, _ := makeToolHandler(t)

	dir := t.TempDir()
	server.SetActiveCodebase(dir)

	// Pre-create .gasoline.json with custom content.
	existing := `{"code_standards":"my-custom-rules.md","file_size_limit":500}`
	if err := os.WriteFile(filepath.Join(dir, ".gasoline.json"), []byte(existing), 0644); err != nil {
		t.Fatal(err)
	}

	resp := callConfigureRaw(h, `{"what":"setup_quality_gates"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("expected success, got error: %s", firstText(result))
	}

	// Verify existing config was NOT overwritten.
	configBytes, _ := os.ReadFile(filepath.Join(dir, ".gasoline.json"))
	if string(configBytes) != existing {
		t.Fatalf(".gasoline.json was overwritten: got %q", string(configBytes))
	}

	data := extractResultJSON(t, result)
	if data["config_existed"] != true {
		t.Fatal("response should indicate config_existed=true")
	}
}

func TestSetupQualityGates_DoesNotOverwriteExistingStandards(t *testing.T) {
	t.Parallel()
	h, server, _ := makeToolHandler(t)

	dir := t.TempDir()
	server.SetActiveCodebase(dir)

	// Pre-create standards file.
	existing := "# My Custom Standards\n\nDo not overwrite me."
	if err := os.WriteFile(filepath.Join(dir, "gasoline-code-standards.md"), []byte(existing), 0644); err != nil {
		t.Fatal(err)
	}

	resp := callConfigureRaw(h, `{"what":"setup_quality_gates"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("expected success, got error: %s", firstText(result))
	}

	// Verify standards file was NOT overwritten.
	standardsBytes, _ := os.ReadFile(filepath.Join(dir, "gasoline-code-standards.md"))
	if string(standardsBytes) != existing {
		t.Fatalf("gasoline-code-standards.md was overwritten")
	}
}

func TestSetupQualityGates_CustomTargetDir(t *testing.T) {
	t.Parallel()
	h, server, _ := makeToolHandler(t)

	dir := t.TempDir()
	server.SetActiveCodebase(dir)

	subdir := filepath.Join(dir, "subproject")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatal(err)
	}

	resp := callConfigureRaw(h, `{"what":"setup_quality_gates","target_dir":"`+subdir+`"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("expected success, got error: %s", firstText(result))
	}

	// Verify files created in subdir.
	if _, err := os.Stat(filepath.Join(subdir, ".gasoline.json")); err != nil {
		t.Fatal(".gasoline.json not created in target_dir")
	}
}

func TestSetupQualityGates_RejectsTargetDirOutsideProject(t *testing.T) {
	t.Parallel()
	h, server, _ := makeToolHandler(t)

	dir := t.TempDir()
	server.SetActiveCodebase(dir)

	outsideDir := t.TempDir() // Different temp dir — outside project.

	resp := callConfigureRaw(h, `{"what":"setup_quality_gates","target_dir":"`+outsideDir+`"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("expected error for target_dir outside project")
	}
	text := firstText(result)
	if !strings.Contains(text, "outside") {
		t.Fatalf("error should mention 'outside', got: %s", text)
	}
}

func TestSetupQualityGates_RequiresActiveCodebase(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	// No active codebase set.
	resp := callConfigureRaw(h, `{"what":"setup_quality_gates"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("expected error when no active codebase is set")
	}
	text := firstText(result)
	if !strings.Contains(text, "codebase") || !strings.Contains(text, "active") {
		t.Fatalf("error should mention active codebase, got: %s", text)
	}
}

func TestSetupQualityGates_CustomStandardsPathInExistingConfig(t *testing.T) {
	t.Parallel()
	h, server, _ := makeToolHandler(t)

	dir := t.TempDir()
	server.SetActiveCodebase(dir)

	// Pre-create config pointing to custom standards file.
	config := `{"code_standards":"docs/my-patterns.md"}`
	if err := os.WriteFile(filepath.Join(dir, ".gasoline.json"), []byte(config), 0644); err != nil {
		t.Fatal(err)
	}

	resp := callConfigureRaw(h, `{"what":"setup_quality_gates"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("expected success, got error: %s", firstText(result))
	}

	// Should NOT create gasoline-code-standards.md because config points elsewhere.
	if _, err := os.Stat(filepath.Join(dir, "gasoline-code-standards.md")); err == nil {
		t.Fatal("should not create default standards file when config points to a custom path")
	}
}

func TestSetupQualityGates_SnakeCaseResponse(t *testing.T) {
	t.Parallel()
	h, server, _ := makeToolHandler(t)

	dir := t.TempDir()
	server.SetActiveCodebase(dir)

	resp := callConfigureRaw(h, `{"what":"setup_quality_gates"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("expected success, got error: %s", firstText(result))
	}

	jsonPart := extractJSONFromText(firstText(result))
	assertSnakeCaseFields(t, jsonPart)
}

func TestSetupQualityGates_ResponseContainsSuggestions(t *testing.T) {
	t.Parallel()
	h, server, _ := makeToolHandler(t)

	dir := t.TempDir()
	server.SetActiveCodebase(dir)

	resp := callConfigureRaw(h, `{"what":"setup_quality_gates"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("expected success, got error: %s", firstText(result))
	}

	data := extractResultJSON(t, result)
	suggestions, ok := data["suggestions"].([]any)
	if !ok || len(suggestions) == 0 {
		t.Fatal("response should include suggestions array")
	}
}

func TestSetupQualityGates_ResponseContainsDefaults(t *testing.T) {
	t.Parallel()
	h, server, _ := makeToolHandler(t)

	dir := t.TempDir()
	server.SetActiveCodebase(dir)

	resp := callConfigureRaw(h, `{"what":"setup_quality_gates"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("expected success, got error: %s", firstText(result))
	}

	data := extractResultJSON(t, result)
	defaults, ok := data["defaults"].(map[string]any)
	if !ok {
		t.Fatal("response should include defaults object")
	}
	if defaults["file_size_limit"] == nil {
		t.Fatal("defaults should include file_size_limit")
	}
	if defaults["code_standards"] == nil {
		t.Fatal("defaults should include code_standards")
	}
}

func TestSetupQualityGates_ResponseContainsHookConfig(t *testing.T) {
	t.Parallel()
	h, server, _ := makeToolHandler(t)

	dir := t.TempDir()
	server.SetActiveCodebase(dir)

	resp := callConfigureRaw(h, `{"what":"setup_quality_gates"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("expected success, got error: %s", firstText(result))
	}

	data := extractResultJSON(t, result)
	hookConfig, ok := data["hook_config"].(map[string]any)
	if !ok {
		t.Fatal("response should include hook_config object")
	}
	if hookConfig["description"] == nil {
		t.Fatal("hook_config should include description")
	}
	if hookConfig["command_hook_json"] == nil {
		t.Fatal("hook_config should include command_hook_json")
	}
	if hookConfig["prompt_hook_json"] == nil {
		t.Fatal("hook_config should include prompt_hook_json")
	}
	if hookConfig["settings_path"] == nil {
		t.Fatal("hook_config should include settings_path")
	}
}
