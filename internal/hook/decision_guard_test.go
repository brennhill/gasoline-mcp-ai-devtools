// decision_guard_test.go — Tests for decision guard hook.

package hook

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunDecisionGuard_PatternMatch(t *testing.T) {
	projectRoot := setupDecisionProject(t)
	input := Input{
		ToolName: "Edit",
		ToolInput: json.RawMessage(`{"file_path":"` + filepath.Join(projectRoot, "handler.go") +
			`","new_string":"client := &http.Client{Timeout: 5 * time.Second}"}`),
	}

	result := RunDecisionGuard(input, projectRoot)
	if result == nil {
		t.Fatal("expected decision guard result for http.Client{}")
	}
	ctx := result.FormatContext()
	if !strings.Contains(ctx, "DECISION-001") {
		t.Errorf("expected DECISION-001 in: %s", ctx)
	}
	if !strings.Contains(ctx, "shared HTTP client") {
		t.Errorf("expected 'shared HTTP client' in: %s", ctx)
	}
	if !strings.Contains(ctx, "DECISION GUARD") {
		t.Errorf("expected 'DECISION GUARD' in: %s", ctx)
	}
}

func TestRunDecisionGuard_RegexMatch(t *testing.T) {
	projectRoot := setupDecisionProject(t)
	input := Input{
		ToolName: "Edit",
		ToolInput: json.RawMessage(`{"file_path":"` + filepath.Join(projectRoot, "handler.go") +
			`","new_string":"import \"database/sql\"\n\nfunc q() { db, _ := sql.Open(\"pg\", \"\") }"}`),
	}

	result := RunDecisionGuard(input, projectRoot)
	if result == nil {
		t.Fatal("expected decision guard result for database/sql")
	}
	ctx := result.FormatContext()
	if !strings.Contains(ctx, "DECISION-002") {
		t.Errorf("expected DECISION-002 in: %s", ctx)
	}
}

func TestRunDecisionGuard_NoMatch(t *testing.T) {
	projectRoot := setupDecisionProject(t)
	input := Input{
		ToolName: "Edit",
		ToolInput: json.RawMessage(`{"file_path":"` + filepath.Join(projectRoot, "handler.go") +
			`","new_string":"func HandleHealth(w http.ResponseWriter, r *http.Request) {\n\tw.WriteHeader(200)\n}"}`),
	}

	result := RunDecisionGuard(input, projectRoot)
	if result != nil {
		t.Errorf("expected nil result for non-violating edit, got: %s", result.FormatContext())
	}
}

func TestRunDecisionGuard_ExpiredDecision(t *testing.T) {
	projectRoot := setupDecisionProject(t)
	// The expired decision (DECISION-003) matches "this-should-never-match-anything-real"
	// which won't match normal code, but let's also test that expired decisions are skipped.
	input := Input{
		ToolName: "Edit",
		ToolInput: json.RawMessage(`{"file_path":"` + filepath.Join(projectRoot, "handler.go") +
			`","new_string":"this-should-never-match-anything-real"}`),
	}

	result := RunDecisionGuard(input, projectRoot)
	if result != nil {
		t.Errorf("expected nil result for expired decision, got: %s", result.FormatContext())
	}
}

func TestRunDecisionGuard_ReadIgnored(t *testing.T) {
	projectRoot := setupDecisionProject(t)
	input := Input{
		ToolName:  "Read",
		ToolInput: json.RawMessage(`{"file_path":"` + filepath.Join(projectRoot, "handler.go") + `"}`),
	}

	result := RunDecisionGuard(input, projectRoot)
	if result != nil {
		t.Errorf("expected nil result for Read tool, got: %s", result.FormatContext())
	}
}

func TestRunDecisionGuard_NoDecisionsFile(t *testing.T) {
	root := t.TempDir()
	// Project without decisions.json.
	writeFile(t, root, ".kaboom.json", `{}`)
	writeFile(t, root, "handler.go", "package main\n")

	input := Input{
		ToolName: "Edit",
		ToolInput: json.RawMessage(`{"file_path":"` + filepath.Join(root, "handler.go") +
			`","new_string":"client := &http.Client{}"}`),
	}

	result := RunDecisionGuard(input, root)
	if result != nil {
		t.Errorf("expected nil result when no decisions.json exists, got: %s", result.FormatContext())
	}
}

func TestRunDecisionGuard_InlineRegex(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, ".kaboom"), 0o755)
	writeFile(t, root, ".kaboom.json", `{}`)
	writeFile(t, root, ".kaboom/decisions.json", `[
		{"id":"INLINE-RE","rule":"No fmt.Println","pattern":"re:fmt\\.Println\\(","reason":"Use structured logging"}
	]`)
	writeFile(t, root, "main.go", "package main\n")

	input := Input{
		ToolName: "Edit",
		ToolInput: json.RawMessage(`{"file_path":"` + filepath.Join(root, "main.go") +
			`","new_string":"fmt.Println(\"debug\")"}`),
	}

	result := RunDecisionGuard(input, root)
	if result == nil {
		t.Fatal("expected match for inline regex pattern")
	}
	if !strings.Contains(result.FormatContext(), "INLINE-RE") {
		t.Errorf("expected INLINE-RE in output: %s", result.FormatContext())
	}
}

func TestMatchesDecision_InvalidRegex(t *testing.T) {
	d := Decision{Regex: "[invalid"}
	if matchesDecision(d, "anything", "") {
		t.Error("invalid regex should not match")
	}
}

func TestIsExpired(t *testing.T) {
	tests := []struct {
		expires string
		expired bool
	}{
		{"", false},
		{"2099-12-31", false},
		{"2020-01-01", true},
		{"invalid-date", false},
	}
	for _, tt := range tests {
		d := Decision{Expires: tt.expires}
		if got := isExpired(d); got != tt.expired {
			t.Errorf("isExpired(expires=%q) = %v, want %v", tt.expires, got, tt.expired)
		}
	}
}

func setupDecisionProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, ".kaboom"), 0o755)

	writeFile(t, root, ".kaboom.json", `{"code_standards":"standards.md","file_size_limit":800}`)
	writeFile(t, root, "handler.go", "package main\n")

	writeFile(t, root, ".kaboom/decisions.json", `[
		{
			"id": "DECISION-001",
			"rule": "Use shared HTTP client from pkg/httpclient",
			"pattern": "http.Client{",
			"reason": "Shared client has timeouts and retries.",
			"enforced": "2026-01-15"
		},
		{
			"id": "DECISION-002",
			"rule": "All database queries must go through the db package",
			"regex": "database/sql|sql\\.Open",
			"reason": "Centralized connection pooling.",
			"enforced": "2026-02-01"
		},
		{
			"id": "DECISION-003",
			"rule": "Expired decision",
			"pattern": "this-should-never-match-anything-real",
			"enforced": "2025-01-01",
			"expires": "2025-06-01"
		}
	]`)

	return root
}
