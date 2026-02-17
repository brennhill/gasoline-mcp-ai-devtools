package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/dev-console/dev-console/internal/capture"
)

func TestGenerateErrorID(t *testing.T) {
	t.Parallel()

	id := generateErrorID("boom", "stacktrace", "https://app.example.com")
	re := regexp.MustCompile(`^err_\d+_[0-9a-f]{8}$`)
	if !re.MatchString(id) {
		t.Fatalf("generateErrorID() = %q, want err_{timestamp}_{8hex}", id)
	}
}

func TestGenerateTestFilename(t *testing.T) {
	t.Parallel()

	// Basic sanitization: colons, quotes, spaces become dashes
	name := generateTestFilename(`Login failed: can't "submit"`, "playwright")
	if !strings.HasSuffix(name, ".spec.ts") {
		t.Fatalf("playwright filename = %q, want .spec.ts", name)
	}
	if strings.ContainsAny(name, `:'"/ \<>*?|%%`) {
		t.Fatalf("filename should be sanitized, got %q", name)
	}

	// Vitest/jest extension
	vitest := generateTestFilename("Short message", "vitest")
	if !strings.HasSuffix(vitest, ".test.ts") {
		t.Fatalf("vitest filename = %q, want .test.ts", vitest)
	}

	// Long input capped at ≤50
	long := strings.Repeat("x", 80)
	longName := generateTestFilename(long, "playwright")
	stem := strings.TrimSuffix(longName, ".spec.ts")
	if len(stem) > 50 {
		t.Fatalf("sanitized long filename stem length = %d, want ≤50", len(stem))
	}

	// URL-like input produces safe slug (issue #96)
	urlLike := generateTestFilename(`data:text/html,%3C%21doctype%20html`, "playwright")
	if strings.ContainsAny(urlLike, `/:,%<>`) {
		t.Fatalf("URL-like input should be fully sanitized, got %q", urlLike)
	}
	stem = strings.TrimSuffix(urlLike, ".spec.ts")
	if stem == "" || stem == "-" {
		t.Fatalf("URL-like input should not produce empty stem, got %q", stem)
	}

	// Empty/whitespace input → fallback
	empty := generateTestFilename("", "playwright")
	if strings.TrimSuffix(empty, ".spec.ts") != "generated-test" {
		t.Fatalf("empty input should fallback to 'generated-test', got %q", empty)
	}
	whitespace := generateTestFilename("   ", "playwright")
	if strings.TrimSuffix(whitespace, ".spec.ts") != "generated-test" {
		t.Fatalf("whitespace input should fallback to 'generated-test', got %q", whitespace)
	}

	// Consecutive special chars collapsed
	multi := generateTestFilename("a///b***c", "playwright")
	stem = strings.TrimSuffix(multi, ".spec.ts")
	if strings.Contains(stem, "--") {
		t.Fatalf("consecutive dashes should be collapsed, got %q", stem)
	}

	// No leading/trailing dashes
	if strings.HasPrefix(stem, "-") || strings.HasSuffix(stem, "-") {
		t.Fatalf("stem should not have leading/trailing dashes, got %q", stem)
	}

	// Reserved Windows filenames should be rewritten to a safe name
	reserved := generateTestFilename("CON", "playwright")
	if strings.TrimSuffix(reserved, ".spec.ts") != "test-con" {
		t.Fatalf("reserved filename should be rewritten, got %q", reserved)
	}
	reserved = generateTestFilename("lpt1", "vitest")
	if strings.TrimSuffix(reserved, ".test.ts") != "test-lpt1" {
		t.Fatalf("reserved filename should be rewritten for vitest, got %q", reserved)
	}
}

func TestExtractSelectorsFromActions(t *testing.T) {
	t.Parallel()

	actions := []capture.EnhancedAction{
		{
			Type: "click",
			Selectors: map[string]any{
				"testId": "submit-btn",
				"role":   map[string]any{"role": "button"},
				"id":     "submit",
			},
		},
		{
			Type: "click",
			Selectors: map[string]any{
				"testId": "submit-btn", // duplicate
			},
		},
		{
			Type:      "click",
			Selectors: nil,
		},
	}

	selectors := extractSelectorsFromActions(actions)
	if len(selectors) != 3 {
		t.Fatalf("extractSelectorsFromActions len = %d, want 3 unique selectors; got %+v", len(selectors), selectors)
	}

	joined := strings.Join(selectors, "\n")
	for _, want := range []string{
		`[data-testid="submit-btn"]`,
		`[role="button"]`,
		`#submit`,
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("selectors missing %q: %+v", want, selectors)
		}
	}
}

func TestNormalizeTimestamp(t *testing.T) {
	t.Parallel()

	ts := time.Date(2026, 2, 11, 8, 40, 0, 0, time.UTC)
	got := normalizeTimestamp(ts.Format(time.RFC3339))
	if got != ts.UnixMilli() {
		t.Fatalf("normalizeTimestamp(RFC3339) = %d, want %d", got, ts.UnixMilli())
	}

	if bad := normalizeTimestamp("not-a-timestamp"); bad != 0 {
		t.Fatalf("normalizeTimestamp(invalid) = %d, want 0", bad)
	}
}

func TestGeneratePlaywrightScript(t *testing.T) {
	t.Parallel()

	actions := []capture.EnhancedAction{
		{Type: "click", Selectors: map[string]any{"target": "#login"}},
		{Type: "input", Selectors: map[string]any{"target": "#email"}, Value: "user@example.com"},
		{Type: "navigate", ToURL: "https://app.example.com/dashboard"},
		{Type: "wait"},
	}

	script := generatePlaywrightScript(actions, "Cannot read property", "https://app.example.com")

	for _, want := range []string{
		"import { test, expect } from '@playwright/test';",
		"await page.goto('https://app.example.com');",
		"await page.click('#login');",
		"await page.fill('#email', 'user@example.com');",
		"await page.goto('https://app.example.com/dashboard');",
		"await page.waitForTimeout(100);",
		"// Expected error: Cannot read property",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("generated script missing %q\nscript:\n%s", want, script)
		}
	}
}

func TestValidateTestFilePath(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	if err := validateTestFilePath("", projectDir); err == nil {
		t.Fatal("validateTestFilePath should reject empty path")
	}
	if err := validateTestFilePath("../outside.spec.ts", projectDir); err == nil {
		t.Fatal("validateTestFilePath should reject traversal path")
	}

	inside := filepath.Join(projectDir, "tests", "flow.spec.ts")
	if err := validateTestFilePath(inside, projectDir); err != nil {
		t.Fatalf("validateTestFilePath(abs inside) error = %v", err)
	}
	if err := validateTestFilePath("tests/flow.spec.ts", projectDir); err != nil {
		t.Fatalf("validateTestFilePath(rel inside) error = %v", err)
	}
}

func TestValidateSelector(t *testing.T) {
	t.Parallel()

	if err := validateSelector(""); err == nil {
		t.Fatal("validateSelector should reject empty selector")
	}

	if err := validateSelector(strings.Repeat("a", 1001)); err == nil {
		t.Fatal("validateSelector should reject >1000 chars")
	}

	for _, dangerous := range []string{
		"javascript:alert(1)",
		"<script>alert(1)</script>",
		"img[onerror=alert(1)]",
		"img[onload=run()]",
	} {
		if err := validateSelector(dangerous); err == nil {
			t.Fatalf("validateSelector should reject dangerous pattern %q", dangerous)
		}
	}

	if err := validateSelector("1bad"); err == nil {
		t.Fatal("validateSelector should reject selectors with invalid starting char")
	}

	for _, valid := range []string{"#id", ".class", "[role='button']", "*", "button"} {
		if err := validateSelector(valid); err != nil {
			t.Fatalf("validateSelector(%q) unexpected error: %v", valid, err)
		}
	}
}

func TestExtractSelectorsFromTestFile(t *testing.T) {
	t.Parallel()

	content := `
		await page.getByTestId('submit-btn').click();
		await page.locator("#email").fill("user@example.com");
		await page.getByRole("button").click();
		await page.getByText("Save").click();
		await document.querySelector("#root");
		await document.querySelectorAll(".item");
		await page.locator("#email").click(); // duplicate
	`
	selectors := extractSelectorsFromTestFile(content)

	if len(selectors) != 6 {
		t.Fatalf("extractSelectorsFromTestFile len = %d, want 6 unique selectors; got %+v", len(selectors), selectors)
	}
	joined := strings.Join(selectors, "\n")
	for _, want := range []string{"submit-btn", "#email", "button", "Save", "#root", ".item"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("selectors missing %q: %+v", want, selectors)
		}
	}
}

func TestAnalyzeTestFile(t *testing.T) {
	projectDir := t.TempDir()
	h := &ToolHandler{}

	missingReq := TestHealRequest{Action: "analyze", TestFile: "tests/missing.spec.ts"}
	if _, err := h.analyzeTestFile(missingReq, projectDir); err == nil || !strings.Contains(err.Error(), ErrTestFileNotFound) {
		t.Fatalf("analyzeTestFile missing error = %v, want %q", err, ErrTestFileNotFound)
	}

	testPath := filepath.Join(projectDir, "tests", "login.spec.ts")
	if err := os.MkdirAll(filepath.Dir(testPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	content := `await page.locator("#login").click(); await page.getByRole("button").click();`
	if err := os.WriteFile(testPath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	req := TestHealRequest{Action: "analyze", TestFile: testPath}
	selectors, err := h.analyzeTestFile(req, projectDir)
	if err != nil {
		t.Fatalf("analyzeTestFile(valid) error = %v", err)
	}
	if len(selectors) != 2 {
		t.Fatalf("analyzeTestFile selectors len = %d, want 2; got %+v", len(selectors), selectors)
	}
}

func TestHealSelectorAndRepairSelectors(t *testing.T) {
	t.Parallel()

	h := &ToolHandler{}

	idHealed, err := h.healSelector("#login")
	if err != nil {
		t.Fatalf("healSelector(#login) error = %v", err)
	}
	if idHealed.NewSelector != "[data-testid='login']" || idHealed.Strategy != "testid_match" {
		t.Fatalf("unexpected id healing result: %+v", idHealed)
	}

	classHealed, err := h.healSelector(".button")
	if err != nil {
		t.Fatalf("healSelector(.button) error = %v", err)
	}
	if classHealed.Strategy != "structural_match" || classHealed.Confidence != 0.3 {
		t.Fatalf("unexpected class healing result: %+v", classHealed)
	}

	if _, err := h.healSelector("xpath=//div"); err == nil {
		t.Fatal("healSelector should fail for unsupported selector style")
	}

	result, err := h.repairSelectors(TestHealRequest{
		BrokenSelectors: []string{"#login", ".button", "", "javascript:alert(1)"},
		AutoApply:       true,
	}, t.TempDir())
	if err != nil {
		t.Fatalf("repairSelectors() error = %v", err)
	}

	if len(result.Healed) != 2 || len(result.Unhealed) != 2 {
		t.Fatalf("repairSelectors unexpected healed/unhealed: %+v", result)
	}
	if result.Summary.TotalBroken != 4 {
		t.Fatalf("TotalBroken = %d, want 4", result.Summary.TotalBroken)
	}
	if result.Summary.HealedAuto != 0 {
		t.Fatalf("HealedAuto = %d, want 0 (no selector has confidence >=0.9)", result.Summary.HealedAuto)
	}
	if result.Summary.HealedManual != 1 {
		t.Fatalf("HealedManual = %d, want 1 (#login has confidence 0.6)", result.Summary.HealedManual)
	}
	if result.Summary.Unhealed != 2 {
		t.Fatalf("Unhealed = %d, want 2", result.Summary.Unhealed)
	}
}

func TestFindTestFiles(t *testing.T) {
	root := t.TempDir()

	paths := []string{
		filepath.Join(root, "a.spec.ts"),
		filepath.Join(root, "b.test.js"),
		filepath.Join(root, "c.ts"),
		filepath.Join(root, "node_modules", "ignored.spec.ts"),
		filepath.Join(root, ".git", "ignored.test.ts"),
	}
	for _, p := range paths {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("MkdirAll(%q) error = %v", p, err)
		}
		if err := os.WriteFile(p, []byte("content"), 0o644); err != nil {
			t.Fatalf("WriteFile(%q) error = %v", p, err)
		}
	}

	files, err := findTestFiles(root)
	if err != nil {
		t.Fatalf("findTestFiles() error = %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("findTestFiles len = %d, want 2; files=%+v", len(files), files)
	}
	for _, f := range files {
		if strings.Contains(f, "node_modules") || strings.Contains(f, ".git") {
			t.Fatalf("findTestFiles should skip ignored dirs; got %q", f)
		}
	}
}

func TestHealTestBatch(t *testing.T) {
	projectDir := t.TempDir()
	h := &ToolHandler{}

	// Non-directory should fail.
	notDirPath := filepath.Join(projectDir, "single-file.spec.ts")
	if err := os.WriteFile(notDirPath, []byte("test"), 0o644); err != nil {
		t.Fatalf("WriteFile(notDirPath) error = %v", err)
	}
	if _, err := h.healTestBatch(TestHealRequest{Action: "batch", TestDir: notDirPath}, projectDir); err == nil {
		t.Fatal("healTestBatch should fail when test_dir is not a directory")
	}

	testDir := filepath.Join(projectDir, "tests")
	if err := os.MkdirAll(testDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(testDir) error = %v", err)
	}

	good := filepath.Join(testDir, "good.spec.ts")
	goodContent := `await page.locator("#login").click(); await page.locator(".btn").click();`
	if err := os.WriteFile(good, []byte(goodContent), 0o644); err != nil {
		t.Fatalf("WriteFile(good) error = %v", err)
	}

	huge := filepath.Join(testDir, "huge.spec.ts")
	if err := os.WriteFile(huge, []byte(strings.Repeat("x", MaxFileSizeBytes+1)), 0o644); err != nil {
		t.Fatalf("WriteFile(huge) error = %v", err)
	}

	ignoredDir := filepath.Join(testDir, "node_modules")
	if err := os.MkdirAll(ignoredDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(ignoredDir) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(ignoredDir, "ignored.spec.ts"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile(ignored) error = %v", err)
	}

	batchResult, err := h.healTestBatch(TestHealRequest{Action: "batch", TestDir: testDir}, projectDir)
	if err != nil {
		t.Fatalf("healTestBatch(valid) error = %v", err)
	}

	if batchResult.FilesProcessed != 1 {
		t.Fatalf("FilesProcessed = %d, want 1", batchResult.FilesProcessed)
	}
	if batchResult.FilesSkipped != 1 {
		t.Fatalf("FilesSkipped = %d, want 1 (huge file)", batchResult.FilesSkipped)
	}
	if batchResult.TotalSelectors != 2 {
		t.Fatalf("TotalSelectors = %d, want 2", batchResult.TotalSelectors)
	}
	if batchResult.TotalHealed != 2 || batchResult.TotalUnhealed != 0 {
		t.Fatalf("unexpected heal totals: %+v", batchResult)
	}
}
