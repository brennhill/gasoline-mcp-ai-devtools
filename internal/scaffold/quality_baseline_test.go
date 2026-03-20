// quality_baseline_test.go — Tests for quality baseline config file generation.

package scaffold

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ============================================
// Quality Baseline: File Generation
// ============================================

func TestWriteQualityBaseline_CreatesPrettierRC(t *testing.T) {
	dir := t.TempDir()
	if err := WriteQualityBaseline(dir); err != nil {
		t.Fatalf("WriteQualityBaseline: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, ".prettierrc"))
	if err != nil {
		t.Fatalf("read .prettierrc: %v", err)
	}

	var cfg map[string]any
	if err := json.Unmarshal(content, &cfg); err != nil {
		t.Fatalf(".prettierrc is not valid JSON: %v", err)
	}

	if cfg["singleQuote"] != true {
		t.Error(".prettierrc: singleQuote should be true")
	}
	if cfg["semi"] != false {
		t.Error(".prettierrc: semi should be false")
	}
}

func TestWriteQualityBaseline_CreatesVitestConfig(t *testing.T) {
	dir := t.TempDir()
	if err := WriteQualityBaseline(dir); err != nil {
		t.Fatalf("WriteQualityBaseline: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "vitest.config.ts"))
	if err != nil {
		t.Fatalf("read vitest.config.ts: %v", err)
	}

	body := string(content)
	if !strings.Contains(body, "happy-dom") {
		t.Error("vitest.config.ts should reference happy-dom")
	}
	if !strings.Contains(body, "@/") {
		t.Error("vitest.config.ts should include path alias @/")
	}
}

func TestWriteQualityBaseline_CreatesComponentInvariantScript(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "scripts"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := WriteQualityBaseline(dir); err != nil {
		t.Fatalf("WriteQualityBaseline: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "scripts", "check-components.sh"))
	if err != nil {
		t.Fatalf("read check-components.sh: %v", err)
	}

	body := string(content)
	checks := []string{
		"style=",    // checks for inline styles
		"any",       // checks for any type
		"../",       // checks for relative imports
	}
	for _, c := range checks {
		if !strings.Contains(body, c) {
			t.Errorf("check-components.sh should check for %q", c)
		}
	}
}

func TestWriteQualityBaseline_CreatesExampleTest(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "src"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := WriteQualityBaseline(dir); err != nil {
		t.Fatalf("WriteQualityBaseline: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "src", "App.test.tsx"))
	if err != nil {
		t.Fatalf("read App.test.tsx: %v", err)
	}

	body := string(content)
	if !strings.Contains(body, "describe") {
		t.Error("App.test.tsx should contain a describe block")
	}
	if !strings.Contains(body, "expect") {
		t.Error("App.test.tsx should contain expect assertions")
	}
}

func TestWriteQualityBaseline_CreatesAppShell(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "src"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := WriteQualityBaseline(dir); err != nil {
		t.Fatalf("WriteQualityBaseline: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "src", "App.tsx"))
	if err != nil {
		t.Fatalf("read App.tsx: %v", err)
	}

	body := string(content)
	if !strings.Contains(body, "export default") {
		t.Error("App.tsx should have a default export")
	}
}

func TestWriteQualityBaseline_CreatesIndexCSS(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "src"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := WriteQualityBaseline(dir); err != nil {
		t.Fatalf("WriteQualityBaseline: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "src", "index.css"))
	if err != nil {
		t.Fatalf("read index.css: %v", err)
	}

	body := string(content)
	if !strings.Contains(body, "@theme") {
		t.Error("index.css should contain @theme block with design tokens")
	}
	if !strings.Contains(body, "--color-primary") {
		t.Error("index.css should define --color-primary design token")
	}
}

func TestWriteQualityBaseline_CreatesEslintConfig(t *testing.T) {
	dir := t.TempDir()
	if err := WriteQualityBaseline(dir); err != nil {
		t.Fatalf("WriteQualityBaseline: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "eslint.config.js"))
	if err != nil {
		t.Fatalf("read eslint.config.js: %v", err)
	}

	body := string(content)
	checks := []string{
		"@eslint/js",
		"typescript-eslint",
		"react-hooks",
	}
	for _, c := range checks {
		if !strings.Contains(body, c) {
			t.Errorf("eslint.config.js should reference %q", c)
		}
	}
}

func TestWriteQualityBaseline_CreatesTsconfig(t *testing.T) {
	dir := t.TempDir()
	if err := WriteQualityBaseline(dir); err != nil {
		t.Fatalf("WriteQualityBaseline: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		t.Fatalf("read tsconfig.json: %v", err)
	}

	body := string(content)
	checks := []string{
		`"strict": true`,
		`"noUncheckedIndexedAccess": true`,
		`"noUnusedLocals": true`,
		`"noUnusedParameters": true`,
		`"@/*"`,
	}
	for _, c := range checks {
		if !strings.Contains(body, c) {
			t.Errorf("tsconfig.json should contain %q", c)
		}
	}
}

func TestWriteQualityBaseline_CreatesViteConfig(t *testing.T) {
	dir := t.TempDir()
	if err := WriteQualityBaseline(dir); err != nil {
		t.Fatalf("WriteQualityBaseline: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "vite.config.ts"))
	if err != nil {
		t.Fatalf("read vite.config.ts: %v", err)
	}

	body := string(content)
	checks := []string{
		"resolve",
		"alias",
		"@",
		"path.resolve",
	}
	for _, c := range checks {
		if !strings.Contains(body, c) {
			t.Errorf("vite.config.ts should contain %q", c)
		}
	}
}

func TestWriteQualityBaseline_AllFilesValid(t *testing.T) {
	dir := t.TempDir()
	// Create directories that WriteQualityBaseline expects.
	for _, d := range []string{"src", "scripts"} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0755); err != nil {
			t.Fatal(err)
		}
	}

	if err := WriteQualityBaseline(dir); err != nil {
		t.Fatalf("WriteQualityBaseline: %v", err)
	}

	expectedFiles := []string{
		".prettierrc",
		"eslint.config.js",
		"tsconfig.json",
		"vite.config.ts",
		"vitest.config.ts",
		"scripts/check-components.sh",
		"src/App.tsx",
		"src/App.test.tsx",
		"src/index.css",
	}

	for _, f := range expectedFiles {
		path := filepath.Join(dir, f)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("missing expected file: %s", f)
		}
	}
}
