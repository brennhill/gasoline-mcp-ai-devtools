// ai_context_test.go — Tests for AI context file generation (CLAUDE.md, skill, hooks, .mcp.json).

package scaffold

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ============================================
// AI Context Generation
// ============================================

func TestWriteAIContext_CreatesClaudeMD(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		Description:  "a todo app with drag and drop",
		Audience:     "just_me",
		FirstFeature: "drag and drop reordering",
		Name:         "todo-app",
	}

	if err := WriteAIContext(dir, cfg); err != nil {
		t.Fatalf("WriteAIContext: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, ".claude", "CLAUDE.md"))
	if err != nil {
		t.Fatalf("read CLAUDE.md: %v", err)
	}

	body := string(content)
	checks := []struct {
		label    string
		contains string
	}{
		{"project name", "todo-app"},
		{"description", "a todo app with drag and drop"},
		{"audience", "just_me"},
		{"first feature", "drag and drop reordering"},
		{"stack info", "React"},
		{"stack info", "TypeScript"},
		{"stack info", "Tailwind"},
		{"stack info", "shadcn"},
		{"dev command", "pnpm dev"},
		{"tailwind-first rule", "Tailwind"},
	}

	for _, c := range checks {
		if !strings.Contains(body, c.contains) {
			t.Errorf("CLAUDE.md missing %s (expected %q)", c.label, c.contains)
		}
	}
}

func TestWriteAIContext_CreatesBootstrapSkill(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		Description:  "a todo app",
		Audience:     "just_me",
		FirstFeature: "drag and drop",
		Name:         "todo-app",
	}

	if err := WriteAIContext(dir, cfg); err != nil {
		t.Fatalf("WriteAIContext: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, ".claude", "skills", "strum-dev", "SKILL.md"))
	if err != nil {
		t.Fatalf("read SKILL.md: %v", err)
	}

	body := string(content)
	checks := []struct {
		label    string
		contains string
	}{
		{"tailwind-first", "Tailwind"},
		{"shadcn-first", "shadcn"},
		{"lucide icons", "lucide"},
		{"no inline styles", "style"},
		{"200 LOC limit", "200"},
		{"path alias", "@/"},
	}

	for _, c := range checks {
		if !strings.Contains(body, c.contains) {
			t.Errorf("SKILL.md missing %s (expected %q)", c.label, c.contains)
		}
	}
}

func TestWriteAIContext_CreatesSessionStartHook(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		Description:  "a todo app",
		Audience:     "just_me",
		FirstFeature: "drag and drop",
		Name:         "todo-app",
	}

	if err := WriteAIContext(dir, cfg); err != nil {
		t.Fatalf("WriteAIContext: %v", err)
	}

	path := filepath.Join(dir, ".claude", "hooks", "session-start.js")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read session-start.js: %v", err)
	}

	body := string(content)
	if !strings.Contains(body, "additionalContext") && !strings.Contains(body, "SKILL.md") {
		t.Error("session-start.js should reference bootstrap skill or additionalContext")
	}
}

func TestWriteAIContext_CreatesMCPJson(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		Description:  "a todo app",
		Audience:     "just_me",
		FirstFeature: "drag and drop",
		Name:         "todo-app",
	}

	if err := WriteAIContext(dir, cfg); err != nil {
		t.Fatalf("WriteAIContext: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, ".mcp.json"))
	if err != nil {
		t.Fatalf("read .mcp.json: %v", err)
	}

	var cfg2 map[string]any
	if err := json.Unmarshal(content, &cfg2); err != nil {
		t.Fatalf(".mcp.json is not valid JSON: %v", err)
	}

	servers, ok := cfg2["mcpServers"]
	if !ok {
		t.Fatal(".mcp.json should have mcpServers key")
	}

	serversMap, ok := servers.(map[string]any)
	if !ok {
		t.Fatal(".mcp.json mcpServers should be an object")
	}

	if _, ok := serversMap["gasoline"]; !ok {
		t.Error(".mcp.json should have a 'gasoline' server configured")
	}
}

func TestWriteAIContext_AllFilesExist(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		Description:  "a todo app",
		Audience:     "just_me",
		FirstFeature: "drag and drop",
		Name:         "todo-app",
	}

	if err := WriteAIContext(dir, cfg); err != nil {
		t.Fatalf("WriteAIContext: %v", err)
	}

	expectedFiles := []string{
		".claude/CLAUDE.md",
		".claude/skills/strum-dev/SKILL.md",
		".claude/hooks/session-start.js",
		".mcp.json",
	}

	for _, f := range expectedFiles {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Errorf("missing expected file: %s", f)
		}
	}
}
