// quality_gate_test.go — Tests for quality gate hook logic.

package hook

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func makeEditInput(filePath string) Input {
	ti, _ := json.Marshal(map[string]string{"file_path": filePath})
	return Input{
		ToolName:  "Edit",
		ToolInput: ti,
	}
}

func setupProject(t *testing.T, lines int) (projectDir, filePath string) {
	t.Helper()
	dir := t.TempDir()

	// Write .gasoline.json
	cfg := `{"code_standards":"standards.md","file_size_limit":100}`
	if err := os.WriteFile(filepath.Join(dir, ".gasoline.json"), []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}

	// Write standards doc
	if err := os.WriteFile(filepath.Join(dir, "standards.md"), []byte("# Standards\n\n- Rule 1\n- Rule 2\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Write a source file with N lines
	var content strings.Builder
	for i := 0; i < lines; i++ {
		content.WriteString("line\n")
	}
	fp := filepath.Join(dir, "main.go")
	if err := os.WriteFile(fp, []byte(content.String()), 0644); err != nil {
		t.Fatal(err)
	}

	return dir, fp
}

func TestRunQualityGate_NotEditOrWrite(t *testing.T) {
	t.Parallel()
	in := Input{ToolName: "Bash"}
	if result := RunQualityGate(in); result != nil {
		t.Error("expected nil for non-Edit/Write tool")
	}
}

func TestRunQualityGate_NoFilePath(t *testing.T) {
	t.Parallel()
	in := Input{ToolName: "Edit", ToolInput: json.RawMessage(`{}`)}
	if result := RunQualityGate(in); result != nil {
		t.Error("expected nil for missing file_path")
	}
}

func TestRunQualityGate_NoGasolineConfig(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	fp := filepath.Join(dir, "foo.go")
	os.WriteFile(fp, []byte("package main\n"), 0644)

	in := makeEditInput(fp)
	if result := RunQualityGate(in); result != nil {
		t.Error("expected nil when no .gasoline.json exists")
	}
}

func TestRunQualityGate_InjectsStandards(t *testing.T) {
	t.Parallel()
	_, fp := setupProject(t, 50) // under limit

	in := makeEditInput(fp)
	result := RunQualityGate(in)
	if result == nil {
		t.Fatal("expected quality gate result")
	}
	if !strings.Contains(result.Context, "PROJECT CODE STANDARDS") {
		t.Error("missing standards header")
	}
	if !strings.Contains(result.Context, "Rule 1") {
		t.Error("missing standards content")
	}
	if !strings.Contains(result.Context, "QUALITY GATE") {
		t.Error("missing review instruction")
	}
}

func TestRunQualityGate_FileSizeWarning(t *testing.T) {
	t.Parallel()
	_, fp := setupProject(t, 120) // over limit of 100

	in := makeEditInput(fp)
	result := RunQualityGate(in)
	if result == nil {
		t.Fatal("expected quality gate result")
	}
	if !strings.Contains(result.Context, "WARNING:") {
		t.Error("missing file size warning")
	}
	if !strings.Contains(result.Context, "must be split") {
		t.Error("missing split instruction")
	}
}

func TestRunQualityGate_FileSizeNote(t *testing.T) {
	t.Parallel()
	_, fp := setupProject(t, 95) // 95% of 100 limit

	in := makeEditInput(fp)
	result := RunQualityGate(in)
	if result == nil {
		t.Fatal("expected quality gate result")
	}
	if !strings.Contains(result.Context, "NOTE:") {
		t.Error("missing approaching-limit note")
	}
}

func TestRunQualityGate_WriteToolAlsoWorks(t *testing.T) {
	t.Parallel()
	_, fp := setupProject(t, 50)

	ti, _ := json.Marshal(map[string]string{"file_path": fp})
	in := Input{ToolName: "Write", ToolInput: ti}
	result := RunQualityGate(in)
	if result == nil {
		t.Fatal("expected quality gate result for Write tool")
	}
}

func TestRunQualityGate_DefaultConfig(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Minimal .gasoline.json with no code_standards field.
	os.WriteFile(filepath.Join(dir, ".gasoline.json"), []byte(`{}`), 0644)
	// Write the default standards file.
	os.WriteFile(filepath.Join(dir, "gasoline-code-standards.md"), []byte("# Default\n"), 0644)

	fp := filepath.Join(dir, "main.go")
	os.WriteFile(fp, []byte("package main\n"), 0644)

	in := makeEditInput(fp)
	result := RunQualityGate(in)
	if result == nil {
		t.Fatal("expected result with default config")
	}
	if !strings.Contains(result.Context, "Default") {
		t.Error("should load default standards file")
	}
}

func TestFindProjectRoot_Nested(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".gasoline.json"), []byte(`{}`), 0644)
	nested := filepath.Join(dir, "src", "pkg")
	os.MkdirAll(nested, 0755)
	fp := filepath.Join(nested, "foo.go")
	os.WriteFile(fp, []byte("package pkg\n"), 0644)

	root := findProjectRoot(fp)
	if root != dir {
		t.Errorf("findProjectRoot = %q, want %q", root, dir)
	}
}

func TestCountLines(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	tests := []struct {
		name    string
		content string
		want    int
	}{
		{"empty", "", 0},
		{"one_line_no_newline", "hello", 1},
		{"one_line_with_newline", "hello\n", 1},
		{"three_lines", "a\nb\nc\n", 3},
	}
	for _, tt := range tests {
		fp := filepath.Join(dir, tt.name)
		os.WriteFile(fp, []byte(tt.content), 0644)
		got, err := countLines(fp)
		if err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}
		if got != tt.want {
			t.Errorf("%s: countLines = %d, want %d", tt.name, got, tt.want)
		}
	}
}
